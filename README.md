# CryptoExchange

A crypto exchange in Go: an order book matching engine behind an HTTP API, with wallet-signed orders and a write-ahead log.

## Features

- In-memory limit order book per symbol (ETH, BTC, LTC) with price-time priority
- Limit orders that match against the book on arrival, remainder rests
- Market orders with a signed worst-price limit
- Integer (tick) prices, no floating point
- Order expiry
- Self-trade prevention
- Wallet-address users: orders and cancels are EIP-712 signatures (`eth_signTypedData_v4`), no server-held user keys
- Replay protection with per-maker nonces
- Write-ahead log with crash recovery and optional HMAC tamper detection
- HTTP API (Echo) with structured request logging (zerolog)
- Graceful shutdown
- Secrets loaded from environment / `.env`, settings from YAML

## Architecture

```text
internal/
  MatchEngine/   order book and matching engine
  symbol/        traded symbols and per-symbol books
  signing/       EIP-712 order/cancel types, signer recovery
    signingtest/ test wallet for signing orders in tests
  exchange/      trading service: orders, cancels, book snapshots, validation
  recovery/      WAL persistence and startup replay
  api/           HTTP handlers, DTOs, routes
  wal/           write-ahead log
  logging/       logger setup, request logging middleware
  config/        configuration loading
cmd/
  api/           server entry point
```

## Requirements

- Go 1.27+ (see `go.mod`)
- A running [Ganache](https://trufflesuite.com/ganache/) node (default `localhost:8545`)

## Building and running

```sh
make build      # builds ./bin/exchange (the server)
make run        # builds the server, then runs it with ./config/config.yaml
make test       # go test -v ./...
make test-race
```

Or directly:

```sh
go build -o ./bin/exchange ./cmd/api
CONFIG_PATH=./config/config.yaml ./bin/exchange
# or: ./bin/exchange -config ./config/config.yaml
```

### Configuration

- `config/config.yaml`: non-secret settings (template: [`config/config.example.yaml`](config/config.example.yaml)), selected with `CONFIG_PATH` or `-config`.
- Environment variables: secrets only, never read from the YAML. A gitignored `.env` (template: [`.env.example`](.env.example)) is loaded on startup.

```yaml
# config/config.yaml
env: "dev"            # "dev" or "prod"
httpServer:
    address: "0.0.0.0"
    port: 8080
ganacheServer:
    address: "localhost"
    port: 8545
logLevel: "info"
logPretty: false
walPath: "./data/wal.log"
```

```sh
# .env
CONFIG_PATH=./config/config.yaml
DB_PATH=postgresql://<username>:<password>@localhost:5432/<DatabaseName>
OPERATOR_PRIVATE_KEY=<PRIVATE_KEY>
WAL_HMAC_KEY=
```

Secrets (environment only):

| Var | Default | Description |
|---|---|---|
| `DB_PATH` | *(required)* | database URL, including its password |
| `OPERATOR_PRIVATE_KEY` | *(required)* | the exchange's own operator key (hex, no `0x`), never a user's key |
| `WAL_HMAC_KEY` | *(unset)* | HMAC key for WAL tamper detection |

Non-secret settings (`config.yaml`, overridable by environment variable):

| Var | YAML key | Default | Description |
|---|---|---|---|
| `CONFIG_PATH` | — | — | which YAML file to load (or `-config`) |
| `HOST` | `httpServer.address` | `0.0.0.0` | HTTP listen address |
| `PORT` | `httpServer.port` | `8080` | HTTP listen port |
| `GANACHE_HOST` | `ganacheServer.address` | `localhost` | Ethereum node host |
| `GANACHE_PORT` | `ganacheServer.port` | `8545` | Ethereum node port |
| `LOG_LEVEL` | `logLevel` | `info` | minimum log level: `debug`, `info`, `warn`, `error` |
| `LOG_PRETTY` | `logPretty` | `false` | `true` for readable console logs instead of JSON |
| `WAL_PATH` | `walPath` | `./data/wal.log` | write-ahead log file (parent dir is created) |

## API

| Method | Path | Body | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/healthz` | — | health check, lists configured symbols |
| `GET` | `/api/v1/exchange/orderbooks/{symbol}` | — | order book for one symbol |
| `GET` | `/api/v1/exchange/orderbooks?symbols=ETH,BTC` | — | order books for the listed symbols, or all symbols if omitted |
| `POST` | `/api/v1/exchange/orders/limit` | signed order, `isMarket: false` | place a limit order (trades what crosses, rests the rest) |
| `POST` | `/api/v1/exchange/orders/market` | signed order, `isMarket: true` | place a market order (fills now, within the worst price) |
| `DELETE` | `/api/v1/exchange/orders/cancel` | signed cancel | cancel one of the maker's resting orders |

Request and response shapes: `internal/api/dto.go`. Signed message types: `internal/signing/signing.go`.

## License

MIT — see [`LICENSE`](LICENSE).
