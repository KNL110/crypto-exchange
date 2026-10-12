# CryptoExchange

A small crypto exchange written in Go: an in-memory limit order book matching engine, wrapped in an HTTP API with graceful shutdown. Users are wallet addresses: every order and cancel is an EIP-712 message signed by the user's wallet, and the server never holds user keys.

## Matching engine

The core matching logic lives in `internal/MatchEngine` (package `matchengine`) and knows nothing about HTTP or symbols — it's just an order book for one trading pair.

- **`Order`** — a single buy (`bid`) or sell (`ask`) order: maker address, remaining size, filled amount, side, expiry, timestamp, and an ID once placed through the exchange layer.
- **`Limit`** — one price level, holding a FIFO linked list of resting orders. Matching walks this list in insertion order, so fills at a price level are always oldest-order-first.
- **`OrderBook`** — both sides of the market for one symbol:
  - `PlaceLimitOrder(order, price)` first trades the order against the opposite side at every price level its limit allows (a buy against asks ≤ price, a sell against bids ≥ price), each fill at the resting order's price. Whatever is left rests at `price`. Returns the fills.
  - `PlaceMarketOrder(order, worstPrice)` fills the whole order immediately, best price first, never past `worstPrice`. If the book can't fill all of it within that bound, nothing is filled and it returns `ErrInsufficientLiquidity`.
  - Prices are whole numbers of ticks (`uint64`), never floats.
  - Expired resting orders are removed before matching, so they never trade.
  - `CancelOrder(id)` / `GetOrder(id)` remove/look up a resting order by ID.

Matching is deterministic: given the same sequence of place/cancel actions against an empty book, you always end up in the same state.

## Architecture

```text
internal/
  MatchEngine/   core order book + matching (above)
  config/        YAML config loading
  exchange/      owns one order book per symbol (ETH, BTC, LTC), assigns
                 order IDs, verifies EIP-712 signatures, HTTP handlers,
                 DTOs and routes
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

GET endpoints take query parameters. All request bodies must be sent as JSON with `Content-Type: application/json`.

| Method | Path | Body | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/healthz` | — | health check, lists configured symbols |
| `GET` | `/api/v1/exchange/orderbook?symbol=ETH` | — | order book snapshot for one symbol |
| `GET` | `/api/v1/exchange/orderbooks?symbols=ETH&symbols=BTC` | — | order book snapshots for several symbols |
| `POST` | `/api/v1/exchange/orders/limit` | signed order, `isMarket: false` | place a limit order (trades what crosses, rests the rest) |
| `POST` | `/api/v1/exchange/orders/market` | signed order, `isMarket: true` | place a market order (fills now, within the worst price) |
| `DELETE` | `/api/v1/exchange/orders/cancel` | signed cancel | cancel one of your resting orders |

Request/response shapes are in `internal/exchange/dto.go`.

### Signed orders

There are no accounts. A user is their wallet address, and every order and cancel must be signed by that wallet with [EIP-712](https://eips.ethereum.org/EIPS/eip-712) (`eth_signTypedData_v4`). The server recovers the signer and rejects the request unless it matches `maker`. Types are defined in `internal/exchange/signing.go`:

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

### Not done yet

Matches are not settled on-chain: there is no settlement contract yet, and the server doesn't check a maker's token balance or allowance before accepting an order. Prices are ticks, but there's no per-market tick size or lot size yet, so a tick has no defined value.

## License

MIT — see [`LICENSE`](LICENSE).
