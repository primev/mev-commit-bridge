package listener

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	sg "github.com/primevprotocol/contracts-abi/clients/SettlementGateway"
	"github.com/rs/zerolog/log"
)

type SettlementFilterer struct {
	*sg.SettlementgatewayFilterer
}

func NewSettlementFilterer(
	gatewayAddr common.Address,
	client *ethclient.Client,
) *SettlementFilterer {
	f, err := sg.NewSettlementgatewayFilterer(gatewayAddr, client)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create settlement gateway filterer")
	}
	return &SettlementFilterer{f}
}

func (f *SettlementFilterer) MustObtainTransferInitiatedBySender(opts *bind.FilterOpts, sender common.Address) TransferInitiatedEvent {
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
		Chain:       Settlement,
	}
}

func (f *SettlementFilterer) ObtainTransferInitiatedEvents(opts *bind.FilterOpts) []TransferInitiatedEvent {
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
			Chain:       Settlement,
		})
	}
	return toReturn
}

func (f *SettlementFilterer) ObtainTransferFinalizedEvent(opts *bind.FilterOpts, counterpartyIdx *big.Int) (TransferFinalizedEvent, bool) {
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
			Chain:           Settlement,
		})
	}
	if len(events) == 0 {
		return TransferFinalizedEvent{}, false
	}
	return events[0], true
}
