# Scrape.do CLI — Phase 3 Build Log

Binary: cli-printing-press v4.20.0 · Run: 20260601-111749

## What was built

**Foundation (hand-authored, governor core):**
- `internal/store/scrapedo_extras.go` — governor + SERP-history schema and methods: cross-process concurrency lease (`concurrency_leases`), credit ledger (`credit_ledger` + `scrape_jobs`), `usage_snapshots`, `serp_snapshots` + `serp_organic`, `cost_budget`. Lease acquire/release with stale reclaim; ledger record + spend rollup by mode/agent; snapshot save + two-latest + organic readback; budget get/set.
- `internal/cli/governor.go` — the governed request path (`runGoverned`): cost-table resolution + domain overrides, spend-ceiling guard, cross-process lease (capped at live `ConcurrentRequest` from cached `/info`), retry on the non-billed 429/502/510 classes, ledger debit from the authoritative `Scrape.do-Request-Cost` header, verify-mode synthetic short-circuit, dogfood retry curtailment. `fetchInfo` (free `/info`).
- `internal/cli/serp_common.go` — SERP param-hash canonicalization, organic extraction, output emission, relative-time parsing.

**Commands (hand-authored):**
- `scrape <url>` — full core-endpoint surface (render/super/geo/regional-geo/device/markdown/return-json/screenshot/full-screenshot/session/wait-*/set-cookies/play/no-block-resources/no-retry/retry-timeout/no-redirect/transparent/target-timeout/out/agent-id/max-credits), governed.
- `google search <query>` — primary workflow; structured SERP JSON, cache-first (`--fresh` bypass), persists `serp_snapshots` + flattened `serp_organic`; `--agent --select` narrowing verified live.
- `google maps|news|shopping|flights|hotels|play|trends <query>` — governed verticals (flat 10 credits).
- `cost` — pre-flight credit estimator (no API call).
- `budget` (+ `budget set`) — ledger + spend attribution + burn forecast + live concurrency headroom; spend ceiling.
- `batch` — multi-agent safe fan-out (worker pool gated by the shared lease + ceiling + retry-on-free-codes).
- `drift <query>` — offline two-snapshot rank diff.
- `movers` — cross-query digest above a threshold.
- `sql <query>` — read-only SELECT over the local store (replaces the generator-omitted `search`/`analytics`).

**Tests (hand-authored, real assertions):**
- `internal/cli/governor_logic_test.go` — cost table (incl. domain overrides), param-hash normalization/sensitivity, organic extraction (+empty/malformed), drift diff (moved/new/dropped), since parsing.
- `internal/store/scrapedo_extras_test.go` — lease cap enforcement + release + stale reclaim + unlimited, ledger spend by mode/agent (unbilled excluded), snapshot round-trip + tracked hashes, single-snapshot-no-fabrication, budget set/get preservation.

All `go build ./...`, `go vet ./...`, and `go test ./internal/...` pass.

## Live validation (real API, ~22 credits used of 1000; 977 remaining)
- `scrape https://example.com` → cost=1 from header, logged to ledger (ok=1).
- `google search "best crm software" --agent --select organic_results.position,organic_results.title,organic_results.link` → correctly narrowed structured SERP; cost=10 (header); 11 organic rows persisted to `serp_organic`; snapshot stored.
- `budget` → live `/info`: remaining=977, cap=5, month spend ledger=12, by_mode {datacenter:2, google:10}.
- `cost` / `drift` (empty-state honest note) / `movers` (empty) / `sql` → all correct offline.

## Intentionally deferred (documented in absorb manifest; surfaced to user at hand-off)
- Amazon & YouTube Ready-API scrapers — exact `/plugin/...` paths not captured precisely in research; deferred rather than ship guessed/broken commands.
- Google Maps `place`/`reviews` sub-endpoints — need specific place_id/data_id the verifier can't synthesize.
- Real custom-HTTP-header forwarding (customHeaders/extraHeaders/forwardHeaders) — v1 ships `--set-cookies`.
- Async API (`q.scrape.do`, X-Token) — `batch` covers bulk synchronously.

## Generator limitations / retro candidates found
1. **One-line description truncation (v4.20.0):** SKILL/goreleaser/agent_context/MCP one-line descriptions cut the headline at the first of `.`/`,`/`:` AND a ~115-char cap. Brand names with a dot (`Scrape.do`) and any multi-clause headline get mangled. Worked around by authoring a single-clause, punctuation-free `headline`/`cli_description`. (`root.go` Short is immune — uses the full text.)
2. **Nested-subcommand `--help` exits 2 (v4.20.0 regression):** `google search --help`, and ALSO generated framework commands `auth status --help` / `profile list --help` / `account info --help`, all exit 2 while printing correct help. An older library binary (serpapi-google-search) exits 0 for the same `auth status --help`. Affects every v4.20.0-generated CLI, not this CLI specifically. Bad for agent UX (exit 2 reads as failure). Needs a generator fix in the root `Execute()` help-error classification.
3. Framework `search`/`analytics`/`tail` are omitted when the spec has no syncable list resource — expected, worked around with hand-built `sql` + `drift`/`movers`.
