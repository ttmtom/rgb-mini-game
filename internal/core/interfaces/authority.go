package interfaces

import "crypto/ed25519"

// PublicAuthority provides read-only access to the minting authority identity.
// Accepted by layers that only need to verify (e.g. Ledger).
type PublicAuthority interface {
	PubKey() ed25519.PublicKey
	PlayerID() string
}

// FullAuthority extends PublicAuthority with signing capability.
// Accepted by layers that need to issue MINT transactions (e.g. Game Server).
type FullAuthority interface {
	PublicAuthority
	Sign(data []byte) []byte
}
