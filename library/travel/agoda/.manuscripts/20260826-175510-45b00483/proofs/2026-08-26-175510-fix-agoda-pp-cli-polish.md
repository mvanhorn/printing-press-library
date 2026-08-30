# Agoda CLI polish pass

```
                    Before    After     Delta
  Scorecard:        82/100    86/100    +4
  Verify:           100%      100%      0 (36/36, no critical)
  Tools-audit:      2         0         -2 pending
  Output review:    6 warns   5 fixed, 1 deferred
  go vet / gofmt:   clean     clean
  PII:              0         0
ship_recommendation: ship
further_polish_recommended: no
```

## The critical find: dates were silently ignored
The Phase 4.85 agentic output review caught a bug that **every mechanical gate
missed**. `buildCitySearch` set `pricing.checkIn` and `pricing.checkout` but
never `pricing.localCheckInDate` / `localCheckoutDate` - and Agoda keys pricing
off the *local* pair. Those two fields stayed pinned to the captured template's
2026-10-15..10-17, so every search returned that date's prices regardless of
`--checkin` or `--nights`.

Property 1935 returned an identical 473.66 for check-ins five months apart.
After the fix: 473.66 / 376.63 / 626.79 across those dates, and `--nights 1/2/5`
gives 183.50 / 376.63 / 902.88 with consistent per-night math.

This defeated the CLI's entire value proposition while verify sat at 36/36 and
the scorecard read 82/100, because the output was well-formed and internally
self-consistent. It was independently re-verified in the parent pipeline before
being accepted.

The same root cause produced `free_cancellation_until` deadlines that sometimes
fell *after* check-in; both resolved together.

**Second critical find: every booking link was dead.** `agoda.html?hotelId=`
redirects to `pagenotfound.html`. Replaced with `partnersearch.aspx?hid=`, and
independently re-verified: all sampled URLs return HTTP 200 with dates,
currency, and occupancy surviving the redirect.

## Other fixes applied
- `meta.source` reported `local` on live network results, telling agents that
  freshly fetched prices were cached. Live commands now report `live`,
  `watch run` reports `computed`, offline `search` stays `local`.
- `--csv` emitted raw JSON on every novel command, because the shared CSV writer
  only renders top-level arrays and these emit an object-wrapped `.results[]`.
- `properties_sampled` was always 1 and actually counted city-level trend
  observations, implying the cheapest-date ranking rested on a single hotel.
  Renamed to `observations`.
- README and SKILL instructed `agoda-pp-cli auth login --chrome`, which is not a
  command in this CLI. Corrected to `AGODA_COOKIE` at the `research.json` source
  so the fix survives regeneration.
- MCP description for `compare` claimed an amenities comparison and a
  three-property cap; neither exists.
- Stale `--city-id` help cross-reference pointed at a non-existent
  `destinations resolve` subcommand (affected 6 commands).
- Removed 3 unreferenced generated helpers plus an orphaned type.

## Regression tests added afterwards in the parent pipeline
The date bug escaped because no test asserted that a varied input reached the
request. Two were added:
- `TestBuildCitySearchWritesLocalPricingDates` - table-driven across three dates,
  asserting `pricing.localCheckInDate` / `localCheckoutDate` match the caller.
- `TestBuildCitySearchDistinctDatesProduceDistinctRequests` - two different
  check-in dates must not serialize to an identical request body.
`TestBookingURLPreservesStayParameters` was also updated: it had been asserting
the now-known-dead `hotelId=` shape, and now pins `partnersearch.aspx?hid=` and
explicitly fails if `agoda.html` returns.

## Deliberately not fixed
- `booking_url` renders `&` as `&` in raw JSON. Verified cosmetic: both
  Python's json module and `jq` decode it to a literal `&`. Fixing it means
  editing generated `helpers.go`. Filed as a retro candidate.
- `vip delta` live-check "failure" is environmental (`AGODA_COOKIE` unset); the
  command correctly emits structured JSON and its declared exit code 4.
- Dogfood's "prices cheapest / vip delta look reimplemented" is a false
  positive; both reach the API client through `newAgodaClient()` indirection
  that the heuristic does not follow.
- `cache_freshness 3/10` is structural: Agoda exposes no bulk catalog endpoint,
  so the corpus accumulates cache-on-read by design.

## The lesson worth carrying forward
Verify was 36/36 and the scorecard read 82 while the CLI returned wrong prices
behind dead booking links. Rule-based gates cannot see this class of defect -
the output was plausible in every mechanical dimension. Only the agentic output
review caught it, which argues for treating that review as a required gate
rather than an advisory one for any CLI whose value is the data it returns.
