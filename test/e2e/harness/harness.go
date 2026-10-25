//go:build e2e

package harness

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"rgb-game/internal/adapter/driven/authority"
	drivenledger "rgb-game/internal/adapter/driven/ledger"
	"rgb-game/internal/adapter/driven/postgres/repositories"
	drivenredis "rgb-game/internal/adapter/driven/redis"
	grpcgame "rgb-game/internal/adapter/driving/grpc/game"
	grpcledger "rgb-game/internal/adapter/driving/grpc/ledger"
	"rgb-game/internal/app/service"
	"rgb-game/internal/domain/engine"
	"rgb-game/internal/domain/types"
	"rgb-game/pkg/crypto"
	"rgb-game/pkg/logger"
	"rgb-game/pkg/pb"
)

const bufSize = 1024 * 1024

// Harness holds live in-process gRPC servers backed by real Docker containers.
// Create one per test binary via TestMain and call Cleanup() in defer.
type Harness struct {
	LedgerClient pb.LedgerServiceClient
	GameClient   pb.GameServiceClient
	// AuthorityKP is the genesis authority keypair; tests can sign MINT txs with it.
	AuthorityKP *crypto.Keypair

	cancel         context.CancelFunc
	ledgerListener *bufconn.Listener
	gameListener   *bufconn.Listener
	ledgerServer   *grpc.Server
	gameServer     *grpc.Server
	ledgerConn     *grpc.ClientConn
	gameConn       *grpc.ClientConn
}

// New starts Postgres + Redis testcontainers, wires all adapters in-process,
// starts Ledger and Game gRPC servers on bufconn listeners, and returns the
// ready harness plus a cleanup function.
func New(ctx context.Context) (*Harness, func(), error) {
	logger.Init()

	ctx, cancel := context.WithCancel(ctx)
	cleanup := func() { cancel() }

	fail := func(err error, msg string) (*Harness, func(), error) {
		cleanup()
		return nil, nil, fmt.Errorf("%s: %w", msg, err)
	}

	// ── Postgres ─────────────────────────────────────────────────────────
	pgContainer, err := tcpostgres.Run(ctx, "postgres:17.0",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
	)
	if err != nil {
		return fail(err, "start postgres container")
	}
	cleanup = chain(cleanup, func() { _ = pgContainer.Terminate(context.Background()) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fail(err, "get postgres connection string")
	}

	db, err := gorm.Open(gormpg.Open(connStr), &gorm.Config{})
	if err != nil {
		return fail(err, "open gorm connection")
	}
	if err := repositories.AutoMigrate(db); err != nil {
		return fail(err, "auto migrate")
	}

	// ── Redis ─────────────────────────────────────────────────────────────
	redisContainer, err := tcredis.Run(ctx, "redis:8.6.2-alpine")
	if err != nil {
		return fail(err, "start redis container")
	}
	cleanup = chain(cleanup, func() { _ = redisContainer.Terminate(context.Background()) })

	redisConnStr, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		return fail(err, "get redis connection string")
	}
	redisAddr, err := parseRedisAddr(redisConnStr)
	if err != nil {
		return fail(err, "parse redis addr")
	}

	redisClient := goredis.NewClient(&goredis.Options{Addr: redisAddr})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fail(err, "ping redis")
	}
	cleanup = chain(cleanup, func() { _ = redisClient.Close() })

	// ── Authority keypair ─────────────────────────────────────────────────
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		return fail(err, "generate authority keypair")
	}
	auth := authority.NewFullAuthority(kp)

	// ── Repositories ──────────────────────────────────────────────────────
	playerRepo := repositories.NewPlayerRepository(db)
	txRepo := repositories.NewTransactionRepository(db)
	blockRepo := repositories.NewBlockRepository(db)
	authorityRepo := repositories.NewAuthorityRepository(db)
	transactor := repositories.NewTransactor(db)
	missionRepo := drivenredis.NewMissionRepository(redisClient)
	challengeRepo := drivenredis.NewChallengeRepository(redisClient)

	// Seed genesis authority.
	if err := authorityRepo.Register(ctx, &types.AuthorityRecord{
		ID:           auth.PlayerID(),
		PubKeyHex:    hex.EncodeToString(auth.PubKey()),
		RegisteredAt: time.Now().Unix(),
	}); err != nil {
		return fail(err, "seed genesis authority")
	}

	// ── Ledger gRPC server (bufconn) ─────────────────────────────────────
	gameEng := engine.New()
	ledgerSvc := service.NewLedgerService(playerRepo, txRepo, blockRepo, authorityRepo, transactor, gameEng)
	ledgerListener := bufconn.Listen(bufSize)
	ledgerHandler := grpcledger.New(ledgerSvc, authorityRepo)
	ledgerServer := grpc.NewServer()
	pb.RegisterLedgerServiceServer(ledgerServer, ledgerHandler)
	go func() { _ = ledgerServer.Serve(ledgerListener) }()

	// ── Ledger gRPC client (for harness helpers + Game server) ───────────
	ledgerConn, err := dialBufconn(ledgerListener)
	if err != nil {
		return fail(err, "dial ledger bufconn")
	}
	ledgerPbClient := pb.NewLedgerServiceClient(ledgerConn)

	// ── Game gRPC server (bufconn) ────────────────────────────────────────
	ledgerClient := drivenledger.New(ledgerPbClient)
	gameSvc := service.NewGameService(missionRepo, auth, ledgerClient, challengeRepo, service.GameServiceConfig{
		MissionCooldown: 5 * time.Second,
		ChallengeTTL:    2 * time.Minute,
	})
	gameListener := bufconn.Listen(bufSize)
	gameHandler := grpcgame.New(gameSvc)
	gameServer := grpc.NewServer()
	pb.RegisterGameServiceServer(gameServer, gameHandler)
	go func() { _ = gameServer.Serve(gameListener) }()

	// ── Game gRPC client ──────────────────────────────────────────────────
	gameConn, err := dialBufconn(gameListener)
	if err != nil {
		return fail(err, "dial game bufconn")
	}

	cleanup = chain(cleanup, func() {
		_ = ledgerConn.Close()
		_ = gameConn.Close()
		ledgerServer.GracefulStop()
		gameServer.GracefulStop()
	})

	h := &Harness{
		LedgerClient:   pb.NewLedgerServiceClient(ledgerConn),
		GameClient:     pb.NewGameServiceClient(gameConn),
		AuthorityKP:    kp,
		cancel:         cancel,
		ledgerListener: ledgerListener,
		gameListener:   gameListener,
		ledgerServer:   ledgerServer,
		gameServer:     gameServer,
		ledgerConn:     ledgerConn,
		gameConn:       gameConn,
	}
	return h, cleanup, nil
}

