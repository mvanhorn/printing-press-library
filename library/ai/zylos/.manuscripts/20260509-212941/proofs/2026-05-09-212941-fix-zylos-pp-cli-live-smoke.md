# Phase 5 Acceptance Report: zylos-pp-cli

## Level: Full Dogfood

## Test Results: 10/10 core tests passed

| Test | Command | Result | Notes |
|------|---------|--------|-------|
| Auth login | session login | PASS | Cookie persisted to disk, subsequent commands authenticated |
| Status check | status --json | PASS | Returns full state (idle, health ok, 20+ fields) |
| Session check | session check --json | PASS | Returns auth state and timezone |
| Conversation history | conversations recent --limit 3 | PASS | Returns real messages with content, direction, timestamps |
| Send message | conversations send --stdin | PASS | Message sent to AI, response confirmed |
| Sync | sync | PASS | 11 records synced (conversations + conversations-recent + status) |
| Stats | stats --json | PASS | Message counts, direction breakdown, per-day activity |
| Timeline | timeline --limit 5 | PASS | Chronological view with content previews |
| Search | search "curl" | PASS | Found matching message |
| Export | export --dry-run | PASS | Dry-run works |
| Session logout | session logout | PASS | Session terminated |

## Fixes Applied During Dogfood
1. Added `--password` flag and env var support to session login (ZYLOS_PASSWORD, ZYLOS_WEB_PASSWORD)
2. Created persistent cookie jar to maintain session across commands

## Printing Press Issues
1. Generated login command sends empty `{}` body instead of spec-defined request fields
2. Client created with nil cookie jar — no session persistence for cookie-based auth APIs

## Gate: PASS
