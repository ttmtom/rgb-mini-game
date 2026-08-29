package service

import (
	"context"
	"fmt"
	"rgb-game/internal/app/port/in"
	"rgb-game/internal/app/port/out"
	"rgb-game/internal/domain/types"
	"rgb-game/pkg/logger"
	"time"

	"github.com/google/uuid"
)

// LedgerService implements in.LedgerUseCase.
// It contains no transport-layer imports — all gRPC concerns live in the driving adapter.
type LedgerService struct {
	playerRepo out.PlayerRepository
	txRepo     out.TransactionRepository
	transactor out.Transactor
	gameEngine out.GameEngine
}

// NewLedgerService wires up the LedgerService with its driven-port dependencies.
func NewLedgerService(
	playerRepo out.PlayerRepository,
	txRepo out.TransactionRepository,
	transactor out.Transactor,
	gameEngine out.GameEngine,
) *LedgerService {
	return &LedgerService{
		playerRepo: playerRepo,
		txRepo:     txRepo,
		transactor: transactor,
		gameEngine: gameEngine,
	}
}

// GetBalance returns the player record (R/G/B + nonce), or nil if not found.
func (s *LedgerService) GetBalance(ctx context.Context, playerID string) (*types.PlayerRecord, error) {
	logger.Infof("GetBalance for player %s", playerID)
	return s.playerRepo.Find(ctx, playerID)
}

