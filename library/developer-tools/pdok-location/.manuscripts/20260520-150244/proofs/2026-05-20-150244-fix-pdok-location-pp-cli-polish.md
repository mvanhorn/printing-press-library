# Phase 5.5 Polish Report

Polish skill ran in mid-pipeline mode (Skill-tool invocation, no `--standalone`). Publish Offer suppressed — main SKILL owns Phase 6.

## Delta

| Metric | Before | After |
|--------|--------|-------|
| Scorecard | 69/100 | 72/100 |
| Verify pass-rate | 100% | 100% |
| go vet | clean | clean |
| tools-audit | 1 pending | 0 pending |
| publish-validate | skipped (mid-pipeline) | skipped (mid-pipeline) |

## Files modified by polish

- `internal/cli/helpers.go` — removed dead `partialFailureErr`, `detectPartialFailure`, `partialFailureReport` (unused Google-Ads template carry-over)
- `internal/cli/root.go` — removed dead `allowPartialFailure` field + `--allow-partial-failure` flag
- `internal/cli/collections.go` — rewrote Short to a verb-led description listing all 13 OGC collections
- `internal/cli/promoted_lookup.go` — Short/Long rewritten from truncated Dutch fragment to English with Required/Optional/Returns
- `internal/cli/promoted_reverse.go` — same shape
- `internal/cli/promoted_suggest.go` — same shape
- `internal/cli/promoted_pdok-location-api.go` — replaced "This document" with a proper API-fetch description
- `internal/cli/novel_features_bbox.go` — strip `<b>...</b>` HTML markup from CSV output; emit stderr note when a requested collection returned 0 hits

## Polish-deferred findings (with rationale)

Polish skipped four scorecard-side findings as structural scorer mismatches:

1. **`path_validity 0/6` on dogfood** — scorer compares against bare spec paths, ignoring `servers[0].url`. The CLI's full path returns 200 from upstream (verified); the bare spec path returns 404. Scorer bug, not a CLI defect.
2. **`novel features reimplemented` flagged on `convert rd-to-ll` and `convert wkt-to-geojson`** — both are deliberately offline pure-math conversions. README/SKILL explicitly promote them as "no API call needed". Scorer flags absence of API/store access; intentional design choice.
3. **Scorecard live-check `convert wkt-to-geojson` substring failure** — scorer expects the WKT input text in the output, but the command emits GeoJSON. Heuristic mismatch.
4. **Scorecard live-check `search` empty result on fresh DB** — environmental (no sync yet), not a CLI defect.

## Verdict

`ship_recommendation: ship`
`further_polish_recommended: no` ("All mechanical gates are clean; residual scorecard gap is structural")
