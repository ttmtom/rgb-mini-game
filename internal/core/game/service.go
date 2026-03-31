package game

import (
	"context"
	"encoding/hex"
	"fmt"
	"rgb-game/config"
	"rgb-game/internal/core/interfaces"
	missionpkg "rgb-game/internal/core/mission"
	"rgb-game/pkg/logger"
	"rgb-game/pkg/pb"
	"sync"

	"google.golang.org/protobuf/proto"
)

// GameService implements pb.GameServiceServer.
type GameService struct {
	pb.UnimplementedGameServiceServer

	missionSvc   *missionpkg.MissionService
	auth         interfaces.FullAuthority
	ledgerClient pb.LedgerServiceClient
	cfg          *config.GameConfig
	mintMu       sync.Mutex // serialises MINT submissions to avoid authority nonce conflicts
}

func newGameService(
	missionSvc *missionpkg.MissionService,
	auth interfaces.FullAuthority,
	ledgerClient pb.LedgerServiceClient,
	cfg *config.GameConfig,
) *GameService {
	return &GameService{
		missionSvc:   missionSvc,
		auth:         auth,
		ledgerClient: ledgerClient,
		cfg:          cfg,
	}
}

// RequestMission issues a new mission for the given player.
func (s *GameService) RequestMission(ctx context.Context, req *pb.RequestMissionRequest) (*pb.MissionResponse, error) {
	playerID := req.GetPlayerId()
	logger.Infof("RequestMission for player %s", playerID)

	record, cooldownRemaining, err := s.missionSvc.RequestMission(ctx, playerID)
	if err != nil {
		return nil, err
	}
	if cooldownRemaining > 0 {
		return &pb.MissionResponse{CooldownSeconds: cooldownRemaining}, nil
	}

	return &pb.MissionResponse{
		MissionId:       record.ID,
		RewardColor:     pb.RewardColor(record.RewardColor),
		CooldownSeconds: int32(s.cfg.Cooldown().Seconds()),
	}, nil
}

// CompleteMission processes mission completion, mints a reward via the Ledger, and returns the new balance.
func (s *GameService) CompleteMission(ctx context.Context, req *pb.CompleteMissionRequest) (*pb.CompleteMissionResponse, error) {
	missionID := req.GetMissionId()
	playerID := req.GetPlayerId()
	logger.Infof("CompleteMission %s for player %s", missionID, playerID)

	record, err := s.missionSvc.ValidateAndComplete(ctx, missionID, playerID)
	if err != nil {
		return &pb.CompleteMissionResponse{Success: false, ErrorMessage: err.Error()}, nil
	}

	// Serialise MINT submissions so concurrent CompleteMission calls don't
	// race for the same authority nonce.
	s.mintMu.Lock()
	defer s.mintMu.Unlock()

	// Fetch authority's current nonce from the Ledger.
	balResp, err := s.ledgerClient.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: s.auth.PlayerID()})
	if err != nil {
		return &pb.CompleteMissionResponse{Success: false, ErrorMessage: fmt.Sprintf("failed to get authority nonce: %v", err)}, nil
	}

	// Build the MINT payload.
	var amtRed, amtGreen, amtBlue uint32
	switch pb.RewardColor(record.RewardColor) {
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
