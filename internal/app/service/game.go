package service

import (
	"context"
	"fmt"
	"rgb-game/internal/app/port/in"
	"rgb-game/internal/app/port/out"
	"rgb-game/internal/domain/enum"
	"rgb-game/pkg/logger"
	"sync"
	"time"
)

// GameServiceConfig holds the tunable parameters needed by GameService.
// It is defined here in the application layer to avoid importing the config package.
type GameServiceConfig struct {
	MissionCooldown time.Duration
}

// GameService implements in.GameUseCase.
// It contains no transport-layer imports — all gRPC concerns live in the driving adapter.
type GameService struct {
	missionSvc   *missionService
	auth         out.FullAuthority
	ledgerClient out.LedgerClient
	cfg          GameServiceConfig
	mintMu       sync.Mutex // serialises MINT submissions to prevent authority nonce races
}

// NewGameService wires up the GameService with its driven-port dependencies.
func NewGameService(
	missionRepo out.MissionRepository,
	auth out.FullAuthority,
	ledgerClient out.LedgerClient,
	cfg GameServiceConfig,
) *GameService {
	return &GameService{
		missionSvc:   newMissionService(missionRepo, cfg),
		auth:         auth,
		ledgerClient: ledgerClient,
		cfg:          cfg,
	}
}

// RequestMission issues a new mission for the player, or reports remaining cooldown.
func (s *GameService) RequestMission(ctx context.Context, playerID string) (in.RequestMissionResult, error) {
	logger.Infof("RequestMission for player %s", playerID)

	record, cooldownRemaining, err := s.missionSvc.requestMission(ctx, playerID)
	if err != nil {
		return in.RequestMissionResult{}, err
	}
	if cooldownRemaining > 0 {
		return in.RequestMissionResult{CooldownSeconds: cooldownRemaining}, nil
	}

	return in.RequestMissionResult{
		MissionID:       record.ID,
		RewardColor:     enum.Color(record.RewardColor),
		CooldownSeconds: int32(s.cfg.MissionCooldown.Seconds()),
	}, nil
}

// CompleteMission validates and completes a mission, mints a reward via the Ledger,
// and returns the tx hash and updated player balance.
func (s *GameService) CompleteMission(ctx context.Context, missionID, playerID string) (in.CompleteMissionResult, error) {
	logger.Infof("CompleteMission %s for player %s", missionID, playerID)

	record, err := s.missionSvc.validateAndComplete(ctx, missionID, playerID)
	if err != nil {
		return in.CompleteMissionResult{Success: false, ErrorMessage: err.Error()}, nil
	}

	// Determine reward amounts from color.
	var amtRed, amtGreen, amtBlue uint32
	switch enum.Color(record.RewardColor) {
	case enum.Red:
		amtRed = 1
	case enum.Green:
		amtGreen = 1
	case enum.Blue:
		amtBlue = 1
	}

	// Serialise MINT submissions so concurrent CompleteMission calls don't
	// race for the same authority nonce.
	s.mintMu.Lock()
	defer s.mintMu.Unlock()

	// Fetch authority's current nonce from the Ledger.
	nonce, err := s.ledgerClient.GetAuthorityNonce(ctx, s.auth.PlayerID())
	if err != nil {
		return in.CompleteMissionResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to get authority nonce: %v", err),
		}, nil
	}

	txHash, newBalance, err := s.ledgerClient.SubmitMint(
		ctx, s.auth,
		s.auth.PlayerID(), playerID,
		amtRed, amtGreen, amtBlue,
		nonce,
	)
	if err != nil {
		return in.CompleteMissionResult{Success: false, ErrorMessage: err.Error()}, nil
	}

	logger.Infof("Mission %s completed, tx=%s", missionID, txHash)
	return in.CompleteMissionResult{
		Success:    true,
		TxHash:     txHash,
		NewBalance: newBalance,
	}, nil
}
