package types

// Mission represents a game mission definition used by the game engine.
// Reward holds signed int8 deltas applied to the player's R/G/B balance.
type Mission struct {
	Name   string
	Reward RGB
}

// MissionRecord represents the lifecycle state of an issued mission.
// Stored in Redis; RewardColor matches enum.Color ordinals (0=Red, 1=Green, 2=Blue).
type MissionRecord struct {
	ID          string `json:"id"`
	PlayerID    string `json:"player_id"`
	RewardColor int32  `json:"reward_color"`
	IssuedAt    int64  `json:"issued_at"` // unix timestamp
	Completed   bool   `json:"completed"`
}
