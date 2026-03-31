package ledger

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"fmt"
	"rgb-game/internal/core/interfaces"
	"rgb-game/internal/core/types"
	"rgb-game/pkg/crypto"
	"rgb-game/pkg/logger"
	"rgb-game/pkg/pb"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

type LedgerService struct {
	pb.UnimplementedLedgerServiceServer
	db         *gorm.DB
	playerRepo interfaces.PlayerRepository
	txRepo     interfaces.TransactionRepository
	gameEngine interfaces.GameEngine
	auth       interfaces.PublicAuthority
}

func newLedgerService(
	db *gorm.DB,
	playerRepo interfaces.PlayerRepository,
	txRepo interfaces.TransactionRepository,
	gameEngine interfaces.GameEngine,
	auth interfaces.PublicAuthority,
) *LedgerService {
	return &LedgerService{
		db:         db,
		playerRepo: playerRepo,
		txRepo:     txRepo,
		gameEngine: gameEngine,
		auth:       auth,
	}
}

func (s *LedgerService) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.BalanceResponse, error) {
	logger.Infof("GetBalance for player %s", req.GetPlayerId())

	player, err := s.playerRepo.Find(s.db, req.GetPlayerId())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch player: %w", err)
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

func (s *LedgerService) SubmitTransaction(ctx context.Context, req *pb.SubmitTransactionRequest) (*pb.SubmitTransactionResponse, error) {
	logger.Infof("SubmitTransaction from sender pub key %x", req.GetSenderPubKey())

	// 1. Unmarshal raw payload
	var payload pb.TransactionPayload
	if err := proto.Unmarshal(req.GetRawPayload(), &payload); err != nil {
		logger.Errorf("Failed to unmarshal transaction payload: %v", err)
		return &pb.SubmitTransactionResponse{Success: false, ErrorMessage: "invalid transaction payload"}, nil
	}

	pubKey := ed25519.PublicKey(req.GetSenderPubKey())

	// 2. Verify ed25519 signature
	if !ed25519.Verify(pubKey, req.GetRawPayload(), req.GetSignature()) {
		logger.Warnf("Invalid signature for sender %s", payload.GetSenderId())
		return &pb.SubmitTransactionResponse{Success: false, ErrorMessage: "invalid signature"}, nil
	}

	// 3. Verify sender_pub_key matches sender_id
	derivedID := crypto.PubKeyToPlayerID(pubKey)
	if derivedID != payload.GetSenderId() {
		logger.Warnf("Sender ID mismatch: derived=%s, payload=%s", derivedID, payload.GetSenderId())
		return &pb.SubmitTransactionResponse{Success: false, ErrorMessage: "sender public key does not match sender_id"}, nil
	}

	// 4. For MINT: only the authority key is allowed
	isMint := payload.GetType() == pb.TransactionPayload_MINT
	if isMint && !bytes.Equal(pubKey, s.auth.PubKey()) {
		logger.Warnf("Unauthorized MINT attempt from %s", payload.GetSenderId())
		return &pb.SubmitTransactionResponse{Success: false, ErrorMessage: "only authority can mint"}, nil
	}

	// 5. Validate required fields.
	if payload.GetSenderId() == "" || payload.GetReceiverId() == "" {
		return &pb.SubmitTransactionResponse{Success: false, ErrorMessage: "sender_id and receiver_id must not be empty"}, nil
	}

	// 6. Validate amounts fit their type constraints:
	//    TRANSFER: each channel must be [0, 255] — max balance per channel (stored as uint8 in PlayerState).
	//    MINT:     each channel must be [0, 127] — amounts are cast to int8 for the game-engine delta;
	//              values 128-255 would wrap to negative, subtracting from the receiver instead of adding.
	maxAmount := uint32(255)
	if isMint {
		maxAmount = 127
	}
	if payload.GetAmountRed() > maxAmount || payload.GetAmountGreen() > maxAmount || payload.GetAmountBlue() > maxAmount {
		return &pb.SubmitTransactionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("amount exceeds maximum %d per channel", maxAmount),
		}, nil
	}

	txHash := uuid.New().String()
	var newBalance *pb.BalanceResponse

	// 7. Execute inside a DB transaction (SELECT FOR UPDATE on both rows)
	dbErr := s.db.Transaction(func(tx *gorm.DB) error {
		senderModel, err := s.playerRepo.FindOrCreate(tx, payload.GetSenderId())
		if err != nil {
			return fmt.Errorf("failed to find/create sender: %w", err)
		}
		receiverModel, err := s.playerRepo.FindOrCreate(tx, payload.GetReceiverId())
		if err != nil {
			return fmt.Errorf("failed to find/create receiver: %w", err)
		}

		// Nonce check — replay protection
		if senderModel.Nonce != payload.GetNonce() {
			return fmt.Errorf("nonce mismatch: expected %d, got %d", senderModel.Nonce, payload.GetNonce())
		}

		if isMint {
			// MINT: reward receiver via PlayerCompleteMission (no balance deducted from sender)
			receiverState := &types.PlayerState{
				R:     uint8(receiverModel.Red),
				G:     uint8(receiverModel.Green),
				B:     uint8(receiverModel.Blue),
				Nonce: receiverModel.Nonce,
			}
			mission := &types.Mission{
				Reward: types.RGB{
					R: int8(payload.GetAmountRed()),
					G: int8(payload.GetAmountGreen()),
					B: int8(payload.GetAmountBlue()),
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

			// Increment authority nonce for replay protection
			senderModel.Nonce++

			if err := s.playerRepo.UpdateBalance(tx, receiverModel); err != nil {
				return fmt.Errorf("failed to update receiver balance: %w", err)
			}
			if err := s.playerRepo.UpdateBalance(tx, senderModel); err != nil {
				return fmt.Errorf("failed to update sender nonce: %w", err)
			}

			// Return updated receiver balance for MINT
			newBalance = &pb.BalanceResponse{
				PlayerId:  receiverModel.ID,
				Red:       receiverModel.Red,
				Green:     receiverModel.Green,
				Blue:      receiverModel.Blue,
				NextNonce: receiverModel.Nonce,
			}
		} else {
			// TRANSFER: validate sender has sufficient balance
			if senderModel.Red < payload.GetAmountRed() ||
				senderModel.Green < payload.GetAmountGreen() ||
				senderModel.Blue < payload.GetAmountBlue() {
				return fmt.Errorf("insufficient balance: have R=%d G=%d B=%d, need R=%d G=%d B=%d",
					senderModel.Red, senderModel.Green, senderModel.Blue,
					payload.GetAmountRed(), payload.GetAmountGreen(), payload.GetAmountBlue())
			}

			// Build PlayerState for engine: from.R/G/B = transfer amounts, to.R/G/B = receiver balance
			senderState := &types.PlayerState{
				R:     uint8(payload.GetAmountRed()),
				G:     uint8(payload.GetAmountGreen()),
				B:     uint8(payload.GetAmountBlue()),
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

			// Subtract transfer amounts from sender; apply receiver's new balance
			senderModel.Red -= payload.GetAmountRed()
			senderModel.Green -= payload.GetAmountGreen()
			senderModel.Blue -= payload.GetAmountBlue()
			senderModel.Nonce = newSenderState.Nonce

			receiverModel.Red = uint32(newReceiverState.R)
			receiverModel.Green = uint32(newReceiverState.G)
			receiverModel.Blue = uint32(newReceiverState.B)
			receiverModel.Nonce = newReceiverState.Nonce

			if err := s.playerRepo.UpdateBalance(tx, senderModel); err != nil {
				return fmt.Errorf("failed to update sender balance: %w", err)
			}
			if err := s.playerRepo.UpdateBalance(tx, receiverModel); err != nil {
				return fmt.Errorf("failed to update receiver balance: %w", err)
			}

			// Return updated sender balance for TRANSFER
			newBalance = &pb.BalanceResponse{
				PlayerId:  senderModel.ID,
				Red:       senderModel.Red,
				Green:     senderModel.Green,
				Blue:      senderModel.Blue,
				NextNonce: senderModel.Nonce,
			}
		}

		// Record transaction
		return s.txRepo.Create(tx, &types.TransactionRecord{
			Hash:       txHash,
			Type:       uint8(payload.GetType()),
			SenderID:   payload.GetSenderId(),
			ReceiverID: payload.GetReceiverId(),
			Red:        payload.GetAmountRed(),
			Green:      payload.GetAmountGreen(),
			Blue:       payload.GetAmountBlue(),
			Nonce:      payload.GetNonce(),
			Timestamp:  time.Now().Unix(),
		})
	})

	if dbErr != nil {
		logger.Errorf("SubmitTransaction failed: %v", dbErr)
		return &pb.SubmitTransactionResponse{
			Success:      false,
			ErrorMessage: dbErr.Error(),
		}, nil
	}

	logger.Infof("Transaction %s committed successfully", txHash)
	return &pb.SubmitTransactionResponse{
		Success:    true,
		TxHash:     txHash,
		NewBalance: newBalance,
	}, nil
}
