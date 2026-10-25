package service

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"rgb-game/internal/app/port/in"
	"rgb-game/internal/app/port/out"
	"rgb-game/internal/domain/enum"
	"rgb-game/internal/domain/types"
	"rgb-game/pkg/crypto"
	"rgb-game/pkg/logger"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"rgb-game/pkg/pb"
)

// GameServiceConfig holds the tunable parameters needed by GameService.
// It is defined here in the application layer to avoid importing the config package.
type GameServiceConfig struct {
	MissionCooldown time.Duration
	ChallengeTTL    time.Duration // how long a pending challenge lives (default 10 min)
}

// GameService implements in.GameUseCase.
// It contains no transport-layer imports — all gRPC concerns live in the driving adapter.
type GameService struct {
	missionSvc      *missionService
	auth            out.FullAuthority
	ledgerClient    out.LedgerClient
	challengeRepo   out.ChallengeRepository
	cfg             GameServiceConfig
	mintMu          sync.Mutex // serialises MINT submissions to prevent authority nonce races
}

// NewGameService wires up the GameService with its driven-port dependencies.
func NewGameService(
	missionRepo out.MissionRepository,
	auth out.FullAuthority,
	ledgerClient out.LedgerClient,
	challengeRepo out.ChallengeRepository,
	cfg GameServiceConfig,
) *GameService {
	if cfg.ChallengeTTL == 0 {
		cfg.ChallengeTTL = 10 * time.Minute
	}
	return &GameService{
		missionSvc:    newMissionService(missionRepo, cfg),
		auth:          auth,
		ledgerClient:  ledgerClient,
		challengeRepo: challengeRepo,
		cfg:           cfg,
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

	effectiveCooldown := s.missionSvc.effectiveCooldown(record)
	return in.RequestMissionResult{
		MissionID:       record.ID,
		RewardColor:     enum.Color(record.RewardColor),
		CooldownSeconds: int32(effectiveCooldown.Seconds()),
		Difficulty:      record.Difficulty,
		RewardRed:       record.RewardRed,
		RewardGreen:     record.RewardGreen,
		RewardBlue:      record.RewardBlue,
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

	// Serialise MINT submissions so concurrent CompleteMission calls don't
	// race for the same authority nonce.
	s.mintMu.Lock()
	defer s.mintMu.Unlock()

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
		uint32(record.RewardRed), uint32(record.RewardGreen), uint32(record.RewardBlue),
		nonce,
	)
	if err != nil {
		return in.CompleteMissionResult{Success: false, ErrorMessage: err.Error()}, nil
	}

	logger.Infof("Mission %s completed, tx=%s (R=%d G=%d B=%d)", missionID, txHash,
		record.RewardRed, record.RewardGreen, record.RewardBlue)
	return in.CompleteMissionResult{
		Success:    true,
		TxHash:     txHash,
		NewBalance: newBalance,
	}, nil
}

// ProposeChallenge creates a new peer-to-peer token-swap challenge.
// The driving adapter must verify the proposer's signature before calling this.
func (s *GameService) ProposeChallenge(ctx context.Context, req in.ProposeChallengeRequest) (in.ChallengeResult, error) {
	logger.Infof("ProposeChallenge from %s to %s", req.ProposerID, req.OpponentID)

	if req.ProposerID == req.OpponentID {
		return in.ChallengeResult{ErrorMessage: "cannot challenge yourself"}, nil
	}
	if req.OfferRed == 0 && req.OfferGreen == 0 && req.OfferBlue == 0 {
		return in.ChallengeResult{ErrorMessage: "offer must include at least one token"}, nil
	}
	if req.WantRed == 0 && req.WantGreen == 0 && req.WantBlue == 0 {
		return in.ChallengeResult{ErrorMessage: "want amounts must include at least one token"}, nil
	}

	// Verify the pre-signed proposer transfer payload.
	if err := verifyTransferPayload(req.RawPayload, req.Signature, req.SenderPubKey,
		req.ProposerID, req.OpponentID, req.OfferRed, req.OfferGreen, req.OfferBlue); err != nil {
		return in.ChallengeResult{ErrorMessage: fmt.Sprintf("invalid proposer signature: %v", err)}, nil
	}

	now := time.Now()
	record := &types.ChallengeRecord{
		ID:           uuid.New().String(),
		ProposerID:   req.ProposerID,
		OpponentID:   req.OpponentID,
		OfferRed:     req.OfferRed,
		OfferGreen:   req.OfferGreen,
		OfferBlue:    req.OfferBlue,
		WantRed:      req.WantRed,
		WantGreen:    req.WantGreen,
		WantBlue:     req.WantBlue,
		Status:       types.ChallengePending,
		CreatedAt:    now.Unix(),
		ExpiresAt:    now.Add(s.cfg.ChallengeTTL).Unix(),
		RawPayload:   req.RawPayload,
		Signature:    req.Signature,
		SenderPubKey: req.SenderPubKey,
	}

	if err := s.challengeRepo.Create(ctx, record, s.cfg.ChallengeTTL); err != nil {
		return in.ChallengeResult{}, fmt.Errorf("store challenge: %w", err)
	}

	logger.Infof("Challenge %s created (proposer=%s opponent=%s)", record.ID, req.ProposerID, req.OpponentID)
	return challengeToResult(record), nil
}

// RespondChallenge lets the opponent accept or decline a challenge.
func (s *GameService) RespondChallenge(ctx context.Context, req in.RespondChallengeRequest) (in.RespondChallengeResult, error) {
	logger.Infof("RespondChallenge %s by %s (accept=%v)", req.ChallengeID, req.PlayerID, req.Accept)

	record, err := s.challengeRepo.FindByID(ctx, req.ChallengeID)
	if err != nil {
		return in.RespondChallengeResult{}, fmt.Errorf("find challenge: %w", err)
	}
	if record == nil {
		return in.RespondChallengeResult{Success: false, ErrorMessage: "challenge not found or expired"}, nil
	}
	if record.OpponentID != req.PlayerID {
		return in.RespondChallengeResult{Success: false, ErrorMessage: "you are not the opponent of this challenge"}, nil
	}
	if record.Status != types.ChallengePending {
		return in.RespondChallengeResult{Success: false, ErrorMessage: "challenge is no longer pending"}, nil
	}

	if !req.Accept {
		if err := s.challengeRepo.UpdateStatus(ctx, req.ChallengeID, types.ChallengeDeclined); err != nil {
			return in.RespondChallengeResult{}, fmt.Errorf("update challenge status: %w", err)
		}
		logger.Infof("Challenge %s declined by %s", req.ChallengeID, req.PlayerID)
		return in.RespondChallengeResult{Success: true}, nil
	}

	// Verify the opponent's pre-signed transfer payload (opponent → proposer).
	if err := verifyTransferPayload(req.RawPayload, req.Signature, req.SenderPubKey,
		req.PlayerID, record.ProposerID, record.WantRed, record.WantGreen, record.WantBlue); err != nil {
		return in.RespondChallengeResult{Success: false, ErrorMessage: fmt.Sprintf("invalid opponent signature: %v", err)}, nil
	}

	// Submit both transfers to the Ledger sequentially.
	// Proposer → Opponent first.
	proposerTxResp, err := s.ledgerClient.SubmitSignedTransfer(ctx, record.RawPayload, record.Signature, record.SenderPubKey)
	if err != nil || !proposerTxResp.Success {
		errMsg := "proposer transfer failed"
		if err != nil {
			errMsg = err.Error()
		} else {
			errMsg = proposerTxResp.ErrorMessage
		}
		return in.RespondChallengeResult{Success: false, ErrorMessage: errMsg}, nil
	}

	// Opponent → Proposer.
	opponentTxResp, err := s.ledgerClient.SubmitSignedTransfer(ctx, req.RawPayload, req.Signature, req.SenderPubKey)
	if err != nil || !opponentTxResp.Success {
		errMsg := "opponent transfer failed"
		if err != nil {
			errMsg = err.Error()
		} else {
			errMsg = opponentTxResp.ErrorMessage
		}
		// Best-effort: mark challenge as failed (completed with error).
		_ = s.challengeRepo.UpdateStatus(ctx, req.ChallengeID, types.ChallengeDeclined)
		return in.RespondChallengeResult{Success: false, ErrorMessage: errMsg}, nil
	}

	if err := s.challengeRepo.UpdateStatus(ctx, req.ChallengeID, types.ChallengeCompleted); err != nil {
		logger.Warnf("Challenge %s: failed to mark completed: %v", req.ChallengeID, err)
	}

	logger.Infof("Challenge %s completed: proposerTx=%s opponentTx=%s", req.ChallengeID,
		proposerTxResp.TxHash, opponentTxResp.TxHash)

	return in.RespondChallengeResult{
		Success:        true,
		ProposerTxHash: proposerTxResp.TxHash,
		OpponentTxHash: opponentTxResp.TxHash,
		NewBalance:     opponentTxResp.NewBalance,
	}, nil
}

// GetChallenge returns the current state of a challenge.
func (s *GameService) GetChallenge(ctx context.Context, challengeID string) (in.ChallengeResult, error) {
	record, err := s.challengeRepo.FindByID(ctx, challengeID)
	if err != nil {
		return in.ChallengeResult{}, fmt.Errorf("find challenge: %w", err)
	}
	if record == nil {
		return in.ChallengeResult{ErrorMessage: "challenge not found or expired"}, nil
	}
	return challengeToResult(record), nil
}

// verifyTransferPayload checks that the signed payload matches the expected transfer parameters.
func verifyTransferPayload(rawPayload, signature, senderPubKey []byte, senderID, receiverID string,
	amtRed, amtGreen, amtBlue uint32) error {

	pubKey := ed25519.PublicKey(senderPubKey)
	if !ed25519.Verify(pubKey, rawPayload, signature) {
		return fmt.Errorf("signature verification failed")
	}

	derivedID := crypto.PubKeyToPlayerID(pubKey)
	if derivedID != senderID {
		return fmt.Errorf("sender public key does not match sender_id")
	}

	var payload pb.TransactionPayload
	if err := proto.Unmarshal(rawPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	if payload.GetType() != pb.TransactionPayload_TRANSFER {
		return fmt.Errorf("payload must be a TRANSFER")
	}
	if payload.GetSenderId() != senderID {
		return fmt.Errorf("payload sender_id mismatch")
	}
	if payload.GetReceiverId() != receiverID {
		return fmt.Errorf("payload receiver_id mismatch")
	}
	if payload.GetAmountRed() != amtRed || payload.GetAmountGreen() != amtGreen || payload.GetAmountBlue() != amtBlue {
		return fmt.Errorf("payload amounts do not match challenge terms")
	}
	return nil
}

func challengeToResult(r *types.ChallengeRecord) in.ChallengeResult {
	return in.ChallengeResult{
		ChallengeID: r.ID,
		ProposerID:  r.ProposerID,
		OpponentID:  r.OpponentID,
		OfferRed:    r.OfferRed,
		OfferGreen:  r.OfferGreen,
		OfferBlue:   r.OfferBlue,
		WantRed:     r.WantRed,
		WantGreen:   r.WantGreen,
		WantBlue:    r.WantBlue,
		Status:      r.Status,
		ExpiresAt:   r.ExpiresAt,
	}
}
