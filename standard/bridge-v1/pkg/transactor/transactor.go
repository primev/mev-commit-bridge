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

type Transactor struct {
	privateKey        *ecdsa.PrivateKey
	rawClient         *ethclient.Client
	gatewayTransactor gatewayTransactor
	gatewayFilterer   gatewayFilterer
	chainID           *big.Int
	chain             listener.Chain
	eventChan         <-chan listener.TransferInitiatedEvent
}

type gatewayTransactor interface {
	FinalizeTransfer(opts *bind.TransactOpts, _recipient common.Address,
		_amount *big.Int, _counterpartyIdx *big.Int) (*types.Transaction, error)
}

type gatewayFilterer interface {
	ObtainTransferFinalizedEvent(opts *bind.FilterOpts, counterpartyIdx *big.Int) (
		listener.TransferFinalizedEvent, bool, error)
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
	switch t.chainID.String() {
	case "39999":
		log.Info().Msg("Starting transactor for local_l1")
		t.chain = listener.L1
	case "17864":
		log.Info().Msg("Starting transactor for mev-commit chain (settlement)")
		t.chain = listener.Settlement
	default:
		log.Fatal().Msgf("Unsupported chain id: %s", t.chainID.String())
	}

	doneChan := make(chan struct{})

	go func() {
		defer close(doneChan)

		for event := range t.eventChan {
			log.Debug().Msgf("Received signal from listener to submit transfer finalization tx on dest chain: %s. "+
				"Where Src chain: %s, recipient: %s, amount: %d, srcTransferIdx: %d",
				t.chain, event.Chain.String(), event.Recipient, event.Amount, event.TransferIdx)
			opts := t.mustGetTransactOpts(ctx, t.chainID)
			if t.transferAlreadyFinalized(ctx, event.TransferIdx) {
				continue
			}
			t.mustSendFinalizeTransfer(ctx, opts, event)
		}
		log.Info().Msgf("Chan to transactor was closed, transactor for chain %s is exiting", t.chain)
	}()
	return doneChan
}

func (s *Transactor) mustGetTransactOpts(ctx context.Context, chainID *big.Int) *bind.TransactOpts {
	auth, err := bind.NewKeyedTransactorWithChainID(s.privateKey, chainID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get keyed transactor")
	}
	nonce, err := s.rawClient.PendingNonceAt(ctx, auth.From)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get pending nonce")
	}
	auth.Nonce = big.NewInt(int64(nonce))

	// Returns priority fee per gas
	gasTip, err := s.rawClient.SuggestGasTipCap(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get gas tip cap")
	}

	// Returns priority fee per gas + base fee per gas
	gasPrice, err := s.rawClient.SuggestGasPrice(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get gas price")
	}

	auth.GasFeeCap = gasPrice
	auth.GasTipCap = gasTip
	auth.GasLimit = uint64(3000000)

	return auth
}

func (t *Transactor) transferAlreadyFinalized(ctx context.Context, transferIdx *big.Int) bool {
	opts := &bind.FilterOpts{
		Start: 0,
		End:   nil,
	}
	event, found, err := t.gatewayFilterer.ObtainTransferFinalizedEvent(opts, transferIdx)
	if err != nil {
		log.Fatal().Err(err).Msg("error obtaining transfer finalized event")
	}

	if found {
		log.Info().Msgf("Transfer already finalized on dest chain: %s, recipient: %s, amount: %d, srcTransferIdx: %d",
			t.chain.String(), event.Recipient, event.Amount, event.CounterpartyIdx)
		return true
	}
	return false
}

func (t *Transactor) mustSendFinalizeTransfer(ctx context.Context, opts *bind.TransactOpts, event listener.TransferInitiatedEvent) {
	tx, err := t.gatewayTransactor.FinalizeTransfer(opts,
		event.Recipient,
		event.Amount,
		event.TransferIdx,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to send finalize transfer tx")
	}
	log.Debug().Msgf("Transfer finalization tx sent, hash: %s, destChain: %s, recipient: %s, amount: %d, srcTransferIdx: %d",
		tx.Hash().Hex(), t.chain.String(), event.Recipient, event.Amount, event.TransferIdx)

	// Wait for the transaction to be included in a block, with a timeout
	idx := 0
	timeoutCount := 20
	for {
		if idx >= timeoutCount {
			log.Fatal().Msgf("Transfer finalization tx not included in block after %d attempts", timeoutCount)
		}
		receipt, err := t.rawClient.TransactionReceipt(ctx, tx.Hash())
		if receipt != nil {
			log.Info().Msgf("Transfer finalization tx included in block %s, hash: %s",
				receipt.BlockNumber, receipt.TxHash.Hex())
			break
		}
		if err != nil && err.Error() != "not found" {
			log.Fatal().Err(err).Msg("failed to get transaction receipt")
		}
		idx++
		time.Sleep(5 * time.Second)
	}
}
