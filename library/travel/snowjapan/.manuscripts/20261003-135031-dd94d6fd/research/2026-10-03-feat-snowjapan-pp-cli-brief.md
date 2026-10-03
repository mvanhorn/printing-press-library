# SnowJapan CLI brief

## API Identity
- Source: https://www.snowjapan.com/ and public SnowJapan resort, season and report pages.
- Users: Japan ski and snowboard travelers deciding where, when and how to visit.
- Data: nationwide active-area directory; elevation, vertical, course difficulty, total lifts, location/access; confirmed historical season dates; regional report metadata and factual observations where present.
- This is an undocumented public website; BROWSER_SNIFF_TARGET_URL=https://www.snowjapan.com/ski-areas. No official API or SDK is advertised. Browser/HTML source discovery is the canonical route; unrelated snow-named SDKs are irrelevant.

## Reachability Risk
- Low for observed public HTML (HTTP 200 without credentials on quick-search and Nagano listings). Wix SSR structure is a parsing drift risk.
- Sandbox DNS errors were an environment restriction, not a provider block; outside-sandbox direct fetch returned 200.
- Native public browser successfully searched Hakuba and inspected ABLE Hakuba Goryu on 2026-10-03.
- Website refreshed in August 2026; new daily snow-depth chart explicitly starts November 2026. Existing winter archives must remain historical.

## Users
- A traveler refining a Japan winter shortlist each week compares named ski areas, terrain proportions, elevation and lift layout; the region-to-town distinction makes tab-by-tab comparison tedious.
- A repeat regional skier revisits historical opening/closing evidence while checking dated regional reports during trip planning; past winters and unupdated preseason fields can otherwise be mistaken for current operations.
- A planning assistant updates the same shortlist after new source facts arrive, needing concise structured output, exact source links and transparent unknowns.

## Top Workflows
1. Search names, municipalities, popular regions or prefectures, with numeric terrain/lift filters and small bounded output.
2. Inspect one exact canonical resort and compare a small shortlist using explicit ability distribution, elevation, lifts and access facts.
3. Inspect confirmed opening/closing dates for a selected past winter; evaluate a chosen historical date without predicting upcoming operation.
4. Inspect a regional report's date and factual snowfall observations, retaining station/region coverage and links to the original.
5. Retain factual snapshots locally for offline search, compare and change inspection with clear observation timestamps.

## Table Stakes
- SnowJapan itself provides name/town/region quick search, prefecture browsing, numeric filters, comparison and five recent seasonal charts.
- Skiresort.com's own Japan comparison pages offer elevation and lift comparisons (https://www.skiresort.com/en/comparison/japan/).
- Powderhounds publishes a resort statistics comparison including beginner/intermediate/advanced percentages and broad season windows (https://www.powderhounds.com/site/DefaultSite/filesystem/documents/TrailMaps/Japan_Resort_Statistics.pdf).
- Concrete pain points: multi-resort region names differ from municipalities; installed lifts do not imply open lifts; broad seasonal plans and last winter's ticket prices do not confirm upcoming dates or quotes.

## Data Layer
- Primary entities: Resort, HistoricalSeason, RegionalReport, EvidenceSnapshot.
- Stable identifiers: canonical SnowJapan path and visible Wix item UUID when available. Preserve English/Japanese names, source URLs, numeric units and observed_at.
- No sync cursor advertised; conservative bounded public fetches and user-requested snapshots.
- FTS/search: exact token/name/region filtering across facts, offline only after explicit sync.
- Store concise factual projections, never entire reports, contributor histories, browser sessions or credentials.

## User Vision
- Nationwide useful ski-area search, comparisons and dated evidence; initial Japan use case.
- Current date: 2026-10-03. Preseason, past seasons, report freshness and unupdated information remain distinct from live operations.
- Read-only planning only; publication already authorized.

## Product Thesis
- Name: snowjapan-pp-cli.
- A small factual planning surface for Japan ski travel, with enough evidence to compare resorts and reason about past season coverage without confusing it with current lift status.

## Build Priorities
1. Resolve replayable nationwide directory coverage and exact resort facts from HTML/SSR before generation.
2. Implement search, inspect, compare and historical season/date queries; validated regional report observations only where structured numeric data exists.
3. Bound requests, pages, bodies, outputs and wall time; fail clearly on source drift, unavailable routes and invalid filters.
4. Agent-native JSON/MCP, factual local store, meaningful fixture/domain tests and actual live matrix.

## Source Evidence
- https://www.snowjapan.com/ski-areas: active database excludes inactive ski areas; region lists are independently compiled and nonexhaustive.
- https://www.snowjapan.com/insights: historical snow depth on 350+ individual areas; daily snow depth chart begins November 2026.
- https://www.snowjapan.com/ski-areas-in-japan/nagano-prefecture/hakuba-village/able-hakuba-goryu: 1676m peak, 926m vertical, 750m base, 35/40/25% terrain split, 12 total lifts; 2026–27 information Not yet updated; historical 2025–26 season 4 December–3 May, 151 days.
- Public raw responses remain in temporary fetch cache only. Durable artifacts hold concise facts and field contracts.

## Resolved public data contract
- Source-owned national statistics chart has 478 canonical resort records. Current chart reference resolved from public Wix rich-content; public Flourish embed holds complete factual rows.
- 2025-2026 completed-season chart has 417 canonical records with exact first/last dates and inclusive elapsed days; rows with missing endpoints are excluded.
- Two bounded GETs per dataset suffice; no cookies, provider account, resident browser or fabricated API required.
