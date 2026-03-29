package game

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"rgb-game/config"
	"sync"
	"time"

	"rgb-game/internal/core/interfaces"
	"rgb-game/pkg/logger"
	"rgb-game/pkg/pb"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

// missionRecord holds the in-memory state of an issued mission.
type missionRecord struct {
	PlayerID    string
	RewardColor pb.RewardColor
	IssuedAt    time.Time
}

// GameService implements pb.GameServiceServer.
type GameService struct {
	pb.UnimplementedGameServiceServer

	mu                  sync.RWMutex
	missions            map[string]*missionRecord // missionID → record
	playerActiveMission map[string]string         // playerID → active missionID
	playerLastComplete  map[string]time.Time      // playerID → last completion time

	auth         interfaces.FullAuthority
	ledgerClient pb.LedgerServiceClient
	cfg          *config.GameConfig
}

func newGameService(
	auth interfaces.FullAuthority,
	ledgerClient pb.LedgerServiceClient,
	cfg *config.GameConfig,
) *GameService {
	return &GameService{
		missions:            make(map[string]*missionRecord),
		playerActiveMission: make(map[string]string),
		playerLastComplete:  make(map[string]time.Time),
		auth:                auth,
		ledgerClient:        ledgerClient,
		cfg:                 cfg,
	}
}

// RequestMission issues a new mission for the given player.
func (s *GameService) RequestMission(_ context.Context, req *pb.RequestMissionRequest) (*pb.MissionResponse, error) {
	playerID := req.GetPlayerId()
	logger.Infof("RequestMission for player %s", playerID)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Reject if player already has an active (uncompleted) mission.
	if activeMissionID, ok := s.playerActiveMission[playerID]; ok {
		return nil, fmt.Errorf("player %s already has an active mission %s", playerID, activeMissionID)
	}

	// Enforce cooldown since last completion.
	if lastComplete, ok := s.playerLastComplete[playerID]; ok {
		elapsed := time.Since(lastComplete)
		if elapsed < s.cfg.Cooldown() {
			remaining := int32((s.cfg.Cooldown() - elapsed).Seconds())
			logger.Infof("Player %s is on cooldown, %ds remaining", playerID, remaining)
			return &pb.MissionResponse{
				CooldownSeconds: remaining,
			}, nil
		}
	}

	// Issue new mission.
	missionID := uuid.New().String()
	rewardColor := pb.RewardColor(rand.Intn(3)) // RED=0, GREEN=1, BLUE=2

	s.missions[missionID] = &missionRecord{
		PlayerID:    playerID,
		RewardColor: rewardColor,
		IssuedAt:    time.Now(),
	}
	s.playerActiveMission[playerID] = missionID

	logger.Infof("Issued mission %s to player %s (reward=%s)", missionID, playerID, rewardColor)
	return &pb.MissionResponse{
		MissionId:       missionID,
		RewardColor:     rewardColor,
		CooldownSeconds: int32(s.cfg.Cooldown().Seconds()),
	}, nil
}

// CompleteMission processes mission completion, mints a reward via the Ledger, and returns the new balance.
func (s *GameService) CompleteMission(ctx context.Context, req *pb.CompleteMissionRequest) (*pb.CompleteMissionResponse, error) {
	missionID := req.GetMissionId()
	playerID := req.GetPlayerId()
	logger.Infof("CompleteMission %s for player %s", missionID, playerID)

	s.mu.Lock()
	record, exists := s.missions[missionID]
	if !exists {
		s.mu.Unlock()
		return &pb.CompleteMissionResponse{Success: false, ErrorMessage: "mission not found"}, nil
	}
	if record.PlayerID != playerID {
		s.mu.Unlock()
		return &pb.CompleteMissionResponse{Success: false, ErrorMessage: "mission does not belong to this player"}, nil
	}

	// Cooldown must have elapsed since the mission was issued.
	elapsed := time.Since(record.IssuedAt)
	if elapsed < s.cfg.Cooldown() {
		remaining := int32((s.cfg.Cooldown() - elapsed).Seconds())
		s.mu.Unlock()
		return &pb.CompleteMissionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("mission not yet ready, %ds remaining", remaining),
		}, nil
	}

	rewardColor := record.RewardColor
	s.mu.Unlock()

	// Fetch authority's current nonce from the Ledger.
	balResp, err := s.ledgerClient.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: s.auth.PlayerID()})
	if err != nil {
		return &pb.CompleteMissionResponse{Success: false, ErrorMessage: fmt.Sprintf("failed to get authority nonce: %v", err)}, nil
	}

	// Build the MINT payload.
	var amtRed, amtGreen, amtBlue uint32
	switch rewardColor {
	case pb.RewardColor_RED:
		amtRed = 1
	case pb.RewardColor_GREEN:
		amtGreen = 1
	case pb.RewardColor_BLUE:
		amtBlue = 1
	}

	payload := &pb.TransactionPayload{
		Type:        pb.TransactionPayload_MINT,
		SenderId:    s.auth.PlayerID(),
		ReceiverId:  playerID,
		AmountRed:   amtRed,
		AmountGreen: amtGreen,
		AmountBlue:  amtBlue,
		Nonce:       balResp.GetNextNonce(),
		Timestamp:   time.Now().Unix(),
	}

	rawPayload, err := proto.Marshal(payload)
	if err != nil {
		return &pb.CompleteMissionResponse{Success: false, ErrorMessage: fmt.Sprintf("failed to marshal payload: %v", err)}, nil
	}

	signature := s.auth.Sign(rawPayload)

	// Submit the MINT transaction to the Ledger.
	txResp, err := s.ledgerClient.SubmitTransaction(ctx, &pb.SubmitTransactionRequest{
		RawPayload:   rawPayload,
		Signature:    signature,
		SenderPubKey: s.auth.PubKey(),
	})
	if err != nil {
		return &pb.CompleteMissionResponse{Success: false, ErrorMessage: fmt.Sprintf("ledger submit failed: %v", err)}, nil
	}
	if !txResp.GetSuccess() {
		return &pb.CompleteMissionResponse{Success: false, ErrorMessage: txResp.GetErrorMessage()}, nil
	}

	// Clean up mission state.
	s.mu.Lock()
	delete(s.missions, missionID)
	delete(s.playerActiveMission, playerID)
	s.playerLastComplete[playerID] = time.Now()
	s.mu.Unlock()

	logger.Infof("Mission %s completed, tx=%s", missionID, txResp.GetTxHash())
	return &pb.CompleteMissionResponse{
		Success:    true,
		TxHash:     txResp.GetTxHash(),
		NewBalance: txResp.GetNewBalance(),
	}, nil
}

// AuthorityInfo returns the authority player ID and hex-encoded public key (for operator setup).
func (s *GameService) AuthorityInfo() (id string, pubKeyHex string) {
	return s.auth.PlayerID(), hex.EncodeToString(s.auth.PubKey())
}
