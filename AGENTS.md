# AGENTS.md — RGB Mini-Game Codebase Guide

## Architecture

Three-binary system (`go 1.27.0`) connected via gRPC (single-node blockchain — no P2P networking or multi-node
consensus).
The codebase follows **Hexagonal Architecture** (Ports & Adapters):

```
        ┌─────────────────────────────────────────────┐
        │                 Domain Ring                 │
        │  internal/domain/{types,enum,engine,        │
        │                   chain,merkle,mission}     │
        └──────────────────────┬──────────────────────┘
                               │ used by
        ┌──────────────────────▼──────────────────────┐
        │               Application Ring              │
        │         internal/app/{port,service}         │
        │  port/in  ← use-case interfaces             │
        │  port/out → infrastructure ports            │
        └────────────┬───────────────────┬────────────┘
                     │                   │
       ┌─────────────▼───┐         ┌─────▼──────────────┐
       │ Driving Adapters│         │  Driven Adapters   │
       │ (Left / Primary)│         │ (Right / Secondary)│
       │ adapter/driving │         │ adapter/driven     │
       │  grpc/ledger    │         │  postgres/         │
       │  grpc/game      │         │  redis/            │
       └─────────────────┘         │  authority/        │
                                   │  ledger/           │
                                   └────────────────────┘
```

| Binary          | Entry                | Port                                      | Status      |
|-----------------|----------------------|-------------------------------------------|-------------|
| **Ledger**      | `cmd/ledger/main.go` | `LEDGER_GRPC_PORT` (default `50051`)      | Implemented |
| **Game Server** | `cmd/server/main.go` | `GAME_SERVER_GRPC_PORT` (default `50052`) | Implemented |
| **Player CLI**  | `cmd/player/main.go` | —                                         | Implemented |

Player and Game Server both call Ledger gRPC. Game Server issues `MINT` txs; players issue `TRANSFER` txs. Ledger
persists everything in Postgres via GORM.

The Ledger also runs a **BlockSealer** background goroutine that periodically mines pending transactions into blocks, and validates the full chain on startup.

---

## Directory Layout

