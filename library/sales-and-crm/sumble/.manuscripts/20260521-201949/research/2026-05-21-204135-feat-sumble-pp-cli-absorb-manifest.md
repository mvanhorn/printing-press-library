# Sumble CLI — Absorb Manifest

API: Sumble v6 (`https://api.sumble.com/v6`). Auth: `Authorization: Bearer $SUMBLE_API_KEY`.
Ecosystem surveyed: official Sumble MCP server (~25 tools, hosted at mcp.sumble.com), local Python lead-gen skill, web app (CSV export + saved lists + cost preview), bare REST. **No community SDK/CLI exists** (only 3 zero-star hobby repos). So "absorb" = match the full REST surface + MCP-only tools + web-app conveniences, then transcend on credit economy.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Find organizations by tech/category/since/query | v6 organizations/find + MCP | generated `organizations find` | cache-first, --json/--select/--csv, cost-estimate, budget guard |
| 2 | Enrich org tech stack | organizations/enrich | generated `organizations enrich` | cached so re-enrich is free locally |
| 3 | Match orgs from name/url/location | organizations/match | generated `organizations match` | the cheap (1cr) resolution path; feeds `reconcile` |
| 4 | AI org intelligence brief (50cr, async 202) | organizations/intelligence-brief | generated `organizations intelligence-brief` | budget-gated (50cr is expensive) |
| 5 | Find people at org by function/level/country | people/find | generated `people find` | structured + free-text query, cached |
| 6 | Find related people (org chart above/below) | people/find-related-people | generated `people find-related-people` | cached relations |
| 7 | Enrich person email (10cr) / phone (80cr) | people/enrich | generated `people enrich` | budget guard + confirm before the 80cr phone reveal |
| 8 | Find jobs by tech/category/country | jobs/find | generated `jobs find` | order-by jobs_count_growth_6mo for intent |
| 9 | Get job detail | jobs/{id} | generated `jobs get` | cached |
| 10 | Find people related to a job | jobs/find-related-people | generated `jobs find-related-people` | cached |
| 11 | Search technologies | technologies/find | generated `technologies find` | offline re-search over cached tech once synced |
| 12 | Org-lists: list / get / create / add | organization-lists | generated `organization-lists` commands | writes are free; surfaced as such |
| 13 | Contact-lists: list / get / create / add | contact-lists | generated `contact-lists` commands | writes free |
| 14 | Ad-hoc SQL over data | MCP Query/ListTables (MCP-only, no REST) | generated `sql` over local SQLite | **beats MCP: offline, zero credits, full SQL** |
| 15 | Offline full-text search | n/a | generated `search` | cross-entity FTS, zero credits |
| 16 | Sync to local store | n/a | generated `sync` | populates cache → every later read is free |
| 17 | Account / balance info | MCP GetAccountInformation (MCP-only, no REST) | transcendence `balance` (below) | **beats MCP: derived from ledger, no MCP dependency** |
| 18 | Lead-list CSV export | local Python skill + web app | `--csv`/`--output` on find commands + cached dedup | JSON/JSONL too; dedup the Python skill lacks |

No stubs. Every absorbed row is either a generated endpoint command or a generated framework command (sql/search/sync).

## Transcendence (only possible with our approach) — all credit-economy themed

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|--------------------------|
| 1 | Pre-call cost estimate / dry-run | `cost-estimate <cmd> [args]` (9/10) | hand-code | Multiplies the static per-endpoint credit table by requested rows and prints spend BEFORE dialing; zeroes rows already cached. No such preview exists in API or web app. |
| 2 | Running credit balance | `balance` (8/10) | hand-code | The REST API has NO balance endpoint; we persist `credits_remaining` from every billed response into a local ledger and read it back. |
| 3 | Budget ceiling guard | `budget set <n>` + `--budget <n>` (8/10) | hand-code | Refuses any billed call whose estimate exceeds the remaining ceiling, before the call dials. Guards against the 80cr-phone / 50cr-brief footguns. |
| 4 | Spend report | `spend [--since] [--by endpoint]` (7/10) | hand-code | Aggregates the local credit_ledger into total / per-endpoint / per-day — "what's eating my credits." Impossible without local persistence. |
| 5 | Stale-cache report | `stale [--older-than 24h]` (7/10) | hand-code | Flags cached entities older than Sumble's documented ~24h freshness lag so the user re-bills only what's actually stale. |
| 6 | Tech-stack diff | `stack-diff <orgA> <orgB>` (7/10) | hand-code | Cross-joins two cached org enrichments to show shared/unique technologies — zero credits, no API or web equivalent. |
| 7 | Free-first match→enrich reconcile | `reconcile <csv>` (6/10) | hand-code | Runs cheap `organizations/match` (1cr, unmatched free) over a CSV, caches IDs, reports which still need a billed enrich — the cheapest CRM-reconciliation path. |

**Hand-code commitment: 7 features.** All require local SQLite (ledger/cache) joins or custom cost logic the generator does not emit. None ship as stubs.