// NewPlayer generates a fresh ed25519 keypair and returns the player ID
// (hex-sha256 of the public key) along with the keypair for signing.
func NewPlayer(t *testing.T) (playerID string, kp *crypto.Keypair) {
	t.Helper()
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatalf("generate player keypair: %v", err)
	}
	return crypto.PubKeyToPlayerID(kp.PublicKey), kp
}

// MintTo submits a signed MINT transaction from the genesis authority to the given player.
func (h *Harness) MintTo(ctx context.Context, t *testing.T, playerID string, r, g, b uint32) *pb.SubmitTransactionResponse {
	t.Helper()
	nonceResp, err := h.LedgerClient.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: h.AuthorityPlayerID()})
	mustNil(t, err, "get authority nonce")

	payload := &pb.TransactionPayload{
		Type:        pb.TransactionPayload_MINT,
		SenderId:    h.AuthorityPlayerID(),
		ReceiverId:  playerID,
		AmountRed:   r,
		AmountGreen: g,
		AmountBlue:  b,
		Nonce:       nonceResp.GetNextNonce(),
	}
	return h.submitSigned(ctx, t, h.AuthorityKP, payload)
}

// Transfer submits a signed TRANSFER transaction from kp's player to toID.
func (h *Harness) Transfer(ctx context.Context, t *testing.T, kp *crypto.Keypair, fromID, toID string, r, g, b uint32, nonce uint64) *pb.SubmitTransactionResponse {
	t.Helper()
	payload := &pb.TransactionPayload{
		Type:        pb.TransactionPayload_TRANSFER,
		SenderId:    fromID,
		ReceiverId:  toID,
		AmountRed:   r,
		AmountGreen: g,
		AmountBlue:  b,
		Nonce:       nonce,
	}
	return h.submitSigned(ctx, t, kp, payload)
}

// AuthorityPlayerID returns the player ID derived from the genesis authority keypair.
func (h *Harness) AuthorityPlayerID() string {
	return crypto.PubKeyToPlayerID(h.AuthorityKP.PublicKey)
}

// submitSigned marshals the payload, signs it, and calls SubmitTransaction.
func (h *Harness) submitSigned(ctx context.Context, t *testing.T, kp *crypto.Keypair, payload *pb.TransactionPayload) *pb.SubmitTransactionResponse {
	t.Helper()
	raw, err := proto.Marshal(payload)
	mustNil(t, err, "marshal payload")
	sig := crypto.Sign(kp.PrivateKey, raw)

	resp, err := h.LedgerClient.SubmitTransaction(ctx, &pb.SubmitTransactionRequest{
		RawPayload:   raw,
		Signature:    sig,
		SenderPubKey: kp.PublicKey,
	})
	mustNil(t, err, "submit transaction RPC")
	return resp
}

// ── internal helpers ──────────────────────────────────────────────────────────

func dialBufconn(lis *bufconn.Listener) (*grpc.ClientConn, error) {
	return grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
}

func parseRedisAddr(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse redis URL %q: %w", rawURL, err)
	}
	if u.Host != "" {
		return u.Host, nil
	}
	return fmt.Sprintf("%s:%s", u.Hostname(), u.Port()), nil
}

// chain returns a cleanup function that calls prev then next.
func chain(prev, next func()) func() {
	return func() { prev(); next() }
}

func mustNil(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}
