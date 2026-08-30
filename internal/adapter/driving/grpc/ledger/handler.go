package grpcledger

import (
	"context"
	"crypto/ed25519"
	"rgb-game/internal/app/port/in"
	"rgb-game/internal/app/port/out"
	"rgb-game/pkg/crypto"
	"rgb-game/pkg/logger"
	"rgb-game/pkg/pb"

	"google.golang.org/protobuf/proto"
)

// Handler is the gRPC driving adapter for the Ledger service.
// It translates pb ↔ domain types and delegates to LedgerUseCase.
type Handler struct {
	pb.UnimplementedLedgerServiceServer
	ledger        in.LedgerUseCase
	authorityRepo out.AuthorityRepository
}

// New creates a new Ledger gRPC handler.
func New(ledger in.LedgerUseCase, authorityRepo out.AuthorityRepository) *Handler {
	return &Handler{ledger: ledger, authorityRepo: authorityRepo}
}

// GetBalance implements pb.LedgerServiceServer.
func (h *Handler) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.BalanceResponse, error) {
	player, err := h.ledger.GetBalance(ctx, req.GetPlayerId())
	if err != nil {
		return nil, err
	}
	if player == nil {
		return &pb.BalanceResponse{
			PlayerId:  req.GetPlayerId(),
			Red:       0,
			Green:     0,
			Blue:      0,
			NextNonce: 0,
		}, nil
	}
	return &pb.BalanceResponse{
		PlayerId:  player.ID,
		Red:       player.Red,
		Green:     player.Green,
		Blue:      player.Blue,
		NextNonce: player.Nonce,
	}, nil
}

// SubmitTransaction implements pb.LedgerServiceServer.
// This method performs ALL cryptographic verification before delegating to the use case:
//  1. Unmarshal raw payload.
//  2. Verify ed25519 signature.
//  3. Verify sha256(senderPubKey) == payload.sender_id.
//  4. For MINT: verify sender is the authority.
func (h *Handler) SubmitTransaction(ctx context.Context, req *pb.SubmitTransactionRequest) (*pb.SubmitTransactionResponse, error) {
	logger.Infof("SubmitTransaction from sender pub key %x", req.GetSenderPubKey())

	// 1. Unmarshal raw payload.
	var payload pb.TransactionPayload
	if err := proto.Unmarshal(req.GetRawPayload(), &payload); err != nil {
		logger.Errorf("Failed to unmarshal transaction payload: %v", err)
		return &pb.SubmitTransactionResponse{Success: false, ErrorMessage: "invalid transaction payload"}, nil
	}

	pubKey := ed25519.PublicKey(req.GetSenderPubKey())

	// 2. Verify ed25519 signature.
	if !ed25519.Verify(pubKey, req.GetRawPayload(), req.GetSignature()) {
		logger.Warnf("Invalid signature for sender %s", payload.GetSenderId())
		return &pb.SubmitTransactionResponse{Success: false, ErrorMessage: "invalid signature"}, nil
	}

	// 3. Verify sender_pub_key matches sender_id.
	derivedID := crypto.PubKeyToPlayerID(pubKey)
	if derivedID != payload.GetSenderId() {
		logger.Warnf("Sender ID mismatch: derived=%s, payload=%s", derivedID, payload.GetSenderId())
		return &pb.SubmitTransactionResponse{Success: false, ErrorMessage: "sender public key does not match sender_id"}, nil
	}

	// 4. For MINT: verify sender is a registered authority.
	isMint := payload.GetType() == pb.TransactionPayload_MINT
	if isMint {
		isAuth, err := h.authorityRepo.Exists(ctx, pubKey)
		if err != nil {
			logger.Errorf("Authority lookup failed: %v", err)
			return &pb.SubmitTransactionResponse{Success: false, ErrorMessage: "authority lookup failed"}, nil
		}
		if !isAuth {
			logger.Warnf("Unauthorized MINT attempt from %s", payload.GetSenderId())
			return &pb.SubmitTransactionResponse{Success: false, ErrorMessage: "sender is not a registered authority"}, nil
		}
	}

	txType := in.TxTypeTransfer
	if isMint {
		txType = in.TxTypeMint
	}

	domainReq := in.SubmitTransactionRequest{
		TxType:      txType,
		SenderID:    payload.GetSenderId(),
		ReceiverID:  payload.GetReceiverId(),
		AmountRed:   payload.GetAmountRed(),
		AmountGreen: payload.GetAmountGreen(),
		AmountBlue:  payload.GetAmountBlue(),
		Nonce:       payload.GetNonce(),
	}

	result, err := h.ledger.SubmitTransaction(ctx, domainReq)
	if err != nil {
		return nil, err
	}

	resp := &pb.SubmitTransactionResponse{
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

// RegisterAuthority implements pb.LedgerServiceServer.
// The caller must prove key ownership by signing their own public key bytes.
func (h *Handler) RegisterAuthority(ctx context.Context, req *pb.RegisterAuthorityRequest) (*pb.RegisterAuthorityResponse, error) {
	logger.Infof("RegisterAuthority from pub key %x", req.GetPubKey())

	domainReq := in.RegisterAuthorityRequest{
		PubKey:    req.GetPubKey(),
		Signature: req.GetSignature(),
	}

	result, err := h.ledger.RegisterAuthority(ctx, domainReq)
	if err != nil {
		return nil, err
	}
	return &pb.RegisterAuthorityResponse{
		Success:      result.Success,
		ErrorMessage: result.ErrorMessage,
		AuthorityId:  result.AuthorityID,
	}, nil
}
