package main

import (
	"context"
	"math/big"
	"math/rand"
	"time"

	transfer "standard-bridge/pkg/user_cli"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

const (
	privateKeyString             = "YOUR_PRIVATE_KEY_HERE"
	transferAddressString        = "YOUR_ACCOUNT"
	settlementRPCUrl             = "SETTLEMENT_CHAIN_RPC_URL"
	l1RPCUrl                     = "L1_CHAIN_RPC_URL"
	l1ContractAddrString         = "L1_CONTRACT_ADDRESS"
	settlementContractAddrString = "SETTLEMENT_CONTRACT_ADDRESS"
	bridgeIntervalSeconds        = 5
)

func main() {
	privateKey, err := crypto.HexToECDSA(privateKeyString)
	if err != nil {
		panic("invalid private key")
	}

	transferAddr := common.HexToAddress(transferAddressString)

	l1ContractAddr := common.HexToAddress(l1ContractAddrString)
	settlementContractAddr := common.HexToAddress(settlementContractAddrString)

	ctx := context.Background()

	for {
		// Generate a random amount of wei in [0, 10 ETH]
		maxWei := new(big.Int).Mul(big.NewInt(10), big.NewInt(params.Ether))
		randomWeiValue := big.NewInt(rand.Int63n(maxWei.Int64()))

		// Create and start the transfer to the settlement chain
		tSettlement := transfer.NewTransferToSettlement(
			randomWeiValue,
			transferAddr,
			privateKey,
			settlementRPCUrl,
			l1RPCUrl,
			l1ContractAddr,
			settlementContractAddr,
		)
		tSettlement.Start(ctx)

		time.Sleep(time.Second * bridgeIntervalSeconds)

		// Create and start the transfer back to L1 with the same amount
		tL1 := transfer.NewTransferToL1(
			randomWeiValue,
			transferAddr,
			privateKey,
			settlementRPCUrl,
			l1RPCUrl,
			l1ContractAddr,
			settlementContractAddr,
		)
		tL1.Start(ctx)

		time.Sleep(time.Second * bridgeIntervalSeconds)
	}
}
