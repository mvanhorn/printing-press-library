# Haneda Airport CLI brief

## API identity
- First-party public website: https://www.tokyo-haneda.com/en/flight/index.html . Anonymous website contracts, not a published supported API.
- Users: travelers checking flight status, people collecting arrivals, agents planning terminal/gate/counter handoffs.
- Observed public source: POST `/en/app/api/v2/flight/search` with flightType 1 domestic / 2 international, arrivalType 1 departure / 2 arrival, searchDt YYYYMMDD, airportCodes, airlineCodes, flightNumber, status numeric array. Metadata supplies English and Japanese airport/airline names.
- Native Chrome showed search controls and notices; public JS revealed replayable contracts. Native CDP event capture listed the page assets and metadata URLs, with credential headers omitted. Subsequent contract probes returned HTTP 200 for all four boards and five metadata/summary surfaces without cookies, keys, accounts or a browser runtime.

## Reachability risk
- Low observed transport risk, ongoing undocumented-contract/schema risk. 2026-10-02 live source returned domestic departures 503, domestic arrivals 502, international departures 159, international arrivals 161. Bodies <= 876118 bytes and successful requests < 1.7 seconds. Evidence: `discovery/contract-probes.json` and public raw responses.
- Source warns airline data may be late/inaccurate during disruption and search results may include adjacent service days. Search lists do not update automatically.
- Browser request inspection was abnormally slow; avoid further optional CDP. Ordinary DOM and already observed public assets are sufficient.

## Top workflows
1. Search a dated domestic/international arrival/departure board by flight, airline, destination, status and terminal; return a bounded slice with source timestamp, total matches and continuation offset.
2. Resolve a flight number or stable group ID to detail, keeping every marketing code in the source group and exposing terminal, gate, counter/security/exit metadata plus canonical source/map/airline handoffs.
3. Inspect provider-reported delays/cancellations, with source summary separate from filtered board counts and unknown/blank states preserved.
4. Save and compare timestamped local snapshots for changed status/time/gate/counter/terminal facts; no polling or notification side effects.
5. Inspect the first-party published monthly schedule index where supported; a schedule does not establish seats or a guaranteed connection.

## Table stakes and competitors
- [Haneda flight search](https://www.tokyo-haneda.com/en/flight/int_search.html) supplies route, airline, number and status filtering, codeshares and operational facilities.
- [HANEDA Navigator](https://www.tokyo-haneda.com/en/other/haneda__navigator_appli.html) advertises flight/facility search, bookmarks and navigation. Account favorites/notifications are outside this anonymous CLI.
- [FlightAware live search](https://www.flightaware.com/live/?hide_header=1) offers airline/number and airport/route search plus delays/cancellations. No additional data provider is included.
- Incumbent pains: multilingual GUI data is awkward to query, marketing codes can be counted as separate services, midnight rollover can invert a naive delay calculation, missing gates/statuses can look falsely confirmed.

## Data layer
- Flight group has stable identity from service date, domestic/international, direction, original ordered primary flight and scheduled time; marketing identifiers retained separately.
- Source does not label an operating carrier explicitly in the search payload. First displayed airline remains source primary; only label operating identity when another verified source establishes it. Preserve source ordering before any filters.
- Minimal user-directed JSON snapshot store with observed_at/source_updated_at and coverage, plus local comparison. No silent cache of dynamic gates/status; bounded catalogs can be fetched per query as needed.

## User vision
Implement the authorized read-only Haneda travel scope through direct builder work and exactly one independent fresh-context MAX reviewer. No reservations, payments, accounts or live browser transport. Preserve scheduled/revised/actual time semantics, JST service-day rollover and adjacent days, source freshness, explicit unknowns and canonical handoffs.

## Product thesis
Haneda Airport CLI turns first-party flight boards into precise, compact planning facts. It preserves source group identity and tells an agent what is known, stale, changed or unavailable without manufacturing seat inventory or connection promises.

## Build priorities
1. Four live boards, strict input/date validation and bounded HTTP/output budgets; meaningful schema/error checks.
2. Normalized exact JST scheduled and changed dates/times; separate changed/revised source field from actual (unknown unless explicit) and retain raw labels.
3. Search/catalog/detail/disruption and terminal planning with Japanese names, source URLs and explicit unknown fields.
4. Useful bounded snapshot/diff and published schedule discovery if verified from the public site.
5. Consequential parser/filter/codeshare/rollover tests, mandatory live dogfood, one independent review with fixes, Press checks and atomic local promotion.
