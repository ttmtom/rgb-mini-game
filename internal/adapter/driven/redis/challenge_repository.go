package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"rgb-game/internal/domain/types"
	"time"

	"github.com/redis/go-redis/v9"
)

func challengeKey(id string) string { return "challenge:" + id }

// ChallengeRepository implements out.ChallengeRepository using Redis.
type ChallengeRepository struct {
	client *redis.Client
}

// NewChallengeRepository creates a ChallengeRepository backed by the given Redis client.
func NewChallengeRepository(client *redis.Client) *ChallengeRepository {
	return &ChallengeRepository{client: client}
}

// Create stores a new challenge record with the given TTL.
func (r *ChallengeRepository) Create(ctx context.Context, record *types.ChallengeRecord, ttl time.Duration) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal challenge: %w", err)
	}
	return r.client.SetEx(ctx, challengeKey(record.ID), string(data), ttl).Err()
}

// FindByID fetches and deserialises a challenge by its ID.
// Returns nil, nil when the key does not exist or has expired.
func (r *ChallengeRepository) FindByID(ctx context.Context, challengeID string) (*types.ChallengeRecord, error) {
	val, err := r.client.Get(ctx, challengeKey(challengeID)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var record types.ChallengeRecord
	if err := json.Unmarshal([]byte(val), &record); err != nil {
		return nil, fmt.Errorf("unmarshal challenge: %w", err)
	}
	return &record, nil
}

// UpdateStatus updates the status field of a stored challenge, preserving other fields.
func (r *ChallengeRepository) UpdateStatus(ctx context.Context, challengeID string, status types.ChallengeStatus) error {
	record, err := r.FindByID(ctx, challengeID)
	if err != nil {
		return err
	}
	if record == nil {
		return fmt.Errorf("challenge %s not found", challengeID)
	}

	record.Status = status
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal updated challenge: %w", err)
	}

	// Preserve remaining TTL.
	ttl, err := r.client.TTL(ctx, challengeKey(challengeID)).Result()
	if err != nil || ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return r.client.SetEx(ctx, challengeKey(challengeID), string(data), ttl).Err()
}
