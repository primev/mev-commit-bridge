package usercli

import (
	"context"
	"crypto/ecdsa"
	"math"
	"math/big"
	"standard-bridge/pkg/listener"
	"time"

	l1g "github.com/primevprotocol/contracts-abi/clients/L1Gateway"
	sg "github.com/primevprotocol/contracts-abi/clients/SettlementGateway"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/sha3"
)

type Transfer struct {
	Amount      uint64
	DestAddress common.Address
	PrivateKey  *ecdsa.PrivateKey

	SrcClient     *ethclient.Client
	SrcChainID    *big.Int
	SrcTransactor GatewayTransactor
	SrcFilterer   GatewayFilterer

	DestFilterer GatewayFilterer
	DestChainID  *big.Int
}

type GatewayTransactor interface {
	InitiateTransfer(opts *bind.TransactOpts, _recipient common.Address,
		amount *big.Int) (*types.Transaction, error)
}

type GatewayFilterer interface {
	ObtainTransferFinalizedEvent(opts *bind.FilterOpts, counterpartyIdx *big.Int,
	) (listener.TransferFinalizedEvent, bool)
	MustObtainTransferInitiatedBySender(opts *bind.FilterOpts, sender common.Address,
	) listener.TransferInitiatedEvent
}

func NewTransferToSettlement(
	amount uint64,
	destAddress common.Address,
	privateKey *ecdsa.PrivateKey,
	settlementRPCUrl string,
	l1RPCUrl string,
	l1ContractAddr common.Address,
	settlementContractAddr common.Address,
) *Transfer {

	t := &Transfer{}
	commonSetup := t.getCommonSetup(privateKey, settlementRPCUrl, l1RPCUrl)

	l1t, err := l1g.NewL1gatewayTransactor(l1ContractAddr, commonSetup.l1Client)
	if err != nil {
		log.Fatal().Msg("failed to create l1 gateway transactor")
	}
	l1f := listener.NewL1Filterer(l1ContractAddr, commonSetup.l1Client)

	sf := listener.NewSettlementFilterer(settlementContractAddr, commonSetup.settlementClient)

	return &Transfer{
		Amount:        amount,
		DestAddress:   destAddress,
		PrivateKey:    privateKey,
		SrcClient:     commonSetup.l1Client,
		SrcChainID:    commonSetup.l1ChainID,
		SrcTransactor: l1t,
		SrcFilterer:   l1f,
		DestFilterer:  sf,
		DestChainID:   commonSetup.settlementChainID,
	}
}

func NewTransferToL1(
	amount uint64,
	destAddress common.Address,
	privateKey *ecdsa.PrivateKey,
	settlementRPCUrl string,
	l1RPCUrl string,
	l1ContractAddr common.Address,
	settlementContractAddr common.Address,
) *Transfer {
	t := &Transfer{}
	commonSetup := t.getCommonSetup(privateKey, settlementRPCUrl, l1RPCUrl)

	st, err := sg.NewSettlementgatewayTransactor(settlementContractAddr, commonSetup.settlementClient)
	if err != nil {
		log.Fatal().Msg("failed to create settlement gateway transactor")
	}
	sf := listener.NewSettlementFilterer(settlementContractAddr, commonSetup.settlementClient)
	l1f := listener.NewL1Filterer(l1ContractAddr, commonSetup.l1Client)

	return &Transfer{
		Amount:        amount,
		DestAddress:   destAddress,
		PrivateKey:    privateKey,
		SrcClient:     commonSetup.settlementClient,
		SrcChainID:    commonSetup.settlementChainID,
		SrcTransactor: st,
		SrcFilterer:   sf,
		DestFilterer:  l1f,
		DestChainID:   commonSetup.l1ChainID,
	}
}

type commonSetup struct {
	l1Client          *ethclient.Client
	l1ChainID         *big.Int
	settlementClient  *ethclient.Client
	settlementChainID *big.Int
}

