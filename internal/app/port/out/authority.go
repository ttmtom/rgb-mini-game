package out

import "crypto/ed25519"

// PublicAuthority provides read-only access to the minting authority identity.
// Used by layers that only need to verify (e.g. Ledger gRPC adapter).
type PublicAuthority interface {
	PubKey() ed25519.PublicKey
	PlayerID() string
}

// FullAuthority extends PublicAuthority with signing capability.
// Used by the Game Server application service to issue MINT transactions.
type FullAuthority interface {
	PublicAuthority
	Sign(data []byte) []byte
}
