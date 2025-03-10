package main

import (
	"fmt"
	"math/big"
	"strings"
)

func ToWei(amount float64) *big.Int {
	const decimals = 18

	wei := new(big.Int)

	// Convert float to string with maximum precision
	amountStr := fmt.Sprintf("%.18f", amount)

	// Remove decimal point and trailing zeros
	amountStr = strings.Replace(amountStr, ".", "", 1)
	amountStr = strings.TrimRight(amountStr, "0")

	// Parse string to big.Int
	wei.SetString(amountStr, 10)

	// Adjust for decimals
	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	wei.Mul(wei, multiplier)

	return wei
}
