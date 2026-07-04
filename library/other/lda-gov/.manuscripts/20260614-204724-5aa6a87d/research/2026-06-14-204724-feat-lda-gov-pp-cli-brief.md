# LDA.gov CLI Brief

## API Identity
- Domain: Lobbying Disclosure Act public filings from the U.S. Congress, exposed through the official LDA.gov REST API.
- Canonical API base: `https://lda.gov/api/v1/`.
- Official docs: `https://lda.gov/api/redoc/v1/`.
- Official OpenAPI: `https://lda.gov/api/openapi/v1/`.
- Users: journalists, researchers, campaign-finance analysts, watchdog groups, civic-tech agents, and compliance teams tracking lobbying registrations, quarterly activity, LD-203 contributions, registrants, clients, and lobbyists.
- Data profile: read-only public records, paginated JSON, deeply nested filing and contribution records, rate-limited, with official source URLs and public print pages.

## Reachability Risk
- Low. Live unauthenticated probes succeeded:
  - `GET https://lda.gov/api/v1/` -> HTTP 200.
  - `GET https://lda.gov/api/v1/filings/?filing_year=2024&page_size=1` -> HTTP 200 with `count: 96854`.
- Auth is optional, not mandatory. Terms say anonymous access needs no special authentication but is throttled more strictly.
- Rate limits: API key registered clients get `120/minute`; anonymous clients get `15/minute`.
- `lda.senate.gov` is deprecated and sunsets June 30, 2026. Generated CLI should default to `lda.gov`, not the stale server URL still present in the official schema.
- Tier/permission hints: none from live 4xx. ToS warns access may be temporarily or permanently blocked if a user attempts to exceed or circumvent limits.
- Probe-safe endpoint used: `GET /api/v1/filings/?filing_year=2024&page_size=1`.

## Reachability Gate
- Decision: PASS.
- Evidence: `GET https://lda.gov/api/v1/` returned HTTP 200.
- Evidence: `GET https://lda.gov/api/v1/filings/?filing_year=2024&page_size=1` returned HTTP 200 and a paginated filings envelope.
- Auth context: anonymous public reads are supported; optional API key only increases rate limit.

## Users
- Investigative reporter: checks quarterly lobbying activity around companies, issues, and bills, then needs source-linked evidence for publication.
- Civic-tech researcher: syncs filings in batches, normalizes entities, and runs SQL/CSV analysis across years without exceeding public rate limits.
- Watchdog analyst: monitors new registrations and LD-203 contributions after filing deadlines, looking for amendments, terminations, duplicates, and conflicts.
- Compliance or policy staffer: resolves registrant/client/lobbyist IDs and validates official source records before citing LDA data in reports.

## Top Workflows
1. Search and download LD-1/LD-2 filings by year, registrant, client, issue, government entity, filing type, and date.
2. Track LD-203 contribution reports and flatten contribution items for analysis.
3. Resolve entities across registrants, clients, and lobbyists, including name variants and ID lookups.
4. Sync filings locally by year/quarter without tripping rate limits, then query with SQL, FTS, JSON, CSV, and agent-native field selection.
5. Audit lobbying spend and activity over time, with warnings for amended/terminated filings, duplicate risk, and client/registrant conflicts.

## Table Stakes
- Cover all 18 official OpenAPI operations: list/retrieve filings, contributions, registrants, clients, lobbyists, and constants.
- Support anonymous reads by default, with optional `Authorization: Token <key>` for higher rate limits.
- Honor rate limits and `Retry-After`; avoid aggressive parallel fetching.
- Preserve pagination rules: `page_size` max 25; filings/contributions need at least one query parameter to paginate beyond page 1.
- Provide JSON, table, CSV, `--select`, `--compact`, and agent-friendly output.
- Add local sync/search/SQL for filings, contribution reports, registrants, clients, lobbyists, and constants.
- Generate public print/source URLs for citations and verification.
- Keep `lda.gov` as the first-class host and warn against `lda.senate.gov` deprecation.

## Data Layer
- Primary entities: filings, contribution reports, registrants, clients, lobbyists, constants.
- Sync cursor: `filing_year`, `filing_period`, `dt_posted`, `date_received`, and endpoint pagination cursor (`page`).
- FTS/search: filing-specific lobbying issues, registrant names, client names, lobbyist names, covered positions, government entities, contribution item text, and source URLs.
- Derived indexes: entity name normalization, client-registrant relationships, lobbyist-to-client relationships, issues, years/quarters, contribution totals.

## Codebase Intelligence
- R `lobby`: covers filings, contributions, registrations, clients, lobbyists; uses `USSLDA_KEY`; still references the legacy Senate host in docs.
- R `lobbyR`: provides the strongest analysis patterns, including duplicate flags, client/registrant conflict checks, and cleaning utilities.
- Ruby `lobbying_disclosure_client`: mirrors list/retrieve endpoints and auth/login/password reset.
- MCP servers `us-gov-open-data-mcp` and `LegisMCP`: expose search/detail/contributions/registrants/lobbyists tools, proving agent demand for a thin LDA interface.
- Scripts and adapters: POLITICO scraper, Open-Case adapter, Contract-Sweeper adapter, and Rust `Congress-Tracker` client all emphasize rate-safe pagination, dedupe, CSV normalization, and source URL preservation.

## Product Thesis
- Name: LDA.gov CLI.
- Thesis: the official API gives access to public lobbying records, but researchers still need rate-safe bulk sync, entity resolution, source citations, and local analysis. A CLI with SQLite, FTS, JSON/CSV, and agent-native output makes LDA data usable without reimplementing the same pagination and cleanup logic in every notebook.

## Build Priorities
1. Generate the full official endpoint surface from the patched OpenAPI spec.
2. Preserve anonymous operation while documenting optional `Authorization: Token <key>` for higher rate limits.
3. Build local sync/search/SQL workflows around filings, contributions, entities, and constants.
4. Add analysis commands for spend audits, entity resolution, monitoring, citation, graph export, and data-quality flags.
5. Keep live dogfood no-auth by default, using bounded public queries such as `filing_year=2024&page_size=1`.
