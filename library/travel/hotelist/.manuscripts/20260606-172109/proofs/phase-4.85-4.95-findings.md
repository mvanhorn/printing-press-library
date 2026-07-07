# Hotelist CLI — Phase 4.85 (output review) + 4.95 (code review) findings

## Phase 4.95 local code review (hand-authored Go) — MINOR FINDINGS, fixed:
- round1/round2 used `int(f*N+0.5)` (half-toward-zero for negatives) → switched to `math.Round` (affects watch-diff deltas). FIXED.
- watch.go loadBaseline + watch list: missing `rows.Err()` after iteration (silent partial baseline) → added checks. FIXED.
- No SQL injection (all parameterized), no nil-deref (pointer fields guarded), no divide-by-zero (price guards), no goroutine/leak issues.

## Phase 4.85 output review — WARN (Wave B, non-blocking), addressed:
- corridor (and value/rank-country) let price=0 ("unknown price") hotels survive `--max-price` and rank in value lists with no value score → added `dropUnpriced` to all value-ranked and price-filtered paths (`hasPriceFilter` auto-detect). FIXED.
- Geo-looseness: a Peniche hotel (~71km) appeared under "Lisbon" due to Hotelist's own geohash bucketing + adaptive-prefix widening. KNOWN LIMITATION — inherent to the site's geohash location model; documented. Not fixed in v1 (would require centroid+radius distance filtering).

## Retro candidate (generated code, not patched here):
- internal/cli/helpers.go `truncate()` slices on bytes, garbling multibyte (CJK/accented) names in human-table output only (JSON unaffected). Generated file → route to printing-press retro rather than patch-in-place.
