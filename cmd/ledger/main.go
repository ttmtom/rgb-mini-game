package main

import (
	"fmt"
	"net"
	"rgb-game/config"
	"rgb-game/internal/adapter/authority"
	"rgb-game/internal/adapter/postgres"
	"rgb-game/internal/core/container"
	"rgb-game/internal/core/game_engine"
	"rgb-game/pkg/logger"

	"google.golang.org/grpc"
)

func main() {
	logger.Init()

	// ── Configuration ───────────────────────────────────────────────────
	logger.Info("Initializing Ledger configuration")
	cfg, err := config.InitLedgerConfig()
	if err != nil {
		logger.Fatalf("failed to initialize config: %v", err)
	}

	// ── Authority public key ────────────────────────────────────────────
	authorityPubKey, err := authority.Load(cfg.AuthorityConfig)
	if err != nil {
		logger.Fatalf("failed to load authority public key: %v", err)
	}
	logger.Infof("Authority public key loaded (%d bytes)", len(authorityPubKey))

	// ── Postgres ────────────────────────────────────────────────────────
	logger.Info("Connecting to Postgres")
	pg, err := postgres.Init(cfg.DatabaseConfig)
	if err != nil {
		logger.Fatalf("failed to connect to Postgres: %v", err)
	}
	db := pg.DB()

	// ── Game Engine ─────────────────────────────────────────────────────
	ge := game_engine.NewGameEngine()

	// ── DI Container & gRPC ─────────────────────────────────────────────
	grpcServer := grpc.NewServer()

	ledgerContainer := container.NewLedgerContainer(db, ge, authorityPubKey)
	ledgerContainer.ServerRegister(grpcServer)

	// ── Listen ──────────────────────────────────────────────────────────
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.ServerConfig.Port))
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}

	logger.Infof("Ledger gRPC server listening on %v", lis.Addr())

	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatalf("failed to serve: %v", err)
	}
}
