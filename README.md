# CryptoExchange

A small crypto exchange written in Go: an in-memory limit order book matching engine, wrapped in an HTTP API with graceful shutdown, plus an Ethereum client connected to a local Ganache node.

## Matching engine

The core matching logic lives in `internal/MatchEngine` (package `matchengine`) and knows nothing about HTTP or symbols — it's just an order book for one trading pair.

- **`Order`** — a single buy (`bid`) or sell (`ask`) order: size, side, timestamp, an ID once placed through the exchange layer.
- **`Limit`** — one price level, holding a FIFO linked list of resting orders. Matching walks this list in insertion order, so fills at a price level are always oldest-order-first.
- **`OrderBook`** — both sides of the market for one symbol:
  - `PlaceLimitOrder(order, price)` rests an order at a price level (creating the level if needed). It does not currently cross the book — a limit order always rests, even if it would be immediately marketable.
  - `PlaceMarketOrder(order)` walks the best price levels on the opposite side, filling the incoming order until it's either fully filled or liquidity runs out, and returns every individual fill as a `MatchedOrder`.
  - `CancelOrder(id)` / `GetOrder(id)` remove/look up a resting order by ID.

Matching is deterministic: given the same sequence of place/cancel actions against an empty book, you always end up in the same state.

## Architecture

```text
internal/
  MatchEngine/   core order book + matching (above)
  config/        YAML config loading
  exchange/      owns one order book per symbol (ETH, BTC, LTC), assigns
                 order IDs, HTTP handlers, DTOs and routes
  users/         user model
cmd/
  api/           exchange server: wires everything together, graceful
                 shutdown, Ethereum client (Ganache)
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

Shut it down with `Ctrl+C` (`SIGINT`) or `SIGTERM` for a graceful drain.

### Configuration

Configuration is loaded from a YAML file (see `internal/config/config.go`), given via the `CONFIG_PATH` env var or the `-config` flag. Copy [`config/config.example.yaml`](config/config.example.yaml) to `config/config.yaml` and fill it in:

```yaml
env: "dev"            # "dev" or "prod"
dbPath: <DATABASE_URL>
httpServer:
    address: "0.0.0.0"
    port: 8080
ganacheServer:
    address: "localhost"
    port: 8545
    privateKey: <PRIVATE_KEY>
```

Some fields can be overridden with environment variables:

| Var | Default | Description |
|---|---|---|
| `HOST` | `0.0.0.0` | HTTP listen address |
| `PORT` | `8080` | HTTP listen port |
| `DB_PATH` | *(required)* | database URL |
| `GANACHE_HOST` | `localhost` | Ganache node host |
| `GANACHE_PORT` | `8545` | Ganache node port |

## API

All request bodies must be sent as JSON with `Content-Type: application/json`.

| Method | Path | Body | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/healthz` | — | health check, lists configured symbols |
| `GET` | `/api/v1/exchange/orderbook` | `{"symbol"}` | order book snapshot for one symbol |
| `GET` | `/api/v1/exchange/orderbooks` | `{"symbols":[...]}` | order book snapshots for several symbols |
| `POST` | `/api/v1/exchange/orders/limit` | `{"symbol","isBid","size","price"}` | place a limit order |
| `POST` | `/api/v1/exchange/orders/market` | `{"symbol","isBid","size"}` | place a market order (matches immediately) |
| `DELETE` | `/api/v1/exchange/orders/cancel` | `{"symbol","orderId"}` | cancel a resting order |

Request/response shapes are in `internal/exchange/dto.go`.

Quick example:

```sh
curl -X POST localhost:8080/api/v1/exchange/orders/limit \
  -H 'Content-Type: application/json' \
  -d '{"symbol":"BTC","isBid":false,"size":10,"price":65000}'

curl -X GET localhost:8080/api/v1/exchange/orderbook \
  -H 'Content-Type: application/json' \
  -d '{"symbol":"BTC"}'

curl -X POST localhost:8080/api/v1/exchange/orders/market \
  -H 'Content-Type: application/json' \
  -d '{"symbol":"BTC","isBid":true,"size":5}'
```

## License

MIT — see [`LICENSE`](LICENSE).
