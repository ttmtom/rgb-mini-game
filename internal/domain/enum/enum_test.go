package enum_test

import (
	"testing"

	"rgb-game/internal/domain/enum"
)

// Ordinal values must match pb.RewardColor constants in game.proto.

func TestColor_Ordinals(t *testing.T) {
	tests := []struct {
		color enum.Color
		want  uint8
		name  string
	}{
		{enum.Red, 0, "Red"},
		{enum.Green, 1, "Green"},
		{enum.Blue, 2, "Blue"},
	}
	for _, tt := range tests {
		if uint8(tt.color) != tt.want {
			t.Errorf("%s ordinal want %d got %d", tt.name, tt.want, uint8(tt.color))
		}
	}
}

func TestColor_Distinct(t *testing.T) {
	if enum.Red == enum.Green || enum.Green == enum.Blue || enum.Red == enum.Blue {
		t.Error("Color constants must all be distinct")
	}
}
