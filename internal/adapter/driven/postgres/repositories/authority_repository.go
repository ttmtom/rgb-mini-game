package repositories

import (
	"context"
	"encoding/hex"
	"rgb-game/internal/domain/types"

	"gorm.io/gorm"
)

// authorityModel is the GORM persistence model for a registered minting authority.
type authorityModel struct {
	ID           string `gorm:"primaryKey;type:varchar(64)"`
	PubKeyHex    string `gorm:"not null;uniqueIndex;type:varchar(128)"`
	RegisteredAt int64  `gorm:"not null"`
}

func (authorityModel) TableName() string { return "authorities" }

// AuthorityRepository implements out.AuthorityRepository using GORM + Postgres.
type AuthorityRepository struct {
	db *gorm.DB
}

// NewAuthorityRepository creates a new AuthorityRepository.
func NewAuthorityRepository(db *gorm.DB) *AuthorityRepository {
	return &AuthorityRepository{db: db}
}

// Register persists a new authority record.
func (r *AuthorityRepository) Register(ctx context.Context, record *types.AuthorityRecord) error {
	db := dbFromContext(ctx, r.db)
	m := &authorityModel{
		ID:           record.ID,
		PubKeyHex:    record.PubKeyHex,
		RegisteredAt: record.RegisteredAt,
	}
	// Use FirstOrCreate so duplicate registrations are idempotent.
	return db.Where("pub_key_hex = ?", m.PubKeyHex).FirstOrCreate(m).Error
}

// Exists returns true if the given raw public key is in the authority registry.
func (r *AuthorityRepository) Exists(ctx context.Context, pubKey []byte) (bool, error) {
	db := dbFromContext(ctx, r.db)
	pubKeyHex := hex.EncodeToString(pubKey)
	var count int64
	if err := db.Model(&authorityModel{}).Where("pub_key_hex = ?", pubKeyHex).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
