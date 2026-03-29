package config

import "rgb-game/pkg/utils"

// AuthorityConfig holds the raw environment values for resolving the
// minting authority's public key. Actual key loading is done by the
// authority adapter (internal/adapter/authority).
type AuthorityConfig struct {
	PubKeyPath string // AUTHORITY_PUB_KEY_PATH – path to JSON keypair file (preferred)
	PubKeyHex  string // AUTHORITY_PUB_KEY      – hex-encoded ed25519 public key (fallback)
}

func InitAuthorityConfig() *AuthorityConfig {
	return &AuthorityConfig{
		PubKeyPath: utils.GetEnv("AUTHORITY_PUB_KEY_PATH"),
		PubKeyHex:  utils.GetEnv("AUTHORITY_PUB_KEY"),
	}
}
