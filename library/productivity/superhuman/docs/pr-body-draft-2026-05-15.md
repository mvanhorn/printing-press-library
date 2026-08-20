# PR body — feat/superhuman-overhaul

Paste-ready content for `gh pr create --base feat/superhuman --title "feat(superhuman): major CLI overhaul — folder coverage, fresh-on-read SQLite, reminders, snippets, composability" --body-file <this-file>`.

---

## Summary

Major overhaul of `superhuman-pp-cli` based on real-workflow gaps observed during a live triage session on 2026-05-15. Closes the five capability gaps that blocked agent + human use of the CLI as a Superhuman replacement: folder coverage, cache freshness, draft ergonomics, send-side features, and composability primitives. Adds an auth-state surfacing pass so silent token-expired errors become actionable.

This is a stacked PR. Base: `feat/superhuman` (the open initial-superhuman PR). The branch carries 23 commits — U1-U17 from the major-overhaul plan, then U2-U5 from a follow-up audit-driven plan.

## What this PR adds

**Folder coverage (U1).** `threads list --type` now accepts every Gmail and Superhuman folder visible in the UI sidebar: `inbox`, `sent`, `done`, `starred`, `archived`, `spam`, `trash`, `important`, `draft`, `reminder`, `scheduled`, `snippet`, `signature`, `knowledge-base`. Gmail-folder types route through `users.threads.list` with the correct `labelIds=`; Superhuman-side types continue through `/v3/userdata.getThreads`.

**SQLite store + Gmail history-delta freshness (U2, U3).** New SQLite schema v3 with `messages`, `messages_fts`, `history_state`, `label_index` tables. `bootstrap` command hydrates last-N messages per folder on first run. Subsequent reads use Gmail's `users.history.list?startHistoryId=` delta polling to apply minimal changes against the local store — fast queries, fresh data.

**Auto-refresh dispatcher (U4, plus follow-up U2 parity port).** `PersistentPreRunE` runs `runAutoRefresh` as the first action of every non-skip-list command. Two surfaces: `cache` (Superhuman backend) and `gmail` (history delta). Three-tier opt-out (`--no-refresh` flag > profile `no-refresh` > `SUPERHUMAN_NO_AUTO_REFRESH=1` env). Mirrors the granola PR #571 canonical pattern: `noRefreshCommands` typed map, `refreshPlan` / `refreshResult` / `refreshSurface` structs, byte-parity provenance line.

**Freshness response envelope (U5).** Opt-in via `--envelope=full`. Returns `{meta: {source, synced_at, history_id, delta_polled_ms_ago, delta_applied}, results: ...}` so agents know freshness state without a second call. Default `minimal` preserves existing stdout shape; no breaking change.

**Send-side features (U6, U7, U9).** `send --remind-in 2d --if-no-reply` schedules a reminder matching Superhuman's UI behavior. `send --schedule-at "Mon 8am"` defers a send. `send --snippet <name> --var first_name=Alice` substitutes the named snippet's body with `{{first_name}}` replacement.

**Snippets as a first-class command group (U7, plus follow-up U4 backend integration).** `snippets list / get / create / update / delete` against the real Superhuman backend (`/v3/userdata.getThreads filter={type:"snippet"}` and `/v3/userdata.writeMessage labelIds=["SNIPPET"]`). CLI snippets sync with the UI Snippets folder — no parallel local pool. One-time migration hint surfaces if a stale `~/.superhuman-pp-cli/snippets.json` is found.

**Draft creation ergonomics (U8).** `drafts new --to <e> --subject <s> --body <b>` creates a fresh outbound draft with no manual JSON construction. Auto-generates `draft-id`, `thread-id`, `rfc822-id`, fingerprint, schemaVersion.

**Query primitives (U10, U11, U12, U13, U15).** `messages list --query "<gmail-search>"` is a direct Gmail search passthrough (`from:`, `to:`, `after:`, `has:attachment`, `label:`). `threads list --label "Pitch"` filters by Auto Label. `threads list --participants-file <path>` and `--intersect-with-stdin` intersect by external email lists. `participants list / show` aggregates correspondents from the local store. `awaiting-reply --since 7d --external-only --min-age 4h` answers "what threads need a follow-up?" in one command. `messages get-by-rfc822 <id>` looks up by the universal mail-system identifier.

**Watch mode (U14).** `watch --account <e> --interval 30s` emits ndjson events on every `users.history.list` delta — message_added, message_deleted, label_added, label_removed. Optional `--filter "label:INBOX"`. Composable with any agent that subscribes to a stdout stream.

**Auth state diagnostics (U16).** `doctor --json` surfaces a `tokens` block (per-account `access_state` / `refresh_state`), `binary_age_days`, and `auto_refresh_active`. Commands that hit HTTP 401 now print `Run 'auth login --chrome' to re-authenticate.` on stderr before the original error.

**Docs (U17).** SKILL.md and README.md updated end-to-end. `.printing-press-patches.json` extended with entries for every inline `// PATCH:` introduced. `tools-manifest.json` regenerated to expose new commands to MCP consumers.

## Test plan

- [x] `go test ./...` — 530 passing tests across 17 packages (up from 267 on `feat/superhuman`).
- [x] `go vet ./...` — clean.
- [x] `python3 .github/scripts/verify-skill/verify_skill.py --dir library/productivity/superhuman/` — passes.
- [x] PII scrub gate — zero matches against the known-PII token list (see `docs/superhuman-snippets-discovery.md` for the gate).
- [x] Live smoke against a real account — all 7 new Gmail-folder `--type` values return real data. Evidence at `library/productivity/superhuman/docs/smoke-evidence-2026-05-15.md`.
- [x] Manual: `bootstrap --account <e> --folders sent,inbox --per-folder 10` runs end-to-end and populates the local store.
- [x] Manual: `awaiting-reply --since 7d --external-only` returns expected threads.
- [x] Manual: `snippets list` returns the UI-visible snippets (sync confirmed against the web app).

## Plan references

- Parent plan: `docs/plans/2026-05-15-001-feat-superhuman-cli-major-overhaul-plan.md` (17 units)
- Follow-up plan: `docs/plans/2026-05-15-002-fix-superhuman-overhaul-followup-plan.md` (6 units, 5 landed)
- Snippets discovery: `library/productivity/superhuman/docs/superhuman-snippets-discovery.md`
- Live smoke evidence: `library/productivity/superhuman/docs/smoke-evidence-2026-05-15.md`
- Canonical autorefresh template: granola PR #571 (commit `185f56b0` on `origin/main`)

## Stats

- 67 files changed, +9435 / -437
- 23 commits on `feat/superhuman-overhaul`
- 530 tests pass, +263 over the `feat/superhuman` base

## Notes for reviewers

- **Stacked PR.** Base is `feat/superhuman`, not `main`. The base PR adds the initial superhuman CLI; this PR layers the overhaul on top. Wait for the base PR to merge before rebasing or merging this one.
- **Schema v3 migration** is stamp-and-continue from v2. Older binaries refuse to open a v3 DB cleanly — the error message points at `rm <db-path>` as the workaround.
- **Auto-refresh is on by default.** Skip list covers `sync, auth, doctor, help, version, completion, agent-context, profile, feedback, which`. Tight loops should pass `--no-refresh`.
- **No new external dependencies.** Existing `modernc.org/sqlite` (pure Go, no CGO) handles the schema v3 additions.

🤖 Generated with [Claude Code](https://claude.com/claude-code) + Codex
