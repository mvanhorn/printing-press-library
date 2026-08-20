# EverBee Absorb Manifest (v4.28 reprint, 2026-07-11)

## Source Tools

- **EverBee itself** (Keyword Research Suite, Product Analytics, Shop Analyzer, Tag Analytics, folders) — the only source of this data. Its own UI is the parity benchmark.
- **eRank**: keyword explorer, "Engagement Score", low-competition keyword workflow, listing audit, competitor tracking, AI listing generator.
- **Alura**: real-time listing SEO scoring, AI review analysis, market-gap surfacing.
- **EtsyHunt / Sale Samurai / Marmalead**: product DB filters, seed→tag generation, keyword clustering ("Storm"), trend forecasting.
- **Third-party EverBee API tooling: NONE.** Confirmed 2026-07-11 — no MCP server, no CLI, no npm/PyPI wrapper, no GitHub client. This CLI is the only programmatic surface for EverBee research data. (EverBee's own dev portal + Store API cover the *Store* commerce platform, not research data.)

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Seeded keyword research (volume, competition, score, CPC, trend) | EverBee Keyword Research Suite | `(generated endpoint) keyword-research search` | Uses the endpoint the UI search box actually calls (`POST /keyword_research/keyword_suggestion`), not the default feed. Scriptable, `--json`, comma-joined multi-seed, local history. |
| 2 | Seeded product research (sales, revenue, tags, price, listing type, age) | EverBee Product Analytics | `everbee-pp-cli products search --search-term "dad shirt"` | Uses `GET /product_analytics?search_term=` (the real search). Hand-authored rather than spec-emitted: the generator elects the shortest path as a resource's canonical sync target, so a GET search at the collection root would make bulk `sync` call it without its required term and 400. 68 fields/row exposed with `--select`. |
| 3 | Seeded shop / competitor search | EverBee Shop Analyzer | `(generated endpoint) shops search` | `GET /shops?search_term=` — the prior CLI never sent a search term. Diffable and joinable to products. |
| 4 | Shop handle resolution | EverBee Shop Analyzer autocomplete | `(generated endpoint) shops resolve` | `GET /shops/search?query=` — resolves a handle to shop_id/rating/reviews so an unresolved handle is never reported as "shop with zero evidence". |
| 5 | Trending / discovery browse feeds | EverBee default UI feeds | `(generated endpoint) keyword-research list`, `(generated endpoint) products` | The `default_*` feeds kept as an **explicit browse surface**, correctly labeled — never as the answer to a query. These are also the only genuinely bulk-syncable collections, so they back `sync`. |
| 6 | Result sort / pagination / time-range filters | all three EverBee tools | `(behavior in everbee-pp-cli keyword-research search)` order-by, order-direction, page, and time-range flags | Reproducible and batchable rather than click-driven. |
| 7 | Plan, quota, and usage reporting | EverBee account UI | `(generated endpoint) account` | Surfaces `current_plan` / `usage_details` so a plan cap (free tier = 10 keyword searches/month) is an honest error, never an empty result. |
| 8 | Local persistence, offline FTS + SQL | eRank/Alura exports (email-only CSV) | `(behavior in everbee-pp-cli sync)` persist keyword, product, shop, and folder responses to SQLite | Exports become queryable and diffable instead of emailed CSV. |
| 9 | Keyword clustering / related-term discovery | Marmalead Storm, Sale Samurai | `(behavior in everbee-pp-cli keyword-research search)` seeded suggestions are the cluster | EverBee's own suggestion engine, with relevance and evidence annotation on top. |
| 10 | Listing tag inspection | eRank listing audit, EtsyHunt | `(behavior in everbee-pp-cli products search)` `tags[]` ride on every product row | No separate endpoint exists upstream; tags are exposed directly and drive `research tags`. |
| 11 | Export | EverBee CSV export (emailed) | `(behavior in everbee-pp-cli products search)` `--csv` / `--json` / `--select` | Immediate local export in three formats, no email round-trip. |

## Transcendence (only possible with our approach)

Evidence layer is **annotate-only, never drop** (user decision): every returned row is emitted and stamped with token-aware relevance, evidence count, and provenance; `--min-relevance` lets the caller filter. Confidence is tied to evidence coverage — never >0.5 with zero keyword evidence, 0 when nothing relevant remains. Metrics EverBee did not return are never fabricated.

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Evidence-aware niche verdict | `research niche <seed>` | hand-code | Joins the seeded keyword POST and seeded product GET for one seed, then computes token-overlap relevance, evidence counts, price bands, saturation, and an opportunity score locally with a 15m live-first cache. No EverBee surface and no competitor tool returns a scored verdict with its own evidence attached. (#1492 F1/F2/F7/F8) | Use this command for a single-seed niche verdict with evidence. Do NOT use it to enumerate child niches; use 'research subniches' instead. For market-shape stats alone, use 'research competitors'. |
| 2 | Batch sub-niche discovery | `research subniches --parent <seed> --product <type>` | hand-code | Fans the niche pipeline over child seeds drawn from EverBee's own suggestion engine (comma-joined multi-seed POST confirmed by capture), applies product-type constraints via `listing_type` + SVG/PNG tag exclusion, and normalizes scores across the batch so children are comparable. (#1492 F9/F10/F14) | Use this command to rank many child niches under one parent. For a deep verdict on a single niche, use 'research niche' instead. |
| 3 | Competitor sampling | `research competitors <seed>` | hand-code | Computes result count, median price, review/sales density, and listing-age quartiles across seeded product rows plus a top-shop roll-up — local statistics over live rows, printed alongside the raw evidence. No Etsy research tool ships market-shape sampling. (#1492 F12) | Use this command for market-shape stats about who you would compete with. For a buy/skip verdict on the niche itself, use 'research niche' instead. |
| 4 | Tag/title consensus + seasonality | `research tags <seed>` | hand-code | Aggregates the `tags[]` array and title tokens across seeded product rows into consensus frequencies, and classifies seasonal-vs-evergreen from keyword `trend` variance against documented thresholds. Capture proved the UI's Tag Analytics tab fires no request — it is client-side derivation, which means we can do it too, scriptably. (#1492 F13) | Use this command for tag/title consensus and seasonality of a seed's listings. For the overall opportunity verdict, use 'research niche' instead. |
| 5 | Niche drift vs baseline | `research drift <seed>` | hand-code | Diffs a fresh niche run against a saved snapshot in the local research store, carrying both fetch timestamps and query scopes in provenance. EverBee exposes no history at all, so week-over-week movement is only knowable locally. (#1492 F14) | Use this command to compare a niche against a saved baseline over time. For a fresh point-in-time verdict, use 'research niche' instead. |
| 6 | Listing / handle identity resolution | `research listing <etsy-url-or-listing-id>` | hand-code | Parses a listing ID out of an Etsy URL and resolves it against the local product store, joining shop identity; handles resolve through `/shops/search`. Capture confirmed there is **no** upstream listing-detail endpoint, so honest local resolution — with an explicit resolved-identity-but-zero-evidence result — is the only truthful implementation. (#1492 F6/F11) | none |
| 7 | Semantic capability self-test | `selftest` | hand-code | Runs a canonical seed through the live seeded endpoints and asserts **semantic** validity, not just transport: relevance share of returned rows, presence of the `searched_keyword` block, and plan-cap detection, with typed exit codes and machine-readable JSON. This encodes the exact failure class that produced #1492. (#1492 F16) | none |

**Hand-code commitment: 7 of 7 transcendence features require hand-written Go after generate (~50-150 LoC each plus `root.go` wiring). 0 are spec-emitted.**

## Stubs

None. Every row above ships fully implemented. If an endpoint proves plan-gated or non-replayable during build/dogfood, return to this gate for revised approval rather than shipping a hidden stub.

## Dropped from brainstorm (available to reinstate at the gate)

`discover` (raw safe-GET probe — maintainer tooling, not a weekly command; #1492 F15 consciously dropped),
`quota` (duplicate of `account show`), `research priceband` (folded into F8 fields),
`shops listings` (accidental sync coverage would mislead), `keywords lowcomp` (contradicts annotate-only),
`research seasonal` (folded into `research tags`).

## Spec Corrections Made During Generation

- **`products search` is hand-authored, not spec-emitted.** The generator elects a resource's shortest endpoint path as its canonical bulk-sync target. `GET /product_analytics` (the seeded search) is shorter than the browse-feed path, so declaring it in the spec made bare `sync` call it without its required `search_term` and 400. Seeded search is query-scoped, not an enumerable collection, so it ships as a hand-written command and the browse feed backs `sync`. (Generator retro candidate: endpoint election ignores `required: true` params.)
- **`folders` dropped.** EverBee's `/folders` requires `?type=`, and bulk `sync` does not send endpoint param defaults, so folders was a guaranteed 400 on a bare `sync`. Folders are saved-UI groupings, not research data, and no novel feature depends on them.
- **Single-endpoint resources are promoted to bare commands.** The generator promotes a resource with one endpoint to a top-level command, so the browse feed is `products` (not `products list`) and the plan readout is `account` (not `account show`). Same capability, framework-conventional path.
- **`http_transport: standard` set.** `--spec-source browser-sniffed` made the generator ship browser-impersonation headers (`Accept: text/html`, `Sec-Fetch-Dest: document`); EverBee's Rails API then tried to render an HTML view for JSON endpoints and returned 500. `probe-reachability` classifies the API `standard_http` (0.95) — no bot protection.

## Risk Notes

- **The reprint's whole premise is an endpoint correction.** The published CLI read EverBee's *default feeds*; the app's search boxes call different endpoints. Seeded search scores 20/20 (keywords) and 16/20 (products) relevance on "dad shirt"; the default feed scores 0/20. Generation must not regress to the `default_*` paths for search.
- Revenue/sales/volume figures are EverBee **estimates**. Present as research signals, never as Etsy truth.
- Free "Hobby" plan caps keyword research at 10 searches/month. Batch commands must surface the cap as a typed error, never as empty evidence.
- Auth is a Google-SSO-minted `x-access-token`; it expires. `auth capture` + honest 401 messaging matter.
- #1492 F15 (raw safe-GET discovery) is intentionally **not** shipped — the brainstorm killed it as maintainer tooling. Flag at the gate if the user wants it back.
