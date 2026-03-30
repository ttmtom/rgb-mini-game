package game

import (
	"rgb-game/config"
	"rgb-game/internal/core/interfaces"
	missionpkg "rgb-game/internal/core/mission"
	"rgb-game/pkg/pb"
)

// GameModule wires together the GameService dependencies.
type GameModule struct {
	service *GameService
}

func NewGameModule(
	missionModule *missionpkg.MissionModule,
	auth interfaces.FullAuthority,
	ledgerClient pb.LedgerServiceClient,
	cfg *config.GameConfig,
) *GameModule {
	return &GameModule{
		service: newGameService(missionModule.Service(), auth, ledgerClient, cfg),
	}
}

func (m *GameModule) Service() *GameService {
	return m.service
}
