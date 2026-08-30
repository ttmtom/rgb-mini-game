# AGENTS.md — RGB Mini-Game Codebase Guide

## Architecture

Three-binary system connected via gRPC (see `plan-distributedRgbMiniGame.prompt.md` for the authoritative diagram).
The codebase follows **Hexagonal Architecture** (Ports & Adapters):

```
        ┌─────────────────────────────────────┐
        │            Domain Ring               │
        │  internal/domain/{types,enum,engine, │
        │                   chain,merkle}      │
        └───────────────┬─────────────────────┘
                        │ used by
        ┌───────────────▼─────────────────────┐
        │          Application Ring            │
        │    internal/app/{port,service}       │
        │  port/in  ← use-case interfaces      │
        │  port/out → infrastructure ports     │
        └────────────┬────────────┬────────────┘
                     │            │
       ┌─────────────▼──┐   ┌─────▼──────────────┐
       │ Driving Adapters│   │  Driven Adapters    │
       │ (Left / Primary)│   │ (Right / Secondary) │
       │ adapter/driving │   │ adapter/driven      │
       │  grpc/ledger    │   │  postgres/          │
       │  grpc/game      │   │  redis/             │
       └─────────────────┘   │  authority/         │
                             │  ledger/            │
                             └─────────────────────┘
```

| Binary          | Entry                | Port                                      | Status              |
|-----------------|----------------------|-------------------------------------------|---------------------|
| **Ledger**      | `cmd/ledger/main.go` | `LEDGER_GRPC_PORT` (default `50051`)      | Implemented         |
| **Game Server** | `cmd/server/main.go` | `GAME_SERVER_GRPC_PORT` (default `50052`) | Implemented         |
| **Player CLI**  | `cmd/player/main.go` | —                                         | Implemented         |

Player and Game Server both call Ledger gRPC. Game Server issues MINT txs; players issue TRANSFER txs. Ledger persists everything in Postgres via GORM.

The Ledger also runs a **BlockSealer** background goroutine that periodically mines pending transactions into blocks, and validates the full chain on startup.

---

## Directory Layout

```
internal/
  domain/                        # Pure domain ring — no external imports
    types/                       # PlayerRecord, PlayerState, TransactionRecord,
    │                            # MissionRecord, Mission, RGB, Block, AuthorityRecord
    enum/color.go                # Color (Red=0, Green=1, Blue=2)
    engine/engine.go             # GameEngine domain logic (clamped calc, PlayerTransactions, PlayerCompleteMission)
    chain/validator.go           # ValidateChain — checks hash linkage, PrevHash, Merkle root, PoW per block
    merkle/merkle.go             # BuildMerkleRoot — deterministic Merkle root over tx hashes

  app/                           # Application ring
    port/
      in/                        # Primary/driving ports (use-case contracts)
        ledger.go                # LedgerUseCase: GetBalance, SubmitTransaction, ValidateChain, RegisterAuthority
        game.go                  # GameUseCase, RequestMissionResult, CompleteMissionResult
      out/                       # Secondary/driven ports (infrastructure contracts)
        transactor.go            # Transactor — InTransaction(ctx, fn)
        repositories.go          # PlayerRepository, TransactionRepository (ctx-based, no *gorm.DB)
        mission_repository.go    # MissionRepository
        block_repository.go      # BlockRepository — LatestBlock, CreateBlock, SealTransactions,
        │                        # PendingTransactions, AllBlocks, TransactionHashesByBlock
        authority_repository.go  # AuthorityRepository — Register, Exists
        authority.go             # PublicAuthority, FullAuthority
        engine.go                # GameEngine interface
        ledger_client.go         # LedgerClient — GetAuthorityNonce, SubmitMint
    service/
      ledger.go                  # LedgerService — implements port/in.LedgerUseCase
      game.go                    # GameService   — implements port/in.GameUseCase
      mission.go                 # missionService (unexported, used only by GameService)
      block_sealer.go            # BlockSealer — background PoW miner + block sealing loop

  adapter/
    driving/                     # Left-side adapters: receive transport calls, invoke use cases
      grpc/
        ledger/handler.go        # pb.LedgerServiceServer; owns ALL crypto verification;
        │                        # MINT check uses AuthorityRepository.Exists (open authority set)
        game/handler.go          # pb.GameServiceServer; translates pb ↔ domain types
    driven/                      # Right-side adapters: implement out/ ports
      postgres/
        postgres.go              # gorm.Open wrapper
        repositories/
          player_repository.go        # out.PlayerRepository via GORM
          transaction_repository.go   # out.TransactionRepository via GORM
          block_repository.go         # out.BlockRepository via GORM (blocks + pending/sealed txs)
          authority_repository.go     # out.AuthorityRepository via GORM (authorities table)
          transactor.go               # out.Transactor via gorm.DB.Transaction + context key
          migrate.go                  # AutoMigrate — players, transactions, blocks, authorities
      redis/
        redis.go                 # Redis client init
        mission_repository.go    # out.MissionRepository via Redis TTL keys
      authority/
        authority.go             # out.PublicAuthority + out.FullAuthority + Load()
      ledger/
        client.go                # out.LedgerClient — wraps pb.LedgerServiceClient;
                                 # handles proto.Marshal + auth.Sign internally
```

