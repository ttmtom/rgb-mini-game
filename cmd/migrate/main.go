package main

import (
	"rgb-game/config"
	"rgb-game/internal/adapter/postgres"
	"rgb-game/internal/adapter/postgres/migrations"
	"rgb-game/pkg/logger"
)

func main() {
	logger.Init()

	logger.Info("Initializing migration config")
	cfg, err := config.InitLedgerConfig()
	if err != nil {
		logger.Fatalf("failed to initialize config: %v", err)
	}

	logger.Info("Connecting to Postgres")
	pg, err := postgres.Init(cfg.DatabaseConfig)
	if err != nil {
		logger.Fatalf("failed to connect to Postgres: %v", err)
	}

	if err := migrations.AutoMigrate(pg.DB()); err != nil {
		logger.Fatalf("migration failed: %v", err)
	}

	logger.Info("Migrations completed successfully")
}
