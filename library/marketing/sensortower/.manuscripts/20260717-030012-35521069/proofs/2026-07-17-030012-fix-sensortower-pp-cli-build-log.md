Manifest transcendence rows: 5 planned, 5 built. Phase 3 gate PASSED.

## Built

Priority 0 — data layer: generator-emitted store (`resources` + FTS5 + learn tables).
Only `categories` is classified syncable: every other endpoint requires a param
(app_id / category / country) that a bare `sync` cannot invent. Consequence: the
rank-history mirror is written by `movers` itself (`rank_snapshot` rows keyed
`<os>:<category>:<country>:<chart>:<date>:<app_id>`), not by `sync`. This is the correct
shape for this API — you snapshot when you look.

Priority 1 — 19 absorbed features, all generator-emitted from the spec. No stubs.

Priority 2 — 5 transcendence features, all hand-coded (~1,164 lines + novel_shared.go):
- `movers <category>`     324 lines — 1 API call; in-payload `previous_rank` delta +
                                      new-entrant set-difference vs prior local snapshot
- `teardown <app-id>`     311 lines — 1 API call; versions[] × category_rankings, cadence stats
- `watch` (add/list/rm/digest) 183 — digest makes ZERO API calls by design (rate limit)
- `divergence <category>` 167 lines — 1 API call; free-rank vs grossing-rank join
- `compare <ios> <android>` 179 lines — 3 calls; cookie-gated unified resolve first (fail fast)

## Bugs found and fixed during Phase 3

1. **`date` is required on category_rankings** (upstream 422 `{"date":["can't be blank"]}`).
   My spec had it optional — the browser capture always supplied one, so the requirement
   was invisible. The generated `rankings ios` command shipped broken (422 on a bare call).
   Fixed at the spec, regenerated; now a clean `required flag "date" not set`.
2. **`find` and `categories` are leaf commands, not parents.** The generator promotes
   single-endpoint resources. `find apps twitch` would have swallowed `apps` as the search
   term. Fixed the manifest rows and every narrative example to `find <term>` / `categories`.
3. **`sync --resources rankings` fails** — rankings is not syncable (required params).
   Quickstart and troubleshooting rewritten around `movers` establishing its own baseline.
4. **Chart depth is clamped to 25 rows** regardless of `--limit` (verified at `--limit 100`).
   Commands report the *applied* depth, not the requested one.

## Findings that contradicted the brief (from implementation)

- `versions[]` is DESCENDING, not ascending — sorted explicitly rather than trusting order.
- The iOS hub object has no `humanized_*` fields (only `{"unit","type","value"}`), unlike
  chart rows which do. `compare` humanizes the bucket into a label string rather than
  printing a precise-looking integer.
- Android uses `version` (not `current_version`) and a package-string `app_id`; IDs are
  carried as `json.RawMessage` so both platforms round-trip with correct types.
- In category 6015 only 1 of 25 apps overlaps free+grossing. Real, not a join bug — a
  consequence of the 25-row clamp.

## Intentionally deferred

- Deeper chart coverage via offset paging: forbidden by the 1-call-per-command budget
  against a ~13-request / 240s-recovery rate limit.
- `rankings` local caching: the response is an object (`data.{free,grossing,paid}`), not an
  array with IDs, so the generic cache path cannot key it. Novel commands own their
  snapshots instead. Generator emits a warning here, which is accurate.

## Generator limitations observed

- Single-endpoint resources are silently promoted to leaf commands; the manifest/narrative
  had no way to know this before generation. (Retro candidate.)