---

## Key Developer Commands

```sh
docker compose up -d             # start Postgres + Redis
make migrate                    # run GORM AutoMigrate (players + transactions tables)
make keygen                     # generate authority ed25519 keypair → .key/
make proto-v1                   # regenerate pkg/pb/ from api/proto/v1/*.proto
make build-ledger               # compile → bin/ledger
make build-server               # compile → bin/server
make build-player               # compile → bin/player
make build-all                  # compile all three binaries (ledger + server + player)
go run cmd/ledger/main.go       # run ledger directly (requires .env)
go run cmd/server/main.go       # run game server directly (requires .env)
go run cmd/player/main.go       # run player CLI directly (requires .env)
```

`.env` is auto-loaded in `dev` mode (`APP_ENV != "dev"` skips it). Required vars: `POSTGRES_*`, and either `AUTHORITY_PUB_KEY_PATH` (JSON keypair file path) or `AUTHORITY_PUB_KEY` (hex string).

Game Server env vars: `GAME_SERVER_GRPC_PORT` (default `50052`), `LEDGER_ADDR` (default `localhost:50051`), `AUTHORITY_KEY_PATH` (default `.key/id_ed25519`, full keypair for signing MINT txs), `MISSION_COOLDOWN_SECONDS` (default `300`). Redis vars: `REDIS_ADDR` (default `localhost:6379`), `REDIS_PASSWORD` (default `""`), `REDIS_DB` (default `0`).

Player CLI env vars: `PLAYER_KEY_PATH` (default `.key/player_ed25519`), `LEDGER_ADDR` (default `localhost:50051`), `SERVER_ADDR` (default `localhost:50052`). The CLI generates a keypair at `PLAYER_KEY_PATH` on first run.

Block Sealer env vars: `BLOCK_INTERVAL_SECONDS` (default `10`), `BLOCK_DIFFICULTY` (default `2` — number of leading zero hex nibbles required in each block hash for PoW).

---

## Hexagonal Architecture Rules

1. **Domain ring** (`internal/domain/`) — zero imports from `internal/app` or `internal/adapter`. Only stdlib.
2. **Application ring** (`internal/app/`) — imports `internal/domain` and stdlib only. Never imports adapters or `pkg/pb`.
3. **Driving adapters** (`internal/adapter/driving/`) — import `internal/app/port/in`, `pkg/pb`, and crypto libs. They own ALL transport-layer concerns (proto marshal/unmarshal, signature verification, authority checks).
4. **Driven adapters** (`internal/adapter/driven/`) — implement `internal/app/port/out` interfaces. Import `internal/domain/types`, infrastructure libs (gorm, redis, protobuf), and `pkg/pb` where needed.
5. **`cmd/`** — the only place that imports from all layers and wires everything together. No business logic.

---

## Identity & Signing Model

- Player/authority ID = `hex(sha256(ed25519_pubkey))` — use `pkg/crypto.PubKeyToPlayerID()`
- Keypair stored as JSON with hex-encoded keys; use `pkg/crypto.LoadOrGenerateKey(path)`
- `make keygen` writes two files: `.key/id_ed25519` (JSON keypair) and `.key/id_ed25519.pub.hex` (bare hex pubkey for use in `AUTHORITY_PUB_KEY`)
- Transaction wire format: marshal `pb.TransactionPayload` → sign bytes → send `{raw_payload, signature, sender_pub_key}` in `SubmitTransactionRequest`
- **Crypto verification lives in the Ledger gRPC driving adapter** (`adapter/driving/grpc/ledger/handler.go`): verify signature, verify `sha256(sender_pub_key) == payload.sender_id`, check MINT authority.
- MINT authority check: `authorityRepo.Exists(pubKey)` — any key registered via `RegisterAuthority` is allowed (open authority set). On Ledger startup the genesis key from config is always seeded into the `authorities` table.
- **`RegisterAuthority` RPC** — a new node proves ownership of its keypair by signing its own public key bytes (self-signed proof). The Ledger stores the key in the `authorities` table; from that point it can issue MINT transactions.

