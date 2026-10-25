package crypto_test

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"

	"rgb-game/pkg/crypto"
)

func TestGenerateKeypair(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error: %v", err)
	}
	if len(kp.PublicKey) != ed25519.PublicKeySize {
		t.Errorf("PublicKey size want %d got %d", ed25519.PublicKeySize, len(kp.PublicKey))
	}
	if len(kp.PrivateKey) != ed25519.PrivateKeySize {
		t.Errorf("PrivateKey size want %d got %d", ed25519.PrivateKeySize, len(kp.PrivateKey))
	}
}

func TestGenerateKeypair_UniqueEachCall(t *testing.T) {
	kp1, _ := crypto.GenerateKeypair()
	kp2, _ := crypto.GenerateKeypair()
	if string(kp1.PublicKey) == string(kp2.PublicKey) {
		t.Error("two generated keypairs should not have the same public key")
	}
}

func TestSignAndVerify_RoundTrip(t *testing.T) {
	kp, _ := crypto.GenerateKeypair()
	msg := []byte("hello rgb-game")

	sig := crypto.Sign(kp.PrivateKey, msg)
	if !crypto.Verify(kp.PublicKey, msg, sig) {
		t.Error("Verify should return true for a valid signature")
	}
}

func TestVerify_TamperedMessage(t *testing.T) {
	kp, _ := crypto.GenerateKeypair()
	msg := []byte("original message")
	sig := crypto.Sign(kp.PrivateKey, msg)

	tampered := []byte("tampered message")
	if crypto.Verify(kp.PublicKey, tampered, sig) {
		t.Error("Verify should return false for a tampered message")
	}
}

func TestVerify_TamperedSignature(t *testing.T) {
	kp, _ := crypto.GenerateKeypair()
	msg := []byte("some data")
	sig := crypto.Sign(kp.PrivateKey, msg)

	// Flip one byte in the signature.
	sig[0] ^= 0xFF
	if crypto.Verify(kp.PublicKey, msg, sig) {
		t.Error("Verify should return false for a tampered signature")
	}
}

func TestVerify_WrongKey(t *testing.T) {
	kp1, _ := crypto.GenerateKeypair()
	kp2, _ := crypto.GenerateKeypair()
	msg := []byte("data")
	sig := crypto.Sign(kp1.PrivateKey, msg)

	if crypto.Verify(kp2.PublicKey, msg, sig) {
		t.Error("Verify should return false when checked with a different public key")
	}
}

func TestPubKeyToPlayerID_Deterministic(t *testing.T) {
	kp, _ := crypto.GenerateKeypair()
	id1 := crypto.PubKeyToPlayerID(kp.PublicKey)
	id2 := crypto.PubKeyToPlayerID(kp.PublicKey)
	if id1 != id2 {
		t.Errorf("PubKeyToPlayerID should be deterministic: %q vs %q", id1, id2)
	}
}

func TestPubKeyToPlayerID_UniquePerKey(t *testing.T) {
	kp1, _ := crypto.GenerateKeypair()
	kp2, _ := crypto.GenerateKeypair()
	if crypto.PubKeyToPlayerID(kp1.PublicKey) == crypto.PubKeyToPlayerID(kp2.PublicKey) {
		t.Error("different public keys should produce different player IDs")
	}
}

func TestPubKeyToPlayerID_Length(t *testing.T) {
	kp, _ := crypto.GenerateKeypair()
	id := crypto.PubKeyToPlayerID(kp.PublicKey)
	// hex(sha256) = 64 hex characters
	if len(id) != 64 {
		t.Errorf("player ID length want 64 got %d", len(id))
	}
}

func TestLoadOrGenerateKey_GeneratesAndPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "id_ed25519")

	kp, err := crypto.LoadOrGenerateKey(path)
	if err != nil {
		t.Fatalf("LoadOrGenerateKey (generate) error: %v", err)
	}
	if len(kp.PublicKey) != ed25519.PublicKeySize {
		t.Error("generated key has wrong public key size")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected key file to be written to disk")
	}
}

func TestLoadOrGenerateKey_LoadsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "id_ed25519")

	kp1, _ := crypto.LoadOrGenerateKey(path)
	kp2, err := crypto.LoadOrGenerateKey(path)
	if err != nil {
		t.Fatalf("LoadOrGenerateKey (load) error: %v", err)
	}
	if string(kp1.PublicKey) != string(kp2.PublicKey) {
		t.Error("reloaded key should match the originally generated key")
	}
}
