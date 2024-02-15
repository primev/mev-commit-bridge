package transfer

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math"
	"math/big"
	shared "standard-bridge/pkg/shared"
	"time"

	l1g "github.com/primevprotocol/contracts-abi/clients/L1Gateway"
	sg "github.com/primevprotocol/contracts-abi/clients/SettlementGateway"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/sha3"
)

type Transfer struct {
	amount      *big.Int
	destAddress common.Address
	privateKey  *ecdsa.PrivateKey

	srcClient     *ethclient.Client
	srcChainID    *big.Int
	srcTransactor shared.GatewayTransactor
	srcFilterer   shared.GatewayFilterer

	destFilterer shared.GatewayFilterer
	destChainID  *big.Int
}

func NewTransferToSettlement(
	amount *big.Int,
	destAddress common.Address,
	privateKey *ecdsa.PrivateKey,
	settlementRPCUrl string,
	l1RPCUrl string,
	l1ContractAddr common.Address,
	settlementContractAddr common.Address,
) (*Transfer, error) {

	t := &Transfer{}
	commonSetup, err := t.getCommonSetup(privateKey, settlementRPCUrl, l1RPCUrl)
	if err != nil {
		return nil, err
	}

	l1t, err := l1g.NewL1gatewayTransactor(l1ContractAddr, commonSetup.l1Client)
	if err != nil {
		return nil, err
	}
	l1f, err := shared.NewL1Filterer(l1ContractAddr, commonSetup.l1Client)
	if err != nil {
		return nil, err
	}
	sf, err := shared.NewSettlementFilterer(settlementContractAddr, commonSetup.settlementClient)
	if err != nil {
		return nil, err
	}

	return &Transfer{
		amount:        amount,
		destAddress:   destAddress,
		privateKey:    privateKey,
		srcClient:     commonSetup.l1Client,
		srcChainID:    commonSetup.l1ChainID,
		srcTransactor: l1t,
		srcFilterer:   l1f,
		destFilterer:  sf,
		destChainID:   commonSetup.settlementChainID,
	}, nil
}

func NewTransferToL1(
	amount *big.Int,
	destAddress common.Address,
	privateKey *ecdsa.PrivateKey,
	settlementRPCUrl string,
	l1RPCUrl string,
	l1ContractAddr common.Address,
	settlementContractAddr common.Address,
) (*Transfer, error) {
	t := &Transfer{}
	commonSetup, err := t.getCommonSetup(privateKey, settlementRPCUrl, l1RPCUrl)
	if err != nil {
		return nil, err
	}

	st, err := sg.NewSettlementgatewayTransactor(settlementContractAddr, commonSetup.settlementClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create settlement gateway transactor: %s", err)
	}
	sf, err := shared.NewSettlementFilterer(settlementContractAddr, commonSetup.settlementClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create settlement filterer: %s", err)
	}
	l1f, err := shared.NewL1Filterer(l1ContractAddr, commonSetup.l1Client)
	if err != nil {
		return nil, fmt.Errorf("failed to create l1 filterer: %s", err)
	}

	return &Transfer{
		amount:        amount,
		destAddress:   destAddress,
		privateKey:    privateKey,
		srcClient:     commonSetup.settlementClient,
		srcChainID:    commonSetup.settlementChainID,
		srcTransactor: st,
		srcFilterer:   sf,
		destFilterer:  l1f,
		destChainID:   commonSetup.l1ChainID,
	}, nil
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
) (*commonSetup, error) {

	pubKey := &privateKey.PublicKey
	pubKeyBytes := crypto.FromECDSAPub(pubKey)
	hash := sha3.NewLegacyKeccak256()
	hash.Write(pubKeyBytes[1:])
	address := hash.Sum(nil)[12:]
	valAddr := common.BytesToAddress(address)
	log.Info().Msg("Signing address used for InitiateTransfer tx on source chain: " + valAddr.Hex())

	l1Client, err := ethclient.Dial(l1RPCUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to dial l1 rpc: %s", err)
	}
	l1ChainID, err := l1Client.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get l1 chain id: %s", err)
	}
	log.Debug().Msg("L1 chain id: " + l1ChainID.String())

	settlementClient, err := ethclient.Dial(settlementRPCUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to dial settlement rpc: %s", err)
	}
	settlementChainID, err := settlementClient.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get settlement chain id: %s", err)
	}
	log.Debug().Msg("Settlement chain id: " + settlementChainID.String())

	return &commonSetup{
		l1Client:          l1Client,
		l1ChainID:         l1ChainID,
		settlementClient:  settlementClient,
		settlementChainID: settlementChainID,
	}, nil
}

