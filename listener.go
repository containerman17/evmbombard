// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type TxListener struct {
	clients          []*ethclient.Client
	seenTxHashes     map[string]bool
	seenTxHashesLen  int
	seenBlockNumbers map[uint64]bool // Track seen block numbers
	mu               sync.RWMutex    // Add mutex for map access
}

func NewTxListener(clients []*ethclient.Client) *TxListener {
	return &TxListener{
		clients:          clients,
		seenTxHashes:     make(map[string]bool),
		seenTxHashesLen:  0,
		seenBlockNumbers: make(map[uint64]bool),
	}
}

func (l *TxListener) AwaitTxMined(txHash string, timeoutSeconds int) error {
	if l.checkTxSeen(txHash) {
		return nil
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	timeout := time.After(time.Duration(timeoutSeconds) * time.Second)

	for {
		select {
		case <-ticker.C:
			if l.checkTxSeen(txHash) {
				return nil
			}
		case <-timeout:
			return fmt.Errorf("timeout waiting for transaction %s after %d seconds", txHash, timeoutSeconds)
		}
	}
}

// Add helper method for safe map reading
func (l *TxListener) checkTxSeen(txHash string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.seenTxHashes[txHash]
}

// Helper to mark a block as seen
func (l *TxListener) markBlockSeen(blockNum uint64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.seenBlockNumbers[blockNum] {
		return false // Block already seen
	}

	l.seenBlockNumbers[blockNum] = true
	return true // First time seeing this block
}

func (l *TxListener) Start(ctx context.Context) {
	if len(l.clients) == 0 {
		log.Fatal("no clients provided")
	}

	// Create a channel to collect headers from all clients
	headerCh := make(chan *types.Header, 100)

	// Start a goroutine for each client to subscribe to new blocks
	var wg sync.WaitGroup
	for i, client := range l.clients {
		wg.Add(1)
		go func(idx int, cl *ethclient.Client) {
			defer wg.Done()

			newHeads := make(chan *types.Header)
			sub, err := cl.SubscribeNewHead(ctx, newHeads)
			if err != nil {
				log.Printf("failed to subscribe to new heads for client %d: %v", idx, err)
				return
			}
			defer sub.Unsubscribe()

			for {
				select {
				case <-ctx.Done():
					return
				case err := <-sub.Err():
					log.Printf("subscription error from client %d: %v", idx, err)
					return
				case header := <-newHeads:
					// Forward the header to the main channel
					headerCh <- header
				}
			}
		}(i, client)
	}

	// Process headers from the channel
	for {
		select {
		case <-ctx.Done():
			// Wait for all subscriber goroutines to terminate
			wg.Wait()
			return
		case header := <-headerCh:
			blockNum := header.Number.Uint64()

			// Skip if we've already seen this block number
			if !l.markBlockSeen(blockNum) {
				continue
			}

			// Get the block from any client
			var block *types.Block
			var err error

			for _, client := range l.clients {
				block, err = client.BlockByNumber(ctx, header.Number)
				if err == nil {
					break
				}
			}

			if err != nil || block == nil {
				log.Printf("failed to get block %d from any client: %v", blockNum, err)
				continue
			}

			// Process transactions
			l.mu.Lock()
			for _, tx := range block.Transactions() {
				l.seenTxHashes[tx.Hash().String()] = true
			}
			l.mu.Unlock()

			log.Printf("New block: %v, tx count: %v\n", block.Number(), len(block.Transactions()))
		}
	}
}