func (t *Transfer) getCommonSetup(
	privateKey *ecdsa.PrivateKey,
	settlementRPCUrl string,
	l1RPCUrl string,
) *commonSetup {

	pubKey := &privateKey.PublicKey
	pubKeyBytes := crypto.FromECDSAPub(pubKey)
	hash := sha3.NewLegacyKeccak256()
	hash.Write(pubKeyBytes[1:])
	address := hash.Sum(nil)[12:]
	valAddr := common.BytesToAddress(address)
	log.Info().Msg("Signing address used for InitiateTransfer tx on source chain: " + valAddr.Hex())

	l1Client, err := ethclient.Dial(l1RPCUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to dial l1 rpc")
	}
	l1ChainID, err := l1Client.ChainID(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get l1 chain id")
	}
	log.Debug().Msg("L1 chain id: " + l1ChainID.String())

	settlementClient, err := ethclient.Dial(settlementRPCUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to dial settlement rpc")
	}
	settlementChainID, err := settlementClient.ChainID(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to dial settlement rpc")
	}
	log.Debug().Msg("Settlement chain id: " + settlementChainID.String())

	return &commonSetup{
		l1Client:          l1Client,
		l1ChainID:         l1ChainID,
		settlementClient:  settlementClient,
		settlementChainID: settlementChainID,
	}
}

// TODO: Consolidate w/ func from transactor.go
func (t *Transfer) mustGetTransactOpts(
	ctx context.Context,
) *bind.TransactOpts {
	auth, err := bind.NewKeyedTransactorWithChainID(t.PrivateKey, t.SrcChainID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get keyed transactor")
	}
	nonce, err := t.SrcClient.PendingNonceAt(ctx, auth.From)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get pending nonce")
	}
	auth.Nonce = big.NewInt(int64(nonce))

	// Returns priority fee per gas
	gasTip, err := t.SrcClient.SuggestGasTipCap(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get gas tip cap")
	}
	// Returns priority fee per gas + base fee per gas
	gasPrice, err := t.SrcClient.SuggestGasPrice(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get gas price")
	}

	auth.GasFeeCap = gasPrice
	auth.GasTipCap = gasTip
	auth.GasLimit = uint64(3000000)
	return auth
}

func (t *Transfer) Start(ctx context.Context) {

	opts := t.mustGetTransactOpts(ctx)

	// Important: tx value must match amount in transfer!
	// TODO: Look into being able to observe error logs from failed transactions that're still included in a block.
	// This method of calling InitiateTransfer silently failed when tx.value != amount.
	amount := big.NewInt(int64(t.Amount))
	opts.Value = amount

	tx, err := t.SrcTransactor.InitiateTransfer(
		opts,
		t.DestAddress,
		amount,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to send initiate transfer tx")
	}
	log.Debug().Msgf("Transfer initialization tx sent, hash: %s, srcChain: %s, recipient: %s, amount: %d",
		tx.Hash().Hex(), t.SrcChainID.String(), t.DestAddress.Hex(), t.Amount)

	// Wait for initiation transaction to be included in a block, or timeout
	idx := 0
	timeoutCount := 20
	includedInBlock := uint64(math.MaxUint64)
	for {
		if idx >= timeoutCount {
			log.Error().Msg("timeout while waiting for transfer initiation tx to be included in a block")
			return
		}
		receipt, err := t.SrcClient.TransactionReceipt(ctx, tx.Hash())
		if receipt != nil {
			log.Info().Msgf("Transfer initialization tx included in block %s, hash: %s",
				receipt.BlockNumber, receipt.TxHash.Hex())
			includedInBlock = receipt.BlockNumber.Uint64()
			break
		}
		if err != nil && err.Error() != "not found" {
			log.Fatal().Err(err).Msg("failed to get transaction receipt")
		}
		idx++
		time.Sleep(5 * time.Second)
	}

	// Obtain event on src chain, transfer idx needed for dest chain
	if includedInBlock == math.MaxUint64 {
		log.Fatal().Msg("failed to obtain block number for transfer initiation tx")
	}
	event := t.SrcFilterer.MustObtainTransferInitiatedBySender(&bind.FilterOpts{
		Start: includedInBlock,
		End:   &includedInBlock,
	}, opts.From)
	log.Info().Msgf("InitiateTransfer event emitted on src chain: %s, recipient: %s, amount: %d, transferIdx: %d",
		t.SrcChainID.String(), event.Recipient, event.Amount, event.TransferIdx)

	log.Debug().Msgf("Waiting for transfer finalization tx from relayer")
	idx = 0
	for {
		if idx >= timeoutCount {
			log.Error().Msg("timeout while waiting for transfer finalization tx from relayer")
			return
		}
		opts := &bind.FilterOpts{
			Start: 0,
			End:   nil,
		}
		event, found := t.DestFilterer.ObtainTransferFinalizedEvent(opts, event.TransferIdx)
		if found {
			log.Info().Msgf("Transfer finalized on dest chain: %s, recipient: %s, amount: %d, srcTransferIdx: %d",
				t.DestChainID.String(), event.Recipient, event.Amount, event.CounterpartyIdx)
			break
		}
		idx++
		time.Sleep(5 * time.Second)
	}
}
