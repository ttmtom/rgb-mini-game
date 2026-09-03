package main

import (
	"context"
	"encoding/hex"
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
	"time"

	"google.golang.org/grpc"
)

// wire constructs all driven adapters, application services, and the gRPC server
// for the Ledger binary. It returns the ready-to-serve gRPC server and a
// startBackground callback that launches background goroutines (BlockSealer,
// chain validation). Callers must invoke startBackground(ctx) before serving.
func wire(cfg *config.LedgerConfig, sealerCfg *config.BlockSealerConfig) (*grpc.Server, func(context.Context), error) {
	// ── Authority public key ────────────────────────────────────────────
	auth, err := authority.Load(cfg.AuthorityConfig)
	if err != nil {
		return nil, nil, err
	}
	logger.Infof("Authority loaded: player ID %s", auth.PlayerID())

	// ── Postgres ────────────────────────────────────────────────────────
	logger.Info("Connecting to Postgres")
	pg, err := drivenpostgres.Init(cfg.DatabaseConfig)
	if err != nil {
		return nil, nil, err
	}
	db := pg.DB()

	// ── Driven adapters (repositories + transactor) ──────────────────────
	playerRepo := repositories.NewPlayerRepository(db)
	txRepo := repositories.NewTransactionRepository(db)
	blockRepo := repositories.NewBlockRepository(db)
	authorityRepo := repositories.NewAuthorityRepository(db)
	transactor := repositories.NewTransactor(db)

	// ── Seed genesis authority into the registry ─────────────────────────
	genesisRecord := &types.AuthorityRecord{
		ID:           auth.PlayerID(),
		PubKeyHex:    hex.EncodeToString(auth.PubKey()),
		RegisteredAt: time.Now().Unix(),
	}
	if err := authorityRepo.Register(context.Background(), genesisRecord); err != nil {
		return nil, nil, err
	}
	logger.Infof("Genesis authority seeded: %s", auth.PlayerID())

	// ── Domain engine + application services ────────────────────────────
	ge := engine.New()
	ledgerSvc := service.NewLedgerService(playerRepo, txRepo, blockRepo, authorityRepo, transactor, ge)
	blockSealer := service.NewBlockSealer(blockRepo, transactor, sealerCfg.Interval(), sealerCfg.Difficulty)

	// ── Driving adapter (gRPC handler) ───────────────────────────────────
	s := grpc.NewServer()
	pb.RegisterLedgerServiceServer(s, grpcledger.New(ledgerSvc, authorityRepo))

	startBackground := func(ctx context.Context) {
		go blockSealer.Start(ctx)
		if err := ledgerSvc.ValidateChain(ctx); err != nil {
			logger.Warnf("Chain integrity check FAILED: %v", err)
		} else {
			logger.Info("Chain integrity check passed")
		}
	}

	return s, startBackground, nil
}