```
cmd/
  ledger/                        # Ledger binary (main.go + wire.go using pkg/di)
  server/                        # Game Server binary (main.go + wire.go using pkg/di)
  player/                        # Interactive Player CLI binary
  keygen/                        # One-shot ed25519 authority keypair generator → .key/
  migrate/                       # Dev-only GORM AutoMigrate runner (APP_ENV=dev)

config/                          # Env-based config structs (godotenv in APP_ENV=dev)
  config.go                      # InitLedgerConfig, InitGameServerFullConfig, InitPlayerFullConfig
  authority.go                   # AuthorityConfig
  block_sealer.go                # BlockSealerConfig
  database.go                    # DatabaseConfig
  game_server.go                 # GameServerConfig + GameConfig
  player.go                      # PlayerConfig
  redis.go                       # RedisConfig
  server.go                      # ServerConfig

api/proto/v1/                    # Protobuf sources (ledger.proto, game.proto)

pkg/
  pb/                            # Generated protobuf & gRPC stubs
  di/                            # Lightweight reflection-based DI container + LIFO Close + Graph
  grpcserver/                    # Graceful gRPC server runner (SIGINT/SIGTERM + ctx cancel)
  crypto/                        # ed25519 keygen/load, signing, SHA-256 player ID derivation
  logger/                        # slog-based structured logger
  utils/                         # GetEnv, Contains helpers

test/e2e/                        # Build-tagged (`e2e`) integration suite using testcontainers + bufconn
  harness/harness.go             # Starts Postgres + Redis containers & in-process gRPC servers
  setup_test.go
  ledger_test.go
  game_test.go
  flow_test.go

internal/
  domain/                        # Pure domain ring — zero imports from internal/app or internal/adapter
    types/                       # PlayerRecord, PlayerState, TransactionRecord,
    │                            # MissionRecord, Mission, RGB, Block, AuthorityRecord,
    │                            # ChallengeRecord, ChallengeStatus
    enum/color.go                # Color (Red=0, Green=1, Blue=2)
    engine/engine.go             # Engine domain logic (clamped calc, PlayerTransactions, PlayerCompleteMission)
    chain/validator.go           # ValidateChain — checks hash, PrevHash, Merkle root, PoW per block
    merkle/merkle.go             # BuildMerkleRoot — deterministic sorted SHA-256 Merkle root over tx hashes
    mission/templates.go         # Difficulty enum (Easy/Medium/Hard), CooldownMultiplier, 17-entry Pool

  app/                           # Application ring
    port/
      in/                        # Primary/driving ports (use-case contracts)
        ledger.go                # LedgerUseCase: GetBalance, SubmitTransaction, ValidateChain,
        │                        # RegisterAuthority, GetTransactionHistory, GetChainStats,
        │                        # GetBlock, ListBlocks, GetLeaderboard
        game.go                  # GameUseCase: RequestMission, CompleteMission,
        │                        # ProposeChallenge, RespondChallenge, GetChallenge
      out/                       # Secondary/driven ports (infrastructure contracts)
        transactor.go            # Transactor — InTransaction(ctx, fn)
        repositories.go          # PlayerRepository, TransactionRepository, LeaderboardEntry
        mission_repository.go    # MissionRepository
        challenge_repository.go  # ChallengeRepository — Create, FindByID, UpdateStatus
        block_repository.go      # BlockRepository — LatestBlock, CreateBlock, SealTransactions,
        │                        # PendingTransactions, AllBlocks, TransactionHashesByBlock,
        │                        # CountBlocks, CountPending, GetByHeight, ListBlocks
        authority_repository.go  # AuthorityRepository — Register, Exists
        authority.go             # PublicAuthority, FullAuthority
        engine.go                # GameEngine interface
        ledger_client.go         # LedgerClient — GetAuthorityNonce, SubmitMint, SubmitSignedTransfer
    service/
      ledger.go                  # LedgerService — implements port/in.LedgerUseCase
      game.go                    # GameService   — implements port/in.GameUseCase
      mission.go                 # missionService (unexported, used only by GameService)
      block_sealer.go            # BlockSealer — background PoW miner + block sealing loop

  adapter/
    driving/                     # Left-side adapters: receive transport calls, invoke use cases
      grpc/
        ledger/handler.go        # pb.LedgerServiceServer; owns crypto verification for txs &
        │                        # RegisterAuthority; MINT check uses AuthorityRepository.Exists
        game/handler.go          # pb.GameServiceServer; translates pb ↔ domain types;
        │                        # verifies proposer/opponent signatures at the transport boundary
    driven/                      # Right-side adapters: implement out/ ports
      postgres/
        postgres.go              # gorm.Open wrapper
        migration/               # Flyway SQL migrations (V1__players .. V4__authorities)
        repositories/
          player_repository.go        # out.PlayerRepository via GORM
          transaction_repository.go   # out.TransactionRepository via GORM
          block_repository.go         # out.BlockRepository via GORM (blocks + pending/sealed txs)
          authority_repository.go     # out.AuthorityRepository via GORM (authorities table)
          transactor.go               # out.Transactor via gorm.DB.Transaction + context key
          migrate.go                  # AutoMigrate — players, transactions, blocks, authorities
      redis/
        redis.go                 # Redis client init (with Ping check)
        mission_repository.go    # out.MissionRepository via Redis TTL keys
        challenge_repository.go  # out.ChallengeRepository via Redis JSON + TTL
      authority/
        authority.go             # out.PublicAuthority + out.FullAuthority + Load()
      ledger/
        client.go                # out.LedgerClient — wraps pb.LedgerServiceClient;
                                 # handles proto.Marshal + auth.Sign internally
```

---

## Key Developer Commands

```sh
docker compose up -d             # start Postgres (17.0) + Redis (8.6.2-alpine)
make migrate                     # run GORM AutoMigrate (dev only; creates players, transactions, blocks, authorities)
make migrate-flyway              # run Flyway SQL migrations (V1–V4) via docker compose --profile migrate
make keygen                      # generate authority ed25519 keypair → .key/
make proto-v1                    # regenerate pkg/pb/ from api/proto/v1/*.proto
make run-ledger                  # run Ledger (go run ./cmd/ledger; requires .env in dev)
make run-server                  # run Game Server (go run ./cmd/server; requires .env in dev)
make run-player                  # run Player CLI (go run ./cmd/player; optional <alias> arg)
make test-unit                   # run unit tests (go test ./internal/... ./pkg/...)
make test-e2e                    # run E2E tests (go test -v -tags e2e -timeout 120s ./test/e2e/...)
```

