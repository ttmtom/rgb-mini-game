package game

import (
	"crypto/ed25519"
	"time"

	"rgb-game/pkg/pb"
)

// GameModule wires together the GameService dependencies.
type GameModule struct {
	service *GameService
}

func NewGameModule(
	authorityID string,
	authorityPubKey ed25519.PublicKey,
	authorityPrivKey ed25519.PrivateKey,
	ledgerClient pb.LedgerServiceClient,
	cooldown time.Duration,
) *GameModule {
	return &GameModule{
		service: newGameService(authorityID, authorityPubKey, authorityPrivKey, ledgerClient, cooldown),
	}
}

func (m *GameModule) Service() *GameService {
	return m.service
}
