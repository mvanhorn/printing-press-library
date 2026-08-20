# Acceptance Report — slack-pp-cli reprint

Run `20260801-150754-271fdf29`. Live-tested against a real Slack workspace owned by the operator (referred to below as **the test workspace**; org name, member emails, and display names are deliberately not reproduced here).

```
Acceptance Report: slack
  Level: Full Dogfood
  Tests: 358/358 passed (219 skipped, 0 failed)
  Gate: PASS
```

Marker: `proofs/phase5-acceptance.json` — `status: pass`, `level: full`, `matrix_size: 358`, written by the runner (not hand-authored).

## What was verified live

Auth context: bearer bot token (`xoxb-`), read scopes only plus `chat:write` (operator declined removing it; see Deviations).

| Check | Result |
|---|---|
| `doctor` | Auth configured, credentials from `env:SLACK_BOT_TOKEN`, API reachable |
| `sync --resources conversations,users` | 61 records — 29 conversations, 32 members |
| `archive sync --with-threads` | 991 messages + 4 thread replies mirrored from one channel |
| `archive recall "deploy"` | 3 hits; **every** returned row's text contains "deploy" |
| `archive recall "zzzznotpresent"` | `[]` — empty array, not `null`, not unrelated rows |
| `archive coverage` | 993 messages, span 17.1 days, first/last timestamps correct |
| `health` | 33.1 msgs/day, 5 distinct posters, median first reply 5m28s |
| `threads stale` | Real thread surfaced, 13.66 days stale, author ID resolved to a display name |
| `catchup --since 30d` | 993 new messages, threads-awaiting-reply populated |
| `users activity <user>` | 8 messages, 2 thread starts, 3 threads carried, reactions counted |
| `users whois` | Resolves by handle **and** by ID to the same record, with tz/email/DND |
| Full command matrix | 358/358 across help, happy-path, JSON fidelity, output modes, error paths |

## Failures found and fixed during this phase

1. **`archive recall` help advertised the pre-rename path** (`slack-pp-cli recall`). Fixed; the matrix could not extract a runnable example from it.
2. **`auth-api` examples used the resource key** (`auth_api`, underscore) rather than the real command name (`auth-api`). Copy-pasting them would fail. Fixed in both affected commands.
3. **`users activity <unknown-person>` returned `[]` with exit 0.** A named person who cannot be resolved is a usage error, not an empty result — returning `[]` is indistinguishable from "they exist but said nothing". Now exits 2 with an actionable message.
4. **Bare `sync` failed the entire run** because `stars` and `reminders` return `not_allowed_token_type` for a bot token. Reclassified: capability limits (`not_allowed_token_type`, `missing_scope`, `not_in_channel`, …) are per-resource warnings; credential failures (`not_authed`, `invalid_auth`, `token_revoked`, `account_inactive`) stay hard errors.
5. **`auth.revoke` destroyed the operator's live token mid-run.** See Incident below.

## Incident: token revoked by the test matrix

**What happened.** Slack serves `auth.revoke` over HTTP `GET`. The generator derives command safety from the HTTP method, so it annotated the command `mcp:read-only: true`, and dogfood's destructive-endpoint skip did not apply. The matrix executed `auth-api revoke` as an ordinary happy-path and json-fidelity test and revoked the live bot token; every subsequent call returned `account_inactive`.

**Contributing cause on the agent side.** In the preceding round this command failed with "missing runnable example", which incidentally prevented it from running. That was corrected as a documentation bug, which removed the accidental protection; the next run executed it.

**Blast radius.** Credential only. No messages sent, no content posted, edited, or deleted, no channel or workspace settings changed. Recovery was a workspace reinstall.

**Fix.** `internal/cli/auth_api_revoke.go` now short-circuits under `cliutil.IsVerifyEnv()` / `cliutil.IsDogfoodEnv()`, printing what it would do instead of acting, and the annotation was corrected from `mcp:read-only` to `mcp:destructive`. Recorded as patch `slack-auth-revoke-harness-guard`. Verified: both harness env vars produce a would-revoke line and exit 0 without issuing the request; the subsequent 358/358 matrix run left the replacement token alive.

**Note for the operator:** the two earlier commitments ("I decline write-side tests" and "I won't pass `--allow-destructive`") were both honoured and both keyed off classifications that declared this endpoint a safe read. Behavioural promises cannot cover a mislabelled endpoint; only the scope-level guarantee could have, and `auth.revoke` requires no scope, so even that would not have prevented it.

## Deviations from the stated test plan

- **Write scope present.** The installed app carries `chat:write`; the operator was told and chose to keep it. No write-side lifecycle tests were requested or run, and `--allow-destructive` was never passed.
- **Three command families unexercised.** `search`, `stars`, and `reminders` require a user token (`xoxp-`); only a bot token was available. They generate and verify structurally but were never called live. This is a hard Slack capability boundary, not a scope misconfiguration.
- **Single-channel message corpus.** The bot was invited to one channel, so message-dependent metrics are computed over one channel's history. Multi-channel aggregation is exercised structurally but not across a wide corpus.
- **Deep pagination only partly covered.** The page cap was reached at 5 pages, so cursor paging is proven across multiple pages but not to exhaustion.

## Printing Press issues (for retro)

1. **No cross-spec dedup on `--spec` merge.** Two specs produced 150 tools = 62 + 88 exactly, shipping the same Slack method twice under different command names. Deduplicated by hand before generation.
2. **Command safety derived from HTTP method.** Any API exposing a destructive operation over `GET` is annotated `mcp:read-only` and auto-invoked by dogfood. This revoked a live credential.
3. **`ok:false`-in-200 not modelled.** The sync loop stores an error envelope as a record and reports success, silently corrupting the mirror. Second occurrence of this defect class on this CLI.
4. **OAuth2 security definition from a secondary spec silently displaced the primary auth model**, leaving `SLACK_BOT_TOKEN` parsed but never sent. Every request went out unauthenticated.
5. **`destructive: true` in the internal spec is accepted and ignored** — no parse error, no effect.
6. **Mock-mode data pipeline yields 0 rows** for this spec shape even with response schemas and `response_path` declared. Isolated as pre-existing (reproduces with all local patches disabled).
7. **Missing template** `workflows/comm_health.go.tmpl` warned on every generation.