> **Build gotcha:** `cmd/ledger` and `cmd/server` each contain two files (`main.go` and `wire.go`). Always pass the
> package path (`./cmd/ledger`, `./cmd/server`) rather than `cmd/*/main.go`:
> ```sh
> go build -o bin/ledger ./cmd/ledger
> go build -o bin/server ./cmd/server
> go build -o bin/player ./cmd/player
> ```
> (Note: `Makefile` targets `build-ledger` / `build-server` currently pass `cmd/*/main.go`, which fails with
> `undefined: wire`.)

### Environment Variables

`.env` is auto-loaded via `godotenv.Load()` only when `APP_ENV == "dev"` (default `"dev"`). `cmd/migrate` also refuses
to run GORM `AutoMigrate` when `APP_ENV != "dev"` (use `make migrate-flyway` outside dev).

- **Ledger:** `LEDGER_GRPC_PORT` (default `50051`), `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_HOST`,
  `POSTGRES_PORT`, `POSTGRES_NAME`, `POSTGRES_SSL_MODE`, and either `AUTHORITY_PUB_KEY_PATH` (JSON keypair path) or
  `AUTHORITY_PUB_KEY` (hex string fallback).
- **Block Sealer:** `BLOCK_INTERVAL_SECONDS` (default `10`), `BLOCK_DIFFICULTY` (default `2`, range `0`–`64` leading
  zero hex nibbles).
- **Game Server:** `GAME_SERVER_GRPC_PORT` (default `50052`), `LEDGER_ADDR` (default `localhost:50051`),
  `AUTHORITY_KEY_PATH` (default `.key/id_ed25519`, full keypair for signing MINT txs), `MISSION_COOLDOWN_SECONDS`
  (default `300`), `REDIS_ADDR` (default `localhost:6379`), `REDIS_PASSWORD` (default `""`), `REDIS_DB` (default `0`).
- **Player CLI:** `PLAYER_KEY_PATH` (default `.key/player_ed25519`), `LEDGER_ADDR` (default `localhost:50051`),
  `SERVER_ADDR` (default `localhost:50052`). Passing an alias (`go run ./cmd/player alice`) overrides the key path to
  `.key/player_alice_ed25519`. Keypairs are generated on first run.

---

## Hexagonal Architecture Rules

1. **Domain ring** (`internal/domain/`) — zero imports from `internal/app` or `internal/adapter`. Only stdlib.
2. **Application ring** (`internal/app/`) — intended to import `internal/domain` and stdlib only, never adapters or
   `pkg/pb`.

- *Current exception:* `internal/app/service/game.go` imports `pkg/pb`, `google.golang.org/protobuf/proto`, and
  `pkg/crypto` in `verifyTransferPayload` to validate challenge `TransactionPayload` bytes. Avoid spreading `pkg/pb`
  imports into other services; new transport parsing belongs in `adapter/driving/`.

3. **Driving adapters** (`internal/adapter/driving/`) — import `internal/app/port/in`, `pkg/pb`, and crypto libs. They
   own transport concerns (proto marshal/unmarshal, signature verification, authority checks).
4. **Driven adapters** (`internal/adapter/driven/`) — implement `internal/app/port/out` interfaces. Import `internal/domain/types`, infrastructure libs (gorm, redis, protobuf), and `pkg/pb` where needed.
5. **`cmd/`** — the only place that imports from all layers and wires everything together. No business logic.

---

## Identity & Signing Model

- Player/authority ID = `hex(sha256(ed25519_pubkey))` — use `pkg/crypto.PubKeyToPlayerID()`
- Keypair stored as JSON (`{private_key, public_key}` hex-encoded); use `pkg/crypto.LoadOrGenerateKey(path)`
- `make keygen` writes two files: `.key/id_ed25519` (JSON keypair) and `.key/id_ed25519.pub.hex` (bare hex pubkey for use in `AUTHORITY_PUB_KEY`)
- Transaction wire format: marshal `pb.TransactionPayload` → sign bytes → send `{raw_payload, signature, sender_pub_key}` in `SubmitTransactionRequest`
- **Crypto verification lives in the Ledger gRPC driving adapter** (`adapter/driving/grpc/ledger/handler.go`): verify signature, verify `sha256(sender_pub_key) == payload.sender_id`, check MINT authority.
- MINT authority check: `authorityRepo.Exists(ctx, pubKey)` — any key registered via `RegisterAuthority` is allowed
  (open authority set). On Ledger startup the genesis key from config is seeded into the `authorities` table via
  `authorityRepo.Register`.
