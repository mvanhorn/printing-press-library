# NAVITIME CLI architecture and acceptance

Root owns this plan and acceptance. Implementation/test workers are gpt-6-sol at max effort, at most two active workers with disjoint file ownership. All work stays in the current Printing Press staging directory until promotion; shared configuration and existing artifacts remain intact.

## Product boundary
NAVITIME Japan Travel website only. Cookie-free Surf Firefox HTTP was live verified. Exclude paid API adapters, resident browsers, booking, live status, seat availability, map rendering and multi-service/multi-day composites. Capability metadata explains credential-gated/unverified alternatives without exposing stubs as working commands.

## Commands proposed for approval
- `places search QUERY [--type station|spot|all] [--limit 5]`: public bilingual candidates; raw source IDs, explicit refs, kind, coordinates/units, source URL and ambiguity. Bound requests/results; no synthetic pagination where the endpoint has none.
- `routes search --from REF --to REF` with exactly one of `--depart-at`, `--arrive-by`, `--first-on`, `--last-on`, plus one optional `--pass ID`: compact alternative summaries. REF is a source station/spot reference returned by lookup, avoiding implicit name selection.
- `routes show ROUTE_ID`: stored route snapshot detail, including ordered point/move legs, full dates, line/service text, walking, transfers, fare groups and optional seat supplements. No repeat network request just to expand details.
- `routes compare` with the same route-query flags: compare only returned alternatives by duration, fare, walking and transfers; optional local maxima filter results without claiming global optimality. Unknown metrics never become zero or rank as best.
- `passes list`: source-advertised pass IDs and labels with freshness, extracted from the same route-result HTML; distinguish advertised support from representative live-tested constraints. At most one pass per anonymous query. Reuse a cached catalogue from route queries, or fetch one dated Tokyo–Kyoto reference route to refresh it; record that exact source URL and request count. The entry page is not a runtime dependency.
- `capabilities`: verified, advertised, credential-gated and unavailable features with source URLs and research date.

## Data boundaries
`internal/navitime` owns the transport, source parsing, validation, domain records, cache and consequential tests. `internal/cli` adapter owns flags, compact JSON, projection, stderr errors/metrics and help. Root owns README/SKILL and review. The implementation can refine exact function signatures after the generated package layout is visible; maintain one owner per file.

Source routing is GET `/en/area/jp/route/result/`. Captured parameters are `start`, `goal`, spot `startCode`/`goalCode`, `date_time`, `search_time_mode`, `pass_list`. Verify effective `shapeParams` matches every requested constraint. Reject ignored constraints, challenge/login HTML, malformed bodies and empty shells with typed errors. Keep source URLs canonical and source IDs strings. Routes have source alternative positions but no verified globally stable ID: give snapshots local content-derived IDs and label that provenance; source IDs remain null where absent.

Datetime inputs must include explicit date and time for depart/arrive; offset-free inputs mean Asia/Tokyo and output uses +09:00. First/last take a date. Prefer source calendar UTC span to anchor overall dates, then order leg times with validated midnight rollover; never assume every leg shares the departure date. Timestamp inference must be labeled and inconsistent source spans rejected. Timetable data is not live service status or seat availability.

Fare parsing preserves displayed source total, base-fare grouping, default seat choice, alternative supplements, IC values and source caveats. Through-fare groups must not be added once per leg. Missing prices and passenger assumptions remain null/unknown. A pass label is not proof of full coverage or zero out-of-pocket cost. Preserve source extra-fare/exclusion text, including Nozomi/Mizuho warnings. Source-advertised pass filters do not calculate whether buying a pass is economical.

## Efficiency contract
Compact JSON by default; pretty output opt-in. Support dotted `--select` and `--fields` alias, bounded results (route default 3, lookup default 5, source cap retained), separate detail commands, explicit nulls and units. Diagnostics/errors and optional metrics go to stderr. No stdout banners.

Use request context deadlines, one in-flight request by default, bounded body size, no cookie jar, bounded retries only for transient failures/429, and capped Retry-After handling. Cache normalized public responses, not browser sessions: lookup/pass catalog 24h, routes 5min proposed TTLs; `--refresh` bypasses read cache, `--no-cache` skips both reads/writes. Expose fetched_at, cache hit/age, and source_updated_at=null when unknown. Snapshot detail retains original fetched_at and describes its stored nature. Cache filenames use hashes rather than untrusted path fragments.

## Acceptance
Each approved capability requires a read-only live query using the actual built binary. Cover English/Japanese ambiguity, station and POI routes, depart/arrive, first/last, a midnight boundary, base+seat fare alternatives, and representative pass behaviour. Deterministic tests cover dates, fare grouping/optional amounts, ambiguity, response rejection, unknown metric comparison, projection and cache refresh semantics. Validate decoded output, not only HTTP status/build success.

Measure cached and uncached command output bytes, request count, wall latency and peak RSS. Use isolated cache dirs and a bounded live matrix. Run Printing Press's receipt-controlled generation, shipcheck, review, dogfood/live acceptance and promotion gates; a passing helper is not product acceptance. Deliver a standalone buildable checkout at the user's workspace after promotion without copying nested run caches into source.
