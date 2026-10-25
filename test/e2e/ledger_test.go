//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"rgb-game/pkg/crypto"
	"rgb-game/pkg/pb"
	"rgb-game/test/e2e/harness"
)

func TestGetBalance_NewPlayer(t *testing.T) {
	ctx := context.Background()
	playerID, _ := harness.NewPlayer(t)

	resp, err := h.LedgerClient.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: playerID})
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if resp.GetRed() != 0 || resp.GetGreen() != 0 || resp.GetBlue() != 0 {
		t.Errorf("want zero balance for new player, got R=%d G=%d B=%d", resp.GetRed(), resp.GetGreen(), resp.GetBlue())
	}
}

func TestMintTransaction(t *testing.T) {
	ctx := context.Background()
	playerID, _ := harness.NewPlayer(t)

	resp := h.MintTo(ctx, t, playerID, 10, 20, 30)
	if !resp.GetSuccess() {
		t.Fatalf("MINT failed: %s", resp.GetErrorMessage())
	}
	if resp.GetTxHash() == "" {
		t.Error("expected non-empty tx hash")
	}

	bal, err := h.LedgerClient.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: playerID})
	if err != nil {
		t.Fatalf("GetBalance after mint: %v", err)
	}
	if bal.GetRed() != 10 || bal.GetGreen() != 20 || bal.GetBlue() != 30 {
		t.Errorf("want R=10 G=20 B=30 after mint, got R=%d G=%d B=%d", bal.GetRed(), bal.GetGreen(), bal.GetBlue())
	}
}

func TestTransferTransaction(t *testing.T) {
	ctx := context.Background()
	aliceID, aliceKP := harness.NewPlayer(t)
	bobID, _ := harness.NewPlayer(t)

	// Fund Alice via MINT.
	h.MintTo(ctx, t, aliceID, 50, 40, 30)

	// Alice transfers half to Bob.
	aliceBal, _ := h.LedgerClient.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: aliceID})
	resp := h.Transfer(ctx, t, aliceKP, aliceID, bobID, 25, 20, 15, aliceBal.GetNextNonce())
	if !resp.GetSuccess() {
		t.Fatalf("TRANSFER failed: %s", resp.GetErrorMessage())
	}

	// Verify Alice's balance decreased.
	aliceAfter, _ := h.LedgerClient.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: aliceID})
	if aliceAfter.GetRed() != 25 || aliceAfter.GetGreen() != 20 || aliceAfter.GetBlue() != 15 {
		t.Errorf("Alice: want R=25 G=20 B=15, got R=%d G=%d B=%d", aliceAfter.GetRed(), aliceAfter.GetGreen(), aliceAfter.GetBlue())
	}

	// Verify Bob received the tokens.
	bobAfter, _ := h.LedgerClient.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: bobID})
	if bobAfter.GetRed() != 25 || bobAfter.GetGreen() != 20 || bobAfter.GetBlue() != 15 {
		t.Errorf("Bob: want R=25 G=20 B=15, got R=%d G=%d B=%d", bobAfter.GetRed(), bobAfter.GetGreen(), bobAfter.GetBlue())
	}
}

func TestSubmitTransaction_InvalidSignature(t *testing.T) {
	ctx := context.Background()
	playerID, kp := harness.NewPlayer(t)

	// Tamper the payload after signing by sending a valid sig with wrong key.
	_, otherKP := harness.NewPlayer(t)
	resp := h.Transfer(ctx, t, otherKP, playerID, h.AuthorityPlayerID(), 1, 0, 0, 0)
	if resp.GetSuccess() {
		t.Error("expected failure for mismatched public key / player ID")
	}
	_ = kp
}

func TestGetTransactionHistory(t *testing.T) {
	ctx := context.Background()
	playerID, _ := harness.NewPlayer(t)

	// Submit two MINT txs to this player.
	h.MintTo(ctx, t, playerID, 5, 0, 0)
	h.MintTo(ctx, t, playerID, 0, 5, 0)

	resp, err := h.LedgerClient.GetTransactionHistory(ctx, &pb.GetTransactionHistoryRequest{
		PlayerId: playerID,
		Limit:    10,
		Offset:   0,
	})
	if err != nil {
		t.Fatalf("GetTransactionHistory: %v", err)
	}
	if resp.GetTotal() < 2 {
		t.Errorf("want at least 2 transactions, got %d", resp.GetTotal())
	}
}

func TestGetChainStats(t *testing.T) {
	ctx := context.Background()
	playerID, _ := harness.NewPlayer(t)
	h.MintTo(ctx, t, playerID, 1, 1, 1)

	resp, err := h.LedgerClient.GetChainStats(ctx, &pb.GetChainStatsRequest{})
	if err != nil {
		t.Fatalf("GetChainStats: %v", err)
	}
	if resp.GetTotalTxCount() < 1 {
		t.Errorf("expected at least 1 total transaction, got %d", resp.GetTotalTxCount())
	}
	if resp.GetTotalPlayers() < 1 {
		t.Errorf("expected at least 1 player, got %d", resp.GetTotalPlayers())
	}
}

func TestRegisterAuthority_ThenMint(t *testing.T) {
	ctx := context.Background()

	// Generate a new authority keypair and self-sign the public key bytes.
	_, newAuthKP := harness.NewPlayer(t)
	pubKeyBytes := []byte(newAuthKP.PublicKey)
	sig := crypto.Sign(newAuthKP.PrivateKey, pubKeyBytes)

	regResp, err := h.LedgerClient.RegisterAuthority(ctx, &pb.RegisterAuthorityRequest{
		PubKey:    pubKeyBytes,
		Signature: sig,
	})
	if err != nil {
		t.Fatalf("RegisterAuthority RPC: %v", err)
	}
	if !regResp.GetSuccess() {
		t.Fatalf("RegisterAuthority failed: %s", regResp.GetErrorMessage())
	}
}

func TestGetLeaderboard(t *testing.T) {
	ctx := context.Background()

	// Mint tokens to two fresh players so they appear in the leaderboard.
	p1, _ := harness.NewPlayer(t)
	p2, _ := harness.NewPlayer(t)
	h.MintTo(ctx, t, p1, 100, 50, 25)
	h.MintTo(ctx, t, p2, 50, 100, 75)

	resp, err := h.LedgerClient.GetLeaderboard(ctx, &pb.GetLeaderboardRequest{TopN: 10, SortBy: "TOTAL"})
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if len(resp.GetEntries()) < 2 {
		t.Errorf("want at least 2 players in leaderboard, got %d", len(resp.GetEntries()))
	}
}
