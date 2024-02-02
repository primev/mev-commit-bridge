package listener

import (
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

func (f *L1Filterer) ObtainTransferInitiatedEvents(opts *bind.FilterOpts) []TransferInitiatedEvent {
	iter, err := f.FilterTransferInitiated(opts, nil, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to filter transfer initiated")
	}
	toReturn := make([]TransferInitiatedEvent, 0)
	for iter.Next() {
		toReturn = append(toReturn, TransferInitiatedEvent{
			Sender:      iter.Event.Sender.String(),
			Recipient:   iter.Event.Recipient.String(),
			Amount:      iter.Event.Amount.Uint64(),
			TransferIdx: iter.Event.TransferIdx.Uint64(),
			srcChain:    l1,
		})
	}
	return toReturn
}
