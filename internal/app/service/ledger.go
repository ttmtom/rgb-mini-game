package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"rgb-game/internal/app/port/in"
	"rgb-game/internal/app/port/out"
	"rgb-game/internal/domain/chain"
	"rgb-game/internal/domain/types"
	"rgb-game/pkg/logger"
	"time"

	"github.com/google/uuid"
)

// LedgerService implements in.LedgerUseCase.
// It contains no transport-layer imports — all gRPC concerns live in the driving adapter.
type LedgerService struct {
	playerRepo    out.PlayerRepository
	txRepo        out.TransactionRepository
	blockRepo     out.BlockRepository
	authorityRepo out.AuthorityRepository
	transactor    out.Transactor
	gameEngine    out.GameEngine
}

// NewLedgerService wires up the LedgerService with its driven-port dependencies.
func NewLedgerService(
	playerRepo out.PlayerRepository,
	txRepo out.TransactionRepository,
	blockRepo out.BlockRepository,
	authorityRepo out.AuthorityRepository,
	transactor out.Transactor,
	gameEngine out.GameEngine,
) *LedgerService {
	return &LedgerService{
		playerRepo:    playerRepo,
		txRepo:        txRepo,
		blockRepo:     blockRepo,
		authorityRepo: authorityRepo,
		transactor:    transactor,
		gameEngine:    gameEngine,
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

// ValidateChain walks every sealed block and verifies hash linkage and Merkle roots.
func (s *LedgerService) ValidateChain(ctx context.Context) error {
	blocks, err := s.blockRepo.AllBlocks(ctx)
	if err != nil {
		return fmt.Errorf("failed to load blocks: %w", err)
	}
	if len(blocks) == 0 {
		return nil // nothing to validate yet
	}

	txsByBlock := make(map[uint64][]string, len(blocks))
	for _, b := range blocks {
		hashes, err := s.blockRepo.TransactionHashesByBlock(ctx, b.Height)
		if err != nil {
			return fmt.Errorf("failed to load tx hashes for block %d: %w", b.Height, err)
		}
		txsByBlock[b.Height] = hashes
	}

	return chain.ValidateChain(blocks, txsByBlock)
}

// RegisterAuthority persists a new minting authority.
// Cryptographic verification (self-signed proof) must be performed by the driving
// adapter before calling this method; AuthorityID must be pre-derived.
func (s *LedgerService) RegisterAuthority(ctx context.Context, req in.RegisterAuthorityRequest) (in.RegisterAuthorityResult, error) {
	record := &types.AuthorityRecord{
		ID:           req.AuthorityID,
		PubKeyHex:    hex.EncodeToString(req.PubKey),
		RegisteredAt: time.Now().Unix(),
	}

	if err := s.authorityRepo.Register(ctx, record); err != nil {
		return in.RegisterAuthorityResult{Success: false, ErrorMessage: "failed to register authority"}, err
	}

	logger.Infof("Authority registered: id=%s", req.AuthorityID)
	return in.RegisterAuthorityResult{Success: true, AuthorityID: req.AuthorityID}, nil
}

// GetTransactionHistory returns paginated transactions where the player is sender or receiver.
func (s *LedgerService) GetTransactionHistory(ctx context.Context, req in.TransactionHistoryRequest) (in.TransactionHistoryResult, error) {
	logger.Infof("GetTransactionHistory for player %s (limit=%d offset=%d)", req.PlayerID, req.Limit, req.Offset)

	records, total, err := s.txRepo.FindByPlayer(ctx, req.PlayerID, req.Limit, req.Offset)
	if err != nil {
		return in.TransactionHistoryResult{}, fmt.Errorf("query transaction history: %w", err)
	}

	entries := make([]*in.TransactionHistoryEntry, len(records))
	for i, r := range records {
		entries[i] = &in.TransactionHistoryEntry{
			Hash:        r.Hash,
			Type:        r.Type,
			SenderID:    r.SenderID,
			ReceiverID:  r.ReceiverID,
			Red:         r.Red,
			Green:       r.Green,
			Blue:        r.Blue,
			Nonce:       r.Nonce,
			Timestamp:   r.Timestamp,
			BlockHeight: r.BlockHeight,
		}
	}
	return in.TransactionHistoryResult{Transactions: entries, Total: total}, nil
}

// GetChainStats returns aggregate chain and network statistics.
func (s *LedgerService) GetChainStats(ctx context.Context) (in.ChainStatsResult, error) {
	latest, err := s.blockRepo.LatestBlock(ctx)
	if err != nil {
		return in.ChainStatsResult{}, fmt.Errorf("fetch latest block: %w", err)
	}

	pendingCount, err := s.blockRepo.CountPending(ctx)
	if err != nil {
		return in.ChainStatsResult{}, fmt.Errorf("count pending txs: %w", err)
	}

	totalTxCount, err := s.txRepo.CountAll(ctx)
	if err != nil {
		return in.ChainStatsResult{}, fmt.Errorf("count total txs: %w", err)
	}

	totalPlayers, err := s.playerRepo.Count(ctx)
	if err != nil {
		return in.ChainStatsResult{}, fmt.Errorf("count players: %w", err)
	}

	result := in.ChainStatsResult{
		PendingTxCount: pendingCount,
		TotalTxCount:   totalTxCount,
		TotalPlayers:   totalPlayers,
	}
	if latest != nil {
		result.ChainHeight = latest.Height
		result.LastBlockHash = latest.Hash
		result.LastBlockTimestamp = latest.Timestamp
	}
	return result, nil
}

// GetBlock returns the block at the given height with its transaction hashes.
func (s *LedgerService) GetBlock(ctx context.Context, height uint64) (in.BlockResult, error) {
	block, err := s.blockRepo.GetByHeight(ctx, height)
	if err != nil {
		return in.BlockResult{}, fmt.Errorf("fetch block %d: %w", height, err)
	}
	if block == nil {
		return in.BlockResult{Found: false}, nil
	}

	hashes, err := s.blockRepo.TransactionHashesByBlock(ctx, height)
	if err != nil {
		return in.BlockResult{}, fmt.Errorf("fetch tx hashes for block %d: %w", height, err)
	}
	return in.BlockResult{Found: true, Block: block, TxHashes: hashes}, nil
}

// ListBlocks returns a paginated list of blocks with their transaction hashes.
func (s *LedgerService) ListBlocks(ctx context.Context, page, pageSize uint32) (in.ListBlocksResult, error) {
	total, err := s.blockRepo.CountBlocks(ctx)
	if err != nil {
		return in.ListBlocksResult{}, fmt.Errorf("count blocks: %w", err)
	}

	blocks, err := s.blockRepo.ListBlocks(ctx, page, pageSize)
	if err != nil {
		return in.ListBlocksResult{}, fmt.Errorf("list blocks: %w", err)
	}

	entries := make([]*in.BlockInfoEntry, len(blocks))
	for i, b := range blocks {
		hashes, err := s.blockRepo.TransactionHashesByBlock(ctx, b.Height)
		if err != nil {
			return in.ListBlocksResult{}, fmt.Errorf("fetch tx hashes for block %d: %w", b.Height, err)
		}
		entries[i] = &in.BlockInfoEntry{Block: b, TxHashes: hashes}
	}
	return in.ListBlocksResult{Blocks: entries, TotalBlocks: total}, nil
}

// GetLeaderboard returns the top-N players sorted by the requested channel.
func (s *LedgerService) GetLeaderboard(ctx context.Context, req in.LeaderboardRequest) ([]*in.LeaderboardEntry, error) {
	logger.Infof("GetLeaderboard top=%d sortBy=%s", req.TopN, req.SortBy)

	entries, err := s.playerRepo.GetLeaderboard(ctx, req.TopN, req.SortBy)
	if err != nil {
		return nil, fmt.Errorf("query leaderboard: %w", err)
	}

	result := make([]*in.LeaderboardEntry, len(entries))
	for i, e := range entries {
		result[i] = &in.LeaderboardEntry{
			Rank:     e.Rank,
			PlayerID: e.PlayerID,
			Red:      e.Red,
			Green:    e.Green,
			Blue:     e.Blue,
			Total:    e.Total,
		}
	}
	return result, nil
}
