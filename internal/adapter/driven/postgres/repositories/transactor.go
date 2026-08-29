package repositories

import (
	"context"

	"gorm.io/gorm"
)

// txKey is the context key used to propagate a *gorm.DB transaction.
type txKey struct{}

// GormTransactor implements out.Transactor using GORM database transactions.
type GormTransactor struct {
	db *gorm.DB
}

// NewTransactor creates a GormTransactor backed by the given *gorm.DB.
func NewTransactor(db *gorm.DB) *GormTransactor {
	return &GormTransactor{db: db}
}

// InTransaction starts a GORM transaction, injects it into the context, and
// calls fn. The transaction is committed on nil return, rolled back on error.
func (t *GormTransactor) InTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return t.db.Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKey{}, tx)
		return fn(txCtx)
	})
}

// dbFromContext retrieves the active *gorm.DB transaction from ctx, falling
// back to the provided default db if no transaction is present.
func dbFromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok && tx != nil {
		return tx
	}
	return fallback
}
