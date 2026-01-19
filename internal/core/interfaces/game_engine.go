package interfaces

import "rgb-game/internal/core/types"

type GameEngine interface {
	PlayerTransactions(from *types.PlayerState, to *types.PlayerState) (*types.PlayerState, *types.PlayerState, error)
	PlayerCompleteMission(play *types.PlayerState, mission *types.Mission) (*types.PlayerState, error)
}
