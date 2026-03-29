package container

import (
	"rgb-game/internal/adapter/postgres/repositories"
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
	auth interfaces.PublicAuthority,
) *LedgerContainer {
	// ── Repositories (adapter layer — wired here, never inside core) ────
	playerRepo := repositories.NewPlayerRepository(db)
	txRepo := repositories.NewTransactionRepository(db)

	ledgerModule := ledger.NewLedgerModule(db, playerRepo, txRepo, gameEngine, auth)

	return &LedgerContainer{ledgerModule}
}

func (c *LedgerContainer) ServerRegister(grpcServer *grpc.Server) {
	pb.RegisterLedgerServiceServer(grpcServer, c.ledgerModule.Service())
}
