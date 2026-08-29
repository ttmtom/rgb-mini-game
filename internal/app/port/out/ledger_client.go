package out

import (
	"context"
	"rgb-game/internal/domain/types"
)

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
}
