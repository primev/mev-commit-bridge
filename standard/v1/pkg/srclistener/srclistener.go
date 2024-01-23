package srclistener

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
)

// See https://goethereumbook.org/event-read/

type SrcListener struct {
	client *ethclient.Client
}

func NewSrcListener(client *ethclient.Client) *SrcListener {
	return &SrcListener{
		client: client,
	}
}

func (s *SrcListener) Start(ctx context.Context) <-chan struct{} {

	doneChan := make(chan struct{})
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		defer close(doneChan)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fmt.Println("src listener running...")
			}
		}
	}()
	return doneChan
}
