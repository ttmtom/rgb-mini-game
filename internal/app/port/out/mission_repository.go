package out

import (
	"context"
	"rgb-game/internal/domain/types"
	"time"
)

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
