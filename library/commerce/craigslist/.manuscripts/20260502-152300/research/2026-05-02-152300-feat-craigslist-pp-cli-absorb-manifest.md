# Craigslist Absorb Manifest

## Tools Considered

| Tool | Type | Notes |
|------|------|-------|
| juliomalegria/python-craigslist | Python lib | Per-category classes, filters, `show_filters()`. Broken since CL went JS-rendered + sapi moved. |
| irahorecka/pycraigslist | Python lib | Module-per-subcategory, `search()` + `search_detail()` with geo. HTML-parser. |
| node-craigslist | Node lib | Promise-based search; HTML scraper. Stale. |
| imclab/craigslist | Node lib | Generic scraper. Stale. |
| ecnepsnai/craigslist | Go lib | Small. |
| craigslist-automation (3d-logic) | Node lib | Puppeteer-based; heavyweight. |
| sa7mon/craigsfeed | Go server | RSS feed gen — dead since CL killed RSS. |
| meub/craigslist-for-sale-alerts | Slack bot | One-off alerts. |
| jgnickerson/craigslist-alert | Email bot | One-off alerts. |
| CPlus / CPlus Classifieds | iOS/Android app | Multi-city, saved searches with push, photo grid, map, posting/renew, multi-account. Sells PII. Paywall after 2 saved searches. |
| SearchTempest / AdHunt'r / searchallcraigslist.org | Web aggregators | Cross-city search; ad-supported. |
| Apify Craigslist scrapers | Hosted SaaS | Paid scraping with MCP exposure. |
| Visualping / PageCrawl.io | Visual diff watchers | Price-drop / new-listing alerts. Paid. |
| craigwatch (assorted scripts) | CLI poll-and-diff | Lean on RSS (broken). |

**Open lane:** No open-source standalone Craigslist MCP server. No free, scriptable, no-PII-selling alternative to CPlus + Visualping.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|--------------------|-------------|
| 1 | Search by city + category + keyword query | python-craigslist, pycraigslist, CPlus | `search "<query>" --site sfbay --category sss` calls `sapi.craigslist.org/web/v8/postings/search/full`, decodes positional items | --json, --csv, --select, MCP-typed |
| 2 | Price range filter (min/max) | all | `--min-price`, `--max-price` mapped to sapi `min_price`/`max_price` | Composable with cross-city, sortable |
| 3 | Has-image filter | all | `--has-pic` mapped to sapi `hasPic=1` | — |
| 4 | Posted-today / posted-since filter | most | `--posted-today`, `--posted-since 24h` | --posted-since accepts duration syntax (`24h`, `3d`, `7d`) |
| 5 | Distance-from-postal filter | pycraigslist, CPlus | `--postal 94110 --distance-mi 25` mapped to sapi `postal`/`search_distance` | — |
| 6 | Title-only search | CL native, python-craigslist | `--title-only` mapped to sapi `srchType=T` | — |
| 7 | By-owner / by-dealer filter | pycraigslist (sub-paths) | `--owner-only`, `--dealer-only` choose category abbr (e.g., `cto` vs `ctd`) | — |
| 8 | Listing detail fetch | all | `listing get <uuid>` calls `rapi.craigslist.org/web/v8/postings/<uuid>?lang=en` | --json, MCP-typed, attributes flattened |
| 9 | Listing detail by integer post ID | most | `listing get-by-pid <int>` resolves PID → UUID via search lookup | — |
| 10 | Show filters available per category | python-craigslist `show_filters`, pycraigslist | `filters show <category>` reads filter schema from the sapi `filters` block | --json output for agents |
| 11 | Pagination | all | `--page N` maps to sapi `batch=<page>-0-360-0-0` | --limit caps result size |
| 12 | Sort by posted-date | CPlus | `--sort posted` maps to sapi `sort=date` | --sort price also supported |
| 13 | Categories list | pycraigslist, CL reference | `categories list [--type J\|H\|S\|...]` from `reference.craigslist.org/Categories` (178 entries) | --type filter, table+json output |
| 14 | Areas / cities list | pycraigslist, CL reference | `areas list [--country US]` from `reference.craigslist.org/Areas` (707 entries) | --country, --region filters; subareas flattened |
| 15 | Cross-city / multi-region search | CPlus, SearchTempest, AdHunt'r | `search ... --sites sfbay,nyc,chicago` fans out parallel sapi calls, source-attributes results | Bounded concurrency, per-source rate limits, source-tagged JSON |
| 16 | Saved searches (persistent) | CPlus, craigwatch | `watch save <name> --query ... --sites ... --category ... --filters ...`, `watch list`, `watch show <name>`, `watch delete <name>` | Local SQLite — no PII selling, no paywall |
| 17 | New-listing alerts | CPlus, Visualping, PageCrawl | `watch run <name>` polls all configured sites, diffs against `seen_listing`, emits `[NEW] / [PRICE-DROP] / [REPOST] / [DUP]` | Free, scriptable, MCP-exposed |
| 18 | Favorites with notes | CPlus | `favorite add <pid> [--note "..."]`, `favorite list`, `favorite remove <pid>` | --json, queryable via `sql` |
| 19 | Map view / geo coordinates | CPlus | `geo bbox` and `geo within` filter local store by GeoCoordinates from rapi/sapi | --json output for downstream map tools |
| 20 | Sync to local store | (none — novel for CLIs) | `sync --site sfbay --category sss [--since <date>]` paginates sapi, persists into `listing` + `listing_snapshot` tables | Foundation for every transcendence feature |
| 21 | Listing image URLs | all | `listing images <uuid>` lists CDN URLs from `images.craigslist.org/...` | --size 600 / 1200 / etc. |
| 22 | Reference catalog refresh | (none) | `catalog refresh` re-pulls Categories + Areas from reference.craigslist.org | TTL aligned with CL's 30-day Cache-Control |

