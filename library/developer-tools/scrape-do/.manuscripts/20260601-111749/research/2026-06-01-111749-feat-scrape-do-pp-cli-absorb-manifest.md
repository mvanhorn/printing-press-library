# Scrape.do CLI — Absorb Manifest

First-mover: no existing Scrape.do CLI or MCP server. Absorb the HTTP API directly (official SDKs are thin); credit them in the README.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Scrape a URL → HTML/markdown | Scrape.do `GET /`, `@scrape-do/client` sendRequest | `scrape-do-pp-cli scrape` | governed (lease+ledger), `--json`/`--select`, per-call cost from header |
| 2 | JS render | `render=true` | `(behavior in scrape-do-pp-cli scrape) --render` | pre-flight cost warning (1→5 credits) |
| 3 | Residential/mobile proxy | `super=true` | `(behavior in scrape-do-pp-cli scrape) --super` | cost warning (→10/25 credits) |
| 4 | Country geo-targeting | `geoCode` | `(behavior in scrape-do-pp-cli scrape) --geo` | ISO code |
| 5 | Regional geo-targeting | `regionalGeoCode` | `(behavior in scrape-do-pp-cli scrape) --regional-geo` | enum region |
| 6 | Sticky proxy session | `sessionId` | `(behavior in scrape-do-pp-cli scrape) --session` | reuse same IP |
| 7 | Device emulation | `device` | `(behavior in scrape-do-pp-cli scrape) --device` | desktop/mobile/tablet |
| 8 | Markdown output | `output=markdown` | `(behavior in scrape-do-pp-cli scrape) --markdown` | agent-friendly text |
| 9 | Structured JSON output | `returnJSON` | `(behavior in scrape-do-pp-cli scrape) --return-json` | implies render |
| 10 | Screenshots | `screenShot`/`fullScreenShot` | `(behavior in scrape-do-pp-cli scrape) --screenshot/--full-screenshot` | write body to --out |
| 11 | Browser interaction script | `playWithBrowser` | `(behavior in scrape-do-pp-cli scrape) --play` | JSON action array (Click/Fill/Wait/Scroll/Execute) |
| 12 | Wait controls | `waitUntil`/`customWait`/`waitSelector` | `(behavior in scrape-do-pp-cli scrape) --wait-until/--wait/--wait-selector` | dynamic content |
| 13 | Cookies | `setCookies` | `(behavior in scrape-do-pp-cli scrape) --set-cookies` | session state |
| 14 | Block resources toggle | `blockResources` | `(behavior in scrape-do-pp-cli scrape) --no-block-resources` | default on for speed |
| 15 | Timeout / retry controls | `timeout`/`retryTimeout`/`disableRetry` | `(behavior in scrape-do-pp-cli scrape) --target-timeout/--retry-timeout/--no-retry` | validated bounds |
| 16 | Disable redirect | `disableRedirection` | `(behavior in scrape-do-pp-cli scrape) --no-redirect` | redirect-location header |
| 17 | Transparent response | `transparentResponse` | `(behavior in scrape-do-pp-cli scrape) --transparent` | passthrough target status |
| 18 | Google Search SERP | `/plugin/google/search`, SerpApi parity | `scrape-do-pp-cli google search` | flattened `serp_organic` + raw `serp_snapshots`, cache-first, governed |
| 19 | Google Maps (search) | `/plugin/google/maps/search` | `scrape-do-pp-cli google maps` | governed, JSON |
| 20 | Google News | `/plugin/google/news` | `scrape-do-pp-cli google news` | governed, JSON |
| 21 | Google Shopping | `/plugin/google/shopping` | `scrape-do-pp-cli google shopping` | governed, JSON |
| 22 | Google Flights | `/plugin/google/flights` | `scrape-do-pp-cli google flights` | governed, JSON |
| 23 | Google Hotels | `/plugin/google/hotels` | `scrape-do-pp-cli google hotels` | governed, JSON |
| 24 | Google Play | `/plugin/google/play` | `scrape-do-pp-cli google play` | governed, JSON |
| 25 | Google Trends | `/plugin/google/trends` | `scrape-do-pp-cli google trends` | governed, JSON |
| 26 | Account state (live) | `/info`, client.statistics() | `(generated endpoint) account info` | live concurrency + remaining-credit readout |
| 27 | Local sync of account state | framework | `(behavior in scrape-do-pp-cli sync) --resources` | refreshes `/info` into the local store |
| 28 | Query stored SERPs/ledger/jobs | hand-built | `scrape-do-pp-cli sql` | read-only SELECT over serp_organic / serp_snapshots / credit_ledger / scrape_jobs / usage_snapshots |
| 29 | SERP share-of-voice + spend analytics | hand-built | `(behavior in scrape-do-pp-cli sql) GROUP BY` | aggregation over the local store; spend attribution also via `budget` |
| 30 | Offline rank intelligence (replaces FTS) | hand-built | `scrape-do-pp-cli drift` | diff stored SERPs; `movers` gives the cross-query view |
| 31 | Health/auth check | framework | `scrape-do-pp-cli doctor` | env-var + reachability check |

