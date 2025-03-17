package main

import (
	"crypto/ecdsa"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
)

var tps int
var keyCount int

const timeoutSeconds = 10

func main() {
	// Parse command line arguments
	flag.IntVar(&tps, "tps", 100, "Target transactions per second")
	flag.IntVar(&keyCount, "keys", 600, "Number of private keys to generate")

	var rpcUrlsArg string
	flag.StringVar(&rpcUrlsArg, "rpc", "", "Comma-separated list of RPC URLs")

	flag.Parse()

	fmt.Printf("Starting with target TPS: %d, key count: %d\n", tps, keyCount)

	// Parse RPC URLs from command line
	var rpcUrls []string
	if rpcUrlsArg != "" {
		rpcUrls = strings.Split(rpcUrlsArg, ",")
	} else {
		// No default URLs - require user to provide them
		log.Fatal("No RPC URLs provided. Use -rpc flag to provide at least one URL.\n" +
			"Example: -rpc \"http://127.0.0.1:9650/ext/bc/chainID/rpc\"\n" +
			"Note: WebSocket endpoints (ws:// or wss://) are preferred for better performance.\n" +
			"HTTP endpoints will be automatically converted to WebSocket endpoints.")
	}

	// Filter out empty RPC URLs
	filteredUrls := make([]string, 0)
	for _, url := range rpcUrls {
		if url != "" {
			filteredUrls = append(filteredUrls, url)
		}
	}
	rpcUrls = filteredUrls

	// Check if there's at least one RPC URL
	if len(rpcUrls) == 0 {
		log.Fatal("No valid RPC URLs provided. Use -rpc flag to provide at least one URL.")
	}

	fmt.Printf("Using %d RPC URLs\n", len(rpcUrls))

	// Convert HTTP URLs to WebSocket URLs
	for i, rpcUrl := range rpcUrls {
		if strings.HasPrefix(rpcUrl, "http") {
			rpcUrl = strings.Replace(rpcUrl, "http", "ws", 1)
		}
		if strings.HasSuffix(rpcUrl, "/rpc") {
			rpcUrl = strings.Replace(rpcUrl, "/rpc", "/ws", 1)
		}
		rpcUrls[i] = rpcUrl
	}

	// Initialize clients
	clients := make([]*ethclient.Client, len(rpcUrls))
	for i, rpcUrl := range rpcUrls {
		client, err := ethclient.Dial(rpcUrl)
		if err != nil {
			log.Fatal("failed to create tx listener", "err", err)
		}
		clients[i] = client
	}

	startGasPriceMonitor(clients[0])

	keys := mustGenPrivateKeys(keyCount)

	err := fund(clients[0], keys, 50)
	if err != nil {
		log.Fatalf("failed to fund: %v", err)
	}

	// Calculate pause between transactions based on TPS and key count
	// For example, if we want 100 TPS across 50 accounts, each account should pause for 500ms
	pauseMillis := int64(1000 * float64(keyCount) / float64(tps))
	pauseDuration := time.Duration(pauseMillis) * time.Millisecond

	fmt.Printf("Each account will send a transaction every %v\n", pauseDuration)

	delayBetweenAccounts := 10000 / keyCount

	clientNumber := 0
	for _, key := range keys {
		go func(key *ecdsa.PrivateKey, clientNum int) {
			bombardWithTransactions(clients[clientNum%len(clients)], key, pauseDuration)
		}(key, clientNumber)
		clientNumber++

		// Slight delay between starting goroutines to distribute load
		time.Sleep(time.Duration(delayBetweenAccounts) * time.Millisecond)
	}

	// Wait indefinitely
	select {}
}
