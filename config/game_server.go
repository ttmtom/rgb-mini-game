package config

import (
	"rgb-game/pkg/logger"
	"rgb-game/pkg/utils"
	"strconv"
	"time"
)

// GameServerConfig holds server-level infrastructure settings for the Game Server binary.
type GameServerConfig struct {
	Port             int
	LedgerAddr       string
	AuthorityKeyPath string
}

func InitGameServerConfig() *GameServerConfig {
	portStr := utils.GetEnv("GAME_SERVER_GRPC_PORT", "50052")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		logger.Fatal("Invalid GAME_SERVER_GRPC_PORT")
	}

	return &GameServerConfig{
		Port:             port,
		LedgerAddr:       utils.GetEnv("LEDGER_ADDR", "localhost:50051"),
		AuthorityKeyPath: utils.GetEnv("AUTHORITY_KEY_PATH", ".key/id_ed25519"),
	}
}

// GameConfig holds game-logic tunable parameters for the Game Server binary.
type GameConfig struct {
	MissionCooldownSeconds int
}

func InitGameConfig() *GameConfig {
	cooldownStr := utils.GetEnv("MISSION_COOLDOWN_SECONDS", "300")
	cooldown, err := strconv.Atoi(cooldownStr)
	if err != nil {
		logger.Fatal("Invalid MISSION_COOLDOWN_SECONDS")
	}

	return &GameConfig{
		MissionCooldownSeconds: cooldown,
	}
}

// Cooldown returns MissionCooldownSeconds as a time.Duration.
func (c *GameConfig) Cooldown() time.Duration {
	return time.Duration(c.MissionCooldownSeconds) * time.Second
}
