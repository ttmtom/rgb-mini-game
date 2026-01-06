package config

import (
	"github.com/joho/godotenv"
	"rgb.game/ledger/pkg/logger"
	"rgb.game/ledger/pkg/utils"
)

type Config struct {
	DatabaseConfig *DatabaseConfig
}

func Init() (*Config, error) {
	if utils.GetEnv("APP_ENV", "dev") == "dev" {
		if err := godotenv.Load(); err != nil {
			logger.Fatalf("Error loading .env file", "err", err)
		}
	}

	databaseConfig := InitDatabaseConfig()

	return &Config{databaseConfig}, nil
}
