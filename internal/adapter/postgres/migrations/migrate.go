package migrations

import (
	"rgb-game/internal/adapter/postgres/repositories"
	"rgb-game/pkg/logger"

	"gorm.io/gorm"
)

// AutoMigrate runs GORM auto-migration for all persistent models.
func AutoMigrate(db *gorm.DB) error {
	logger.Info("Running auto-migrations...")
	return db.AutoMigrate(
		&repositories.PlayerModel{},
		&repositories.TransactionModel{},
	)
}
