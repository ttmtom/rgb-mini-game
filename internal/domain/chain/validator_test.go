package chain_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"rgb-game/internal/domain/chain"
	"rgb-game/internal/domain/merkle"
	"rgb-game/internal/domain/types"
)

// buildHash mirrors service.computeBlockHash so tests stay self-contained.
func buildHash(b *types.Block) string {
	h := sha256.New()
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, b.Height)
	h.Write(heightBytes)
	h.Write([]byte(b.PrevHash))
	h.Write([]byte(b.MerkleRoot))
	tsBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBytes, uint64(b.Timestamp))
	h.Write(tsBytes)
	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, b.Nonce)
	h.Write(nonceBytes)
	return hex.EncodeToString(h.Sum(nil))
}

func makeBlock(height uint64, prevHash string, txHashes []string, difficulty uint8) *types.Block {
	b := &types.Block{
		Height:     height,
		PrevHash:   prevHash,
		MerkleRoot: merkle.BuildMerkleRoot(txHashes),
		Timestamp:  1000000 + int64(height)*10,
		Difficulty: difficulty,
	}
	// Mine until PoW satisfied.
	prefix := make([]byte, difficulty)
	for i := range prefix {
		prefix[i] = '0'
	}
	target := string(prefix)
	for {
		b.Hash = buildHash(b)
		if len(b.Hash) >= int(difficulty) && b.Hash[:difficulty] == target {
			break
		}
		b.Nonce++
	}
	return b
}

func TestValidateChain_EmptyChain(t *testing.T) {
	if err := chain.ValidateChain(nil, nil); err != nil {
		t.Errorf("empty chain should be valid: %v", err)
	}
}

func TestValidateChain_GenesisOnly(t *testing.T) {
	genesis := makeBlock(0, types.GenesisBlockPrevHash, nil, 0)
	txsByBlock := map[uint64][]string{0: nil}

	if err := chain.ValidateChain([]*types.Block{genesis}, txsByBlock); err != nil {
		t.Errorf("valid genesis should pass: %v", err)
	}
}

func TestValidateChain_MultipleBlocks(t *testing.T) {
	genesis := makeBlock(0, types.GenesisBlockPrevHash, nil, 0)
	b1 := makeBlock(1, genesis.Hash, []string{"tx1", "tx2"}, 0)
	b2 := makeBlock(2, b1.Hash, []string{"tx3"}, 0)

	blocks := []*types.Block{genesis, b1, b2}
	txsByBlock := map[uint64][]string{
		0: nil,
		1: {"tx1", "tx2"},
		2: {"tx3"},
	}

	if err := chain.ValidateChain(blocks, txsByBlock); err != nil {
		t.Errorf("valid 3-block chain should pass: %v", err)
	}
}

func TestValidateChain_TamperedHash(t *testing.T) {
	genesis := makeBlock(0, types.GenesisBlockPrevHash, nil, 0)
	genesis.Hash = "0000000000000000000000000000000000000000000000000000000000000bad"

	txsByBlock := map[uint64][]string{0: nil}
	if err := chain.ValidateChain([]*types.Block{genesis}, txsByBlock); err == nil {
		t.Error("tampered hash should be detected")
	}
}

func TestValidateChain_TamperedPrevHash(t *testing.T) {
	genesis := makeBlock(0, types.GenesisBlockPrevHash, nil, 0)
	b1 := makeBlock(1, genesis.Hash, nil, 0)
	b1.PrevHash = "badbad"
	// Recompute hash with the wrong PrevHash so the stored hash matches.
	b1.Hash = buildHash(b1)

	blocks := []*types.Block{genesis, b1}
	txsByBlock := map[uint64][]string{0: nil, 1: nil}
	if err := chain.ValidateChain(blocks, txsByBlock); err == nil {
		t.Error("tampered PrevHash should be detected")
	}
}

func TestValidateChain_TamperedMerkleRoot(t *testing.T) {
	genesis := makeBlock(0, types.GenesisBlockPrevHash, nil, 0)
	b1 := makeBlock(1, genesis.Hash, []string{"tx1"}, 0)
	// Claim different tx hashes than what was used to build the block.
	txsByBlock := map[uint64][]string{
		0: nil,
		1: {"tx_tampered"},
	}

	blocks := []*types.Block{genesis, b1}
	if err := chain.ValidateChain(blocks, txsByBlock); err == nil {
		t.Error("tampered Merkle root (via different tx hashes) should be detected")
	}
}

func TestValidateChain_PoWDifficulty(t *testing.T) {
	genesis := makeBlock(0, types.GenesisBlockPrevHash, nil, 2)
	txsByBlock := map[uint64][]string{0: nil}

	// Valid mined block should pass.
	if err := chain.ValidateChain([]*types.Block{genesis}, txsByBlock); err != nil {
		t.Errorf("valid PoW block should pass: %v", err)
	}

	// Manually break PoW by mutating the nonce (and recomputing hash to match hash check).
	genesis.Nonce++
	genesis.Hash = buildHash(genesis)
	if err := chain.ValidateChain([]*types.Block{genesis}, txsByBlock); err == nil {
		t.Error("block that fails PoW check should be rejected")
	}
}

func TestValidateChain_WrongGenesisParent(t *testing.T) {
	b := makeBlock(0, "not_the_genesis_prev_hash", nil, 0)
	txsByBlock := map[uint64][]string{0: nil}
	if err := chain.ValidateChain([]*types.Block{b}, txsByBlock); err == nil {
		t.Error("genesis with wrong PrevHash should be rejected")
	}
}
