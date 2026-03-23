package config

import (
	"rgb-game/pkg/logger"
	"rgb-game/pkg/utils"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseConfig *DatabaseConfig
	ServerConfig   *ServerConfig
}

// LedgerConfig holds all configuration for the Ledger binary.
type LedgerConfig struct {
	DatabaseConfig  *DatabaseConfig
	ServerConfig    *ServerConfig
	AuthorityPubKey string // hex-encoded ed25519 public key of the minting authority
}

func loadEnv() {
	if utils.GetEnv("APP_ENV", "dev") == "dev" {
		if err := godotenv.Load(); err != nil {
			logger.Fatal("Error loading .env file", "err", err)
		}
	}
}

// InitLedgerConfig builds the full configuration for the Ledger binary,
// including the authority public key used to validate MINT transactions.
func InitLedgerConfig() (*LedgerConfig, error) {
	loadEnv()

	databaseConfig := InitDatabaseConfig()
	serverConfig := InitServerConfig()
	authorityPubKey := utils.GetEnv("AUTHORITY_PUB_KEY")

	return &LedgerConfig{
		DatabaseConfig:  databaseConfig,
		ServerConfig:    serverConfig,
		AuthorityPubKey: authorityPubKey,
	}, nil
}
