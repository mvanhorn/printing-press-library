<!-- slop-gate: off -->
# Acceptance Report: tailscale

Level: Full Dogfood (live, against the test tailnet with an API access token)
Tests: 327/327 passed (433 skipped: mutating commands run dry-run only, commands with no fixture in the test tailnet, help-only framework commands)
Gate: PASS

## Loops
- Run 1: 277/339 passed, 62 failed, 4 novel features hollow.
- Run 2: 326/329 passed, 3 failed.
- Run 3: 327/327 passed, 0 failed, no hollow features.

## Fixes applied (CLI)
- 49 generated endpoint commands had no --help Example (opaque IDs); added x-pp-example invocations with obvious placeholder IDs.
- Removed fake spec example IDs from path parameters so the runner resolves real IDs from list commands or the store, or records a fixture skip, instead of calling the API with a placeholder.
- acl preview got a working fixture (ipport query plus a minimal policy body on stdin).
- Novel parent groups carried stale examples (placeholder device names); now use `self` or read-only forms.
- routes approve/unapprove happy-args dropped bare boolean tokens (the runner renders them as `--flag true`, which a positional command reads as an extra argument).
- policy restore accepts `latest` (newest backup), which also gives it a live happy path.
- Plan output now carries `dry_run: true` and an `action` field under --dry-run.
- `tailnet logging get-log-streaming-status` declares exit 3 as expected (the API returns 404 when log streaming is not configured, which is the normal state).

## Printing Press issues (retro)
- Runner renders bare boolean happy-args as `--flag true`.
- Spec example IDs on path params become pp:happy-args positionals and bypass fixture resolution.
- Path-item-level parameter examples are honored separately from operation-level ones (easy to miss when stripping).

## Safety
- No --allow-destructive. Every generated POST/PUT/PATCH/DELETE ran dry-run only.
- Hand-written writes ran without --yes, so they printed plans from live reads and changed nothing.
- Runner used a sandboxed HOME; no policy backups or config were written to the operator's home.
