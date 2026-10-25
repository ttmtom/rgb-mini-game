package out

import (
	"context"
	"rgb-game/internal/domain/types"
)

// TransferResult holds the result of a TRANSFER submission.
type TransferResult struct {
	Success      bool
	ErrorMessage string
	TxHash       string
	NewBalance   *types.PlayerRecord
}

// LedgerClient abstracts remote calls to the Ledger service made by the Game Server.
// The implementation (adapter/driven/ledger) handles all proto marshaling and signing.
type LedgerClient interface {
	// GetAuthorityNonce fetches the current next-nonce for the given authority player ID.
	GetAuthorityNonce(ctx context.Context, authorityID string) (uint64, error)

	// SubmitMint signs and submits a MINT transaction to the Ledger.
	// auth is used to sign the transaction payload.
	// Returns the assigned tx hash and the receiver's updated balance.
	SubmitMint(
		ctx context.Context,
		auth FullAuthority,
		senderID, receiverID string,
		amtRed, amtGreen, amtBlue uint32,
		nonce uint64,
	) (txHash string, newBalance *types.PlayerRecord, err error)

	// SubmitSignedTransfer forwards a pre-signed TRANSFER payload to the Ledger.
	// The caller is responsible for verifying the signature before calling this.
	SubmitSignedTransfer(ctx context.Context, rawPayload, signature, senderPubKey []byte) (TransferResult, error)
}
