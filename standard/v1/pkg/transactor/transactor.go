package transactor

import (
	"standard-bridge/pkg/listener"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	l1g "github.com/primevprotocol/contracts-abi/clients/L1Gateway"
	sg "github.com/primevprotocol/contracts-abi/clients/SettlementGateway"
	"github.com/rs/zerolog/log"
)

type Transactor struct {
	SettlementTransactor *sg.SettlementgatewayTransactor
	L1Transactor         *l1g.L1gatewayTransactor
	EventChan            <-chan listener.TransferInitiatedEvent
}

func NewTransactor(
	settlementAddr common.Address,
	settlementClient *ethclient.Client,
	l1Addr common.Address,
	l1Client *ethclient.Client,
	eventChan <-chan listener.TransferInitiatedEvent,
) *Transactor {
	st, err := sg.NewSettlementgatewayTransactor(settlementAddr, settlementClient)
	if err != nil {
		log.Fatal().Msg("failed to create settlement gateway transactor")
	}
	l1t, err := l1g.NewL1gatewayTransactor(l1Addr, l1Client)
	if err != nil {
		log.Fatal().Msg("failed to create L1 gateway transactor")
	}
	return &Transactor{
		SettlementTransactor: st,
		L1Transactor:         l1t,
		EventChan:            eventChan,
	}
}

func (t *Transactor) Start() {
	go func() {
		for event := range t.EventChan {
			log.Info().Msgf("Received event at Transactor!%+v", event)
		}
	}()
}
