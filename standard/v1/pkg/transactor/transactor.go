package transactor

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"standard-bridge/pkg/listener"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
)

// TODO: unit tests

// TODO: Improve impl to wait on txes async and send in succession

type Transactor struct {
	privateKey        *ecdsa.PrivateKey
	rawClient         *ethclient.Client
	gatewayTransactor gatewayTransactor
	gatewayFilterer   gatewayFilterer
	chainID           *big.Int
	eventChan         <-chan listener.TransferInitiatedEvent
}

type gatewayTransactor interface {
	FinalizeTransfer(opts *bind.TransactOpts, _recipient common.Address,
		_amount *big.Int, _counterpartyIdx *big.Int) (*types.Transaction, error)
}

type gatewayFilterer interface {
	ObtainTransferFinalizedEvent(opts *bind.FilterOpts, counterpartyIdx uint64) (
		listener.TransferFinalizedEvent, bool)
}

func NewTransactor(
	pk *ecdsa.PrivateKey,
	gatewayAddr common.Address,
	ethClient *ethclient.Client,
	gatewayTransactor gatewayTransactor,
	gatewayFilterer gatewayFilterer,
	eventChan <-chan listener.TransferInitiatedEvent,
) *Transactor {
	return &Transactor{
		privateKey:        pk,
		rawClient:         ethClient,
		gatewayTransactor: gatewayTransactor,
		gatewayFilterer:   gatewayFilterer,
		eventChan:         eventChan,
	}
}

func (t *Transactor) Start(ctx context.Context) <-chan struct{} {
	var err error
	t.chainID, err = t.rawClient.ChainID(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get chain id")
	}

	doneChan := make(chan struct{})

	go func() {
		defer close(doneChan)

		for event := range t.eventChan {
			log.Info().Msgf("Received event at Transactor!%+v", event)
			opts, err := t.getTransactOpts(ctx, t.chainID)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to get transact opts")
			}
			if t.transferAlreadyFinalized(ctx, event.TransferIdx) {
				continue
			}
			err = t.sendFinalizeTransfer(ctx, opts, event)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to send finalize transfer")
			}
		}
	}()
	return doneChan
}

// Adaptation of func from oracle repo
func (s *Transactor) getTransactOpts(ctx context.Context, chainID *big.Int) (*bind.TransactOpts, error) {
	auth, err := bind.NewKeyedTransactorWithChainID(s.privateKey, chainID)
	if err != nil {
		return nil, err
	}
	nonce, err := s.rawClient.PendingNonceAt(ctx, auth.From)
	if err != nil {
		return nil, err
	}
	auth.Nonce = big.NewInt(int64(nonce))

	gasTip, err := s.rawClient.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, err
	}

	gasPrice, err := s.rawClient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, err
	}

	gasFeeCap := new(big.Int).Add(gasTip, gasPrice)
	auth.GasFeeCap = gasFeeCap
	auth.GasTipCap = gasTip
	auth.GasLimit = uint64(3000000)

	return auth, nil
}

func (t *Transactor) transferAlreadyFinalized(ctx context.Context, transferIdx uint64) bool {
	// TODO: improve upon this
	opts := &bind.FilterOpts{
		Start: 0,
		End:   nil,
	}
	event, found := t.gatewayFilterer.ObtainTransferFinalizedEvent(opts, transferIdx)
	if found {
		log.Info().Msgf("Transfer already finalized %+v", event)
		return true
	}
	return false
}

func (t *Transactor) sendFinalizeTransfer(ctx context.Context, opts *bind.TransactOpts, event listener.TransferInitiatedEvent) error {
	tx, err := t.gatewayTransactor.FinalizeTransfer(opts,
		common.HexToAddress(event.Recipient),
		big.NewInt(int64(event.Amount)),
		big.NewInt(int64(event.TransferIdx)),
	)
	if err != nil {
		return err
	}
	log.Info().Msgf("Transaction sent, hash: %s", tx.Hash().Hex())

	// Wait for the transaction to be included in a block
	for {
		receipt, err := t.rawClient.TransactionReceipt(ctx, tx.Hash())
		if receipt != nil {
			log.Info().Msgf("Transaction included in block %s, hash: %s", receipt.BlockNumber, receipt.TxHash.Hex())
			break
		}
		if err != nil && err.Error() != "not found" {
			log.Fatal().Err(err).Msg("failed to get transaction receipt")
		}
		time.Sleep(5 * time.Second) // Polling interval
	}
	return nil
}
