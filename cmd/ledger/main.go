package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"rgb-game/config"
	"rgb-game/internal/adapter/authority"
	"rgb-game/internal/adapter/postgres"
	"rgb-game/internal/core/container"
	"rgb-game/internal/core/game_engine"
	"rgb-game/pkg/logger"
	"syscall"

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
	auth, err := authority.Load(cfg.AuthorityConfig)
	if err != nil {
		logger.Fatalf("failed to load authority: %v", err)
	}
	logger.Infof("Authority loaded: player ID %s", auth.PlayerID())

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

	ledgerContainer := container.NewLedgerContainer(db, ge, auth)
	ledgerContainer.ServerRegister(grpcServer)

	// ── Listen ──────────────────────────────────────────────────────────
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.ServerConfig.Port))
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}

	logger.Infof("Ledger gRPC server listening on %v", lis.Addr())

	// Serve in the background so we can handle shutdown signals.
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatalf("failed to serve: %v", err)
		}
	}()

	// Wait for SIGINT or SIGTERM, then drain in-flight RPCs.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Infof("Received %s, shutting down gracefully…", sig)
	grpcServer.GracefulStop()
	logger.Info("Ledger server stopped")
}
