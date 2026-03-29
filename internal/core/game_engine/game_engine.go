package game_engine

import (
	"errors"
	"rgb-game/internal/core/types"
)

type GameEngine struct {
}

func NewGameEngine() *GameEngine {
	return &GameEngine{}
}

// calculation adds val (signed delta) to color, clamping the result to [0, 255].
func (ge *GameEngine) calculation(color uint8, val int8) (uint8, *error) {
	result := int16(color) + int16(val)
	if result < 0 {
		return 0, nil
	}
	if result > 255 {
		return 255, nil
	}
	return uint8(result), nil
}

// clampUint8 clamps an int32 value to the uint8 range [0, 255].
func clampUint8(v int32) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// PlayerTransactions applies a token transfer between two players.
//
// Calling convention:
//   - from.R/G/B  = amounts to transfer (from TransactionPayload.amount_*)
//   - from.Nonce  = sender's current nonce
//   - to.R/G/B    = receiver's current balance
//   - to.Nonce    = receiver's current nonce
//
// Returns:
//   - newFrom: {R:0, G:0, B:0, Nonce+1} — the consumed amounts (zeroed)
//   - newTo:   receiver balance increased by the transferred amounts, Nonce+1
//
// WHY the GameEngine does NOT validate sender balance:
//
// The interface only carries two PlayerState values. With from.R/G/B already
// used for transfer amounts, there is no channel to also pass the sender's
// current balance — the engine never sees it, so it cannot check it.
//
// All pre-call validation is the Ledger Service's responsibility:
//  1. Verify ed25519 signature.
//  2. Verify payload.Nonce == senderModel.Nonce (replay protection).
//  3. Verify senderModel.Red/Green/Blue >= payload amounts (sufficient funds).
//  4. Call this function inside a DB transaction (SELECT FOR UPDATE on both rows).
//
// After this call the service must:
//   - Subtract the transfer amounts from the sender's stored balance.
//   - Use newTo.R/G/B as the receiver's new stored balance.
//   - Persist both models and record the TransactionModel.
func (ge *GameEngine) PlayerTransactions(from *types.PlayerState, to *types.PlayerState) (*types.PlayerState, *types.PlayerState, error) {
	if from == nil || to == nil {
		return nil, nil, errors.New("player states must not be nil")
	}

	// Add the transferred amounts to the receiver, clamped to [0, 255].
	// We use int32 arithmetic here because amounts can be 0–255 (full uint8
	// range), which exceeds the positive half of int8 (0–127) used by
	// calculation(); the clamping semantics are identical.
	newToR := clampUint8(int32(to.R) + int32(from.R))
	newToG := clampUint8(int32(to.G) + int32(from.G))
	newToB := clampUint8(int32(to.B) + int32(from.B))

	newFrom := &types.PlayerState{
		R:     0,
		G:     0,
		B:     0,
		Nonce: from.Nonce + 1,
	}
	newTo := &types.PlayerState{
		R:     newToR,
		G:     newToG,
		B:     newToB,
		Nonce: to.Nonce + 1,
	}
	return newFrom, newTo, nil
}

// PlayerCompleteMission applies a mission reward to the player's state.
// mission.Reward.R/G/B are signed int8 deltas that are added to the player's
// current R/G/B balance via calculation(), which clamps each channel to [0, 255].
func (ge *GameEngine) PlayerCompleteMission(play *types.PlayerState, mission *types.Mission) (*types.PlayerState, error) {
	if play == nil || mission == nil {
		return nil, errors.New("player state and mission must not be nil")
	}

	newR, _ := ge.calculation(play.R, mission.Reward.R)
	newG, _ := ge.calculation(play.G, mission.Reward.G)
	newB, _ := ge.calculation(play.B, mission.Reward.B)

	return &types.PlayerState{
		R:     newR,
		G:     newG,
		B:     newB,
		Nonce: play.Nonce + 1,
	}, nil
}
