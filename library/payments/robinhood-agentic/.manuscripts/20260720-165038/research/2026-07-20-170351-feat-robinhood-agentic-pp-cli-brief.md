# Robinhood Agentic Trading MCP CLI Brief

## API Identity
- Domain: Retail brokerage — Robinhood's official **Agentic Trading MCP server** (`https://agent.robinhood.com/mcp/trading`), launched in beta 2026-05-27 as the first agent-native surface from a major US retail broker. This is the sanctioned, ToS-compliant API for programmatic Robinhood access — unlike the reverse-engineered `api.robinhood.com` private API every prior community tool depends on.
- Users: (1) Robinhood retail traders who script/automate their workflow; (2) AI-agent operators who connected Claude/ChatGPT/Cursor to the MCP and want the same surface in shell scripts, cron jobs, and CI; (3) quant-curious power users who left robin_stocks when Robinhood's login hardening broke it.
- Data profile: accounts, portfolio + P&L history, equity/option positions, orders (with review-before-place preflight), real-time + historical quotes, fundamentals, technical indicators, earnings, index data, watchlists, and server-side market scanners. Reads span ALL of a user's accounts; writes (order placement) are hard-scoped by Robinhood to a dedicated Agentic account the user funds separately.
- Protocol: MCP (JSON-RPC 2.0 over streamable HTTP). `tools/call` per operation; `Mcp-Session-Id` session header; responses as JSON or SSE frames.

## Reachability Risk
- **None.** Probe on 2026-07-20: `POST /mcp/trading` → clean 401 + `WWW-Authenticate: Bearer resource_metadata=...` (standard MCP OAuth discovery). `/.well-known/oauth-protected-resource/mcp/trading` and path-based AS metadata both resolve. This is an official, CDN-fronted (CloudFront), envoy-gatewayed service — the opposite of the 403-riddled private API that community wrappers fight.
- Auth: OAuth2 authorization_code + PKCE (S256), **dynamic client registration** (RFC 7591) at `https://agent.robinhood.com/oauth/trading/register`, authorize at `https://robinhood.com/oauth`, token at `https://api.robinhood.com/oauth2/token/`, public client (`token_endpoint_auth_methods_supported: ["none"]`), `scopes_supported: ["internal"]`, refresh_token grant supported.

## Top Workflows
1. **Morning portfolio check** — positions, day/total P&L, open orders, watchlist movers in one shot (today: 4+ app screens or 4+ MCP tool calls through a chat agent).
2. **Order preflight → place → track** — `review_equity_order` before `place_equity_order`, then poll order status. The review tool is a server-side dry-run: the CLI maps `--dry-run` onto it.
3. **Screen → watch → act** — run server-side scanners (`run_scan`), promote hits to a watchlist, quote them, size a position.
4. **Options chain triage** — chains → filter by expiry/strike/greeks → quotes → (review) order.
5. **P&L bookkeeping** — realized P&L and trade history exports for taxes/journaling (`get_realized_pnl`, `get_pnl_trade_history`).

