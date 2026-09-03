package main

import (
	"rgb-game/config"
	"rgb-game/internal/adapter/driven/authority"
	drivenledger "rgb-game/internal/adapter/driven/ledger"
	drivenredis "rgb-game/internal/adapter/driven/redis"
	grpcgame "rgb-game/internal/adapter/driving/grpc/game"
	"rgb-game/internal/app/service"
	"rgb-game/pkg/crypto"
	"rgb-game/pkg/logger"
	"rgb-game/pkg/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// wire constructs all driven adapters, application services, and the gRPC server
// for the Game Server binary. It returns the ready-to-serve gRPC server.
// Callers are responsible for closing the returned cleanup resources (handled
// via defer in main via the returned cleanup func).
func wire(cfg *config.GameServerFullConfig) (*grpc.Server, func(), error) {
	gsCfg := cfg.GameServerConfig

	// ── Authority keypair ───────────────────────────────────────────────
	keypair, err := crypto.LoadOrGenerateKey(gsCfg.AuthorityKeyPath)
	if err != nil {
		return nil, nil, err
	}
	auth := authority.NewFullAuthority(keypair)
	logger.Infof("Authority Player ID: %s", auth.PlayerID())
	logger.Infof("Authority Public Key (hex): %x", auth.PubKey())
	logger.Infof("Set AUTHORITY_PUB_KEY=%x in the Ledger .env if not using a shared key file", auth.PubKey())

	// ── Redis ───────────────────────────────────────────────────────────
	redisClient, err := drivenredis.Init(cfg.RedisConfig)
	if err != nil {
		return nil, nil, err
	}

	// ── Ledger gRPC client ──────────────────────────────────────────────
	logger.Infof("Connecting to Ledger at %s", gsCfg.LedgerAddr)
	ledgerConn, err := grpc.NewClient(gsCfg.LedgerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		redisClient.Close()
		return nil, nil, err
	}

	// ── Driven adapters ─────────────────────────────────────────────────
	missionRepo := drivenredis.NewMissionRepository(redisClient)
	ledgerClient := drivenledger.New(pb.NewLedgerServiceClient(ledgerConn))

	// ── Application service ──────────────────────────────────────────────
	gameSvc := service.NewGameService(missionRepo, auth, ledgerClient, service.GameServiceConfig{
		MissionCooldown: cfg.GameConfig.Cooldown(),
	})

	// ── Driving adapter (gRPC handler) ───────────────────────────────────
	s := grpc.NewServer()
	pb.RegisterGameServiceServer(s, grpcgame.New(gameSvc))

	cleanup := func() {
		ledgerConn.Close()
		redisClient.Close()
	}

	return s, cleanup, nil
}
