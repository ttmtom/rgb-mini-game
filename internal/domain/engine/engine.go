package engine

import (
	"errors"
	"rgb-game/internal/domain/types"
)

// Engine implements the core game calculation rules.
type Engine struct{}

func New() *Engine {
	return &Engine{}
}

// calculation adds val (signed delta) to color, clamping the result to [0, 255].
func (e *Engine) calculation(color uint8, val int8) uint8 {
	result := int16(color) + int16(val)
	if result < 0 {
		return 0
	}
	if result > 255 {
		return 255
	}
	return uint8(result)
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
func (e *Engine) PlayerTransactions(from *types.PlayerState, to *types.PlayerState) (*types.PlayerState, *types.PlayerState, error) {
	if from == nil || to == nil {
		return nil, nil, errors.New("player states must not be nil")
	}

	newToR := clampUint8(int32(to.R) + int32(from.R))
	newToG := clampUint8(int32(to.G) + int32(from.G))
	newToB := clampUint8(int32(to.B) + int32(from.B))

	newFrom := &types.PlayerState{R: 0, G: 0, B: 0, Nonce: from.Nonce + 1}
	newTo := &types.PlayerState{R: newToR, G: newToG, B: newToB, Nonce: to.Nonce + 1}
	return newFrom, newTo, nil
}

// PlayerCompleteMission applies a mission reward to the player's state.
// mission.Reward.R/G/B are signed int8 deltas clamped to [0, 255].
func (e *Engine) PlayerCompleteMission(play *types.PlayerState, mission *types.Mission) (*types.PlayerState, error) {
	if play == nil || mission == nil {
		return nil, errors.New("player state and mission must not be nil")
	}
	return &types.PlayerState{
		R:     e.calculation(play.R, mission.Reward.R),
		G:     e.calculation(play.G, mission.Reward.G),
		B:     e.calculation(play.B, mission.Reward.B),
		Nonce: play.Nonce + 1,
	}, nil
}
