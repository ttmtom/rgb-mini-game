package out

import "rgb-game/internal/domain/types"

// GameEngine defines the domain calculation contract.
type GameEngine interface {
	PlayerTransactions(from *types.PlayerState, to *types.PlayerState) (*types.PlayerState, *types.PlayerState, error)
	PlayerCompleteMission(play *types.PlayerState, mission *types.Mission) (*types.PlayerState, error)
}
