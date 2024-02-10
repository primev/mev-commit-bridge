package shared

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
)

func CreateTransactOpts(
	ctx context.Context,
	privateKey *ecdsa.PrivateKey,
	srcChainID *big.Int,
	srcClient *ethclient.Client,
) (*bind.TransactOpts, error) {
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, srcChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to create transactor: %w", err)
	}

	fromAddress := auth.From
	nonce, err := srcClient.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending nonce: %w", err)
	}
	auth.Nonce = big.NewInt(int64(nonce))

	// Returns priority fee per gas
	gasTip, err := srcClient.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas tip cap: %w", err)
	}
	// Returns priority fee per gas + base fee per gas
	gasPrice, err := srcClient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	auth.GasFeeCap = gasPrice
	auth.GasTipCap = gasTip
	auth.GasLimit = uint64(3000000)
	return auth, nil
}

func CancelPendingTxes(ctx context.Context, privateKey *ecdsa.PrivateKey, rawClient *ethclient.Client) error {
	cancelAllPendingTransactions(ctx, privateKey, rawClient)
	idx := 0
	timeoutSec := 60
	for {
		if idx >= timeoutSec {
			return fmt.Errorf("timeout: failed to cancel all pending transactions")
		}
		exist, err := PendingTransactionsExist(ctx, privateKey, rawClient)
		if err != nil {
			return fmt.Errorf("failed to check pending transactions: %w", err)
		}
		if !exist {
			log.Info().Msg("All pending transactions for signing account have been cancelled")
			return nil
		}
		time.Sleep(1 * time.Second)
		idx++
	}
}

func cancelAllPendingTransactions(
	ctx context.Context,
	privateKey *ecdsa.PrivateKey,
	rawClient *ethclient.Client,
) error {
	chainID, err := rawClient.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get chain id: %w", err)
	}
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	currentNonce, err := rawClient.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return fmt.Errorf("failed to get current pending nonce: %w", err)
	}
	log.Debug().Msgf("Current pending nonce: %d", currentNonce)

	latestNonce, err := rawClient.NonceAt(ctx, fromAddress, nil)
	if err != nil {
		return fmt.Errorf("failed to get latest nonce: %w", err)
	}
	log.Debug().Msgf("Latest nonce: %d", latestNonce)

	if currentNonce <= latestNonce {
		log.Info().Msg("No pending transactions to cancel")
		return nil
	}

	suggestedGasPrice, err := rawClient.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to get suggested gas price: %w", err)
	}
	log.Debug().Msgf("Suggested gas price: %s wei", suggestedGasPrice.String())

	for nonce := latestNonce; nonce < currentNonce; nonce++ {
		gasPrice := new(big.Int).Set(suggestedGasPrice)
		const maxRetries = 5
		for retry := 0; retry < maxRetries; retry++ {
			if retry > 0 {
				increase := new(big.Int).Div(gasPrice, big.NewInt(10))
				gasPrice = gasPrice.Add(gasPrice, increase)
				gasPrice = gasPrice.Add(gasPrice, big.NewInt(1))
				log.Debug().Msgf("Increased gas price for retry %d: %s wei", retry, gasPrice.String())
			}

			tx := types.NewTransaction(nonce, fromAddress, big.NewInt(0), 21000, gasPrice, nil)
			signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
			if err != nil {
				return fmt.Errorf("failed to sign cancellation transaction for nonce %d: %w", nonce, err)
			}

			err = rawClient.SendTransaction(ctx, signedTx)
			if err != nil {
				if err.Error() == "replacement transaction underpriced" {
					log.Warn().Err(err).Msgf("Retry %d: underpriced transaction for nonce %d, increasing gas price", retry+1, nonce)
					continue // Try again with a higher gas price
				}
				if err.Error() == "already known" {
					log.Warn().Err(err).Msgf("Retry %d: already known transaction for nonce %d", retry+1, nonce)
					continue // Try again with a higher gas price
				}
				return fmt.Errorf("failed to send cancellation transaction for nonce %d: %w", nonce, err)
			}
			log.Info().Msgf("Sent cancel transaction for nonce %d with tx hash: %s, gas price: %s wei", nonce, signedTx.Hash().Hex(), gasPrice.String())
			break
		}
	}
	return nil
}

func PendingTransactionsExist(ctx context.Context, privateKey *ecdsa.PrivateKey, rawClient *ethclient.Client) (bool, error) {
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	currentNonce, err := rawClient.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return false, fmt.Errorf("failed to get current pending nonce: %w", err)
	}

	latestNonce, err := rawClient.NonceAt(ctx, fromAddress, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get latest nonce: %w", err)
	}

	return currentNonce > latestNonce, nil
}