func (t *Transfer) Start(ctx context.Context) error {

	opts, err := shared.CreateTransactOpts(ctx, t.privateKey, t.srcChainID, t.srcClient)
	if err != nil {
		return fmt.Errorf("failed to get transact opts: %s", err)
	}

	// Important: tx value must match amount in transfer!
	// TODO: Look into being able to observe error logs from failed transactions that're still included in a block.
	// This method of calling InitiateTransfer silently failed when tx.value != amount.
	opts.Value = t.amount

	tx, err := t.srcTransactor.InitiateTransfer(
		opts,
		t.destAddress,
		t.amount,
	)
	if err != nil {
		return fmt.Errorf("failed to initiate transfer: %s", err)
	}
	log.Debug().Msgf("Transfer initialization tx sent, hash: %s, srcChain: %s, recipient: %s, amount: %d",
		tx.Hash().Hex(), t.srcChainID.String(), t.destAddress.Hex(), t.amount)

	// Wait for initiation transaction to be included in a block, or timeout
	idx := 0
	timeoutCount := 50
	includedInBlock := uint64(math.MaxUint64)
	for {
		if idx >= timeoutCount {
			return fmt.Errorf("timeout while waiting for transfer initiation tx to be included in a block")
		}
		receipt, err := t.srcClient.TransactionReceipt(ctx, tx.Hash())
		if receipt != nil {
			log.Info().Msgf("Transfer initialization tx included in block %s, hash: %s",
				receipt.BlockNumber, receipt.TxHash.Hex())
			includedInBlock = receipt.BlockNumber.Uint64()
			break
		}
		if err != nil && err.Error() != "not found" {
			return fmt.Errorf("error getting receipt for transfer initiation tx: %s", err)
		}
		idx++
		time.Sleep(5 * time.Second)
	}

	// Obtain event on src chain, transfer idx needed for dest chain
	if includedInBlock == math.MaxUint64 {
		return fmt.Errorf("transfer initiation tx not included in block")
	}
	event, err := t.srcFilterer.ObtainTransferInitiatedBySender(&bind.FilterOpts{
		Start: includedInBlock,
		End:   &includedInBlock,
	}, opts.From)
	if err != nil {
		return fmt.Errorf("error obtaining transfer initiated event: %s", err)
	}
	log.Info().Msgf("InitiateTransfer event emitted on src chain: %s, recipient: %s, amount: %d, transferIdx: %d",
		t.srcChainID.String(), event.Recipient, event.Amount, event.TransferIdx)

	log.Debug().Msgf("Waiting for transfer finalization tx from relayer")
	idx = 0
	for {
		if idx >= timeoutCount {
			return fmt.Errorf("timeout while waiting for transfer finalization tx from relayer")
		}
		opts := &bind.FilterOpts{
			Start: includedInBlock, // Start listening from block where InitiateTransfer tx was included
			End:   nil,
		}
		event, found, err := t.destFilterer.ObtainTransferFinalizedEvent(opts, event.TransferIdx)
		if err != nil {
			return fmt.Errorf("error obtaining transfer finalized event: %s", err)
		}
		if found {
			log.Info().Msgf("Transfer finalized on dest chain: %s, recipient: %s, amount: %d, srcTransferIdx: %d",
				t.destChainID.String(), event.Recipient, event.Amount, event.CounterpartyIdx)
			break
		}
		idx++
		time.Sleep(5 * time.Second)
	}
	return nil
}
