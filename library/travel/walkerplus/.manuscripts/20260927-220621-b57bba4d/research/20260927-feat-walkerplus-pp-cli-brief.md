# Walkerplus CLI Brief

## API Identity
Walkerplus public Japanese event listings, https://www.walkerplus.com/event_list/. Read-only traveler discovery, no official API claimed; HTTP HTML and event JSON-LD. Live HTTP200 without credentials. Native listing page contains ten event cards, region/city/category/date routes and next-page links. Captures in discovery/.

## Reachability Risk
Low at present: raw HTML served without challenge; site is an undocumented changing contract. No wrapper/spec search required for website target. Public library registry checked: no Walkerplus entry. Native cache header max-age10800; record fetched_at independently.

## Users
- An agent planning a Japan itinerary across cities and dates, repeatedly gathering bounded candidates for a traveler.
- A traveler revisiting the shortlist near departure to check schedule exceptions, reservation requirements and the current event edition.
- A traveler looking for a free or explicitly indoor alternative on a given day; unknown attributes must stay unknown.

## User Vision
Focused Walkerplus-only CLI; truthful dates and source facts, bounded discovery, lightweight runtime, compact agent JSON. No broad multi-provider trip planner, booking action, database sync, or shared configuration edits.

## Top Workflows
1. Bounded deduplicated trip shortlist by prefecture/city, category and exact ISO date interval, explainable date/location/category relevance.
2. Starting-during and ending-during variants to catch limited opportunities.
3. Lazy detail by stable event ID/URL: venue, access, hours/exclusions, admission, reservation and organizer URLs.
4. Source-backed free/indoor filters and compact selected fields for agent pipelines.
5. Discover valid areas/categories instead of guessing source slugs.

## Table Stakes and Pain Points
Japan Guide event calendar https://www.japan-guide.com/event/?aMONTH=7&aYEAR=2026 supplies month/year and city events; JAPAN47GO https://www.japan47go.travel/en/event supplies detailed geographic/event criteria. Neither becomes a runtime source. Traveler pain points: ambiguous annual dates; Japanese detail hidden behind broad ranges; unbounded browsing. Walkerplus depth and faithful Japanese source facts are the differentiation.

## Verified Source Contract
Listing paths /event_list/10/ar0726/eg0055/ (Kyoto festivals October 2026), /event_list/ar0101/eg0102/ (Hokkaido fireworks), /event_list/ar0313113/shibuya/ and /2.html pagination. UI date values today, tommorow, weekend, MMDD, month; no arbitrary range/year control found. Filters are path segments. CLI must apply exact year/date checks locally, expose bounded coverage and never imply exhaustive discovery. Category/native names retained alongside English aliases.
Detail /event/ar0313e603640/ supplies Event JSON-LD startDate/endDate, Japanese name and Place address. Its offers.price is literal None despite paid admission: treat as unknown and parse displayed facts, never infer reservation availability from InStock. /data.html and /price.html have fuller facts. Overall start/end is an envelope, never proof every day is active; parse concrete exclusions/recurrence conservatively and retain raw schedule. Approximate seasons and unannounced editions remain distinct and cannot silently match future year.

## Data Layer
Entities: source event edition, source area/category route, derived trip match. Small bounded HTTP cache with TTL/refresh only; no database sync or background crawling. Missing scalar facts null; collections []; stdout compact bounded JSON, stderr diagnostics. Default lazy detail; enforce page/request/concurrency bounds.

## Product Thesis
walkerplus: a focused lightweight Go CLI for agent travelers that turns Walkerplus source evidence into honest, bounded trip candidates. Preserve original Japanese titles and clearly label derived match/sort fields.

## Build Priorities
search, shortlist, event detail, areas/categories. Exact interface and approved scope await phase08. Deterministic schedule/year-boundary/dedup/unknown-price tests plus live region/category relevance matrix and measured bytes/requests/latency/RSS. Checkout promoted to workspace root; no publication or shared configuration changes.
