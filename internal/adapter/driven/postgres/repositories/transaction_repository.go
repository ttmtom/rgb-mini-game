package repositories

import (
	"context"
	"rgb-game/internal/domain/types"

	"gorm.io/gorm"
)

// transactionModel is the GORM persistence model for a ledger transaction.
// Intentionally private; callers work with domain types.TransactionRecord.
type transactionModel struct {
	BlockHeight *uint64 `gorm:"index"` // NULL until sealed into a block
	Hash        string  `gorm:"primaryKey;type:varchar(64)"`
	SenderID    string  `gorm:"not null;type:varchar(64);index"`
	ReceiverID  string  `gorm:"not null;type:varchar(64);index"`
	Nonce       uint64  `gorm:"not null"`
	Timestamp   int64   `gorm:"not null"`
	Red         uint32  `gorm:"not null;default:0"`
	Green       uint32  `gorm:"not null;default:0"`
	Blue        uint32  `gorm:"not null;default:0"`
	Type        uint8   `gorm:"not null"` // 0=TRANSFER, 1=MINT
}

func (transactionModel) TableName() string { return "transactions" }

func fromTransactionRecord(r *types.TransactionRecord) *transactionModel {
	return &transactionModel{
		Hash: r.Hash, Type: r.Type,
		SenderID: r.SenderID, ReceiverID: r.ReceiverID,
		Red: r.Red, Green: r.Green, Blue: r.Blue,
		Nonce: r.Nonce, Timestamp: r.Timestamp,
	}
}

func toTransactionRecord(m *transactionModel) *types.TransactionRecord {
	blockHeight := int64(-1)
	if m.BlockHeight != nil {
		blockHeight = int64(*m.BlockHeight)
	}
	return &types.TransactionRecord{
		Hash:        m.Hash,
		Type:        m.Type,
		SenderID:    m.SenderID,
		ReceiverID:  m.ReceiverID,
		Red:         m.Red,
		Green:       m.Green,
		Blue:        m.Blue,
		Nonce:       m.Nonce,
		Timestamp:   m.Timestamp,
		BlockHeight: blockHeight,
	}
}

// TransactionRepository implements out.TransactionRepository using GORM + Postgres.
type TransactionRepository struct {
	db *gorm.DB
}

// NewTransactionRepository creates a new TransactionRepository.
func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// Create persists a new transaction record.
func (r *TransactionRepository) Create(ctx context.Context, record *types.TransactionRecord) error {
	db := dbFromContext(ctx, r.db)
	return db.Create(fromTransactionRecord(record)).Error
}

// FindByPlayer returns transactions where playerID is sender or receiver,
// ordered by timestamp descending, with limit/offset pagination.
func (r *TransactionRepository) FindByPlayer(ctx context.Context, playerID string, limit, offset uint32) ([]*types.TransactionRecord, uint64, error) {
	db := dbFromContext(ctx, r.db)

	var count int64
	if err := db.Model(&transactionModel{}).
		Where("sender_id = ? OR receiver_id = ?", playerID, playerID).
		Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if limit == 0 || limit > 100 {
		limit = 20
	}

	var models []transactionModel
	if err := db.Where("sender_id = ? OR receiver_id = ?", playerID, playerID).
		Order("timestamp DESC").
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}

	records := make([]*types.TransactionRecord, len(models))
	for i, m := range models {
		records[i] = toTransactionRecord(&m)
	}
	return records, uint64(count), nil
}

// CountAll returns the total number of transaction records.
func (r *TransactionRepository) CountAll(ctx context.Context) (uint64, error) {
	db := dbFromContext(ctx, r.db)
	var count int64
	if err := db.Model(&transactionModel{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return uint64(count), nil
}
