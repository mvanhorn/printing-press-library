# Prospeo Absorb Manifest

## Sources Surveyed
- Official Prospeo HTTP API docs (https://prospeo.io/api-docs) — 8 active endpoints (post-March 2026 overhaul; old `email-finder`, `mobile-finder`, `social-url-enrichment` deprecated and removed).
- Official Prospeo MCP server (`mcp.prospeo.io`) — 5 tools mirroring the new endpoints.
- Meerkats-Ai/prospeo-mcp-server (GitHub) — Node MCP, 3 tools, uses deprecated endpoints (stale).
- The Crucible's existing `prospeo` skill (Python + `find_email.py`) — wraps deprecated email-finder with Trykitt waterfall.
- Adjacent best-practice CLIs: Hunter.io CLI, Apollo MCP, FindyMail, SmartLead CLI (this workspace's `smartlead` skill).

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Enrich person by LinkedIn URL | Official MCP `enrich_person` | `person enrich --linkedin <url>` | `--json`, `--select`, `--dry-run`, cached, `--show-cost` |
| 2 | Enrich person by name + company | Official MCP | `person enrich --first-name F --last-name L --company-website C` | Cache keys on canonical inputs; lifetime dupes return free hits |
| 3 | Enrich person by email | Official API `/enrich-person` | `person enrich --email <e>` | Composable with `cache` and `search-then-enrich` |
| 4 | Enrich person by person_id | Official API | `person enrich --person-id <id>` | Used by the search-then-enrich pipeline |
| 5 | Mobile finder (10-credit gate) | Official API `enrich_mobile: true` | `person enrich ... --mobile` (warns + requires `--yes` over $0.10 threshold) | Explicit opt-in flag prevents accidental 10x burn |
| 6 | Verified-only filtering | Official API | `--verified-only`, `--verified-mobile-only` | Maps cleanly to flags |
| 7 | Bulk enrich up to 50 persons | Official API `/bulk-enrich-person` | `person bulk --input leads.csv --output enriched.csv` | Auto-chunks N>50 rows; resumable on rate-limit; per-row caller_id from CSV row index; total_cost summary |
| 8 | Search persons with 30+ filters | Official MCP `search_person` | `person search --job-title <t> --seniority <s> --location <l> ...` | Stores page snapshots in SQLite for paginated reads without re-spending credits |
| 9 | Search pagination | Official API `page` param | `person search ... --page N` or `--all` (walks pages) | `--all` with credit cap |
| 10 | Enrich company by website | Official MCP `enrich_company` | `company enrich --website <d>` | Cached; surfaces tech stack, funding, employees |
| 11 | Enrich company by LinkedIn | Official API | `company enrich --linkedin <url>` |  |
| 12 | Enrich company by company_id | Official API | `company enrich --company-id <id>` | Used by search-then-enrich |
| 13 | Bulk enrich up to 50 companies | Official API | `company bulk --input companies.csv --output enriched.csv` | Auto-chunking; per-row caller_id |
| 14 | Search companies with industry/size/funding/tech | Official MCP `search_company` | `company search --industry X --employees-min N --tech-stack Y ...` |  |
| 15 | Search suggestions (location autocomplete) | Official API `/search-suggestions` | `suggest location <prefix>` | Free, fast, no credit |
| 16 | Search suggestions (job title autocomplete) | Official API | `suggest job-title <prefix>` |  |
| 17 | Account info / credit balance | Official MCP `get_account_info` | `account info` | Snapshots into `account_snapshots` for trend analysis |
| 18 | CSV input/output for bulk enrich | Existing Crucible `prospeo` skill | `--input CSV --output CSV` with flexible column names | Trykitt waterfall fallback (`--waterfall`) re-implemented as an optional second-pass enricher |
| 19 | Trykitt waterfall fallback | Existing Crucible skill | `person bulk ... --waterfall trykitt` | Optional, off by default; reads `TRYKITT_API_KEY` |
| 20 | Concurrency control for bulk | Existing skill `--concurrency 5` | `--concurrency N` (respects per-plan rate limits) |  |
| 21 | Email status surfacing (valid/not_found/api_error) | Existing skill output columns | Added as `email_status`, `email_confidence`, `email_source` columns in `--output` CSV | Matches existing skill's CSV contract for drop-in replacement |
| 22 | Doctor / health check | Standard | `doctor` | Auth check via `/account-information` + credit display |

22 features absorbed. Every feature from the official MCP (5/5), the official API docs (8/8 active endpoints), and the existing Crucible skill (drop-in replacement contract). The deprecated Meerkats-Ai wrapper is not absorbed — it uses removed endpoints.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Lifetime-dupe-aware cache | `cache hit <linkedin-url>` and implicit on every enrich | 9/10 | Prospeo gives `free_enrichment: true` for lifetime dupes — but only on the wire. We cache the canonical-input → response mapping so re-enrichments are zero RTT *and* zero credit, even on cold processes. |
| 2 | Credit pre-flight + max-cost guard | `person bulk --max-cost 50 --input leads.csv` | 9/10 | Before sending, count rows × per-row cost (1 or 10), check `account info` remaining_credits, and refuse if exceeding `--max-cost` or balance. Nothing exists for Prospeo today. |
| 3 | Credit burn timeline | `credits burn --days 30` | 8/10 | Joins `account_snapshots` (collected on every `account info`) with `enrichments` rows to plot daily burn, projected runway, and identify burn-spike workflows. Requires local state. |
| 4 | Lookalike from seed | `lookalike --seed-company stripe.com --employees-min 200 --employees-max 1000` | 8/10 | Auto-derives industry, size, tech stack, location from a seed `enrich-company` response and composes a `search-company` filter. One command, instead of three. |
| 5 | Search-then-enrich pipeline | `person funnel --filters search.json --max 25 --output funnel.csv` | 9/10 | One command runs `search-person` with paging, pipes each hit into `enrich-person`, caches everything, writes CSV. Today: requires 25+ HTTP calls plus glue. |
| 6 | Offline search over cache | `search "VP of Sales tech industry SF"` (FTS5 over cached persons/companies) | 8/10 | Once a contact is in cache, this is free and instant. Hunter, Apollo, FindyMail offer no offline lookup. |
| 7 | Dedup-aware CSV merge | `person bulk --input new.csv --merge existing.csv --output merged.csv` | 7/10 | Skips rows already present in `existing.csv` (matched by canonical key) so re-runs are cheap and incremental. |
| 8 | Bulk cost simulator | `person bulk --input leads.csv --dry-run` | 7/10 | Counts rows × per-row cost, shows projected total, lists which rows are cache hits (free) vs cold. No API call made. |
| 9 | Waterfall stats | `person bulk ... --waterfall trykitt --stats` | 6/10 | Prints find-rate per source (Prospeo X/N, Trykitt Y/M, none Z/K) so you can see whether the second source actually paid for itself. |
| 10 | Per-plan rate limiter | Built into every call | 7/10 | Reads `x-daily-request-left` / `x-minute-request-left` response headers; uses `cliutil.AdaptiveLimiter` to slow down before 429s rather than retry through them. |
| 11 | ICP fit scorer | `score --icp icp.yaml --input enriched.csv --output scored.csv` | 9/10 | Cross-source join: enriched rows × YAML rules (titles, sizes, geos, tech, seniority) → score column with reasons. CRM-grade ICP scoring in a CLI, no separate vendor needed. |
| 12 | Spend-attribution ledger | `ledger by-source` and `ledger by-campaign --tag <tag>` | 9/10 | Three-table SQLite join (enrichments × csv_jobs × account_snapshots) shows spend per CSV / per campaign tag. Prospeo's dashboard shows balance only — never which job burned what. |

12 transcendence features. All scored ≥6/10. Customer model: SDR/sales-ops running a list-build → enrich → SmartLead pipeline weekly. Their pain: burning credits on dupes; not knowing cost before a bulk run; manual search → manual enrich loop; no offline tool to "find that VP I enriched last month" without burning a credit. Every feature above directly resolves one of those.

## Coverage Summary
- 22 absorbed (full match of official MCP + API + existing Crucible skill)
- 12 transcendence features (≥6/10)
- 34 total commands

## Notes
- All POST bodies for `/enrich-person`, `/enrich-company` wrap the matching fields in a top-level `data: {...}` envelope per official docs. The generator emits flat body params; Phase 3 patches the client to wrap correctly. Bulk endpoints take an array shape that's hand-authored in Phase 3.
- Lookalike: Prospeo's "AI Search & Lookalikes" UI feature surfaces as ICP signal filters in `search-person`/`search-company` (Growth plan+). No standalone endpoint exists. Our `lookalike` command composes those filters automatically from a seed.
- Trykitt waterfall: the existing Crucible skill calls Trykitt as a fallback when Prospeo returns `not_found`. We carry the contract over so the Python script can be deprecated cleanly.
- **Cache backend = Supabase, not local SQLite.** The Crucible has a self-hosted Supabase at `upliftedconsulting.com` with the `outreach` schema (45,254 people, 94,439 companies) where `outreach.people.external_id` and `outreach.companies.external_id` store the Prospeo hash. Our cache, lifetime-dupe prediction, FTS, ICP scoring, ledger, and burn-timeline transcendence features all read/write Supabase via PostgREST (`SUPABASE_URL` + `SUPABASE_SERVICE_KEY`). Phase 3 adds three audit tables (`outreach.prospeo_enrichments`, `outreach.prospeo_account_snapshots`, `outreach.prospeo_csv_jobs`) via migration `0002_prospeo_audit.sql`. No local SQLite cache.
