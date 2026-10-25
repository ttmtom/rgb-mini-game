//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"

	"rgb-game/pkg/crypto"
	"rgb-game/pkg/pb"
	"rgb-game/test/e2e/harness"
)

// TestFullGameFlow verifies the end-to-end mission loop:
// request mission → complete mission → check increased balance → transfer tokens.
func TestFullGameFlow(t *testing.T) {
	ctx := context.Background()
	aliceID, aliceKP := harness.NewPlayer(t)
	bobID, _ := harness.NewPlayer(t)

	// 1. Request a mission for Alice.
	missionResp, err := h.GameClient.RequestMission(ctx, &pb.RequestMissionRequest{PlayerId: aliceID})
	if err != nil {
		t.Fatalf("RequestMission: %v", err)
	}
	if missionResp.GetMissionId() == "" {
		t.Fatal("expected a mission ID")
	}

	// 2. Complete the mission — this triggers a MINT on the Ledger.
	completeResp, err := h.GameClient.CompleteMission(ctx, &pb.CompleteMissionRequest{
		MissionId: missionResp.GetMissionId(),
		PlayerId:  aliceID,
	})
	if err != nil {
		t.Fatalf("CompleteMission: %v", err)
	}
	if !completeResp.GetSuccess() {
		t.Fatalf("CompleteMission failed: %s", completeResp.GetErrorMessage())
	}

	// 3. Verify Alice's balance increased.
	aliceBal, err := h.LedgerClient.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: aliceID})
	if err != nil {
		t.Fatalf("GetBalance Alice: %v", err)
	}
	aliceTotal := aliceBal.GetRed() + aliceBal.GetGreen() + aliceBal.GetBlue()
	if aliceTotal == 0 {
		t.Fatal("Alice should have a positive balance after mission completion")
	}

	// 4. Alice transfers all her red tokens to Bob (if any).
	if aliceBal.GetRed() > 0 {
		transferResp := h.Transfer(ctx, t, aliceKP, aliceID, bobID, aliceBal.GetRed(), 0, 0, aliceBal.GetNextNonce())
		if !transferResp.GetSuccess() {
			t.Fatalf("TRANSFER failed: %s", transferResp.GetErrorMessage())
		}

		// Bob should now have Alice's red tokens.
		bobBal, _ := h.LedgerClient.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: bobID})
		if bobBal.GetRed() != aliceBal.GetRed() {
			t.Errorf("Bob.Red want %d got %d", aliceBal.GetRed(), bobBal.GetRed())
		}
	}
}

// TestChallengeFlow exercises the full ProposeChallenge → RespondChallenge flow,
// verifying that both legs of the token swap are settled on the Ledger.
func TestChallengeFlow(t *testing.T) {
	ctx := context.Background()
	aliceID, aliceKP := harness.NewPlayer(t)
	bobID, bobKP := harness.NewPlayer(t)

	// Seed balances: Alice gets R=20, Bob gets G=10.
	h.MintTo(ctx, t, aliceID, 20, 0, 0)
	h.MintTo(ctx, t, bobID, 0, 10, 0)

	// Alice's current nonce for the pre-signed TRANSFER.
	aliceBal, _ := h.LedgerClient.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: aliceID})
	bobBal, _ := h.LedgerClient.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: bobID})

	// Alice proposes: offer R=10, want G=5.
	// She pre-signs TRANSFER(alice→bob, R=10, G=0, B=0).
	alicePayload := &pb.TransactionPayload{
		Type:        pb.TransactionPayload_TRANSFER,
		SenderId:    aliceID,
		ReceiverId:  bobID,
		AmountRed:   10,
		AmountGreen: 0,
		AmountBlue:  0,
		Nonce:       aliceBal.GetNextNonce(),
	}
	aliceRaw, err := proto.Marshal(alicePayload)
	if err != nil {
		t.Fatalf("marshal alice payload: %v", err)
	}
	aliceSig := crypto.Sign(aliceKP.PrivateKey, aliceRaw)

	proposeResp, err := h.GameClient.ProposeChallenge(ctx, &pb.ProposeChallengeRequest{
		ProposerId:   aliceID,
		OpponentId:   bobID,
		OfferRed:     10,
		OfferGreen:   0,
		OfferBlue:    0,
		WantRed:      0,
		WantGreen:    5,
		WantBlue:     0,
		RawPayload:   aliceRaw,
		Signature:    aliceSig,
		SenderPubKey: aliceKP.PublicKey,
	})
	if err != nil {
		t.Fatalf("ProposeChallenge: %v", err)
	}
	if proposeResp.GetChallengeId() == "" {
		t.Fatalf("ProposeChallenge failed: %s", proposeResp.GetErrorMessage())
	}
	challengeID := proposeResp.GetChallengeId()

	// Bob accepts: pre-signs TRANSFER(bob→alice, R=0, G=5, B=0).
	// Amounts must match the challenge's want_* fields.
	bobPayload := &pb.TransactionPayload{
		Type:        pb.TransactionPayload_TRANSFER,
		SenderId:    bobID,
		ReceiverId:  aliceID,
		AmountRed:   0,
		AmountGreen: 5,
		AmountBlue:  0,
		Nonce:       bobBal.GetNextNonce(),
	}
	bobRaw, err := proto.Marshal(bobPayload)
	if err != nil {
		t.Fatalf("marshal bob payload: %v", err)
	}
	bobSig := crypto.Sign(bobKP.PrivateKey, bobRaw)

	respondResp, err := h.GameClient.RespondChallenge(ctx, &pb.RespondChallengeRequest{
		ChallengeId:  challengeID,
		PlayerId:     bobID,
		Accept:       true,
		RawPayload:   bobRaw,
		Signature:    bobSig,
		SenderPubKey: bobKP.PublicKey,
	})
	if err != nil {
		t.Fatalf("RespondChallenge: %v", err)
	}
	if !respondResp.GetSuccess() {
		t.Fatalf("RespondChallenge failed: %s", respondResp.GetErrorMessage())
	}

	// Verify balances reflect the swap: Alice -R=10 +G=5, Bob +R=10 -G=5.
	aliceAfter, _ := h.LedgerClient.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: aliceID})
	bobAfter, _ := h.LedgerClient.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: bobID})

	if aliceAfter.GetRed() != 10 {
		t.Errorf("Alice.Red: want 10, got %d", aliceAfter.GetRed())
	}
	if aliceAfter.GetGreen() != 5 {
		t.Errorf("Alice.Green: want 5, got %d", aliceAfter.GetGreen())
	}
	if bobAfter.GetRed() != 10 {
		t.Errorf("Bob.Red: want 10, got %d", bobAfter.GetRed())
	}
	if bobAfter.GetGreen() != 5 {
		t.Errorf("Bob.Green: want 5, got %d", bobAfter.GetGreen())
	}

	// Challenge should be marked completed.
	challengeState, err := h.GameClient.GetChallenge(ctx, &pb.GetChallengeRequest{ChallengeId: challengeID})
	if err != nil {
		t.Fatalf("GetChallenge: %v", err)
	}
	if challengeState.GetStatus() != pb.ChallengeStatus_COMPLETED {
		t.Errorf("expected challenge status COMPLETED, got %v", challengeState.GetStatus())
	}
}
