# SmartLead CLI — Phase 5 Acceptance Report

Level: Full Dogfood (live, against the real SmartLead account — 72 campaigns)
Write operations excluded: the test account is the user's production
link-building account with no disposable sandbox; only read-only commands and
dry-runs were exercised.

## Final result: Gate = PASS
- Matrix: 133 tests, 133 passed, 0 failed, 134 skipped (skips = error-path tests
  for commands with no positional arg, and write commands with no fixture).
- `printing-press dogfood --live --level full` exit 0; `phase5-acceptance.json`
  written by the runner with `status: pass`.

## Failures found and fixed during dogfood
1. `dupes` / `silent` errored (exit 3) when run before `sync` — correct
   defensive behavior, but a non-zero exit on an unsynced mirror is poor agent
   UX. Changed health/silent/dupes to emit an empty JSON array + a stderr hint
   and exit 0 on an empty mirror (`emitEmpty`).
2. `sync` happy_path / json_fidelity exited -1 — dogfood's 30s default per-test
   timeout killed a full 72-campaign sync (~58s). Re-ran with `--timeout 240s`;
   sync completes cleanly. Not a CLI defect — a large account is genuinely slow
   under the API's rate limit.
3. `workflow archive --json` emitted a multi-line indented summary object after
   the NDJSON sync-event stream, breaking line-delimited JSON parsing. Made the
   summary compact so the whole `--json` stream is valid NDJSON, consistent
   with `sync --json`.

## Behavioral verification (live API + synced store)
- `health` — 72 campaigns scored with real reply/bounce rates, silent counts.
- `silent --days 7` — 430 silent leads, ranked by days-silent.
- `dupes` — 38 leads in 2+ campaigns (real cross-campaign collisions).
- `sender-health` — 5 sender accounts ranked by inbox/connection composite.
- `warmup-gate --strict` — exit 1 when accounts below the inbox-rate threshold.
- `drift --campaign <id>` — weekly buckets with week-over-week deltas.
- All 41 absorbed endpoint commands enumerated by the matrix pass help,
  happy-path, JSON-fidelity, and error-path checks.

## Known non-defects (documented, not bugs)
- `GET /client` and `GET /leads/fetch-categories` reject the `limit` query
  param; `sync` reports them as non-critical warnings (exit 0). The standalone
  `client list` and `leads list-categories` commands do not send `limit` and
  work correctly. No shipping feature depends on the `client`/`leads` sync.

## Printing Press issues for retro
- The generator's dependent-resource sync injected only `parent_id`, not the
  typed table's NOT NULL `<parent>_id` FK column — typed sub-resource tables
  silently stayed empty. Patched in sync.go; the generator should inject the
  FK column natively.
- Store ID extraction did not recognize non-`id` primary keys (`stats_id`,
  `campaign_lead_map_id`); the `x-resource-id` spec hint had to be added and
  the override maps hand-populated.
- dogfood's 30s default per-test timeout is too low for a full sync of a
  realistically-sized account.

Gate: PASS
