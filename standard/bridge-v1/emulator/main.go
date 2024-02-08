package main

import (
	"context"
	"crypto/rand"
	"math/big"
	mathrand "math/rand"

	"os"
	"time"

	"github.com/rs/zerolog/log"

	transfer "standard-bridge/pkg/transfer"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

const (
	settlementRPCUrl             = "http://sl-bootnode:8545"
	l1RPCUrl                     = "http://l1-bootnode:8545"
	l1ContractAddrString         = "0x1a18dfEc4f2B66207b1Ad30aB5c7A0d62Ef4A40b"
	settlementContractAddrString = "0xc1f93bE11D7472c9B9a4d87B41dD0a491F1fbc75"
)

func main() {

	privateKeyString := os.Getenv("PRIVATE_KEY")
	if privateKeyString == "" {
		log.Fatal().Msg("PRIVATE_KEY env var is required")
	}
	privateKey, err := crypto.HexToECDSA(privateKeyString)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse private key")
	}

	transferAddressString := os.Getenv("ACCOUNT_ADDR")
	if transferAddressString == "" {
		log.Fatal().Msg("ACCOUNT_ADDR env var is required")
	}
	if !common.IsHexAddress(transferAddressString) {
		log.Fatal().Msg("ACCOUNT_ADDR is not a valid address")
	}
	transferAddr := common.HexToAddress(transferAddressString)

	l1ContractAddr := common.HexToAddress(l1ContractAddrString)
	settlementContractAddr := common.HexToAddress(settlementContractAddrString)
	ctx := context.Background()

	for {
		// Generate a random amount of wei in [0.01, 10] ETH
		maxWei := new(big.Int).Mul(big.NewInt(10), big.NewInt(params.Ether))
		randWeiValue, err := rand.Int(rand.Reader, maxWei)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to generate random value")
		}
		if randWeiValue.Cmp(big.NewInt(params.Ether/100)) < 0 {
			// Enforce minimum value of 0.01 ETH
			randWeiValue = big.NewInt(params.Ether / 100)
		}

		// Create and start the transfer to the settlement chain
		tSettlement := transfer.NewTransferToSettlement(
			randWeiValue,
			transferAddr,
			privateKey,
			settlementRPCUrl,
			l1RPCUrl,
			l1ContractAddr,
			settlementContractAddr,
		)
		tSettlement.Start(ctx)

		// Sleep for random interval between 0 and 5 seconds
		time.Sleep(time.Duration(mathrand.Intn(6)) * time.Second)

		// Bridge back same amount minus 0.009 ETH for fees
		pZZNineEth := big.NewInt(9 * params.Ether / 1000)
		amountBack := new(big.Int).Sub(randWeiValue, pZZNineEth)

		// Create and start the transfer back to L1 with the same amount
		tL1 := transfer.NewTransferToL1(
			amountBack,
			transferAddr,
			privateKey,
			settlementRPCUrl,
			l1RPCUrl,
			l1ContractAddr,
			settlementContractAddr,
		)
		tL1.Start(ctx)

		// Sleep for random interval between 0 and 5 seconds
		time.Sleep(time.Duration(mathrand.Intn(6)) * time.Second)
	}
}
