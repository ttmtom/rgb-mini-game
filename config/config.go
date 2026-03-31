package config

import (
	"rgb-game/pkg/logger"
	"rgb-game/pkg/utils"

	"github.com/joho/godotenv"
)

// LedgerConfig holds all configuration for the Ledger binary.
type LedgerConfig struct {
	DatabaseConfig  *DatabaseConfig
	ServerConfig    *ServerConfig
	AuthorityConfig *AuthorityConfig
}

func loadEnv() {
	if utils.GetEnv("APP_ENV", "dev") == "dev" {
		if err := godotenv.Load(); err != nil {
			logger.Fatal("Error loading .env file", "err", err)
		}
	}
}

// InitLedgerConfig builds the full configuration for the Ledger binary.
func InitLedgerConfig() (*LedgerConfig, error) {
	loadEnv()

	return &LedgerConfig{
		DatabaseConfig:  InitDatabaseConfig(),
		ServerConfig:    InitServerConfig(),
		AuthorityConfig: InitAuthorityConfig(),
	}, nil
}

// GameServerFullConfig holds all configuration for the Game Server binary.
type GameServerFullConfig struct {
	GameServerConfig *GameServerConfig
	GameConfig       *GameConfig
	RedisConfig      *RedisConfig
}

// InitGameServerFullConfig builds the full configuration for the Game Server binary.
func InitGameServerFullConfig() (*GameServerFullConfig, error) {
	loadEnv()

	return &GameServerFullConfig{
		GameServerConfig: InitGameServerConfig(),
		GameConfig:       InitGameConfig(),
		RedisConfig:      InitRedisConfig(),
	}, nil
}

// PlayerFullConfig holds all configuration for the Player CLI binary.
type PlayerFullConfig struct {
	PlayerConfig *PlayerConfig
}

// InitPlayerFullConfig builds the full configuration for the Player CLI binary.
func InitPlayerFullConfig() (*PlayerFullConfig, error) {
	loadEnv()

	return &PlayerFullConfig{
		PlayerConfig: InitPlayerConfig(),
	}, nil
}
