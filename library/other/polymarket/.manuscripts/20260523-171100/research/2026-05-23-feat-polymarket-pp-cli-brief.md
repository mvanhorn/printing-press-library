# Polymarket CLI Brief

## API Identity

- **Domain:** Prediction markets / decentralized betting on real-world events (politics, sports, crypto, news). Largest such platform globally as of 2026, on Polygon mainnet.
- **Users:** Retail traders, market makers, arbitrageurs, AI/algo traders, researchers tracking public sentiment, news/journalism teams citing market-implied odds.
- **Data profile:** Three REST surfaces + one WebSocket + on-chain Polygon contracts.
  - **Gamma API** (`https://gamma-api.polymarket.com`) — read-only, **no auth**. Markets, events, tags, series, comments, sports, search, profiles. The discovery API.
  - **CLOB API** (`https://clob.polymarket.com`) — order book, prices, midpoint, spread, price history (public reads). Order placement / cancel / list, trades, balances, API key management (authenticated writes).
  - **Data API** (`https://data-api.polymarket.com`) — positions with valuation, portfolio value, user activity feed (TRADE / SPLIT / MERGE / REDEEM / REWARD / CONVERSION), market holders, liquidity rewards data.
  - **WebSocket** (`wss://ws-subscriptions-clob.polymarket.com/ws/market` and `/ws/user`) — live book + user feed. Hard limit 5 concurrent connections per IP.
  - **On-chain** (Polygon, chain ID 137) — USDC.e collateral, CTF (Conditional Token Framework) ERC-1155 outcome tokens, proxy wallet (Safe-based) for browser/email login flows, EOA for direct wallet keys. Contracts: approve once (6 tx), then `ctf split` / `merge` / `redeem` for position lifecycle.

## Reachability Risk

**HIGH — Cloudflare WAF actively blocks datacenter IPs.**

