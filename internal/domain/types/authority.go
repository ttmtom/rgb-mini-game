package types

// AuthorityRecord represents a registered minting authority.
// Any node that has registered its public key is permitted to issue MINT transactions.
type AuthorityRecord struct {
	ID           string // hex(sha256(pubKey)) — same derivation as PlayerRecord.ID
	PubKeyHex    string // hex-encoded ed25519 public key
	RegisteredAt int64  // unix timestamp
}
