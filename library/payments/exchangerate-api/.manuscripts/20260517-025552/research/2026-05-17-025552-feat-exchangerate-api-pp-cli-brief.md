# ExchangeRate-API CLI Brief

## API Identity
- Domain: Currency conversion and live FX rates (161 currencies, ISO 4217)
- Users: Developers, fintech apps, e-commerce price localizers, accounting tools, AI agents that need real-time FX context
- Data profile: Small, fast-refreshing JSON. Two hosts: `https://v6.exchangerate-api.com/v6/{KEY}/...` (paid + free key) and `https://open.er-api.com/v6/...` (no-key, 24h refresh, attribution required)

## Reachability Risk
- **None.** `probe-reachability` returns `standard_http` (confidence 0.95, stdlib 200 OK). Live key probes succeed for `/codes`, `/latest/{BASE}`, `/pair/{BASE}/{TARGET}`, `/quota`. Paid endpoints (`/enriched`, `/history`) return HTTP 403 `plan-upgrade-required` — documented expected behavior for Free tier, not a reachability issue.

## Endpoints (Authoritative)

| Endpoint | Path | Tier | Notes |
|----------|------|------|-------|
| Standard | `GET /v6/{KEY}/latest/{BASE}` | Free+ | Returns `conversion_rates` map of code→rate |
| Pair | `GET /v6/{KEY}/pair/{BASE}/{TARGET}[/{AMOUNT}]` | Free+ | Single rate; optional amount adds `conversion_result` |
| Enriched | `GET /v6/{KEY}/enriched/{BASE}/{TARGET}` | Business+ | Adds `target_data` (locale, name, flag URL, symbol codepoint) |
| Historical | `GET /v6/{KEY}/history/{BASE}/{YEAR}/{MONTH}/{DAY}[/{AMOUNT}]` | Pro+ | Date back to 1990; amount switches `conversion_rates` → `conversion_amounts` |
| Supported Codes | `GET /v6/{KEY}/codes` | Free+ | Returns `[["USD","US Dollar"],...]` pair array |
| Quota | `GET /v6/{KEY}/quota` | Free+ | Counts against quota; 5-60 min reporting lag |
| Open Access | `GET https://open.er-api.com/v6/latest/{BASE}` | None | No key, 24h refresh, 429 on hammer, 20-min cooldown |

Errors: `unsupported-code`, `malformed-request`, `invalid-key`, `inactive-account`, `quota-reached`, `plan-upgrade-required`, `no-data-available` (historical). Carried in `"error-type"` field on `result: "error"` responses. Open-access endpoint also includes `time_eol_unix` for API deprecation.

## Top Workflows
1. **"What's USD→EUR right now?"** — quick spot check, copy-paste rate into a doc/message
2. **"Convert $250 to JPY"** — amount conversion; CLI returns one number, agent prints it
3. **"Pull all rates for USD as JSON"** — feeding downstream scripts, dashboards, agents
4. **"How much quota do I have left?"** — engineers tracking burn rate
5. **"Did EUR/USD move > 1% since yesterday?"** — drift detection for monitoring, alerting
6. **"Show me the 30-day chart of GBP/JPY"** — historical analysis (paid tier) or local-snapshot reconstruction (free tier, novel)

## Table Stakes (every competitor has these)
- Single pair lookup (`er USD EUR`)
- Amount conversion (`er USD 100 EUR`)
- Multi-target conversion (`er USD 100 EUR,GBP,JPY`)
- Plain-text and JSON output
- Currency code lookup / search
- Default base from config

## Data Layer
- **codes** — currency metadata (code, name, optional locale/flag/symbol from enriched); seed once, refresh on demand. FTS on name.
- **rates_snapshots** — every `/latest` sync writes one row per (base, target, rate, captured_at, source_endpoint). Append-only. Enables historical reconstruction from free-tier captures.
- **pair_history** — derived from rates_snapshots for fast pair queries.
- **quota_snapshots** — every `/quota` sync writes one row; lets users see quota burn over time.
- **watchlist** — user-defined (base, target, threshold_pct, last_alerted_at, last_known_rate); enables drift alerts.
- **conversions_log** — every conversion the user runs (base, target, amount, result, captured_at); enables "what did I convert?", recurring conversion analysis, agent context.

## Codebase Intelligence
- No DeepWiki target — the API has no major SDK wrapper repo with > 100 stars. The space is fragmented: small PowerShell wrapper (markwragg/PowerShell-CurrencyConverter), npm `@ivanvr/exchangerate-api-wrapper` (~6 stars), several Python toys.
- MCP servers: `cahthuranag/realtime-exchange-rate-mcp` (uses AllRatesToday, not us), `wesbos/currency-conversion-mcp` (uses Frankfurter, not us). **No mature public MCP server uses ExchangeRate-API.** Whitespace.
- Closest CLI: `TimothyYe/exchangerate` (Go, 33 stars) — uses currencyconverterapi.com, not us. Shows the shape: `er USD`, `er USD 40.98`, `er USD 12 CNY,JPY`. We absorb this UX and beat it with local storage + agent native + MCP.

## Product Thesis
- **Name:** `exchangerate-api-pp-cli` (Printing Press convention; canonical brand: "ExchangeRate-API")
- **Why it should exist:**
  1. The space has *no* CLI specifically for ExchangeRate-API with offline persistence, agent-native output, or MCP exposure
  2. Free-tier users can't access historical endpoints — but they CAN sync `/latest` daily and build their own local history. No other tool does this.
  3. AI agents need FX context in their tools palette. There's no good agent-shaped FX surface; toy MCPs exist but none combines the auth, local cache, and intent-based MCP tools an agent actually wants.
  4. Quota-aware design: 1500 free requests/month is real; a CLI that caches aggressively and warns at thresholds is uniquely valuable.

## User Vision
- User passed the API key as `$EXCHANGERATE_API_KEY` and chose Codex mode. They want a working CLI that can be used immediately for live FX queries and that ships clean. No further constraints stated.

## Build Priorities
1. **Foundation:** SQLite store (codes, rates_snapshots, quota_snapshots, watchlist, conversions_log), client with API key auth via `EXCHANGERATE_API_KEY`, generic JSON/select/csv output.
2. **Absorbed (P1):** All 7 typed endpoints (latest, pair, pair-with-amount, enriched, historical, codes, quota), open-access fallback, `convert <amount> <from> <to>` ergonomic wrapper, multi-target conversion, codes search, doctor.
3. **Transcendence (P2):** quota-aware caching/sync, local history reconstruction from free-tier syncs, watchlist + drift detection, multi-base compare matrix, agent-native MCP via cobratree, time-traveling cache reads, "burn rate" projections.
