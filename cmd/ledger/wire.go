package main

import (
	"context"
	"encoding/hex"
	"rgb-game/config"
	"rgb-game/internal/adapter/driven/authority"
	drivenpostgres "rgb-game/internal/adapter/driven/postgres"
	"rgb-game/internal/adapter/driven/postgres/repositories"
	grpcledger "rgb-game/internal/adapter/driving/grpc/ledger"
	"rgb-game/internal/app/port/in"
	"rgb-game/internal/app/port/out"
	"rgb-game/internal/app/service"
	"rgb-game/internal/domain/engine"
	"rgb-game/internal/domain/types"
	"rgb-game/pkg/di"
	"rgb-game/pkg/logger"
	"rgb-game/pkg/pb"
	"time"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

// wire constructs all driven adapters, application services, and the gRPC server
// for the Ledger binary using a DI container for automatic dependency resolution.
// It returns the ready-to-serve gRPC server and a startBackground callback.
func wire(cfg *config.LedgerConfig, sealerCfg *config.BlockSealerConfig) (*grpc.Server, func(context.Context), error) {
	c := di.New()

	// ── Config values (zero-arg lambdas close over outer cfg) ─────────────
	c.Provide(func() *config.DatabaseConfig  { return cfg.DatabaseConfig })
	c.Provide(func() *config.AuthorityConfig { return cfg.AuthorityConfig })

	// ── Infrastructure ────────────────────────────────────────────────────
	c.Provide(drivenpostgres.Init) // *config.DatabaseConfig → (*Postgres, error)
	c.Provide(func(pg *drivenpostgres.Postgres) *gorm.DB { return pg.DB() })
	c.Provide(authority.Load) // *config.AuthorityConfig → (*authority.Authority, error)

	// ── Repositories — registered under their port/out interface types ─────
	c.ProvideAs(repositories.NewPlayerRepository,      (*out.PlayerRepository)(nil))
	c.ProvideAs(repositories.NewTransactionRepository, (*out.TransactionRepository)(nil))
	c.ProvideAs(repositories.NewBlockRepository,       (*out.BlockRepository)(nil))
	c.ProvideAs(repositories.NewAuthorityRepository,   (*out.AuthorityRepository)(nil))
	c.ProvideAs(repositories.NewTransactor,            (*out.Transactor)(nil))

	// ── Domain engine ─────────────────────────────────────────────────────
	c.ProvideAs(engine.New, (*out.GameEngine)(nil))

	// ── Application services ──────────────────────────────────────────────
	c.Provide(service.NewLedgerService)
	c.Alias((*in.LedgerUseCase)(nil), (**service.LedgerService)(nil))

	// BlockSealer wraps primitive config values that cannot be auto-resolved by type.
	c.Provide(func(blockRepo out.BlockRepository, transactor out.Transactor) *service.BlockSealer {
		return service.NewBlockSealer(blockRepo, transactor, sealerCfg.Interval(), sealerCfg.Difficulty)
	})

	// ── Driving adapter ───────────────────────────────────────────────────
	c.Provide(grpcledger.New)
	c.Provide(func(h *grpcledger.Handler) *grpc.Server {
		s := grpc.NewServer()
		pb.RegisterLedgerServiceServer(s, h)
		return s
	})

	// ── Seed genesis authority (startup side-effect, before server starts) ─
	var auth *authority.Authority
	var authorityRepo out.AuthorityRepository
	if err := c.Resolve(&auth); err != nil {
		return nil, nil, err
	}
	logger.Infof("Authority loaded: player ID %s", auth.PlayerID())
	if err := c.Resolve(&authorityRepo); err != nil {
		return nil, nil, err
	}
	genesisRecord := &types.AuthorityRecord{
		ID:           auth.PlayerID(),
		PubKeyHex:    hex.EncodeToString(auth.PubKey()),
		RegisteredAt: time.Now().Unix(),
	}
	if err := authorityRepo.Register(context.Background(), genesisRecord); err != nil {
		return nil, nil, err
	}
	logger.Infof("Genesis authority seeded: %s", auth.PlayerID())

	// ── Resolve root object (triggers remaining dependency chain) ──────────
	var s *grpc.Server
	if err := c.Resolve(&s); err != nil {
		return nil, nil, err
	}

	// ── startBackground captures already-cached singletons ────────────────
	var ledgerSvc *service.LedgerService
	var blockSealer *service.BlockSealer
	if err := c.Resolve(&ledgerSvc); err != nil {
		return nil, nil, err
	}
	if err := c.Resolve(&blockSealer); err != nil {
		return nil, nil, err
	}

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