- **`RegisterAuthority` RPC** — a new node proves ownership of its keypair by signing its own raw public key bytes
  (`ed25519.Verify(pubKey, pubKey, signature)`). The Ledger stores the key in the `authorities` table; from that point
  it can issue MINT transactions.

---

## Type Conventions

- `domain/types.RGB{R, G, B int8}` — **signed delta** used in mission rewards and engine logic (Ledger caps MINT amounts
  at `127` per channel and TRANSFER amounts at `255` per channel)
- `domain/types.PlayerRecord{ID, Red, Green, Blue uint32, Nonce uint64}` — **unsigned storage** in core domain /
  Postgres
- `domain/types.PlayerState{R, G, B uint8, Nonce uint64}` — engine working state
- `domain/types.Block{Height, Hash, PrevHash, MerkleRoot, Timestamp, Nonce, Difficulty}` — a sealed block; `Nonce` is the PoW nonce, `Difficulty` is the number of leading zero hex nibbles required
- `domain/types.AuthorityRecord{ID, PubKeyHex, RegisteredAt}` — a registered MINT authority
- `domain/types.ChallengeRecord{ID, ProposerID, OpponentID, OfferRed/Green/Blue, WantRed/Green/Blue, Status, CreatedAt, ExpiresAt, RawPayload, Signature, SenderPubKey}` — a peer-to-peer token-swap challenge stored in Redis
- `domain/types.ChallengeStatus` — `ChallengePending=0`, `ChallengeAccepted=1`, `ChallengeDeclined=2`, `ChallengeExpired=3`, `ChallengeCompleted=4`
- `domain/engine.Engine.calculation(color uint8, val int8)` is unexported; clamps result to `[0, 255]`
- `app/port/in.TxType`: `TxTypeTransfer = 0`, `TxTypeMint = 1` — mirrors `pb.TransactionPayload_TxType`
- `domain/enum.Color{Red=0, Green=1, Blue=2}` — matches `pb.RewardColor` ordinals in `game.proto`

---

## Transactor Pattern

`app/port/out.Transactor` abstracts DB transaction coordination:

```go
type Transactor interface {
    InTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
```

`adapter/driven/postgres/repositories.GormTransactor` stores the active `*gorm.DB` transaction in the context using a
private key. All repository methods call `dbFromContext(ctx, r.db)` to transparently use the active transaction or fall
back to the plain DB handle. **`*gorm.DB` never appears in any port interface.**

`PlayerRepository.FindOrCreate` applies `SELECT FOR UPDATE` — always call it inside a `Transactor.InTransaction` block.

Available repository methods (all accept `context.Context`):

- `PlayerRepository`: `Find(ctx, playerID)`, `FindOrCreate(ctx, playerID)`, `UpdateBalance(ctx, *types.PlayerRecord)`,
  `Count(ctx)`, `GetLeaderboard(ctx, topN, sortBy)`
- `TransactionRepository`: `Create(ctx, *types.TransactionRecord)`, `FindByPlayer(ctx, playerID, limit, offset)`,
  `CountAll(ctx)`
- `BlockRepository`: `LatestBlock(ctx)`, `CreateBlock(ctx, *types.Block)`,
  `SealTransactions(ctx, blockHeight, txHashes)`, `PendingTransactions(ctx)`, `AllBlocks(ctx)`,
  `TransactionHashesByBlock(ctx, blockHeight)`, `CountBlocks(ctx)`, `CountPending(ctx)`, `GetByHeight(ctx, height)`,
  `ListBlocks(ctx, page, pageSize)`
- `AuthorityRepository`: `Register(ctx, *types.AuthorityRecord)`, `Exists(ctx, pubKey []byte)`

---

## DI / Wiring Pattern

`cmd/ledger/wire.go` and `cmd/server/wire.go` use `pkg/di`, a lightweight reflection-based DI container supporting:

