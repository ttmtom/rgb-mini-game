// keygen is a one-shot utility that generates a hex-encoded ed25519 keypair,
// writes the full JSON keypair to .key/id_ed25519, and writes the bare
// hex-encoded public key to .key/id_ed25519.pub.hex.
//
// Usage:
//
//	go run cmd/keygen/main.go
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"rgb-game/pkg/crypto"
)

func main() {
	const (
		keyDir      = ".key"
		keypairPath = ".key/id_ed25519"
		pubHexPath  = ".key/id_ed25519.pub.hex"
	)

	// Ensure the key directory exists with restrictive permissions.
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create key directory: %v\n", err)
		os.Exit(1)
	}

	// Generate (or load existing) keypair.
	kp, err := crypto.LoadOrGenerateKey(keypairPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load/generate keypair: %v\n", err)
		os.Exit(1)
	}

	// Write the bare public-key hex file (convenient for external consumers).
	pubHex := hex.EncodeToString(kp.PublicKey)
	if err := os.WriteFile(pubHexPath, []byte(pubHex+"\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write public key hex file: %v\n", err)
		os.Exit(1)
	}

	// Pretty-print a summary.
	type summary struct {
		KeypairFile   string `json:"keypair_file"`
		PubKeyHexFile string `json:"pub_key_hex_file"`
		PublicKey     string `json:"public_key"`
	}
	out, _ := json.MarshalIndent(summary{
		KeypairFile:   keypairPath,
		PubKeyHexFile: pubHexPath,
		PublicKey:     pubHex,
	}, "", "  ")

	fmt.Println(string(out))
	fmt.Printf("\nSet in your .env:\n  AUTHORITY_PUB_KEY_PATH=%s\n", keypairPath)
}
