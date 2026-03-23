package repositories

import "gorm.io/gorm"

// PlayerModel represents a player's on-ledger balance and nonce.
type PlayerModel struct {
	ID    string `gorm:"primaryKey;type:varchar(64)"` // hex(sha256(pubkey))
	Red   uint32 `gorm:"not null;default:0"`
	Green uint32 `gorm:"not null;default:0"`
	Blue  uint32 `gorm:"not null;default:0"`
	Nonce uint64 `gorm:"not null;default:0"`
}

func (PlayerModel) TableName() string {
	return "players"
}

// PlayerRepository provides persistence operations for PlayerModel.
type PlayerRepository struct {
	db *gorm.DB
}

// NewPlayerRepository creates a new PlayerRepository.
func NewPlayerRepository(db *gorm.DB) *PlayerRepository {
	return &PlayerRepository{db: db}
}

// FindOrCreate returns the player for the given ID, creating a zero-balance
// record if none exists. The returned record is locked with SELECT FOR UPDATE
// when called inside a transaction.
func (r *PlayerRepository) FindOrCreate(tx *gorm.DB, playerID string) (*PlayerModel, error) {
	var player PlayerModel
	result := tx.Set("gorm:query_option", "FOR UPDATE").
		Where("id = ?", playerID).
		First(&player)

	if result.Error == gorm.ErrRecordNotFound {
		player = PlayerModel{ID: playerID}
		if err := tx.Create(&player).Error; err != nil {
			return nil, err
		}
		return &player, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &player, nil
}

// UpdateBalance persists the updated player record.
func (r *PlayerRepository) UpdateBalance(tx *gorm.DB, player *PlayerModel) error {
	return tx.Save(player).Error
}
