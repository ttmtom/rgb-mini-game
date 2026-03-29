package ledger

import (
	"crypto/ed25519"
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
	authorityPubKey ed25519.PublicKey,
) *LedgerModule {

	return &LedgerModule{
		service: newLedgerService(db, playerRepo, txRepo, gameEngine, authorityPubKey),
	}
}

func (m *LedgerModule) Service() *LedgerService {
	return m.service
}
