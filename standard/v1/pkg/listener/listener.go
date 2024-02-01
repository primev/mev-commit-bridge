package listener

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	l1g "github.com/primevprotocol/contracts-abi/clients/L1Gateway"
	sg "github.com/primevprotocol/contracts-abi/clients/SettlementGateway"
	"github.com/rs/zerolog/log"
)

type GatewayListener struct {
	sGatewayCaller    *sg.SettlementgatewayCaller
	sGatewayFilterer  *sg.SettlementgatewayFilterer
	l1GatewayCaller   *l1g.L1gatewayCaller
	l1GatewayFilterer *l1g.L1gatewayFilterer
}

func NewGatewayListener(
	sGatewayCaller *sg.SettlementgatewayCaller,
	sGatewayFilterer *sg.SettlementgatewayFilterer,
	l1GatewayCaller *l1g.L1gatewayCaller,
	l1GatewayFilterer *l1g.L1gatewayFilterer,
) *GatewayListener {
	return &GatewayListener{
		sGatewayCaller:    sGatewayCaller,
		sGatewayFilterer:  sGatewayFilterer,
		l1GatewayCaller:   l1GatewayCaller,
		l1GatewayFilterer: l1GatewayFilterer,
	}

}

// method for start
func (listener *GatewayListener) Start(ctx context.Context) <-chan struct{} {

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
				owner, err := listener.sGatewayCaller.Owner(&bind.CallOpts{Context: ctx})
				if err != nil {
					log.Fatal().Err(err).Msg("failed to get owner")
				}
				log.Info().Str("owner", owner.String()).Msg("l1 owner")
				opts := &bind.FilterOpts{
					Start:   0,
					End:     nil,
					Context: ctx,
				}
				l1Iter, err := listener.l1GatewayFilterer.FilterTransferInitiated(opts, nil, nil)
				if err != nil {
					log.Fatal().Err(err).Msg("failed to filter transfer initiated")
				}
				idx := 0
				for l1Iter.Next() {
					log.Info().Str("sender", l1Iter.Event.Sender.String()).
						Str("recipient", l1Iter.Event.Recipient.String()).
						Str("amount", l1Iter.Event.Amount.String()).
						Msg("transfer initiated on l1")
					idx++
				}
				log.Debug().Int("count", idx).Msg("transfer initiated l1 count")

				sIter, err := listener.sGatewayFilterer.FilterTransferInitiated(opts, nil, nil)
				if err != nil {
					log.Fatal().Err(err).Msg("failed to filter transfer initiated")
				}
				idx = 0
				for sIter.Next() {
					log.Info().Str("sender", sIter.Event.Sender.String()).
						Str("recipient", sIter.Event.Recipient.String()).
						Str("amount", sIter.Event.Amount.String()).
						Msg("transfer initiated on settlement")
					idx++
				}
				log.Debug().Int("count", idx).Msg("transfer initiated settlement count")
			}
		}
	}()
	return doneChan
}
