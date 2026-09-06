# Copilot Instructions

## Build & Run

```sh
docker compose up -d      # start Postgres + Redis
make keygen               # generate authority keypair → .key/
make migrate              # run GORM AutoMigrate
make build-all            # compile → bin/ledger, bin/server, bin/player
make proto-v1             # regenerate pkg/pb/ after .proto changes
```

Run services in order (each depends on the previous):

```sh
make run-ledger   # Terminal 1
make run-server   # Terminal 2
make run-player   # Terminal 3
```

There are no automated tests in this repository.

## Architecture

Three gRPC binaries: **Ledger** (`cmd/ledger`, port 50051), **Game Server** (`cmd/server`, port 50052), and **Player
CLI** (`cmd/player`). Both Player and Game Server call the Ledger; Game Server issues MINT txs, players issue TRANSFER
txs.

The codebase uses **Hexagonal Architecture** with strict import rules:

| Layer            | Package                     | Import constraint                           |
|------------------|-----------------------------|---------------------------------------------|
| Domain           | `internal/domain/`          | stdlib only                                 |
| Application      | `internal/app/`             | domain + stdlib only — no `pkg/pb`, no gorm |
| Driving adapters | `internal/adapter/driving/` | imports `app/port/in`, `pkg/pb`, crypto     |
| Driven adapters  | `internal/adapter/driven/`  | implement `app/port/out` interfaces         |
| Entry points     | `cmd/`                      | wires all layers; no business logic         |

## Key Conventions

### Identity & Signing

- Player/authority ID = `hex(sha256(ed25519_pubkey))` via `pkg/crypto.PubKeyToPlayerID()`
- Transaction wire format: `proto.Marshal(TransactionPayload)` → sign bytes → send
  `{raw_payload, signature, sender_pub_key}`
- **All crypto verification lives in `adapter/driving/grpc/ledger/handler.go`** — the application service receives only
  pre-verified, typed requests

### Transactor Pattern

`app/port/out.Transactor.InTransaction(ctx, fn)` abstracts DB transactions. All repository methods call
`dbFromContext(ctx, r.db)` to transparently use the active transaction or fall back to the plain handle. `*gorm.DB`
never appears in any port interface. `PlayerRepository.FindOrCreate` uses `SELECT FOR UPDATE` — always call inside a
transaction.

### Logging & Config

- Always use `pkg/logger` (slog-based): `logger.Infof/Warnf/Errorf/Fatalf` — never `fmt.Print` or stdlib `log`
- Call `logger.Init()` at the top of every `main()`
- Access env vars via `utils.GetEnv("KEY", "default")` — never `os.Getenv` directly
- `POSTGRES_URL` is **ignored** — DSN is always reconstructed from individual `POSTGRES_*` vars

### Domain Types

- `types.RGB{R, G, B int8}` — signed delta (mission rewards, engine logic)
- `types.PlayerRecord{Red, Green, Blue uint32}` — unsigned storage
- `types.PlayerState{R, G, B uint8, Nonce uint64}` — engine working state
- `enum.Color{Red=0, Green=1, Blue=2}` matches `pb.RewardColor` ordinals

### Blockchain

- `service.BlockSealer` mines blocks in a background goroutine (PoW via `BLOCK_DIFFICULTY` leading zero hex nibbles)
- `domain/chain.ValidateChain()` is a pure function (no I/O); called on Ledger startup
- `domain/merkle.BuildMerkleRoot()` takes sorted tx hashes
- Genesis authority key is always seeded into the `authorities` table on startup; additional authorities self-register
  via `RegisterAuthority` RPC

### Adding a New Feature

1. Domain types/logic → `internal/domain/`
2. New port interface → `internal/app/port/out/` or `port/in/`
3. Application service change → `internal/app/service/` (no pb/gorm imports)
4. Driving adapter (new RPC) → `internal/adapter/driving/grpc/{service}/handler.go`
5. Driven adapter (new infra) → `internal/adapter/driven/{tech}/`
6. Wire everything → `cmd/*/main.go`
