package relayer

import (
	"context"
	"crypto/ecdsa"
	"database/sql"
	"errors"
	"fmt"
	"standard-bridge/pkg/listener"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/sha3"
)

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
		// log.Fatal().Err(err).Msg("failed to get l1 chain id")
		log.Debug().Msg("Skipping l1 chain id")
	}
	log.Info().Msg("L1 chain id: " + l1ChainID.String())

	settlementClient, err := ethclient.Dial(opts.SettlementRPCUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to dial settlement rpc")
	}

	settlementChainID, err := settlementClient.ChainID(context.Background())
	if err != nil {
		// log.Fatal().Err(err).Msg("failed to dial settlement rpc")
		log.Debug().Msg("Skipping settlement rpc")
	}
	log.Info().Msg("Settlement chain id: " + settlementChainID.String())

	// TODO: read-only contract clients

	// TODO: server

	l1Listener := listener.NewListener(l1Client)
	settlementListener := listener.NewListener(settlementClient)

	ctx, cancel := context.WithCancel(context.Background())

	l1ListenerClosed := l1Listener.Start(ctx)
	settlementListenerClosed := settlementListener.Start(ctx)

	r.waitOnCloseRoutines = func() {
		// Close ctx's Done channel
		cancel()

		// TODO: stop server

		allClosed := make(chan struct{})
		go func() {
			defer close(allClosed)
			<-l1ListenerClosed
			<-settlementListenerClosed
		}()
		<-allClosed
	}
	return r
}

// TryCloseAll attempts to close all workers and the database connection.
func (r *Relayer) TryCloseAll() (err error) {
	log.Debug().Msg("closing all workers and db connection")
	defer func() {
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
