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
	Difficulty      uint8 // PoW: number of leading zero hex nibbles required in each block hash
}

// InitBlockSealerConfig loads BlockSealerConfig from environment variables.
func InitBlockSealerConfig() *BlockSealerConfig {
	intervalStr := utils.GetEnv("BLOCK_INTERVAL_SECONDS", "10")
	interval, err := strconv.Atoi(intervalStr)
	if err != nil {
		logger.Fatal("Invalid BLOCK_INTERVAL_SECONDS")
	}

	difficultyStr := utils.GetEnv("BLOCK_DIFFICULTY", "2")
	difficulty, err := strconv.Atoi(difficultyStr)
	if err != nil || difficulty < 0 || difficulty > 64 {
		logger.Fatal("Invalid BLOCK_DIFFICULTY (must be 0-64)")
	}

	return &BlockSealerConfig{IntervalSeconds: interval, Difficulty: uint8(difficulty)}
}

// Interval returns IntervalSeconds as a time.Duration.
func (c *BlockSealerConfig) Interval() time.Duration {
	return time.Duration(c.IntervalSeconds) * time.Second
}
