package main

import (
	"rgb-game/config"
	"rgb-game/pkg/grpcserver"
	"rgb-game/pkg/logger"

	"context"
)

func main() {
	logger.Init()

	logger.Info("Initializing Game Server configuration")
	cfg, err := config.InitGameServerFullConfig()
	if err != nil {
		logger.Fatalf("failed to initialize config: %v", err)
	}

	s, cleanup, err := wire(cfg)
	if err != nil {
		logger.Fatalf("failed to wire dependencies: %v", err)
	}
	defer cleanup()

	ctx := context.Background()
	if err := grpcserver.Run(ctx, s, cfg.GameServerConfig.Port); err != nil {
		logger.Fatalf("gRPC server error: %v", err)
	}
	logger.Info("Game Server stopped")
}

