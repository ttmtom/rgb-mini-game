package ledger_config

import (
	"rgb-game/pkg/logger"
	"rgb-game/pkg/utils"
	"strconv"
)

type ServerConfig struct {
	Port int
}

func InitServerConfig() *ServerConfig {
	portStr := utils.GetEnv("LEDGER_GRPC_PORT", "50051")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		logger.Fatal("Invalid port number")
	}

	return &ServerConfig{
		Port: port,
	}
}
