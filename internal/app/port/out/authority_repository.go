package out

import (
	"context"
	"rgb-game/internal/domain/types"
)

// AuthorityRepository manages the set of registered minting authorities.
type AuthorityRepository interface {
	// Register persists a new authority record.
	Register(ctx context.Context, record *types.AuthorityRecord) error

	// Exists returns true if a registered authority with the given raw public key exists.
	Exists(ctx context.Context, pubKey []byte) (bool, error)
}
