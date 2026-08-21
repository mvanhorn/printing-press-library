# FEC CLI Brief

Run: 20260821-194418
Issue: mvanhorn/printing-press-library#1194

## API Identity
- Domain: US Federal Election Commission public campaign-finance data (api.open.fec.gov, official FEC API)
- Users: journalists, researchers, political scientists, compliance/oppo researchers, civic-tech devs, anyone tracking money in federal elections
- Data profile: read-only, heavily relational (candidate -> committee -> filings -> itemized receipts/disbursements), paginated JSON, two-year transaction periods as the key time dimension. Updated nightly.

## Reachability Risk
- None. Official government API, no anti-bot. Verified live this session:
  - GET /v1/candidates/search/?q=lee -> 200 with real results
  - GET /v1/audit-category/ -> 200
- Auth: free api.data.gov key via query param `api_key` or `x-api-key` header.
  - DEMO_KEY works for smoke tests at ~40 req/hr
  - Registered key: 120 req/min, 1000 req/day (Hunter's key in scope .env, never committed)

## Source Priority
- Primary: official Swagger 2.0 spec, https://api.open.fec.gov/swagger/ (849KB, 100 paths, 203 definitions), downloaded to $PRESS_HOME/fec-swagger.json
- Single source, not a combo CLI.

## Top Workflows
1. Find a candidate or committee by name; see who they are, what office, what cycle (candidates/search, committees)
2. Follow the money: itemized individual contributions to a committee (schedules/schedule_a), sorted by receipt date/amount
3. Committee financial picture: totals + reports per cycle (totals/{committee_id}, reports and efile endpoints for raw filings)
4. Election landscape: all candidates in a race for a cycle (elections/*), plus independent expenditures and communication costs
5. Legal/compliance research: audits, MURs, ADRs, advisory opinions via legal search

## Table Stakes
- name/id search on candidates and committees
- pagination with per-page control and total counts
- schedule_a itemized donor listing with sort + two_year_transaction_period filter
- committee/candidate detail by ID
- JSON output everywhere, --fields projection

## Data Layer
- Primary entities: candidates, committees, filings, schedule_a receipts, totals
- Sync cursor: last contribution_receipt_date (or file_number) seen per entity
- FTS/search: candidate/committee name search is server-side (q param); local FTS over cached donors optional

## Product Thesis
- Name: fec-pp-cli
- Why it should exist: every newsroom/researcher workflow against OpenFEC starts with curl + a swagger page + manual pagination. The CLI turns "who gave how much to whom" into one command with sane defaults (sort by date desc, cycle filter, JSON out). Issue #1194 explicitly requests this shape: candidates/committees/filings/donors from the terminal with SQLite sync.
- Explicit non-goal per issue: no donor targeting/harassment tooling; public-records research only.

## Novel Features (Phase 1.5 candidates)
1. Money-window default: schedule_a queries without --begin/--end default to contributions received in the current two-year period ending today, so bare `fec receipts --committee-id C...` returns live recent money instead of a param error (API requires one of two_year_transaction_period/committee_id/date filters).
2. Donor rollup: `fec receipts top-donors --committee-id X` groups schedule_a by contributor_name+employer with summed amounts, the #1 journalist question, which the API answers only client-side today.
3. Cycle-aware candidate view: `fec candidates get <id>` auto-enriches with latest-cycle totals (raised/spent) in one call instead of two API round-trips.

## Build Priorities
1. candidates + committees search/get (the entry point everything routes through)
2. schedule_a receipts w/ sort, filters, money-window default
3. reports/totals per committee cycle
4. elections lookup + independent expenditures
5. SQLite store sync on receipts (issue-requested), cursor = last receipt date
