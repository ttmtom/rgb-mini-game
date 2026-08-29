package postgres

import (
	"fmt"
	"rgb-game/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Postgres wraps a *gorm.DB connection.
type Postgres struct {
	db *gorm.DB
}

// Init opens a Postgres connection using individual DSN components.
func Init(config *config.DatabaseConfig) (*Postgres, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.DBName,
		config.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &Postgres{db: db}, nil
}

// DB returns the underlying *gorm.DB (used for wiring only — never passed to ports).
func (p *Postgres) DB() *gorm.DB {
	return p.db
}
