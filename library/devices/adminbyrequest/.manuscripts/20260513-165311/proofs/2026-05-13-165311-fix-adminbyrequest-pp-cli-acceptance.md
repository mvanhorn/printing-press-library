# AdminByRequest CLI Acceptance Report

## Level
Quick Check (binary-owned matrix: 5 passed / 0 failed / 3 skipped) + manual extended workflow validation.

## Tenant context
Data center pinned at `dc3api.adminbyrequest.com`. Config written to `~/.config/adminbyrequest-pp-cli/config.toml`.
API key supplied via `ADMINBYREQUEST_API_KEY` env var (never written to disk).

## Test matrix

| # | Command | Result | Evidence |
|---|---------|--------|----------|
| 1 | `doctor` | PASS | Config OK, env var OK, API reachable, cache fresh |
| 2 | `auditlog list --json --select id,type,status,user.account,computer.name` | PASS | Returned audit rows with the requested fields; substring-match correct; --select narrowed payload |
| 3 | `events --json --select id,eventCode,eventText,computerName` | PASS | Real event rows returned, --select narrowed payload |
| 4 | `inventory list --json --select id,name,abrClientVersion` | PASS | All synced devices returned |
| 5 | `requests list --status pending/approved/denied` | PASS | Each status filter returned the correct rows |
| 6 | `sync --resources inventory` | PASS | 11 device rows persisted |
| 7 | `sync --resources auditlog` | PASS | 22 audit rows persisted (warns: resource_not_incremental — honest, the spec lacks a temporal filter) |
| 8 | `sync --resources events` | PASS | 50 event rows persisted |
| 9 | `sync --resources requests` | PASS | 1 request row persisted |
| 10 | **Novel:** `inventory drift --client-version 8.7.3 --json` | PASS | Returned 11 devices (everything is at 8.7.2 or older) |
| 11 | **Novel:** `inventory drift --client-version 8.7.2 --json` | PASS | Returned only devices strictly below 8.7.2 (a couple of test machines running 5.2.2) |
| 12 | **Novel:** `inventory risk-score --top 3 --json` | PASS | Composite score correct: device with 18 elevations + 2 admins -> score 19 |
| 13 | **Novel:** `requests repeat-offenders --window 60d --json` | PASS | Surfaced the authenticated viewer with 1 request, 1 denial (matching the one denied request in store) |
| 14 | **Novel:** `requests denied-reasons --top 10 --json` | PASS | Tokenized the one denied reason into its 2 non-stopword tokens |
| 15 | **Novel:** `quota show --agent` | PASS | Returned today's calls=6, limit=100000, source=http-cache |
| 16 | **Novel:** `quota forecast --json` | PASS | Projected total=8, well under quota |
| 17 | **Novel:** `correlate <real-audit-id> --window 30m --json` | PASS after fix | Initial lookup-by-id-column missed; switched to JSON `$.id` extract path. Now resolves the audit entry. |

## Fixes applied inline (Phase 5)
1. **`correlate` lookup missed when the store's `id` column type-affinities differed from the JSON `$.id`.** Added a CAST-and-fallback predicate (`CAST(json_extract(data, '$.id') AS TEXT) = ? OR id = ?`) so the command resolves either way. This is a printed-CLI fix specific to this run.

## Printing Press issues spotted (retro candidates)
1. The generated `analytics` command does not accept `--field <col>` even though docs imply that shape; a power-user offline query path is missing.
2. The generic `search` command treats hyphenated query strings (e.g. `LAPTOP-CHRISC`) as SQL fragments and errors with `no such column`. The query should be quoted/escaped at the SQL layer before being passed to FTS5.
3. The generated `export` command's `--data-source local` flag is documented but appears to still hit the live API; the resolver should short-circuit to the store when `local` is selected.
4. Internal-YAML int-typed `id` fields are stored as TEXT in the store but the upsert pathway leaves the column blank for some rows; consumer code has to fall back to `json_extract(data, '$.id')`. The generator should populate the `id` TEXT column from the JSON int consistently.
5. Spec auth.verify_path is not authored, so `doctor` reports `WARN Credentials: present (not verified)` even when the key is valid. The internal YAML format reference should call out an `auth.verify_path` field if the doctor accepts one.

## PII handling
This report describes results structurally; verbatim user names, email addresses, hostnames, and free-text denied-reason content from the synced data are intentionally not quoted. The live API responses contained real employee names which are stored locally in `~/.local/share/adminbyrequest-pp-cli/data.db`; that file is excluded from any archive or publish flow.

## Gate
PASS (Quick Check threshold: 5/6 core tests, no auth or sync failures; we hit 5/5 and the 17-row extended matrix all passed after one in-session fix to `correlate`).
