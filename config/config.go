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

func InitLedgerServerConfig() (*Config, error) {
	if utils.GetEnv("APP_ENV", "dev") == "dev" {
		if err := godotenv.Load(); err != nil {
			logger.Fatal("Error loading .env file", "err", err)
		}
	}

	databaseConfig := InitDatabaseConfig()
	serverConfig := InitServerConfig()

	return &Config{databaseConfig, serverConfig}, nil
}