- `Provide(fn)` / `ProvideAs(fn, (*Iface)(nil))` / `Alias((*Iface)(nil), (**Concrete)(nil))`
- Named bindings (`ProvideNamed`, `ProvideAsNamed`, `ResolveNamed`) and lifetimes (`Singleton`, `Scoped`, `Transient`)
- Constructor shapes: `func(...) T`, `func(...) (T, error)`, `func(...) (T, func(), error)` (auto-registers cleanup)
- Cycle detection, `c.Close()` for LIFO cleanup, `c.Scope()` for child scopes, and `c.Graph()` for Graphviz DOT output

Both `cmd/ledger/main.go` and `cmd/server/main.go` serve gRPC via `pkg/grpcserver.Run(ctx, s, port)` with graceful
`SIGINT`/`SIGTERM` shutdown.

**Ledger** (`cmd/ledger/wire.go`):
```
config (*DatabaseConfig, *AuthorityConfig)
  → drivenpostgres.Init → *gorm.DB
  → authority.Load
  → repositories.{Player,Transaction,Block,Authority,Transactor}
  → engine.New → service.NewLedgerService + service.NewBlockSealer
  → authorityRepo.Register(genesisRecord)        ← seed genesis authority on boot
  → grpcledger.New(ledgerSvc, authorityRepo) → pb.RegisterLedgerServiceServer
  → startBackground(ctx):
      go blockSealer.Start(ctx)                  ← PoW mining loop
      ledgerSvc.ValidateChain(ctx)               ← logs warning if integrity check fails
```

**Game Server** (`cmd/server/wire.go`):
```
*config.RedisConfig → drivenredis.Init (+ Close cleanup)
gsCfg.LedgerAddr    → grpc.NewClient (+ Close cleanup) → pb.NewLedgerServiceClient → drivenledger.New
gsCfg.AuthorityKeyPath → crypto.LoadOrGenerateKey → authority.NewFullAuthority
drivenredis.{NewMissionRepository, NewChallengeRepository}
service.NewGameService(missionRepo, auth, ledgerClient, challengeRepo, cfg)
grpcgame.New(gameSvc) → pb.RegisterGameServiceServer
```

**Player CLI** (`cmd/player/main.go`):
`config.InitPlayerFullConfig → crypto.LoadOrGenerateKey → grpc.NewClient(LedgerAddr) → grpc.NewClient(ServerAddr) → interactive menu loop (options 1–12)`.

---

## Authority Adapter

`internal/adapter/driven/authority` provides a concrete `Authority` struct:
- `authority.NewPublicAuthority(pubKey)` — implements `out.PublicAuthority` (Ledger: verify-only)
- `authority.NewFullAuthority(kp *crypto.Keypair)` — implements `out.FullAuthority` (Game Server: signing)
- `authority.Load(cfg *config.AuthorityConfig)` — convenience loader for the Ledger (`AUTHORITY_PUB_KEY_PATH` takes
  precedence over `AUTHORITY_PUB_KEY`)

---

## LedgerClient Driven Adapter

`internal/adapter/driven/ledger/client.go` implements `out.LedgerClient`. It encapsulates proto marshaling and MINT
signing:
- `GetAuthorityNonce(ctx, authorityID)` → calls `pb.GetBalance`, returns `NextNonce`
- `SubmitMint(ctx, auth, senderID, receiverID, amtR, amtG, amtB, nonce)` → builds `pb.TransactionPayload`, calls `auth.Sign(rawPayload)`, submits via `pb.SubmitTransaction`
- `SubmitSignedTransfer(ctx, rawPayload, signature, senderPubKey)` → forwards a pre-signed TRANSFER payload to the
  Ledger (used for Challenges); returns `out.TransferResult{Success, TxHash, NewBalance, ErrorMessage}`

---

## Redis Mission & Challenge Storage

### Missions (`adapter/driven/redis/mission_repository.go`)

- `mission:{id}` — JSON `types.MissionRecord`, TTL = `max(2 × effectiveCooldown, 5m)`
  (`minMissionTTL = 5 * time.Minute`); rewritten with `Completed: true` and 5m TTL on completion
- `player:active:{playerID}` — active mission ID pointer, same TTL; deleted on completion
- `player:cooldown:{playerID}` — presence-only cooldown sentinel `"1"`, set on completion with TTL = `effectiveCooldown`

