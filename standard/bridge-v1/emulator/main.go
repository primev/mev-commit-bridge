package main

import (
	"context"
	"crypto/rand"
	"math/big"
	mathrand "math/rand"

	"os"
	"time"

	"github.com/rs/zerolog/log"

	"standard-bridge/pkg/shared"
	transfer "standard-bridge/pkg/transfer"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"

	datadog "github.com/DataDog/datadog-api-client-go/api/v2/datadog"
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

	// DD setup
	ctx := context.WithValue(context.Background(), datadog.ContextAPIKeys, map[string]datadog.APIKey{
		"apiKeyAuth": {
			Key: os.Getenv("DD_API_KEY"),
		},
		"appKeyAuth": {
			Key: os.Getenv("DD_APP_KEY"),
		},
	})

	configuration := datadog.NewConfiguration()
	apiClient := datadog.NewAPIClient(configuration)

	// Construct two eth clients and cancel all pending txes on both chains
	l1Client, err := ethclient.Dial(l1RPCUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to dial l1 rpc")
	}
	settlementClient, err := ethclient.Dial(settlementRPCUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to dial settlement rpc")
	}
	shared.CancelPendingTxes(ctx, privateKey, l1Client)
	shared.CancelPendingTxes(ctx, privateKey, settlementClient)

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

		// TODO: support persistent connections

		// Create and start the transfer to the settlement chain
		startTime := time.Now()
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
		completionTimeSec := time.Since(startTime).Seconds()
		metricName := "bridging.success"
		postMetricToDatadog(ctx, apiClient, metricName, completionTimeSec,
			[]string{"environment:bridge_test", "account_addr:" + transferAddressString, "to_chain_id:" + "17864"},
		)

		// Sleep for random interval between 0 and 5 seconds
		time.Sleep(time.Duration(mathrand.Intn(6)) * time.Second)

		// Bridge back same amount minus 0.009 ETH for fees
		pZZNineEth := big.NewInt(9 * params.Ether / 1000)
		amountBack := new(big.Int).Sub(randWeiValue, pZZNineEth)

		// Create and start the transfer back to L1 with the same amount
		startTime = time.Now()
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
		completionTimeSec = time.Since(startTime).Seconds()
		metricName = "bridging.success"
		postMetricToDatadog(ctx, apiClient, metricName, completionTimeSec,
			[]string{"environment:bridge_test", "account_addr:" + transferAddressString, "to_chain_id:" + "39999"},
		)

		// Sleep for random interval between 0 and 5 seconds
		time.Sleep(time.Duration(mathrand.Intn(6)) * time.Second)
	}
}

func postMetricToDatadog(ctx context.Context, client *datadog.APIClient, metricName string, value float64, tags []string) {
	now := time.Now().Unix()
	point := datadog.MetricPoint{
		Timestamp: datadog.PtrInt64(now),
		Value:     datadog.PtrFloat64(value),
	}
	series := datadog.MetricSeries{
		Metric: metricName,
		Type:   datadog.METRICINTAKETYPE_GAUGE.Ptr(),
		Points: []datadog.MetricPoint{point},
		Tags:   tags,
	}
	payload := datadog.MetricPayload{
		Series: []datadog.MetricSeries{series},
	}
	_, _, err := client.MetricsApi.SubmitMetrics(ctx, payload)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to post metric to Datadog")
	}
	log.Debug().Msgf("Posted metric %s with value %f and tags %v", metricName, value, tags)
}
