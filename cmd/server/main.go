package main

import (
	"fmt"
	"net"
	"rgb-game/config"
	"rgb-game/internal/adapter/authority"
	"rgb-game/internal/core/container"
	"rgb-game/pkg/crypto"
	"rgb-game/pkg/logger"
	"rgb-game/pkg/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger.Init()

	// ── Configuration ───────────────────────────────────────────────────
	logger.Info("Initializing Game Server configuration")
	cfg, err := config.InitGameServerFullConfig()
	if err != nil {
		logger.Fatalf("failed to initialize config: %v", err)
	}
	gsCfg := cfg.GameServerConfig

	// ── Authority keypair ───────────────────────────────────────────────
	keypair, err := crypto.LoadOrGenerateKey(gsCfg.AuthorityKeyPath)
	if err != nil {
		logger.Fatalf("failed to load/generate authority keypair: %v", err)
	}
	auth := authority.NewFullAuthority(keypair)
	logger.Infof("Authority Player ID: %s", auth.PlayerID())
	logger.Infof("Authority Public Key (hex): %x", auth.PubKey())
	logger.Infof("Set AUTHORITY_PUB_KEY=%x in the Ledger .env if not using a shared key file", auth.PubKey())

	// ── Ledger gRPC client ──────────────────────────────────────────────
	logger.Infof("Connecting to Ledger at %s", gsCfg.LedgerAddr)
	ledgerConn, err := grpc.NewClient(gsCfg.LedgerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatalf("failed to connect to Ledger: %v", err)
	}
	defer ledgerConn.Close()
	ledgerClient := pb.NewLedgerServiceClient(ledgerConn)

	// ── DI Container & gRPC ─────────────────────────────────────────────
	grpcServer := grpc.NewServer()

	gameContainer := container.NewGameContainer(auth, ledgerClient, cfg.GameConfig)
	gameContainer.ServerRegister(grpcServer)

	// ── Listen ──────────────────────────────────────────────────────────
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", gsCfg.Port))
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}

	logger.Infof("Game Server gRPC listening on %v", lis.Addr())
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatalf("failed to serve: %v", err)
	}
}
