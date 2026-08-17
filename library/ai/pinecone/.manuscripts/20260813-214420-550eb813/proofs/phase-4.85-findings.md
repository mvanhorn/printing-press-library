# Pinecone CLI — Phase 4.85 Output Review Findings

Source: printing-press-output-review sub-skill (Wave B — all findings warnings, non-blocking).

## Findings (all fixed in-session)

1. **check: aggregation accounting** (warning) — `cascade` dim-mismatch early return could skip per-index failure accounting for unresolvable/mismatched indexes.
   - **Fix:** per-index dimension validation added in the fan-out loop; unresolvable or wrong-dimension indexes are now recorded in `fetch_failures` instead of silently dropped.
2. **check: result ordering** (warning) — `cascade` merged results were ordered by channel-arrival (nondeterministic), not score.
   - **Fix:** `sort.Slice(merged, score desc)` before emitting.
3. **check: format** (warning) — `usage` empty-state emitted `"snapshots": null` while sibling commands emit `[]`.
   - **Fix:** initialize `snapshots`/`nsData` to empty slices.

## Verified clean
- Semantic relevance: text-query/cascade dimension-mismatch note is the correct graceful fallback (1536-dim index vs 1024-dim hosted models).
- No HTML entities, mojibake, or malformed URLs.
- snapshot/prune/coverage/check-vectors output shapes match purpose.
- usage/snapshot diff growth math verified plausible against live snapshots (2928 → 2928, delta 0).

## Verdict
All 3 findings fixed before ship. No remaining output-plausibility issues.
