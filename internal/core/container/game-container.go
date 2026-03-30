package container

import (
	"rgb-game/config"
	redisadapter "rgb-game/internal/adapter/redis"
	"rgb-game/internal/core/game"
	"rgb-game/internal/core/interfaces"
	"rgb-game/internal/core/mission"
	"rgb-game/pkg/pb"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

// GameContainer wires the GameModule into the gRPC server.
type GameContainer struct {
	gameModule *game.GameModule
}

func NewGameContainer(
	redisClient *redis.Client,
	auth interfaces.FullAuthority,
	ledgerClient pb.LedgerServiceClient,
	gameCfg *config.GameConfig,
) *GameContainer {
	missionRepo := redisadapter.NewMissionRepository(redisClient)
	missionModule := mission.NewMissionModule(missionRepo, gameCfg)
	gameModule := game.NewGameModule(missionModule, auth, ledgerClient, gameCfg)

	return &GameContainer{gameModule: gameModule}
}

func (c *GameContainer) ServerRegister(grpcServer *grpc.Server) {
	pb.RegisterGameServiceServer(grpcServer, c.gameModule.Service())
}
