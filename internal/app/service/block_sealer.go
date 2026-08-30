package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"rgb-game/internal/app/port/out"
	"rgb-game/internal/domain/merkle"
	"rgb-game/internal/domain/types"
	"rgb-game/pkg/logger"
	"time"
)

// BlockSealer is a background service that periodically seals pending
// transactions into a new block, forming a cryptographically linked chain.
type BlockSealer struct {
	blockRepo  out.BlockRepository
	transactor out.Transactor
	interval   time.Duration
}

// NewBlockSealer creates a BlockSealer with the given dependencies.
func NewBlockSealer(blockRepo out.BlockRepository, transactor out.Transactor, interval time.Duration) *BlockSealer {
	return &BlockSealer{
		blockRepo:  blockRepo,
		transactor: transactor,
		interval:   interval,
	}
}

// Start runs the block-sealing loop. It blocks until ctx is cancelled.
// Call as a goroutine: go sealer.Start(ctx).
func (s *BlockSealer) Start(ctx context.Context) {
	logger.Infof("BlockSealer started — sealing every %s", s.interval)
	if err := s.ensureGenesis(ctx); err != nil {
		logger.Errorf("BlockSealer: failed to ensure genesis block: %v", err)
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("BlockSealer stopping")
			return
		case <-ticker.C:
			if err := s.seal(ctx); err != nil {
				logger.Errorf("BlockSealer: seal error: %v", err)
			}
		}
	}
}

// ensureGenesis creates the genesis block (height 0) if the chain is empty.
func (s *BlockSealer) ensureGenesis(ctx context.Context) error {
	latest, err := s.blockRepo.LatestBlock(ctx)
	if err != nil {
		return err
	}
	if latest != nil {
		return nil // chain already started
	}

	now := time.Now().Unix()
	emptyMerkle := merkle.BuildMerkleRoot(nil)
	genesis := &types.Block{
		Height:     0,
		PrevHash:   types.GenesisBlockPrevHash,
		MerkleRoot: emptyMerkle,
		Timestamp:  now,
	}
	genesis.Hash = computeBlockHash(genesis)

	if err := s.blockRepo.CreateBlock(ctx, genesis); err != nil {
		return fmt.Errorf("create genesis block: %w", err)
	}
	logger.Infof("BlockSealer: genesis block created (hash=%s)", genesis.Hash)
	return nil
}

// seal collects all pending transactions, bundles them into a new block,
// and persists everything atomically.
func (s *BlockSealer) seal(ctx context.Context) error {
	return s.transactor.InTransaction(ctx, func(txCtx context.Context) error {
		pending, err := s.blockRepo.PendingTransactions(txCtx)
		if err != nil {
			return fmt.Errorf("fetch pending transactions: %w", err)
		}

		latest, err := s.blockRepo.LatestBlock(txCtx)
		if err != nil {
			return fmt.Errorf("fetch latest block: %w", err)
		}
		if latest == nil {
			return fmt.Errorf("no genesis block found — chain not initialised")
		}

		// Build Merkle root from pending tx hashes (empty block is valid).
		txHashes := make([]string, len(pending))
		for i, tx := range pending {
			txHashes[i] = tx.Hash
		}

		newBlock := &types.Block{
			Height:     latest.Height + 1,
			PrevHash:   latest.Hash,
			MerkleRoot: merkle.BuildMerkleRoot(txHashes),
			Timestamp:  time.Now().Unix(),
		}
		newBlock.Hash = computeBlockHash(newBlock)

		if err := s.blockRepo.CreateBlock(txCtx, newBlock); err != nil {
			return fmt.Errorf("create block %d: %w", newBlock.Height, err)
		}

		if len(txHashes) > 0 {
			if err := s.blockRepo.SealTransactions(txCtx, newBlock.Height, txHashes); err != nil {
				return fmt.Errorf("seal transactions for block %d: %w", newBlock.Height, err)
			}
		}

		logger.Infof("BlockSealer: sealed block %d (txs=%d, hash=%s)", newBlock.Height, len(txHashes), newBlock.Hash)
		return nil
	})
}

// computeBlockHash returns sha256(height_bytes + prev_hash + merkle_root + timestamp_bytes)
// as a lower-case hex string.
func computeBlockHash(b *types.Block) string {
	h := sha256.New()

	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, b.Height)
	h.Write(heightBytes)
	h.Write([]byte(b.PrevHash))
	h.Write([]byte(b.MerkleRoot))

	tsBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBytes, uint64(b.Timestamp))
	h.Write(tsBytes)

	return hex.EncodeToString(h.Sum(nil))
}
