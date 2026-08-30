package main

import (
	"context"
	"encoding/hex"
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
	"rgb-game/internal/domain/types"
	"rgb-game/pkg/logger"
	"rgb-game/pkg/pb"
	"syscall"
	"time"

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
	sealerCfg := config.InitBlockSealerConfig()

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
	playerRepo    := repositories.NewPlayerRepository(db)
	txRepo        := repositories.NewTransactionRepository(db)
	blockRepo     := repositories.NewBlockRepository(db)
	authorityRepo := repositories.NewAuthorityRepository(db)
	transactor    := repositories.NewTransactor(db)

	// ── Seed genesis authority into the registry ─────────────────────────
	genesisRecord := &types.AuthorityRecord{
		ID:           auth.PlayerID(),
		PubKeyHex:    hex.EncodeToString(auth.PubKey()),
		RegisteredAt: time.Now().Unix(),
	}
	if err := authorityRepo.Register(context.Background(), genesisRecord); err != nil {
		logger.Fatalf("failed to seed genesis authority: %v", err)
	}
	logger.Infof("Genesis authority seeded: %s", auth.PlayerID())

	// ── Domain engine ────────────────────────────────────────────────────
	ge := engine.New()

	// ── Application services ─────────────────────────────────────────────
	ledgerSvc := service.NewLedgerService(playerRepo, txRepo, blockRepo, authorityRepo, transactor, ge)
	blockSealer := service.NewBlockSealer(blockRepo, transactor, sealerCfg.Interval(), sealerCfg.Difficulty)

	// ── Driving adapter (gRPC handler) ───────────────────────────────────
	grpcServer := grpc.NewServer()
	pb.RegisterLedgerServiceServer(grpcServer, grpcledger.New(ledgerSvc, authorityRepo))

	// ── Listen ──────────────────────────────────────────────────────────
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.ServerConfig.Port))
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}
	logger.Infof("Ledger gRPC server listening on %v", lis.Addr())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go blockSealer.Start(ctx)

	// Validate chain integrity on startup.
	if err := ledgerSvc.ValidateChain(context.Background()); err != nil {
		logger.Warnf("Chain integrity check FAILED: %v", err)
	} else {
		logger.Info("Chain integrity check passed")
	}
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatalf("failed to serve: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Infof("Received %s, shutting down gracefully…", sig)
	cancel()
	grpcServer.GracefulStop()
	logger.Info("Ledger server stopped")
}
