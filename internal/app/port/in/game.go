package in

import (
	"context"
	"rgb-game/internal/domain/enum"
	"rgb-game/internal/domain/types"
)

// RequestMissionResult is returned when a mission is issued or the player is on cooldown.
type RequestMissionResult struct {
	// MissionID is empty when the player is still on cooldown.
	MissionID       string
	RewardColor     enum.Color
	CooldownSeconds int32
}

// CompleteMissionResult is returned after a CompleteMission attempt.
type CompleteMissionResult struct {
	Success      bool
	ErrorMessage string
	TxHash       string
	NewBalance   *types.PlayerRecord
}

// GameUseCase defines the application operations exposed by the Game Server.
type GameUseCase interface {
	// RequestMission issues a new mission for the player, or reports cooldown remaining.
	RequestMission(ctx context.Context, playerID string) (RequestMissionResult, error)

	// CompleteMission validates and completes a mission, mints a reward via the Ledger,
	// and returns the tx hash and updated balance.
	CompleteMission(ctx context.Context, missionID, playerID string) (CompleteMissionResult, error)
}
