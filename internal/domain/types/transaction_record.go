package types

// TransactionRecord is the core domain representation of a ledger transaction.
// It intentionally carries no persistence tags; the adapter layer is responsible
// for mapping this to its own storage model.
type TransactionRecord struct {
	Hash       string
	Type       uint8 // 0=TRANSFER, 1=MINT
	SenderID   string
	ReceiverID string
	Red        uint32
	Green      uint32
	Blue       uint32
	Nonce      uint64
	Timestamp  int64
}
