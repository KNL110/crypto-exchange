# CryptoExchange

Crypto exchange backend written in Go.

## Features

- Order book per symbol (ETH, BTC, LTC)
- Limit and market orders
- Order expiry
- Self-trade prevention
- Orders and cancels signed with the user's wallet (EIP-712)
- Nonce-based replay protection
- Write-ahead log with crash recovery
- HTTP API with request logging
- Graceful shutdown

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

Settings are read from `config/config.yaml` (see [`config/config.example.yaml`](config/config.example.yaml)). Secrets are read from environment variables; a `.env` file (see [`.env.example`](.env.example)) is loaded at startup.

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

Secrets:

| Var | Default | Description |
|---|---|---|
| `DB_PATH` | *(required)* | database connection URL |
| `OPERATOR_PRIVATE_KEY` | *(required)* | operator private key (hex) |
| `WAL_HMAC_KEY` | *(unset)* | HMAC key for the write-ahead log |

Settings (can be overridden by environment variables):

| Var | YAML key | Default | Description |
|---|---|---|---|
| `CONFIG_PATH` | — | — | config file path (or `-config`) |
| `HOST` | `httpServer.address` | `0.0.0.0` | HTTP listen address |
| `PORT` | `httpServer.port` | `8080` | HTTP listen port |
| `GANACHE_HOST` | `ganacheServer.address` | `localhost` | Ethereum node host |
| `GANACHE_PORT` | `ganacheServer.port` | `8545` | Ethereum node port |
| `LOG_LEVEL` | `logLevel` | `info` | minimum log level: `debug`, `info`, `warn`, `error` |
| `LOG_PRETTY` | `logPretty` | `false` | human-readable log output |
| `WAL_PATH` | `walPath` | `./data/wal.log` | write-ahead log file |

## API

| Method | Path | Body | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/healthz` | — | health check |
| `GET` | `/api/v1/exchange/orderbooks/{symbol}` | — | order book for one symbol |
| `GET` | `/api/v1/exchange/orderbooks?symbols=ETH,BTC` | — | order books for several symbols (all if omitted) |
| `POST` | `/api/v1/exchange/orders/limit` | signed order | place a limit order |
| `POST` | `/api/v1/exchange/orders/market` | signed order | place a market order |
| `DELETE` | `/api/v1/exchange/orders/cancel` | signed cancel | cancel an order |

Request and response types are in `internal/api/dto.go`; signed message types are in `internal/signing/signing.go`.

## License

MIT — see [`LICENSE`](LICENSE).
