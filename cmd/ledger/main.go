package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"rgb-game/config"
	"rgb-game/internal/adapter/driven/authority"
	drivenpostgres "rgb-game/internal/adapter/driven/postgres"
	"rgb-game/internal/adapter/driven/postgres/repositories"
	grpcledger "rgb-game/internal/adapter/driving/grpc/ledger"
	"rgb-game/internal/app/service"
	"rgb-game/internal/domain/engine"
	"rgb-game/pkg/logger"
	"rgb-game/pkg/pb"
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
	pg, err := drivenpostgres.Init(cfg.DatabaseConfig)
	if err != nil {
		logger.Fatalf("failed to connect to Postgres: %v", err)
	}
	db := pg.DB()

	// ── Driven adapters (repositories + transactor) ──────────────────────
	playerRepo := repositories.NewPlayerRepository(db)
	txRepo := repositories.NewTransactionRepository(db)
	transactor := repositories.NewTransactor(db)

	// ── Domain engine ────────────────────────────────────────────────────
	ge := engine.New()

	// ── Application service ──────────────────────────────────────────────
	ledgerSvc := service.NewLedgerService(playerRepo, txRepo, transactor, ge)

	// ── Driving adapter (gRPC handler) ───────────────────────────────────
	grpcServer := grpc.NewServer()
	pb.RegisterLedgerServiceServer(grpcServer, grpcledger.New(ledgerSvc, auth))

	// ── Listen ──────────────────────────────────────────────────────────
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.ServerConfig.Port))
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}
	logger.Infof("Ledger gRPC server listening on %v", lis.Addr())

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatalf("failed to serve: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Infof("Received %s, shutting down gracefully…", sig)
	grpcServer.GracefulStop()
	logger.Info("Ledger server stopped")
}
