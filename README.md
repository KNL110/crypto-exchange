# CryptoExchange

A small crypto exchange written in Go: an in-memory limit order book matching engine, wrapped in an HTTP API with graceful shutdown and crash recovery from a write-ahead log. Users are wallet addresses: every order and cancel is an EIP-712 message signed by the user's wallet, and the server never holds user keys.

## Matching engine

The core matching logic lives in `internal/MatchEngine` (package `matchengine`) and knows nothing about HTTP or symbols — it's just an order book for one trading pair.

- **`Order`** — a single buy (`bid`) or sell (`ask`) order: maker address, remaining size, filled amount, side, expiry, timestamp, and an ID once placed through the exchange layer.
- **`Limit`** — one price level, holding a FIFO linked list of resting orders. Matching walks this list in insertion order, so fills at a price level are always oldest-order-first.
- **`OrderBook`** — both sides of the market for one symbol:
  - `PlaceLimitOrder(order, price)` first trades the order against the opposite side at every price level its limit allows (a buy against asks ≤ price, a sell against bids ≥ price), each fill at the resting order's price. Whatever is left rests at `price`. Returns the fills.
  - `PlaceMarketOrder(order, worstPrice)` fills the whole order immediately, best price first, never past `worstPrice`. If the book can't fill all of it within that bound, nothing is filled and it returns `ErrInsufficientLiquidity`.
  - Prices are whole numbers of ticks (`uint64`), never floats.
  - Expired resting orders are removed before matching, so they never trade, and before the book is shown.
  - Self-trade prevention: an order never fills against a resting order from the same maker. That resting order is cancelled instead, and it doesn't count as liquidity for a market order.
  - `CancelOrder(id)` / `GetOrder(id)` remove/look up a resting order by ID.

Matching is deterministic: given the same sequence of place/cancel actions against an empty book, you always end up in the same state.

## Architecture

```text
internal/
  MatchEngine/   core order book + matching (above)
  symbol/        the traded symbols (ETH, BTC, LTC) and each symbol's book + lock
  signing/       EIP-712 Order/Cancel types and signer recovery
    signingtest/ test wallet that signs orders, shared by the tests
  exchange/      the trading service: place/cancel orders, book snapshots,
                 signature/expiry/nonce checks, order IDs; journals every
                 action and can replay it (knows nothing about HTTP or files)
  recovery/      persists actions to the WAL and replays it into the
                 exchange on startup
  api/           HTTP layer: handlers, DTOs, routes, error -> status mapping
  wal/           write-ahead log: append (fsynced) + replay, optional HMAC chain
  logging/       zerolog setup + one log line per HTTP request
  config/        YAML config loading
cmd/
  api/           exchange server: wires everything together, graceful
                 shutdown, Ethereum client (reads the chain ID for signing)
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

Shut it down with `Ctrl+C` (`SIGINT`) or `SIGTERM` for a graceful drain. `kill -9` is safe too: the order books come back on the next start (see [Persistence](#persistence)).

### Configuration

Settings come from two places:

- **`config/config.yaml`** holds non-secret settings. Copy [`config/config.example.yaml`](config/config.example.yaml) to it. The file is chosen with `CONFIG_PATH` or the `-config` flag.
- **Environment variables** hold the secrets, which are never read from the YAML. For local development, copy [`.env.example`](.env.example) to `.env` (gitignored) and fill it in. `.env` in the working directory is loaded automatically on startup. Variables already set in the real environment win over `.env`, and environment variables win over `config.yaml`. In production, set the secrets from your secrets manager instead of a file.

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
| `WAL_HMAC_KEY` | *(unset)* | makes the WAL tamper-evident, see [Persistence](#persistence) |

Non-secret settings (in `config.yaml`, each can be overridden by its variable):

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

GET endpoints take query parameters. All request bodies must be sent as JSON with `Content-Type: application/json`.

| Method | Path | Body | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/healthz` | — | health check, lists configured symbols |
| `GET` | `/api/v1/exchange/orderbook?symbol=ETH` | — | order book snapshot for one symbol |
| `GET` | `/api/v1/exchange/orderbooks?symbols=ETH&symbols=BTC` | — | order book snapshots for several symbols |
| `POST` | `/api/v1/exchange/orders/limit` | signed order, `isMarket: false` | place a limit order (trades what crosses, rests the rest) |
| `POST` | `/api/v1/exchange/orders/market` | signed order, `isMarket: true` | place a market order (fills now, within the worst price) |
| `DELETE` | `/api/v1/exchange/orders/cancel` | signed cancel | cancel one of your resting orders |

