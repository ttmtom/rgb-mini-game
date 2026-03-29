package ledger

import (
	"crypto/ed25519"
	"rgb-game/internal/adapter/postgres/repositories"
	"rgb-game/internal/core/interfaces"

	"gorm.io/gorm"
)

type LedgerModule struct {
	service *LedgerService
}

func NewLedgerModule(
	db *gorm.DB,
	gameEngine interfaces.GameEngine,
	authorityPubKey ed25519.PublicKey,
) *LedgerModule {
	// ── Repositories ────────────────────────────────────────────────────
	playerRepo := repositories.NewPlayerRepository(db)
	txRepo := repositories.NewTransactionRepository(db)

	return &LedgerModule{
		service: newLedgerService(db, playerRepo, txRepo, gameEngine, authorityPubKey),
	}
}

func (m *LedgerModule) Service() *LedgerService {
	return m.service
}
