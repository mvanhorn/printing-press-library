# Carstay overnight planning CLI brief

## API identity and user vision
Carstay public overnight stations at https://carstay.jp/ja/stations/ and https://carstay.jp/en/stations/. Build a read-only Japan campervan itinerary planner: discover designated overnight spots, inspect facilities and rules, compare options, assess explicit constraints, and hand off to canonical booking pages. No booking, payment, account, or authentication operation.

## Source evidence and reachability
A local provider shortlist followed by ordinary native Chrome navigation and DOM/accessibility inspection verified 397 Japanese versus 103 English search results on 2026-10-03 JST. English coverage is incomplete. Both public pages and their observed source bundles return HTTP 200 using ordinary curl. DOM-embedded scripts are temporary discovery evidence; public environment fields and any keys are excluded from durable artifacts. No official developer API or Carstay wrapper was identified in focused web research; there is no wrapper repository whose issue tracker is applicable. The CLI must use observed replayable HTTP/HTML surfaces only.

## Top workflows
1. Find overnight stations near an itinerary coordinate or prefecture and preserve source language and coverage.
2. Read a station's original name, stable ID, price-from, facilities/options/nearby amenities, dimensions and rules.
3. Compare a small shortlist on price and requirements while exposing unknowns.
4. Evaluate a vehicle and facility requirements against explicit source evidence.
5. Generate a canonical booking handoff URL with JST dates. Verify date controls and actual vacancy semantics before offering any dated inventory claim.

## Table stakes and competitors
Kurumatabi provides vehicle-compatible overnight parks, vehicle dimensions, toilets, electric power, bathing proximity and rules. Nap-camp offers campervan-entry campsites and dated plans. Carstay's distinct value is designated stations and camping spots with station-specific descriptions and booking handoff. Avoid confusing generic vehicle access or a tent category with approval for every vehicle/pitch. Focused web search found Carstay's official usage guide (https://carstay.jp/ja/usage/): users inspect calendar vacancies, dates, options and payment in provider booking flow; that guide does not establish an unauthenticated vacancy API.

## Data layer
Primary entity: station keyed by stable source ID, canonical path and source language. Cache public search/detail observations locally for repeated route planning, with observation timestamps and explicit freshness. Local filtering/shortlists need Japanese text preservation and bounded JSON; no cookie store. Prefer targeted data fetches and bounded, explicit page scope to broad scrape.

## Product thesis
`carstay-pp-cli`: agent-friendly discovery and comparison of Japan's designated overnight spots, with honest evidence about facilities, prices and vehicle constraints. Install when the public website's partial English coverage and many page visits make itinerary decisions cumbersome.

## Build priorities
Search/detail/compare/fit/booking-handoff. Date query results are not automatically dated vacancy. Starting prices are not complete dated quotes. Unknown facilities, dimensions, availability and fees remain unknown. Preserve Japanese names, IDs, canonical URLs, JST dates, numeric units, language coverage, observation time and empty arrays. Meaningful deterministic contract tests plus live read-only dogfood and measured representative timings.

## Users
- A self-drive Japan campervan traveler repeatedly choosing designated overnight stops near a planned route, balancing power, toilets, bathing and explicit vehicle constraints.
- An English-speaking planner who needs Japanese-only options preserved and must compare translated listings without assuming English covers Japan.
- An itinerary agent narrowing many source pages into a small decision table and passing a booking link to the traveler.

## Verified discovery
Ordinary native Chrome date flow selected 2026-10-10 check-in and 2026-10-11 check-out; Japanese results changed from 397 to 134. Canonical first detail https://carstay.jp/ja/stations/kinki/station/632c59b82b614b99a252d1b2/ received both date query parameters. Source list JSON also reports total=134 for that query. Calendar detail contains recurring slots and orders, whose full booking calculation has not been independently replicated. Dated search is useful provider-filtered candidacy, not a guaranteed vacancy or complete quote.
