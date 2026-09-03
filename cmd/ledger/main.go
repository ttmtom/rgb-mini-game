package main

import (
	"context"
	"rgb-game/config"
	"rgb-game/pkg/grpcserver"
	"rgb-game/pkg/logger"
)

func main() {
	logger.Init()

	logger.Info("Initializing Ledger configuration")
	cfg, err := config.InitLedgerConfig()
	if err != nil {
		logger.Fatalf("failed to initialize config: %v", err)
	}
	sealerCfg := config.InitBlockSealerConfig()

	s, startBackground, err := wire(cfg, sealerCfg)
	if err != nil {
		logger.Fatalf("failed to wire dependencies: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startBackground(ctx)

	if err := grpcserver.Run(ctx, s, cfg.ServerConfig.Port); err != nil {
		logger.Fatalf("gRPC server error: %v", err)
	}
	logger.Info("Ledger server stopped")
}

