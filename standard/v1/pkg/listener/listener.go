package listener

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
)

// See https://goethereumbook.org/event-read/

type Listener struct {
	client *ethclient.Client
}

func NewListener(client *ethclient.Client) *Listener {
	return &Listener{
		client: client,
	}
}

func (s *Listener) Start(ctx context.Context) <-chan struct{} {

	doneChan := make(chan struct{})

	go func() {
		defer close(doneChan)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		log.Debug().Msg("starting listener")
		for {
			select {
			case <-ctx.Done():
				log.Debug().Msg("stopping listener")
				return
			case <-ticker.C:
				log.Info().Msg("ticking listener")
			}
		}
	}()
	return doneChan
}
