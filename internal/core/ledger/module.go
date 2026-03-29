package ledger

import (
	"rgb-game/internal/core/interfaces"

	"gorm.io/gorm"
)

type LedgerModule struct {
	service *LedgerService
}

func NewLedgerModule(
	db *gorm.DB,
	playerRepo interfaces.PlayerRepository,
	txRepo interfaces.TransactionRepository,
	gameEngine interfaces.GameEngine,
	auth interfaces.PublicAuthority,
) *LedgerModule {

	return &LedgerModule{
		service: newLedgerService(db, playerRepo, txRepo, gameEngine, auth),
	}
}

func (m *LedgerModule) Service() *LedgerService {
	return m.service
}
