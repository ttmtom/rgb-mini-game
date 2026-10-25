//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"rgb-game/pkg/pb"
	"rgb-game/test/e2e/harness"
)

func TestRequestMission(t *testing.T) {
	ctx := context.Background()
	playerID, _ := harness.NewPlayer(t)

	resp, err := h.GameClient.RequestMission(ctx, &pb.RequestMissionRequest{PlayerId: playerID})
	if err != nil {
		t.Fatalf("RequestMission: %v", err)
	}
	if resp.GetMissionId() == "" {
		t.Error("expected non-empty mission ID")
	}
	if resp.GetCooldownSeconds() != 0 {
		t.Errorf("expected zero cooldown on first request, got %d", resp.GetCooldownSeconds())
	}
	// At least one channel should have a positive reward.
	if resp.GetRewardRed() == 0 && resp.GetRewardGreen() == 0 && resp.GetRewardBlue() == 0 {
		t.Error("expected at least one non-zero reward channel")
	}
}

func TestRequestMission_ReturnsExistingIfActive(t *testing.T) {
	ctx := context.Background()
	playerID, _ := harness.NewPlayer(t)

	first, err := h.GameClient.RequestMission(ctx, &pb.RequestMissionRequest{PlayerId: playerID})
	if err != nil {
		t.Fatalf("first RequestMission: %v", err)
	}
	second, err := h.GameClient.RequestMission(ctx, &pb.RequestMissionRequest{PlayerId: playerID})
	if err != nil {
		t.Fatalf("second RequestMission: %v", err)
	}
	// Should return the same mission while it's still active.
	if first.GetMissionId() != second.GetMissionId() {
		t.Errorf("expected same mission ID: first=%s second=%s", first.GetMissionId(), second.GetMissionId())
	}
}

func TestCompleteMission(t *testing.T) {
	ctx := context.Background()
	playerID, _ := harness.NewPlayer(t)

	missionResp, err := h.GameClient.RequestMission(ctx, &pb.RequestMissionRequest{PlayerId: playerID})
	if err != nil {
		t.Fatalf("RequestMission: %v", err)
	}

	completeResp, err := h.GameClient.CompleteMission(ctx, &pb.CompleteMissionRequest{
		MissionId: missionResp.GetMissionId(),
		PlayerId:  playerID,
	})
	if err != nil {
		t.Fatalf("CompleteMission RPC: %v", err)
	}
	if !completeResp.GetSuccess() {
		t.Fatalf("CompleteMission failed: %s", completeResp.GetErrorMessage())
	}
	if completeResp.GetTxHash() == "" {
		t.Error("expected non-empty tx hash after mission completion")
	}

	// The new balance should reflect minted tokens.
	newBal := completeResp.GetNewBalance()
	if newBal == nil {
		t.Fatal("expected new_balance in CompleteMissionResponse")
	}
	total := newBal.GetRed() + newBal.GetGreen() + newBal.GetBlue()
	if total == 0 {
		t.Error("expected non-zero token balance after completing mission")
	}
}

func TestCompleteMission_WrongPlayer(t *testing.T) {
	ctx := context.Background()
	playerID, _ := harness.NewPlayer(t)
	otherPlayerID, _ := harness.NewPlayer(t)

	missionResp, err := h.GameClient.RequestMission(ctx, &pb.RequestMissionRequest{PlayerId: playerID})
	if err != nil {
		t.Fatalf("RequestMission: %v", err)
	}

	// Try to complete the mission as a different player.
	resp, err := h.GameClient.CompleteMission(ctx, &pb.CompleteMissionRequest{
		MissionId: missionResp.GetMissionId(),
		PlayerId:  otherPlayerID,
	})
	if err != nil {
		t.Fatalf("CompleteMission RPC: %v", err)
	}
	if resp.GetSuccess() {
		t.Error("expected CompleteMission to fail for wrong player")
	}
}

func TestMissionCooldown(t *testing.T) {
	ctx := context.Background()
	playerID, _ := harness.NewPlayer(t)

	// Request and complete a mission.
	missionResp, _ := h.GameClient.RequestMission(ctx, &pb.RequestMissionRequest{PlayerId: playerID})
	completeResp, err := h.GameClient.CompleteMission(ctx, &pb.CompleteMissionRequest{
		MissionId: missionResp.GetMissionId(),
		PlayerId:  playerID,
	})
	if err != nil {
		t.Fatalf("CompleteMission: %v", err)
	}
	if !completeResp.GetSuccess() {
		t.Fatalf("CompleteMission failed: %s", completeResp.GetErrorMessage())
	}

	// Immediately request another mission — should be in cooldown.
	cooldownResp, err := h.GameClient.RequestMission(ctx, &pb.RequestMissionRequest{PlayerId: playerID})
	if err != nil {
		t.Fatalf("second RequestMission: %v", err)
	}
	if cooldownResp.GetCooldownSeconds() <= 0 {
		t.Error("expected positive cooldown_seconds after completing a mission")
	}
}
