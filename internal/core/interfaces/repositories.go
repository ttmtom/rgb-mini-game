package interfaces

import (
	"rgb-game/internal/core/types"

	"gorm.io/gorm"
)

// PlayerRepository defines the persistence contract for player records.
// Implementations live in the adapter layer; the core only depends on this interface.
//
// Note: *gorm.DB is accepted as a transaction handle so callers can participate
// in database transactions (e.g. SELECT FOR UPDATE). This is a pragmatic trade-off
// that keeps transaction coordination in the service layer without a full UoW abstraction.
type PlayerRepository interface {
	// Find returns the player for the given ID, or nil if not found.
	Find(db *gorm.DB, playerID string) (*types.PlayerRecord, error)

	// FindOrCreate returns the player for the given ID, creating a zero-balance
	// record if none exists. Must be called inside a DB transaction (SELECT FOR UPDATE).
	FindOrCreate(tx *gorm.DB, playerID string) (*types.PlayerRecord, error)

	// UpdateBalance persists the updated player record.
	UpdateBalance(tx *gorm.DB, player *types.PlayerRecord) error
}

// TransactionRepository defines the persistence contract for ledger transactions.
type TransactionRepository interface {
	// Create persists a new transaction record.
	Create(tx *gorm.DB, record *types.TransactionRecord) error
}
