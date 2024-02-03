package listener

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
)

// TODO: unit tests

type Listener struct {
	rawClient       *ethclient.Client
	gatewayFilterer GatewayFilterer
	sync            bool
	chain           Chain
	DoneChan        chan struct{}
	EventChan       chan TransferInitiatedEvent
}

type GatewayFilterer interface {
	ObtainTransferInitiatedEvents(opts *bind.FilterOpts) []TransferInitiatedEvent
}

func NewListener(
	client *ethclient.Client,
	gatewayFilterer GatewayFilterer,
	sync bool,
) *Listener {
	return &Listener{
		rawClient:       client,
		gatewayFilterer: gatewayFilterer,
		sync:            true,
	}
}

func (listener *Listener) Start(ctx context.Context) (
	<-chan struct{}, <-chan TransferInitiatedEvent,
) {
	chainID, err := listener.rawClient.ChainID(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get chain id")
	}
	switch chainID.String() {
	case "39999":
		log.Info().Msg("Starting listener for local_l1")
		listener.chain = L1
	case "17864":
		log.Info().Msg("Starting listener for mev-commit chain (settlement)")
		listener.chain = Settlement
	default:
		log.Fatal().Msgf("Unsupported chain id: %s", chainID.String())
	}

	listener.DoneChan = make(chan struct{})
	listener.EventChan = make(chan TransferInitiatedEvent, 10) // Buffer up to 10 events

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
			opts := listener.GetFilterOpts(ctx, 0, blockNumHandled)
			events := listener.gatewayFilterer.ObtainTransferInitiatedEvents(opts)
			for _, event := range events {
				log.Info().Msgf("Transfer initiated event seen by listener: %+v", event)
				listener.EventChan <- event
			}
		}

		for {
			select {
			case <-ctx.Done():
				log.Debug().Msg("stopping listener")
				return
			case <-ticker.C:
			}

			currentBlockNum := listener.mustGetBlockNum(ctx)
			if blockNumHandled < currentBlockNum {
				opts := listener.GetFilterOpts(ctx, blockNumHandled+1, currentBlockNum)
				events := listener.gatewayFilterer.ObtainTransferInitiatedEvents(opts)
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

// GetFilterOpts returns the filter options for the listener, end is inclusive
func (listener *Listener) GetFilterOpts(ctx context.Context, start uint64, end uint64) *bind.FilterOpts {
	return &bind.FilterOpts{
		Start:   start,
		End:     nil,
		Context: ctx,
	}
}