// SubmitTransaction processes a pre-verified, typed transaction request.
// Signature verification and authority checks must be performed by the
// driving adapter before calling this method.
func (s *LedgerService) SubmitTransaction(ctx context.Context, req in.SubmitTransactionRequest) (in.SubmitTransactionResult, error) {
	logger.Infof("SubmitTransaction type=%d sender=%s receiver=%s", req.TxType, req.SenderID, req.ReceiverID)

	// Validate required fields.
	if req.SenderID == "" || req.ReceiverID == "" {
		return in.SubmitTransactionResult{Success: false, ErrorMessage: "sender_id and receiver_id must not be empty"}, nil
	}

	isMint := req.TxType == in.TxTypeMint

	// Validate amounts.
	// TRANSFER: max 255 per channel; MINT: max 127 (int8 delta cap).
	maxAmount := uint32(255)
	if isMint {
		maxAmount = 127
	}
	if req.AmountRed > maxAmount || req.AmountGreen > maxAmount || req.AmountBlue > maxAmount {
		return in.SubmitTransactionResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("amount exceeds maximum %d per channel", maxAmount),
		}, nil
	}

	txHash := uuid.New().String()
	var newBalance *types.PlayerRecord

	err := s.transactor.InTransaction(ctx, func(txCtx context.Context) error {
		senderModel, err := s.playerRepo.FindOrCreate(txCtx, req.SenderID)
		if err != nil {
			return fmt.Errorf("failed to find/create sender: %w", err)
		}
		receiverModel, err := s.playerRepo.FindOrCreate(txCtx, req.ReceiverID)
		if err != nil {
			return fmt.Errorf("failed to find/create receiver: %w", err)
		}

		// Nonce check — replay protection.
		if senderModel.Nonce != req.Nonce {
			return fmt.Errorf("nonce mismatch: expected %d, got %d", senderModel.Nonce, req.Nonce)
		}

		if isMint {
			receiverState := &types.PlayerState{
				R:     uint8(receiverModel.Red),
				G:     uint8(receiverModel.Green),
				B:     uint8(receiverModel.Blue),
				Nonce: receiverModel.Nonce,
			}
			mission := &types.Mission{
				Reward: types.RGB{
					R: int8(req.AmountRed),
					G: int8(req.AmountGreen),
					B: int8(req.AmountBlue),
				},
			}
			newReceiverState, err := s.gameEngine.PlayerCompleteMission(receiverState, mission)
			if err != nil {
				return fmt.Errorf("PlayerCompleteMission failed: %w", err)
			}

			receiverModel.Red = uint32(newReceiverState.R)
			receiverModel.Green = uint32(newReceiverState.G)
			receiverModel.Blue = uint32(newReceiverState.B)
			receiverModel.Nonce = newReceiverState.Nonce
			senderModel.Nonce++ // increment authority nonce for replay protection

			if err := s.playerRepo.UpdateBalance(txCtx, receiverModel); err != nil {
				return fmt.Errorf("failed to update receiver balance: %w", err)
			}
			if err := s.playerRepo.UpdateBalance(txCtx, senderModel); err != nil {
				return fmt.Errorf("failed to update sender nonce: %w", err)
			}

			newBalance = &types.PlayerRecord{
				ID:    receiverModel.ID,
				Red:   receiverModel.Red,
				Green: receiverModel.Green,
				Blue:  receiverModel.Blue,
				Nonce: receiverModel.Nonce,
			}
		} else {
			// TRANSFER: validate sufficient balance.
			if senderModel.Red < req.AmountRed ||
				senderModel.Green < req.AmountGreen ||
				senderModel.Blue < req.AmountBlue {
				return fmt.Errorf("insufficient balance: have R=%d G=%d B=%d, need R=%d G=%d B=%d",
					senderModel.Red, senderModel.Green, senderModel.Blue,
					req.AmountRed, req.AmountGreen, req.AmountBlue)
			}

			senderState := &types.PlayerState{
				R:     uint8(req.AmountRed),
				G:     uint8(req.AmountGreen),
				B:     uint8(req.AmountBlue),
				Nonce: senderModel.Nonce,
			}
			receiverState := &types.PlayerState{
				R:     uint8(receiverModel.Red),
				G:     uint8(receiverModel.Green),
				B:     uint8(receiverModel.Blue),
				Nonce: receiverModel.Nonce,
			}

			newSenderState, newReceiverState, err := s.gameEngine.PlayerTransactions(senderState, receiverState)
			if err != nil {
				return fmt.Errorf("PlayerTransactions failed: %w", err)
			}

			senderModel.Red -= req.AmountRed
			senderModel.Green -= req.AmountGreen
			senderModel.Blue -= req.AmountBlue
			senderModel.Nonce = newSenderState.Nonce

			receiverModel.Red = uint32(newReceiverState.R)
			receiverModel.Green = uint32(newReceiverState.G)
			receiverModel.Blue = uint32(newReceiverState.B)
			receiverModel.Nonce = newReceiverState.Nonce

			if err := s.playerRepo.UpdateBalance(txCtx, senderModel); err != nil {
				return fmt.Errorf("failed to update sender balance: %w", err)
			}
			if err := s.playerRepo.UpdateBalance(txCtx, receiverModel); err != nil {
				return fmt.Errorf("failed to update receiver balance: %w", err)
			}

			newBalance = &types.PlayerRecord{
				ID:    senderModel.ID,
				Red:   senderModel.Red,
				Green: senderModel.Green,
				Blue:  senderModel.Blue,
				Nonce: senderModel.Nonce,
			}
		}

		return s.txRepo.Create(txCtx, &types.TransactionRecord{
			Hash:       txHash,
			Type:       uint8(req.TxType),
			SenderID:   req.SenderID,
			ReceiverID: req.ReceiverID,
			Red:        req.AmountRed,
			Green:      req.AmountGreen,
			Blue:       req.AmountBlue,
			Nonce:      req.Nonce,
			Timestamp:  time.Now().Unix(),
		})
	})

	if err != nil {
		logger.Errorf("SubmitTransaction failed: %v", err)
		return in.SubmitTransactionResult{Success: false, ErrorMessage: err.Error()}, nil
	}

	logger.Infof("Transaction %s committed successfully", txHash)
	return in.SubmitTransactionResult{
		Success:    true,
		TxHash:     txHash,
		NewBalance: newBalance,
	}, nil
}
