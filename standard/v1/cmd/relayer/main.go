package main

import (
	"fmt"
	"os"
	"path/filepath"
	"standard-bridge/pkg/relayer"
	"strings"
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
	configFile := c.String(optionConfig.Name)
	fmt.Fprintf(c.App.Writer, "starting standard bridge relayer with config file: %s\n", configFile)

	var cfg config
	buf, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file at '%s': %w", configFile, err)
	}

	if err := yaml.Unmarshal(buf, &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config file at '%s': %w", configFile, err)
	}

	if err := checkConfig(&cfg); err != nil {
		return fmt.Errorf("invalid config file at '%s': %w", configFile, err)
	}

	lvl, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to parse log level '%s': %w", cfg.LogLevel, err)
	}

	zerolog.SetGlobalLevel(lvl)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(os.Stdout).With().Caller().Logger()

	privKeyFile := cfg.PrivKeyFile
	if strings.HasPrefix(privKeyFile, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}
		privKeyFile = filepath.Join(homeDir, privKeyFile[2:])
	}

	privKey, err := crypto.LoadECDSA(privKeyFile)
	if err != nil {
		return fmt.Errorf("failed to load private key from file '%s': %w", cfg.PrivKeyFile, err)
	}

	relayer := relayer.NewRelayer(&relayer.Options{
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

	<-c.Done()
	fmt.Fprintf(c.App.Writer, "shutting down...\n")
	closedAllSuccessfully := make(chan struct{})
	go func() {
		defer close(closedAllSuccessfully)

		err := relayer.TryCloseAll()
		if err != nil {
			log.Error().Err(err).Msg("failed to close relayer")
		}
	}()
	select {
	case <-closedAllSuccessfully:
	case <-time.After(5 * time.Second):
		log.Error().Msg("failed to close relayer in time")
	}

	return nil
}
