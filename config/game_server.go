package config

import (
	"rgb-game/pkg/logger"
	"rgb-game/pkg/utils"
	"strconv"
)

type GameServerConfig struct {
	Port                   int
	LedgerAddr             string
	AuthorityKeyPath       string
	MissionCooldownSeconds int
}

func InitGameServerConfig() *GameServerConfig {
	portStr := utils.GetEnv("GAME_SERVER_GRPC_PORT", "50052")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		logger.Fatal("Invalid GAME_SERVER_GRPC_PORT")
	}

	cooldownStr := utils.GetEnv("MISSION_COOLDOWN_SECONDS", "300")
	cooldown, err := strconv.Atoi(cooldownStr)
	if err != nil {
		logger.Fatal("Invalid MISSION_COOLDOWN_SECONDS")
	}

	return &GameServerConfig{
		Port:                   port,
		LedgerAddr:             utils.GetEnv("LEDGER_ADDR", "localhost:50051"),
		AuthorityKeyPath:       utils.GetEnv("AUTHORITY_KEY_PATH", ".key/id_ed25519"),
		MissionCooldownSeconds: cooldown,
	}
}
