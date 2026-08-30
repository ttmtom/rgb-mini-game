package chain

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"rgb-game/internal/domain/merkle"
	"rgb-game/internal/domain/types"
)

// ValidateChain verifies the integrity of the full block chain.
//
// For every block it checks:
//  1. block.Hash == sha256(height + prevHash + merkleRoot + timestamp + nonce)
//  2. block.PrevHash == hash of the previous block (genesis uses 64 zero chars)
//  3. block.MerkleRoot == merkle.BuildMerkleRoot(txHashes for that height)
//  4. block.Hash starts with block.Difficulty leading zero hex nibbles (PoW)
//
// blocks must be sorted by ascending Height starting at 0.
// txsByBlock maps block height → slice of transaction hashes sealed in that block.
func ValidateChain(blocks []*types.Block, txsByBlock map[uint64][]string) error {
	for i, b := range blocks {
		// 1. Recompute and compare the block hash.
		expected := computeBlockHash(b)
		if b.Hash != expected {
			return fmt.Errorf("block %d: hash mismatch (stored=%s, computed=%s)", b.Height, b.Hash, expected)
		}

		// 2. Check PrevHash linkage.
		if i == 0 {
			if b.PrevHash != types.GenesisBlockPrevHash {
				return fmt.Errorf("genesis block has unexpected PrevHash: %s", b.PrevHash)
			}
		} else {
			if b.PrevHash != blocks[i-1].Hash {
				return fmt.Errorf("block %d: PrevHash mismatch (stored=%s, prev block hash=%s)", b.Height, b.PrevHash, blocks[i-1].Hash)
			}
		}

		// 3. Verify Merkle root over the block's transaction hashes.
		txHashes := txsByBlock[b.Height]
		expectedMerkle := merkle.BuildMerkleRoot(txHashes)
		if b.MerkleRoot != expectedMerkle {
			return fmt.Errorf("block %d: merkle root mismatch (stored=%s, computed=%s)", b.Height, b.MerkleRoot, expectedMerkle)
		}

		// 4. Verify PoW difficulty target.
		if b.Difficulty > 0 {
			if uint8(len(b.Hash)) < b.Difficulty {
				return fmt.Errorf("block %d: hash too short to satisfy difficulty %d", b.Height, b.Difficulty)
			}
			for j := uint8(0); j < b.Difficulty; j++ {
				if b.Hash[j] != '0' {
					return fmt.Errorf("block %d: hash %s does not satisfy difficulty %d", b.Height, b.Hash, b.Difficulty)
				}
			}
		}
	}
	return nil
}

// computeBlockHash mirrors the hash function used in block_sealer.go.
// It is duplicated here so the domain package has no dependency on the service layer.
// Must be kept in sync with service.computeBlockHash.
func computeBlockHash(b *types.Block) string {
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
