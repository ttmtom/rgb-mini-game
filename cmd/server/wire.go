package main

import (
	"rgb-game/config"
	"rgb-game/internal/adapter/driven/authority"
	drivenledger "rgb-game/internal/adapter/driven/ledger"
	drivenredis "rgb-game/internal/adapter/driven/redis"
	grpcgame "rgb-game/internal/adapter/driving/grpc/game"
	"rgb-game/internal/app/port/in"
	"rgb-game/internal/app/port/out"
	"rgb-game/internal/app/service"
	"rgb-game/pkg/crypto"
	"rgb-game/pkg/di"
	"rgb-game/pkg/logger"
	"rgb-game/pkg/pb"

	redis "github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// wire constructs all driven adapters, application services, and the gRPC server
// for the Game Server binary using a DI container. Returns the server and a
// cleanup func that closes all connections when called (e.g. via defer).
func wire(cfg *config.GameServerFullConfig) (*grpc.Server, func(), error) {
	gsCfg := cfg.GameServerConfig
	c := di.New()

	// ── Config values ─────────────────────────────────────────────────────
	c.Provide(func() *config.RedisConfig { return cfg.RedisConfig })

	// ── Infrastructure — cleanup registered automatically via (T,func(),error) shape ──
	c.Provide(func(redisCfg *config.RedisConfig) (*redis.Client, func(), error) {
		client, err := drivenredis.Init(redisCfg)
		if err != nil {
			return nil, nil, err
		}
		return client, func() { client.Close() }, nil
	})
	c.Provide(func() (*grpc.ClientConn, func(), error) {
		logger.Infof("Connecting to Ledger at %s", gsCfg.LedgerAddr)
		conn, err := grpc.NewClient(gsCfg.LedgerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, nil, err
		}
		return conn, func() { conn.Close() }, nil
	})

	// ── Authority keypair ─────────────────────────────────────────────────
	c.Provide(func() (*crypto.Keypair, error) {
		return crypto.LoadOrGenerateKey(gsCfg.AuthorityKeyPath)
	})
	c.Provide(authority.NewFullAuthority)
	c.Alias((*out.FullAuthority)(nil), (**authority.Authority)(nil))

	// ── Ledger gRPC client ────────────────────────────────────────────────
	c.Provide(func(conn *grpc.ClientConn) pb.LedgerServiceClient {
		return pb.NewLedgerServiceClient(conn)
	})
	c.ProvideAs(drivenledger.New, (*out.LedgerClient)(nil))

	// ── Mission repository ────────────────────────────────────────────────
	c.ProvideAs(drivenredis.NewMissionRepository, (*out.MissionRepository)(nil))

	// ── Game service config + service ─────────────────────────────────────
	c.Provide(func() service.GameServiceConfig {
		return service.GameServiceConfig{MissionCooldown: cfg.GameConfig.Cooldown()}
	})
	c.Provide(service.NewGameService)
	c.Alias((*in.GameUseCase)(nil), (**service.GameService)(nil))

	// ── Driving adapter ───────────────────────────────────────────────────
	c.Provide(grpcgame.New)
	c.Provide(func(h *grpcgame.Handler) *grpc.Server {
		s := grpc.NewServer()
		pb.RegisterGameServiceServer(s, h)
		return s
	})

	// ── Log authority info (informational, not part of wiring chain) ──────
	var auth *authority.Authority
	if err := c.Resolve(&auth); err != nil {
		c.Close()
		return nil, nil, err
	}
	logger.Infof("Authority Player ID: %s", auth.PlayerID())
	logger.Infof("Authority Public Key (hex): %x", auth.PubKey())
	logger.Infof("Set AUTHORITY_PUB_KEY=%x in the Ledger .env if not using a shared key file", auth.PubKey())

	// ── Resolve root (triggers full wiring) ───────────────────────────────
	var s *grpc.Server
	if err := c.Resolve(&s); err != nil {
		c.Close()
		return nil, nil, err
	}

	return s, c.Close, nil
}

