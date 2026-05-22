# Prospeo CLI Brief

## API Identity
- Domain: B2B contact data / people-and-company enrichment. Email finder, mobile finder, company enrichment, search, ICP signals.
- Users: SDR/sales ops, recruiters, growth marketers, lead-gen agencies. Often piped into cold-email tools (SmartLead, Instantly, Lemlist).
- Data profile: People + companies. Credit-billed. 75 free credits, paid tiers 1k–50k credits/mo. Synchronous JSON HTTP API at `https://api.prospeo.io`. All POST except `/account-information` (GET).

## Reachability Risk
- None. `api.prospeo.io` reachable from CLIs, no Cloudflare blocks reported. Probe of `GET /account-information` without key returned HTTP 400 (expected; endpoint exists).

## Top Workflows
1. **Single-shot enrichment** — "find email + mobile for {first_name, last_name, company}" or "enrich from LinkedIn URL". Power user pipes one row.
2. **Bulk CSV enrichment** — 50-row chunks via `/bulk-enrich-person`, often as the second step of a list-build pipeline.
3. **Search-then-enrich funnel** — `/search-person` with filters → page through results → call `/enrich-person` per hit to unmask contacts.
4. **Company prospecting** — `/search-company` with tech stack / funding / industry filters, then `/enrich-company` for tech stack, funding rounds, employees.
5. **Credit watch** — check `/account-information` to track burn, especially before mobile (10-credit) calls.

## Table Stakes (from research)
- Person enrich by linkedin_url, by name+company, by email, by person_id
- Mobile finder (10-credit gate)
- Company enrich by website / linkedin_url / company_id
- Person search with 30+ filters (job_title, seniority, department, location, company filters, ICP signals)
- Company search with industry/employees/revenue/funding/tech-stack filters
- Bulk endpoints with caller IDs and `total_cost` summary
- Search suggestions (free, no credit)
- Account information + credit display

## Data Layer
- Primary entities: `persons`, `companies`, `enrichments` (request→response with caller_id + cost), `searches` (page snapshots with filters JSON + total_pages), `account_snapshots` (credits over time), `csv_jobs` (bulk run history)
- Sync cursor: none (no list API), but `enrichments` and `search` rows act as a local cache keyed by canonical inputs (linkedin_url, email hash, name+domain). Re-enrichments within 30 days are free; cache to avoid even hitting the API.
- FTS/search: FTS5 on `persons.full_name`, `persons.job_title`, `companies.name`, `companies.domain` for offline lookup.

## Codebase Intelligence
- Auth: `X-KEY: <key>` header; env var `PROSPEO_API_KEY`. No OAuth.
- Rate limiting: per-plan; Starter 5 req/s enrich / 1 req/s search; Pro 30 enrich / 5 search. Response headers `x-daily-request-left`, `x-minute-request-left`. 429 on breach.
- Error model: top-level `error: bool` + HTTP code. `NO_MATCH`, `INVALID_DATAPOINTS`, `INSUFFICIENT_CREDITS`, `RATE_LIMITED`.
- Pagination: 25 per page, max 25,000 total results. Pages 1..total_pages from response.

## User Vision
- "Include everything that is present within the API docs" — exhaustive coverage of all 8 documented endpoints.
- "Lookalikes" — Prospeo markets "AI Search & Lookalikes" in the platform UI but exposes it as ICP signal filters inside `/search-person`/`/search-company` (Growth plan+). No standalone lookalike endpoint. We surface this as filter-shaped CLI flags and a focused `lookalike` workflow command.

## Competing Tools / Wrappers
- **Meerkats-Ai/prospeo-mcp-server** (Node MCP) — 3 tools, uses deprecated endpoints (`email-finder`, `domain-emails`, `enrich-from-linkedin`). Stale.
- **Official Prospeo MCP** at `mcp.prospeo.io` — hosted, OAuth2, 5 tools (enrich_person, enrich_company, search_person, search_company, get_account_info).
- **No npm package**, **no PyPI package**.
- **Existing Crucible skill** at `.claude/skills/prospeo/` — Python script wrapping the (now-deprecated) `email-finder` endpoint with Trykitt waterfall fallback. CLI we build supersedes it.
- **Adjacent tools to absorb from conceptually**: Hunter.io (`hunter` CLI patterns), Apollo (Apollo MCP), Snov (snov CLI). None have native CLIs of note; their command surfaces are aspirational.

## Product Thesis
- **Name:** `prospeo-pp-cli`
- **Display name:** Prospeo
- **Why it exists:** Prospeo ships only a hosted MCP and HTTP API. There's no real CLI, no local cache, no bulk CSV pipeline that hides the 50-row API chunking, no credit-aware throttle, no lookalike workflow surfacing the ICP filters. Pipeline-friendly `--json`, `--csv`, `--select`, `--dry-run`, and a SQLite cache turn this into the kind of tool an SDR can wire into shell scripts and SmartLead pipelines without owning a Python venv.
- **Differentiator:** The only Prospeo client that (a) caches lifetime hits locally so you don't burn credits re-enriching, (b) splits CSV inputs into 50-row bulk requests with credit pre-flight, (c) gives a one-flag `--mobile` opt-in to the 10-credit gate, (d) ships an offline `search` over previously-enriched contacts.

## Build Priorities
1. Core data layer (persons, companies, enrichments cache, searches, account_snapshots, csv_jobs) + sync command (refresh `/account-information`).
2. All 8 endpoints as first-class commands with `--json`, `--select`, `--csv`, `--compact`, `--dry-run`.
3. Bulk CSV pipelines: `enrich person bulk --input leads.csv --output enriched.csv --waterfall? --concurrency N` with auto-chunking to 50.
4. Credit pre-flight: `--show-cost` and `--max-cost N` flags on enrich + search.
5. Transcendence: lookalike workflow, credit burn timeline, dupe-aware enrich, search-then-enrich pipeline, offline contact search.
