package game

import (
	"time"

	"rgb-game/internal/core/interfaces"
	"rgb-game/pkg/pb"
)

// GameModule wires together the GameService dependencies.
type GameModule struct {
	service *GameService
}

func NewGameModule(
	auth interfaces.FullAuthority,
	ledgerClient pb.LedgerServiceClient,
	cooldown time.Duration,
) *GameModule {
	return &GameModule{
		service: newGameService(auth, ledgerClient, cooldown),
	}
}

func (m *GameModule) Service() *GameService {
	return m.service
}
