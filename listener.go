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
	client          *ethclient.Client
	seenTxHashes    map[string]bool
	seenTxHashesLen int
	mu              sync.RWMutex // Add mutex for map access
}

func NewTxListener(client *ethclient.Client) *TxListener {
	return &TxListener{client: client, seenTxHashes: make(map[string]bool), seenTxHashesLen: 0}
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

func (l *TxListener) Start(ctx context.Context) {
	newHeads := make(chan *types.Header)
	sub, err := l.client.SubscribeNewHead(ctx, newHeads)
	if err != nil {
		log.Fatal("failed to subscribe to new heads", "err", err) // if we cannot subscribe, the program is useless, so kill it.
	}

	for {
		select {
		case <-ctx.Done():
			sub.Unsubscribe()
			return
		case err := <-sub.Err():
			log.Fatal("subscription error", "err", err) // if we get an error, the program is useless, so kill it.
		case header := <-newHeads:
			block, err := l.client.BlockByNumber(ctx, header.Number)
			if err != nil {
				log.Println("failed to get block by hash", "err", err)
				continue
			}
			for _, tx := range block.Transactions() {
				l.mu.Lock()
				l.seenTxHashes[tx.Hash().String()] = true
				l.mu.Unlock()
			}

			log.Printf("New block: %v, tx count: %v\n", block.Number(), len(block.Transactions()))
		}
	}
}
