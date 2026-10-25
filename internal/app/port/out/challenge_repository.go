package out

import (
	"context"
	"rgb-game/internal/domain/types"
	"time"
)

// ChallengeRepository defines the persistence contract for player challenges.
// Implementations store challenges in Redis with TTL-based expiry.
type ChallengeRepository interface {
	// Create stores a new challenge record. ttl controls expiry.
	Create(ctx context.Context, record *types.ChallengeRecord, ttl time.Duration) error

	// FindByID returns the challenge with the given ID, or nil if not found/expired.
	FindByID(ctx context.Context, challengeID string) (*types.ChallengeRecord, error)

	// UpdateStatus updates the status field of an existing challenge.
	UpdateStatus(ctx context.Context, challengeID string, status types.ChallengeStatus) error
}
