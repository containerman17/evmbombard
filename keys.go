package main

import (
	"bufio"
	"crypto/ecdsa"
	"encoding/hex"
	"log"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

func mustGenPrivateKeys(count int) []*ecdsa.PrivateKey {
	const keysFile = ".keys.txt"
	keys := []*ecdsa.PrivateKey{}

	// Try to read existing keys from file
	existingKeys := readKeysFromFile(keysFile)
	keys = append(keys, existingKeys...)

	// Generate additional keys if needed
	for len(keys) < count {
		key, err := crypto.GenerateKey()
		if err != nil {
			log.Fatal("failed to generate key", "err", err)
		}
		keys = append(keys, key)
	}

	// Save all keys to file
	saveKeysToFile(keys, keysFile)

	// Return only the requested number of keys
	return keys[:count]
}

func readKeysFromFile(filename string) []*ecdsa.PrivateKey {
	keys := []*ecdsa.PrivateKey{}

	file, err := os.Open(filename)
	if err != nil {
		// File doesn't exist or can't be opened, return empty slice
		return keys
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Convert hex string to private key
		privateKeyBytes, err := hex.DecodeString(line)
		if err != nil {
			log.Printf("skipping invalid key in file: %v", err)
			continue
		}

		privateKey, err := crypto.ToECDSA(privateKeyBytes)
		if err != nil {
			log.Printf("skipping invalid key in file: %v", err)
			continue
		}

		keys = append(keys, privateKey)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("error reading keys file: %v", err)
	}

	return keys
}

func saveKeysToFile(keys []*ecdsa.PrivateKey, filename string) {
	file, err := os.Create(filename)
	if err != nil {
		log.Printf("failed to save keys to file: %v", err)
		return
	}
	defer file.Close()

	for _, key := range keys {
		privateKeyBytes := crypto.FromECDSA(key)
		hexKey := hex.EncodeToString(privateKeyBytes)
		if _, err := file.WriteString(hexKey + "\n"); err != nil {
			log.Printf("error writing key to file: %v", err)
		}
	}
}
