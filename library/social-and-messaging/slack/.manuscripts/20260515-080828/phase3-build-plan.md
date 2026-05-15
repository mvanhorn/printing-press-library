# slack-pp-cli v1.1 — Phase 3 Build Plan (resume doc)

> Checkpoint: 2026-05-15. Printing Press reprint run `20260515-080828`.
> Phases 0–2 complete. This doc is the turnkey resume for Phase 3 onward.

## Resume state

- **Working dir** (generated CLI, builds clean): `~/printing-press/.runstate/cursor-664f634d/runs/20260515-080828/working/slack-pp-cli/`
- **Run dir**: `~/printing-press/.runstate/cursor-664f634d/runs/20260515-080828/`
- **Spec**: `~/printing-press/library/slack/spec.json` (OpenAPI 3.0.0, 174 endpoints, mcp block, x-auth-vars)
- **research.json**: `<run dir>/research.json` — 8 novel_features + narrative
- **Brief / absorb manifest / brainstorm**: `<run dir>/research/2026-05-15-081209-*`
- **Lock**: released. Re-acquire with `printing-press lock acquire --cli slack-pp-cli --scope cursor-664f634d`.
- Press version 4.6.1. Binary name `slack-pp-cli`, MCP `slack-pp-mcp`, module `slack-pp-cli`.

## What the generator produced (done)

- 174 endpoint commands (`internal/cli/promoted_*.go`)
- Framework: `sync.go`, `search.go`, `which.go`, `tail.go`, `doctor.go`, `analytics.go`, etc.
- `internal/store/store.go` — 16 spec-derived resource tables + `sync_state` + FTS5
- All Go quality gates PASS

## Phase 3 — what remains to hand-build (~3-5K LOC)

### Priority 0 — store schema extension + sync engine
The generator's store has spec-endpoint resource tables, NOT the mirror entities the
novel verbs need. Hand-extend (non-generated file in `internal/store/`, or migration):
- Tables: `channels`, `users`, `messages` (+ FTS5 over text), `threads`, `reactions`,
  `usergroups`, `files`, `audit_log`. Per-channel `latest_ts` cursor in `sync_state`.
- Sync logic: `conversations.list` + `users.list` + per-channel `conversations.history`
  + `conversations.replies`, cursor-aware pagination, `Retry-After` respect,
  silent-truncation WARNING when returned page < requested.
- Bump `StoreSchemaVersion`.

### Priority 1 — 12 absorbed NOI verbs (hand-built on the mirror)
Reads: `digest`, `customer-intel`, `drift`, `dms-summary`, `dormant`, `attention`,
`who-said`, `thread-summary`. Writes (cron-safe, `--dry-run`): `post`, `schedule`.
Plumbing: `channel-find`, `user-find`. (`sync`/`search`/`which` already exist —
reconcile, don't duplicate.)

### Priority 2 — 8 transcendence features (research.json novel_features)
1. `customer-intel-deep` (9/10) — 4-mirror ATTACH DATABASE join (pp-attio/pp-asana/pp-fathom)
2. `dm-engagement` (8/10) — 3-mirror join
3. `action-followthrough` (8/10) — pp-fathom × pp-slack FTS5 join
4. `goal-channel-pulse` (6/10) — pp-asana × pp-slack join, explicit `slack_channel:` Rock YAML
5. `reactions summarize` (7/10) — local GROUP BY emoji
6. `unreads --priority` (7/10) — mirror last_read scan (xoxp branch only; xoxc → v1.1)
7. `usergroups list` + emit-time `<!subteam^S…>` → `@handle` substitution (7/10)
8. `agent-audit` (6/10) — read over `audit_log` table

Privacy posture (load-bearing): local-only SQLite, DM-read audit log, `--redact-sensitivity`
flag, `~/.config/slack-pp-cli/skip.yaml` allow/deny list. See brief §Privacy Posture.

Cross-source mirror paths (T1-T4): pp-attio/pp-asana/pp-fathom SQLite DBs in their
`%LOCALAPPDATA%` dirs; degrade gracefully with `--skip-missing`.

### Build approach
Skill's Phase 3 delegation pattern: dispatch subagents (P0 first, then P1/P2 verbs can
parallelize) with per-feature behavioral acceptance tests. Register novel commands in
`root.go` (generated — edits revert on regen, which is fine; this is the final reprint).
Use the skill's verify-friendly RunE template + store-query/API-call skeletons.

## Phases 4-6 remaining (printing-press skill)
- Phase 4: `printing-press shipcheck --dir <work> --spec <spec> --research-dir <run dir>`
- Phase 4.8/4.85/4.9: agentic SKILL / output / README reviews
- Phase 5: `printing-press dogfood --live --level full` (SLACK_USER_TOKEN approved, read-only)
- Phase 5.5: `/printing-press-polish <work dir>`
- Phase 5.6: `printing-press lock promote --cli slack-pp-cli --dir <work dir>`
- Phase 6: next-steps menu (publish PR / retro)

## Resume command
Re-enter the printing-press flow at Phase 3, OR continue hand-building directly in the
working dir then run shipcheck. The run state (research.json, brief, manifest) is all
on disk — no need to re-run Phases 0-2.
