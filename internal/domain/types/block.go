package types

// Block represents a sealed block in the chain.
// Each block commits to a batch of transactions via a Merkle root and links
// to the previous block via PrevHash, forming an immutable chain.
//
// Nonce and Difficulty support Proof-of-Work: the block hash must start with
// Difficulty leading zero hex nibbles. The sealer increments Nonce until the
// constraint is satisfied.
type Block struct {
	Height     uint64
	Hash       string // sha256(height + prev_hash + merkle_root + timestamp + nonce)
	PrevHash   string // hash of the previous block; genesis uses 64 zero hex chars
	MerkleRoot string // Merkle root of all tx hashes sealed in this block
	Timestamp  int64
	Nonce      uint64 // PoW nonce — incremented until hash satisfies difficulty
	Difficulty uint8  // number of leading zero hex nibbles required in the hash
}

// GenesisBlockPrevHash is the canonical PrevHash of the genesis block.
const GenesisBlockPrevHash = "0000000000000000000000000000000000000000000000000000000000000000"
