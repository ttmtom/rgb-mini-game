package types

// TransactionRecord is the core domain representation of a ledger transaction.
// It intentionally carries no persistence tags; the adapter layer is responsible
// for mapping this to its own storage model.
type TransactionRecord struct {
	Hash       string
	SenderID   string
	ReceiverID string
	Nonce      uint64
	Timestamp  int64
	Red        uint32
	Green      uint32
	Blue       uint32
	Type       uint8 // 0=TRANSFER, 1=MINT
}
