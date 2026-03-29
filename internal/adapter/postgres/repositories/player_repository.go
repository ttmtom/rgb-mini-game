package repositories

import (
	"rgb-game/internal/core/types"

	"gorm.io/gorm"
)

// PlayerModel is the GORM persistence model for a player's on-ledger state.
// It is intentionally kept internal to the adapter; callers work with types.PlayerRecord.
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

func toPlayerRecord(m *PlayerModel) *types.PlayerRecord {
	return &types.PlayerRecord{
		ID:    m.ID,
		Red:   m.Red,
		Green: m.Green,
		Blue:  m.Blue,
		Nonce: m.Nonce,
	}
}

func fromPlayerRecord(r *types.PlayerRecord) *PlayerModel {
	return &PlayerModel{
		ID:    r.ID,
		Red:   r.Red,
		Green: r.Green,
		Blue:  r.Blue,
		Nonce: r.Nonce,
	}
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
func (r *PlayerRepository) FindOrCreate(tx *gorm.DB, playerID string) (*types.PlayerRecord, error) {
	var player PlayerModel
	result := tx.Set("gorm:query_option", "FOR UPDATE").
		Where("id = ?", playerID).
		First(&player)

	if result.Error == gorm.ErrRecordNotFound {
		player = PlayerModel{ID: playerID}
		if err := tx.Create(&player).Error; err != nil {
			return nil, err
		}
		return toPlayerRecord(&player), nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return toPlayerRecord(&player), nil
}

// Find returns the player for the given ID, or nil if not found.
// This is a plain read with no row lock — suitable for read-only queries.
func (r *PlayerRepository) Find(db *gorm.DB, playerID string) (*types.PlayerRecord, error) {
	var player PlayerModel
	result := db.Where("id = ?", playerID).First(&player)
	if result.Error == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return toPlayerRecord(&player), nil
}

// UpdateBalance persists the updated player record.
func (r *PlayerRepository) UpdateBalance(tx *gorm.DB, player *types.PlayerRecord) error {
	return tx.Save(fromPlayerRecord(player)).Error
}
