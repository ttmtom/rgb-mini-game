package interfaces

import (
	"context"
	"rgb-game/internal/core/types"
	"time"

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

// MissionRepository defines the persistence contract for game missions.
// Implementations store missions in Redis with TTL-based expiry.
type MissionRepository interface {
	// Create stores a new mission. missionTTL controls how long the mission key
	// (and the player's active-mission pointer) lives in Redis.
	Create(ctx context.Context, record *types.MissionRecord, missionTTL time.Duration) error

	// FindByID returns the mission with the given ID, or nil if not found / expired.
	FindByID(ctx context.Context, missionID string) (*types.MissionRecord, error)

	// FindActiveByPlayer returns the current uncompleted mission for a player,
	// or nil if the player has no active mission.
	FindActiveByPlayer(ctx context.Context, playerID string) (*types.MissionRecord, error)

	// GetCooldownRemaining returns how long the player must still wait before
	// requesting a new mission. Returns 0 when the cooldown has elapsed.
	GetCooldownRemaining(ctx context.Context, playerID string) (time.Duration, error)

	// Complete marks a mission as completed, removes the player's active-mission
	// pointer, and sets a post-completion cooldown key with the given TTL.
	Complete(ctx context.Context, missionID, playerID string, cooldownTTL time.Duration) error
}
