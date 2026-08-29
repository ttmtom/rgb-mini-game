package repositories

import (
	"context"
	"rgb-game/internal/domain/types"

	"gorm.io/gorm"
)

// transactionModel is the GORM persistence model for a ledger transaction.
// Intentionally private; callers work with domain types.TransactionRecord.
type transactionModel struct {
	Hash       string `gorm:"primaryKey;type:varchar(64)"`
	Type       uint8  `gorm:"not null"` // 0=TRANSFER, 1=MINT
	SenderID   string `gorm:"not null;type:varchar(64);index"`
	ReceiverID string `gorm:"not null;type:varchar(64);index"`
	Red        uint32 `gorm:"not null;default:0"`
	Green      uint32 `gorm:"not null;default:0"`
	Blue       uint32 `gorm:"not null;default:0"`
	Nonce      uint64 `gorm:"not null"`
	Timestamp  int64  `gorm:"not null"`
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
