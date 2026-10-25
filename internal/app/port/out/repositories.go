package out

import (
	"context"
	"rgb-game/internal/domain/types"
)

// LeaderboardEntry is a ranked player summary returned by the leaderboard query.
type LeaderboardEntry struct {
	Rank     uint32
	PlayerID string
	Red      uint32
	Green    uint32
	Blue     uint32
	Total    uint32
}

// PlayerRepository defines the persistence contract for player records.
type PlayerRepository interface {
	Find(ctx context.Context, playerID string) (*types.PlayerRecord, error)
	FindOrCreate(ctx context.Context, playerID string) (*types.PlayerRecord, error)
	UpdateBalance(ctx context.Context, player *types.PlayerRecord) error
	Count(ctx context.Context) (uint64, error)

	// GetLeaderboard returns the top-n players sorted by the given channel.
	// sortBy must be one of "RED", "GREEN", "BLUE", "TOTAL".
	GetLeaderboard(ctx context.Context, topN uint32, sortBy string) ([]*LeaderboardEntry, error)
}

// TransactionRepository defines the persistence contract for ledger transactions.
type TransactionRepository interface {
	Create(ctx context.Context, record *types.TransactionRecord) error
	FindByPlayer(ctx context.Context, playerID string, limit, offset uint32) ([]*types.TransactionRecord, uint64, error)
	CountAll(ctx context.Context) (uint64, error)
}
