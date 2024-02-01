package listener

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	l1g "github.com/primevprotocol/contracts-abi/clients/L1Gateway"
	sg "github.com/primevprotocol/contracts-abi/clients/SettlementGateway"
	"github.com/rs/zerolog/log"
)

type GatewayListener struct {
	sRawClient        *ethclient.Client
	sGatewayCaller    *sg.SettlementgatewayCaller
	sGatewayFilterer  *sg.SettlementgatewayFilterer
	l1RawClient       *ethclient.Client
	l1GatewayCaller   *l1g.L1gatewayCaller
	l1GatewayFilterer *l1g.L1gatewayFilterer
	sync              bool
}

func NewGatewayListener(
	settlementAddr common.Address,
	settlementClient *ethclient.Client,
	l1Addr common.Address,
	l1Client *ethclient.Client,
) *GatewayListener {
	sGatewayCaller, err := sg.NewSettlementgatewayCaller(settlementAddr, settlementClient)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create settlement gateway caller")
	}
	sGatewayFilterer, err := sg.NewSettlementgatewayFilterer(settlementAddr, settlementClient)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create settlement gateway filterer")
	}
	l1GatewayCaller, err := l1g.NewL1gatewayCaller(l1Addr, l1Client)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create L1 gateway caller")
	}
	l1GatewayFilterer, err := l1g.NewL1gatewayFilterer(l1Addr, l1Client)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create L1 gateway filterer")
	}
	return &GatewayListener{
		sRawClient:        settlementClient,
		sGatewayCaller:    sGatewayCaller,
		sGatewayFilterer:  sGatewayFilterer,
		l1RawClient:       l1Client,
		l1GatewayCaller:   l1GatewayCaller,
		l1GatewayFilterer: l1GatewayFilterer,
		sync:              true,
	}
}

func (listener *GatewayListener) Start(ctx context.Context) <-chan struct{} {

	doneChan := make(chan struct{})

	go func() {
		defer close(doneChan)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		log.Debug().Msg("starting listener")

		// Blocks up to this value have been handled
		var settlementBlockNum uint64
		var l1BlockNum uint64

		if listener.sync {
			settlementBlockNum = listener.GetSettlementBlockNum(ctx)
			listener.HandleSettlementEvents(ctx, 0, settlementBlockNum)
			l1BlockNum = listener.GetL1BlockNum(ctx)
			listener.HandleL1Events(ctx, 0, l1BlockNum)
		}

		for {
			select {
			case <-ctx.Done():
				log.Debug().Msg("stopping listener")
				return
			case <-ticker.C:
			}

			//
			// Settlement
			//
			newNum := listener.GetSettlementBlockNum(ctx)
			log.Debug().Uint64("settlementBlockNum", settlementBlockNum).Msg("settlement block num")
			listener.HandleSettlementEvents(ctx, settlementBlockNum, newNum)
			settlementBlockNum = newNum

			//
			// L1
			//
			newNum = listener.GetL1BlockNum(ctx)
			log.Debug().Uint64("l1BlockNum", l1BlockNum).Msg("l1 block num")
			listener.HandleL1Events(ctx, l1BlockNum, newNum)
			l1BlockNum = newNum
		}
	}()
	return doneChan
}

func (listener *GatewayListener) GetSettlementBlockNum(ctx context.Context) uint64 {
	settlementBlockNum, err := listener.sRawClient.BlockNumber(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get settlement block number")
	}
	return settlementBlockNum
}

func (listener *GatewayListener) GetL1BlockNum(ctx context.Context) uint64 {
	l1BlockNum, err := listener.l1RawClient.BlockNumber(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get l1 block number")
	}
	return l1BlockNum
}

func (listener *GatewayListener) GetFilterOpts(ctx context.Context, start uint64, end uint64) *bind.FilterOpts {
	return &bind.FilterOpts{
		Start:   start, // TODO: Confirm no off-by-one error
		End:     nil,
		Context: ctx,
	}
}

func (listener *GatewayListener) HandleSettlementEvents(ctx context.Context, start uint64, end uint64) {
	opts := listener.GetFilterOpts(ctx, start, end)
	sIter, err := listener.sGatewayFilterer.FilterTransferInitiated(opts, nil, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to filter transfer initiated")
	}
	idx := 0
	for sIter.Next() {
		log.Info().Str("sender", sIter.Event.Sender.String()).
			Str("recipient", sIter.Event.Recipient.String()).
			Str("amount", sIter.Event.Amount.String()).
			Msg("transfer initiated on settlement")
		idx++
	}
	log.Debug().Int("count", idx).Msg("count of transfers initiated on settlement this cycle")
}

func (listener *GatewayListener) HandleL1Events(ctx context.Context, start uint64, end uint64) {
	opts := listener.GetFilterOpts(ctx, start, end)
	l1Iter, err := listener.l1GatewayFilterer.FilterTransferInitiated(opts, nil, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to filter transfer initiated")
	}
	idx := 0
	for l1Iter.Next() {
		log.Info().Str("sender", l1Iter.Event.Sender.String()).
			Str("recipient", l1Iter.Event.Recipient.String()).
			Str("amount", l1Iter.Event.Amount.String()).
			Msg("transfer initiated on L1")
		idx++
	}
	log.Debug().Int("count", idx).Msg("count of transfers initiated on L1 this cycle")
}
