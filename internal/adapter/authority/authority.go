package authority

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"rgb-game/config"
	"rgb-game/pkg/crypto"
)

// Authority holds the minting authority's identity and optional signing key.
// Use NewPublicAuthority for read-only access (Ledger),
// or NewFullAuthority for full signing capability (Game Server).
type Authority struct {
	pubKey   ed25519.PublicKey
	playerID string
	privKey  ed25519.PrivateKey // nil when constructed via NewPublicAuthority
}

// NewPublicAuthority creates a pub-only Authority (no signing capability).
func NewPublicAuthority(pubKey ed25519.PublicKey) *Authority {
	return &Authority{
		pubKey:   pubKey,
		playerID: crypto.PubKeyToPlayerID(pubKey),
	}
}

// NewFullAuthority creates an Authority with full signing capability.
func NewFullAuthority(kp *crypto.Keypair) *Authority {
	return &Authority{
		pubKey:   kp.PublicKey,
		playerID: crypto.PubKeyToPlayerID(kp.PublicKey),
		privKey:  kp.PrivateKey,
	}
}

// PubKey returns the ed25519 public key.
func (a *Authority) PubKey() ed25519.PublicKey { return a.pubKey }

// PlayerID returns the hex-encoded SHA-256 of the public key.
func (a *Authority) PlayerID() string { return a.playerID }

// Sign signs data with the private key.
// Panics if this Authority was constructed without a private key.
func (a *Authority) Sign(data []byte) []byte {
	if a.privKey == nil {
		panic("authority: Sign called on a public-only Authority (no private key loaded)")
	}
	return crypto.Sign(a.privKey, data)
}

// Load resolves the minting authority from AuthorityConfig and returns a
// public-only Authority (suitable for Ledger — no private key).
//
// Resolution order (first non-empty wins):
//  1. PubKeyPath – path to a JSON keypair file produced by crypto.LoadOrGenerateKey.
//  2. PubKeyHex  – raw hex-encoded ed25519 public key.
func Load(cfg *config.AuthorityConfig) (*Authority, error) {
	switch {
	case cfg.PubKeyPath != "":
		kp, err := crypto.LoadOrGenerateKey(cfg.PubKeyPath)
		if err != nil {
			return nil, fmt.Errorf("authority: failed to load keypair from %q: %w", cfg.PubKeyPath, err)
		}
		return NewPublicAuthority(kp.PublicKey), nil

	case cfg.PubKeyHex != "":
		pubBytes, err := hex.DecodeString(cfg.PubKeyHex)
		if err != nil {
			return nil, fmt.Errorf("authority: invalid AUTHORITY_PUB_KEY (must be hex-encoded ed25519 public key): %w", err)
		}
		return NewPublicAuthority(ed25519.PublicKey(pubBytes)), nil

	default:
		return nil, fmt.Errorf("authority: one of AUTHORITY_PUB_KEY_PATH or AUTHORITY_PUB_KEY must be set")
	}
}
