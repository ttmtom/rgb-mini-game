package postgres

import (
	"fmt"
	"rgb-game/config/ledger"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Postgres struct {
	db *gorm.DB
}

func Init(config *ledger.DatabaseConfig) (*Postgres, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.DBName,
		config.SSLMode,
	)

	PostgresDb, dbOpenErr := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if dbOpenErr != nil {
		return nil, dbOpenErr
	}

	return &Postgres{
		db: PostgresDb,
	}, nil
}

func (p *Postgres) DB() *gorm.DB {
	return p.db
}
