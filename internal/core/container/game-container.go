package container

import (
	"rgb-game/config"
	"rgb-game/internal/core/game"
	"rgb-game/internal/core/interfaces"
	"rgb-game/pkg/pb"

	"google.golang.org/grpc"
)

// GameContainer wires the GameModule into the gRPC server.
type GameContainer struct {
	gameModule *game.GameModule
}

func NewGameContainer(
	auth interfaces.FullAuthority,
	ledgerClient pb.LedgerServiceClient,
	gameCfg *config.GameConfig,
) *GameContainer {
	return &GameContainer{
		gameModule: game.NewGameModule(auth, ledgerClient, gameCfg),
	}
}

func (c *GameContainer) ServerRegister(grpcServer *grpc.Server) {
	pb.RegisterGameServiceServer(grpcServer, c.gameModule.Service())
}
