package enum

// Color represents a single RGB channel, matching proto RewardColor ordinals.
type Color uint8

const (
	Red   Color = 0
	Green Color = 1
	Blue  Color = 2
)
