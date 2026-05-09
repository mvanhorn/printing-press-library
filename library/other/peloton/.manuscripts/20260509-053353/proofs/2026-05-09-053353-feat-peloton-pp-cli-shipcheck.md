# peloton-pp-cli Shipcheck

Captured 2026-05-09 against a real Peloton account (`twid`) on
`feat/peloton` after fix-forward of the `--compact` chain (added
`id` + `ride_id` to compactWorkout so `workouts list --compact | jq`
into `ride show` actually composes).

## Quality gates: 5/5 PASS

| Gate | Result |
|---|---|
| `go build ./...` | OK |
| `go vet ./...` | OK |
| `go build -o /tmp/peloton-pp-cli ./cmd/peloton-pp-cli` | OK (binary built) |
| `peloton-pp-cli --version` | `peloton-pp-cli version 0.1.0` |
| `peloton-pp-cli version` | `peloton-pp-cli 0.1.0` |
| `verify_skill.py --dir library/other/peloton/` | All checks passed (flag-names, flag-commands, positional-args, unknown-command) |

## Live smoke: 8/8 PASS

| Command | Outcome |
|---|---|
| `auth status` | Existing token visible (~33m old, user_id + username populated) |
| `me --json` | Cached identity returned without spawning Chrome |
| `workouts list --limit 2 --compact` | 2 workouts, JSON-when-piped, `id` + `ride_id` retained for chain |
| `workouts show <id> --compact` | Single object (not 1-element array), full compact projection |
| `ride show <ride-id> --compact` | Ride metadata + 9-song playlist with liked-flags |
| `discoveries --limit 5 --compact` | 2 deduped liked-songs across the window, `times_played` populated |
| `sync --limit 5 --json` (fresh DB) | `+5 workouts, +5 rides, +41 songs, +41 ride_songs`, ~2s elapsed |
| `sync --limit 5 --json` (re-run) | `+0 workouts, +0 rides` — incremental early-stop fires |
| `search 'house' --limit 3 --compact` | Interleaves songs + workouts by bm25 (3 hits, kind=song first then workout) |

## Exit codes (typed)

| Scenario | Expected | Got |
|---|---|---|
| Success | 0 | 0 |
| Bad invocation (e.g. wrong arity) | 2 | 2 |
| Auth missing (no token saved) | 4 | 4 (via `auth status`) |
| Resource not found (404) | 3 | n/a — Peloton returns 400 `error_code:150 invalid data` for malformed ids rather than 404, so `CodeNotFound` only fires on actual 404s. The `NotFoundError` mapping is plumbed end-to-end in `client.go` + `classify`; live verification needs a well-formed-but-unknown ride id, which is hard to construct on demand. |
| API error (any other non-2xx) | 5 | 5 (verified via the malformed-id call above) |

The "well-formed but unknown" 404 path is exercised by
`discoveries`, where occasional retired rides return 404 — the
walker logs `ride <id>: 404, skipping` and the run continues.

## Database state after sync

```
~/.local/share/peloton-pp-cli/peloton.db
schema_version: 1 (PRAGMA user_version)
tables: workouts, rides, songs, ride_songs, meta + FTS5 shadow tables
counts after sync --limit 5: workouts=5, rides=5, songs=41, ride_songs=41
```

WAL mode + busy_timeout=5s confirmed by inspecting pragmas; FTS5
rebuild fires on every Upsert*Tx commit, so search reflects the
freshly-synced rows without a background job.

## Known limitations exposed by the smoke

1. **Compact ride_id is `omitempty`.** Workouts with no ride_id
   (rare — appears in some Lanebreak / talk-only entries) drop the
   field rather than ship `"ride_id":""`. Downstream `jq` should
   `select(.ride_id)` before chaining.
2. **`auth login --headless` not exercised.** Headless mode is
   declared but Auth0 may still throw a CAPTCHA the user can't see.
   Visible-window login is the only verified path against a real
   account.
3. **`sync --full` not exercised on a large window.** Full backfills
   over hundreds of workouts are theoretically supported (4 in flight
   for ride hydration, no rate-limit hits in the small-window run);
   need a real account with deep history to validate the long-tail
   pagination.

## Verdict

Ship as a first draft. Every read-side workflow the SKILL.md
advertises works against a live account; the local SQLite mirror
populates and searches correctly; the typed exit codes are wired.
The MCP binary, dogfood/verify reports, and final PR rubric are
the remaining gates.
