package repositories

import (
	"context"
	"rgb-game/internal/domain/types"

	"gorm.io/gorm"
)

// blockModel is the GORM persistence model for a sealed block.
type blockModel struct {
	Height     uint64 `gorm:"primaryKey"`
	Hash       string `gorm:"not null;uniqueIndex;type:varchar(64)"`
	PrevHash   string `gorm:"not null;type:varchar(64)"`
	MerkleRoot string `gorm:"not null;type:varchar(64)"`
	Timestamp  int64  `gorm:"not null"`
}

func (blockModel) TableName() string { return "blocks" }

func toBlockRecord(m *blockModel) *types.Block {
	return &types.Block{
		Height:     m.Height,
		Hash:       m.Hash,
		PrevHash:   m.PrevHash,
		MerkleRoot: m.MerkleRoot,
		Timestamp:  m.Timestamp,
	}
}

func fromBlockRecord(b *types.Block) *blockModel {
	return &blockModel{
		Height:     b.Height,
		Hash:       b.Hash,
		PrevHash:   b.PrevHash,
		MerkleRoot: b.MerkleRoot,
		Timestamp:  b.Timestamp,
	}
}

// BlockRepository implements out.BlockRepository using GORM + Postgres.
type BlockRepository struct {
	db *gorm.DB
}

// NewBlockRepository creates a new BlockRepository.
func NewBlockRepository(db *gorm.DB) *BlockRepository {
	return &BlockRepository{db: db}
}

// LatestBlock returns the block with the greatest height, or nil if none exist.
func (r *BlockRepository) LatestBlock(ctx context.Context) (*types.Block, error) {
	db := dbFromContext(ctx, r.db)
	var m blockModel
	result := db.Order("height DESC").First(&m)
	if result.Error == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return toBlockRecord(&m), nil
}

// CreateBlock persists a new block record.
func (r *BlockRepository) CreateBlock(ctx context.Context, block *types.Block) error {
	db := dbFromContext(ctx, r.db)
	return db.Create(fromBlockRecord(block)).Error
}

// SealTransactions sets block_height on all transactions identified by txHashes.
func (r *BlockRepository) SealTransactions(ctx context.Context, blockHeight uint64, txHashes []string) error {
	db := dbFromContext(ctx, r.db)
	return db.Model(&transactionModel{}).
		Where("hash IN ?", txHashes).
		Update("block_height", blockHeight).Error
}

// PendingTransactions returns all transactions that have not yet been sealed
// into a block (block_height IS NULL).
func (r *BlockRepository) PendingTransactions(ctx context.Context) ([]*types.TransactionRecord, error) {
	db := dbFromContext(ctx, r.db)
	var models []transactionModel
	if err := db.Where("block_height IS NULL").Find(&models).Error; err != nil {
		return nil, err
	}
	records := make([]*types.TransactionRecord, len(models))
	for i, m := range models {
		records[i] = &types.TransactionRecord{
			Hash:       m.Hash,
			Type:       m.Type,
			SenderID:   m.SenderID,
			ReceiverID: m.ReceiverID,
			Red:        m.Red,
			Green:      m.Green,
			Blue:       m.Blue,
			Nonce:      m.Nonce,
			Timestamp:  m.Timestamp,
		}
	}
	return records, nil
}
