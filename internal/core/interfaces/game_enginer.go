package interfaces

import "rgb-game/internal/core/types"

type GameEngine interface {
	Start() error
	GetPlayerState(id string) (*types.PlayerState, error)
}
