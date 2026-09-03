package service

import (
	"context"
	"fmt"
	"math/rand"
	"rgb-game/internal/app/port/out"
	"rgb-game/internal/domain/types"
	"rgb-game/pkg/logger"
	"time"

	"github.com/google/uuid"
)

// missionService handles the full lifecycle of a game mission:
// issuing, cooldown enforcement, validation, and completion.
// It is unexported — consumed only by GameService within this package.
type missionService struct {
	repo     out.MissionRepository
	cooldown time.Duration
}

func newMissionService(repo out.MissionRepository, cfg GameServiceConfig) *missionService {
	return &missionService{repo: repo, cooldown: cfg.MissionCooldown}
}

// requestMission tries to issue a new mission for the player.
//
// Return semantics:
//   - (mission, 0, nil)      — mission issued successfully
//   - (nil, remaining, nil)  — player is on post-completion cooldown; remaining > 0
//   - (nil, 0, err)          — player already has an active mission, or a storage error
func (s *missionService) requestMission(ctx context.Context, playerID string) (*types.MissionRecord, int32, error) {
	// Reject if the player already has an uncompleted mission.
	active, err := s.repo.FindActiveByPlayer(ctx, playerID)
	if err != nil {
		return nil, 0, fmt.Errorf("check active mission: %w", err)
	}
	if active != nil {
		return nil, 0, fmt.Errorf("player %s already has an active mission %s", playerID, active.ID)
	}

	// Enforce post-completion cooldown.
	remaining, err := s.repo.GetCooldownRemaining(ctx, playerID)
	if err != nil {
		return nil, 0, fmt.Errorf("check cooldown: %w", err)
	}
	if remaining > 0 {
		secs := int32(remaining.Seconds())
		logger.Infof("Player %s is on cooldown, %ds remaining", playerID, secs)
		return nil, secs, nil
	}

	// Issue new mission (RewardColor: 0=Red, 1=Green, 2=Blue).
	record := &types.MissionRecord{
		ID:          uuid.New().String(),
		PlayerID:    playerID,
		RewardColor: int32(rand.Intn(3)),
		IssuedAt:    time.Now().Unix(),
	}
	if err := s.repo.Create(ctx, record, s.cooldown); err != nil {
		return nil, 0, fmt.Errorf("store mission: %w", err)
	}

	logger.Infof("Issued mission %s to player %s (rewardColor=%d)", record.ID, playerID, record.RewardColor)
	return record, 0, nil
}

// validateAndComplete verifies that the mission can be completed, marks it complete,
// and returns the mission record so the caller can extract the reward color.
func (s *missionService) validateAndComplete(ctx context.Context, missionID, playerID string) (*types.MissionRecord, error) {
	record, err := s.repo.FindByID(ctx, missionID)
	if err != nil {
		return nil, fmt.Errorf("find mission: %w", err)
	}
	if record == nil {
		return nil, fmt.Errorf("mission not found")
	}
	if record.PlayerID != playerID {
		return nil, fmt.Errorf("mission does not belong to this player")
	}
	if record.Completed {
		return nil, fmt.Errorf("mission already completed")
	}

	elapsed := time.Since(time.Unix(record.IssuedAt, 0))
	if elapsed < s.cooldown {
		remaining := int32((s.cooldown - elapsed).Seconds())
		return nil, fmt.Errorf("mission not yet ready, %ds remaining", remaining)
	}

	if err := s.repo.Complete(ctx, missionID, playerID, s.cooldown); err != nil {
		return nil, fmt.Errorf("complete mission: %w", err)
	}

	record.Completed = true
	return record, nil
}
