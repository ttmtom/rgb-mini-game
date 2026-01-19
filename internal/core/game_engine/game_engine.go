package game_engine

import "rgb-game/internal/core/types"

type GameEngine struct {
}

func NewGameEngine() *GameEngine {
	return &GameEngine{}
}

func (ge *GameEngine) calculation(color uint8, val int8) (uint8, *error) {
	// The calculation adds a value to a color component, clamping the result
	// to the valid 0-255 range for a uint8.
	result := int16(color) + int16(val)

	if result < 0 {
		return 0, nil
	}
	if result > 255 {
		return 255, nil
	}
	return uint8(result), nil
}

func (ge *GameEngine) PlayerTransactions(from *types.PlayerState, to *types.PlayerState) (*types.PlayerState, *types.PlayerState, error) {
	//TODO implement me
	panic("implement me")
}

func (ge *GameEngine) PlayerCompleteMission(play *types.PlayerState, mission *types.Mission) (*types.PlayerState, error) {
	//TODO implement me
	panic("implement me")
}
