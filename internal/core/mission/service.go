package mission

import (
	"context"
	"fmt"
	"math/rand"
	"rgb-game/config"
	"rgb-game/internal/core/interfaces"
	"rgb-game/internal/core/types"
	"rgb-game/pkg/logger"
	"rgb-game/pkg/pb"
	"time"

	"github.com/google/uuid"
)

// MissionService handles the full lifecycle of a game mission:
// issuing, cooldown enforcement, validation, and completion.
type MissionService struct {
	repo interfaces.MissionRepository
	cfg  *config.GameConfig
}

func newMissionService(repo interfaces.MissionRepository, cfg *config.GameConfig) *MissionService {
	return &MissionService{repo: repo, cfg: cfg}
}

// RequestMission tries to issue a new mission for the player.
//
// Return semantics:
//   - (mission, 0, nil)      — mission issued successfully
//   - (nil, remaining, nil)  — player is on post-completion cooldown; remaining > 0
//   - (nil, 0, err)          — player already has an active mission, or a storage error
func (s *MissionService) RequestMission(ctx context.Context, playerID string) (*types.MissionRecord, int32, error) {
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

	// Issue new mission.
	record := &types.MissionRecord{
		ID:          uuid.New().String(),
		PlayerID:    playerID,
		RewardColor: int32(rand.Intn(3)), // 0=RED, 1=GREEN, 2=BLUE
		IssuedAt:    time.Now().Unix(),
	}
	if err := s.repo.Create(ctx, record, s.cfg.Cooldown()); err != nil {
		return nil, 0, fmt.Errorf("store mission: %w", err)
	}

	logger.Infof("Issued mission %s to player %s (reward=%s)", record.ID, playerID, pb.RewardColor(record.RewardColor))
	return record, 0, nil
}

// ValidateAndComplete verifies that the mission can be completed (exists, belongs to the
// player, cooldown elapsed since issue), marks it complete in the store, and returns the
// mission record so the caller can extract the reward color for the MINT transaction.
func (s *MissionService) ValidateAndComplete(ctx context.Context, missionID, playerID string) (*types.MissionRecord, error) {
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
	if elapsed < s.cfg.Cooldown() {
		remaining := int32((s.cfg.Cooldown() - elapsed).Seconds())
		return nil, fmt.Errorf("mission not yet ready, %ds remaining", remaining)
	}

	if err := s.repo.Complete(ctx, missionID, playerID, s.cfg.Cooldown()); err != nil {
		return nil, fmt.Errorf("complete mission: %w", err)
	}

	record.Completed = true
	return record, nil
}
