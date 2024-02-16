package relayer

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"standard-bridge/pkg/shared"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
)

type Transactor struct {
	privateKey        *ecdsa.PrivateKey
	rawClient         *ethclient.Client
	gatewayTransactor shared.GatewayTransactor
	gatewayFilterer   shared.GatewayFilterer
	chainID           *big.Int
	chain             shared.Chain
	eventChan         <-chan shared.TransferInitiatedEvent
	mostRecentFinalized
}

type mostRecentFinalized struct {
	event shared.TransferFinalizedEvent
	opts  bind.FilterOpts
}

func (m mostRecentFinalized) String() string {
	return "Event: " + m.event.String() + ". Opts start: " + fmt.Sprint(m.opts.Start) + ". Opts end: " + fmt.Sprint(*m.opts.End)
}

func NewTransactor(
	pk *ecdsa.PrivateKey,
	gatewayAddr common.Address,
	ethClient *ethclient.Client,
	gatewayTransactor shared.GatewayTransactor,
	gatewayFilterer shared.GatewayFilterer,
	eventChan <-chan shared.TransferInitiatedEvent,
) *Transactor {
	return &Transactor{
		privateKey:        pk,
		rawClient:         ethClient,
		gatewayTransactor: gatewayTransactor,
		gatewayFilterer:   gatewayFilterer,
		eventChan:         eventChan,
		mostRecentFinalized: mostRecentFinalized{
			event: shared.TransferFinalizedEvent{},
			opts:  bind.FilterOpts{Start: 0, End: nil}, // TODO: cache doesn't need to start at 0 once non-syncing relayer is implemented
		},
	}
}

func (t *Transactor) Start(
	ctx context.Context,
) <-chan struct{} {

	var err error
	t.chainID, err = t.rawClient.ChainID(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get chain id")
	}
	switch t.chainID.String() {
	case "39999":
		log.Info().Msg("Starting transactor for local_l1")
		t.chain = shared.L1
	case "17000":
		log.Info().Msg("Starting transactor for Holesky L1")
		t.chain = shared.L1
	case "17864":
		log.Info().Msg("Starting transactor for mev-commit chain (settlement)")
		t.chain = shared.Settlement
	default:
		log.Fatal().Msgf("Unsupported chain id: %s", t.chainID.String())
	}

	doneChan := make(chan struct{})

	go func() {
		defer close(doneChan)

		shared.CancelPendingTxes(ctx, t.privateKey, t.rawClient)

		for event := range t.eventChan {
			log.Debug().Msgf("Received signal from listener to submit transfer finalization tx on dest chain: %s. "+
				"Where Src chain: %s, recipient: %s, amount: %d, srcTransferIdx: %d",
				t.chain, event.Chain.String(), event.Recipient, event.Amount, event.TransferIdx)
			opts, err := shared.CreateTransactOpts(ctx, t.privateKey, t.chainID, t.rawClient)
			if err != nil {
				log.Err(err).Msg("failed to create transact opts for transfer finalization tx. Relayer likely requires restart.")
				log.Warn().Msgf("skipping transfer finalization tx for src transfer idx: %d", event.TransferIdx)
				continue
			}
			finalized, err := t.transferAlreadyFinalized(ctx, event.TransferIdx)
			if err != nil {
				log.Err(err).Msg("failed to check if transfer already finalized. Relayer likely requires restart.")
				log.Warn().Msgf("skipping transfer finalization tx for src transfer idx: %d", event.TransferIdx)
				continue
			}
			if finalized {
				continue
			}
			receipt, err := t.sendFinalizeTransfer(ctx, opts, event)
			if err != nil {
				log.Err(err).Msg("failed to send transfer finalization tx. Relayer likely requires restart.")
				log.Warn().Msgf("skipping transfer finalization tx for src transfer idx: %d", event.TransferIdx)
				continue
			}
			// Event should be obtainable to update cache
			eventBlock := receipt.BlockNumber.Uint64()
			filterOpts := &bind.FilterOpts{Start: eventBlock, End: &eventBlock, Context: ctx}
			_, found, err := t.obtainTransferFinalizedAndUpdateCache(ctx, filterOpts, event.TransferIdx)
			if err != nil {
				log.Err(err).Msg("failed to obtain transfer finalized event after sending tx")
				log.Warn().Msg("TransferFinalized cache will be incorrect. Relayer likely requires restart.")
				continue
			}
			if !found {
				log.Warn().Msg("transfer finalized event not found after sending tx")
				continue
			}
		}
		log.Info().Msgf("Chan to transactor was closed, transactor for chain %s is exiting", t.chain)
	}()
	return doneChan
}

