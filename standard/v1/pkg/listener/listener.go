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
	listener.DoneChan = make(chan struct{})
	// Buffer up to 10 events
	listener.EventChan = make(chan TransferInitiatedEvent, 10)

	go func() {
		defer close(listener.DoneChan)
		defer close(listener.EventChan)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		log.Info().Msg("starting listener")

		// Blocks up to this value have been handled
		blockNumHandled := uint64(0)

		if listener.sync {
			blockNumHandled = listener.mustGetBlockNum(ctx)
			// Fetch events up to the current block and handle them
			opts := listener.GetFilterOpts(ctx, 0, blockNumHandled)
			events := listener.gatewayFilterer.ObtainTransferInitiatedEvents(opts)
			for _, event := range events {
				log.Info().Msgf("Received event at Listener!%+v", event)
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
				log.Debug().Uint64("start", blockNumHandled+1).Uint64("end", currentBlockNum).Msg("block numbers")
				events := listener.gatewayFilterer.ObtainTransferInitiatedEvents(opts)
				for _, event := range events {
					log.Info().Msgf("Received event at Listener!%+v", event)
					listener.EventChan <- event
				}
				// Update blockNumHandled to reflect the last block we have processed
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
