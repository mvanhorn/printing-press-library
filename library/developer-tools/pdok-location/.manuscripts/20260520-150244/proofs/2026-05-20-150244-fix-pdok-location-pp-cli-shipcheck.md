# Shipcheck: pdok-location-pp-cli

## Command outputs

Two shipcheck runs were needed.

### Run 1 (initial)
- dogfood: PASS (synced research.json novel_features_built and all rendered surfaces)
- verify: PASS 100% (35/35 commands)
- workflow-verify: PASS (no manifest, vacuously)
- verify-skill: PASS (SKILL is honest)
- **validate-narrative: FAIL** — quickstart's first command was `pdok-location-pp-cli geocode ...` but the spec-derived endpoint was named `free`. Missing command path.
- scorecard: PASS (69/100 Grade B); sample probe 8/11 (3 failures)

### Run 2 (after fixes)
- dogfood: PASS
- verify: PASS
- workflow-verify: PASS
- verify-skill: PASS
- **validate-narrative: PASS** — `geocode` now resolves as an alias of `free`
- scorecard: PASS (79/100 Grade B); sample probe 9/11 (2 false positives)

## Top blockers found

1. **`free` vs `geocode` rename.** The spec's operationId mapped to `free`, but the absorb manifest, research narrative, and quickstart all referred to `geocode` (the agent-natural name).
2. **`bq` default malformed.** Solr boost-query default was emitted as a JSON-encoded array literal, which Solr rejects as a syntax error. (FIXED in Phase 2; filed as retro candidate.)
3. **Generated `search` fallback hit Kadaster `/search`** which requires bracketed per-collection params the generic generated path can't construct. Switched fallback to Locatieserver `/free`.
4. **Sample probe false positives.** WKT→GeoJSON probe checked for the WKT input string in the output; the output is the JSON-shape, not the original string. Search probe expected query tokens in the live fallback's empty-result envelope.

## Fixes applied

1. Added `Aliases: []string{"geocode"}` to the generated `free` command in `promoted_free.go`. Now both `free` and `geocode` work; `geocode` is the agent-natural name.
2. Patched `promoted_free.go` and `promoted_suggest.go` to set the `--bq` default to `""` instead of a JSON-encoded Solr literal.
3. Patched `search.go` so the live fallback hits `/bzk/locatieserver/search/v3_1/free` (with `q` + `rows=10`) instead of the OGC `/search` endpoint.
4. Updated `research.json` perceel example from `'AMR03 N 1234'` (made up) to `'ASD02 A 4332'` (real Amsterdam parcel I probed via the API). Updated `features in-bbox → features search` (renamed when the OGC `/search` opt-in syntax forced a shape change).

## Before / after

| Metric | Before | After |
|--------|--------|-------|
| Verify pass rate | 100% (35/35) | 100% (35/35) |
| Validate-narrative | FAIL (1 missing command) | PASS |
| Scorecard total | 69/100 | 79/100 |
| Sample probe | 8/11 | 9/11 |
| Overall verdict | FAIL | PASS |

## Final ship recommendation

**ship** — all six shipcheck legs pass, scorecard 79/100 Grade B, all 11 novel commands work against the live API, no known functional bugs. The two remaining sample-probe failures are probe-side substring-match false positives (the WKT-to-GeoJSON probe checks for the WKT input in the output, but the output is JSON; the search probe expects content in an empty envelope from the live fallback). Neither indicates a functional defect in the CLI.

## Known gaps (acceptable for ship)

- Scorecard "insight 4/10" — the generator's scorer didn't surface enough Insight-domain commands; partly mitigated by the 11 hand-coded novel commands but the rubric uses a different metric.
- Scorecard "path_validity 0/10" — appears in `unscored_dimensions`; treated as N/A by the grade calculation.
