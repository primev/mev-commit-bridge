package shared

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
)

func CancelPendingTxes(ctx context.Context, privateKey *ecdsa.PrivateKey, rawClient *ethclient.Client, chainID *big.Int) {
	cancelAllPendingTransactions(ctx, privateKey, rawClient, chainID)
	idx := 0
	timeoutSec := 60
	for {
		if idx >= timeoutSec {
			log.Fatal().Msg("Timeout reached while waiting for pending transactions to be cancelled")
		}
		if !pendingTransactionsExist(ctx, privateKey, rawClient) {
			break
		}
		time.Sleep(1 * time.Second)
		idx++
	}
}

func cancelAllPendingTransactions(
	ctx context.Context,
	privateKey *ecdsa.PrivateKey,
	rawClient *ethclient.Client,
	chainID *big.Int,
) {
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	currentNonce, err := rawClient.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get current pending nonce")
		return
	}
	log.Debug().Msgf("Current pending nonce: %d", currentNonce)

	latestNonce, err := rawClient.NonceAt(ctx, fromAddress, nil) // nil for the latest block
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get latest nonce")
		return
	}
	log.Debug().Msgf("Latest nonce: %d", latestNonce)

	if currentNonce <= latestNonce {
		log.Info().Msg("No pending transactions to cancel")
		return
	}

	suggestedGasPrice, err := rawClient.SuggestGasPrice(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to suggest gas price")
		return
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
				log.Fatal().Err(err).Msgf("Failed to sign transaction for nonce %d", nonce)
				break
			}

			err = rawClient.SendTransaction(ctx, signedTx)
			if err != nil {
				if err.Error() == "replacement transaction underpriced" {
					log.Warn().Err(err).Msgf("Retry %d: underpriced transaction for nonce %d, increasing gas price", retry+1, nonce)
					continue // Try again with a higher gas price
				}
				log.Fatal().Err(err).Msgf("Failed to send cancel transaction for nonce %d", nonce)
				break
			}
			log.Info().Msgf("Sent cancel transaction for nonce %d with tx hash: %s, gas price: %s wei", nonce, signedTx.Hash().Hex(), gasPrice.String())
			break
		}
	}
}

func pendingTransactionsExist(ctx context.Context, privateKey *ecdsa.PrivateKey, rawClient *ethclient.Client) bool {
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	currentNonce, err := rawClient.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get current pending nonce")
		return true
	}

	latestNonce, err := rawClient.NonceAt(ctx, fromAddress, nil) // nil for the latest block
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get latest nonce")
		return true
	}

	return currentNonce > latestNonce
}
