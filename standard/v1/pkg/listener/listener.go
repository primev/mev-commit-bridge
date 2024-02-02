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

// TODO: unit tests

type Listener struct {
	sRawClient        *ethclient.Client
	sGatewayCaller    *sg.SettlementgatewayCaller
	sGatewayFilterer  *sg.SettlementgatewayFilterer
	l1RawClient       *ethclient.Client
	l1GatewayCaller   *l1g.L1gatewayCaller
	l1GatewayFilterer *l1g.L1gatewayFilterer
	sync              bool
	DoneChan          chan struct{}
	EventChan         chan TransferInitiatedEvent
}

func NewListener(
	settlementAddr common.Address,
	settlementClient *ethclient.Client,
	l1Addr common.Address,
	l1Client *ethclient.Client,
) *Listener {
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
	return &Listener{
		sRawClient:        settlementClient,
		sGatewayCaller:    sGatewayCaller,
		sGatewayFilterer:  sGatewayFilterer,
		l1RawClient:       l1Client,
		l1GatewayCaller:   l1GatewayCaller,
		l1GatewayFilterer: l1GatewayFilterer,
		sync:              true,
	}
}

type TransferInitiatedEvent struct {
	Sender      string
	Recipient   string
	Amount      uint64
	TransferIdx uint64
	srcChain    srcChain
}

type srcChain int

const (
	settlement srcChain = iota
	l1
)

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

		log.Debug().Msg("starting listener")

		// Blocks up to this value have been handled
		settlementBlockNum := uint64(0)
		l1BlockNum := uint64(0)

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
	return listener.DoneChan, listener.EventChan
}

func (listener *Listener) GetSettlementBlockNum(ctx context.Context) uint64 {
	settlementBlockNum, err := listener.sRawClient.BlockNumber(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get settlement block number")
	}
	return settlementBlockNum
}

func (listener *Listener) GetL1BlockNum(ctx context.Context) uint64 {
	l1BlockNum, err := listener.l1RawClient.BlockNumber(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get l1 block number")
	}
	return l1BlockNum
}

func (listener *Listener) GetFilterOpts(ctx context.Context, start uint64, end uint64) *bind.FilterOpts {
	return &bind.FilterOpts{
		Start:   start, // TODO: Confirm no off-by-one error
		End:     nil,
		Context: ctx,
	}
}

func (listener *Listener) HandleSettlementEvents(ctx context.Context, start uint64, end uint64) {
	opts := listener.GetFilterOpts(ctx, start, end)
	sIter, err := listener.sGatewayFilterer.FilterTransferInitiated(opts, nil, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to filter transfer initiated")
	}
	for sIter.Next() {
		log.Info().Str("sender", sIter.Event.Sender.String()).
			Str("recipient", sIter.Event.Recipient.String()).
			Str("amount", sIter.Event.Amount.String()).
			Msg("transfer initiated on settlement")
		listener.EventChan <- TransferInitiatedEvent{
			Sender:      sIter.Event.Sender.String(),
			Recipient:   sIter.Event.Recipient.String(),
			Amount:      sIter.Event.Amount.Uint64(),
			TransferIdx: sIter.Event.TransferIdx.Uint64(),
			srcChain:    settlement,
		}
	}
}

func (listener *Listener) HandleL1Events(ctx context.Context, start uint64, end uint64) {
	opts := listener.GetFilterOpts(ctx, start, end)
	l1Iter, err := listener.l1GatewayFilterer.FilterTransferInitiated(opts, nil, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to filter transfer initiated")
	}
	for l1Iter.Next() {
		log.Info().Str("sender", l1Iter.Event.Sender.String()).
			Str("recipient", l1Iter.Event.Recipient.String()).
			Str("amount", l1Iter.Event.Amount.String()).
			Msg("transfer initiated on L1")
		listener.EventChan <- TransferInitiatedEvent{
			Sender:      l1Iter.Event.Sender.String(),
			Recipient:   l1Iter.Event.Recipient.String(),
			Amount:      l1Iter.Event.Amount.Uint64(),
			TransferIdx: l1Iter.Event.TransferIdx.Uint64(),
			srcChain:    l1,
		}
	}
}
