# slack-pp-cli v1.1 — Phase 5 Live Dogfood Acceptance

> Reprint run `20260515-080828`. Quick Check (user-selected). Live xoxp token,
> read-only + dry-run only — no public posts, no workspace mutation.

## Level: Quick Check — Gate: PASS

Binary-owned runner: `printing-press dogfood --live --level quick` →
`status: pass, matrix_size: 6, tests_passed: 6, tests_skipped: 3` (mutating
commands correctly skipped as dry-run-only). Marker: `proofs/phase5-acceptance.json`.

## Manual Quick Check matrix (against the live AtomChat workspace `T01GKHKBY7R`)

| # | Test | Result |
|---|------|--------|
| 1 | `doctor` — auth + reachability | PASS — `auth_source: env:SLACK_USER_TOKEN`, API reachable |
| 1b | `auth-test` | PASS — `ok:true, team=Atom, user=eholmann` |
| 2 | `conversations-list` (live list) | PASS — 259 public channels returned |
| 3 | `sync mirror --since 7d --channels <3>` | PASS — 1580 channels, 609 users, 131 messages, 17 threads, 61 reactions, 21 files |
| 4 | `who-said "Atom" --window 7d` (FTS5) | PASS — real messages, timestamps, authors, channels |
| 5 | `who-said --json --select channel,author,permalink` | PASS — field narrowing works; permalinks populated |
| 6 | `customer-intel "Atom"` | PASS — cross-channel timeline |
| 7 | `drift --window 7d` | PASS — `[]` (honest: no stale threads in window) |
| 8 | `reactions summarize` (transcendence) | PASS — `total_reactions:7`, top messages by reaction count |
| 9 | `post --dry-run` / `schedule --dry-run` | PASS — request shown, nothing sent |

## Bugs found and fixed inline (3 — all CLI fixes, fix-before-ship)

1. **Bearer-prefix bug** (`internal/config/config.go`) — `AuthHeader()` returned the
   bare token; Slack needs `Authorization: Bearer <token>`. Every live API call
   returned `not_authed`. **This is a regeneration of v1's retro #1365** — the
   generator still emits the apiKey scheme without the Bearer prefix. Fixed:
   `AuthHeader()` now prefixes `Bearer ` and falls back to the xoxb bot token.
   Re-verified: `auth-test ok:true`.
2. **`sync mirror` aborted on `usergroups.list: missing_scope`** (`internal/cli/sync_mirror.go`)
   — the AtomChat xoxp token lacks `usergroups:read`; one stage's permission error
   killed the whole sync. Fixed: added `isPermissionErr()`; permission failures on
   the usergroups stage and per-channel history are now non-fatal warnings collected
   in `stats.Warnings`. Re-verified: sync completes, `usergroups skipped` warning surfaced.
3. **Empty permalinks** (`internal/cli/sync_mirror.go`) — `conversations.history`
   doesn't return permalinks, so `customer-intel`'s "every line cited with a permalink"
   claim was unmet. Fixed: `resolveTeamDomain()` (one `auth.test` call at sync start)
   + `messagePermalink()` synthesize the deterministic archive URL. Re-verified:
   `https://atom-chat.slack.com/archives/C04GR3DQK2L/p1778874316233309`.

## Printing Press issues for retro

- **#1365 regression** — the apiKey-without-Bearer generator bug recurred on this
  reprint despite the v1 retro. The spec's auth scheme needs the `http: bearer`
  shape, or the generator must emit the prefix. File/comment on retro.

## Fixes applied: 3 · Printing Press issues: 1 · Gate: PASS
