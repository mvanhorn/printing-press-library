# Sumble CLI Brief

## API Identity
- Domain: B2B sales intelligence — **technographics** (what tech a company uses, derived from job postings), org charts / people, hiring-signal ("intent") data, and AI intelligence briefs.
- Base URL: `https://api.sumble.com/v6` (the SDK in `~/Projects/claude/sumble-skill` uses `/v3`; **v6 is current** and is the build target — both versions authenticate with the user's key, confirmed via 422 validation probes).
- Auth: `Authorization: Bearer $SUMBLE_API_KEY`. Key page: https://sumble.com/account/api-keys
- Users: SDRs/AEs, growth/RevOps, founders doing ICP technographic prospecting and account-based people sourcing.
- Data profile: organizations, people, jobs, technologies, saved org-lists & contact-lists. ~24h data freshness lag.

## Reachability Risk
- **Low.** Live key returns 422 (auth OK, path exists) on both `/v3` and `/v6`. No 403/blocked/deprecation issues found on any community repo. Rate limit 10 req/s aggregate, 429 on exceed. Only risk is **version churn** (v3→v5→v6 in <1yr) — isolate the base path so a future bump is one edit.

## Cost Model (the central design constraint — user explicitly wants the CLI to minimize credit burn)
- Usage-based credits. Free 500/mo, Pro 9,900/mo, Enterprise custom.
- **Every JSON response carries `credits_used` and `credits_remaining`.** There is NO REST account/balance endpoint (balance otherwise only via the official MCP `GetAccountInformation`). So the CLI derives & tracks balance from any response.
- Per-endpoint cost (verbatim from docs):
  - `organizations/find` — **5 cr/row returned**
  - `organizations/enrich` — **5 cr per technology found**
  - `organizations/match` — 1 cr per matched org (unmatched free)
  - `organizations/{id}/intelligence-brief` — **50 cr** (202-pending is free)
  - `people/find` / `find-related-people` — 1 cr/person
  - `people/enrich` — **email 10 cr, phone 80 cr** (first reveal only; cached/unavailable = free)
  - `jobs/find` — 2 cr/job (3 with descriptions); `jobs/{id}` — 1 cr; `jobs/find-related-people` — 1 cr/person
  - `technologies/find` — 1 cr if ≥1 match, else free
  - list reads — 1 cr/row; list writes (create / add) — **free**
- **Cost-control is the headline differentiator.** No native cost preview, dedup, or balance tracking exists. A CLI that (a) estimates cost before a call, (b) tracks running balance, (c) caches results in SQLite so the same org/person/job is never re-billed, and (d) refuses calls over a budget ceiling is uniquely valuable.

## Top Workflows
1. **ICP technographic prospecting** — `organizations/find` by technologies/categories → review → save to org-list. (5 cr/row → must be bounded & cached.)
2. **Account-based people sourcing** — resolve org → `people/find` by job_function/level/country → optional `people/enrich` for email/phone (expensive). Save to contact-list.
3. **Tech-stack enrichment** — `organizations/enrich` for a known domain to read its full stack with last-seen / job-count signals.
4. **Hiring-signal monitoring** — `jobs/find` + `order_by jobs_count_growth_6mo` to spot accounts ramping on a target tech (intent).
5. **CSV/CRM reconciliation** — `organizations/match` (cheap, 1 cr) to resolve a list of names/URLs to Sumble IDs before enriching.

## Table Stakes (match every existing surface)
- All 4 organization endpoints, all 3 people endpoints, all 3 jobs endpoints, technologies/find, org-lists CRUD, contact-lists CRUD.
- Structured filters (technologies[], technology_categories[], countries[], job_functions[], job_levels[], since) AND the free-text `query` form (v6 `query` is **natural language**, not the v3 SQL-DSL).
- `order_by_column` enums for orgs; limit/offset pagination (orgs ≤200, people ≤250, jobs ≤100, offset ≤10000).
- JSON output, `--select`, CSV export (web app is CSV-only; JSON/JSONL is a real add).

## Data Layer
- Primary entities: organizations, people, jobs, technologies, org_lists, contact_lists. Plus a local **credit_ledger** (one row per billed call: endpoint, credits_used, credits_remaining, timestamp, args-hash).
- Cache key per entity: org by id/slug/domain; person by id; job by id. Cached reads are free locally and avoid re-billing on repeat.
- Sync cursor: `since` (YYYY-MM-DD) on find/enrich/jobs.
- FTS/search: offline search over cached orgs (name/domain/industry), people (name/title), jobs (title/description), technologies (name/slug).

## User Vision (from briefing)
- Use case: **prospecting and lead enrichment.**
- **Pricing is usage-based → the CLI must help the agent reduce credits used.** This is the primary product driver: every command should be cost-aware, every result cached, every expensive call gated behind an estimate/confirmation or a budget.
- Auth is a simple API key (confirmed: Bearer header, `SUMBLE_API_KEY`).

## Product Thesis
- Name: **Sumble CLI** (`sumble-pp-cli`).
- Why it should exist: the only Sumble interface that is **credit-aware by construction** — it estimates spend before every billed call, tracks the running balance Sumble's REST API won't give you directly, and caches everything in local SQLite so an agent never pays twice for the same org, person, or job. Full v6 surface + offline search + agent-native JSON, beating the web app (CSV-only, no dedup, no cost preview) and the bare API (no balance endpoint, no caching).

## Build Priorities
1. **Data layer + credit ledger** for all entities (foundation for caching = credit savings).
2. **All absorbed endpoints** (orgs/people/jobs/technologies/lists) with cache-first reads.
3. **Transcendence: the credit-economy commands** — cost estimator/dry-run, balance tracker, budget guard, cache-hit dedup, spend report. These are what make it "the credit-frugal Sumble CLI."