`effectiveCooldown` = `MISSION_COOLDOWN_SECONDS × Difficulty.CooldownMultiplier()`.
`out.MissionRepository` methods: `Create`, `FindByID`, `FindActiveByPlayer`, `GetCooldownRemaining`, `Complete`.

### Challenges (`adapter/driven/redis/challenge_repository.go`)

- `challenge:{id}` — JSON `types.ChallengeRecord`, TTL = `GameServiceConfig.ChallengeTTL` (defaults to
  `10 * time.Minute` in `NewGameService` when zero)

`out.ChallengeRepository` methods: `Create(ctx, record, ttl)`, `FindByID(ctx, id)`, `UpdateStatus(ctx, id, status)`
(preserves remaining TTL, falling back to 5m if expired/missing).

---

## Variable Mission Difficulty

`internal/domain/mission/templates.go` defines:
- `Difficulty` type: `Easy=0`, `Medium=1`, `Hard=2`
- `Difficulty.CooldownMultiplier()` — Easy×1, Medium×2, Hard×4
- `Pool []Template` — 17 pre-defined templates: Easy (+1 single channel), Medium (+3 single-channel or +2/+1 multi),
  Hard (+5 single-channel or +3/+2 or +2/+2/+1 multi-combos)
- `Template{Difficulty, Reward types.RGB, PrimaryColor int32}` — sampled uniformly at `RequestMission` time

`MissionResponse` carries `difficulty`, `reward_red`, `reward_green`, `reward_blue` in addition to the backward-compat
`reward_color`.

---

## Player-to-Player Challenge System

Challenges let two players agree to swap tokens without a trusted intermediary. The proposer pre-signs their TRANSFER leg; the opponent signs theirs only upon acceptance.

**Flow:**

1. Proposer calls `ProposeChallenge` — pre-signs `TRANSFER(proposer→opponent)` and sends
   `{raw_payload, signature, sender_pub_key}`. Driving adapter and `GameService` verify signature, identity, and terms,
   then store a `ChallengeRecord` in Redis with `ChallengePending` status.
2. Opponent calls `RespondChallenge(accept=true)` — pre-signs `TRANSFER(opponent→proposer)`. Game Server verifies the
   opponent's signed payload, then submits both transfers to the Ledger sequentially (proposer leg first, opponent leg
   second). If the second leg fails after the first succeeded, the challenge status is set to `ChallengeDeclined`
   (best-effort); the first leg is not automatically reversed on the Ledger.
3. Either party can call `GetChallenge(challenge_id)` to check status.

**Player CLI options** (`cmd/player/main.go`):
1 = Get Balance, 2 = Request Mission, 3 = Complete Mission, 4 = Transfer, 5 = Transaction History, 6 = Leaderboard, 7 =
Chain Stats, 8 = Block Explorer, 9 = Propose Challenge, 10 = Respond to Challenge, 11 = View Challenge, 12 = Quit.

---

## Proto & Generated Code

- Proto sources: `api/proto/v1/ledger.proto`, `api/proto/v1/game.proto` (`game.proto` imports `ledger.proto` for
  `BalanceResponse`)
- Both use `option go_package = "./pkg/pb"` → all generated types land in `pkg/pb`
- Regenerate after any `.proto` change: `PATH="$PATH:$(go env GOPATH)/bin" make proto-v1`
- `LedgerService` RPCs:
  - `GetBalance(player_id) → BalanceResponse`
  - `SubmitTransaction(raw_payload, signature, sender_pub_key) → SubmitTransactionResponse`
  - `RegisterAuthority(pub_key, signature) → RegisterAuthorityResponse`
  - `GetTransactionHistory(player_id, limit, offset) → TransactionHistoryResponse`
  - `GetChainStats() → ChainStatsResponse`
  - `GetBlock(height) → BlockResponse`
  - `ListBlocks(page, page_size) → ListBlocksResponse`
  - `GetLeaderboard(top_n, sort_by) → LeaderboardResponse`
- `GameService` RPCs:
  - `RequestMission(player_id) → MissionResponse`
  - `CompleteMission(mission_id, player_id) → CompleteMissionResponse`
  - `ProposeChallenge(...) → ChallengeResponse`
  - `RespondChallenge(...) → RespondChallengeResponse`
  - `GetChallenge(challenge_id) → ChallengeResponse`

