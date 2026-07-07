# Google Play CLI — Absorb Manifest

## Scope
- Absorbed: 10 capabilities spanning the entire public-store surface, matching the best of facundoolano/google-play-scraper (JS), JoMingyu/google-play-scraper (Python), the `aso` package, mcp-appstore, and AppStoreCat.
- Novel on top: 7 transcendence features, all hand-coded, all powered by the local SQLite mirror (rank/keyword history, listing change detection) — the one thing every existing tool lacks because they are all stateless.
- Stacks up: matches every public-store method the reference scrapers expose, in a single maintained Go binary (the only Go port, n0madic, is 2.5 years stale), then adds the stateful intelligence layer that Sensor Tower / AppTweak / AppMagic paywall.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | App full detail (60+ fields, histogram, IAP range, exact realInstalls, dev contact) | npm `app` + PyPI realInstalls | `(behavior in google-play-pp-cli app)` hand-built GET+HTML ds:5 extractor | --json/--select/--compact, offline cache snapshot, exact install count |
| 2 | Top charts (TOP_FREE/TOP_PAID/GROSSING x 58 categories x country) | npm `list` (vyAe2) | `google-play-pp-cli top` | persisted snapshots enable history/movers; --json, --limit |
| 3 | Search (term, price filter, country/lang) | npm `search` (qnKhOb) | `google-play-pp-cli search-store` | agent JSON, --select; distinct from framework FTS `search` |
| 4 | Autocomplete suggest | npm `suggest` (IJ4APc) | `google-play-pp-cli suggest` | seed expansion for ASO, --json |
| 5 | Reviews (sort, star/device filter, dev replies, pagination) | npm + PyPI `reviews` (oCPfdb) | `google-play-pp-cli reviews` | honest pagination, persists to store for review-digest, --json |
| 6 | Similar apps | npm `similar` (ag2B9c) | `google-play-pp-cli similar` | competitive set, --json |
| 7 | Permissions (grouped) | npm `permissions` (xdSrCf) | `google-play-pp-cli permissions` | --json/--compact |
| 8 | Data safety (shared/collected/security practices) | npm `datasafety` | `google-play-pp-cli datasafety` | --json |
| 9 | Developer portfolio (name or numeric id) | npm `developer` | `google-play-pp-cli developer` | --json, --select |
| 10 | Categories enumeration | npm `categories` | `(generated endpoint) categories list` (html_extract links) | self-updating from live store nav |
| 11 | Local SQLite mirror + FTS | (our infra) | `(behavior in google-play-pp-cli sync)` + framework `search`/`sql` | offline queries, composability |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Chart rank history | `rank-history` | hand-code | Requires chart_snapshots persisted locally over time; Play shows only live state | Use for ONE app's rank trajectory over time within a chart. For ranking changes across the whole chart between two snapshots, use 'movers' instead. |
| 2 | Chart movers (climbers/droppers/new) | `movers` | hand-code | Requires diffing two local chart snapshots; no single API call gives deltas | Use for the whole-chart diff between two points in time. For a single app's trajectory, use 'rank-history' instead. |
| 3 | Listing change detection | `watch-listing` | hand-code | Requires field-by-field diff of two local app-detail snapshots over time | Use to see what changed on a tracked listing over time. For a current full field dump, use 'app'. |
| 4 | Keyword rank capture (live + persist) | `keyword-rank` | hand-code | Live search + writes a keyword_ranks snapshot nothing else keeps | Use to capture today's rank for a term (and persist it). For raw search results use 'search-store'; for the trend use 'keyword-history'. |
| 5 | Keyword rank history | `keyword-history` | hand-code | Requires keyword_ranks time-series in local SQLite | Use for the rank trend of one term/app/country over time. To capture a fresh point first, run 'keyword-rank'. |
| 6 | Review aggregation by version/star | `review-digest` | hand-code | Local aggregation over synced reviews (star/version histogram, reply-rate, token frequency); no NLP | Use for mechanical review stats. For raw reviews use 'reviews'; for prose summary pipe to an LLM. |
| 7 | Multi-app side-by-side | `compare` | hand-code | Agent-shaped transpose of N live detail fetches into one selectable table | Use to compare current details of multiple apps. For one app's full field set use 'app'; for change-over-time use 'watch-listing'. |

## Stubs
None. All shipping scope.

## Notes
- The entire Play client (`internal/gplay/`) is hand-built: batchexecute envelope + `)]}'` framing, AF_initDataCallback ds-key extraction with fallback index paths, ~1 req/s throttle, exponential backoff on 429/503, and `PlayGatewayError`-in-200 detection as a typed `*cliutil.RateLimitError`.
- Hand-code count: 7 transcendence + ~9 absorbed live commands (app, top, search-store, suggest, reviews, similar, permissions, datasafety, developer) = 16 hand-built commands. Only `categories list` is generator-emitted.
