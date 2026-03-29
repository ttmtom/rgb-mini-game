package container

import (
	"crypto/ed25519"
	"time"

	"rgb-game/internal/core/game"
	"rgb-game/pkg/pb"

	"google.golang.org/grpc"
)

// GameContainer wires the GameModule into the gRPC server.
type GameContainer struct {
	gameModule *game.GameModule
}

func NewGameContainer(
	authorityID string,
	authorityPubKey ed25519.PublicKey,
	authorityPrivKey ed25519.PrivateKey,
	ledgerClient pb.LedgerServiceClient,
	cooldown time.Duration,
) *GameContainer {
	return &GameContainer{
		gameModule: game.NewGameModule(authorityID, authorityPubKey, authorityPrivKey, ledgerClient, cooldown),
	}
}

func (c *GameContainer) ServerRegister(grpcServer *grpc.Server) {
	pb.RegisterGameServiceServer(grpcServer, c.gameModule.Service())
}