Evidence:
- [Cloudflare community: WAF blocking Supabase Edge Functions](https://community.cloudflare.com/t/cloudflare-waf-blocking-legitimate-api-requests-from-supabase-edge-functions-to-pol/869437) — 30–50% intermittent block rate.
- [py-clob-client #91](https://github.com/Polymarket/py-clob-client/issues/91) — Cloudflare block when creating API key credentials.
- [poly-market-maker #72](https://github.com/Polymarket/poly-market-maker/issues/72) — Cloudflare block on server-to-server.
- [Fly.io community thread](https://community.fly.io/t/cloudflare-403-blocks-when-accessing-third-party-apis/26631) — datacenter IPs broadly blocked.

**Mitigation strategy for the CLI:**
- Ship browser-shaped headers by default (Chrome 132 UA, `Accept-Language`, `Sec-Fetch-*`, realistic `Referer: https://polymarket.com/`).
- Add `--surf` flag that switches to a Surf-style TLS-fingerprinting transport for users running from cloud/datacenter IPs (Vercel, Fly, AWS).
- Surface `doctor` that distinguishes "API down" vs "you got Cloudflare-blocked, switch transports."
- Rate limit headers (`x-ratelimit-*`) are present; respect them in an adaptive limiter.
- For *runtime probe* during research: this sandbox's HTTP returned HTTP 000 (network failure, not a Polymarket-level signal). Plan the printed CLI to ship Surf transport and document the cloud-IP gotcha in README troubleshooting.

## Top Workflows

1. **Discover a market and check current odds** — search "Trump 2024" → see implied probability + 24h volume + liquidity → drill into order book and recent trades. Done with no auth, no wallet.
2. **Track a portfolio** — given a wallet address, list current positions, mark them to current price, compute realized + unrealized P&L, surface upcoming resolutions.
3. **Place a limit order on a market outcome** — sign with EOA, derive L2 API key once, post a GTC bid. Cancel single / cancel-by-market / cancel-all.
4. **Claim winnings (redeem)** — after a market resolves, call CTF `redeem` to swap winning outcome tokens for USDC. Repeat across N positions.
5. **Run a market-maker / arbitrage strategy** — stream WebSocket order book, compute mid + spread cross-venue (vs Kalshi etc.), place rebated maker orders, monitor rewards/scoring.
6. **Research-grade data pull** — sync all active markets + events + price history for offline analysis (correlations, base-rate calibration, news-driven moves).

## Table Stakes (must match what competing tools do)

The official `polymarket-cli` (Rust, by Polymarket itself) defines the floor. Our CLI MUST match every command:

**Discovery (Gamma):** `markets list/search/get`, `events list/get/tags`, `series list/get`, `tags list`, `comments`, `profile lookup`.

**CLOB reads:** order book (`book`), midpoint, price, spread, price history, server time, neg-risk markets.

**CLOB writes (require L1 PK + L2 HMAC):** `create-order` (GTC/GTD/FOK/FAK), `market-order`, `cancel <id>`, `cancel-orders [ids]`, `cancel-market`, `cancel-all`, `orders`, `order <id>`, `trades`, `balance`.

**API key mgmt:** `api-keys`, `create-api-key`, `delete-api-key`, `derive-api-key` (the L1→L2 bootstrap).

**Rewards:** `rewards`, `earnings`, `earnings-markets`, `reward-percentages`, `current-rewards`, `market-reward`, `order-scoring`, `orders-scoring`.

**On-chain:** `approve set` (6 tx batch), `ctf split`, `ctf merge`, `ctf redeem`, `wallet create/import/show/address/reset`, balance queries.

**Data API:** `data positions <addr>`, `data value <addr>`, `data activity <addr> --type {trade|split|merge|redeem|reward|conversion}`, `data holders <market>`.

**Meta:** `doctor` / `status`, `setup` (guided wizard), shell REPL, JSON output everywhere.

## Data Layer

Primary entities to persist in local SQLite + FTS5:

- **markets** (id, condition_id, question, slug, active, closed, accepting_orders, end_date, volume, liquidity, outcomePrices JSON, clobTokenIds JSON, tags, eventId, last_synced_at)
- **events** (id, ticker, slug, title, description, active, closed, start/end, tags, market_ids, volume, liquidity)
- **tags** (id, label, slug, force_show, published_at)
- **series** (id, ticker, slug, title, recurrence)
- **token_outcomes** (token_id, market_id, outcome_label "Yes"/"No"/etc, current_price, current_size)
- **trades** (tx_hash, market_id, token_id, side, size, price, ts, taker, maker)
- **orders** (id, status, market_id, token_id, side, type, price, size_remaining, expiration, signer, owner)
- **positions** (user, market_id, token_id, size, avg_price, current_price, current_value, realized_pnl, unrealized_pnl, last_snapshot_ts)
- **activity** (tx_hash, user, type, market_id, token_id, side, size, price, value, ts)
- **rewards** (date, market_id, asset_addr, score, payout)
- **api_keys** (key, secret_redacted, passphrase_redacted, derived_at, signer_addr)
- **snapshots** (entity_type, entity_id, snapshot_ts, json_blob) — for time-series of price/volume/positions, powers novel features.

FTS5 over `markets.question + description + tags` and `events.title + description`. Sync cursor = `last_synced_at` per entity, plus `next_cursor` token from Gamma's pagination.

## Codebase Intelligence

Synthesized from Polymarket/py-clob-client-v2, Polymarket/polymarket-cli (Rust, official), Polymarket/agents (Python), and 4 popular MCP server repos (guangxiang, IQAI, CarlosIbCu, pab1it0).

- **Auth model:** Three levels.
  - L0 = no auth (Gamma reads + most CLOB reads).
  - L1 = EIP-712 wallet sig with the user's Polygon EOA private key. Used to derive L2 creds (`POST /auth/api-key`) and to sign every order before submission.
  - L2 = HMAC-SHA256 over (timestamp + method + path + body) using `api_secret`, with `POLY_ADDRESS`, `POLY_SIGNATURE`, `POLY_TIMESTAMP`, `POLY_API_KEY`, `POLY_PASSPHRASE` headers.
- **Signature types:** `0` = EOA (MetaMask, hardware, raw PK), `1` = Magic email proxy, `2` = browser-wallet proxy. When `>0`, a `funder` (proxy wallet address holding USDC) is mandatory.
- **Data model:** Markets nest under Events nest under Series; markets have `clobTokenIds` (the two ERC-1155 outcome tokens), `outcomePrices` (mid for each), `clobRewards` (liquidity reward config). `outcomePrices` and `clobTokenIds` are stringified JSON arrays inside the JSON response — must double-parse.
- **Rate limiting:** Public 60/min. CLOB general 9,000/10s. POST /order 3,500/10s burst + 36,000/10min sustained (~60/s average). All exposed via `x-ratelimit-limit` / `-remaining` / `-reset` headers. WebSocket capped at 5 concurrent per IP.
- **Architecture:** Off-chain matching engine (CLOB) + on-chain settlement (CTF on Polygon). Orders are EIP-712 signed off-chain, matched off-chain, then settled in batches on-chain via the `Exchange` contract. USDC moves between user proxy wallet ↔ exchange contract. Resolution is via on-chain reporter posting to the CTF; users then call `redeem` to convert winning tokens to USDC.

## User Vision

User said (Indonesian): "I'll use a wallet myself, you map out all the functions and set up the env vars, I'll fill in the PK later." Implications captured at [briefing-context.md](../briefing-context.md):

- Build the **full trading surface** — not stubs. Place / cancel / replace, balance, positions, activity, redeem.
- Wire **L1 (wallet PK) auth path** end-to-end so the user can drop in `POLYMARKET_PRIVATE_KEY` and `auth derive` bootstraps the L2 creds.
- Env vars: `POLYMARKET_PRIVATE_KEY`, `POLYMARKET_API_KEY`, `POLYMARKET_API_SECRET`, `POLYMARKET_API_PASSPHRASE`, `POLYMARKET_FUNDER` (optional, only for proxy wallets), `POLYMARKET_SIGNATURE_TYPE` (default 0 = EOA), `POLYMARKET_CHAIN_ID` (default 137).
- `doctor` must clearly explain "no PK = read-only, PK only = orders + auto-derive L2, PK + L2 = full trading + history."

## Product Thesis

- **Name:** `polymarket-pp-cli` (binary), `polymarket` (library directory).
- **Why it should exist:**
  1. The official Polymarket Rust CLI exists but is missing key research/analytics primitives that quant traders actually want. We absorb every official command AND beat it with offline SQLite + FTS5 + agent-native JSON + cross-venue context.
  2. The 10+ existing MCP servers each pick a subset. We mirror every Cobra command into MCP (Cloudflare-pattern code orchestration) so an agent gets the full surface at ~1K tokens.
  3. No tool currently does **portfolio drift detection, resolution radar (markets resolving in N hours), reward-yield ranking across markets, or whale-tracking across activity feeds**. Those are agentic-first questions only possible with a local snapshot store.
  4. No tool currently distinguishes "API down" from "Cloudflare blocked your IP" — our `doctor` does.

## Build Priorities

1. **Priority 0 (foundation):** spec-driven generation of Gamma + CLOB + Data + WebSocket endpoints, SQLite store for the 11 entities above, `sync` with FTS5, `search`, `sql`, `doctor` with auth-tier validation, two-tier (L1+L2) auth + key derivation flow, Surf-style transport fallback for cloud IPs.
2. **Priority 1 (absorb the entire competing landscape):** every command from the official Rust CLI + every tool from IQAI + guangxiang MCPs. Discovery, order book, trades, positions, balance, place/cancel/all-cancel, CTF split/merge/redeem, approvals, rewards, API key CRUD, REPL mode, status/setup. Roughly 50–60 leaf commands.
3. **Priority 2 (transcend, agent-decided in Phase 1.5c.5):** see absorb manifest. Candidates: portfolio drift, resolution radar, reward-yield ranking, whale activity feed, market depth aggregator, cross-venue (Kalshi) implied-prob diff, P&L attribution, news-bound shock detector.

## Source Citations

- [Polymarket Documentation index](https://docs.polymarket.com/api-reference/introduction) (intercepted by ISP; verified via mirrors)
- [Polymarket/polymarket-cli (Rust, official)](https://github.com/Polymarket/polymarket-cli) — full README indexed
- [Polymarket/py-clob-client-v2](https://github.com/Polymarket/py-clob-client-v2) — auth + order patterns
- [Polymarket/py-clob-client (archived)](https://github.com/Polymarket/py-clob-client) — historical reference
- [Polymarket/agents](https://github.com/Polymarket/agents) — Gamma client source code indexed
- [IQAIcom/mcp-polymarket](https://github.com/IQAIcom/mcp-polymarket) — 19 MCP tool inventory
- [guangxiangdebizi/PolyMarket-MCP](https://github.com/guangxiangdebizi/PolyMarket-MCP) — 8 high-level analytics tools
- [Rate Limits — Polymarket Docs](https://docs.polymarket.com/api-reference/rate-limits)
- [AgentBets.ai — Polymarket Rate Limits Guide March 2026](https://agentbets.ai/guides/polymarket-rate-limits-guide/)
- [QuantVPS — Polymarket no longer geoblocked in US (2026)](https://www.quantvps.com/blog/polymarket-us-api-available)
