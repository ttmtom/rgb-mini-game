package repositories

import "gorm.io/gorm"

// TransactionModel represents a persisted ledger transaction.
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

// TransactionRepository provides persistence operations for TransactionModel.
type TransactionRepository struct {
	db *gorm.DB
}

// NewTransactionRepository creates a new TransactionRepository.
func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// Create persists a new transaction record.
func (r *TransactionRepository) Create(tx *gorm.DB, model *TransactionModel) error {
	return tx.Create(model).Error
}
