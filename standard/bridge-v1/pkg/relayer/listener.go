package relayer

import (
	"context"
	"fmt"
	"standard-bridge/pkg/shared"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
)

type Listener struct {
	rawClient       *ethclient.Client
	gatewayFilterer shared.GatewayFilterer
	sync            bool
	chain           shared.Chain
	DoneChan        chan struct{}
	EventChan       chan shared.TransferInitiatedEvent
}

func NewListener(
	client *ethclient.Client,
	gatewayFilterer shared.GatewayFilterer,
	sync bool,
) *Listener {
	return &Listener{
		rawClient:       client,
		gatewayFilterer: gatewayFilterer,
		sync:            true,
	}
}

func (listener *Listener) Start(ctx context.Context) (
	<-chan struct{}, <-chan shared.TransferInitiatedEvent, error,
) {
	chainID, err := listener.rawClient.ChainID(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get chain id: %w", err)
	}
	switch chainID.String() {
	case "39999":
		log.Info().Msg("Starting listener for local_l1")
		listener.chain = shared.L1
	case "17864":
		log.Info().Msg("Starting listener for mev-commit chain (settlement)")
		listener.chain = shared.Settlement
	default:
		return nil, nil, fmt.Errorf("unsupported chain id: %s", chainID.String())
	}

	listener.DoneChan = make(chan struct{})
	listener.EventChan = make(chan shared.TransferInitiatedEvent, 10) // Buffer up to 10 events

	go func() {
		defer close(listener.DoneChan)
		defer close(listener.EventChan)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// Blocks up to this value have been handled
		blockNumHandled := uint64(0)

		if listener.sync {
			blockNumHandled, err = listener.obtainBlockNum(ctx)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to obtain block number during sync")
			}
			// Fetch events up to the current block and handle them
			opts := &bind.FilterOpts{Start: 0, End: &blockNumHandled, Context: ctx}
			events, err := listener.gatewayFilterer.ObtainTransferInitiatedEvents(opts)
			if err != nil {
				log.Fatal().Err(err).Msg("listener failed to fetch transfer initiated events during sync")
			}

			for _, event := range events {
				log.Info().Msgf("Transfer initiated event seen by listener during sync: %+v", event)
				listener.EventChan <- event
			}
		}

		for {
			select {
			case <-ctx.Done():
				log.Info().Msgf("Listener for %s shutting down", listener.chain)
				return
			case <-ticker.C:
			}

			currentBlockNum, err := listener.obtainBlockNum(ctx)
			if err != nil {
				// TODO: Secondary url if rpc fails. For now just start over...
				log.Error().Err(err).Msg("failed to obtain block number. Listener restarting from block 0...")
				blockNumHandled = 0
				continue
			}
			if blockNumHandled < currentBlockNum {
				opts := &bind.FilterOpts{Start: blockNumHandled + 1, End: &currentBlockNum, Context: ctx}
				events, err := listener.gatewayFilterer.ObtainTransferInitiatedEvents(opts)
				if err != nil {
					// TODO: Secondary url if rpc fails. For now just start over...
					log.Error().Err(err).Msgf("failed to fetch transfer initiated events from block %d to %d on %s",
						blockNumHandled+1, currentBlockNum, listener.chain.String())
					log.Error().Msg("Listener restarting from block 0...")
					blockNumHandled = 0
					continue
				}
				log.Debug().Msgf("Fetched %d events from block %d to %d on %s",
					len(events), blockNumHandled+1, currentBlockNum, listener.chain.String())
				for _, event := range events {
					log.Info().Msgf("Transfer initiated event seen by listener: %+v", event)
					listener.EventChan <- event
				}
				blockNumHandled = currentBlockNum
			}
		}
	}()
	return listener.DoneChan, listener.EventChan, nil
}

func (listener *Listener) obtainBlockNum(ctx context.Context) (uint64, error) {
	blockNum, err := listener.rawClient.BlockNumber(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to obtain block number: %w", err)
	}
	return blockNum, nil
}
