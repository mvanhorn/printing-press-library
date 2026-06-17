# World Bank CLI Brief

## API Identity
- Domain: `https://api.worldbank.org/v2` — World Bank Open Data, classic Indicators API v2.
- Users: economists, data journalists, policy researchers, data-pipeline/consulting engineers, AI agents answering macro/development questions.
- Data profile: ~16,000 indicators × ~200 economies × decades of annual time-series, plus reference dims (sources, topics, regions, income levels, lending types).
- Auth: **NONE** — fully public, keyless. `format=json` required (XML default otherwise).

## Reachability Risk
- **None.** Live probe `GET /v2/country/USA/indicator/NY.GDP.MKTP.CD?format=json` → HTTP 200, real data (US GDP 2024 = $28.75T).
- No bot protection, no rate-limit gate observed on light use. Probe-safe endpoint: `GET /v2/country` (keyless).

## Response Envelope Quirk (critical for generation)
- Every list response is a **two-element JSON array**: `[ {page,pages,per_page,total,sourceid,lastupdated}, [ ...rows... ] ]` — element 0 is pagination metadata, element 1 is the data rows. NOT an object.
- The generator must extract the **second array element** as the data payload (`response_path` equivalent) and read pagination from element 0. This is exactly the FRED-retro `response_path` failure class — hand-author the spec so the data path is explicit rather than relying on docs inference.
- Pagination is `page`/`per_page` (max 32,500/page realistically use 1000); `mrv=N` (most-recent-values), `date=YYYY:YYYY` range, `gapfill=Y`, `frequency` (Y/Q/M for sources that support it).

## Top Workflows
1. **Indicator time-series for a country**: `GET /country/{iso}/indicator/{id}?date=2000:2024` — the core command.
2. **Cross-country compare** for one indicator (e.g. GDP across US;CN;IN via `country/US;CN;IN/indicator/{id}`).
3. **Discover indicators**: search/browse `/indicator`, `/topic/{id}/indicator`, `/source/{id}/...`.
4. **Country metadata**: `/country/{iso}` (region, income level, lending type, capital, lat/long).
5. **Reference dims**: `/source`, `/topic`, `/region`, `/incomeLevel`, `/lendingType`.

## Table Stakes (absorb from wbgapi / wbdata / world_bank_data / MCP servers)
- Fetch indicator data by country + date range (+ mrv, gapfill).
- Multi-country, multi-indicator queries (`;`-joined codes).
- List/search indicators, countries, sources, topics, regions, income levels, lending types.
- Indicator metadata lookup.
- CSV/JSON/pandas-friendly output.
- Most-recent-value convenience.

## Data Layer
- Primary entities: `indicators`, `countries`, `sources`, `topics`, `regions`, `income_levels`, `lending_types`, and `observations` (the time-series rows).
- Sync cursor: indicators/countries/reference dims are slow-moving → ideal local SQLite mirror for offline search. `observations` synced per (country,indicator) query.
- FTS/search: indicator catalog (~16k rows with long names) is the killer offline-search target — wbgapi's own docs note indicator discovery is the hardest part.

## Codebase Intelligence (from ecosystem)
- wbgapi: surface = `data`, `series` (indicators), `economy` (countries), `source`, `topic`, `region`, `income`, `lending`, plus `.info()`/`.search()` on each. This is the de-facto feature ceiling to match.
- MCP servers expose: list countries, list indicators, analyze indicator for country+range. Thin — easy to beat with offline store + SQL.

## User Vision (from Luke)
- Lands in the **finance/data cluster** → category `other` until a `finance`/`data` category exists (deliberate; builds evidence for the category proposal from the FRED retro P1).
- Headline = indicator time-series by country. Value for data-pipeline/consulting work: scriptable bulk pulls, offline indicator discovery, cross-country export.

## Product Thesis
- Name: **world-bank-pp-cli** — "Every World Bank indicator, queryable offline, with cross-country SQL no wrapper has."
- Why it should exist: existing tools are Python-library-bound (need a notebook) or thin MCP shims. A single-binary, agent-native CLI with a local SQLite mirror of the 16k-indicator catalog + observations gives offline search, `--json`/`--select`/`--csv`, typed exit codes, and cross-country joins that none of the wrappers or MCP servers offer.

## Build Priorities
1. Data layer: indicators + countries + reference dims + observations in SQLite; sync + FTS.
2. Absorb: data fetch (country/indicator/date/mrv/gapfill), multi-country compare, list/search every dimension, indicator metadata.
3. Transcend: offline indicator search, cross-country SQL, time-series diff/trend, "what changed" between vintages, bulk pipeline export.

## Spec Strategy
- No official OpenAPI exists. Hand-author an internal YAML spec (FRED-style) so the two-element-array `response_path`, pagination, and `;`-multi-code paths are explicit. `--docs` would likely miss the envelope quirk.
- Category: `other` (finance/data cluster).
