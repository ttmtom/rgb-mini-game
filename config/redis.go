package config

import (
	"rgb-game/pkg/utils"
	"strconv"

	"rgb-game/pkg/logger"
)

// RedisConfig holds connection settings for the Redis server.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

func InitRedisConfig() *RedisConfig {
	dbStr := utils.GetEnv("REDIS_DB", "0")
	db, err := strconv.Atoi(dbStr)
	if err != nil {
		logger.Fatal("Invalid REDIS_DB")
	}

	return &RedisConfig{
		Addr:     utils.GetEnv("REDIS_ADDR", "localhost:6379"),
		Password: utils.GetEnv("REDIS_PASSWORD", ""),
		DB:       db,
	}
}
