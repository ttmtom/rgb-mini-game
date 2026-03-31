package config

import (
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
	return &DatabaseConfig{
		URL:      utils.GetEnv("POSTGRES_URL", ""),
		DBName:   utils.GetEnv("POSTGRES_NAME"),
		Host:     utils.GetEnv("POSTGRES_HOST"),
		Port:     utils.GetEnv("POSTGRES_PORT"),
		Password: utils.GetEnv("POSTGRES_PASSWORD"),
		User:     utils.GetEnv("POSTGRES_USER"),
		SSLMode:  utils.GetEnv("POSTGRES_SSL_MODE"),
	}
}
