package relayer

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"standard-bridge/pkg/shared"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
)

type Transactor struct {
	privateKey        *ecdsa.PrivateKey
	rawClient         *ethclient.Client
	gatewayTransactor shared.GatewayTransactor
	gatewayFilterer   shared.GatewayFilterer
	chainID           *big.Int
	chain             shared.Chain
	eventChan         <-chan shared.TransferInitiatedEvent
}

func NewTransactor(
	pk *ecdsa.PrivateKey,
	gatewayAddr common.Address,
	ethClient *ethclient.Client,
	gatewayTransactor shared.GatewayTransactor,
	gatewayFilterer shared.GatewayFilterer,
	eventChan <-chan shared.TransferInitiatedEvent,
) *Transactor {
	return &Transactor{
		privateKey:        pk,
		rawClient:         ethClient,
		gatewayTransactor: gatewayTransactor,
		gatewayFilterer:   gatewayFilterer,
		eventChan:         eventChan,
	}
}

func (t *Transactor) Start(
	ctx context.Context,
) <-chan struct{} {

	var err error
	t.chainID, err = t.rawClient.ChainID(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get chain id")
	}
	switch t.chainID.String() {
	case "39999":
		log.Info().Msg("Starting transactor for local_l1")
		t.chain = shared.L1
	case "17864":
		log.Info().Msg("Starting transactor for mev-commit chain (settlement)")
		t.chain = shared.Settlement
	default:
		log.Fatal().Msgf("Unsupported chain id: %s", t.chainID.String())
	}

	doneChan := make(chan struct{})

	go func() {
		defer close(doneChan)

		shared.CancelPendingTxes(ctx, t.privateKey, t.rawClient)

		for event := range t.eventChan {
			log.Debug().Msgf("Received signal from listener to submit transfer finalization tx on dest chain: %s. "+
				"Where Src chain: %s, recipient: %s, amount: %d, srcTransferIdx: %d",
				t.chain, event.Chain.String(), event.Recipient, event.Amount, event.TransferIdx)
			opts, err := shared.CreateTransactOpts(ctx, t.privateKey, t.chainID, t.rawClient)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to get transact opts")
			}
			if t.transferAlreadyFinalized(ctx, event.TransferIdx) {
				continue
			}
			t.mustSendFinalizeTransfer(ctx, opts, event)
		}
		log.Info().Msgf("Chan to transactor was closed, transactor for chain %s is exiting", t.chain)
	}()
	return doneChan
}

func (t *Transactor) transferAlreadyFinalized(
	ctx context.Context,
	transferIdx *big.Int,
) bool {
	opts := &bind.FilterOpts{
		Start: 0,
		End:   nil,
	}
	event, found, err := t.gatewayFilterer.ObtainTransferFinalizedEvent(opts, transferIdx)
	if err != nil {
		log.Fatal().Err(err).Msg("error obtaining transfer finalized event")
	}

	if found {
		log.Info().Msgf("Transfer already finalized on dest chain: %s, recipient: %s, amount: %d, srcTransferIdx: %d",
			t.chain.String(), event.Recipient, event.Amount, event.CounterpartyIdx)
		return true
	}
	return false
}

func (t *Transactor) mustSendFinalizeTransfer(
	ctx context.Context,
	opts *bind.TransactOpts,
	event shared.TransferInitiatedEvent,
) {
	tx, err := t.gatewayTransactor.FinalizeTransfer(opts,
		event.Recipient,
		event.Amount,
		event.TransferIdx,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to send finalize transfer tx")
	}
	log.Debug().Msgf("Transfer finalization tx sent, hash: %s, destChain: %s, recipient: %s, amount: %d, srcTransferIdx: %d",
		tx.Hash().Hex(), t.chain.String(), event.Recipient, event.Amount, event.TransferIdx)

	// Wait for the transaction to be included in a block, with a timeout
	// TODO: Use 	"github.com/ethereum/go-ethereum/accounts/abi/bind" waitMined
	// Also implement retries with 10% tip increase

	idx := 0
	timeoutCount := 50
	for {
		if idx >= timeoutCount {
			log.Fatal().Msgf("Transfer finalization tx not included in block after %d attempts", timeoutCount)
		}
		receipt, err := t.rawClient.TransactionReceipt(ctx, tx.Hash())
		if receipt != nil {
			log.Info().Msgf("Transfer finalization tx included in block %s, hash: %s, srcTransferIdx: %d",
				receipt.BlockNumber, receipt.TxHash.Hex(), event.TransferIdx)
			break
		}
		if err != nil && err.Error() != "not found" {
			log.Fatal().Err(err).Msg("failed to get transaction receipt")
		}
		idx++
		time.Sleep(5 * time.Second)
	}
}
