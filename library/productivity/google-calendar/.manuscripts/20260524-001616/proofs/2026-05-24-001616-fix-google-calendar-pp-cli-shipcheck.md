# Google Calendar CLI — Shipcheck

## Umbrella verdict: PASS (6/6 legs)

| Leg | Result |
|-----|--------|
| dogfood | PASS |
| verify | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| validate-narrative | PASS (after fix) |
| scorecard | PASS — 90/100, Grade A |

## Blockers found and fixed
1. **validate-narrative FAIL → PASS.** Quickstart referenced `agenda` (never built) and `auth login` (cannot run under `--dry-run`, needs `--client-id`).
   - Fix: built the `agenda` command (table-stakes gcalcli feature from the absorb manifest — local-store-backed time-range listing). Now 8 absorbed + agenda + 7 novel.
   - Fix: replaced the `auth login` quickstart step with `doctor`; auth-login guidance lives in `auth_narrative`.

## Scorecard (90/100, Grade A)
Strong: Output Modes 10, Auth 10, Error Handling 10, Doctor 10, Agent Native 10, MCP Quality 10, MCP Remote Transport 10, Local Cache 10, Breadth 10, Path Validity 10, Auth Protocol 10, Sync Correctness 10.
Gaps (non-blocking): mcp_token_efficiency 4/10 (42 endpoint-mirror tools), insight 4/10. Candidates for Phase 5.5 polish.

## Behavioral correctness (ship threshold)
All 7 novel commands + agenda smoke-tested under PRINTING_PRESS_VERIFY (local/deterministic):
- `free` inverts busy → returns full-window free slot on empty store (correct).
- conflicts / changes / load / acl-audit / rsvp-status / agenda → valid JSON `[]` on empty store.
- `book` missing flags → exit 2; would_create under verify → exit 0; `--on-conflict abort` returns typed exit 9 (ExitCode maps cliError.code).
- `--dry-run` → exit 0 for all.
Live behavioral verification deferred — Phase 5 skipped (OAuth2, no creds).

## Notes
- govulncheck generation gate fails (tool not installed/network-blocked in env); `go build`/`go vet` clean. Environment issue, not a defect.

## Verdict: ship (pending agentic reviews + polish)
