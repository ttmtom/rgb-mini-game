package container

import (
	"rgb-game/internal/core/ledger"
	"rgb-game/pkg/pb"

	"google.golang.org/grpc"
)

type LedgerContainer struct {
	ledgerModule *ledger.LedgerModule
}

func NewLedgerContainer() *LedgerContainer {
	ledgerModule := ledger.NewLedgerModule()

	return &LedgerContainer{ledgerModule}
}

func (c *LedgerContainer) ServerRegister(grpcServer *grpc.Server) {
	pb.RegisterLedgerServiceServer(grpcServer, c.ledgerModule.Service())
}
