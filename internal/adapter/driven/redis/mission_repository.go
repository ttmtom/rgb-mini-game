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

// Key layout:
//   mission:{id}               → JSON MissionRecord,  TTL = 2 × missionTTL
//   player:active:{playerID}   → missionID string,    TTL = 2 × missionTTL
//   player:cooldown:{playerID} → "1" (presence only), TTL = cooldownTTL

func missionKey(id string) string        { return "mission:" + id }
func activeKey(playerID string) string   { return "player:active:" + playerID }
func cooldownKey(playerID string) string { return "player:cooldown:" + playerID }

// MissionRepository implements out.MissionRepository using Redis.
type MissionRepository struct {
	client *redis.Client
}

// NewMissionRepository creates a MissionRepository backed by the given Redis client.
func NewMissionRepository(client *redis.Client) *MissionRepository {
	return &MissionRepository{client: client}
}

// minMissionTTL is the minimum lifetime for a mission key regardless of cooldown.
// Prevents very short cooldown settings (e.g. MISSION_COOLDOWN_SECONDS=1) from
// expiring the mission before the player can navigate the menu and complete it.
const minMissionTTL = 5 * time.Minute

// Create stores the mission JSON and an active-mission pointer for the player.
func (r *MissionRepository) Create(ctx context.Context, record *types.MissionRecord, missionTTL time.Duration) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal mission: %w", err)
	}

	ttl := missionTTL * 2
	if ttl < minMissionTTL {
		ttl = minMissionTTL
	}
	pipe := r.client.TxPipeline()
	pipe.SetEx(ctx, missionKey(record.ID), string(data), ttl)
	pipe.SetEx(ctx, activeKey(record.PlayerID), record.ID, ttl)
	_, err = pipe.Exec(ctx)
	return err
}

// FindByID fetches and deserialises a mission by its ID.
// Returns nil, nil when the key does not exist.
func (r *MissionRepository) FindByID(ctx context.Context, missionID string) (*types.MissionRecord, error) {
	val, err := r.client.Get(ctx, missionKey(missionID)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var record types.MissionRecord
	if err := json.Unmarshal([]byte(val), &record); err != nil {
		return nil, fmt.Errorf("unmarshal mission: %w", err)
	}
	return &record, nil
}

// FindActiveByPlayer resolves the player's active-mission pointer and returns that mission.
func (r *MissionRepository) FindActiveByPlayer(ctx context.Context, playerID string) (*types.MissionRecord, error) {
	missionID, err := r.client.Get(ctx, activeKey(playerID)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, missionID)
}

// GetCooldownRemaining returns the TTL of the player's cooldown key.
// Returns 0 when no cooldown is active.
func (r *MissionRepository) GetCooldownRemaining(ctx context.Context, playerID string) (time.Duration, error) {
	ttl, err := r.client.TTL(ctx, cooldownKey(playerID)).Result()
	if err != nil {
		return 0, err
	}
	if ttl < 0 {
		return 0, nil
	}
	return ttl, nil
}

// Complete marks the mission as completed, removes the active pointer, and sets cooldown.
func (r *MissionRepository) Complete(ctx context.Context, missionID, playerID string, cooldownTTL time.Duration) error {
	record, err := r.FindByID(ctx, missionID)
	if err != nil {
		return err
	}
	if record == nil {
		return fmt.Errorf("mission %s not found", missionID)
	}

	record.Completed = true
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal completed mission: %w", err)
	}

	pipe := r.client.TxPipeline()
	pipe.SetEx(ctx, missionKey(missionID), string(data), 5*time.Minute)
	pipe.Del(ctx, activeKey(playerID))
	pipe.SetEx(ctx, cooldownKey(playerID), "1", cooldownTTL)
	_, err = pipe.Exec(ctx)
	return err
}
