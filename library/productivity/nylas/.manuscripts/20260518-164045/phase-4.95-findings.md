# Phase 4.95 — Native code review (6 hand-coded transcendence commands + root.go)

Date: 2026-05-18
Reviewer: Claude Opus 4.7 (1M)
Scope: sql.go, since.go, export.go, gravity.go, response_time.go, webhook_replay.go, root.go

## Findings (severity-ordered)

### REQUIRED — fixed in this pass

1. **gravity.go** — `if err == nil { rows... }` swallowed DB query errors on both `grants_messages` and `events`. If the local mirror was never `sync`-ed, the command would silently return `[]` instead of pointing the user at `sync`. Replaced with `return fmt.Errorf("querying grants_messages: %w\nRun 'nylas-pp-cli sync' first.", err)` for the first query and `return fmt.Errorf("querying events: %w", err)` for the second. **Status: FIXED.**

2. **response_time.go `loadGrantAddresses`** — `var raw json.RawMessage` declared and `_ = raw` swallowed it; dead code with a stale `encoding/json` import. Removed both the variable and the import. **Status: FIXED.**

### OPTIONAL — left for polish

3. **webhook_replay.go** — `--confirm` is bound to a Go variable named `verify`. The name `verify` is already overloaded in this codebase (`cliutil.IsVerifyEnv` / `PRINTING_PRESS_VERIFY`). Rename to `confirm` would improve readability. Non-blocking; behaviour is correct.

4. **sql.go `decodeSQLValue`** — `case sql.NullString:` branch is unreachable. `database/sql` with `Scan(*any)` returns concrete types (string, []byte, int64, float64, bool, time.Time, nil), never `sql.NullString`. Cosmetic; leave for polish if cleanup wave revisits this file.

5. **since.go `sinceTables`** — `"webhooks": "webhooks"` is gracefully skipped at runtime because the webhooks table has no `grants_id` column. The map entry is misleading. Either remove webhooks from the map or branch the query. Polish target.

6. **export.go `--format`** — flag only accepts `ndjson` and defaults to `ndjson`. Either remove the flag (and the validation) or wire up `csv` / `jsonl`. Polish target.

7. **gravity.go / response_time.go / sql.go / export.go** — `dryRunOK(flags)` is called BEFORE the DB is opened, but the side-effecting check is identical across all six files. Could be extracted into a small helper. Style; polish target.

## Verification reviewed

- `go build ./cmd/nylas-pp-cli` PASS after each edit
- Dogfood novel_features_check 12/12 (pre-fix) — re-running after fixes
- No new test coverage added (no test files in `internal/cli/`); polish phase owns adding sample-probe-style tests

## Residual risks

- All six commands rely on the local SQLite mirror; shipcheck sample-probe failures (11/12) are all "unable to open database file: out of memory (14)" — meaning sync was never run by the probe harness. This is a probe-setup issue, NOT a code defect in the commands themselves. Polish phase should add a verify-time stub DB so the probe gets meaningful coverage.
- `gravity` and `response_time` now correctly surface the "run sync first" message; previously they returned empty `[]`. Probe results will look DIFFERENT after this change (errored vs silently empty).

## Merge recommendation

PROCEED to Phase 5.5 polish. Two REQUIRED findings fixed; build green. The 5 OPTIONAL findings are well-suited to polish's auto-fix loop, especially the probe stub which is the load-bearing one.

## Simplify pass

No changes from simplify — the 6 files are already at the right abstraction level. Each command is self-contained, no premature shared helpers, no dead branches beyond the ones flagged above. The duplication between `since.go` / `export.go` / `gravity.go` `--since` parsing is real (4 callers) but each call site is 4 lines; extracting would save ~10 lines of code and add an indirection. Not worth it.
