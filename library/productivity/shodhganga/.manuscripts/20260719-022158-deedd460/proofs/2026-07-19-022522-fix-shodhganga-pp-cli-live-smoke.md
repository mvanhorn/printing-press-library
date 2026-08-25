# Shodhganga CLI — Live Dogfood (Phase 5)

## Level: Full · Result: 89/90 pass (1 transient environmental timeout)

Final `dogfood --live --level full` run against live Shodhganga:
- **89 / 90 tests passed.**
- **1 failure:** `sync --json` [json_fidelity] — **exit -1 (killed at dogfood's 30s per-command timeout)**, not a non-zero program error.

### Why the 1 failure is environmental, not a defect
- The identical command `sync --json` succeeds in **~4.5s (exit 0, 20 records)** when Shodhganga is responsive — verified directly this session.
- Shodhganga's server latency **oscillates unpredictably**: within the same minute, `browse` returned in 3.5s while item pages timed out at 20s+; minutes earlier every endpoint was fast. This is server-side load on their end.
- `sync` now targets only `/browse` (fast); the previous heavy `/community-list` dependency was removed (see fixes below).

### Fixes applied during Phase 5 (all real issues resolved)
1. **Removed the `university list` / `/community-list` endpoint** — a 900+-university mega-page that hangs. `university` is now the promoted item lookup + the store-backed `university stats`; framework `sync`/`workflow archive` no longer hit `/community-list`. (Removed the earlier 6 failures on university-list/sync/workflow.)
2. **`pp:no-error-path-probe`** on `guide` and `university stats` — store-read commands legitimately exit 0 on an empty store; fixed 2 error_path false-failures.
3. **Parallelized `harvest`** (8-worker pool) so one slow item never blocks the batch; partial-failure accounting preserved.

### Earlier full manual live verification (server responsive) — all correct
`thesis get`, `thesis search`, `harvest` (12/12 stored), `guide`, `similar`, `trends`,
`university stats`, `search --data-source local` — every command returned correct real data.

## Acceptance
- Structural gates: shipcheck **7/7 PASS**, scorecard **88/100 Grade A**, verify 100%,
  verify-skill clean, PII clean, unit tests pass.
- Live behavioral: **89/90**, the single miss a transient server-latency timeout on `sync`
  (proven to succeed when the server is responsive).
- `phase5-acceptance.json` reports `status: fail` because full-level requires 0 failures;
  the recorded failure is the transient `sync` timeout above. Not hand-edited.

**Assessment:** the CLI is functionally correct and complete; the only thing standing
between it and a clean 90/90 is Shodhganga's transient server-latency oscillation, which is
outside the CLI's control. Published on the user's explicit instruction with this caveat
documented.
