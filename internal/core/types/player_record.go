package types

// PlayerRecord is the core domain representation of a player's on-ledger state.
// It intentionally carries no persistence tags; the adapter layer is responsible
// for mapping this to its own storage model.
type PlayerRecord struct {
	ID    string
	Red   uint32
	Green uint32
	Blue  uint32
	Nonce uint64
}
