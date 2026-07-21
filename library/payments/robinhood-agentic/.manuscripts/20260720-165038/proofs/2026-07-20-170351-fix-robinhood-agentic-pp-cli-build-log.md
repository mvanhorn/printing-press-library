# Build Log — robinhood-agentic-pp-cli (Phase 3)

## What was built

**Priority 0 — foundation (generator + hand-written seam):**
- Generated data layer (accounts, equities, market, options, portfolio, scans, watchlists tables + FTS + generic `resources`), sync, search, SQL, doctor, `which`, profiles, `--deliver`, provenance envelope, typed exit codes, `--agent`/`--select`/`--compact`/`--csv`.
- **MCP transport bridge** (`internal/client/mcp_transport.go`, hand-written, ~450 lines): the correctness core. Robinhood's Agentic surface is one JSON-RPC/MCP endpoint, not REST. The generated client's REST-shaped requests (`GET/POST /tools/<name>`) are intercepted by an `http.RoundTripper` that:
  - performs the one-time MCP `initialize` + `notifications/initialized` handshake, caching `Mcp-Session-Id`;
  - rewrites each request into `tools/call` with per-tool argument coercion (`toolArgSpecs`) — array-splitting `symbols`/`instrument_ids`/`option_ids`, int/float/bool coercion, and the `get_indexes.symbols`-stays-a-string exception that motivates the per-tool table;
  - folds flat single-leg option-order fields into the `legs[]` array the MCP requires;
  - parses JSON **and** SSE (`text/event-stream`) response frames;
  - normalizes all three MCP result shapes (structuredContent / structuredContent.data / content[].text-JSON) into the `{data, guide}` envelope the generated `response_path` extraction expects;
  - passes through untouched in verify/dogfood mock mode so spec-shaped mock responses flow normally.
  OAuth (login/refresh via generated `auth.go` — PKCE + dynamic client registration), dry-run, retry/backoff, rate limiting, and response_path extraction all stay in the generated code.

**Priority 1 — absorbed surface (49 typed endpoint commands):** every live MCP tool as a typed command across accounts / portfolio / market / equities / options / watchlists / scans, including the four undocumented-in-support-docs tools surfaced from authenticated `tools/list` exports (price book, financials, tax lots, scanner filter specs) and option upgrade info.

**Mutation safety (hand-written, centralized at the transport chokepoint):**
- `internal/client` exposes `MutationGuard`/`MutationJournal` package hooks; `internal/cli/mutation_safety.go` installs them at init.
- Three layers: (1) **write gate** — no mutating tool call reaches the network unless `ROBINHOOD_AGENTIC_PP_ALLOW_WRITES=1` (the hard floor that makes read-only testing safe by construction); (2) **guard policy** — order placement additionally checked against per-order cap, daily cap, allow/denylist, kill switch; (3) **journal** — every mutation attempt + outcome recorded locally.
- `review_*` (server-side sims) and `run_scan` are correctly classified read-only and never gated.

**Priority 2 — transcendence (8 features, all shipped, zero stubs):**
1. `portfolio history [--sparkline --since]` — local append-only portfolio time series (API has none).
2. `guard set|status` — client-side trade policy (the only enforceable limit layer given the all-or-nothing OAuth scope).
3. `equities settle <id> [--wait]` — polls an order to verified terminal truth past the cancel `{accepted}` race and the null-until-backfilled market price.
4. `brief [--agent]` — one-command pre-open check joining portfolio + orders + positions + day-over-day snapshot delta; records a fresh snapshot each run.
5. `audit [--since --denied --placed --tool]` — queries the local write journal.
6. `portfolio winrate [--by-symbol --span]` — round-trip win rate aggregated locally from trade history.
7. `surface capture` / `surface diff` — snapshots MCP `tools/list` and diffs consecutive surfaces (the beta surface churns without notice).
8. `wheel status [SYMBOL]` — infers wheel stage by joining option orders × equity positions × tax lots (no assignment/exercise tool exists).

## Store layer (hand-written, `internal/store/agentic.go`)
New tables created lazily (CREATE TABLE IF NOT EXISTS, regen-safe): `portfolio_snapshots`, `write_journal`, `tool_surface_snapshots`, `guard_policy`. Accessors + a pure `GuardPolicy.EvaluateOrder`.

## Tests
Pure-logic table-driven tests for every hand-written package: transport (tool-name parse, arg coercion incl. the get_indexes exception, option-leg fold, mutating-tool set, result normalization across all 3 shapes, SSE parse), store (guard eval, journal round-trip, snapshot round-trip, notional parse), cli (winrate aggregation, wheel-stage inference across all branches, settle terminal-state, audit filter, guard flag apply, brief delta/default-account/open-order state, mutation notional/action/symbol, surface diff). `go build ./...`, `go vet ./...`, `go test ./...` all green.

## Intentionally deferred / scope notes
- Multi-leg option orders: the MCP itself is single-leg only (documented boundary), so the option order builder is single-leg.
- `orders place --from-review <id>` (preview-fingerprint reuse): the shipped safety model is review-command + hard env gate + guard policy; the from-review reference is documented as the review-first workflow rather than a flag on the generated place command (avoids editing generator-owned command files).
- Crypto, ACH transfers, dividends feed, banking/card MCP: not exposed by the Agentic Trading MCP — documented as non-goals.

## Generator limitations found (for retro)
- Novel-feature scaffolds annotated two read-only commands (`wheel status`, `equities settle`) as `mcp:read-only: false`; corrected by hand.
- No generator support for an MCP/JSON-RPC transport target — the REST→MCP bridge is entirely hand-written. A `--client-pattern mcp-jsonrpc` would be a high-value generator feature (Robinhood won't be the last official MCP).
