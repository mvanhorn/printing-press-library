# USGS Earthquakes — Phase 5 Acceptance Report

## Verdict
- **Gate: PASS**
- **Level:** Full Dogfood
- **Tests:** 89/89 passed (52 skipped — commands with no positional args don't get error-path tests)
- **Auth:** none (USGS endpoints are public; no API key required)

## Failures and fixes applied during Phase 5

1. **`feeds __printing_press_invalid__` returned exit 0** — promoted_feeds.go forwarded any positional arg to USGS as-is; FDSN returned a "404 File Not Found" body string but HTTP 200, so the CLI exited 0.
   - **Fix:** added `validFeedName` check in `internal/cli/promoted_feeds.go` (one-line guard); invalid feed names now exit 2 with a helpful error pointing at `feed-list`.

2. **`workflow archive --json` emitted NDJSON sync events on stdout** — the framework's `workflow archive` command called `syncResource` which writes per-event NDJSON lines to stdout, polluting the final JSON envelope.
   - **Fix:** redirected `os.Stdout` to `/dev/null` during the syncResource loop in `internal/cli/channel_workflow.go` when `--json` is set; restored before encoding the final summary envelope.

## Printing Press machine-improvement candidates (for retro)

- **R1** — `pageItemKeys` in `sync.go.tmpl` does not include `features`. GeoJSON FeatureCollection is a canonical wrapper key for any geo API (USGS, OpenStreetMap, ArcGIS, Mapbox); adding `features`/`Features` is a small, safe addition that would have made USGS earthquake sync work out of the box. Hand-patched in this CLI; should land in the template.
- **R2** — `wrapperArrayKeys` fallback at `internal/profiler/profiler.go:824-827` matches type names containing "RESULT" or "COLLECTION", which caused `CountResult` to be picked as a sync candidate alphabetically before `events.search` (`/count` < `/query`). Renaming the spec types unblocked it; the heuristic could be tightened or the picker should prefer endpoints whose name is in `collectionEndpointTerms` over arbitrary alphabetic order.
- **R3** — Per-param `default` values declared in a spec endpoint's params (`format: geojson` here) are honored by read commands but not by `sync`. Users have to pass `--param format=geojson` to make sync hit the right URL. A small generator change to propagate defaults would remove this footgun.
- **R4** — `workflow archive --json` emits NDJSON events from syncResource that pollute the final summary envelope. The template should silence per-event stdout when `--json` is set on the parent workflow command.
- **R5** — `feeds <invalid-name>` returns exit 0 with a "404 File Not Found" body because the FDSN feed paths return text bodies on missing feeds rather than HTTP 4xx that the client classifies. The generator could emit enum-based positional validation when the spec declares an enum on a positional path-substituted param.
- **R6** — The novel-features subagent emitted a wholly-static catalog command (`feed-list`) without the `// pp:novel-static-reference` marker. The subagent prompt should include guidance to apply that marker when the command body is purely a static table.
- **R7** — Phase 4.95's code-review autofix produced 6 high-quality fixes (command injection guard via `shellQuote`, sort.Slice replacement of hand-rolled bubble sort, dead-var removal, etc.). The retro candidates filed by that pass (R1-R4 in the agent's report) are worth wiring back into the novel-features subagent's Go-style coaching.

## Live behavioral evidence (samples)

- `recent --since 24h --min-magnitude 5 --limit 3 --json` → returned real M5+ events (Vanuatu M5.7, Solomon Islands M4.8, Fiji M5.7).
- `feeds significant_week --json` → returned the 2-event significant feed with full GeoJSON FeatureCollection.
- `near "37.77,-122.42" --radius-km 500 --since 7d` → returned ~120 km nearest event (Geyserville) with computed distance_km.
- `top --window 24h --limit 5` → composite ranking lifted Vanuatu M5.7 (9 felt, sig=503) above larger but unfelt events.
- `brief us6000synm --format markdown` → produced a complete agent-ready briefing with PAGER, DYFI, MMI, products inventory.
- `aftershocks us6000synm --radius-km 200 --days 30` → ran the local SQLite haversine query (0 aftershocks for a 5h-old event, as expected).
- `compare --region-a -123,37,-122,38 --region-b -119,33.5,-118,34.5 --window 30d` → computed delta -17 events (-26.55%), energy ratio 0.38.
- `decode-id us7000abcd` → parsed "us" + "7000abcd" → "USGS National Earthquake Information Center (NEIC)".
- `swarm-detect --window 7d --min-events 3` → scanned 80 events, no clusters meeting threshold (expected for sparse global data).
- `changes --since 7d` → reported "no revisions recorded yet" (correctly disclosed first-install state).

## Conclusion

The CLI is shippable. Both Phase 5 failures were fixed in-session (no defer-to-v0.2 patterns). 7 retro candidates filed for machine improvements that would lift the floor for the next geo/feed-shaped CLI.
