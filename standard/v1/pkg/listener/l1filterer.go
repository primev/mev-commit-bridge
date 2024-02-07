package listener

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	l1g "github.com/primevprotocol/contracts-abi/clients/L1Gateway"
	"github.com/rs/zerolog/log"
)

type L1Filterer struct {
	*l1g.L1gatewayFilterer
}

func NewL1Filterer(
	gatewayAddr common.Address,
	client *ethclient.Client,
) *L1Filterer {
	f, err := l1g.NewL1gatewayFilterer(gatewayAddr, client)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create settlement gateway filterer")
	}
	return &L1Filterer{f}
}

func (f *L1Filterer) MustObtainTransferInitiatedBySender(opts *bind.FilterOpts, sender common.Address) TransferInitiatedEvent {
	iter, err := f.FilterTransferInitiated(opts, []common.Address{sender}, nil, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to filter transfer initiated")
	}
	if !iter.Next() {
		log.Fatal().Msg("failed to obtain single transfer initiated event with sender: " + sender.String())
	}
	return TransferInitiatedEvent{
		Sender:      iter.Event.Sender,
		Recipient:   iter.Event.Recipient,
		Amount:      iter.Event.Amount,
		TransferIdx: iter.Event.TransferIdx,
		Chain:       L1,
	}
}

func (f *L1Filterer) ObtainTransferInitiatedEvents(opts *bind.FilterOpts) []TransferInitiatedEvent {
	iter, err := f.FilterTransferInitiated(opts, nil, nil, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to filter transfer initiated")
	}
	toReturn := make([]TransferInitiatedEvent, 0)
	for iter.Next() {
		toReturn = append(toReturn, TransferInitiatedEvent{
			Sender:      iter.Event.Sender,
			Recipient:   iter.Event.Recipient,
			Amount:      iter.Event.Amount,
			TransferIdx: iter.Event.TransferIdx,
			Chain:       L1,
		})
	}
	return toReturn
}

func (f *L1Filterer) ObtainTransferFinalizedEvent(opts *bind.FilterOpts, counterpartyIdx *big.Int) (TransferFinalizedEvent, bool) {
	iter, err := f.FilterTransferFinalized(opts, nil, []*big.Int{counterpartyIdx})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to filter transfer finalized")
	}
	events := make([]TransferFinalizedEvent, 0)
	for iter.Next() {
		events = append(events, TransferFinalizedEvent{
			Recipient:       iter.Event.Recipient,
			Amount:          iter.Event.Amount,
			CounterpartyIdx: iter.Event.CounterpartyIdx,
			Chain:           L1,
		})
	}
	if len(events) == 0 {
		return TransferFinalizedEvent{}, false
	}
	return events[0], true
}
