package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"standard-bridge/pkg/shared"
	transfer "standard-bridge/pkg/transfer"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

var (
	optionConfig = &cli.StringFlag{
		Name:     "config",
		Usage:    "path to CLI config file",
		Required: true,
		EnvVars:  []string{"STANDARD_BRIDGE_CLI_CONFIG"},
	}
)

func main() {
	app := &cli.App{
		Name:  "bridge-cli",
		Usage: "CLI for interacting with a custom between L1 and the mev-commit (settlement) chain",
		Commands: []*cli.Command{
			{
				Name:  "bridge-to-settlement",
				Usage: "Submit a transaction to bridge ether to the settlement chain",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:     "amount",
						Usage:    "Amount of ether to bridge in wei",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "dest-addr",
						Usage:    "Destination address on the mev-commit (settlement) chain",
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "cancel-pending",
						Usage: "Automatically cancel existing pending transactions",
					},
					optionConfig,
				},
				Action: func(c *cli.Context) error {
					return bridgeToSettlement(c)
				},
			},
			{
				Name:  "bridge-to-l1",
				Usage: "Submit a transaction to bridge ether back to L1",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:     "amount",
						Usage:    "Amount of ether to bridge in wei",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "dest-addr",
						Usage:    "Destination address on L1",
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "cancel-pending",
						Usage: "Automatically cancel existing pending transactions",
					},
					optionConfig,
				},
				Action: func(c *cli.Context) error {
					return bridgeToL1(c)
				},
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(app.Writer, "Exited with error: %v\n", err)
	}
}

func bridgeToSettlement(c *cli.Context) error {
	config := preTransfer(c)
	autoCancel := c.Bool("cancel-pending")
	handlePendingTxes(context.Background(), config.PrivateKey, config.L1RPCUrl, autoCancel)
	t := transfer.NewTransferToSettlement(
		config.Amount,
		config.DestAddress,
		config.PrivateKey,
		config.SettlementRPCUrl,
		config.L1RPCUrl,
		config.L1ContractAddr,
		config.SettlementContractAddr,
	)
	t.Start(context.Background())
	return nil
}

func bridgeToL1(c *cli.Context) error {
	config := preTransfer(c)
	autoCancel := c.Bool("cancel-pending")
	handlePendingTxes(context.Background(), config.PrivateKey, config.SettlementRPCUrl, autoCancel)
	t := transfer.NewTransferToL1(
		config.Amount,
		config.DestAddress,
		config.PrivateKey,
		config.SettlementRPCUrl,
		config.L1RPCUrl,
		config.L1ContractAddr,
		config.SettlementContractAddr,
	)
	t.Start(context.Background())
	return nil
}

type preTransferConfig struct {
	Amount                 *big.Int
	DestAddress            common.Address
	PrivateKey             *ecdsa.PrivateKey
	SettlementRPCUrl       string
	L1RPCUrl               string
	L1ContractAddr         common.Address
	SettlementContractAddr common.Address
}

func preTransfer(c *cli.Context) preTransferConfig {

	configFilePath := c.String(optionConfig.Name)

	var cfg config
	buf, err := os.ReadFile(configFilePath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read config file at: " + configFilePath)
	}

	if err := yaml.Unmarshal(buf, &cfg); err != nil {
		log.Fatal().Err(err).Msg("failed to unmarshal config file at: " + configFilePath)
	}

	if err := checkConfig(&cfg); err != nil {
		log.Fatal().Err(err).Msg("invalid config")
	}

	lvl, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse log level")
	}
	zerolog.SetGlobalLevel(lvl)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	privKeyFile := cfg.PrivKeyFile
	if strings.HasPrefix(privKeyFile, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Err(err).Msg("failed to get user home dir")
		}
		privKeyFile = filepath.Join(homeDir, privKeyFile[2:])
	}

	privKey, err := crypto.LoadECDSA(privKeyFile)
	if err != nil {
		log.Err(err).Msg("failed to load private key")
	}

	amount := c.Int("amount")
	if amount <= 0 {
		log.Fatal().Msg("amount must be greater than 0")
	}

	destAddr := c.String("dest-addr")
	if !common.IsHexAddress(destAddr) {
		log.Fatal().Msg("dest-addr must be a valid hex address")
	}

	return preTransferConfig{
		Amount:                 big.NewInt(int64(amount)),
		DestAddress:            common.HexToAddress(destAddr),
		PrivateKey:             privKey,
		SettlementRPCUrl:       cfg.SettlementRPCUrl,
		L1RPCUrl:               cfg.L1RPCUrl,
		L1ContractAddr:         common.HexToAddress(cfg.L1ContractAddr),
		SettlementContractAddr: common.HexToAddress(cfg.SettlementContractAddr),
	}
}

