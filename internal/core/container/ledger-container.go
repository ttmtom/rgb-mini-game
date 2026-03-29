package container

import (
	"crypto/ed25519"
	"rgb-game/internal/core/interfaces"
	"rgb-game/internal/core/ledger"
	"rgb-game/pkg/pb"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

type LedgerContainer struct {
	ledgerModule *ledger.LedgerModule
}

func NewLedgerContainer(
	db *gorm.DB,
	gameEngine interfaces.GameEngine,
	authorityPubKey ed25519.PublicKey,
) *LedgerContainer {
	ledgerModule := ledger.NewLedgerModule(db, gameEngine, authorityPubKey)

	return &LedgerContainer{ledgerModule}
}

func (c *LedgerContainer) ServerRegister(grpcServer *grpc.Server) {
	pb.RegisterLedgerServiceServer(grpcServer, c.ledgerModule.Service())
}
