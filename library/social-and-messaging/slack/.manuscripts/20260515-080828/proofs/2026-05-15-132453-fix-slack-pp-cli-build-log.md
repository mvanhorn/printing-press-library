# slack-pp-cli v1.1 — Phase 3 Build Log

> Reprint run `20260515-080828`. Phase 3 hand-build of the mirror store/sync engine
> plus 20 novel verbs on top of the generator's 174 endpoint commands.

## What was built

### P0 — mirror store + sync engine
- `internal/store/mirror.go` (758 lines) — 8 mirror tables (`m_channels`, `m_users`,
  `m_messages`, `m_threads`, `m_usergroups`, `m_reactions`, `m_files`, `m_audit_log`)
  + `m_messages_fts` FTS5. Lazy `EnsureMirrorSchema`; generated `migrate()` untouched.
- `internal/cli/sync_mirror.go` (681 lines) — `sync mirror` subcommand: conversations.list,
  users.list, usergroups.list, per-channel history/replies with cursor pagination,
  Retry-After respect, silent-truncation warning, skip.yaml allow/deny, DM-read audit log.
- `internal/store/mirror_test.go` (415 lines) — 11 table-driven tests.

### P1 — 12 NOI verbs (2,694 lines, 16 files)
digest, customer-intel, drift, dms-summary, dormant, attention, who-said, thread-summary,
post, schedule, channel-find, user-find. All with `--json`/`--select`, verify-friendly RunE,
typed exit codes; write verbs (`post`/`schedule`) `--dry-run` + verify-env short-circuit;
`--redact-sensitivity` on digest/dms-summary/customer-intel/attention; DM reads audited.

### P2 — 8 transcendence verbs (2,779 lines, 16 files)
customer-intel-deep, dm-engagement, action-followthrough, goal-channel-pulse,
reactions summarize, unreads, usergroups list, agent-audit. T1-T4 are cross-source
SQLite ATTACH-DATABASE joins against pp-attio/pp-asana/pp-fathom mirrors with graceful
`--skip-missing` degradation (`missing_sources` field). All read-only (`mcp:read-only`).

### Wiring
`internal/cli/root.go` — 3 attach calls added (`attachMirrorSyncCmd`, `attachP1Cmds`,
`attachP2Cmds`). Only generated file edited; sanctioned for the final reprint.

## Phase 3 Completion Gate
- Transcendence features in manifest: 8. Built: 8. ✅ No gap.
- P1 verbs in user vision: 12. Built: 12. ✅
- All 21 hand-built commands resolve `--help` against the binary. ✅
- Pure-logic packages have table-driven tests (store mirror, p1_common, p2_common,
  reactions, agent_audit, unreads, goal_channel_pulse). ✅

## Verification
- `go build ./...` → exit 0
- `go vet ./...` → exit 0
- `go test ./...` → all packages ok (cli 7.0s, store 2.8s, cliutil 10.5s, client, mcp)

## Intentionally deferred
- `export`/`convert`/`view` (slackdump parity) — stubs, no weekly persona use (manifest rows 10-12).
- Multi-workspace `--workspace` — single AtomChat workspace today (row 13).
- xoxc+xoxd stealth auth — xoxp covers all reads; v1.1 unlock (row 18).
- `unreads` uses the mirror sync high-water-mark as the read cursor (Slack `last_read`
  not mirrored); documented in the verb `--help`.

## Generator limitations found
- modernc.org/sqlite rejects `?N` positional named params (`sql.Named("1",...)`); use
  plain `?` placeholders. Noted for future store-query code.
