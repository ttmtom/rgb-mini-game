package main

import (
	"rgb-game/config"
	drivenpostgres "rgb-game/internal/adapter/driven/postgres"
	"rgb-game/internal/adapter/driven/postgres/repositories"
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
	pg, err := drivenpostgres.Init(cfg.DatabaseConfig)
	if err != nil {
		logger.Fatalf("failed to connect to Postgres: %v", err)
	}

	if err := repositories.AutoMigrate(pg.DB()); err != nil {
		logger.Fatalf("migration failed: %v", err)
	}

	logger.Info("Migrations completed successfully")
}
