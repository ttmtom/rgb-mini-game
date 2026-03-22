package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

type Keypair struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

type keypairJSON struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

func GenerateKeypair() (*Keypair, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Keypair{
		PrivateKey: privKey,
		PublicKey:  pubKey,
	}, nil
}

func Sign(privKey ed25519.PrivateKey, data []byte) []byte {
	return ed25519.Sign(privKey, data)
}

func Verify(pubKey ed25519.PublicKey, data, sig []byte) bool {
	return ed25519.Verify(pubKey, data, sig)
}

func PubKeyToPlayerID(pubKey ed25519.PublicKey) string {
	hash := sha256.Sum256(pubKey)
	return hex.EncodeToString(hash[:])
}

func LoadOrGenerateKey(path string) (*Keypair, error) {
	path = expandHome(path)

	if raw, err := os.ReadFile(path); err == nil {
		var kj keypairJSON
		if err := json.Unmarshal(raw, &kj); err != nil {
			return nil, err
		}
		privBytes, err := hex.DecodeString(kj.PrivateKey)
		if err != nil {
			return nil, err
		}
		pubBytes, err := hex.DecodeString(kj.PublicKey)
		if err != nil {
			return nil, err
		}
		return &Keypair{
			PrivateKey: ed25519.PrivateKey(privBytes),
			PublicKey:  ed25519.PublicKey(pubBytes),
		}, nil
	}

	kp, err := GenerateKeypair()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}

	kj := keypairJSON{
		PrivateKey: hex.EncodeToString(kp.PrivateKey),
		PublicKey:  hex.EncodeToString(kp.PublicKey),
	}
	data, err := json.MarshalIndent(kj, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return nil, err
	}

	return kp, nil
}

func expandHome(path string) string {
	if len(path) == 0 || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
