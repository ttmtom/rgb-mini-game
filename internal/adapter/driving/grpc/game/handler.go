package grpcgame

import (
	"context"
	"crypto/ed25519"
	"rgb-game/internal/app/port/in"
	"rgb-game/pkg/crypto"
	"rgb-game/pkg/pb"

	"google.golang.org/protobuf/proto"
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
		Difficulty:      pb.MissionDifficulty(result.Difficulty),
		RewardRed:       result.RewardRed,
		RewardGreen:     result.RewardGreen,
		RewardBlue:      result.RewardBlue,
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

// ProposeChallenge implements pb.GameServiceServer.
// Verifies the proposer's signed payload before forwarding to the use case.
func (h *Handler) ProposeChallenge(ctx context.Context, req *pb.ProposeChallengeRequest) (*pb.ChallengeResponse, error) {
	// Verify signature and identity.
	pubKey := ed25519.PublicKey(req.GetSenderPubKey())
	if !ed25519.Verify(pubKey, req.GetRawPayload(), req.GetSignature()) {
		return &pb.ChallengeResponse{ErrorMessage: "invalid proposer signature"}, nil
	}
	derivedID := crypto.PubKeyToPlayerID(pubKey)
	if derivedID != req.GetProposerId() {
		return &pb.ChallengeResponse{ErrorMessage: "sender public key does not match proposer_id"}, nil
	}

	// Verify the payload matches the challenge terms.
	var payload pb.TransactionPayload
	if err := proto.Unmarshal(req.GetRawPayload(), &payload); err != nil {
		return &pb.ChallengeResponse{ErrorMessage: "invalid payload"}, nil
	}
	if payload.GetType() != pb.TransactionPayload_TRANSFER ||
		payload.GetSenderId() != req.GetProposerId() ||
		payload.GetReceiverId() != req.GetOpponentId() ||
		payload.GetAmountRed() != req.GetOfferRed() ||
		payload.GetAmountGreen() != req.GetOfferGreen() ||
		payload.GetAmountBlue() != req.GetOfferBlue() {
		return &pb.ChallengeResponse{ErrorMessage: "payload does not match challenge terms"}, nil
	}

	result, err := h.game.ProposeChallenge(ctx, in.ProposeChallengeRequest{
		ProposerID:   req.GetProposerId(),
		OpponentID:   req.GetOpponentId(),
		OfferRed:     req.GetOfferRed(),
		OfferGreen:   req.GetOfferGreen(),
		OfferBlue:    req.GetOfferBlue(),
		WantRed:      req.GetWantRed(),
		WantGreen:    req.GetWantGreen(),
		WantBlue:     req.GetWantBlue(),
		RawPayload:   req.GetRawPayload(),
		Signature:    req.GetSignature(),
		SenderPubKey: req.GetSenderPubKey(),
	})
	if err != nil {
		return nil, err
	}
	return challengeResultToProto(result), nil
}

// RespondChallenge implements pb.GameServiceServer.
func (h *Handler) RespondChallenge(ctx context.Context, req *pb.RespondChallengeRequest) (*pb.RespondChallengeResponse, error) {
	domainReq := in.RespondChallengeRequest{
		ChallengeID: req.GetChallengeId(),
		PlayerID:    req.GetPlayerId(),
		Accept:      req.GetAccept(),
	}

	if req.GetAccept() {
		pubKey := ed25519.PublicKey(req.GetSenderPubKey())
		if !ed25519.Verify(pubKey, req.GetRawPayload(), req.GetSignature()) {
			return &pb.RespondChallengeResponse{Success: false, ErrorMessage: "invalid opponent signature"}, nil
		}
		derivedID := crypto.PubKeyToPlayerID(pubKey)
		if derivedID != req.GetPlayerId() {
			return &pb.RespondChallengeResponse{Success: false, ErrorMessage: "sender public key does not match player_id"}, nil
		}
		domainReq.RawPayload = req.GetRawPayload()
		domainReq.Signature = req.GetSignature()
		domainReq.SenderPubKey = req.GetSenderPubKey()
	}

	result, err := h.game.RespondChallenge(ctx, domainReq)
	if err != nil {
		return nil, err
	}

	resp := &pb.RespondChallengeResponse{
		Success:        result.Success,
		ErrorMessage:   result.ErrorMessage,
		ProposerTxHash: result.ProposerTxHash,
		OpponentTxHash: result.OpponentTxHash,
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

// GetChallenge implements pb.GameServiceServer.
func (h *Handler) GetChallenge(ctx context.Context, req *pb.GetChallengeRequest) (*pb.ChallengeResponse, error) {
	result, err := h.game.GetChallenge(ctx, req.GetChallengeId())
	if err != nil {
		return nil, err
	}
	return challengeResultToProto(result), nil
}

func challengeResultToProto(r in.ChallengeResult) *pb.ChallengeResponse {
	return &pb.ChallengeResponse{
		ChallengeId:  r.ChallengeID,
		ProposerId:   r.ProposerID,
		OpponentId:   r.OpponentID,
		OfferRed:     r.OfferRed,
		OfferGreen:   r.OfferGreen,
		OfferBlue:    r.OfferBlue,
		WantRed:      r.WantRed,
		WantGreen:    r.WantGreen,
		WantBlue:     r.WantBlue,
		Status:       pb.ChallengeStatus(r.Status),
		ExpiresAt:    r.ExpiresAt,
		ErrorMessage: r.ErrorMessage,
	}
}
