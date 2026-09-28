# tenki.jp scope manifest — approved

Focused local delivery; user brief overrides generic ecosystem breadth. No stubs, no publishing, no shared configuration changes. Root alone handles intent/architecture/review/acceptance; implementation and tests go directly to gpt-6-sol max.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Municipality/place resolution | tenki.jp search, leisure and mountain directories | tenki-pp-cli places search | Bounded candidates, category, canonical URL, ambiguity and forecast reference |
| 2 | Location detail | tenki.jp location pages | tenki-pp-cli places show | Attraction identity distinct from forecast municipality/elevation |
| 3 | Daily outlook | tenki.jp /10days.html; rrencanno/tenki_scraper weekly high/low | tenki-pp-cli forecast daily | Up to 14 actual source dates, confidence, null missing data, explicit units |
| 4 | Hourly detail | tenki.jp /1hour.html; algon-320/tenki period limits | tenki-pp-cli forecast hourly | True supported horizon, interval/instant semantics, past estimates distinguished |
| 5 | Seasonal discovery and detail | tenki.jp sakura/kouyou | tenki-pp-cli seasonal list; tenki-pp-cli seasonal show | Current report, prediction, normal, year and ended/unavailable statuses separated |
| 6 | Mountain detail | tenki.jp mountain pages | tenki-pp-cli mountain show | Named foothill reference and distinct numerical model levels/initialization |
| 7 | Efficient agent output/cache | User brief; rrencanno caching | (behavior in tenki-pp-cli forecast daily) compact JSON, field selection, refresh and explicit stale cache | Bounded output, stderr diagnostics, source freshness, request deduplication |

## Transcendence
| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Date/destination criteria matrix | compare | 10/10 | hand-code | Evaluates thresholds against bounded weather records with source values and assumptions | User brief, forecast FAQ | Compare explicit places/dates; forecast hourly inspects individual intervals. |
| 2 | Activity-window evaluation | compare --hours | 9/10 | hand-code | Selects complete hourly intervals and explicit instant endpoints for a JST activity window | Hourly source semantics | Whole-day daily values remain separate from window values. |
| 3 | Seasonal weather pairing | compare --season | 9/10 | hand-code | Joins seasonal spot evidence with its linked municipality forecast and separate time/year contexts | Active foliage and ended Sakura pages | seasonal show returns source report details. |
| 4 | Evidence coverage gate | compare | 10/10 | hand-code | Missing, stale, unsupported and level-mismatched evidence produces insufficient_data | User correctness brief, official location FAQ | No positive recommendation from missing evidence. |

Four custom behaviors share one compare command; all four require handwritten Go, zero are fully generator-emitted. Full-scope acceptance commits to all four. No universal score, inferred summit forecast, future-year seasonal invention, external provider, route optimizer, daemon, nationwide crawler or unrelated ski product.

## Bounds and semantics
- Search default 10, maximum 50 results, at most 2 explicitly selected pages; return pagination/truncation metadata. Japanese names/postal codes and canonical tenki.jp URLs are supported; arbitrary English geocoding is not promised.
- Compare 1–5 selected places over 1–14 source-covered dates; maximum 70 cells. Hourly default max 24 rows, hard cap 72; hourly availability is remainder of today plus two calendar days, not guaranteed 72 future hours.
- User criteria: maximum rain probability (%), rain amount/rate with explicit interval (mm / mm/h), temperature range (°C), maximum wind (m/s); deterministic ties and pass/fail/unknown. No threshold supplied means validation error rather than arbitrary default recommendation.
- Keep issue/init, valid/interval, fetch, expiry times in Asia/Tokyo, source URLs, forecast reference place and destination elevation. Source estimated past values are distinct from forecasts and observations.
- Cache forecast pages up to 1 hour; use product source age to expose staleness. Seasonal/mountain refresh cadence unverified: conservative one-hour TTL is a CLI policy, not provider cadence. Catalog cache up to 7 days. --refresh bypasses cache; stale output requires explicit selection and cannot earn a positive comparison.
- Current 2026 foliage is live-testable; Sakura 2026 updates ended. Current/future requests for unavailable seasons return explicit statuses; retained information is labelled historical/ended. Dated fixture tests cover in-season parsing and mixed/year boundaries separately from live verification.

## Validation and access
Live E2E across Hokkaido, Tokyo, Kyoto and Okinawa municipality samples; leisure, Fuji model/foothill and current foliage; ended Sakura; invalid and out-of-horizon dates. Fixture tests for missing values, 24:00/month/year rollover, mixed seasonal/weather timestamps, wrong-year sidebar and stale-cache boundaries. Benchmarks report cold/warm/refresh output bytes, HTTP requests, elapsed time and peak RSS. Ship only after required Printing Press checks, live acceptance and promotion; copy the promoted buildable checkout into the user workspace.

Technical access: unauthenticated public HTML currently works. No official tenki.jp API contract verified. Source terms Article 8(6) restrict non-browser/RSS acquisition; this limitation is explicitly part of the scope decision. No claim of provider authorization or supported API. HTML contract changes and unknown seasonal/model update schedules remain limitations.

User approved full scope including disclosed public HTML access limitation on 2026-09-27.

## Verified discovery adaptation during implementation
Leisure /leisure/search/?keyword=清水寺 returned403 in standard HTTP, Surf, and the site's own browser form. Readable /leisure/6/29/ returns11 supported attraction links. The same approved bounded places search capability therefore filters an explicitly chosen source directory (`--directory`, default selected-site root); it reports the directory, scanned count and incomplete coverage. Canonical leisure URLs still resolve. This changes source acquisition strategy, not the approved command families; no site-wide keyword coverage is claimed. User was informed. Evidence: discovery/leisure-reachability.json, leisure-browser-result.json, http/tenki.jp-leisure-6-29-5b60835e7dd04e3e.html.
