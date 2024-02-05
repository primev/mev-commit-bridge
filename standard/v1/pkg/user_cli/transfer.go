package usercli

import (
	"context"
	"crypto/ecdsa"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/sha3"
)

type Transfer struct {
	Amount                 uint64
	DestAddress            common.Address
	PrivateKey             *ecdsa.PrivateKey
	L1Client               *ethclient.Client
	L1ChainID              *big.Int
	SettlementClient       *ethclient.Client
	SettlementChainID      *big.Int
	L1ContractAddr         common.Address
	SettlementContractAddr common.Address
}

func NewTransfer(
	amount uint64,
	destAddress common.Address,
	privateKey *ecdsa.PrivateKey,
	settlementRPCUrl string,
	l1RPCUrl string,
	l1ContractAddr common.Address,
	settlementContractAddr common.Address,
) *Transfer {
	pubKey := &privateKey.PublicKey
	pubKeyBytes := crypto.FromECDSAPub(pubKey)
	hash := sha3.NewLegacyKeccak256()
	hash.Write(pubKeyBytes[1:])
	address := hash.Sum(nil)[12:]
	valAddr := common.BytesToAddress(address)
	log.Info().Msg("Bridge tx signing address: " + valAddr.Hex())

	l1Client, err := ethclient.Dial(l1RPCUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to dial l1 rpc")
	}
	l1ChainID, err := l1Client.ChainID(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get l1 chain id")
	}
	log.Info().Msg("L1 chain id: " + l1ChainID.String())

	settlementClient, err := ethclient.Dial(settlementRPCUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to dial settlement rpc")
	}
	settlementChainID, err := settlementClient.ChainID(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to dial settlement rpc")
	}
	log.Info().Msg("Settlement chain id: " + settlementChainID.String())

	return &Transfer{
		Amount:                 amount,
		DestAddress:            destAddress,
		PrivateKey:             privateKey,
		L1Client:               l1Client,
		L1ChainID:              l1ChainID,
		SettlementClient:       settlementClient,
		SettlementChainID:      settlementChainID,
		L1ContractAddr:         l1ContractAddr,
		SettlementContractAddr: settlementContractAddr,
	}
}

func (t *Transfer) BridgeToSettlement() {
	// log transfer params
	log.Info().Msgf("amount: %d", t.Amount)
	log.Info().Msgf("dest address: %s", t.DestAddress.Hex())
	log.Info().Msgf("l1 contract address: %s", t.L1ContractAddr.Hex())
	log.Info().Msgf("settlement contract address: %s", t.SettlementContractAddr.Hex())
}

func (t *Transfer) BridgeToL1() {
	// log transfer params
	log.Info().Msgf("amount: %d", t.Amount)
	log.Info().Msgf("dest address: %s", t.DestAddress.Hex())
	log.Info().Msgf("l1 contract address: %s", t.L1ContractAddr.Hex())
	log.Info().Msgf("settlement contract address: %s", t.SettlementContractAddr.Hex())
}
