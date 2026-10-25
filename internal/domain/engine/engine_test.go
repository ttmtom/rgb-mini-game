package engine_test

import (
	"testing"

	"rgb-game/internal/domain/engine"
	"rgb-game/internal/domain/types"
)

var eng = engine.New()

// ─── PlayerTransactions ───────────────────────────────────────────────────────

func TestPlayerTransactions_BasicTransfer(t *testing.T) {
	from := &types.PlayerState{R: 10, G: 5, B: 3, Nonce: 1}
	to := &types.PlayerState{R: 20, G: 30, B: 40, Nonce: 7}

	newFrom, newTo, err := eng.PlayerTransactions(from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// sender amounts are zeroed; nonce increments
	if newFrom.R != 0 || newFrom.G != 0 || newFrom.B != 0 {
		t.Errorf("newFrom RGB should be 0,0,0 got %d,%d,%d", newFrom.R, newFrom.G, newFrom.B)
	}
	if newFrom.Nonce != 2 {
		t.Errorf("newFrom.Nonce want 2 got %d", newFrom.Nonce)
	}
	// receiver accumulates from's amounts
	if newTo.R != 30 || newTo.G != 35 || newTo.B != 43 {
		t.Errorf("newTo RGB want 30,35,43 got %d,%d,%d", newTo.R, newTo.G, newTo.B)
	}
	if newTo.Nonce != 8 {
		t.Errorf("newTo.Nonce want 8 got %d", newTo.Nonce)
	}
}

func TestPlayerTransactions_ClampAtMax(t *testing.T) {
	from := &types.PlayerState{R: 200, G: 200, B: 200, Nonce: 0}
	to := &types.PlayerState{R: 100, G: 100, B: 100, Nonce: 0}

	_, newTo, err := eng.PlayerTransactions(from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 200 + 100 = 300 → clamped to 255
	if newTo.R != 255 || newTo.G != 255 || newTo.B != 255 {
		t.Errorf("clamping failed: got %d,%d,%d", newTo.R, newTo.G, newTo.B)
	}
}

func TestPlayerTransactions_ZeroTransfer(t *testing.T) {
	from := &types.PlayerState{R: 0, G: 0, B: 0, Nonce: 5}
	to := &types.PlayerState{R: 10, G: 20, B: 30, Nonce: 3}

	newFrom, newTo, err := eng.PlayerTransactions(from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newTo.R != 10 || newTo.G != 20 || newTo.B != 30 {
		t.Errorf("receiver should be unchanged: got %d,%d,%d", newTo.R, newTo.G, newTo.B)
	}
	if newFrom.Nonce != 6 {
		t.Errorf("nonce should still increment: got %d", newFrom.Nonce)
	}
}

func TestPlayerTransactions_NilInputs(t *testing.T) {
	_, _, err := eng.PlayerTransactions(nil, &types.PlayerState{})
	if err == nil {
		t.Error("expected error for nil from")
	}
	_, _, err = eng.PlayerTransactions(&types.PlayerState{}, nil)
	if err == nil {
		t.Error("expected error for nil to")
	}
}

// ─── PlayerCompleteMission ────────────────────────────────────────────────────

func TestPlayerCompleteMission_PositiveDelta(t *testing.T) {
	player := &types.PlayerState{R: 10, G: 20, B: 30, Nonce: 4}
	mission := &types.Mission{Reward: types.RGB{R: 5, G: 10, B: 15}}

	newState, err := eng.PlayerCompleteMission(player, mission)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newState.R != 15 || newState.G != 30 || newState.B != 45 {
		t.Errorf("want 15,30,45 got %d,%d,%d", newState.R, newState.G, newState.B)
	}
	if newState.Nonce != 5 {
		t.Errorf("nonce want 5 got %d", newState.Nonce)
	}
}

func TestPlayerCompleteMission_NegativeDelta_ClampToZero(t *testing.T) {
	player := &types.PlayerState{R: 2, G: 2, B: 2, Nonce: 0}
	mission := &types.Mission{Reward: types.RGB{R: -5, G: -5, B: -5}}

	newState, err := eng.PlayerCompleteMission(player, mission)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 2 + (-5) = -3 → clamped to 0
	if newState.R != 0 || newState.G != 0 || newState.B != 0 {
		t.Errorf("clamping below 0 failed: got %d,%d,%d", newState.R, newState.G, newState.B)
	}
}

func TestPlayerCompleteMission_ClampAtMax(t *testing.T) {
	player := &types.PlayerState{R: 250, G: 250, B: 250, Nonce: 0}
	mission := &types.Mission{Reward: types.RGB{R: 100, G: 100, B: 100}}

	newState, err := eng.PlayerCompleteMission(player, mission)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 250 + 100 = 350 → clamped to 255
	if newState.R != 255 || newState.G != 255 || newState.B != 255 {
		t.Errorf("clamping above 255 failed: got %d,%d,%d", newState.R, newState.G, newState.B)
	}
}

func TestPlayerCompleteMission_NilInputs(t *testing.T) {
	_, err := eng.PlayerCompleteMission(nil, &types.Mission{})
	if err == nil {
		t.Error("expected error for nil player")
	}
	_, err = eng.PlayerCompleteMission(&types.PlayerState{}, nil)
	if err == nil {
		t.Error("expected error for nil mission")
	}
}

func TestPlayerCompleteMission_MultiColorReward(t *testing.T) {
	player := &types.PlayerState{R: 0, G: 100, B: 200, Nonce: 9}
	mission := &types.Mission{Reward: types.RGB{R: 3, G: 2, B: 1}}

	newState, err := eng.PlayerCompleteMission(player, mission)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newState.R != 3 || newState.G != 102 || newState.B != 201 {
		t.Errorf("multi-color reward failed: got %d,%d,%d", newState.R, newState.G, newState.B)
	}
}
