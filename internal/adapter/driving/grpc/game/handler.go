package grpcgame

import (
	"context"
	"rgb-game/internal/app/port/in"
	"rgb-game/pkg/pb"
)

// Handler is the gRPC driving adapter for the Game service.
// It translates pb ↔ domain types and delegates to GameUseCase.
type Handler struct {
	pb.UnimplementedGameServiceServer
	game in.GameUseCase
}

// New creates a new Game gRPC handler.
func New(game in.GameUseCase) *Handler {
	return &Handler{game: game}
}

// RequestMission implements pb.GameServiceServer.
func (h *Handler) RequestMission(ctx context.Context, req *pb.RequestMissionRequest) (*pb.MissionResponse, error) {
	result, err := h.game.RequestMission(ctx, req.GetPlayerId())
	if err != nil {
		return nil, err
	}
	return &pb.MissionResponse{
		MissionId:       result.MissionID,
		RewardColor:     pb.RewardColor(result.RewardColor),
		CooldownSeconds: result.CooldownSeconds,
	}, nil
}

// CompleteMission implements pb.GameServiceServer.
func (h *Handler) CompleteMission(ctx context.Context, req *pb.CompleteMissionRequest) (*pb.CompleteMissionResponse, error) {
	result, err := h.game.CompleteMission(ctx, req.GetMissionId(), req.GetPlayerId())
	if err != nil {
		return nil, err
	}

	resp := &pb.CompleteMissionResponse{
		Success:      result.Success,
		ErrorMessage: result.ErrorMessage,
		TxHash:       result.TxHash,
	}
	if result.NewBalance != nil {
		resp.NewBalance = &pb.BalanceResponse{
			PlayerId:  result.NewBalance.ID,
			Red:       result.NewBalance.Red,
			Green:     result.NewBalance.Green,
			Blue:      result.NewBalance.Blue,
			NextNonce: result.NewBalance.Nonce,
		}
	}
	return resp, nil
}
