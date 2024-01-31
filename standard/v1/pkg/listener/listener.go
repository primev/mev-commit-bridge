package listener

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"

	l1gateway "github.com/primevprotocol/contracts-abi/clients/L1Gateway"
	settlementgateway "github.com/primevprotocol/contracts-abi/clients/SettlementGateway"
)

type Listener struct {
	settlementClient        *ethclient.Client
	l1Client                *ethclient.Client
	settlementGatewayCaller *settlementgateway.SettlementgatewayCaller
	l1GatewayCaller         *l1gateway.L1gatewayCaller
}

func NewListener(settlementClient *ethclient.Client,
	settlementGatewayAddr common.Address,
	l1Client *ethclient.Client,
	l1GatewayAddr common.Address,
) *Listener {
	sGatewayCaller, err := settlementgateway.NewSettlementgatewayCaller(
		settlementGatewayAddr, settlementClient)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create settlement gateway caller")
	}
	l1GatewayCaller, err := l1gateway.NewL1gatewayCaller(
		l1GatewayAddr, l1Client)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create L1 gateway caller")
	}
	return &Listener{
		settlementClient:        settlementClient,
		l1Client:                l1Client,
		settlementGatewayCaller: sGatewayCaller,
		l1GatewayCaller:         l1GatewayCaller,
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
				owner, err := s.settlementGatewayCaller.Owner(nil)
				if err != nil {
					log.Error().Err(err).Msg("failed to get owner")
				}
				log.Info().Str("owner", owner.String()).Msg("owner")
				owner, err = s.l1GatewayCaller.Owner(nil)
				if err != nil {
					log.Error().Err(err).Msg("failed to get owner")
				}
				log.Info().Str("owner", owner.String()).Msg("owner")
			}
		}
	}()
	return doneChan
}
