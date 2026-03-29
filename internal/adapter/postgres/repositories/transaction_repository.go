package repositories

import (
	"rgb-game/internal/core/types"

	"gorm.io/gorm"
)

// TransactionModel is the GORM persistence model for a ledger transaction.
// It is intentionally kept internal to the adapter; callers work with types.TransactionRecord.
type TransactionModel struct {
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

func (TransactionModel) TableName() string {
	return "transactions"
}

func fromTransactionRecord(r *types.TransactionRecord) *TransactionModel {
	return &TransactionModel{
		Hash:       r.Hash,
		Type:       r.Type,
		SenderID:   r.SenderID,
		ReceiverID: r.ReceiverID,
		Red:        r.Red,
		Green:      r.Green,
		Blue:       r.Blue,
		Nonce:      r.Nonce,
		Timestamp:  r.Timestamp,
	}
}

// TransactionRepository provides persistence operations for transaction records.
type TransactionRepository struct {
	db *gorm.DB
}

// NewTransactionRepository creates a new TransactionRepository.
func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// Create persists a new transaction record.
func (r *TransactionRepository) Create(tx *gorm.DB, record *types.TransactionRecord) error {
	return tx.Create(fromTransactionRecord(record)).Error
}