func (t *Transactor) transferAlreadyFinalized(
	ctx context.Context,
	transferIdx *big.Int,
) (bool, error) {
	const maxBlockRange = 40000
	var interEndBlock uint64 // Intermediate end block for each range

	currentBlock, err := t.rawClient.BlockNumber(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get current block number: %w", err)
	}

	startBlock := t.mostRecentFinalized.opts.Start

	for start := startBlock; start <= currentBlock; start = interEndBlock + 1 {
		interEndBlock = start + maxBlockRange
		if interEndBlock > currentBlock {
			interEndBlock = currentBlock
		}
		opts := &bind.FilterOpts{
			Start:   start,
			End:     &interEndBlock,
			Context: ctx,
		}
		event, found, err := t.obtainTransferFinalizedAndUpdateCache(ctx, opts, transferIdx)
		if err != nil {
			return false, fmt.Errorf("failed to obtain transfer finalized event: %w", err)
		}
		if found {
			log.Debug().Msgf("Transfer already finalized on dest chain: %s, recipient: %s, amount: %d, srcTransferIdx: %d",
				t.chain.String(), event.Recipient, event.Amount, event.CounterpartyIdx)
			return true, nil
		}
	}
	return false, nil
}

func (t *Transactor) sendFinalizeTransfer(
	ctx context.Context,
	opts *bind.TransactOpts,
	event shared.TransferInitiatedEvent,
) (*gethtypes.Receipt, error) {

	// Capture event params in closure and define tx submission callback
	submitFinalizeTransfer := func(
		ctx context.Context,
		opts *bind.TransactOpts,
	) (*gethtypes.Transaction, error) {
		tx, err := t.gatewayTransactor.FinalizeTransfer(opts, event.Recipient, event.Amount, event.TransferIdx)
		if err != nil {
			return nil, fmt.Errorf("failed to send finalize transfer tx: %w", err)
		}
		log.Debug().Msgf("Transfer finalization tx sent, hash: %s, destChain: %s, recipient: %s, amount: %d, srcTransferIdx: %d",
			tx.Hash().Hex(), t.chain.String(), event.Recipient, event.Amount, event.TransferIdx)
		return tx, nil
	}

	receipt, err := shared.WaitMinedWithRetry(ctx, t.rawClient, opts, submitFinalizeTransfer)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for finalize transfer tx to be mined: %w", err)
	}
	includedInBlock := receipt.BlockNumber.Uint64()
	log.Info().Msgf("FinalizeTransfer tx included in block: %d on: %v", includedInBlock, t.chain.String())

	return receipt, nil
}

func (t *Transactor) obtainTransferFinalizedAndUpdateCache(
	ctx context.Context,
	opts *bind.FilterOpts,
	transferIdx *big.Int,
) (shared.TransferFinalizedEvent, bool, error) {
	event, found, err := t.gatewayFilterer.ObtainTransferFinalizedEvent(opts, transferIdx)
	if err != nil {
		return shared.TransferFinalizedEvent{}, false, fmt.Errorf("failed to obtain transfer finalized event: %w", err)
	}
	if found {
		t.mostRecentFinalized = mostRecentFinalized{event, *opts}
		log.Debug().Msgf("mostRecentFinalized cache updated: %+v", t.mostRecentFinalized)
	}
	return event, found, nil
}
