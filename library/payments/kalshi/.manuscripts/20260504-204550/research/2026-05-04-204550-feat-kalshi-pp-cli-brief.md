# Kalshi CLI Brief (Reprint, REDO research)

## Reprint Context
- Prior CLI: shipped 2026-04-11 on Printing Press v1.2.2-dev, 89 typed MCP tools, 7 of 8 transcendence features built (Odds History dropped).
- Driver: machine-version delta (v1.2.2-dev → v3.9.0). Phase 0 user accepted regen.
- User architectural call-out: **Kalshi supports read-only and read/write API key tiers** — design must surface this so users (and agents) understand which mutations the loaded credential can perform.

## API Identity
- Domain: regulated CFTC-licensed event-contract exchange ("prediction markets"). Yes/no contracts on politics, weather, sports, economics, finance, crypto, AI milestones, etc.
- Server: `https://api.elections.kalshi.com/trade-api/v2` (single host; the elections-domain prefix is now the canonical Trade API host).
- Spec: OpenAPI 3.0.0, info.version `3.15.0`, 83 documented endpoints. Kalshi describes the spec as "manually defined endpoints being migrated to spec-first" — the runtime API has additional endpoints beyond what the spec covers.
- Users: retail traders, hobbyist quant/ML traders, journalists/researchers tracking event probability, AI agents executing market-data lookups.
- Data profile: medium-volume time-series. Markets number in the thousands; events/series in the hundreds. Per-account fills/orders/settlements typically <10k rows. Orderbook depth and price snapshots are the high-volume surface.

## Reachability Risk
- **Low.** Public API is reachable without auth for market/event/series/orderbook reads; the elections-domain probe responded HTTP 200 with valid OpenAPI on 2026-05-04. No bot-detection. No issues found in prior research mentioning 403/CF challenge.
- Auth-required endpoints (portfolio, orders, fills, balance) require RSA-PSS signed requests; verified prior CLI implements this correctly.

## Auth (composed signature, NOT plain API key)
Kalshi uses a 3-header composed signature:
- `KALSHI-ACCESS-KEY`: UUID access key id
- `KALSHI-ACCESS-TIMESTAMP`: current Unix-ms timestamp
- `KALSHI-ACCESS-SIGNATURE`: base64(RSA-PSS sign(timestamp + method + path, private_key))

Prior CLI: implementation in `internal/client/client.go` uses `crypto/rsa.SignPSS` correctly. Spec's three `securitySchemes` (`kalshiAccessKey`, `kalshiAccessSignature`, `kalshiAccessTimestamp`) describe the headers but generic generators interpret as 3 independent api keys. Map this to `auth.type: composed` in the internal spec, with a hand-authored signing helper. Env vars: `KALSHI_API_KEY` (id), `KALSHI_PRIVATE_KEY_PATH` or `KALSHI_PRIVATE_KEY` (PEM contents).

**Read-only vs read/write key tiers (USER VISION — must address):**
Kalshi issues two key flavors. Read-only keys can only call GET endpoints; read/write keys can place/cancel orders, transfer funds. The CLI must:
1. `doctor` should probe a known write endpoint with `--probe-only` (e.g., HEAD or dry-run order) and report detected scope, OR call an endpoint Kalshi exposes for key metadata if one exists.
2. `auth status` should display the detected scope ("Read-only key loaded — write commands will fail with 403").
3. Mutating commands should print a one-line warning when invoked with a read-only key, not just rely on the API to reject.
4. README and SKILL must describe the two tiers prominently in the auth section.

## Top Workflows
1. **Browse/discover markets** — by category (politics, sports, climate, economics, crypto), by event, by ticker; filter to currently-open markets, find low-liquidity opportunities.
2. **Track positions and P&L** — see open positions, settled positions, daily P&L, cash balance; reconcile fills against orders.
3. **Place and manage orders** — limit/market orders, batch orders, cancel-all by event/series, decrease-by-percent.
4. **Monitor price movements** — orderbook depth, recent trades, price progression on watched markets, "movers" since last sync.
5. **Analyze strategy** — win rate, ROI, attribution by category/series, exposure concentration warnings, settlement calendar.