Request/response shapes are in `internal/api/dto.go`. Both order endpoints respond with:

```json
{ "order": { … }, "matches": [ { "sizeFilled": 10, "price": 3500 } ], "cancelled": [ { … } ] }
```

`order` is the order after matching (`size` = unfilled, `filled` = traded). `cancelled` lists your own resting orders that self-trade prevention removed.

### Signed orders

There are no accounts. A user is their wallet address, and every order and cancel must be signed by that wallet with [EIP-712](https://eips.ethereum.org/EIPS/eip-712) (`eth_signTypedData_v4`). The server recovers the signer and rejects the request unless it matches `maker`. Types are defined in `internal/signing/signing.go`:

```text
EIP712Domain { name: "CryptoExchange", version: "1", chainId }   // chainId read from the node
Order  { address maker, string symbol, bool isBid, bool isMarket, uint256 size, uint256 price, uint256 nonce, uint256 expiry }
Cancel { address maker, string symbol, uint256 orderId }
```

- `price` is a whole number of ticks. For a limit order it's the limit price; for a market order it's the worst price you accept (slippage protection), and it must be > 0.
- `isMarket` is part of what's signed, so a signed limit order can't be submitted as a market order or the other way round.
- `nonce` must be unique per maker. Reusing one is rejected, which stops replays.
- `expiry` is a unix timestamp in seconds. Expired orders are rejected.
- Only the maker of an order can cancel it.

Order request body:

```json
{
  "maker": "0xYourWalletAddress",
  "symbol": "ETH",
  "isBid": false,
  "isMarket": false,
  "size": 10,
  "price": 3500,
  "nonce": 1,
  "expiry": 1767225600,
  "signature": "0x…65-byte signature…"
}
```

Cancel request body:

```json
{ "maker": "0xYourWalletAddress", "symbol": "ETH", "orderId": 1, "signature": "0x…" }
```

| Status | Meaning |
|---|---|
| `401` | signature doesn't match `maker` (or is for another chain) |
| `403` | cancel for an order you don't own |
| `409` | nonce already used, or not enough liquidity for a market order |
| `400` | bad size/price, expired order, `isMarket` doesn't match the endpoint |

## Persistence

The order books live in memory and are rebuilt on startup from a write-ahead log (`internal/wal`, default `./data/wal.log`):

- Every accepted order and cancel is appended to the log, fsynced, **before** it touches the book. Each record is the action exactly as the maker signed it (maker, side, size, price, nonce, expiry, signature), plus its order ID and the time it was applied.
- On startup, `recovery.Recover` replays the log through the same code as live requests, at each record's original time. Matching is deterministic, so the same records in the same order rebuild the same books.
- Replay also rebuilds the used nonces, so a signed order can't be resubmitted after a restart. The order ID counter resumes after the highest recovered ID.
- A half-written last line left by a crash is ignored. Corruption anywhere else stops startup.

Tampering with the log:

- **Without any key**, editing or inventing an order or cancel fails on replay, because each record's signature is verified again and only the maker's wallet can produce it. The server refuses to start.
- **Deleting, reordering or duplicating** validly signed records can't be caught by their signatures. Set `WAL_HMAC_KEY` (`openssl rand -hex 32`) to chain every record to the one before it with HMAC-SHA256; any break stops startup. Keep the key out of the WAL's directory and backups. Turning it on against an existing unsigned log fails by design, so start a fresh log when you enable it.
- Truncating the end of the log back to an earlier valid point can't be detected without an external checkpoint.

### Not done yet

Matches are not settled on-chain: there is no settlement contract yet, and the server doesn't check a maker's token balance or allowance before accepting an order. Prices are ticks, but there's no per-market tick size or lot size yet, so a tick has no defined value.

## License

MIT — see [`LICENSE`](LICENSE).
