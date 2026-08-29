package repositories

import (
	"context"
	"rgb-game/internal/domain/types"

	"gorm.io/gorm"
)

// playerModel is the GORM persistence model for a player's on-ledger state.
// Intentionally private; callers work with domain types.PlayerRecord.
type playerModel struct {
	ID    string `gorm:"primaryKey;type:varchar(64)"` // hex(sha256(pubkey))
	Red   uint32 `gorm:"not null;default:0"`
	Green uint32 `gorm:"not null;default:0"`
	Blue  uint32 `gorm:"not null;default:0"`
	Nonce uint64 `gorm:"not null;default:0"`
}

func (playerModel) TableName() string { return "players" }

func toPlayerRecord(m *playerModel) *types.PlayerRecord {
	return &types.PlayerRecord{ID: m.ID, Red: m.Red, Green: m.Green, Blue: m.Blue, Nonce: m.Nonce}
}

func fromPlayerRecord(r *types.PlayerRecord) *playerModel {
	return &playerModel{ID: r.ID, Red: r.Red, Green: r.Green, Blue: r.Blue, Nonce: r.Nonce}
}

// PlayerRepository implements out.PlayerRepository using GORM + Postgres.
type PlayerRepository struct {
	db *gorm.DB
}

// NewPlayerRepository creates a new PlayerRepository.
func NewPlayerRepository(db *gorm.DB) *PlayerRepository {
	return &PlayerRepository{db: db}
}

// Find returns the player for the given ID, or nil if not found.
func (r *PlayerRepository) Find(ctx context.Context, playerID string) (*types.PlayerRecord, error) {
	db := dbFromContext(ctx, r.db)
	var m playerModel
	result := db.Where("id = ?", playerID).First(&m)
	if result.Error == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return toPlayerRecord(&m), nil
}

// FindOrCreate returns the player for the given ID, creating a zero-balance record if
// none exists. Must be called inside a GormTransactor.InTransaction block so that the
// SELECT FOR UPDATE clause takes effect.
func (r *PlayerRepository) FindOrCreate(ctx context.Context, playerID string) (*types.PlayerRecord, error) {
	db := dbFromContext(ctx, r.db)
	var m playerModel
	result := db.Set("gorm:query_option", "FOR UPDATE").
		Where("id = ?", playerID).
		First(&m)

	if result.Error == gorm.ErrRecordNotFound {
		m = playerModel{ID: playerID}
		if err := db.Create(&m).Error; err != nil {
			return nil, err
		}
		return toPlayerRecord(&m), nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return toPlayerRecord(&m), nil
}

// UpdateBalance persists the updated player record.
func (r *PlayerRepository) UpdateBalance(ctx context.Context, player *types.PlayerRecord) error {
	db := dbFromContext(ctx, r.db)
	return db.Save(fromPlayerRecord(player)).Error
}
