package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"standard-bridge/pkg/relayer"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

const (
	defaultHTTPPort = 8080
)

var (
	optionConfig = &cli.StringFlag{
		Name:     "config",
		Usage:    "path to relayer config file",
		Required: false, // Can also set config via env var
		EnvVars:  []string{"STANDARD_BRIDGE_RELAYER_CONFIG"},
	}
)

func main() {
	app := &cli.App{
		Name:  "standard-bridge-relayer",
		Usage: "Entry point for relayer of mev-commit standard bridge",
		Commands: []*cli.Command{
			{
				Name:  "start",
				Usage: "Start standard bridge relayer",
				Flags: []cli.Flag{
					optionConfig,
				},
				Action: func(c *cli.Context) error {
					return start(c)
				},
			},
		}}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(app.Writer, "exited with error: %v\n", err)
	}
}

func loadConfigFromEnv() config {
	cfg := config{
		PrivKeyFilePath:        os.Getenv("PRIVATE_KEY_FILE_PATH"),
		LogLevel:               os.Getenv("LOG_LEVEL"),
		L1RPCUrl:               os.Getenv("L1_RPC_URL"),
		SettlementRPCUrl:       os.Getenv("SETTLEMENT_RPC_URL"),
		L1ContractAddr:         os.Getenv("L1_CONTRACT_ADDR"),
		SettlementContractAddr: os.Getenv("SETTLEMENT_CONTRACT_ADDR"),
	}
	return cfg
}

func loadConfigFromFile(cfg *config, filePath string) error {
	buf, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file at: %s, %w", filePath, err)
	}
	if err := yaml.Unmarshal(buf, cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config file at: %s, %w", filePath, err)
	}
	return nil
}

func setupLogging(logLevel string) {
	lvl, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse log level")
	}
	zerolog.SetGlobalLevel(lvl)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

type config struct {
	PrivKeyFilePath        string `yaml:"priv_key_file_path" json:"priv_key_file_path"`
	LogLevel               string `yaml:"log_level" json:"log_level"`
	L1RPCUrl               string `yaml:"l1_rpc_url" json:"l1_rpc_url"`
	SettlementRPCUrl       string `yaml:"settlement_rpc_url" json:"settlement_rpc_url"`
	L1ContractAddr         string `yaml:"l1_contract_addr" json:"l1_contract_addr"`
	SettlementContractAddr string `yaml:"settlement_contract_addr" json:"settlement_contract_addr"`
}

func checkConfig(cfg *config) error {
	if cfg.PrivKeyFilePath == "" {
		return fmt.Errorf("priv_key_file_path is required")
	}

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	if cfg.L1RPCUrl == "" {
		return fmt.Errorf("l1_rpc_url is required")
	}

	if cfg.SettlementRPCUrl == "" {
		return fmt.Errorf("settlement_rpc_url is required")
	}

	if cfg.L1ContractAddr == "" {
		return fmt.Errorf("oracle_contract_addr is required")
	}

	if cfg.SettlementContractAddr == "" {
		return fmt.Errorf("preconf_contract_addr is required")
	}

	return nil
}

func start(c *cli.Context) error {
	cfg := loadConfigFromEnv()

	configFilePath := c.String(optionConfig.Name)
	if configFilePath == "" {
		log.Info().Msg("env var config will be used")
	} else {
		log.Info().Str("config_file", configFilePath).Msg(
			"overriding env var config with file")
		if err := loadConfigFromFile(&cfg, configFilePath); err != nil {
			log.Fatal().Err(err).Msg("failed to load config provided as file")
		}
	}

	if err := checkConfig(&cfg); err != nil {
		log.Fatal().Err(err).Msg("invalid config")
	}

	setupLogging(cfg.LogLevel)

	privKeyFilePath := cfg.PrivKeyFilePath

	if strings.HasPrefix(privKeyFilePath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Err(err).Msg("failed to get user home dir")
		}
		privKeyFilePath = filepath.Join(homeDir, privKeyFilePath[2:])
	}

	privKey, err := crypto.LoadECDSA(privKeyFilePath)
	if err != nil {
		log.Err(err).Msg("failed to load private key")
	}

	r := relayer.NewRelayer(&relayer.Options{
		PrivateKey:             privKey,
		L1RPCUrl:               cfg.L1RPCUrl,
		SettlementRPCUrl:       cfg.SettlementRPCUrl,
		L1ContractAddr:         common.HexToAddress(cfg.L1ContractAddr),
		SettlementContractAddr: common.HexToAddress(cfg.SettlementContractAddr),
	})

	interruptSigChan := make(chan os.Signal, 1)
	signal.Notify(interruptSigChan, os.Interrupt, syscall.SIGTERM)

	// Block until interrupt signal OR context's Done channel is closed.
	select {
	case <-interruptSigChan:
	case <-c.Done():
	}
	fmt.Fprintf(c.App.Writer, "shutting down...\n")

	closedAllSuccessfully := make(chan struct{})
	go func() {
		defer close(closedAllSuccessfully)

		err := r.TryCloseAll()
		if err != nil {
			log.Error().Err(err).Msg("failed to close all routines and db connection")
		}
	}()
	select {
	case <-closedAllSuccessfully:
	case <-time.After(5 * time.Second):
		log.Error().Msg("failed to close all in time")
	}

	return nil
}
