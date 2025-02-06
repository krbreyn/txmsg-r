package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

// Configuration
const (
	scanDepth      = 100 // Number of blocks to scan from latest
	minMsgLength   = 4   // Minimum message length to consider
	asciiPrintable = 32  // Starting ASCII code for printable characters
	asciiMax       = 126 // Maximum ASCII code for printable characters
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	infuraKey := os.Getenv("INFURA_KEY")
	if infuraKey == "" {
		log.Fatal("INFURA_KEY not found in .env file")
	}

	infuraURL := fmt.Sprintf("wss://mainnet.infura.io/ws/v3/%s", infuraKey)

	// Connect to Ethereum node
	client, err := ethclient.Dial(infuraURL)
	if err != nil {
		log.Fatal("Connection error:", err)
	}

	// Get latest block number
	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Fatal("Block header error:", err)
	}
	endBlock := header.Number.Int64()
	startBlock := endBlock - scanDepth

	// Create message pattern detector
	msgPattern := regexp.MustCompile(fmt.Sprintf("[a-zA-Z0-9 ]{%d,}", minMsgLength))

	// Iterate through blocks
	for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
		time.Sleep(100 * time.Millisecond)
		processBlock(client, blockNum, msgPattern)
	}
}

func processBlock(client *ethclient.Client, blockNum int64, pattern *regexp.Regexp) {
	block, err := client.BlockByNumber(context.Background(), big.NewInt(blockNum))
	if err != nil {
		log.Printf("Block %d fetch error: %v", blockNum, err)
		return
	}

	// Analyze each transaction
	for _, tx := range block.Transactions() {
		analyzeTransaction(tx, blockNum, pattern)
	}
}

func analyzeTransaction(tx *types.Transaction, blockNum int64, pattern *regexp.Regexp) {
	data := tx.Data()
	if len(data) == 0 {
		return
	}

	// Convert data to printable format
	cleanData := filterPrintable(data)
	if matches := pattern.FindAllString(cleanData, -1); len(matches) > 0 {
		printResults(tx, blockNum, matches)
	}
}

func filterPrintable(data []byte) string {
	var sb strings.Builder
	for _, b := range data {
		if b >= asciiPrintable && b <= asciiMax {
			sb.WriteByte(b)
		} else {
			sb.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

func printResults(tx *types.Transaction, blockNum int64, messages []string) {
	var hasShownInfo bool
	for _, msg := range messages {
		if isMeaningful(msg) {
			if !hasShownInfo {
				fmt.Printf("\nBlock %d | Tx: %s\n", blockNum, tx.Hash().Hex())
				fmt.Println("From:", tx.To().Hex())
				fmt.Println("Possible messages:")
				hasShownInfo = true
			}
			fmt.Println("  -", msg)
		}
	}
}

const (
	minWordLength = 3
	minWords      = 2
)

func isMeaningful(s string) bool {
	words := strings.Fields(s)
	if len(words) < minWords {
		return false
	}

	validWords := 0
	for _, word := range words {
		if len(word) >= minWordLength && hasLetters(word) {
			validWords++
		}
	}

	return validWords >= minWords
}

func hasLetters(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}
