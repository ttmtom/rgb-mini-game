package authority

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"rgb-game/config"
	"rgb-game/pkg/crypto"
)

// Load resolves the ed25519 public key of the minting authority from the
// provided AuthorityConfig.
//
// Resolution order (first non-empty wins):
//  1. PubKeyPath – path to a JSON keypair file produced by crypto.LoadOrGenerateKey;
//     preferred when sharing the key via a Docker volume.
//  2. PubKeyHex  – raw hex-encoded ed25519 public key (fallback for simple deployments).
func Load(cfg *config.AuthorityConfig) (ed25519.PublicKey, error) {
	switch {
	case cfg.PubKeyPath != "":
		kp, err := crypto.LoadOrGenerateKey(cfg.PubKeyPath)
		if err != nil {
			return nil, fmt.Errorf("authority: failed to load keypair from %q: %w", cfg.PubKeyPath, err)
		}
		return kp.PublicKey, nil

	case cfg.PubKeyHex != "":
		pubKey, err := hex.DecodeString(cfg.PubKeyHex)
		if err != nil {
			return nil, fmt.Errorf("authority: invalid AUTHORITY_PUB_KEY (must be hex-encoded ed25519 public key): %w", err)
		}
		return pubKey, nil

	default:
		return nil, fmt.Errorf("authority: one of AUTHORITY_PUB_KEY_PATH or AUTHORITY_PUB_KEY must be set")
	}
}
