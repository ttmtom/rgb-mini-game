package ledger

import (
	"context"
	"rgb-game/pkg/logger"
	"rgb-game/pkg/pb"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

type LedgerService struct {
	pb.UnimplementedLedgerServiceServer
}

func newLedgerService() *LedgerService {
	return &LedgerService{}
}

func (s *LedgerService) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.BalanceResponse, error) {
	logger.Infof("GetBalance for address %s", req.GetPlayerId())

	// TODO: Implement actual logic to get balance from the ledger
	return &pb.BalanceResponse{
		PlayerId:  req.PlayerId,
		Red:       0,
		Green:     0,
		Blue:      0,
		NextNonce: 0,
	}, nil
}

func (s *LedgerService) SubmitTransaction(ctx context.Context, req *pb.SubmitTransactionRequest) (*pb.SubmitTransactionResponse, error) {
	logger.Infof("SubmitTransaction from sender with public key %s", req.SenderPubKey)

	// TODO: Implement actual logic to submit the transaction
	// 1. Verify the signature
	// 2. Decode the raw payload
	// 3. Validate the transaction (e.g., check balance)
	// 4. Apply the transaction to the ledger

	var payload pb.TransactionPayload
	if err := proto.Unmarshal(req.RawPayload, &payload); err != nil {
		logger.Errorf("Failed to unmarshal transaction payload: %v", err)
		return &pb.SubmitTransactionResponse{
			Success:      false,
			ErrorMessage: "invalid transaction payload",
		}, nil
	}

	logger.Infof("Transaction details: from=%s,to=%s, r=%d, g=%d, b=%d", payload.GetSenderId(), payload.GetReceiverId(), payload.AmountRed, payload.AmountGreen, payload.AmountBlue)

	txHash := uuid.New().String()

	// TODO: Return the new balance of the sender
	return &pb.SubmitTransactionResponse{
		Success: true,
		TxHash:  txHash,
	}, nil
}