## Table Stakes (from competitor catalog)
Every Kalshi tool worth absorbing offers these — all must be in our CLI:
- List markets / events / series with category and status filters
- Get market detail (current yes/no prices, volume, expiry)
- Get orderbook (depth on each side)
- Recent trades (a market's tape)
- Place order (buy/sell yes/no, limit/market)
- Cancel order, cancel-all
- List positions, fills, settlements
- Get cash balance
- Authenticated session that survives reboot (key-id + private key file)

## Codebase Intelligence
- **Source: prior CLI `internal/client/client.go` + Kalshi public OpenAPI v3.15.0**
- Auth: composed RSA-PSS signature; three `KALSHI-ACCESS-*` headers; `crypto/rsa.SignPSS` with PSSSaltLengthEqualsHash; SHA-256 digest of `${timestamp_ms}${method}${path}`.
- Data model: `series → events → markets → trades`. Markets have `yes_bid`, `yes_ask`, `no_bid`, `no_ask`, `last_price`, `previous_yes_bid`, `liquidity`, `volume`, `volume_24h`, `open_interest`, `expiration_time`, `status`. Positions are `{ticker, total_traded, position, market_exposure, realized_pnl, fees_paid}`. Fills are `{order_id, ticker, side, action, count, yes_price, no_price, is_taker, created_time}`.
- Rate limiting: Kalshi enforces per-account limits; `account/api/limits` endpoint returns the current quota. The `Get Account API Limits` operation is in the prior CLI as `account` command.
- Architecture insight: settlements arrive via separate endpoint (`/portfolio/settlements`) with cursor pagination; market price snapshots require periodic polling (no WebSocket in REST spec, though Kalshi has a separate WebSocket service).

## Data Layer
- Primary entities: markets, events, series, positions, fills, orders, settlements, trades, balance, exchange_status.
- Sync cursor: cursor-based pagination on `cursor` query param; most list endpoints return `cursor` in response payload.
- FTS/search: full-text on market `title` + `subtitle` + ticker prefix; events on `title`; series on `title` + ticker.
- Time-series snapshots: maintain a `market_price_history` table populated on each `markets` sync (this is what powers Movers, Correlate, History — and was the missing piece that made Odds History a stub-and-drop in the prior CLI).

## User Vision
- (User declined Briefing context-share, but flagged at API-key gate:) **Read-only vs read/write key tiers must be considered in the implementation.** This affects auth setup, doctor output, and per-command warnings.

## Product Thesis
- Name: `kalshi-pp-cli`
- Why: Kalshi has 14+ unofficial wrappers and 5+ MCP servers (4-5 launched in the last month), but none of them combine offline SQLite analytics with composed-signature auth that distinguishes read vs write keys. Power users who trade Kalshi need:
  1. A CLI that just works with their existing `~/.kalshi/private_key.pem` setup
  2. Local persistent storage so movers/correlate/history are computable without re-fetching the world
  3. Honest scope handling so a read-only key doesn't hit 403 surprises mid-script
  4. Agent-native output (`--json --select --compact`) so AI agents can drive trading workflows without burning tokens on schema parsing
- Differentiator vs the new MCP servers: they are MCP-only, mostly read-only, and don't persist data. Our CLI ships with both surfaces and a local store that compounds.

## Build Priorities
1. **Composed-signature auth** with proper PEM loading, both env-var and file-path support, scope detection (read-only vs read/write surfaced via doctor + auth status).
2. **Full data layer + sync** for markets, events, series, positions, fills, orders, settlements, trades. Snapshot table for market_price_history populated on every markets sync.
3. **All 83 endpoint-mirror commands** from the spec, with the MCP intent/orchestration pattern enabled (the spec's ~83 endpoints + ~13 framework commands + novel commands push us well over 50 — Cloudflare pattern is mandatory).
4. **8 transcendence features** (subagent will validate against current personas; prior 7 built + Odds History rebuilt with the snapshot table).
5. **Read/write tier UX**: doctor probe, auth status output, per-command scope check.

## Reachability + Anti-Reimplementation Notes
- All novel commands must call the real client or read from store. No hand-rolled response builders.
- Market price history is fed by the store population in the markets sync; Odds History reads from there. This is the carve-out for "novel command computes from local data" (`internal/store` access is allowed).

## Sources Referenced
- https://docs.kalshi.com/openapi.yaml
- https://github.com/9crusher/mcp-server-kalshi
- https://github.com/joinQuantish/kalshi-mcp
- https://github.com/kalashdotai/mcp
- https://github.com/JamesANZ/prediction-market-mcp
- https://github.com/shaanmajid/prediction-mcp
- https://github.com/yakub268/kalshi-mcp
- https://github.com/newyorkcompute/kalshi
- https://github.com/AndrewNolte/KalshiPythonClient
- https://github.com/ammario/kalshi
- https://github.com/fsctl/go-kalshi
- https://github.com/kurushdubash/kalshi-sdk
- https://github.com/humz2k/kalshi-python-unofficial
- https://github.com/OctagonAI/kalshi-deep-trading-bot
- https://github.com/austron24/kalshi-cli
- https://github.com/JThomasDevs/kalshi-cli
