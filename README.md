# RGB Mini-Game

A distributed blockchain mini-game built with Go, gRPC, PostgreSQL and Redis.  
Players complete missions to earn RGB tokens, then freely transfer them between each other. All token state is persisted
on a tamper-resistant ledger backed by Postgres.

The codebase follows **Hexagonal Architecture** (Ports & Adapters), keeping domain logic and application use cases
completely independent of transport and infrastructure concerns.

---

## Architecture

### System topology

Three independent binaries communicate over gRPC:

```
┌──────────┐  gRPC: RequestMission / CompleteMission   ┌──────────┐
│  Player  │ ◄──────────────────────────────────────►  │  Server  │
│  (CLI)   │                                           │  (Game)  │
└────┬─────┘                                           └────┬─────┘
     │ gRPC: GetBalance / SubmitTransaction(TRANSFER)       │ gRPC: SubmitTransaction(MINT)
     └──────────────────► ┌──────────┐ ◄────────────────────┘
                          │  Ledger  │
                          └──────────┘
                               │
                          ┌────┴─────┐
                          │ Postgres │
                          └──────────┘
```

| Binary          | Entry point          | Default port |
|-----------------|----------------------|--------------|
| **Ledger**      | `cmd/ledger/main.go` | `50051`      |
| **Game Server** | `cmd/server/main.go` | `50052`      |
| **Player CLI**  | `cmd/player/main.go` | —            |

- **Ledger** — validates and persists every transaction (MINT & TRANSFER) in Postgres. The gRPC driving adapter
  verifies ed25519 signatures and enforces the authority public-key rule; the application service receives only
  pre-verified, typed requests.
- **Game Server** — issues missions to players and mints reward tokens via signed MINT transactions sent to the Ledger.
  Stores active mission state in Redis with a configurable cooldown.
- **Player CLI** — interactive terminal client. Lets a player check their balance, request/complete missions, and
  transfer tokens to other players.

### Layered design

```
        ┌─────────────────────────────────────┐
        │            Domain Ring               │
        │  internal/domain/{types,enum,engine} │
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

| Layer | Rule |
|---|---|
| **Domain** (`internal/domain/`) | Zero external imports — only stdlib |
| **Application** (`internal/app/`) | Imports domain only — no `pkg/pb`, no gorm |
| **Driving adapters** (`adapter/driving/`) | Own all transport concerns (proto, crypto) |
| **Driven adapters** (`adapter/driven/`) | Implement out-ports; own all infra concerns |
| **`cmd/`** | Wires all layers; no business logic |

---

## Prerequisites

| Tool                                              | Version                     |
|---------------------------------------------------|-----------------------------|
| Go                                                | 1.25+                       |
| Docker & Docker Compose                           | any recent                  |
| `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc` | for proto regeneration only |

---

## Quick Start

### 1 — Start infrastructure

```sh
docker compose up -d      # starts Postgres (5432) and Redis (6379)
```

### 2 — Configure environment

Copy and fill in the required values:

```sh
cp .env.example .env
```

Then edit `.env` — the key values that need your attention:

| Variable                 | Where to get it                                                         |
|--------------------------|-------------------------------------------------------------------------|
| `POSTGRES_PASSWORD`      | Set to your own secret                                                  |
| `AUTHORITY_PUB_KEY_PATH` | Leave as `.key/id_ed25519` (populated by `make keygen` in step 4)       |
| `AUTHORITY_PUB_KEY`      | Alternatively paste the hex from `.key/id_ed25519.pub.hex` after step 4 |

> **Note:** `POSTGRES_URL` is intentionally ignored — the Ledger always reconstructs the DSN from the individual
`POSTGRES_*` variables.

### 3 — Generate the authority keypair

```sh
make keygen
```

This writes two files:

| File                      | Contents                                                           |
|---------------------------|--------------------------------------------------------------------|
| `.key/id_ed25519`         | JSON keypair (used by Game Server)                                 |
| `.key/id_ed25519.pub.hex` | Bare hex public key — copy this into `AUTHORITY_PUB_KEY` in `.env` |

### 4 — Run database migrations

```sh
make migrate
```

### 5 — Build all binaries

```sh
make build-all
# produces: bin/ledger  bin/server  bin/player
```

### 6 — Run the services

Open three terminals. **Start in order** — each service depends on the one before it:

```sh
# Terminal 1 — Ledger (must be up first)
make run-ledger

# Terminal 2 — Game Server (connects to Ledger on start)
make run-server

