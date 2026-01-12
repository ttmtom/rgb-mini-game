package ledger

import (
	"rgb-game/pkg/logger"
	"rgb-game/pkg/utils"

	"github.com/joho/godotenv"
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
