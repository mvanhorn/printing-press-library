# Phase 5 Acceptance Report: airbnb-vrbo-pp-cli

**Level:** Full Dogfood (live network)
**Date:** 2026-05-03
**Verdict:** **HOLD**

## Tests run

### Mechanical (PASS)
- `go build`, `go vet`, `go test ./... -short` — all PASS
- Help on every command (33+ subcommands) — all return exit 0 with real --help text
- Dry-run on every command — all return exit 0 with valid JSON
- shipcheck umbrella — 5/5 legs PASS, scorecard 75/100 Grade B

### Live (PARTIAL)
| Command | Status | Notes |
|---|---|---|
| `doctor` | PASS | Reports config OK, API reachable, cache fresh |
| `airbnb-listing search "Lake Tahoe"` | PASS | Returns 18 real listings with IDs, titles, ratings, badges, 15 pagination cursors |
| `airbnb-listing get <id>` | **FAIL** | Title returns "Select dates" placeholder; host, coordinate, raw_sections all empty. SSR detail-page parser is matching the wrong script tag or the cache shape on detail pages differs from search. |
| `vrbo-listing search "Lake Tahoe"` | **FAIL** | Returns empty output. Akamai warmup likely not firing or the GraphQL operation/payload is wrong. |
| `cheapest <airbnb-url>` | **PARTIAL** | Composes correctly (calls listing get + host extract + search backend) but propagates empty data from get. host=empty, all option totals=null, direct candidates=null. cheapest verdict reports `source=airbnb, total=0`. |
| `compare`, `match`, `plan`, `find-twin`, `host extract`, `host portfolio`, `watch *`, `wishlist *`, `fingerprint` | UNTESTED | All depend on the above clients working. Untested live. |

## Failures

1. **Airbnb listing detail SSR extraction broken** (`internal/source/airbnb/client.go::Get`)
   - The detail page has a different SSR structure than the search page. `firstByKey(root, "title")` is finding "Select dates" (Airbnb's date-picker section title) before the actual listing title.
   - Fix needed: walk `data.presentation.stayProductDetailPage.sections.sections[]` filtered by `sectionId` (listingTitle, hostProfile, location, amenities) per the openbnb extraction notes.

2. **VRBO search broken** (`internal/source/vrbo/`)
   - GraphQL operationName mismatch (we guessed `propertySearch`; actual may be `LodgingPropertyFlexibility` or similar) OR Akamai warmup not firing.
   - Fix needed: capture a real VRBO HAR (which we deferred earlier) OR iterate operationName guesses with codex sandbox testing OR drop GraphQL and use SSR `__PLUGIN_STATE__` extraction only.

3. **Cheapest propagates empty data** (`internal/cli/cheapest.go::computeCheapest`)
   - Even after #1 fix, the direct-site search step needs to actually fetch candidate URLs and extract prices. Currently `candidates: null` suggests the search backend integration isn't running or returning results.
   - Fix needed: add diagnostic logging, ensure DDG backend is hit by default when no API key set.

## Bugs already fixed in this session

- ✓ verify-skill (Use strings for `host portfolio` / `airbnb get` now declare `[name]` / `[id]`)
- ✓ Airbnb search ID extraction (now extracts numeric IDs via multiple-key fallback + Relay decode)
- ✓ Airbnb search primary_line/secondary_line stringification (now extracts `body`/`text` from structured objects, not Go map dump)
- ✓ VRBO + Airbnb search now accept positional `[location]`

## Recommendation

**HOLD.** The CLI's mechanical layer is solid (shipcheck PASS 5/5, scorecard 75/100 Grade B) but the live data-extraction layer needs another 1-2 hours of focused codex iteration. The headline `cheapest` command does not produce useful output today.

**Path forward:**
1. Run `/printing-press-polish airbnb-vrbo` after manually capturing a VRBO HAR (or use a different VRBO library — `markswendsen-code/mcp-vrbo` is a working DOM-based reference).
2. Fix Airbnb detail extraction by walking `stayProductDetailPage.sections.sections[]` with sectionId filtering (per the openbnb research).
3. Fix VRBO endpoint by either (a) HAR-confirmed operation names or (b) pivoting to `__PLUGIN_STATE__` SSR-only.
4. Re-run `printing-press shipcheck` and the live killer-path tests.

## What's preserved

- Full scaffold with 33 commands wired and registered
- Real Airbnb SEARCH client working live
- All shipcheck-passing structural code
- Pluggable web-search backend interface (parallel/ddg/brave/tavily) with implementations
- Host extraction logic (untested but present)
- Listing fingerprint logic
- Local store schema (watchlist, price_snapshots, hosts, listings_index)
- 9 novel commands wired in root.go
- Full README + SKILL.md + manifest

The working dir stays at:
`~/printing-press/.runstate/mvanhorn-eb6a05f2/runs/20260502-210359/working/airbnb-vrbo-pp-cli`

Polish skill or follow-up session can pick up here.
