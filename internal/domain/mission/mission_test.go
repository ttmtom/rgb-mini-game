package mission_test

import (
	"testing"

	"rgb-game/internal/domain/mission"
)

// ─── Difficulty.CooldownMultiplier ───────────────────────────────────────────

func TestCooldownMultiplier_Easy(t *testing.T) {
	if got := mission.Easy.CooldownMultiplier(); got != 1 {
		t.Errorf("Easy.CooldownMultiplier() = %d, want 1", got)
	}
}

func TestCooldownMultiplier_Medium(t *testing.T) {
	if got := mission.Medium.CooldownMultiplier(); got != 2 {
		t.Errorf("Medium.CooldownMultiplier() = %d, want 2", got)
	}
}

func TestCooldownMultiplier_Hard(t *testing.T) {
	if got := mission.Hard.CooldownMultiplier(); got != 4 {
		t.Errorf("Hard.CooldownMultiplier() = %d, want 4", got)
	}
}

func TestCooldownMultiplier_UnknownDefaultsToOne(t *testing.T) {
	unknown := mission.Difficulty(99)
	if got := unknown.CooldownMultiplier(); got != 1 {
		t.Errorf("unknown difficulty CooldownMultiplier() = %d, want 1", got)
	}
}

// ─── Pool ────────────────────────────────────────────────────────────────────

func TestPool_NotEmpty(t *testing.T) {
	if len(mission.Pool) == 0 {
		t.Error("mission.Pool must not be empty")
	}
}

func TestPool_AllEntriesHaveValidDifficulty(t *testing.T) {
	for i, tmpl := range mission.Pool {
		switch tmpl.Difficulty {
		case mission.Easy, mission.Medium, mission.Hard:
			// valid
		default:
			t.Errorf("Pool[%d] has unknown difficulty %d", i, tmpl.Difficulty)
		}
	}
}

func TestPool_AllEntriesHaveNonZeroReward(t *testing.T) {
	for i, tmpl := range mission.Pool {
		r := tmpl.Reward
		if r.R == 0 && r.G == 0 && r.B == 0 {
			t.Errorf("Pool[%d] has all-zero reward (Difficulty=%d)", i, tmpl.Difficulty)
		}
	}
}

func TestPool_PrimaryColorInValidRange(t *testing.T) {
	for i, tmpl := range mission.Pool {
		if tmpl.PrimaryColor < 0 || tmpl.PrimaryColor > 2 {
			t.Errorf("Pool[%d] has out-of-range PrimaryColor %d (must be 0–2)", i, tmpl.PrimaryColor)
		}
	}
}

func TestPool_ContainsAllDifficulties(t *testing.T) {
	seen := map[mission.Difficulty]bool{}
	for _, tmpl := range mission.Pool {
		seen[tmpl.Difficulty] = true
	}
	for _, d := range []mission.Difficulty{mission.Easy, mission.Medium, mission.Hard} {
		if !seen[d] {
			t.Errorf("Pool is missing entries for difficulty %d", d)
		}
	}
}

func TestPool_HardRewardsLargerThanEasy(t *testing.T) {
	maxEasy := int8(0)
	maxHard := int8(0)
	for _, tmpl := range mission.Pool {
		for _, v := range []int8{tmpl.Reward.R, tmpl.Reward.G, tmpl.Reward.B} {
			if tmpl.Difficulty == mission.Easy && v > maxEasy {
				maxEasy = v
			}
			if tmpl.Difficulty == mission.Hard && v > maxHard {
				maxHard = v
			}
		}
	}
	if maxHard <= maxEasy {
		t.Errorf("Hard missions should offer larger rewards than Easy: maxHard=%d maxEasy=%d", maxHard, maxEasy)
	}
}
