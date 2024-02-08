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
		Required: true,
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

type config struct {
	PrivKeyFile            string `yaml:"priv_key_file" json:"priv_key_file"`
	HTTPPort               int    `yaml:"http_port" json:"http_port"`
	LogLevel               string `yaml:"log_level" json:"log_level"`
	L1RPCUrl               string `yaml:"l1_rpc_url" json:"l1_rpc_url"`
	SettlementRPCUrl       string `yaml:"settlement_rpc_url" json:"settlement_rpc_url"`
	L1ContractAddr         string `yaml:"l1_contract_addr" json:"l1_contract_addr"`
	SettlementContractAddr string `yaml:"settlement_contract_addr" json:"settlement_contract_addr"`
	PgHost                 string `yaml:"pg_host" json:"pg_host"`
	PgPort                 int    `yaml:"pg_port" json:"pg_port"`
	PgUser                 string `yaml:"pg_user" json:"pg_user"`
	PgPassword             string `yaml:"pg_password" json:"pg_password"`
	PgDbname               string `yaml:"pg_dbname" json:"pg_dbname"`
}

func checkConfig(cfg *config) error {
	if cfg.PrivKeyFile == "" {
		return fmt.Errorf("priv_key_file is required")
	}

	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = defaultHTTPPort
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

	if cfg.PgHost == "" || cfg.PgPort == 0 || cfg.PgUser == "" || cfg.PgPassword == "" || cfg.PgDbname == "" {
		return fmt.Errorf("pg_host, pg_port, pg_user, pg_password, pg_dbname are required")
	}

	return nil
}

func start(c *cli.Context) error {

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

	r := relayer.NewRelayer(&relayer.Options{
		PrivateKey:             privKey,
		HTTPPort:               cfg.HTTPPort,
		L1RPCUrl:               cfg.L1RPCUrl,
		SettlementRPCUrl:       cfg.SettlementRPCUrl,
		L1ContractAddr:         common.HexToAddress(cfg.L1ContractAddr),
		SettlementContractAddr: common.HexToAddress(cfg.SettlementContractAddr),
		PgHost:                 cfg.PgHost,
		PgPort:                 cfg.PgPort,
		PgUser:                 cfg.PgUser,
		PgPassword:             cfg.PgPassword,
		PgDbname:               cfg.PgDbname,
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
