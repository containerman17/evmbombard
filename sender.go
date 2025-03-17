package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"sync"
	"time"

	"log"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var errorCount = 0
var lastError string

func init() {
	go func() {
		for {
			if errorCount > 0 {
				fmt.Printf("Errors: %d, Last error: %s\n", errorCount, lastError)
				errorCount = 0
				lastError = ""
			}
			time.Sleep(3 * time.Second)
		}
	}()
}

func bombardWithTransactions(client *ethclient.Client, key *ecdsa.PrivateKey, listener *TxListener) {
	fromAddress := crypto.PubkeyToAddress(key.PublicKey)

	gasLimit := uint64(21000)
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		log.Printf("failed to get chain ID: %v", err)
		return
	}

	shouldRefetchNonce := true

	nonce := uint64(0)

	to := crypto.PubkeyToAddress(key.PublicKey) // Send to self

	for {
		// Re-fetch nonce if previous transactions had errors
		if shouldRefetchNonce {
			newNonce, err := client.PendingNonceAt(context.Background(), fromAddress)
			if err != nil {
				log.Printf("failed to refresh nonce: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}
			nonce = newNonce
			shouldRefetchNonce = false
		}

		gasPrice := big.NewInt(1000000001 * 1000)

		signedTxs := make([]*types.Transaction, 0, batchSize)
		for i := 0; i < batchSize; i++ {
			var data []byte
			tx := types.NewTransaction(nonce, to, big.NewInt(int64(nonce)), gasLimit, gasPrice, data)

			signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), key)
			if err != nil {
				log.Fatalf("failed to sign transaction: %v", err)
			}

			signedTxs = append(signedTxs, signedTx)
			nonce++
		}

		var txHashesMutex sync.Mutex
		txHashes := make([]string, 0, len(signedTxs))
		var wg sync.WaitGroup
		errChan := make(chan error, len(signedTxs))
		hasError := false

		for _, signedTx := range signedTxs {
			wg.Add(1)
			go func(tx *types.Transaction) {
				defer wg.Done()

				err := client.SendTransaction(context.Background(), tx)
				if err != nil {
					errChan <- fmt.Errorf("failed to send transaction: %w", err)
					return
				}

				txHashesMutex.Lock()
				txHashes = append(txHashes, tx.Hash().String())
				txHashesMutex.Unlock()
			}(signedTx)
		}

		wg.Wait()
		close(errChan)

		// Log any transaction send errors
		for err := range errChan {
			if err != nil {
				lastError = err.Error()
				errorCount++
				hasError = true
			}
		}

		// If we had errors, mark that we should refetch the nonce
		if hasError {
			shouldRefetchNonce = true
			time.Sleep(1 * time.Second)
			continue
		}

		// Wait for all transactions to be mined
		for _, hash := range txHashes {
			if err := listener.AwaitTxMined(hash, timeoutSeconds); err != nil {
				lastError = err.Error()
				errorCount++
				shouldRefetchNonce = true
				time.Sleep(1 * time.Second)
			}
		}

		// fmt.Printf("Batch of %d transactions sent and mined\n", batchSize)
	}
}
