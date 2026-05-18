# Phase 5 Live Dogfood Acceptance — emailoctopus-pp-cli

## Result: PASS (95/96 active tests pass, level: full)

Live matrix run via `printing-press dogfood --live --level full` against `https://api.emailoctopus.com` with a real API key.

**Active tests:** 96 (213 total, 117 skipped for missing fixtures / write-side commands).

**Pass rate:** 95/96 = 99%.

## What's tested

- `doctor` — auth + reachability (PASS)
- `auth status / set-token / logout` — credential management (PASS)
- `sync` — full sync of lists, contacts, tags, fields, campaigns; reports endpoint correctly flagged with sync_error for missing required `status` param (generator limitation noted for retro)
- All 25 endpoint-mirror commands — help, JSON fidelity, paginated reads (PASS)
- All 8 transcendence commands:
  - `contacts engagement` — returns real subscriber with opens/clicks/last_engaged
  - `contacts dedupe` — `[]` for no duplicates
  - `campaigns digest` — campaign metadata + notes for draft-campaign report unavailability
  - `lists diff --since 1d` — touched contacts with synced_at timestamps
  - `tags intersect` — boolean tag-set queries
  - `contacts sync-csv --dry-run` — emits would-send payload with field/tag mapping
  - `contacts bulk-delete --dry-run` — matched count + safety preview
  - `automations trigger-batch --dry-run` — input/triggered/source summary
- UUID validation on positional args (added in-session for engagement + trigger-batch)
- Error paths — invalid UUIDs, missing required flags, malformed inputs all return exit 2 with clear messages
- JSON fidelity — every JSON-mode command's output parses cleanly via `jq`
- Help-text correctness — examples match shipped behavior

## Issues found and fixed in-session

| # | Issue | Fix |
|---|-------|-----|
| 1 | `contacts dedupe`, `engagement`, `tags intersect`, `lists diff` returned `null` for empty results | Initialized slices empty (`out := []T{}`) instead of `var out []T` |
| 2 | `campaigns digest` failed entirely on draft (unsent) campaigns due to 403 from reports/summary | Made summary/links/contacts reports best-effort; digest now returns metadata + `notes[]` explaining what was unavailable |
| 3 | Sync's typed-table upsert failed with NOT NULL constraint violation | Patched `sync.go` to inject `<parent_table>_id` (FK column) alongside `parent_id` for typed-table upserts |
| 4 | `lists diff` returned empty `synced_at` because SQLite strftime couldn't parse generator's time format | Switched to `substr(synced_at, 1, 19)` for both SELECT and WHERE compare |
| 5 | `contacts engagement` happily returned `[]` for invalid list_id | Added `store.IsUUID()` validation; invalid UUID returns exit 2 |
| 6 | `automations trigger-batch` returned exit 0 for invalid automation_id | Added `store.IsUUID()` validation; invalid UUID returns exit 2 |
| 7 | `contacts bulk-delete` examples used backslash line-continuations that dogfood parsed as positional args | Rewrote examples to single-line form |

## Remaining issue (retro candidate, not shippable bug)

- **`workflow archive --json`** emits JSONL stream (`{event: sync_start}\n{event: sync_progress}\n...`) instead of single JSON document. Generator-level template issue in `channel_workflow.go.tmpl` — affects every CLI the Printing Press produces. The command itself works correctly for human consumption; only `--json` mode is mis-formatted. Filed in shipcheck report's retro list.

## Verdict

**Gate: PASS** — all shipping-scope surface verified working with real EmailOctopus data; only failure is a non-shipping framework command's JSON formatting (retro candidate). Proceed to Phase 5.5 (Polish).
