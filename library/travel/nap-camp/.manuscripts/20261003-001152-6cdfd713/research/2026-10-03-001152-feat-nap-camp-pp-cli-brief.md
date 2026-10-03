# Nap Camp CLI Brief

## API Identity
- Domain: https://www.nap-camp.com; public Japanese campsite planning website. No official OpenAPI or matching CLI/SDK established in scoped searches; this is an unofficial read-only client.
- Users: campervan travelers comparing campsites, specific pitches, dates and source restrictions before booking directly.
- Data profile: facilities, public campervan-entry categories, plans/pitches, amenities, price-from labels, reservation status, reviews/ratings and canonical handoffs. No account, booking or payment features.

## Reachability Risk
- Low for observed public surfaces; ordinary native Chrome category and Kanto list loaded, raw category HTTP returned 200. Public contract may change. Existing shortlist checked 2026-10-02 reused and revalidated.
- Source advertised 2,612 campervan-entry campsites, 5,967 total facilities at observation; these are changing source counts.
- Specific plan 20005062 at facility 11007 says passenger-car entry, no AC power, 100–150 square metres, capacity 5; the category alone does not establish campervan entry for this pitch.
- Calendar labels: ○ 受付中, ▲ 残りわずか, × 受付終了, - 準備中, 待 キャンセル発生通知対象. Preserve these source semantics; acceptance status is not guaranteed vacancy.

## Top Workflows
1. Discover campervan-entry facilities by Japanese region and relevant source filters with bounded pagination.
2. Inspect a campsite and its specific plans for entry rules, pitch area, facilities, season, costs and canonical links.
3. Inspect publicly exposed dated acceptance/price evidence if replay semantics are verified; otherwise return dated handoff with explicit unknowns.
4. Compare a small shortlist and record unresolved vehicle dimensions, required services and dated totals.
5. Save observations and search a local itinerary shortlist without repeating source requests.

## Table Stakes
- Japanese names, stable facility/plan IDs, regions, ratings and canonical URLs.
- Facility filters for AC power, shower, laundry, waste disposal and pets; plan-level fields over facility-category inference.
- Source status and price qualifiers with JST dates and observation timestamps.
- Related official competitor sources: Carstay stations (https://carstay.jp/en/stations/) offers spot/overnight categories and starting prices; Kurumatabi search (https://www.kurumatabi.com/park/search.php) gives vehicle/size and facility evidence.
- No relevant Nap Camp wrapper/SDK found, therefore no top-wrapper issue tracker to inspect. Follow public assets/HTML contract.

## User Pain Points
- A broad vehicle-entry category can hide specific pitch restrictions.
- Facilities and opening/reservation dates differ by plan; starting price alone omits dated group/options totals.
- Multi-site itineraries need stable source evidence and a small list of unresolved checks.

## Data Layer
- Primary entities: campsite, plan, source observation and local shortlist.
- Sync cursor: explicitly bounded pages and limits; source site search is the authority, not a complete offline Japan database.
- Local JSON observations suffice for focused itinerary snapshots; generated store supports additional local search if useful. Keep timestamps/coverage.

## Product Thesis
- Name: nap-camp-pp-cli.
- Why it should exist: source-backed campsite-to-pitch planning with conservative vehicle and date interpretation, useful structured JSON and direct source handoff.

## Build Priorities
1. Public discovery with useful named region/facility filters and bounded JSON.
2. Campsite/plan detail and facilities/rules/fees with preserved Japanese source.
3. Verified date/calendar evidence or honest unknown and canonical handoff.
4. Bounded shortlist compare/snapshot workflows with vehicle-fit questions.
5. Deterministic contract tests, independent output/docs/code review, real live matrix and measured performance.
