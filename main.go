package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

// Configuration
const (
	scanDepth     = 100 // Number of blocks to scan from the current block downward
	minMsgLength  = 4   // Minimum message length to consider
	minWordLength = 3   // Minimum word length in valid message
	minWords      = 2   // Minimum words in valid message
	letterRatio   = 0.6 // Minimum ratio of letters in valid message
)

var (
	// Common Ethereum function signatures (first 4 bytes of keccak256 hash)
	functionSignatures = map[string]string{
		"a9059cbb": "ERC20 transfer",
		"23b872dd": "ERC20 transferFrom",
		"095ea7b3": "ERC20 approve",
		"42842e0e": "ERC721 safeTransferFrom",
		"b88d4fde": "ERC721 safeTransferFrom with data",
		"a22cb465": "setApprovalForAll",
		"6352211e": "ownerOf (ERC721)",
		"70a08231": "balanceOf",
		"06fdde03": "name()",
		"95d89b41": "symbol()",
	}
)

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	infuraKey := os.Getenv("INFURA_KEY")
	if infuraKey == "" {
		log.Fatal("INFURA_KEY not found in .env file")
	}

	client, err := ethclient.Dial(fmt.Sprintf("wss://mainnet.infura.io/ws/v3/%s", infuraKey))
	if err != nil {
		log.Fatal("Connection error:", err)
	}

	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Fatal("Block header error:", err)
	}

	endBlock := header.Number.Int64()
	startBlock := endBlock - scanDepth

	// Compile a regex to match candidate messages.
	msgPattern := regexp.MustCompile(fmt.Sprintf(`[\p{L}\p{N}\s]{%d,}`, minMsgLength))
	msgPattern.Longest()

	// Count down from the current block to the startBlock.
	for blockNum := endBlock; blockNum >= startBlock; blockNum-- {
		processBlock(client, blockNum, msgPattern)
		time.Sleep(250 * time.Millisecond)
	}
}

// processBlock fetches the block and groups valid transactions (with messages)
// so that the block header is printed only once.
func processBlock(client *ethclient.Client, blockNum int64, pattern *regexp.Regexp) {
	block, err := client.BlockByNumber(context.Background(), big.NewInt(blockNum))
	if err != nil {
		log.Printf("Block %d fetch error: %v", blockNum, err)
		return
	}

	// Accumulate output for all transactions in this block.
	var blockOutputs []string
	for _, tx := range block.Transactions() {
		validMessages := analyzeTransaction(tx, pattern)
		if len(validMessages) > 0 {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Tx: %s\nPossible messages:\n", tx.Hash().Hex()))
			for _, msg := range validMessages {
				sb.WriteString(fmt.Sprintf("  - %q\n", msg))
			}
			blockOutputs = append(blockOutputs, sb.String())
		}
	}

	// If any transaction in this block contained a valid message, print them.
	if len(blockOutputs) > 0 {
		fmt.Printf("\nBlock %d\n", blockNum)
		for _, out := range blockOutputs {
			fmt.Println(out)
		}
	}
}

// analyzeTransaction checks a transaction’s data and returns valid messages, if any.
func analyzeTransaction(tx *types.Transaction, pattern *regexp.Regexp) []string {
	data := tx.Data()
	// Skip transactions with no data or known contract call signatures.
	if len(data) == 0 || isContractCall(data) {
		return nil
	}

	utf8Data := decodeUTF8(data)
	matches := pattern.FindAllString(utf8Data, -1)
	if len(matches) == 0 {
		return nil
	}

	var validMessages []string
	for _, msg := range matches {
		if isValidMessage(msg) {
			validMessages = append(validMessages, msg)
		}
	}
	return validMessages
}

// isContractCall checks if the first 4 bytes of data match a known function signature.
func isContractCall(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	sig := hex.EncodeToString(data[:4])
	_, exists := functionSignatures[sig]
	return exists
}

// decodeUTF8 decodes a byte slice into a cleaned-up UTF-8 string.
func decodeUTF8(data []byte) string {
	var sb strings.Builder
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError {
			data = data[1:]
			continue
		}
		if unicode.IsPrint(r) {
			sb.WriteRune(r)
		}
		data = data[size:]
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

// isValidMessage applies our heuristics (letter ratio and valid words) to the message.
func isValidMessage(s string) bool {
	words := strings.Fields(s)
	if len(words) < minWords {
		return false
	}

	letterCount := 0
	totalChars := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letterCount++
		}
		if !unicode.IsSpace(r) {
			totalChars++
		}
	}

	return float64(letterCount)/float64(totalChars) >= letterRatio &&
		hasValidWords(words)
}

// hasValidWords requires that each word is at least minWordLength, contains letters,
// and (with our extra heuristic) includes at least one vowel.
func hasValidWords(words []string) bool {
	validWords := 0
	for _, word := range words {
		if len(word) >= minWordLength && hasLetters(word) && hasVowel(word) {
			validWords++
		}
	}
	return validWords >= minWords
}

// hasLetters checks if there is at least one letter in the string.
func hasLetters(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// hasVowel returns true if the string contains at least one vowel (a, e, i, o, u).
func hasVowel(s string) bool {
	for _, r := range s {
		switch unicode.ToLower(r) {
		case 'a', 'e', 'i', 'o', 'u':
			return true
		}
	}
	return false
}