type config struct {
	PrivKeyFile            string `yaml:"priv_key_file"`
	LogLevel               string `yaml:"log_level" json:"log_level"`
	L1RPCUrl               string `yaml:"l1_rpc_url"`
	SettlementRPCUrl       string `yaml:"settlement_rpc_url"`
	L1ChainID              int    `yaml:"l1_chain_id"`
	SettlementChainID      int    `yaml:"settlement_chain_id"`
	L1ContractAddr         string `yaml:"l1_contract_addr"`
	SettlementContractAddr string `yaml:"settlement_contract_addr"`
}

func checkConfig(cfg *config) error {
	if cfg.PrivKeyFile == "" {
		return fmt.Errorf("priv_key_file is required")
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.L1RPCUrl == "" || cfg.SettlementRPCUrl == "" {
		return fmt.Errorf("both l1_rpc_url and settlement_rpc_url are required")
	}
	if cfg.L1ChainID != 39999 && cfg.L1ChainID != 17000 {
		return fmt.Errorf("l1_chain_id must be 39999 (local l1) or 17000 (Holesky). Only test instances are supported")
	}
	if cfg.SettlementChainID != 17864 {
		return fmt.Errorf("settlement_chain_id must be 17864. Only test chains are supported")
	}
	if !common.IsHexAddress(cfg.L1ContractAddr) || !common.IsHexAddress(cfg.SettlementContractAddr) {
		return fmt.Errorf("both l1_contract_addr and settlement_contract_addr must be valid hex addresses")
	}

	// Create clients via url and cross check with expected chain id
	l1Client, err := ethclient.Dial(cfg.L1RPCUrl)
	if err != nil {
		return fmt.Errorf("failed to create l1 client: %v", err)
	}
	obtainedL1ChainID, err := l1Client.ChainID(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get l1 chain id: %v", err)
	}
	if obtainedL1ChainID.Cmp(big.NewInt(int64(cfg.L1ChainID))) != 0 {
		log.Fatal().Msgf("l1 chain id mismatch. Expected: %d, Obtained: %d", cfg.L1ChainID, obtainedL1ChainID)
	}

	settlementClient, err := ethclient.Dial(cfg.SettlementRPCUrl)
	if err != nil {
		return fmt.Errorf("failed to create settlement client: %v", err)
	}
	obtainedSettlementChainID, err := settlementClient.ChainID(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get settlement chain id: %v", err)
	}
	if obtainedSettlementChainID.Cmp(big.NewInt(int64(cfg.SettlementChainID))) != 0 {
		log.Fatal().Msgf("settlement chain id mismatch. Expected: %d, Obtained: %d", cfg.SettlementChainID, obtainedSettlementChainID)
	}

	return nil
}

func handlePendingTxes(
	ctx context.Context,
	privateKey *ecdsa.PrivateKey,
	url string,
	autoCancel bool,
) {
	ethClient, err := ethclient.Dial(url)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to eth client")
	}
	if !shared.PendingTransactionsExist(ctx, privateKey, ethClient) {
		return
	}
	if autoCancel {
		log.Info().Msg("Automatically cancelling existing pending transactions.")
		shared.CancelPendingTxes(ctx, privateKey, ethClient)
		return
	}
	fmt.Println("Pending transactions exist for signing account. Do you want to cancel them? (y/n)")
	var response string
	_, err = fmt.Scanln(&response)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read user input")
	}
	if strings.ToLower(response) == "y" {
		shared.CancelPendingTxes(ctx, privateKey, ethClient)
		return
	}
	log.Fatal().Msg("User chose not to cancel pending transactions. Exiting.")
}
