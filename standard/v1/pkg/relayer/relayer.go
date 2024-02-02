package relayer

import (
	"context"
	"crypto/ecdsa"
	"database/sql"
	"errors"
	"fmt"
	listener "standard-bridge/pkg/listener"
	"standard-bridge/pkg/transactor"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	l1g "github.com/primevprotocol/contracts-abi/clients/L1Gateway"
	sg "github.com/primevprotocol/contracts-abi/clients/SettlementGateway"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/sha3"
)

// TODO: unit tests

type Options struct {
	PrivateKey             *ecdsa.PrivateKey
	HTTPPort               int
	SettlementRPCUrl       string
	L1RPCUrl               string
	L1ContractAddr         common.Address
	SettlementContractAddr common.Address
	PgHost                 string
	PgPort                 int
	PgUser                 string
	PgPassword             string
	PgDbname               string
}

type Relayer struct {
	// Closes ctx's Done channel and waits for all goroutines to close.
	waitOnCloseRoutines func()
	db                  *sql.DB
}

func NewRelayer(opts *Options) *Relayer {

	r := &Relayer{}

	// TODO: db

	// db, err := initDB(opts)
	// if err != nil {
	// 	log.Fatal("failed to init db", err)
	// }
	// r.db = db

	// st, err := store.NewStore(db)
	// if err != nil {
	// 	log.Fatal("failed to init store", err)
	// }

	pubKey := &opts.PrivateKey.PublicKey
	pubKeyBytes := crypto.FromECDSAPub(pubKey)
	hash := sha3.NewLegacyKeccak256()
	hash.Write(pubKeyBytes[1:])
	address := hash.Sum(nil)[12:]
	valAddr := common.BytesToAddress(address)

	log.Info().Msg("Relayer signing address: " + valAddr.Hex())

	l1Client, err := ethclient.Dial(opts.L1RPCUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to dial l1 rpc")
	}

	l1ChainID, err := l1Client.ChainID(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get l1 chain id")
	}
	log.Info().Msg("L1 chain id: " + l1ChainID.String())

	settlementClient, err := ethclient.Dial(opts.SettlementRPCUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to dial settlement rpc")
	}

	settlementChainID, err := settlementClient.ChainID(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to dial settlement rpc")
	}
	log.Info().Msg("Settlement chain id: " + settlementChainID.String())

	// TODO: server

	ctx, cancel := context.WithCancel(context.Background())

	sFilterer := listener.NewSettlementFilterer(opts.SettlementContractAddr, settlementClient)
	sListener := listener.NewListener(settlementClient, sFilterer, false)
	sListenerClosed, settlementEventChan := sListener.Start(ctx)

	l1Filterer := listener.NewL1Filterer(opts.L1ContractAddr, l1Client)
	l1Listener := listener.NewListener(l1Client, l1Filterer, true)
	l1ListenerClosed, l1EventChan := l1Listener.Start(ctx)

	st, err := sg.NewSettlementgatewayTransactor(opts.SettlementContractAddr, settlementClient)
	if err != nil {
		log.Fatal().Msg("failed to create settlement gateway transactor")
	}
	settlementTransactor := transactor.NewTransactor(
		opts.PrivateKey,
		opts.SettlementContractAddr,
		settlementClient,
		st,
		sFilterer,
		l1EventChan, // L1 transfer initiations result in settlement finalizations
	)
	stClosed := settlementTransactor.Start(ctx)

	l1t, err := l1g.NewL1gatewayTransactor(opts.L1ContractAddr, l1Client)
	if err != nil {
		log.Fatal().Msg("failed to create l1 gateway transactor")
	}
	l1Transactor := transactor.NewTransactor(
		opts.PrivateKey,
		opts.L1ContractAddr,
		l1Client,
		l1t,
		l1Filterer,
		settlementEventChan, // Settlement transfer initiations result in L1 finalizations
	)
	l1tClosed := l1Transactor.Start(ctx)

	r.waitOnCloseRoutines = func() {
		// Close ctx's Done channel
		cancel()

		// TODO: stop server

		allClosed := make(chan struct{})
		go func() {
			defer close(allClosed)
			<-sListenerClosed
			<-l1ListenerClosed
			<-stClosed
			<-l1tClosed
		}()
		<-allClosed
	}
	return r
}

// TryCloseAll attempts to close all workers and the database connection.
func (r *Relayer) TryCloseAll() (err error) {
	log.Debug().Msg("closing all workers and db connection")
	defer func() {
		if r.db == nil {
			return
		}
		if err2 := r.db.Close(); err2 != nil {
			err = errors.Join(err, err2)
		}
	}()

	workersClosed := make(chan struct{})
	go func() {
		defer close(workersClosed)
		r.waitOnCloseRoutines()
	}()

	select {
	case <-workersClosed:
		log.Info().Msg("all workers closed")
		return nil
	case <-time.After(10 * time.Second):
		msg := "failed to close all workers in 10 sec"
		log.Error().Msg(msg)
		return errors.New(msg)
	}
}

func initDB(opts *Options) (db *sql.DB, err error) {
	// Connection string
	psqlInfo := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		opts.PgHost, opts.PgPort, opts.PgUser, opts.PgPassword, opts.PgDbname,
	)

	// Open a connection
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, err
	}

	// Check the connection
	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, err
}
