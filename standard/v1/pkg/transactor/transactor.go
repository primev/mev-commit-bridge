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

// TODO: Have listener as a part of tx process that monitors for finalized transfers
// and doesn't double-count.

type Transactor struct {
	privateKey        *ecdsa.PrivateKey
	rawClient         *ethclient.Client
	gatewayTransactor gatewayTransactor
	chainID           *big.Int
	eventChan         <-chan listener.TransferInitiatedEvent
}

type gatewayTransactor interface {
	FinalizeTransfer(opts *bind.TransactOpts, _recipient common.Address,
		_amount *big.Int, _counterpartyIdx *big.Int) (*types.Transaction, error)
}

func NewTransactor(
	pk *ecdsa.PrivateKey,
	gatewayAddr common.Address,
	ethClient *ethclient.Client,
	gatewayTransactor gatewayTransactor,
	eventChan <-chan listener.TransferInitiatedEvent,
) *Transactor {
	return &Transactor{
		privateKey:        pk,
		rawClient:         ethClient,
		gatewayTransactor: gatewayTransactor,
		eventChan:         eventChan,
	}
}

func (t *Transactor) Start(ctx context.Context) {

	var err error
	t.chainID, err = t.rawClient.ChainID(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get chain id")
	}

	go func() {
		for event := range t.eventChan {

			log.Info().Msgf("Received event at Transactor!%+v", event)

			opts, err := t.getTransactOpts(ctx, t.chainID)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to get transact opts")
			}
			log.Info().Msgf("opts: %+v", opts)
			tx, err := t.gatewayTransactor.FinalizeTransfer(opts,
				common.HexToAddress(event.Recipient),
				big.NewInt(int64(event.Amount)),
				big.NewInt(int64(event.TransferIdx)),
			)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to finalize transfer")
			}
			log.Info().Msgf("tx: %+v", tx)
			log.Info().Msgf("tx hash: %+v", tx.Hash().Hex())

			// for 10 iterations
			for i := 0; i < 10; i++ {
				recpt, err := t.rawClient.TransactionReceipt(ctx, tx.Hash())
				if err != nil {
					log.Error().Err(err).Msg("failed to get transaction receipt")
				}
				log.Info().Msgf("recpt: %+v", recpt)
				// sleep 5
				time.Sleep(5 * time.Second)
			}
		}
	}()
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
