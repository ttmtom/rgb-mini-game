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
	Difficulty      int32 // 0=Easy, 1=Medium, 2=Hard
	RewardRed       int32
	RewardGreen     int32
	RewardBlue      int32
}

// CompleteMissionResult is returned after a CompleteMission attempt.
type CompleteMissionResult struct {
	Success      bool
	ErrorMessage string
	TxHash       string
	NewBalance   *types.PlayerRecord
}

// ProposeChallengeRequest carries data to create a new peer-to-peer challenge.
// The driving adapter verifies the signature before constructing this.
type ProposeChallengeRequest struct {
	ProposerID   string
	OpponentID   string
	OfferRed     uint32
	OfferGreen   uint32
	OfferBlue    uint32
	WantRed      uint32
	WantGreen    uint32
	WantBlue     uint32
	RawPayload   []byte // signed proposer TRANSFER payload bytes
	Signature    []byte
	SenderPubKey []byte
}

// ChallengeResult holds the current state of a challenge.
type ChallengeResult struct {
	ChallengeID  string
	ProposerID   string
	OpponentID   string
	OfferRed     uint32
	OfferGreen   uint32
	OfferBlue    uint32
	WantRed      uint32
	WantGreen    uint32
	WantBlue     uint32
	Status       types.ChallengeStatus
	ExpiresAt    int64
	ErrorMessage string
}

// RespondChallengeRequest carries the opponent's response to a challenge.
type RespondChallengeRequest struct {
	ChallengeID  string
	PlayerID     string
	Accept       bool
	RawPayload   []byte // signed opponent TRANSFER payload bytes (required if Accept=true)
	Signature    []byte
	SenderPubKey []byte
}

// RespondChallengeResult is returned after an opponent responds.
type RespondChallengeResult struct {
	Success        bool
	ErrorMessage   string
	ProposerTxHash string
	OpponentTxHash string
	NewBalance     *types.PlayerRecord
}

// GameUseCase defines the application operations exposed by the Game Server.
type GameUseCase interface {
	// RequestMission issues a new mission for the player, or reports cooldown remaining.
	RequestMission(ctx context.Context, playerID string) (RequestMissionResult, error)

	// CompleteMission validates and completes a mission, mints a reward via the Ledger,
	// and returns the tx hash and updated balance.
	CompleteMission(ctx context.Context, missionID, playerID string) (CompleteMissionResult, error)

	// ProposeChallenge creates a new peer-to-peer token-swap challenge.
	ProposeChallenge(ctx context.Context, req ProposeChallengeRequest) (ChallengeResult, error)

	// RespondChallenge lets the opponent accept or decline a challenge.
	RespondChallenge(ctx context.Context, req RespondChallengeRequest) (RespondChallengeResult, error)

	// GetChallenge returns the current state of a challenge.
	GetChallenge(ctx context.Context, challengeID string) (ChallengeResult, error)
}
