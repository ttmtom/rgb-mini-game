package main

import (
	"fmt"
	"net"
	"rgb-game/config"
	"rgb-game/internal/core/container"
	"rgb-game/pkg/logger"

	"google.golang.org/grpc"
)

func main() {
	logger.Init()

	logger.Info("Init Ledger Server Config")
	cfg, err := config.InitLedgerServerConfig()
	if err != nil {
		logger.Fatalf("failed to initialize config: %v", err)
	}

	grpcServer := grpc.NewServer()

	ledgerContainer := container.NewLedgerContainer()
	ledgerContainer.ServerRegister(grpcServer)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.ServerConfig.Port))
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}

	logger.Infof("gRPC server listening on %v", lis.Addr())

	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatalf("failed to serve: %v", err)
	}
}
