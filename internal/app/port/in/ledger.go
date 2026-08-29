package in

import (
	"context"
	"rgb-game/internal/domain/types"
)

// TxType distinguishes TRANSFER from MINT transactions at the application layer.
type TxType uint8

const (
	TxTypeTransfer TxType = 0
	TxTypeMint     TxType = 1
)

// SubmitTransactionRequest carries a pre-verified, typed transaction.
// Signature verification and authority checks are the responsibility of the
// driving adapter (gRPC handler) before calling the use case.
type SubmitTransactionRequest struct {
	TxType      TxType
	SenderID    string
	ReceiverID  string
	AmountRed   uint32
	AmountGreen uint32
	AmountBlue  uint32
	Nonce       uint64
}

// SubmitTransactionResult is returned after a successful (or failed) submission.
type SubmitTransactionResult struct {
	Success      bool
	ErrorMessage string
	TxHash       string
	// NewBalance holds the updated balance of the primary party:
	// for TRANSFER it is the sender's balance; for MINT it is the receiver's.
	NewBalance *types.PlayerRecord
}

// LedgerUseCase defines the application operations exposed by the Ledger service.
type LedgerUseCase interface {
	// GetBalance returns the player record (R/G/B + nonce), or nil if not found.
	GetBalance(ctx context.Context, playerID string) (*types.PlayerRecord, error)

	// SubmitTransaction processes a pre-verified transaction and persists the result.
	SubmitTransaction(ctx context.Context, req SubmitTransactionRequest) (SubmitTransactionResult, error)
}
