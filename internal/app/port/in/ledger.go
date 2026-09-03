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
	SenderID    string
	ReceiverID  string
	Nonce       uint64
	AmountRed   uint32
	AmountGreen uint32
	AmountBlue  uint32
	TxType      TxType
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

// RegisterAuthorityRequest carries the data for registering a new minting authority.
// The driving adapter is responsible for verifying the self-signed proof before
// constructing this request; AuthorityID must be pre-derived by the adapter.
type RegisterAuthorityRequest struct {
	PubKey      []byte // raw ed25519 public key
	AuthorityID string // hex(sha256(PubKey)), derived and verified by the driving adapter
}

// RegisterAuthorityResult is returned after a RegisterAuthority call.
type RegisterAuthorityResult struct {
	Success      bool
	ErrorMessage string
	AuthorityID  string
}

// LedgerUseCase defines the application operations exposed by the Ledger service.
type LedgerUseCase interface {
	// GetBalance returns the player record (R/G/B + nonce), or nil if not found.
	GetBalance(ctx context.Context, playerID string) (*types.PlayerRecord, error)

	// SubmitTransaction processes a pre-verified transaction and persists the result.
	SubmitTransaction(ctx context.Context, req SubmitTransactionRequest) (SubmitTransactionResult, error)

	// ValidateChain walks the full block chain and verifies every block's hash,
	// PrevHash linkage, and Merkle root. Returns an error describing the first
	// integrity violation found, or nil if the chain is intact.
	ValidateChain(ctx context.Context) error

	// RegisterAuthority adds a new minting authority after verifying the self-signed proof.
	RegisterAuthority(ctx context.Context, req RegisterAuthorityRequest) (RegisterAuthorityResult, error)
}
