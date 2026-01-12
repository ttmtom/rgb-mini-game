package ledger

import (
	"fmt"
	"rgb-game/pkg/utils"
)

type DatabaseConfig struct {
	URL      string
	User     string
	Password string
	Host     string
	Port     string
	DBName   string
	SSLMode  string
}

func InitDatabaseConfig() *DatabaseConfig {
	c := &DatabaseConfig{
		URL:      utils.GetEnv("POSTGRES_URL"),
		DBName:   utils.GetEnv("POSTGRES_NAME"),
		Host:     utils.GetEnv("POSTGRES_HOST"),
		Port:     utils.GetEnv("POSTGRES_PORT"),
		Password: utils.GetEnv("POSTGRES_PASSWORD"),
		User:     utils.GetEnv("POSTGRES_USER"),
		SSLMode:  utils.GetEnv("POSTGRES_SSL_MODE"),
	}

	if c.URL == "" {
		c.URL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			c.User,
			c.Password,
			c.Host,
			c.Port,
			c.DBName,
			c.SSLMode,
		)
	}

	return c
}
