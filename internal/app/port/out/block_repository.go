package out

import (
	"context"
	"rgb-game/internal/domain/types"
)

// BlockRepository manages block persistence and transaction sealing.
type BlockRepository interface {
	// LatestBlock returns the most recently sealed block, or nil if none exist.
	LatestBlock(ctx context.Context) (*types.Block, error)

	// CreateBlock persists a new block record.
	CreateBlock(ctx context.Context, block *types.Block) error

	// SealTransactions sets block_height on all transactions whose hashes are
	// in txHashes, binding them to the given block.
	SealTransactions(ctx context.Context, blockHeight uint64, txHashes []string) error

	// PendingTransactions returns all transaction records that have not yet
	// been assigned to a block (block_height IS NULL).
	PendingTransactions(ctx context.Context) ([]*types.TransactionRecord, error)

	// AllBlocks returns every block ordered by ascending height.
	AllBlocks(ctx context.Context) ([]*types.Block, error)

	// TransactionHashesByBlock returns all transaction hashes sealed in the
	// given block (identified by height).
	TransactionHashesByBlock(ctx context.Context, blockHeight uint64) ([]string, error)

	// CountBlocks returns the total number of sealed blocks.
	CountBlocks(ctx context.Context) (uint64, error)

	// CountPending returns the number of transactions not yet sealed into a block.
	CountPending(ctx context.Context) (uint64, error)

	// GetByHeight returns the block at the given height, or nil if not found.
	GetByHeight(ctx context.Context, height uint64) (*types.Block, error)

	// ListBlocks returns a paginated slice of blocks ordered by ascending height.
	ListBlocks(ctx context.Context, page, pageSize uint32) ([]*types.Block, error)
}
