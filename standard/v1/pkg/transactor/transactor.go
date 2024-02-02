package transactor

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"standard-bridge/pkg/listener"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	l1g "github.com/primevprotocol/contracts-abi/clients/L1Gateway"
	sg "github.com/primevprotocol/contracts-abi/clients/SettlementGateway"
	"github.com/rs/zerolog/log"
)

type Transactor struct {
	privateKey           *ecdsa.PrivateKey
	settlementRawClient  *ethclient.Client
	settlementTransactor *sg.SettlementgatewayTransactor
	settlementChainID    *big.Int
	l1RawClient          *ethclient.Client
	l1Transactor         *l1g.L1gatewayTransactor
	l1ChainID            *big.Int
	eventChan            <-chan listener.TransferInitiatedEvent
}

func NewTransactor(
	pk *ecdsa.PrivateKey,
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
		privateKey:           pk,
		settlementRawClient:  settlementClient,
		settlementTransactor: st,
		l1RawClient:          l1Client,
		l1Transactor:         l1t,
		eventChan:            eventChan,
	}
}

func (t *Transactor) Start(ctx context.Context) {

	var err error
	t.settlementChainID, err = t.settlementRawClient.ChainID(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get settlement chain id")
	}
	t.l1ChainID, err = t.l1RawClient.ChainID(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get l1 chain id")
	}

	go func() {
		for event := range t.eventChan {

			log.Info().Msgf("Received event at Transactor!%+v", event)

			opts, err := t.getTransactOpts(ctx, t.settlementChainID)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to get transact opts")
			}
			log.Info().Msgf("opts: %+v", opts)
			tx, err := t.settlementTransactor.FinalizeTransfer(opts,
				common.HexToAddress(event.Recipient),
				big.NewInt(int64(event.Amount)),
				big.NewInt(int64(event.TransferIdx)),
			)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to finalize transfer")
			}
			log.Info().Msgf("tx: %+v", tx)
			// hash
			log.Info().Msgf("tx hash: %+v", tx.Hash().Hex())

			// for 10 iterations
			for i := 0; i < 10; i++ {
				recpt, err := t.settlementRawClient.TransactionReceipt(ctx, tx.Hash())
				if err != nil {
					log.Error().Err(err).Msg("failed to get transaction receipt")
				}
				log.Info().Msgf("recpt: %+v", recpt)
				// sleep 5
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

// Adaptation of func from oracle repo
func (s *Transactor) getTransactOpts(ctx context.Context, chainID *big.Int) (*bind.TransactOpts, error) {
	auth, err := bind.NewKeyedTransactorWithChainID(s.privateKey, chainID)
	if err != nil {
		return nil, err
	}
	nonce, err := s.settlementRawClient.PendingNonceAt(ctx, auth.From)
	if err != nil {
		return nil, err
	}
	auth.Nonce = big.NewInt(int64(nonce))

	gasTip, err := s.settlementRawClient.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, err
	}

	gasPrice, err := s.settlementRawClient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, err
	}

	gasFeeCap := new(big.Int).Add(gasTip, gasPrice)
	auth.GasFeeCap = gasFeeCap
	auth.GasTipCap = gasTip
	auth.GasLimit = uint64(3000000)

	return auth, nil
}
