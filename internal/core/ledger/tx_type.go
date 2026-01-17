package ledger

type TxType uint8

const (
	TxTypeTransfer TxType = 0
	TxTypeMint     TxType = 1
)
