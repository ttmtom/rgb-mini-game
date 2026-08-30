package config

import (
	"rgb-game/pkg/logger"
	"rgb-game/pkg/utils"
	"strconv"
	"time"
)

// BlockSealerConfig holds tunable parameters for the block-sealing background service.
type BlockSealerConfig struct {
	IntervalSeconds int
}

// InitBlockSealerConfig loads BlockSealerConfig from environment variables.
func InitBlockSealerConfig() *BlockSealerConfig {
	intervalStr := utils.GetEnv("BLOCK_INTERVAL_SECONDS", "10")
	interval, err := strconv.Atoi(intervalStr)
	if err != nil {
		logger.Fatal("Invalid BLOCK_INTERVAL_SECONDS")
	}
	return &BlockSealerConfig{IntervalSeconds: interval}
}

// Interval returns IntervalSeconds as a time.Duration.
func (c *BlockSealerConfig) Interval() time.Duration {
	return time.Duration(c.IntervalSeconds) * time.Second
}