## Table Stakes
- **The full live tool surface is 49-50 tools, not the 44 the support article lists.** Authenticated `tools/list` exports (rinebob/rel-str 2026-07-17, 49 tools; a 2026-07-18 listTools returned 50) add `get_equity_price_book` (Level-2, ≤4 symbols), `get_financials` (≤20 symbols/40 periods), `get_equity_tax_lots` (specified-lot sells, ≤30 lots/order), `get_scanner_filter_specs` (runtime scan-DSL discovery), and `get_option_level_upgrade_info`. Every one of these becomes a typed command.
- Typed order builders with review-first flow (`review_*_order` ⇒ our `--dry-run`) — the existing library print only offers raw `--body-json`, and Schwab's community MCP proved the preview→place-by-reference pattern stops LLM parameter drift.
- Agent-native output floor from the library's finance prints (kalshi, prediction-goat house style): `--agent`, `--select`, `--compact`, `--csv`, provenance envelope, `which`, `doctor`, typed exit codes, profiles, `--deliver` sinks.
- Multi-account rollups (reads span all accounts; verygoodplugins #12/#21 show demand), cursor pagination helpers with loop guards, per-call symbol-limit batching (20 quotes / 10 fundamentals / 4 price-book) handled transparently.
- Local SQLite store + FTS + sync (positions, orders, P&L, quotes snapshots, watchlists, scans) — the "response slimming" demand (verygoodplugins #11/#13) and Alpaca's context-bloat issue (#45) both validate offline-first.
- Known live-schema gotchas encoded, not rediscovered: TIF must be `gfd`/`gtc` (`day` rejected), `get_realized_pnl` requires `asset_classes` despite docs, P&L tools take `rhs_account_number`, historicals tool answers to `get_equity-historicals` (hyphen), `get_indexes` takes a comma-separated string while `get_index_quotes` wants UUID arrays, `dollar_amount` XOR `quantity` (market orders only), cancel returns `{accepted}` acknowledgement that can race a fill.
- MCP result envelope: `{"data": {...}, "guide": "..."}` via structuredContent or TextContent-JSON — transport must unwrap all three shapes (robin-sdk pattern).

## Data Layer
- Primary entities: accounts, positions (equity + option), orders (equity + option), quotes snapshots, historicals (OHLCV), fundamentals, earnings, watchlists + items, scans + scan results, realized P&L lots, portfolio snapshots (time series for drift/trends).
- Sync cursor: orders + P&L trade history by updated/executed timestamp; positions/portfolio are snapshot-on-sync (append-only local history — the MCP itself has no time-series portfolio endpoint, which is our transcendence seam).
- FTS/search: symbols, instrument names, watchlist names, scan names, order notes.

## Codebase Intelligence
- Source: direct OAuth metadata probes (this run) + MCP docs. The MCP server is Robinhood-internal (no public source). Envelope: JSON-RPC 2.0 `tools/call`; session via `Mcp-Session-Id` response header on `initialize`; SSE (`text/event-stream`) possible on responses — transport must parse both.
- Auth: Bearer access token from the PKCE flow; refresh at the same token endpoint. Client is public; client_id obtained per-install via RFC 7591 dynamic registration (no shipped client secret — cardinal-rule clean).
- Rate limiting: not documented; treat 429 via adaptive limiter (cliutil.AdaptiveLimiter) and surface typed RateLimitError.
- Architecture: single MCP endpoint; all operations are tool calls — the generated REST client is bridged by an MCP transport shim (Phase 3) that maps virtual `/tools/<name>` endpoints to `tools/call` JSON-RPC with typed argument coercion.

## User Vision
- Kevin (goal briefing): "Robinhood now has an MCP!! Let's build this into a printing press CLI… run through all the printing press steps, validate it works, and don't stop until it's developed to a state where it will pass the printing press review. I have authenticated the [Robinhood] MCP… **do not test with any real transfers.**"
- Implications: (1) the CLI targets the official agentic MCP, not the private web API; (2) live validation is strictly read-only — no order placement, no cancels, no money movement during testing; write commands ship gated (env + flag + review-first dry-run default, matching the library's existing robinhood print convention); (3) ship quality must clear the printing-press-library review bar (greptile + verify-library-conventions CI).

## Source Priority
- Single source: the official Agentic Trading MCP. (The Banking/credit-card MCP from the same announcement is documented as out of scope — different product, different risk surface; noted in README's non-goals.)

## Product Thesis
- Name: `robinhood-agentic` (binary `robinhood-agentic-pp-cli`, display name "Robinhood Agentic Trading")
- Why it should exist: Every existing Robinhood tool — including the library's own `robinhood` print — rides the reverse-engineered private API with browser-lifted bearer tokens that Robinhood actively breaks (robin_stocks login issues are a genre). This CLI is the first built on the **official, OAuth-sanctioned agent surface**: stable auth (PKCE + refresh + dynamic registration, no token theft), server-side order preflight as a true dry-run, server-side scanners no wrapper exposes, and an offline SQLite layer (portfolio time series, P&L journal, order audit log) that neither the MCP nor the chat agents using it can provide. It's also the reference pattern for wrapping ANY remote MCP server as a printing-press CLI.

## Build Priorities
1. MCP transport + OAuth (register→login→refresh→doctor) — nothing works without it; it's also the novel infrastructure.
2. Read surface absorbed completely (accounts, portfolio, positions, orders, quotes, historicals, fundamentals, indicators, earnings, indexes, watchlists, scans) with --json/--select/--csv agent-native output.
3. Local store + sync: portfolio snapshots, orders, positions, P&L → FTS + SQL.
4. Safety-gated write surface: review-first order commands (--dry-run ⇒ review_* tool; live placement requires env gate + explicit flag), watchlist/scan management.
5. Transcendence: the offline/journaling/drift features from the approved manifest.
