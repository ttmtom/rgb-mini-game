package types

// Block represents a sealed block in the chain.
// Each block commits to a batch of transactions via a Merkle root and links
// to the previous block via PrevHash, forming an immutable chain.
type Block struct {
	Height     uint64
	Hash       string // sha256(height + prev_hash + merkle_root + timestamp)
	PrevHash   string // hash of the previous block; genesis uses 64 zero hex chars
	MerkleRoot string // Merkle root of all tx hashes sealed in this block
	Timestamp  int64
}

// GenesisBlockPrevHash is the canonical PrevHash of the genesis block.
const GenesisBlockPrevHash = "0000000000000000000000000000000000000000000000000000000000000000"
