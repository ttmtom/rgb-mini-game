package out

import (
	"context"
	"rgb-game/internal/domain/types"
)

// PlayerRepository defines the persistence contract for player records.
// All methods receive a context.Context; implementations retrieve the active
// database transaction (if any) from the context via the Transactor port.
type PlayerRepository interface {
	// Find returns the player for the given ID, or nil if not found.
	Find(ctx context.Context, playerID string) (*types.PlayerRecord, error)

	// FindOrCreate returns the player for the given ID, creating a zero-balance
	// record if none exists. Must be called inside a Transactor.InTransaction
	// block so the implementation can apply SELECT FOR UPDATE.
	FindOrCreate(ctx context.Context, playerID string) (*types.PlayerRecord, error)

	// UpdateBalance persists the updated player record.
	UpdateBalance(ctx context.Context, player *types.PlayerRecord) error
}

// TransactionRepository defines the persistence contract for ledger transactions.
type TransactionRepository interface {
	// Create persists a new transaction record.
	Create(ctx context.Context, record *types.TransactionRecord) error
}
