package relayer

import (
	"context"
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
	<-chan struct{}, <-chan shared.TransferInitiatedEvent,
) {
	chainID, err := listener.rawClient.ChainID(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get chain id")
	}
	switch chainID.String() {
	case "39999":
		log.Info().Msg("Starting listener for local_l1")
		listener.chain = shared.L1
	case "17864":
		log.Info().Msg("Starting listener for mev-commit chain (settlement)")
		listener.chain = shared.Settlement
	default:
		log.Fatal().Msgf("Unsupported chain id: %s", chainID.String())
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
			blockNumHandled = listener.mustGetBlockNum(ctx)
			// Fetch events up to the current block and handle them
			opts := &bind.FilterOpts{Start: 0, End: &blockNumHandled, Context: ctx}
			events, err := listener.gatewayFilterer.ObtainTransferInitiatedEvents(opts)
			if err != nil {
				log.Fatal().Err(err).Msg("error obtaining transfer initiated events")
			}
			for _, event := range events {
				log.Info().Msgf("Transfer initiated event seen by listener: %+v", event)
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

			currentBlockNum := listener.mustGetBlockNum(ctx)
			if blockNumHandled < currentBlockNum {
				opts := &bind.FilterOpts{Start: blockNumHandled + 1, End: &currentBlockNum, Context: ctx}
				events, err := listener.gatewayFilterer.ObtainTransferInitiatedEvents(opts)
				if err != nil {
					log.Fatal().Err(err).Msg("error obtaining transfer initiated events")
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
	return listener.DoneChan, listener.EventChan
}

func (listener *Listener) mustGetBlockNum(ctx context.Context) uint64 {
	blockNum, err := listener.rawClient.BlockNumber(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get settlement block number")
	}
	return blockNum
}
