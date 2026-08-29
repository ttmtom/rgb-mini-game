package redis

import (
	"context"
	"fmt"
	"rgb-game/config"
	"rgb-game/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// Init creates a Redis client, pings the server, and returns the client.
func Init(cfg *config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	logger.Infof("Redis connected at %s (db=%d)", cfg.Addr, cfg.DB)
	return client, nil
}