> **Note:** `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc` must be on `PATH`.

---

## Logging & Config

- Always use `pkg/logger` (slog-based): `logger.Info/Infof(...)`, `logger.Warn/Warnf(...)`, `logger.Error/Errorf(...)`,
  `logger.Fatal/Fatalf(...)` — never `fmt.Print` (outside CLI UI in `cmd/player` / `cmd/keygen`) or stdlib `log`
- Call `logger.Init()` at the top of every `main()`
- Env vars via `utils.GetEnv("KEY", "optional_default")` — no direct `os.Getenv`
- New config structs belong in `config/` following the pattern in `config/server.go`
- **`POSTGRES_URL` is ignored by `postgres.Init`** — it always reconstructs the DSN from individual `POSTGRES_USER/PASSWORD/HOST/PORT/NAME/SSL_MODE` vars

---

## Blockchain Features

### Chain Structure

Each `types.Block` links to the previous via `PrevHash`, commits transactions via a Merkle root, and requires a PoW nonce:

```
Block {
    Height     uint64   // sequential, starts at 0 (genesis)
    Hash       string   // sha256(height + prevHash + merkleRoot + timestamp + nonce)
    PrevHash   string   // hash of block Height-1; genesis uses 64 zero hex chars
    MerkleRoot string   // merkle.BuildMerkleRoot(sorted tx hashes)
    Timestamp  int64
    Nonce      uint64   // PoW nonce — incremented until hash satisfies difficulty
    Difficulty uint8    // leading zero hex nibbles required in hash
}
```

### Proof-of-Work

`service.BlockSealer` calls `mineBlock(b)` which increments `b.Nonce` until `b.Hash` starts with `b.Difficulty` `'0'`
characters. Default difficulty is `2` (e.g. hash starts with `00…`).

### Chain Integrity Validation

`domain/chain.ValidateChain(blocks, txsByBlock)` is a pure function checking:
1. Each block's stored hash matches recomputed `sha256(…)`
2. Each block's `PrevHash` matches the previous block's `Hash`
3. Each block's `MerkleRoot` matches `merkle.BuildMerkleRoot(txHashes)`
4. Each block's hash satisfies its `Difficulty` PoW target

`LedgerService.ValidateChain(ctx)` loads all blocks + tx hashes from Postgres and delegates to `chain.ValidateChain`.
Called on Ledger startup (logs a warning on failure).

### Authority Registry

MINT transactions are open to any registered authority:
- **Table:** `authorities` — `(id, pub_key_hex, registered_at)`
- **Register:** `RegisterAuthority` RPC with `(pub_key, signature_of_pub_key)`
- **Check on MINT:** `authorityRepo.Exists(ctx, pubKey)` in `adapter/driving/grpc/ledger/handler.go`
- **Genesis seed:** config key (`AUTHORITY_PUB_KEY_PATH` / `AUTHORITY_PUB_KEY`) is inserted idempotently on Ledger
  startup

---

## Testing

- **Unit tests** (`make test-unit` → `go test ./internal/... ./pkg/...`): cover
  `internal/domain/{chain,engine,enum,merkle,mission}` and `pkg/{crypto,di,utils}`.
- **E2E tests** (`make test-e2e` → `go test -v -tags e2e -timeout 120s ./test/e2e/...`): require Docker running;
  `test/e2e/harness/harness.go` spins up Postgres `17.0` + Redis `8.6.2-alpine` via `testcontainers-go` and runs live
  Ledger and Game gRPC servers over `bufconn`.

---

## Adding a New Feature (Checklist)

1. **Domain types/logic** → `internal/domain/types/` or `internal/domain/engine/`
2. **New port** → `internal/app/port/out/` (driven) or `internal/app/port/in/` (driving)
3. **Application service change** → `internal/app/service/` — avoid `pb`/`gorm` imports
4. **Driving adapter** (new RPC) → `internal/adapter/driving/grpc/{service}/handler.go` — translate pb ↔ domain, verify
   crypto, then call use case
5. **Driven adapter** (new infra) → `internal/adapter/driven/{tech}/` — implement out port
6. **Wire** → register providers/aliases in `cmd/ledger/wire.go` or `cmd/server/wire.go` (and
   `test/e2e/harness/harness.go` if dependencies changed)
