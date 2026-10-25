package mission

import "rgb-game/internal/domain/types"

// Difficulty tiers for missions.
type Difficulty int32

const (
	Easy   Difficulty = 0
	Medium Difficulty = 1
	Hard   Difficulty = 2
)

// CooldownMultiplier returns the cooldown multiplier for a difficulty tier.
// Easy = ×1, Medium = ×2, Hard = ×4.
func (d Difficulty) CooldownMultiplier() int32 {
	switch d {
	case Medium:
		return 2
	case Hard:
		return 4
	default:
		return 1
	}
}

// Template defines a mission's difficulty and multi-channel RGB reward deltas.
type Template struct {
	Difficulty  Difficulty
	Reward      types.RGB
	PrimaryColor int32 // enum.Color ordinal of the dominant reward channel
}

// Pool is the canonical library of mission templates sampled at RequestMission time.
// Templates are organised from easy (small single-color gains) to hard
// (large multi-color combos).  All reward values are positive int8 deltas.
var Pool = []Template{
	// ── Easy: +1 to a single channel ─────────────────────────────────────
	{Easy, types.RGB{R: 1, G: 0, B: 0}, 0},
	{Easy, types.RGB{R: 0, G: 1, B: 0}, 1},
	{Easy, types.RGB{R: 0, G: 0, B: 1}, 2},

	// ── Medium: +3 single-channel or +2/+1 multi-channel ─────────────────
	{Medium, types.RGB{R: 3, G: 0, B: 0}, 0},
	{Medium, types.RGB{R: 0, G: 3, B: 0}, 1},
	{Medium, types.RGB{R: 0, G: 0, B: 3}, 2},
	{Medium, types.RGB{R: 2, G: 1, B: 0}, 0},
	{Medium, types.RGB{R: 0, G: 2, B: 1}, 1},
	{Medium, types.RGB{R: 1, G: 0, B: 2}, 2},

	// ── Hard: +5 single-channel or +3/+2 multi-channel combos ────────────
	{Hard, types.RGB{R: 5, G: 0, B: 0}, 0},
	{Hard, types.RGB{R: 0, G: 5, B: 0}, 1},
	{Hard, types.RGB{R: 0, G: 0, B: 5}, 2},
	{Hard, types.RGB{R: 3, G: 2, B: 0}, 0},
	{Hard, types.RGB{R: 0, G: 3, B: 2}, 1},
	{Hard, types.RGB{R: 2, G: 0, B: 3}, 2},
	{Hard, types.RGB{R: 2, G: 2, B: 1}, 0},
	{Hard, types.RGB{R: 1, G: 2, B: 2}, 2},
}