---

## Type Conventions

- `domain/types.RGB{R, G, B int8}` — **signed delta** used in mission rewards and engine logic
- `domain/types.PlayerRecord{Red, Green, Blue uint32}` — **unsigned storage** in core domain / Postgres
- `domain/types.PlayerState{R, G, B uint8, Nonce uint64}` — engine working state
- `domain/types.Block{Height, Hash, PrevHash, MerkleRoot, Timestamp, Nonce, Difficulty}` — a sealed block; `Nonce` is the PoW nonce, `Difficulty` is the number of leading zero hex nibbles required
- `domain/types.AuthorityRecord{ID, PubKeyHex, RegisteredAt}` — a registered MINT authority
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

The `adapter/driven/postgres/repositories.GormTransactor` implementation stores the active `*gorm.DB` transaction in the context using a private key. All repository methods call `dbFromContext(ctx, r.db)` to transparently use the active transaction or fall back to the plain DB handle. **`*gorm.DB` never appears in any port interface.**

`PlayerRepository.FindOrCreate` applies `SELECT FOR UPDATE` — always call it inside a `Transactor.InTransaction` block.

Available repository write methods (all accept `context.Context`):
- `repos.Player.Find(ctx, playerID) (*types.PlayerRecord, error)` — plain read, no lock
- `repos.Player.FindOrCreate(ctx, playerID) (*types.PlayerRecord, error)` — upserts and locks (must be inside tx)
- `repos.Player.UpdateBalance(ctx, *types.PlayerRecord) error` — saves updated R/G/B/Nonce
- `repos.Transaction.Create(ctx, *types.TransactionRecord) error` — inserts a new tx record
- `repos.Block.AllBlocks(ctx) ([]*types.Block, error)` — all blocks ordered by ascending height
- `repos.Block.TransactionHashesByBlock(ctx, height) ([]string, error)` — tx hashes sealed in a block
- `repos.Authority.Register(ctx, *types.AuthorityRecord) error` — idempotent upsert
- `repos.Authority.Exists(ctx, pubKey []byte) (bool, error)` — MINT authority check

---

## DI / Wiring Pattern

`cmd/*.go` manually wires dependencies — no framework. The old `Module` / `Container` wrappers have been removed; construction is direct and flat.

**Ledger** (`cmd/ledger/main.go`):
```
drivenpostgres.Init → db
repositories.NewPlayerRepository(db)
repositories.NewTransactionRepository(db)
repositories.NewBlockRepository(db)
repositories.NewAuthorityRepository(db)
repositories.NewTransactor(db)              ← Transactor port
authorityRepo.Register(genesisRecord)       ← seed genesis authority on boot
engine.New()                                ← domain engine
service.NewLedgerService(...)               ← application service
service.NewBlockSealer(blockRepo, transactor, interval, difficulty)
grpcledger.New(ledgerSvc, authorityRepo)    ← driving adapter (open MINT check)
pb.RegisterLedgerServiceServer(grpcServer, handler)
ledgerSvc.ValidateChain(ctx)                ← chain integrity check on startup
go blockSealer.Start(ctx)                   ← PoW mining loop
```

**Game Server** (`cmd/server/main.go`):
```
authority.NewFullAuthority(kp)
drivenredis.Init → redisClient
drivenredis.NewMissionRepository(redisClient)
drivenledger.New(pb.NewLedgerServiceClient(conn)) ← LedgerClient driven adapter
service.NewGameService(missionRepo, auth, ledgerClient, cfg)
grpcgame.New(gameSvc)                              ← driving adapter
pb.RegisterGameServiceServer(grpcServer, handler)
```

**Player CLI** (`cmd/player/main.go`): `config.InitPlayerFullConfig → crypto.LoadOrGenerateKey → grpc.NewClient(LedgerAddr) → grpc.NewClient(ServerAddr) → interactive menu loop`.

---

## Authority Adapter

`internal/adapter/driven/authority` provides a concrete `Authority` struct:
- `authority.NewPublicAuthority(pubKey)` — implements `out.PublicAuthority` (Ledger: verify-only)
- `authority.NewFullAuthority(kp *crypto.Keypair)` — implements `out.FullAuthority` (Game Server: signing)
- `authority.Load(cfg *config.AuthorityConfig)` — convenience loader for the Ledger

---

## LedgerClient Driven Adapter

