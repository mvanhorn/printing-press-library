# FAA Aircraft Registry — Ecosystem Research (subagent final report)

(Orchestrator note: §3's claim that owner-name search is only available via amsrvs.registry.faa.gov/airmeninquiry is WRONG — verified live that GET /aircraftinquiry/Search/NameResult?nametxt=DELTA+AIR+LINES returns 1,558 aircraft across 32 pages. Airmen inquiry is the pilot-certificate registry, a different dataset. Also all criteria forms accept GET with query params, verified live.)

## 1. Existing tools

| Tool | URL | Lang | Stars | Features |
|---|---|---|---|---|
| adsbtrack (closest competitor) | github.com/frankea/adsbtrack | Python | ~9 | SQLite-backed CLI: registry update (bulk zip), lookup --hex/--tail, owner --name (SQL LIKE), address --state --city, tail→hex; enrich (FAA+Mictronics+hexdb.io), mil hex, ADS-B traces. Registry subordinated to ADS-B tracking. |
| FAA-registry-checker (Jxck-S) | github.com/Jxck-S/FAA-registry-checker | Python | ~19 | Daily cron bulk download, watches owner-name list (exact ALL CAPS), Discord webhook alerts. Dereg/update/reserve detection roadmapped, unbuilt. |
| faaDb (ThreeSixes) | github.com/ThreeSixes/faaDb | Python | ~0 | Bulk zip → MongoDB + Flask REST; tail/hex/ICAO-integer lookup. Excludes deregistered aircraft. |
| faa-aircraft-registry (ClearAerospace) | PyPI | Python | small | Parses MASTER/ACFTREF/ENGINE into dicts. No CLI/search/hex. |
| icao-nnumber_converter (guillaumemichel) | GitHub+PyPI | Python | ~25 | Pure hex↔N math (a00001=N1 … adf7c7=N99999). Reference algorithm. |
| @squawk/icao-registry | npm | JS | — | Hex→registration with bundled FAA snapshot. |
| simonw/scrape-faa-releasable-aircraft | GitHub | Python | — | Git-scraping of the daily zip → diffable history pattern. |
| BrentIO/Aircraft-Registration-and-Operator-Information | GitHub | — | — | Multi-source → MySQL + API microservice, operator-focused. |
| ARLA (njfdev) | arla.njf.dev | TS | ~4 | Hosted free lookup API, FAA-only, thin. |
| SkyLink API | RapidAPI | hosted | — | Commercial, 615k worldwide, reg/hex lookup + photos, freemium 1k/mo. |
| aviation-mcp (blevinstein) | GitHub | — | small | Weather/NOTAM/charts MCP. No dedicated FAA-registry MCP exists anywhere. |

## 2. Releasable DB
- Daily refresh ~11:30pm Central (bulk) vs web app "each federal working day at midnight" — clocks can disagree mid-cycle.
- Space-padded comma-delimited; layout drifts (DOCINDEX gained Doc Type column 2024-07-30).

## 3. Contract notes
- GET /AircraftInquiry/Search/NNumberResult?nNumberTxt=<NNUM> (param case-insensitive); no documented JSON API anywhere; every tool scrapes HTML or uses bulk zip.

## 4. Hex algorithm
- US block A00000-AFFFFF; sequential mapping a00001→N1, adf7c7→N99999; letters A-Z minus I,O; MASTER carries both octal + hex Mode S.

## 5. Pain points
1. No ownership history on live site (needs DEREG).
2. No owner→fleet reverse search worth using; adsbtrack is SQL LIKE only.
3. Owner matching brittle/exact-caps; no fuzzy anywhere.
4. One-at-a-time HTML-only official surface.
5. Deregistered/reserved data siloed; most tools drop them.
6. Silent layout drift breaks parsers.
7. Two refresh clocks confuse consumers.

## 6. Gaps (open lanes)
- Offline SQLite full-registry search (thin), owner fleet queries (weak), expiring-registration alerts (open), deregistered lookup (open), reserved-N availability (open), dedicated registry MCP (open).