# Terminal 3 — Player CLI (connects to both)
make run-player
```

> **Tip:** To run a pre-built binary instead, use `make build-all` first, then `./bin/ledger`, `./bin/server`, `./bin/player`.

---

## Configuration Reference

### Ledger (`cmd/ledger/main.go`)

| Env var                  | Default     | Description                                              |
|--------------------------|-------------|----------------------------------------------------------|
| `LEDGER_GRPC_PORT`       | `50051`     | gRPC listen port                                         |
| `AUTHORITY_PUB_KEY_PATH` | —           | Path to JSON keypair file (public key extracted)         |
| `AUTHORITY_PUB_KEY`      | —           | Hex-encoded ed25519 public key (takes precedence if set) |
| `POSTGRES_USER`          | —           | Postgres credentials                                     |
| `POSTGRES_PASSWORD`      | —           |                                                          |
| `POSTGRES_HOST`          | `localhost` |                                                          |
| `POSTGRES_PORT`          | `5432`      |                                                          |
| `POSTGRES_NAME`          | —           |                                                          |
| `POSTGRES_SSL_MODE`      | `disable`   |                                                          |

### Game Server (`cmd/server/main.go`)

| Env var                    | Default           | Description                                            |
|----------------------------|-------------------|--------------------------------------------------------|
| `GAME_SERVER_GRPC_PORT`    | `50052`           | gRPC listen port                                       |
| `LEDGER_ADDR`              | `localhost:50051` | Ledger gRPC address                                    |
| `AUTHORITY_KEY_PATH`       | `.key/id_ed25519` | Full keypair used to sign MINT transactions            |
| `MISSION_COOLDOWN_SECONDS` | `300`             | Seconds a player must wait between mission completions |
| `REDIS_ADDR`               | `localhost:6379`  | Redis address                                          |
| `REDIS_PASSWORD`           | `""`              | Redis password                                         |
| `REDIS_DB`                 | `0`               | Redis database index                                   |

### Player CLI (`cmd/player/main.go`)

| Env var           | Default               | Description                                  |
|-------------------|-----------------------|----------------------------------------------|
| `PLAYER_KEY_PATH` | `.key/player_ed25519` | Player keypair (auto-generated on first run) |
| `LEDGER_ADDR`     | `localhost:50051`     | Ledger gRPC address                          |
| `SERVER_ADDR`     | `localhost:50052`     | Game Server gRPC address                     |

---

## Playing the Game

When you run the Player CLI it prints your **Player ID** (derived as `hex(sha256(pubkey))`) and shows an interactive
menu:

```
1. Get Balance
2. Request Mission
3. Complete Mission
4. Transfer
5. Quit
```

| Action               | What happens                                                                                                    |
|----------------------|-----------------------------------------------------------------------------------------------------------------|
| **Get Balance**      | Fetches your current R/G/B token balances and nonce from the Ledger                                             |
| **Request Mission**  | Game Server assigns a random color mission (Red / Green / Blue); prints a live countdown until it's completable |
| **Complete Mission** | Game Server mints 1 token of the mission color into your balance via a signed MINT transaction on the Ledger    |
| **Transfer**         | Send R/G/B tokens directly to another player; requires their Player ID and the amounts                          |

**Cooldown rule:** after completing a mission you must wait `MISSION_COOLDOWN_SECONDS` (default 5 min) before requesting
a new one.

---

## Identity & Signing Model

- **Player / authority ID** = `hex(sha256(ed25519_pubkey))`
- Every transaction is a signed `TransactionPayload` protobuf:  
  `raw_payload | signature | sender_pub_key`
- The Ledger **gRPC driving adapter** verifies: `sha256(sender_pub_key) == payload.sender_id` **and** the signature is
  valid. The application service (`LedgerService`) receives only pre-verified, typed data.
- **MINT** transactions additionally require `sender_pub_key == AUTHORITY_PUB_KEY`. The authority (Game Server) needs no
  prior balance.

---

## Developer Commands

```sh
make proto-v1        # regenerate pkg/pb/ from api/proto/v1/*.proto
make keygen          # generate authority ed25519 keypair → .key/
make migrate         # run GORM AutoMigrate (creates players + transactions tables)
make build-ledger    # compile → bin/ledger
make build-server    # compile → bin/server
make build-player    # compile → bin/player
make build-all       # compile all three binaries
```

---

## Project Layout

```
cmd/
  ledger/           Ledger entry point & DI wiring
  server/           Game Server entry point & DI wiring
  player/           Player CLI entry point
  keygen/           Authority keypair generator
  migrate/          Database migration runner
config/             Env-based config structs
internal/
  domain/           Pure domain ring (no external imports)
    types/          PlayerRecord, PlayerState, TransactionRecord, MissionRecord, RGB
    enum/           Color (Red/Green/Blue)
    engine/         Game calculation logic (PlayerTransactions, PlayerCompleteMission)
  app/              Application ring
    port/
      in/           Primary ports — LedgerUseCase, GameUseCase + request/result types
      out/          Secondary ports — Transactor, PlayerRepository, TransactionRepository,
                    MissionRepository, GameEngine, PublicAuthority, FullAuthority, LedgerClient
    service/        LedgerService, GameService, missionService (no pb/gorm imports)
  adapter/
    driving/        Left-side (primary) adapters — receive gRPC, call use cases
      grpc/
        ledger/     pb.LedgerServiceServer — owns crypto verification
        game/       pb.GameServiceServer
    driven/         Right-side (secondary) adapters — implement out/ ports
      postgres/     GormTransactor + PlayerRepository + TransactionRepository
      redis/        MissionRepository (TTL-based)
      authority/    PublicAuthority + FullAuthority + Load()
      ledger/       LedgerClient — wraps pb.LedgerServiceClient, handles signing
api/proto/v1/       Protobuf definitions (ledger.proto, game.proto)
pkg/
  pb/               Generated gRPC code
  crypto/           Key generation, signing helpers
  logger/           slog-based structured logger
  utils/            Env helpers
```

---

## Tech Stack

- **Go 1.25** — all three binaries
- **gRPC / protobuf** — inter-service communication
- **PostgreSQL 17** (via GORM) — persistent transaction & balance storage
- **Redis 8** — ephemeral mission state & cooldown tracking
- **ed25519** — transaction signing and authority verification

---

## License

MIT

