package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const fundAmount = float64(100000.0)

// eth address: 0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC
const hardHatKeyStr = "56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027"

func fund(client *ethclient.Client, keys []*ecdsa.PrivateKey, listener *TxListener, batchSize int) error {
	// Handle empty keys array
	if len(keys) == 0 {
		return nil
	}

	// First check which accounts need funding
	accountsToFund := make([]*ecdsa.PrivateKey, 0)
	targetBalance := ToWei(fundAmount)

	for _, key := range keys {
		address := crypto.PubkeyToAddress(key.PublicKey)
		balance, err := client.BalanceAt(context.Background(), address, nil)
		if err != nil {
			return fmt.Errorf("failed to get balance for address %s: %w", address.Hex(), err)
		}

		if balance.Cmp(targetBalance) < 0 {
			accountsToFund = append(accountsToFund, key)
		}
	}

	if len(accountsToFund) == 0 {
		fmt.Println("all accounts already have sufficient balance")
		return nil
	}

	fmt.Printf("funding %d accounts\n", len(accountsToFund))

	batchCount := len(accountsToFund) / batchSize
	// Process full batches
	for i := 0; i < batchCount; i++ {
		batchKeys := accountsToFund[i*batchSize : (i+1)*batchSize]
		err := fundBatch(client, batchKeys, listener)
		if err != nil {
			return err
		}
	}

	// Process remaining keys
	remainingKeys := accountsToFund[batchCount*batchSize:]
	if len(remainingKeys) > 0 {
		err := fundBatch(client, remainingKeys, listener)
		if err != nil {
			return err
		}
	}

	fmt.Println("all accounts funded")
	return nil
}

func fundBatch(client *ethclient.Client, keys []*ecdsa.PrivateKey, listener *TxListener) error {
	privateKey, err := crypto.HexToECDSA(hardHatKeyStr)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return fmt.Errorf("failed to get nonce: %w", err)
	}

	gasLimit := uint64(21000)
	// Fund with double the amount
	value := ToWei(fundAmount * 2)
	gasPrice := big.NewInt(1000000001 * 100)

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get chain ID: %w", err)
	}

	signedTxs := make([]*types.Transaction, 0, len(keys))
	for _, key := range keys {
		to := crypto.PubkeyToAddress(key.PublicKey)
		var data []byte
		tx := types.NewTransaction(nonce, to, value, gasLimit, gasPrice, data)

		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
		if err != nil {
			log.Fatal(err)
		}

		signedTxs = append(signedTxs, signedTx)

		nonce++
	}

	// Send all transactions first
	var txHashesMutex sync.Mutex
	txHashes := make([]string, 0, len(signedTxs))
	var wg sync.WaitGroup
	errChan := make(chan error, len(signedTxs))

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

	// Check if any transactions failed to send
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	// Wait for all transactions to be mined
	for _, hash := range txHashes {
		if err := listener.AwaitTxMined(hash, 10); err != nil {
			return fmt.Errorf("transaction failed to mine: %w", err)
		}
	}

	return nil
}
