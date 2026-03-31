package config

import "rgb-game/pkg/utils"

// PlayerConfig holds all configuration for the Player CLI binary.
type PlayerConfig struct {
	PlayerKeyPath string
	LedgerAddr    string
	ServerAddr    string
}

func InitPlayerConfig() *PlayerConfig {
	return &PlayerConfig{
		PlayerKeyPath: utils.GetEnv("PLAYER_KEY_PATH", ".key/player_ed25519"),
		LedgerAddr:    utils.GetEnv("LEDGER_ADDR", "localhost:50051"),
		ServerAddr:    utils.GetEnv("SERVER_ADDR", "localhost:50052"),
	}
}
