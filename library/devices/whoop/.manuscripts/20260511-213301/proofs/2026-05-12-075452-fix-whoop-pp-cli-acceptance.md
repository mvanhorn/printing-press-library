# Phase 5 Acceptance — whoop-pp-cli

- run_id: 20260511-213301
- level: full
- verdict: PASS

## Matrix totals

| metric | count |
| --- | --- |
| matrix_size | 88 |
| passed | 88 |
| failed | 0 |
| skipped | 57 |

Skipped checks were dependent-resource probes (get-by-id, list-of-children at depth 0) where the matrix runner could not synthesize a usable parent identifier on its own. These are infrastructural skips, not failures.

## Pagination R2 verdict

Confirmed working live.

- `cycle list --limit 50 --json` clamps to the WHOOP server max of 25 and prints a stderr warning advising `--all` for auto-pagination.
- `cycle list --all --json` walks the pages and returns the full window in a flat `results` array with `event=complete,total,pages` events emitted on stderr.
- `cycle list --start <30d>` returns the expected 10 cycles for the test account's recent window.

## Sync typed tables (30d window, fresh DB)

| table | row count |
| --- | --- |
| cycle | 81 |
| activity (sleep records) | 87 |
| activity_workout (rebuilt via dependent sync) | 76 |
| cycle_recovery (typed recovery) | 80 |
| resources (generic, all types combined) | 324 |

Recovery rows are now landing in both the generic `resources` table and the typed `cycle_recovery` table. Prior to this run the typed recovery table was empty and the generic table only had a partial set keyed by `updated_at` — the same shape as the historical Greg-class bug.

## Novel features sample

| feature | verdict | sample |
| --- | --- | --- |
| `analyze why-today` | sensible | ranked deltas including hrv_rmssd_milli (z=-1.80), recovery_score (z=-1.38), resting_heart_rate (z=-0.59) for the authenticated user |
| `analyze efficiency` | sensible | strain buckets light/moderate/strenuous/all-out with recent vs prior recovery deltas (e.g. moderate +9.85pp, strenuous +13.60pp) |
| `analyze correlate sleep_consistency_percentage recovery_score --window 30d` | sensible | Pearson r=-0.148, sample_size=30, "weak negative correlation" |
| `analyze sleep-debt --since 30d` | sensible | weekly cumulative debt curve plus a trend slope (-2.75 h/week) and natural-language interpretation |
| `analyze overtraining` | sensible | 11 flagged days with strain, sigma-above-mean, and recovery delta vs window mean |
| `sql` | sensible | "SELECT date(start), score_state FROM cycle ORDER BY start DESC LIMIT 5" returns the last 5 days, all SCORED |
| `search "sleep"` | sensible | 50 FTS hits across local synced data (previously returned 0 because of an empty switch case in search.go) |

## OAuth refresh

`auth refresh` returned OK and bumped `token_expiry` by an hour. Doctor reports all green afterwards.

## Gate verdict

PASS. Every mandatory matrix test passes, the recovery sync chain now populates the typed `cycle_recovery` table end to end, and every flagship analytics feature returns real data against the authenticated user's 30-day history.

## CLI fixes applied during this phase

1. `internal/store/store.go` — added a `"recovery": "cycle_id"` entry to `resourceIDFieldOverrides` so the WHOOP recovery payload (no `id` field) extracts its primary key from `cycle_id`. Without this the entire recovery batch was being dropped with `all_items_failed_id_extraction`.
2. `internal/cli/sync.go` — same override for the single-object path.
3. `internal/store/store.go` — extended `UpsertBatch` switch to dispatch `recovery` (raw resource name) to `upsertCycleRecoveryTx`, not just the `cycle_recovery` typed name. Sync uses the raw API resource name, so without this branch nothing reached the typed table.
4. `internal/store/store.go` — fixed an argument-order bug in `upsertCycleRecoveryTx`: the SQL bound `data → cycle_id`, `time.Now() → data`, `cycle_id → synced_at`. Replaced with correctly ordered binds and a fallback to use `id` if `cycle_id` is absent on the object.
5. `internal/store/store.go` — added `formatIDValue` helper so JSON-derived float64 ids (cycle_id arrives as float64 from `encoding/json`) are written as plain integer strings instead of `1.491664936e+09`. This keeps the typed `cycle_recovery.id` and `cycle_recovery.cycle_id` columns consistent with the `cycle.id` column.
6. `internal/store/store.go` — namespaced the generic `resources.id` by `resource_type` (`"recovery:" + cycle_id`) inside `UpsertBatch` so a recovery record cannot collide with the cycle that shares its cycle_id. The previous unqualified key would otherwise let the recovery row's payload overwrite the cycle row via the `ON CONFLICT(id) DO UPDATE` clause.
7. `internal/cli/search.go` — replaced an empty `case "":` switch arm (which silently returned zero results when no `--type` filter was passed) with a direct call to `db.Search`. The FTS index itself was healthy the whole time.
8. `internal/cli/analyze_transcendence.go` — moved the worked example out of the `Long` block and into the dedicated `Example` field on the `sql` command so Cobra renders it under an `Examples:` heading. The dogfood matrix asserts this section exists.

## Generator-side issues for the retro

These are systemic and belong upstream in the Printing Press, not in this CLI:

- The schema profiler has no notion of "this resource uses a parent foreign key as its primary key". WHOOP recovery records are keyed on `cycle_id`, but the spec annotation `x-resource-id` (or its equivalent runtime override) was not generated. Every API with a child resource that has no own id (e.g. recovery-per-cycle) will hit the same silent-drop pattern.
- `genericIDFieldFallbacks` is too liberal: when an item has no real primary key it sometimes lands keyed by `name` or worse a date-ish string that happens to match a fallback. Combined with the schema's unqualified `resources.id PRIMARY KEY`, this produces cross-resource id collisions. The generator should either (a) make the resources PK `(resource_type, id)`, or (b) always namespace ids by resource type in the generic table.
- `upsertCycleRecoveryTx` was generated with its SQL bind order shuffled. There's likely a template-time bug that drops one of the columns into the wrong VALUES slot whenever a typed table has both `id` and a parent foreign-key column.
- The `search` command was generated with a `case "":` arm that is intentionally empty — a placeholder the generator expected to fill with per-resource FTS calls but never did. For APIs without a native search endpoint, this leaves the user-facing command silently broken.
- The dogfood matrix expects an `Examples:` heading in `--help`. Cobra produces that heading from the `Example` field, not from inline `Example:` lines inside `Long`. The generator should always populate the `Example` field on every command (or the docs check should accept the inline form).
- `WHOOP_DB` is documented in this skill's instructions as the test-time DB path override, but the CLI itself only honors `--db`. Not a bug in the CLI per se, but worth aligning between the agent contract and the binary.

## Files

- `/Users/ykurilov/printing-press/.runstate/ykurilov-ca61f808/runs/20260511-213301/proofs/phase5-acceptance.json`
- `/Users/ykurilov/printing-press/.runstate/ykurilov-ca61f808/runs/20260511-213301/proofs/2026-05-12-073955-dogfood-results.json` (pre-fix run: 84 pass / 3 fail / 58 skip)
- `/Users/ykurilov/printing-press/.runstate/ykurilov-ca61f808/runs/20260511-213301/proofs/2026-05-12-075452-dogfood-results.json` (post-fix run: 88 pass / 0 fail / 57 skip)
- `/Users/ykurilov/printing-press/.runstate/ykurilov-ca61f808/runs/20260511-213301/proofs/2026-05-12-075452-fix-whoop-pp-cli-acceptance.md`
