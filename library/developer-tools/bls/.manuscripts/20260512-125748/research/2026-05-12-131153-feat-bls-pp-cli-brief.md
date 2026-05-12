# BLS Public Data API CLI Brief

## API Identity
- Domain: U.S. labor & economic time-series — CPI, PPI, employment (CES), unemployment (LAUS/CPS), JOLTS, ECI, productivity, QCEW, OEWS, average prices.
- Base: `https://api.bls.gov/publicAPI/v2/`
- Users: economists, journalists, policy analysts, finance/macro researchers, dashboards, retrieval-augmented agents that need authoritative U.S. labor data.
- Data profile: ~one million time series across ~30 surveys. Mostly monthly; some quarterly/annual. Values come back as `{year, period, periodName, value, footnotes}` rows.

## Reachability Risk
- **Low for `api.bls.gov`** — stable JSON API, no Akamai bot wall on this hostname. Documented 500 queries/day registered, 25/day unregistered; 50 series/request, 20-year window.
- **High for `www.bls.gov` and `download.bls.gov`** — both are wrapped in Akamai's "BLS bot policy" 403 page. The marketing site, the release calendar, and the flat-file mirror all return 403 to bare curl. Bulk-download fallback requires a real browser UA and gentle throttling; expect intermittent blocks. Treat this as discovery-only, never the runtime hot path.
- Government-shutdown release-date shifts (e.g. 2025-lapse-revised-release-dates) periodically reshuffle the release calendar.

## Top Workflows
1. **Resolve a series ID and pull recent observations** — `bls series get CUUR0000SA0 --years 5 --json` returning a clean tidy frame.
2. **Cross-survey snapshot** — "what's the latest CPI, unemployment rate, and payrolls?" in one call (POST batch up to 50 series).
3. **Series-ID discovery without leaving the terminal** — find the right code for "Los Angeles CPI all items" or "California unemployment rate" by name/area/item.
4. **Year-over-year + period-over-period calculations** — agents and analysts want the `percent change` columns BLS already exposes via `calculations: true` (key required).
5. **Release-calendar awareness** — "what's the next CES release? When does CPI drop next?" Today this is HTML-only.

## Table Stakes (must match the wrapper field)
- Single-series fetch, batch fetch (50 series max).
- Latest-observation fetch.
- Survey list and survey detail.
- Popular-series listing per survey.
- Registration-key support in request body (lowercase `registrationkey`).
- `catalog`, `calculations`, `annualaverage`, `aspects` body flags.
- Tidy JSON output, CSV output.
- Sensible rate limiting + retry on 429.

## Data Layer (entities worth persisting locally)
- **Series catalog** — id, title, units, survey, seasonal-adj flag, begin/end year, last update. Source: `<abbr>.series` flat files (one-time bulk import; refresh monthly).
- **Surveys** — abbreviation, name, status. Source: `GET /surveys/`.
- **Areas** — codes by survey (CPI areas, LAUS state/metro, OEWS MSAs).
- **Items** — CPI items, average-price items, CES supersectors/industries (NAICS), JOLTS industries, OEWS occupations (SOC).
- **Periodicity codes** — M01–M13, Q01–Q05, A01, S01–S03.
- **Seasonal-adjustment decoder** — position-3 lookup per survey.
- **Release calendar** — survey → next-release-date; scraped or hand-curated.
- **Observations cache** — `(seriesId, year, period) → value, footnotes` for offline replay.
- **Popular-series cache** — per survey.

FTS5 over the series catalog (title + survey + area + item) is the unlock for series-ID discovery without ever leaving the terminal.

## Codebase Intelligence
- BLS publishes the API signature page (`bls.gov/developers/api_signature_v2.htm`) and feature page (`bls.gov/bls/api_features.htm`) as authoritative docs. There is no OpenAPI spec — we must author one.
- No public BLS MCP server exists. `lzinga/us-gov-open-data-mcp` bundles BLS into a 40-API multiplexer; coverage is shallow.
- Most-active wrapper is `keberwein/blscrapeR` (R, 117 stars, active 2026-04). Python's `OliverSherouse/bls` (84★) is the canonical Python wrapper but stale since 2020.
- Auth pattern is body-only: `registrationkey: "..."` in the POST JSON. No header.

## User Vision
None volunteered at briefing — the user said "Let's go."

## Product Thesis
- Name: `bls-pp-cli` (slug `bls`).
- Why it should exist: there is **no feature-rich BLS CLI today**, only language-specific library wrappers. Series-ID discovery is the universally cited #1 pain point and it is impossible to solve with the live API alone — only a locally-cached, FTS-searchable series catalog makes "find the right series by name" tractable. The combination of (a) tidy JSON, (b) offline series search backed by the flat-file dump, (c) batch fetch + calculations + catalog metadata, (d) MCP exposure of every command, and (e) a release calendar pre-cached locally is genuinely new ground.

## Build Priorities
1. **Series surface (Priority 0/1)** — `series get <id>`, `series batch --ids ...`, `series latest <id>`, `series search "<query>"`, `surveys list`, `surveys get <abbr>`, `popular --survey <abbr>`.
2. **Local catalog + search (Priority 0)** — `sync` bulk-imports the BLS flat-file series catalogs (one-time) and refreshes monthly; `search` runs FTS5 over title/survey/area/item; `sql` for arbitrary queries.
3. **Series-ID builder (Priority 2 transcendence)** — `series build --survey CPI --area "Los Angeles" --item "All items" --adjust seasonal` synthesizes the packed series ID from human-readable inputs by joining the local lookup tables.
4. **Release calendar (Priority 2 transcendence)** — `releases next`, `releases for <survey>`, `releases watch` reading a locally-curated calendar JSON.
5. **Calculations passthrough (Priority 1)** — `series get ... --calc --annual-avg --catalog` toggles the BLS calculations features.
6. **Snapshot / dashboard (Priority 2 transcendence)** — `snapshot macro` returns latest values for the top-15 macro indicators (CPI, unemployment, payrolls, JOLTS openings, PPI, ECI, productivity, etc.) in one call.
7. **Footnote decoder (Priority 2 transcendence)** — `footnotes decode P` explains each footnote code in plain English, joining the flat-file footnote table.