`internal/adapter/driven/ledger/client.go` implements `out.LedgerClient`. It encapsulates all proto marshaling and MINT signing, keeping `GameService` free of transport concerns:
- `GetAuthorityNonce(ctx, authorityID)` → calls `pb.GetBalance`, returns `NextNonce`
- `SubmitMint(ctx, auth, senderID, receiverID, amtR, amtG, amtB, nonce)` → builds `pb.TransactionPayload`, calls `auth.Sign(rawPayload)`, submits via `pb.SubmitTransaction`

---

## Redis Mission Storage

Game Server stores mission state in Redis via `adapter/driven/redis.MissionRepository`. Key layout:
- `mission:{id}` — JSON `types.MissionRecord`, TTL = 2 × `MISSION_COOLDOWN_SECONDS`
- `player:active:{playerID}` — active mission ID pointer, same TTL
- `player:cooldown:{playerID}` — presence-only cooldown sentinel, TTL = `MISSION_COOLDOWN_SECONDS`

`out.MissionRepository` methods: `Create`, `FindByID`, `FindActiveByPlayer`, `GetCooldownRemaining`, `Complete`. Construct via `redis.NewMissionRepository(client *redis.Client)`.

---

## Proto & Generated Code

- Proto sources: `api/proto/v1/ledger.proto`, `api/proto/v1/game.proto`
- `game.proto` imports `ledger.proto` (shares `BalanceResponse`)
- Both use `option go_package = "./pkg/pb"` → all generated types land in `pkg/pb`
- Regenerate after any `.proto` change: `PATH="$PATH:$(go env GOPATH)/bin" make proto-v1`
- `LedgerService` RPCs: `GetBalance`, `SubmitTransaction`, `RegisterAuthority`; `SubmitTransactionResponse` returns `new_balance BalanceResponse`
- `GameService` RPCs: `RequestMission(player_id) → MissionResponse` and `CompleteMission(mission_id, player_id) → CompleteMissionResponse`

> **Note:** `protoc-gen-go` and `protoc-gen-go-grpc` must be on `PATH`. Install with:
> `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`
> `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest`
> `protoc` itself can be installed with `brew install protobuf` on macOS.

---

## Logging & Config

- Always use `pkg/logger` (slog-based): `logger.Infof(...)`, `logger.Warnf(...)`, `logger.Errorf(...)`, `logger.Fatalf(...)` — never `fmt.Print` or stdlib `log`
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

`service.BlockSealer` calls `mineBlock(b)` which increments `b.Nonce` until `b.Hash` starts with `b.Difficulty` `'0'` characters.
Default difficulty is `2` (e.g. hash starts with `00…`). Raise `BLOCK_DIFFICULTY` to slow mining.

### Chain Integrity Validation

`domain/chain.ValidateChain(blocks, txsByBlock)` is a pure function (no I/O) that checks:
1. Each block's stored hash matches recomputed `sha256(…)`
2. Each block's `PrevHash` matches the previous block's `Hash`
3. Each block's `MerkleRoot` matches `merkle.BuildMerkleRoot(txHashes)`
4. Each block's hash satisfies its `Difficulty` PoW target

`LedgerService.ValidateChain(ctx)` loads all blocks + tx hashes from Postgres and delegates to the domain function.
Called automatically on Ledger startup; also available as a use-case for future RPC exposure.

### Authority Registry

MINT transactions are now open to any registered authority (not a single hardcoded key).
- **Table:** `authorities` — `(id, pub_key_hex, registered_at)`
- **Register:** call `RegisterAuthority` RPC with `(pub_key, signature_of_pub_key)` — the Ledger verifies the self-signed proof and inserts the record
- **Check on MINT:** `authorityRepo.Exists(pubKey)` in the gRPC driving adapter
- **Genesis seed:** the key from `AUTHORITY_PUB_KEY_PATH`/`AUTHORITY_PUB_KEY` config is automatically inserted on Ledger startup

---

## Adding a New Feature (Checklist)

1. **Domain types/logic** → `internal/domain/types/` or `internal/domain/engine/`
2. **New port** → `internal/app/port/out/` (driven) or `internal/app/port/in/` (driving)
3. **Application service change** → `internal/app/service/` — no pb/gorm imports allowed
4. **Driving adapter** (new RPC) → `internal/adapter/driving/grpc/{service}/handler.go` — translate pb ↔ domain, then call use case
5. **Driven adapter** (new infra) → `internal/adapter/driven/{tech}/` — implement out port
6. **Wire** → update the relevant `cmd/*/main.go`
