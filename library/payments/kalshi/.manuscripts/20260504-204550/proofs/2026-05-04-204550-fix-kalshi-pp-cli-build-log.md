# Kalshi Phase 3 Build Log

## What was built

### Auth (critical fix-up)
- Replaced generator-emitted `KALSHI_TRADE_MANUAL_KALSHI_ACCESS_KEY` with conventional `KALSHI_API_KEY` env var
- Added `KALSHI_PRIVATE_KEY_PATH` and `KALSHI_PRIVATE_KEY` env var support with PKCS8/PKCS1 PEM parser
- Implemented RSA-PSS composed-signature signing (`signKalshiRequest()`) emitting all three KALSHI-ACCESS-* headers per request
- Added `KALSHI_ENV=demo` shortcut that swaps base URL to demo-api.kalshi.co
- Verified live: doctor reports auth ok; `portfolio get-balance --json` returned `{"balance":1000,"portfolio_value":0}`

### Read-only safe mode (NEW novel feature)
- Added `KALSHI_READ_ONLY=1` env var honored at config load
- Added `--read-only` global persistent flag
- `client.do()` returns `ErrReadOnlyMode` for any POST/PUT/PATCH/DELETE before signing or network IO
- Verified: `--read-only portfolio cancel-order test-id` errors client-side with "client is in read-only mode"

### Transcendence commands ported from prior CLI (8)
1. `portfolio attribution` — P&L by category × series × time window
2. `portfolio winrate` — W/L ratio + EV + ROI across settled positions
3. `portfolio calendar` — upcoming settlement dates with positions
4. `portfolio exposure` — risk by category with concentration warnings
5. `markets movers` — biggest price changes since last sync (uses Kalshi-returned `previous_yes_bid`)
6. `markets correlate` — Pearson correlation of two markets' price histories
7. `markets heatmap` — category-level activity aggregation
8. `events lifecycle` — event progression from creation through settlement

### Transcendence commands new (3 — were not in prior CLI)
1. `markets history <ticker>` — REFRAMED. Prior dropped because no snapshot table; this round adds `market_price_history` table populated by every `markets` sync. Best-effort INSERT OR IGNORE keyed on (ticker, snapshot_ts).
2. `subaccounts rollup` — aggregate positions/balance across subaccounts (reads local store; degrades gracefully when subaccounts not synced)
3. `watch` group (`add`, `list`, `remove`, `diff`) — local watchlist with snapshot deltas joined to market_price_history

### Schema migrations
- `market_price_history` table + `idx_market_price_history_ticker` index
- `watchlist` table

### Per Phase 1.5 reprint verdict
- `portfolio stale` deliberately NOT registered (subagent dropped per "Settlement Calendar covers it"). Constructor remains in transcendence.go as ported code; unregistered.

## Verify-friendly RunE compliance
- All ported commands had `cobra.ExactArgs(1)` / `cobra.ExactArgs(2)` replaced with `len(args)` checks falling through to `cmd.Help()` (per AGENTS.md "verify-friendly RunE" rule).

## MCP read-only annotations
- 8 ported read-only commands + `markets history` + `subaccounts rollup` + `watch list` + `watch diff` annotated `mcp:read-only: true`.
- `watch add` / `watch remove` deliberately unannotated (write to local store).

## Narrative validation
- 9 of 10 commands resolved on first pass; one recipe used `--price` (doesn't exist) — fixed to `--yes-price 50 --action buy`.
- Final: 10/10 OK.

## Generator notes (retro candidates)
1. **Pre-gen MCP enrichment** for OpenAPI specs is unsupported — `x-mcp-transport`, `x-mcp-orchestration`, `x-mcp-endpoint-tools` extensions don't exist in the OpenAPI parser. 96-tool surface generated as endpoint-mirror with a generator warning to add `mcp:` block to the spec, but the parser only honors the block in internal YAML format. Memory feedback flagged this; documenting again. Suggested machine fix: parse `x-mcp-*` extensions or root-level `mcp:` YAML in OpenAPI source files.

2. **Composed signature auth (RSA-PSS) is unsupported** — the generator's `composed` auth type targets cookie-template formats only. RSA-PSS-signed APIs (Kalshi, AWS Sigv4, etc.) require hand-port of signing code. Suggested machine fix: introduce an `auth.type: signed` family with `format: rsa-pss-sha256` (and Sigv4, HMAC-SHA256, etc.) variants emitting per-request signing helpers.

3. **Spec-title pollution in env-var names** — Kalshi's spec title "Kalshi Trade API Manual Endpoints" propagated into `KALSHI_TRADE_MANUAL_KALSHI_ACCESS_KEY`. Convention is `<API_SLUG>_API_KEY`. Suggested machine fix: when spec slug exactly equals one of the security scheme name prefixes (case-insensitive), collapse to `<SLUG>_<KEY-FIELD>` form.

4. **Ugly resource name from path collision** — `/search/filters_by_sport` and `/search/tags_by_categories` collided with the framework `search` command and were renamed to `kalshi-trade-manual-search` and `kalshi-trade-manual-search-2`. Suggested fix: when a generated resource name shadows a framework command, prefer renaming to `search-filters` / `search-tags` (last path segment) over prefixing with the spec title.

## Build status
- `go build ./...` — pass
- `kalshi-pp-cli --help` — pass (shows all 11 novel commands in Highlights block)
- 13 novel commands verified runnable via `--help`

## Deferred to Phase 4
- Run dogfood, verify, workflow-verify, verify-skill, scorecard
- Address scorecard MCP-architecture dimensions (expected weak per memory feedback_polish_mcp_misclassify; can't fix in polish)
