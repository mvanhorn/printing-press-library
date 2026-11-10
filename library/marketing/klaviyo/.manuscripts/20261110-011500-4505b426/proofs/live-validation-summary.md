# Live validation, run 20261110-011500-4505b426

## Why phase 5 recorded a skip

This CLI is tenant-gated: every API command resolves its credential through a client profile
(`PRINTING_PRESS_CLIENT_PROFILE`, with the key held behind a credential reference). `printing-press dogfood --live` runs each
command with a fresh scoped HOME and XDG directories, so the profile file does not exist there and every gated command exits
1 before reaching the API. `phase5-skip.json` (reason `external_credentials_unavailable`, written by
`printing-press dogfood --write-skip`, fingerprinted to this source) records that. The checks below ran the same binary
outside that sandbox against one real Klaviyo account through a client profile.

## Command tree

See `live-matrix.md`: all 550 leaf commands from `agent-context`. GETs ran live with IDs discovered from list responses;
every write ran with `--dry-run`; hand-written commands passed `--help`.

## Live writes and hand-written commands

Writes only touched throwaway `pp-test-*` objects, each read back through the API and then deleted.

| Command | Live result |
|---|---|
| `plan qa-gate --campaign-id <pre-2026-10-15 ID>` | Verdict returned (was HTTP 404 before the campaign model change) |
| `campaigns get-id <old ID>` | 404 with the new hint pointing at `campaign-messages get-id` |
| `campaigns deploy` (list included, list + segment excluded) | Unscheduled draft; API read-back: status draft, no send time, audiences as given, variation on a non-reusable template copy |
| `campaigns image-swap` | URL replaced in the variation's template copy, old URL gone |
| `campaign-clone --stdin` | Clone reads back as draft |
| `campaigns test-send --dry-run` | QA ran on a real campaign (warn, 3 findings); planned send shown; nothing sent |
| `plan qa-gate --brand` | 27 s on a real campaign (was over 10 minutes before the filtered logo lookup); footer found, no old logos |
| `send-budget --no-refresh` | Current billing period usage against the cap; one scheduled email reported with no estimate (unknown, not zero) |
| `flows insert-step` | Preview wrote nothing; `--apply` inserted a 2-hour delay after the anchor, API read-back confirmed the link |
| `forms publish` / `forms unpublish` | Embed test form went live, then back to draft; API read-back each time |
| `learnings stats` | Runs under the selected profile; refused for a profile that does not exist |
| `metrics get`, `lists get` on an old database | Rows save to typed tables after the schema repair (checked on a database copy) |

All test objects (3 campaigns, 5 templates, 1 flow, 1 form) were deleted and return 404.

## Gate

`go build ./...`, `go vet ./...`, `go test ./...` and `verify_skill.py` pass on this source.