**Stub-or-omit (deferred for v1):**

| # | Feature | Reason |
|---|---------|--------|
| 23 | Posting / renew / edit / delete | (stub — v1 is read-only; write path requires `accounts.craigslist.org` cookie session and is the most-monitored CL surface; documented as future work) |
| 24 | Anonymous reply via mailto relay | (stub — same reason; `/reply` is robots.txt-disallowed) |
| 25 | Multi-account management | (stub — depends on posting flow) |

## Transcendence (only possible with our approach)

User-first personas (from the brief):

- **A** — Apartment hunter in a competitive market: checks CL several times a day for new 1BR-under-$X; first responder gets the place.
- **B** — Deal-hunting reseller / collector: watches across all 700+ cities for specific items (vintage synths, Eames furniture, Apple silicon at fire-sale prices).
- **C** — Job seeker in a niche: software roles, NOT senior, NYC, posted last 7 days, NOT recruiter posts.
- **D** — Suspicious housing-scam reader: sees a too-good listing, wants to verify whether the same images/text appear in other cities before showing up.
- **E** — AI agent / power scripter: wants typed MCP tools for "median price for X in city Y last week" and "stream new listings as JSON-line".

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| T1 | **Smart watch with diff types** — emits NEW / PRICE-DROP / REPOST / DUP, not just "anything I haven't seen" | `watch run <name>` | 10/10 | After fetching latest sapi results for a saved search, joins against `seen_listing` (per-watch) + `listing_snapshot` (price/title history) + `duplicate_cluster` (cross-city) and tags each result. Persona A, B, C. | HN id=24840310 (RSS removed); CPlus paywalls after 2 saved searches; Visualping/PageCrawl monetize this; native CL email alerts include edits/reposts as "new" |
| T2 | **Streaming watch tail** — long-running process emits new diff events as JSON lines | `watch tail <name> --interval 5m` | 7/10 | Calls `watch run` in a loop, jittered, prints one JSON event per `[NEW]/[PRICE-DROP]/[REPOST]/[DUP]` to stdout. Persona A, E. | Alerts persona; Phase 5 dogfood-friendly (poll once mode for tests). Side-effect command — emits print-by-default with `--launch hook` for browser/Slack integration |
| T3 | **Price drift history** — show the price timeline for a listing across all snapshots | `drift <pid>` | 9/10 | Joins `listing_snapshot` rows for the post ID, sorts by observed_at, prints chart-friendly table. Persona A, B. | Visualping/PageCrawl monetize price-drop alerts; sellers reduce prices on stale listings as documented behavior |
| T4 | **Cross-city duplicate clusters** — group listings whose body+image fingerprints match | `dupe-cluster [--min-cluster-size 3]` | 10/10 | SimHash over body_text + average-hash over thumbnail bytes; cluster on hamming distance ≤ threshold; emit cluster summaries. Persona D. | Housing scam pain explicitly documented (web801 quote in brief); cross-city dupes are a known scam signal |
| T5 | **Scam score** — heuristic 0-100 over a listing using rule-based signals | `scam-score <pid>` | 10/10 | Pure-logic scorer: rules include `body_age < 24h`, `price < median * 0.5`, `mentions wire/zelle/cashapp`, `dupe-cluster size > 2`, `seller_email_outside_relay`, `body has Western Union pattern`. Persona D. | Housing scam pain quoted; rental scams documented; CL native has no scam rating |
| T6 | **Median price across the local market** — p25/p50/p75 per query, per category, per city, optionally over time window | `median <query> --category sss [--since 30d] [--by-city]` | 8/10 | SQL aggregation over `listing` + `listing_snapshot`; outputs table or JSON. Persona B, C. | Resellers persona; CL native has no aggregation; no existing tool offers this |
| T7 | **Negative-keyword smart search** — search with NOT terms CL itself doesn't support | `search "1BR" --negate furnished,sublet,studio --category apa` | 8/10 | Initial fan-out to sapi with the positive query, then local FTS5 NOT filter against `listing.body_text`. Persona A, C. | CL search is keyword-only — pycraigslist README explicitly notes lack of negation; FTS5 has built-in NOT |
| T8 | **One-shot "what's new since"** — ad-hoc diff without setting up a saved search | `since 24h --site sfbay --category sss [--query ipad]` | 8/10 | Reads from `/sitemap/date/<dates>/cat/<cat>/sitemap.xml` for each day in the window, fetches any new listing detail not in store, outputs new ones. Persona B, E. | "What hit while I was sleeping" persona pattern; sitemap-by-date is the RSS replacement |
| T9 | **Reposts radar** — find posts that have been reposted N times in last X days | `reposts <query> --min-reposts 3 --window 30d` | 8/10 | Cluster `listing_snapshot` rows by body_hash; count distinct posted_at days; threshold filter. Persona B (motivated seller signal) and D (spam flooder signal). | HN: native alerts repeat reposts; reseller persona |
| T10 | **City-by-city heat map** — across cities, count new listings in a category in a window | `cities heat --category sss --since 24h` | 5/10 | Reads sitemap-by-date counts per (city, category) without fetching detail. Outputs ranked table. Persona B. | Resellers persona; CL native has no view of this |

(Feature T1 + T2 form a pair; T1 is the once-per-call diff; T2 is the long-running tail.)

## Reprint Reconciliation

First print of `craigslist-pp-cli` — no prior research.json. Skipped.

## Build Order

The Phase 1.5 manifest above is the build list. Phase 3 must implement every absorbed feature (1-22) plus every transcendence feature scored ≥ 5/10 (T1-T10). Stubbed items (23-25) are intentionally documented as out-of-scope-v1 in the README, with clear rationale (write path requires browser cookie session against the most-monitored CL surface).

Total: **22 absorbed + 10 transcendence = 32 features.**

That is the GOAT bar for this CLI.