**Generator-omitted commands (honest scoping):** The framework `search` (FTS), `analytics`, and `tail` commands are emitted only when the spec has a syncable *list* resource. Scrape.do has none (SERPs/scrapes are created by calls, not listed), so those commands are not generated. Their value is delivered by hand-built `sql` (ad-hoc querying incl. GROUP BY analytics) plus `drift`/`movers` (offline rank intelligence). The local SQLite store backs all of it.

**Deferred breadth (honest scoping; not stubbed, surfaced to the user at hand-off):**
- **Amazon & YouTube Ready-API scrapers** — the exact `/plugin/...` paths were not captured precisely in research; rather than ship guessed/broken commands, these are deferred pending a live-confirmed path probe.
- **Google Maps `place` / `reviews` sub-endpoints** — require specific `place_id` / `data_id` inputs that the verifier cannot synthesize; the primary `google maps` (search) ships now.
- **Real custom-HTTP-header forwarding** (`customHeaders` / `extraHeaders` / `forwardHeaders` with actual headers) — v1 ships `--set-cookies` (query param); full header forwarding needs a governed-client extension.
- **Async API** (`q.scrape.do`, `X-Token` header, separate concurrency pool) — second host + different auth mode; the `batch` command covers bulk scraping synchronously through the same governor.

The shipped surface (core `scrape` + the full Google family + the credit/concurrency governor) covers the user's stated primary workflow and the documented majority. The deferred items are tracked for a follow-up, not shipped as stubs.

## Transcendence (only possible with our approach)

The marquee capability is the **shared concurrency lease + local credit governor**: a SQLite-backed lease table that every billed command (`scrape`, `google search`, `batch`) acquires before firing, capped at the account's live `ConcurrentRequest`, so any number of independent AI-agent invocations of the CLI on one machine stay under the plan's concurrent cap and inside a credit ceiling. Spend is debited from the authoritative `Scrape.do-Request-Cost` response header. These behaviors are woven into the absorbed commands; the five commands below are the hand-code surfaces.

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|------------------------|
| 1 | Pre-flight cost estimator (incl. domain overrides) | `cost` | hand-code | Local credit-table lookup (1/5/10/25, google=10, LinkedIn=30…) prints expected credits with ZERO API spend before deciding — no Scrape.do tool estimates cost |
| 2 | Credit ledger + spend attribution + burn forecast | `budget` | hand-code | Cross-table join of `credit_ledger` (debited from the authoritative cost header) + cached `usage_snapshots`; attributes spend by agent/mode/query-family and forecasts burn vs days-left — the live counter can't |
| 3 | Multi-agent safe fan-out w/ shared concurrency lease + spend-ceiling guard | `batch` | hand-code | SQLite-backed shared lease caps N independent agent processes at the plan's `ConcurrentRequest`; retries only the non-billed 429/502/510 classes; `--max-credits`/`--max-monthly-pct` refuse dispatch past a ceiling — impossible without shared local state |
| 4 | SERP rank-drift diff (single query) | `drift` | hand-code | Diffs the two most recent `serp_snapshots` for a query+params-hash entirely offline — position deltas with no re-spend; needs local snapshot history |
| 5 | Cross-query movers digest | `movers` | hand-code | Scans every tracked query's latest-vs-prior snapshot and lists only the movers above a threshold — a local aggregation no single API call provides |

**Cross-cutting hand-code behaviors (built in Phase 3, surfaced via the commands above and absorbed scrape/google commands):**
- Shared concurrency lease (transcendence engine, score 9/10) → wraps `scrape`, `google search`, `batch`.
- Spend-ceiling guard `--max-credits`/`--max-monthly-pct` (7/10) → on dispatching commands.
- Cache-first dedupe with `--fresh` bypass (6/10) → on `scrape`, `google search`.

Minimum 5 transcendence commands met. All `hand-code`.
