package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	"log"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var errorCount = 0
var lastError string

var gasPrice = big.NewInt(0)

func startGasPriceMonitor(client *ethclient.Client) {
	gasPrice = big.NewInt(100000000 * 10)

	// Update gas price periodically
	go func() {
		for {
			if price, err := client.SuggestGasPrice(context.Background()); err == nil {
				oldGasPrice := gasPrice
				// Convert to gwei (1 gwei = 10^9 wei)
				gasPrice = new(big.Int).Mul(price, big.NewInt(2))
				if gasPrice.Cmp(oldGasPrice) != 0 {
					oldPriceGwei := new(big.Int).Div(oldGasPrice, big.NewInt(1000000000))
					newPriceGwei := new(big.Int).Div(gasPrice, big.NewInt(1000000000))
					fmt.Printf("Gas price changed from %d gwei to %d gwei\n", oldPriceGwei, newPriceGwei)
				}
			}
			time.Sleep(time.Millisecond * 100)
		}
	}()
}

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

func bombardWithTransactions(client *ethclient.Client, key *ecdsa.PrivateKey, pauseDuration time.Duration) {
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
				time.Sleep(100 * time.Millisecond)
				continue
			}
			nonce = newNonce
			shouldRefetchNonce = false
		}

		// Send single transaction
		var data []byte
		tx := types.NewTransaction(nonce, to, big.NewInt(int64(nonce)), gasLimit, gasPrice, data)

		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), key)
		if err != nil {
			log.Printf("failed to sign transaction: %v", err)
			time.Sleep(pauseDuration)
			continue
		}

		err = client.SendTransaction(context.Background(), signedTx)
		if err != nil {
			if lastError == "" {
				lastError = err.Error()
			}
			errorCount++

			if strings.Contains(err.Error(), "nonce too low") || strings.Contains(err.Error(), "replacement transaction underpriced") {
				//do nothing, nonce will increase naturally
			} else if strings.Contains(err.Error(), "future transaction tries to replace pending") {
				shouldRefetchNonce = true
			} else if strings.Contains(err.Error(), "transaction underpriced") {
				//FIXME: should not happen
			} else if strings.Contains(err.Error(), "already known") {
				//do nothing, we will retry
			} else {
				fmt.Println("This error is not handled: ", err.Error())
				panic(err)
			}
		}

		nonce++

		// Wait for the specified pause duration before sending the next transaction
		time.Sleep(pauseDuration)
	}
}
