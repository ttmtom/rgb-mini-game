package merkle

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// BuildMerkleRoot computes the Merkle root of a set of transaction hashes.
//
// Rules:
//   - Inputs are sorted for determinism.
//   - Each leaf is sha256(txHash bytes).
//   - Odd number of leaves: the last leaf is duplicated.
//   - Empty slice: root is sha256("").
func BuildMerkleRoot(txHashes []string) string {
	if len(txHashes) == 0 {
		h := sha256.Sum256([]byte{})
		return hex.EncodeToString(h[:])
	}

	sorted := make([]string, len(txHashes))
	copy(sorted, txHashes)
	sort.Strings(sorted)

	// Build leaf nodes.
	level := make([][]byte, len(sorted))
	for i, h := range sorted {
		sum := sha256.Sum256([]byte(h))
		level[i] = sum[:]
	}

	// Reduce until one root remains.
	for len(level) > 1 {
		if len(level)%2 != 0 {
			level = append(level, level[len(level)-1]) // duplicate last
		}
		next := make([][]byte, len(level)/2)
		for i := 0; i < len(level); i += 2 {
			combined := append(level[i], level[i+1]...)
			sum := sha256.Sum256(combined)
			next[i/2] = sum[:]
		}
		level = next
	}

	return hex.EncodeToString(level[0])
}
