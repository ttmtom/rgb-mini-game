package merkle_test

import (
	"testing"

	"rgb-game/internal/domain/merkle"
)

func TestBuildMerkleRoot_Empty(t *testing.T) {
	root := merkle.BuildMerkleRoot(nil)
	if root == "" {
		t.Error("empty root should not be an empty string")
	}
	// Calling again must return the same value (deterministic).
	root2 := merkle.BuildMerkleRoot([]string{})
	if root != root2 {
		t.Errorf("empty and nil should produce same root: %q vs %q", root, root2)
	}
}

func TestBuildMerkleRoot_SingleHash(t *testing.T) {
	hashes := []string{"abc123"}
	root := merkle.BuildMerkleRoot(hashes)
	if root == "" {
		t.Error("single-element root should not be empty")
	}
}

func TestBuildMerkleRoot_Deterministic(t *testing.T) {
	hashes := []string{"tx1", "tx2", "tx3"}
	r1 := merkle.BuildMerkleRoot(hashes)
	r2 := merkle.BuildMerkleRoot(hashes)
	if r1 != r2 {
		t.Errorf("same input must produce same root: %q vs %q", r1, r2)
	}
}

func TestBuildMerkleRoot_OrderMatters(t *testing.T) {
	h1 := merkle.BuildMerkleRoot([]string{"a", "b"})
	h2 := merkle.BuildMerkleRoot([]string{"b", "a"})
	// Sorted internally → same result regardless of input order.
	if h1 != h2 {
		t.Errorf("roots should be equal (input is sorted internally): %q vs %q", h1, h2)
	}
}

func TestBuildMerkleRoot_DifferentInputs(t *testing.T) {
	r1 := merkle.BuildMerkleRoot([]string{"tx1", "tx2"})
	r2 := merkle.BuildMerkleRoot([]string{"tx3", "tx4"})
	if r1 == r2 {
		t.Error("different inputs should produce different roots")
	}
}

func TestBuildMerkleRoot_PowerOfTwoAndOdd(t *testing.T) {
	// 4 hashes (power of 2)
	r4 := merkle.BuildMerkleRoot([]string{"a", "b", "c", "d"})
	// 5 hashes (odd)
	r5 := merkle.BuildMerkleRoot([]string{"a", "b", "c", "d", "e"})
	if r4 == r5 {
		t.Error("roots for 4 and 5 hashes should differ")
	}
	if r4 == "" || r5 == "" {
		t.Error("roots should not be empty")
	}
}
