---
title: "feat: Major overhaul of superhuman-pp-cli — folder coverage, fresh-on-read SQLite, reminders, snippets, composability"
type: feat
status: active
created: 2026-05-15
depth: deep
target_repo: printing-press-library
target_package: library/productivity/superhuman
---

# feat: Major overhaul of superhuman-pp-cli — folder coverage, fresh-on-read SQLite, reminders, snippets, composability

## Summary

Land a comprehensive upgrade to `superhuman-pp-cli` based on gaps observed during a live triage session on 2026-05-15. The session attempted to answer two questions — "which meetings this week need follow-up email?" and "draft replies for these two threads" — and surfaced concrete failures in five areas: folder coverage (no access to Sent / Done / Starred / Archived / Spam / Trash), cache freshness (no auto-refresh, silent staleness), draft creation ergonomics (could not create a new outbound draft without hand-rolling the full `writeMessage` payload), send-side features (no reminder flags, no snippets, no scheduled send), and composability primitives (no way to feed external participant lists or thread-ID sets to filter queries).

This PR closes those gaps in a single feature branch. The structure mirrors the proven shape from `feat(granola): auto-refresh on every CLI invocation` (PR #571 in this repo, plan `docs/plans/2026-05-14-001-feat-cli-auto-refresh-every-invocation-plan.md`): a foundation of store + freshness + folder coverage, layered features on top, and an auth/doctor sweep that surfaces the silent failure modes the live session exposed.

All commits live under `library/productivity/superhuman/`. PII (account email, Google user ID, draft IDs, message subjects, person names from the originating session) is excluded from code, fixtures, tests, examples, and documentation. Anything Matt-specific that needs to persist locally lives under `.printing-press-patches.json` per the AGENTS.md customization contract. A pre-merge PII scrub gate (U17) verifies no known-PII tokens slipped through.

---

## Problem Frame

**Current behavior gaps observed 2026-05-15.**

1. **Folder coverage.** `superhuman-pp-cli threads list --type sent` errors: `unsupported --type "sent" (valid: [signature knowledge-base inbox draft reminder scheduled snippet])`. The same blocker applies to `done`, `starred`, `archived`, `spam`, `trash`. The current Gmail passthrough is hardcoded to `labelIds=INBOX` and a small whitelist of Superhuman-side system lists, so the user cannot answer "what did I send this week?" or "what did I archive after replying?" from the CLI at all. This blocked the most common triage question of the session.

2. **Cache freshness.** The CLI has a `sync` command and a local SQLite store under `internal/store/`, but read commands do not auto-refresh. `messages.query` (the AI proxy) returns `data: null` for structured queries. `ai --query` SSE returns HTTP 500 / `unknown-error`. `drafts list` returns HTTP 400 against `/v3/userdata.getThreads`. The user has no way to enumerate sent or archived mail without manual `sync` and faces silent staleness even with manual sync (the cache returns yesterday's data without flagging it). Mirrors the failure mode `feat(granola): auto-refresh on every CLI invocation` already solved for Granola in PR #571.

3. **Draft creation ergonomics.** `drafts write` requires `--thread-id`. The CLI exposes no way to create a fresh outbound draft (no thread to reply to) without hand-constructing the full `writeMessage` payload — draft ID, thread ID, rfc822 ID, fingerprint, schemaVersion, label IDs, client timestamps — and piping via `--stdin`. The session had to do this twice (Zoox follow-up, Freestyle follow-up). The 30+ line payload construction is a per-call cost the CLI should absorb once.

4. **Send-side features.** No equivalent of Superhuman's UI reminder picker (`Remind me — in 2 weekdays if no reply`), no snippet insertion (Superhuman has a Snippets folder with reusable email templates), no scheduled send (`Send later — Tue 8am`). These are all present in the underlying Superhuman backend and exposed in the UI, but absent from the CLI.

5. **Composability primitives.** No way to filter threads by a list of email addresses sourced from another tool. The session wanted to ask "for each Granola meeting attendee this week, what's the email follow-up state?" but had to scan the inbox manually and grep for attendee emails because the CLI has no `--participants-file` or stdin-piped intersection.

6. **Auth refresh state surfacing.** `auth status` reports `status: refresh_expired` on accounts with stale tokens, but read commands fail with opaque HTTP errors (`HTTP 400`, `unknown-error`, `context deadline exceeded`) instead of pointing at the refresh requirement. `sync --resources messages` errored with no actionable message — the root cause (expired refresh token) was hidden two layers deep.

**Why this work matters now.** The session was a real user workflow: triage the week's email, draft follow-ups for two meetings, and answer questions a non-AI assistant would have answered cleanly. The CLI failed at the points listed above, and each failure required the agent (and the user) to invent a workaround. Closing these gaps is the difference between "agent has a Superhuman tool" and "agent can do Superhuman work."

**Why a single PR.** The user explicitly chose single-PR delivery. The features form a layered stack — folder coverage requires store schema changes that also serve awaiting-reply, freshness work hooks PersistentPreRunE which also serves auth refresh surfacing, and the response envelope is shared by every read command — so splitting them into separate PRs would force re-coordination at each layer. Single PR is the explicit user preference recorded against the scope and delivery questions in the Phase 0 question round.

---

## Goals and Non-Goals

### Goals

- Restore parity with the Superhuman UI sidebar (Inbox, Starred, Drafts, Sent, Done, Auto Archived, Scheduled, Reminders, Snippets, Spam, Trash) so `threads list --type <any-folder>` works.
- Make the local SQLite store fresh-on-read via Gmail `users.history.list` delta polling, modeled after the granola autorefresh pattern from PR #571.
- Add a `bootstrap` command that pulls the last N (default 100) messages from each major folder on first run, so subsequent reads are SQLite-fast.
- Ship send-side features the UI has: reminders, snippets, scheduled send.
- Add composability primitives (`--participants-file`, `--intersect-with-stdin`, `messages get-by-rfc822`) without coupling to any specific external CLI.
- Surface the auth refresh state explicitly in `doctor` and as a fail-fast hint when reads error.
- Document everything in `SKILL.md` and `README.md`, and gate the PR on a PII scrub of known-author tokens.

### Non-Goals

- Calendar surface (already deferred per `project_superhuman_v1_1.md` to v1.2). Calendar APIs are not part of this PR.
- MCP server changes beyond what trivially follows from new CLI commands. The MCP tool list will be regenerated, but no new MCP-only features ship here.
- Gmail push notifications via Pub/Sub. `watch` mode uses history-list polling only; push subscription support is deferred.
- AI proxy / semantic search improvements. `messages.query` and `ai --query` issues observed today are out of scope; this plan replaces those flows with deterministic Gmail-search passthrough (U10).
- Granola-specific integration code. The plan adds the composability primitives that make a Granola join trivial in a shell pipeline; it does not ship any code that imports or depends on the Granola CLI.

### Deferred to Follow-Up Work

- Gmail push subscriptions on top of `watch` mode.
- Auto-label content classifiers (Marketing / News / Pitch / Social custom rules).
- Cross-account aggregation commands (`participants list --across-accounts`).
- The deeper auth re-auth flow (browser-driven OAuth flow inside the CLI) — this PR surfaces the failure clearly and points at `auth login --chrome`, but does not automate the re-auth itself.

---

## Output Structure

```
library/productivity/superhuman/
├── .printing-press-patches.json          # NEW patch entries cataloged
├── README.md                              # UPDATED: bootstrap flow, new flags
├── SKILL.md                               # UPDATED: command coverage
├── docs/
│   └── plans/
│       └── 2026-05-15-001-feat-superhuman-cli-major-overhaul-plan.md   # THIS FILE
├── internal/
│   ├── auth/
│   │   └── refresh_state.go               # NEW (U16): detect/classify expired-token states
│   ├── cli/
│   │   ├── agent_context.go               # UPDATED (U4): expose auto_refresh contract
│   │   ├── autorefresh.go                 # NEW (U4): PersistentPreRunE dispatcher
│   │   ├── autorefresh_test.go            # NEW (U4)
│   │   ├── bootstrap.go                   # NEW (U2): last-N per folder
│   │   ├── bootstrap_test.go              # NEW (U2)
│   │   ├── doctor.go                      # UPDATED (U16): binary_age, refresh_expired field
│   │   ├── drafts_new.go                  # NEW (U8): fresh outbound, no thread-id
│   │   ├── drafts_new_test.go             # NEW (U8)
│   │   ├── messages_list.go               # NEW (U10): Gmail search passthrough
│   │   ├── messages_list_test.go          # NEW (U10)
│   │   ├── messages_get_by_rfc822.go      # NEW (U15)
│   │   ├── messages_get_by_rfc822_test.go # NEW (U15)
│   │   ├── participants.go                # NEW (U12)
│   │   ├── participants_test.go           # NEW (U12)
│   │   ├── awaiting_reply.go              # NEW (U13)
│   │   ├── awaiting_reply_test.go         # NEW (U13)
│   │   ├── root.go                        # UPDATED (U4, U16): PersistentPreRunE, refresh hint
│   │   ├── send.go                        # UPDATED (U6, U7, U9): remind/snippet/schedule flags
│   │   ├── send_test.go                   # UPDATED (U6, U7, U9)
│   │   ├── snippets.go                    # NEW (U7)
│   │   ├── snippets_test.go               # NEW (U7)
│   │   ├── sync_autorefresh.go            # NEW (U4): per-surface refresh helpers
│   │   ├── sync_autorefresh_test.go       # NEW (U4)
│   │   ├── threads_list.go                # UPDATED (U1, U11): new --type values, --label, intersect
│   │   ├── threads_list_inbox_test.go     # UPDATED (U1): label-routed cases
│   │   └── watch.go                       # NEW (U14): ndjson event stream
│   │   └── watch_test.go                  # NEW (U14)
│   ├── gmail/
│   │   ├── history.go                     # NEW (U3): users.history.list wrapper
│   │   ├── history_test.go                # NEW (U3)
│   │   ├── messages.go                    # UPDATED (U10): list-with-query helper
│   │   └── client.go                      # UNCHANGED (read-only reference)
│   ├── store/
│   │   ├── store.go                       # UPDATED (U2, U3): schema v3 with messages, history_state
│   │   ├── schema_migrations.go           # NEW (U2): v2->v3 path
│   │   └── schema_migrations_test.go      # NEW (U2)
│   └── types/
│       └── envelope.go                    # NEW (U5): freshness response envelope
```

This is a scope declaration showing where new and modified files will land. The per-unit `**Files:**` sections remain authoritative.

---

## High-Level Technical Design

The overhaul threads through three architectural seams: the **Gmail passthrough** in `internal/gmail/`, the **SQLite store** in `internal/store/`, and the **PersistentPreRunE auto-refresh hook** in `internal/cli/root.go`. Every new command flows through all three.

**Read path with freshness contract (directional sketch, not implementation):**

```
┌─────────────────────┐
│ CLI command invoked │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────────────────────────────────┐
│ PersistentPreRunE: shouldSkipAutoRefresh()?      │
│  - sync, auth, doctor, help, etc: skip          │
│  - everything else: continue                    │
└──────────┬──────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────┐
│ runAutoRefresh(ctx)                              │
│  - cache surface (Superhuman backend writes)    │
│  - gmail surface (history.list delta poll)      │
│  - apply deltas to SQLite, advance history_id   │
└──────────┬──────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────┐
│ Command runs against now-fresh SQLite           │
│  Response envelope:                             │
│    { meta: { source: "local",                   │
│              synced_at: ...,                    │
│              history_id: ...,                   │
│              delta_polled_ms_ago: 312,          │
│              delta_applied: 2 } ,               │
│      results: [...] }                           │
└─────────────────────────────────────────────────┘
```

**Granola PR #571 is the canonical template.** Read `internal/cli/autorefresh.go` and `internal/cli/sync_autorefresh.go` from `library/productivity/granola/` before starting U4 — the skip list shape, refresh result struct, suppression matrix, opt-out precedence (flag → profile → env), and provenance line format are all directly adaptable. The major difference: where granola's two surfaces are "cache" (encrypted desktop file) and "api" (public REST), Superhuman's two surfaces are "writeMessage cache" (Superhuman backend) and "gmail" (Gmail passthrough). The dispatcher structure is the same; the per-surface helpers differ.

**Threads-list label decoupling (directional sketch):**

Today's `threads list --type inbox` builds a Gmail URL with a hardcoded `labelIds=INBOX` query parameter. The fix is to map the `--type` value through a `systemLabelID(typeName) string` function that resolves `inbox`, `sent`, `done`, `starred`, `archived`, `spam`, `trash`, `important` to their Gmail label IDs (`INBOX`, `SENT`, etc.). For `--type=draft|reminder|scheduled|...` the routing stays through `/v3/userdata.getThreads` because those are Superhuman-side lists, not Gmail folders. The split is:

| `--type` value | Backend |
|----------------|---------|
| inbox, sent, done, starred, archived, spam, trash, important | Gmail passthrough `/users/me/threads?labelIds=<MAPPED>` |
| draft, reminder, scheduled, snippet, signature, knowledge-base | Superhuman `/v3/userdata.getThreads` |

`done` and `archived` collapse to "remove `INBOX` label" semantics in Gmail — they're effectively `-label:INBOX in:anywhere`. The implementation resolves these via `users.threads.list?q=in:anywhere -label:inbox` rather than a single labelId because Gmail does not expose `DONE` or `AUTO_ARCHIVED` as system labels.

This sketch is directional guidance for review, not implementation specification.

---

## Key Technical Decisions

### D1. Mirror granola's autorefresh shape rather than inventing a new pattern

PR #571 (granola) already proved the PersistentPreRunE + skip-list + best-effort-with-stderr-provenance + three-tier-opt-out pattern works for a printed CLI in this repo. Re-using the structure (file naming, function names like `runAutoRefresh`, `shouldSkipAutoRefresh`, `refreshPlan`, `refreshResult`) keeps the two CLIs cognitively aligned and lets the next CLI maintainer recognize the pattern at a glance. The only change is per-surface implementations: where granola has `runCacheSync` and `runApiSync`, Superhuman has `runSuperhumanBackendSync` and `runGmailHistorySync`. Skip list is the same shape (sync, auth, doctor, help, version, completion, agent-context, profile, feedback, which).

### D2. Store schema bumps to v3 with a stamp-and-continue migration

`internal/store/store.go` declares `StoreSchemaVersion = 2`. U2 adds tables (`messages`, `messages_fts`, `history_state`, `label_index`) and U3 adds `history_state` rows for delta tracking. The migration follows the existing stamp-and-continue rule (see `schema_version_test.go::TestSchemaVersion_StampExistingZeroDB`): a v2 DB opened by a v3 binary runs the migration in-place. Refusing to open older binaries against a v3 DB is already the contract — this PR adds a clearer error message ("this database was created by a newer binary; downgrade by `rm <db-path>` or upgrade the binary").

### D3. Gmail historyId as the freshness signal, not full re-sync

`users.history.list?startHistoryId=<last>` is cheap (typically ~100ms) and returns only changes since the last sync. Storing `last_history_id` per account lets every read command issue one delta poll and apply minimal SQLite writes. Gmail keeps history for ~7 days; on `404 Not Found` (history expired) the CLI falls back to a full re-sync of recent threads (last 100 inbox + 100 sent by default), with a clear stderr warning. This trade-off matches what mature Gmail clients (e.g., the official Gmail API quickstart docs) recommend.

### D4. `--no-refresh` flag + `SUPERHUMAN_NO_AUTO_REFRESH=1` env + profile field

Three-tier opt-out (flag > profile > env), same precedence as granola. The flag is documented at the persistent flag level so every subcommand shows it in `--help`. Default is `on`. Pipeline mode (`--agent --json`) does not change the default — the refresh runs but the provenance line on stderr is suppressed for non-TTY callers.

### D5. Freshness envelope is opt-in via `--envelope full` initially

Wrapping every existing read command's output in a meta+results envelope would break consumers that grep stdout. U5 ships the envelope behind a `--envelope=full|minimal|off` flag with `minimal` as the default (existing stdout shape, plus a single stderr line summarizing freshness when interactive). A follow-up release can flip the default. This avoids a breaking change in this PR.

### D6. Reminders via `inReplyToRfc822Id` + `reminder` field, not a separate API

Superhuman's reminder system uses the same `writeMessage` payload pattern as sends, with an additional `reminder: { triggerAt: <unix-ms>, condition: "if-no-reply" | "always" }` field. U6 piggybacks on the existing `send` payload construction rather than introducing a new endpoint. Cancel-reminder is the existing `reminders cancel` command and already works.

### D7. Snippets as a first-class command, not a `send`-only flag

Snippets in Superhuman are persistent user-created templates with a subject, body, and label. They are useful outside the send context (an agent might want to list available snippets to pick one, or copy a snippet body into an arbitrary surface). U7 ships them as a `snippets list / get / create / update / delete` group; the `send --snippet <name>` flag in U7 is a thin wrapper that reads the snippet via the same code path and pipes it into the send payload as the body.

### D8. Watch mode uses history-list polling, not Gmail push subscriptions

Push subscriptions require Cloud Pub/Sub configuration and project-level GCP setup, which is out of scope for a CLI. U14 polls `users.history.list` every N seconds (default 30s, configurable via `--interval`) and emits ndjson events on each new delta. This is the same approach the underlying Gmail Quickstart watch examples recommend for non-Pub/Sub clients. Push support is deferred.

### D9. Patches mechanism for any PII or Matt-specific artifacts

`.printing-press-patches.json` is the existing customization catalog (see `AGENTS.md::Local Customizations`). Any inline `// PATCH:` comment in this PR's source is mirrored to that file with id, summary, reason, files list. The PII-scrub pass (U17) verifies no Matt-specific tokens (`mvh@esperlabs.ai`, the Google user ID, draft IDs, subject lines, person names from the session) appear in code, fixtures, or docs. Anything that needs to persist locally to Matt's account ergonomics lives in his local profile file or `.printing-press-patches.json`, not upstream.

---

## System-Wide Impact

| Surface | Impact |
|---------|--------|
| `internal/cli/threads_list.go` | New `--type` values, new `--label` / `--participants-file` / `--intersect-with-stdin` flags. Existing values continue to work. |
| `internal/cli/sync.go` | Unchanged user-facing surface; receives one new helper call from `runAutoRefresh` (U4) that bypasses the worker-pool ceremony. |
| `internal/cli/root.go` | `PersistentPreRunE` gains the auto-refresh hook; new `--no-refresh` persistent flag. |
| `internal/cli/send.go` | New flags: `--remind-in`, `--remind-on`, `--if-no-reply`, `--snippet`, `--schedule-at`. Existing flags unchanged. |
| `internal/store/store.go` | Schema bump to v3; new `messages`, `messages_fts`, `history_state`, `label_index` tables. Migration is stamp-and-continue. |
| `internal/gmail/messages.go` | New helper `ListWithQuery(ctx, q)` for Gmail search passthrough. |
| `internal/gmail/history.go` | New file: `ListHistory(ctx, startHistoryId)`. |
| `internal/auth/refresh_state.go` | New file: classify the `refresh_expired` state and emit a structured hint. |
| `internal/cli/doctor.go` | New fields in JSON output: `binary_age_days`, `tokens.refresh_expired_count`, `auto_refresh_active`. |
| `internal/cli/agent_context.go` | New top-level `auto_refresh` field exposing the contract (default, flag, env, profile field, skip list). |
| `SKILL.md` | New sections: Folders, Bootstrap, Reminders, Snippets, Watch, Composability. |
| `README.md` | Updated Auth Setup with refresh-state guidance; new sections matching `SKILL.md`. |
| `.printing-press-patches.json` | New entries for any inline `// PATCH:` comments introduced (auto-refresh dispatcher, label decoupling, etc.). |
| MCP server | Tool list regenerated from CLI changes via existing codegen path; no manual MCP edits in this PR. |

**Affected parties:** existing CLI users (additive changes, no breaking flags), agent consumers using `--agent --json` (envelope is opt-in; freshness is silent on non-TTY stderr), the MCP server consumers (will see new tools after regen).

---

## Implementation Units

### U1. Extend `threads list --type` to cover every Gmail/Superhuman folder

**Goal:** Make `threads list --type sent`, `--type done`, `--type starred`, `--type archived`, `--type spam`, `--type trash`, `--type important` all return real data. Keep existing values working.

**Requirements:** Closes Problem Frame #1.

**Dependencies:** None — pure CLI surface change.

**Files:**
- `internal/cli/threads_list.go` — extend valid-types whitelist; add `systemLabelID(typeName) string` mapping.
- `internal/cli/threads_list_inbox_test.go` — rename to `threads_list_test.go` or add a new `threads_list_labels_test.go` for parameterized label cases.
- `internal/gmail/messages.go` — extend list helper to accept arbitrary `labelIds` (currently hardcoded).
- `README.md`, `SKILL.md` — document new types.

**Approach:**
1. Add new constants (e.g., `typeSent`, `typeDone`, etc.) to the existing whitelist.
2. Split routing: Gmail-folder types route through `users.threads.list?labelIds=<id>`; Superhuman-side types continue through `/v3/userdata.getThreads`.
3. `done` and `archived` resolve to `q=in:anywhere -label:inbox` (Gmail does not expose them as system labels).
4. `important` resolves to `labelIds=IMPORTANT`.
5. Update the error message for unsupported types to include the new list.

**Patterns to follow:** Existing `threads_list.go` already has clear routing between the two backends in `runThreadsListInbox` vs the Superhuman path. Mirror that structure.

**Test scenarios:**
- `--type=sent` returns threads where `From == active account email` (mocked Gmail response with SENT label).
- `--type=starred` returns threads with `STARRED` label.
- `--type=archived` returns threads where the query is `in:anywhere -label:inbox`; verify the query string sent to Gmail.
- `--type=spam` returns threads with `SPAM` label; verify Gmail-side filter applied.
- `--type=trash` returns threads with `TRASH` label.
- `--type=important` returns threads with `IMPORTANT` label.
- Existing types (`inbox`, `draft`, `scheduled`, `reminder`, `snippet`, `signature`, `knowledge-base`) continue to return their existing shapes.
- Invalid `--type=foo` returns the error with the full extended list of valid values.
- `--page-token` continues to work for Gmail-routed types.
- `--limit` continues to bound the response across all routed paths.

**Verification:** Manual smoke against a real Esper Labs account (via patches-overlaid local profile) returns non-empty SENT and STARRED results.

---

### U2. SQLite bootstrap-to-store + `bootstrap` command

**Goal:** Pull last N (default 100) messages per folder into the local SQLite store on first run, enabling SQLite-fast queries for subsequent reads. Schema bumps to v3.

**Requirements:** Closes Problem Frame #2 (foundation), unblocks U10, U11, U12, U13.

**Dependencies:** U1 (folder coverage drives which folders we bootstrap).

**Files:**
- `internal/store/store.go` — bump `StoreSchemaVersion` to 3; add `messages`, `messages_fts`, `history_state`, `label_index` table definitions.
- `internal/store/schema_migrations.go` — new file; v2→v3 migration.
- `internal/store/schema_migrations_test.go` — new file; verifies migration is idempotent and preserves existing v2 data.
- `internal/cli/bootstrap.go` — new command: `bootstrap --account <e> --folders sent,inbox,archived --per-folder 100`.
- `internal/cli/bootstrap_test.go` — new file.

**Approach:**
1. Define `messages` table with columns mirroring Gmail's message resource shape: `id`, `thread_id`, `account_email`, `label_ids` (JSON array), `from`, `to`, `cc`, `subject`, `snippet`, `body_plain`, `body_html`, `rfc822_id`, `internal_date`, `synced_at`.
2. Define `messages_fts` virtual table over `subject`, `body_plain`, `from`, `to`, `cc`, `snippet`.
3. Define `history_state` table: `account_email PRIMARY KEY`, `last_history_id`, `last_polled_at`, `last_full_sync_at`.
4. `label_index` is a denormalized view for `--type=<label>` queries; refreshed via trigger on `messages` upsert.
5. The `bootstrap` command iterates folders, calls `gmail.ListMessages` per label, fetches each message in batches of 100 (Gmail's `batchGet` style), upserts into `messages`.
6. Stamp `history_state.last_history_id` from the latest message's `historyId` so subsequent auto-refresh delta polls have a starting point.
7. Provide `--full` to discard previous state and re-bootstrap.
8. Emit ndjson progress events under `--agent --json`: `{event:"bootstrap_progress", folder:"sent", fetched:42, total:100}`.

**Patterns to follow:**
- The existing `sync.go` resource iteration shape (workers, channel, summary).
- `store.go` upsert patterns (see `upsert_batch_test.go`).
- Schema versioning as in `schema_version_test.go::TestSchemaVersion_StampExistingZeroDB`.

**Test scenarios:**
- Fresh DB stamps v3; bootstrap creates rows in `messages` for each folder requested.
- `--per-folder 25` limits each folder to 25 rows.
- `--full` discards previous `history_state` and re-fetches.
- Mocked Gmail response with 5 messages produces 5 rows, FTS index queryable by subject.
- Migration from v2 DB (with no `messages` table) populates schema and leaves v2 data intact.
- Bootstrap is interruptible: SIGINT mid-fetch leaves partial state recorded in `history_state.last_polled_at` for resume.
- Re-running `bootstrap` without `--full` is idempotent for already-synced messages (upsert dedup by `id`).
- `bootstrap --folders nonexistent` returns a usage error listing valid folder names.

**Verification:** `sqlite3 <db> "SELECT COUNT(*) FROM messages WHERE label_ids LIKE '%SENT%'"` matches the count returned by `threads list --type sent --limit 1000`.

---

### U3. Gmail `users.history.list` wrapper + delta application

**Goal:** Implement the cheap delta-poll primitive that auto-refresh (U4) will call on every command, and the local-store mutation logic that applies its results.

**Requirements:** Closes Problem Frame #2 (mechanism); enables U4.

**Dependencies:** U2 (schema must have `history_state` and `messages` tables).

**Files:**
- `internal/gmail/history.go` — new file: `ListHistory(ctx, startHistoryId) (*HistoryResponse, error)` wrapping `GET /users/me/history?startHistoryId=<n>`.
- `internal/gmail/history_test.go` — new file; covers normal delta, expired-history 404 path, empty delta, label-only changes.
- `internal/store/store.go` — add `ApplyHistoryDelta(ctx, delta *gmail.HistoryResponse) error` helper.
- Existing `internal/gmail/client.go` is used as-is for the HTTP+refresh path.

**Approach:**
1. `ListHistory` calls Gmail's history endpoint, parses the response into a `HistoryResponse` struct with `History []HistoryRecord` and `HistoryId string`.
2. Each `HistoryRecord` carries `MessagesAdded`, `MessagesDeleted`, `LabelsAdded`, `LabelsRemoved` arrays.
3. `ApplyHistoryDelta` walks the records and: inserts new messages (fetching full body via `messages.get` when needed), removes deleted ones, updates label arrays for label changes.
4. On 404 ("history expired"), return a typed `HistoryExpiredError` so callers know to fall back to a partial re-bootstrap.
5. Update `history_state.last_history_id` to the response's top-level `historyId` after a successful apply.

**Patterns to follow:** Mirror `internal/gmail/messages.go` shape — typed response, typed errors, `Client.DoWithRefresh` for HTTP.

**Test scenarios:**
- Empty delta (no changes since `startHistoryId`) returns no rows and advances `last_history_id`.
- `MessagesAdded` with 3 records produces 3 rows in `messages` and the corresponding FTS entries.
- `MessagesDeleted` with 1 record removes 1 row and its FTS entry.
- `LabelsAdded` with `{messageId: X, labelIds: ["STARRED"]}` updates row X's `label_ids` JSON array.
- `LabelsRemoved` with INBOX label flips that message from inbox view (label_index refresh).
- Gmail 404 "Requested entity was not found" maps to `HistoryExpiredError`.
- Network error mid-apply leaves `history_state.last_history_id` unchanged (atomic apply, rollback on error).

**Verification:** Integration test: bootstrap to N records, simulate a Gmail delta with 1 add + 1 delete + 1 label change, assert SQLite matches expected post-state.

---

### U4. Auto-refresh PersistentPreRunE hook

**Goal:** Wire `runAutoRefresh` into root's `PersistentPreRunE` so every command (outside the skip list and any opt-out) refreshes the local store as its first action.

**Requirements:** Closes Problem Frame #2 (delivery).

**Dependencies:** U3 (delta-poll primitive must exist).

**Files:**
- `internal/cli/autorefresh.go` — new file; dispatcher, skip list, refresh result types. Modeled directly on `library/productivity/granola/internal/cli/autorefresh.go` from PR #571.
- `internal/cli/sync_autorefresh.go` — new file; per-surface helpers (`runSuperhumanBackendRefresh`, `runGmailHistoryRefresh`).
- `internal/cli/autorefresh_test.go` — new file.
- `internal/cli/sync_autorefresh_test.go` — new file.
- `internal/cli/agent_context.go` — new top-level `auto_refresh` field with `default`, `flag`, `env`, `profile_field`, `surfaces`, `skip_list`.
- `internal/cli/root.go` — wire `PersistentPreRunE`; add `--no-refresh` persistent flag.

**Approach:**
1. Skip list: `sync`, `auth`, `doctor`, `help`, `version`, `completion`, `agent-context`, `profile`, `feedback`, `which`. Walk `cmd.Parent()` to handle subcommands of these.
2. Refresh plan: detect which surfaces are configured for the active account. Superhuman backend writes always available if any account is configured. Gmail history-poll available if a Gmail-bound account has a non-expired access token.
3. Per-surface: `runGmailHistoryRefresh` calls `gmail.ListHistory(startHistoryId=history_state.last_history_id)`, then `store.ApplyHistoryDelta`. On `HistoryExpiredError` fall back to a small re-bootstrap (last 50 messages per folder).
4. Provenance line on stderr: `auto-refresh: gmail=ok (520ms, 2 rows)  cache=ok (110ms)` only when stderr is a TTY and not under `--agent / --json / --compact / --quiet`.
5. Three-tier opt-out: `--no-refresh` flag (wins) → profile field `no-refresh` → `SUPERHUMAN_NO_AUTO_REFRESH=1` env.
6. Best-effort: refresh failures emit a one-line warning and the original command proceeds. `gmail=failed (HTTP 401 — run 'auth login --chrome')` is the example error shape.

**Patterns to follow:** Read PR #571's `autorefresh.go` end-to-end before writing. The skip-list ancestor walking, suppression matrix, and provenance line format port directly.

**Execution note:** Implement test-first for the dispatcher (U4) — the dispatcher's correctness is dominated by the skip-list semantics and opt-out precedence, which is exactly what tests should pin first.

**Test scenarios:**
- `meetings list` (Granola comparison test name, replace with `threads list --type sent`) triggers refresh; `sync` does not.
- `--no-refresh` flag skips refresh.
- `SUPERHUMAN_NO_AUTO_REFRESH=1` env skips refresh; flag wins over env.
- Profile field `no-refresh: true` skips refresh; flag wins over profile.
- Auth path failure (`auth use` with no configured account) skips refresh silently.
- Gmail refresh failure (HTTP 401) emits stderr warning, command proceeds.
- Gmail refresh failure (history expired) triggers fallback bootstrap, applies partial sync, advances history_id.
- Provenance line absent under `--agent`.
- Provenance line absent when stderr is not a TTY.
- `agent-context --json` includes the full `auto_refresh` contract object (default=on, flag=`--no-refresh`, env=`SUPERHUMAN_NO_AUTO_REFRESH`, profile_field=`no-refresh`, surfaces=`["gmail","cache"]`, skip_list=[...]).

**Verification:** Smoke: `superhuman-pp-cli threads list --type sent --limit 1` after waiting >5 minutes shows the provenance line and returns fresh data. `superhuman-pp-cli --no-refresh threads list --type sent --limit 1` shows no provenance line.

---

### U5. Freshness response envelope (opt-in)

**Goal:** When `--envelope=full`, every read command wraps its output in `{meta: {...}, results: ...}` exposing freshness state.

**Requirements:** Surfaces the freshness contract to agent callers.

**Dependencies:** U4 (PersistentPreRunE provides the freshness state).

**Files:**
- `internal/types/envelope.go` — new file: `type Envelope struct { Meta EnvelopeMeta; Results json.RawMessage }`.
- `internal/cli/deliver.go` — extend with `WriteEnvelope(out io.Writer, data any, meta EnvelopeMeta)`.
- `internal/cli/root.go` — add `--envelope` persistent flag with values `full | minimal | off` (default `minimal`).
- Every read command updates its output to route through `WriteEnvelope` when `--envelope=full`.

**Approach:**
1. `EnvelopeMeta` carries: `source` ("local" | "live" | "mixed"), `synced_at`, `history_id`, `delta_polled_ms_ago`, `delta_applied`, `staleness_seconds`.
2. `minimal` default keeps existing stdout shape so existing consumers do not break; freshness info is only printed to stderr as a single line when interactive.
3. `full` wraps stdout output in the envelope. Agents opt in via `--envelope=full` or `--agent --envelope=full`.
4. `off` returns bare results with no envelope or stderr line.

**Test scenarios:**
- `--envelope=full` wraps `threads list` output; `meta.source=="local"` when refresh was a no-op.
- `--envelope=full` after a delta-applied refresh shows `meta.delta_applied > 0`.
- `--envelope=minimal` (default) shows raw existing shape.
- `--envelope=off` suppresses the stderr freshness line.
- `--envelope=invalid` returns a usage error listing valid values.

**Verification:** `superhuman-pp-cli threads list --type sent --envelope=full | jq .meta` returns a populated metadata object.

---

### U6. Reminder flags on `send`

**Goal:** `send --remind-in 2d --if-no-reply` schedules a reminder that fires 2 days later if the recipient has not replied, matching Superhuman's UI behavior.

**Requirements:** Closes Problem Frame #4 (reminders).

**Dependencies:** None (extends existing `send.go`).

**Files:**
- `internal/cli/send.go` — add `--remind-in <duration>`, `--remind-on <RFC3339-or-relative>`, `--if-no-reply` flags.
- `internal/cli/send_test.go` — add cases.
- `internal/cli/types/` (or wherever the send payload is built) — extend payload struct with `reminder` field.

**Approach:**
1. `--remind-in 2d` parses Go-duration plus day/week shortcuts (`2d`, `1w`, `48h`).
2. `--remind-on 2026-05-20T08:00:00-07:00` accepts RFC3339 or human-friendly forms.
3. `--if-no-reply` sets the reminder condition; default condition is `always`.
4. Reminder field on the writeMessage payload: `{ "reminder": { "triggerAt": <unix-ms>, "condition": "if-no-reply" | "always" } }`.
5. Mutually exclusive: `--remind-in` and `--remind-on` cannot both be set.
6. Profile field `remind-default` allows persistent default (e.g., every send gets `2d if-no-reply`).

**Test scenarios:**
- `--remind-in 2d` produces a reminder payload with `triggerAt` ≈ now + 2 days.
- `--remind-in 2d --if-no-reply` sets `condition: if-no-reply`.
- `--remind-on 2026-06-01T08:00:00-07:00` produces the exact timestamp.
- `--remind-in 2d --remind-on ...` returns a usage error.
- `--remind-in 30s` is rejected as below the minimum reminder window (assume Superhuman requires ≥ 1h).
- Profile `remind-default: 2d` applied; explicit `--remind-in 1d` overrides.
- Send without reminder flags produces a payload with no `reminder` field (existing behavior).

**Verification:** Dry-run send with `--remind-in 2d --if-no-reply` produces the expected payload JSON. Real send (in user's hands) shows the reminder in Superhuman's Reminders folder.

---

### U7. Snippets command group + `send --snippet`

**Goal:** Expose Superhuman's Snippets folder via CLI: list, get, create, update, delete. Allow `send --snippet <name>` to substitute the snippet body.

**Requirements:** Closes Problem Frame #4 (snippets).

**Dependencies:** None.

**Files:**
- `internal/cli/snippets.go` — new file: `snippets list / get / create / update / delete`.
- `internal/cli/snippets_test.go` — new file.
- `internal/cli/send.go` — add `--snippet <name>` and `--var key=value` flags (already partial UPDATE in U6).

**Approach:**
1. `snippets list` calls `/v3/userdata.getThreads` with `--type=snippet` (already supported); returns snippet `id`, `name`, `subject_preview`, `body_preview`.
2. `snippets get <name>` returns full snippet body (`writeMessage`-style draft contents).
3. `snippets create --name <n> --body <b>` upserts via `/v3/userdata.writeMessage` with `labelIds: ["SNIPPET"]`.
4. `snippets update <name>` modifies the existing snippet.
5. `snippets delete <name>` removes it.
6. `send --snippet <name>` reads the snippet body and uses it as the send body. `--var first_name=Randy` substitutes `{{first_name}}` placeholders.
7. Variable substitution is simple `{{key}}` text replacement, no Go-template features (no conditionals, loops). Documented as such.

**Test scenarios:**
- `snippets list` returns mocked snippet array.
- `snippets get foo` returns the snippet body.
- `snippets create --name foo --body "Hi"` writes the snippet.
- `snippets update foo --body "Hello"` updates the body.
- `snippets delete foo` removes it.
- `send --snippet foo --to a@b.com --subject "test"` uses the snippet body.
- `send --snippet foo --var first_name=Alice` substitutes `{{first_name}}`.
- `send --snippet nonexistent` returns an error.
- Body with unmatched `{{nope}}` placeholder: log a warning and leave the literal in place (no abort, no substitution surprise).

**Verification:** Live test against the Esper Labs account: `snippets list` returns the user's actual snippets (`Esper Blurb`, `scheduling`, etc., per the session screenshot).

---

### U8. `drafts new` command — fresh outbound, no thread-id required

**Goal:** Create a new outbound draft (no existing thread) with a single command. Currently this requires hand-rolling the writeMessage payload.

**Requirements:** Closes Problem Frame #3.

**Dependencies:** None.

**Files:**
- `internal/cli/drafts_new.go` — new file: `drafts new --to <e> --subject <s> --body <b>`.
- `internal/cli/drafts_new_test.go` — new file.
- `internal/cli/drafts_write.go` — refactor common payload construction into helper `buildDraftPayload(opts) *DraftPayload` so `drafts new` and `drafts write` share it.

**Approach:**
1. `drafts new` auto-generates `draft-id` (`draft00` + 14 hex), `thread-id` (same as draft-id for new threads), `rfc822-id` (UUID-based @we.are.superhuman.com).
2. Constructs the full `writeMessage` payload (path, value, fingerprint, schemaVersion=3, etc.).
3. POSTs to `/v3/userdata.writeMessage` with the same auth path the existing `drafts write` uses.
4. Returns the new `draft-id` for follow-up commands.
5. Accepts `--to`, `--cc`, `--bcc` (repeatable), `--subject`, `--body | --body-file | --body-stdin`, `--snippet <name>` (via U7), reminder flags (via U6), `--schedule-at` (via U9).

**Test scenarios:**
- `drafts new --to a@b.com --subject foo --body bar` creates a draft, returns the draft-id, persists in Superhuman's Drafts folder.
- Auto-generated draft-id matches `draft00[0-9a-f]{14}` pattern.
- Auto-generated thread-id equals draft-id for new outbound.
- `--cc` and `--bcc` flags accept multiple emails.
- `--body-stdin` reads stdin body.
- `--snippet foo` (when U7 lands) substitutes snippet body.
- `--remind-in 2d` (when U6 lands) adds reminder.
- `--dry-run` returns the constructed payload without POSTing.

**Verification:** `drafts new --to test@example.com --subject test --body test --dry-run | jq .step1.body.writes[0].value.subject` returns `"test"`.

---

### U9. `send --schedule-at <when>`

**Goal:** Schedule a send for a future time via Superhuman's Send Later feature.

**Requirements:** Closes Problem Frame #4 (scheduled send).

**Dependencies:** None.

**Files:**
- `internal/cli/send.go` — add `--schedule-at <RFC3339-or-relative>` flag.
- `internal/cli/send_test.go` — add cases.

**Approach:**
1. `--schedule-at 2026-05-20T08:00:00-07:00` sets the `scheduledFor` field on the writeMessage payload.
2. Accept relative forms: `--schedule-at "Mon 8am"`, `--schedule-at "+2d"`.
3. Mutually exclusive with `--undo` (undo is for immediate-send hold; schedule is for future-send).
4. Reminder flags (U6) and schedule (U9) can coexist: a scheduled send with a follow-up reminder is valid.
5. `--cancel-schedule <draft-id>` cancels a scheduled send (writes `scheduledFor: null`).

**Test scenarios:**
- `--schedule-at "2026-05-20T08:00:00-07:00"` sets `scheduledFor` to the parsed RFC3339 time.
- `--schedule-at "+2d"` schedules for now+2 days.
- `--schedule-at "Mon 8am"` schedules for the next Monday at 8am in the user's local TZ.
- Past time (`--schedule-at "2024-01-01"`) returns an error.
- `--schedule-at` with `--undo 30s` returns a usage error (mutually exclusive).
- `--cancel-schedule draft00XXX` removes the schedule from an existing draft.

**Verification:** `send --to test@example.com --subject test --body test --schedule-at "+2d" --dry-run` shows `scheduledFor` populated.

---

### U10. `messages list --query "<gmail-search>"` — Gmail search passthrough

**Goal:** Direct passthrough of Gmail search syntax (`from:`, `to:`, `after:`, `has:attachment`, `label:`, free text). Replaces the unreliable AI-proxy semantic search.

**Requirements:** Closes Problem Frame #2 (query side); replaces broken `messages.query` flow.

**Dependencies:** None.

**Files:**
- `internal/cli/messages_list.go` — new file.
- `internal/cli/messages_list_test.go` — new file.
- `internal/gmail/messages.go` — add `ListWithQuery(ctx, q string, opts) (*ListResponse, error)`.

**Approach:**
1. `messages list --query "from:alice@example.com after:2026-05-01"` calls Gmail's `users.messages.list?q=<query>`.
2. Returns message IDs + thread IDs paginated via Gmail's `nextPageToken`.
3. `--full` resolves each ID via `messages.get` and returns full headers/body — slow but useful for direct inspection.
4. `--limit N` bounds the count.
5. Output respects `--envelope` (U5).

**Test scenarios:**
- `--query "from:a@b.com"` returns mocked messages matching the from filter.
- `--query "after:2026-05-01 before:2026-05-15"` produces the date-bounded query.
- `--query "subject:test has:attachment"` passes both filters.
- `--full --limit 5` fetches 5 full messages.
- `--page-token` continues pagination.
- Empty query returns a usage error.
- Invalid Gmail query syntax (`--query "from:[malformed"`) surfaces Gmail's error message.

**Verification:** `messages list --query "from:noreply@github.com" --limit 3` returns recent GitHub notifications.

---

### U11. `threads list --label`, `--participants-file`, `--intersect-with-stdin`

**Goal:** Composability primitives: filter threads by an Auto Label, by a file of emails, or by piping IDs / emails from another tool.

**Requirements:** Closes Problem Frame #5.

**Dependencies:** U1 (folder coverage), U2 (store schema for participant index).

**Files:**
- `internal/cli/threads_list.go` — extend with new flags.
- `internal/cli/threads_list_intersect_test.go` — new file.
- `internal/store/store.go` — add `ListThreadsByParticipants(ctx, emails []string, since time.Time)`.

**Approach:**
1. `--label "Pitch"` (Auto Label name from Superhuman) resolves via `labels list` → labelId, then routes through Gmail thread list with the label filter.
2. `--participants-file <path>` reads newline-delimited or JSON-array-delimited emails; filters threads to those involving any listed email.
3. `--intersect-with-stdin` reads stdin as newline-delimited thread IDs or emails (auto-detect: if matches `[0-9a-f]{16}`, treat as thread ID; else treat as email).
4. Filtering happens against the local SQLite `messages` table (post-U2), so no extra Gmail calls.
5. Combined flags AND together: `--type sent --label Pitch --participants-file customers.txt` returns sent threads labeled Pitch involving listed customers.

**Test scenarios:**
- `--label "Pitch"` returns mocked threads matching that label name.
- `--participants-file emails.txt` filters to threads where any participant matches.
- `--intersect-with-stdin` reads piped thread IDs and filters to that subset.
- `--intersect-with-stdin` reads piped emails (auto-detected) and filters to threads involving those emails.
- Mixed stdin (some IDs, some emails) is treated per-line as the auto-detected kind.
- Combined `--type sent --label Pitch --participants-file emails.txt` ANDs the filters.
- Empty file or empty stdin returns no results (not a usage error).
- File path that does not exist returns a usage error.

**Verification:** `echo "test@example.com" | superhuman-pp-cli threads list --type sent --intersect-with-stdin --json` returns sent threads involving that address.

---

### U12. `participants list` / `participants show` — composability primitive

**Goal:** Expose the per-correspondent aggregation that makes joins to other tools (Granola, CRM, contact-goat) trivial.

**Requirements:** Closes Problem Frame #5.

**Dependencies:** U2 (messages must be in store), U3 (delta keeps them fresh).

**Files:**
- `internal/cli/participants.go` — new file: `participants list / show`.
- `internal/cli/participants_test.go` — new file.

**Approach:**
1. `participants list --since 7d` aggregates from the `messages` table: for each unique email address in From / To / Cc, count messages and last-touched timestamp.
2. Output: `[{email, count, last_touched_at, direction: "received|sent|both"}, ...]`.
3. `participants show <email>` returns per-email details: thread count, message count, first/last touch, recent subjects.
4. Sortable via `--sort count|last_touched|email`.
5. Honors `--account` for multi-account installs.

**Test scenarios:**
- `--since 7d` aggregates only messages from the last 7 days.
- `--sort count` returns by descending message count.
- `participants show test@example.com` returns the per-email detail object.
- `participants show nonexistent@example.com` returns an empty result, not an error.
- `--direction sent` filters to people the user has sent to.
- `--direction received` filters to people who have sent to the user.
- Multi-account: `--account a@b.com` and `--account c@d.com` produce independent aggregations.

**Verification:** `participants list --since 7d --json | jq 'length'` matches the count of unique emails in `messages` table over the last 7 days.

---

### U13. `awaiting-reply` workflow command

**Goal:** Direct command for the question the session spent an hour answering: "which threads have an external last message, are older than N hours, and have not been replied to?"

**Requirements:** Closes the primary use case the session exposed.

**Dependencies:** U2, U3 (synced messages with label state).

**Files:**
- `internal/cli/awaiting_reply.go` — new file.
- `internal/cli/awaiting_reply_test.go` — new file.

**Approach:**
1. `awaiting-reply --since 7d --external-only --min-age 4h`.
2. Queries local store: threads where the latest message's `From` is not the active account, the thread is in INBOX (not archived), and the latest message age > `--min-age`.
3. `--external-only` excludes intra-team threads (sender domain matches account domain).
4. `--include-snoozed` brings in reminders/snoozed threads (default excludes).
5. `--include-archived` looks at done/archived too (default INBOX only).
6. Output: `[{thread_id, last_message_id, from, subject, last_message_at, age_hours, snippet}, ...]`.

**Test scenarios:**
- Thread with last message from external sender, age 5 hours, in INBOX, `--min-age 4h` → included.
- Same thread with `--min-age 6h` → excluded.
- Thread with last message from active account (self-sent) → excluded.
- `--external-only` excludes threads where sender domain == account domain.
- `--include-snoozed` brings reminder-flagged threads in.
- Multi-account: per-account aggregation; `--account` filters.
- Empty result set returns `[]`, not an error.
- `--since 1d` bounds the window.

**Verification:** `awaiting-reply --since 7d --external-only` returns threads matching the manually-derived list from this session's triage.

---

### U14. `watch` mode — ndjson event stream

**Goal:** Long-running command that polls Gmail history every N seconds and emits ndjson events for each new message, label change, or delete.

**Requirements:** Closes Problem Frame #5 (composability with reactive agents).

**Dependencies:** U3 (history-poll primitive).

**Files:**
- `internal/cli/watch.go` — new file.
- `internal/cli/watch_test.go` — new file.

**Approach:**
1. `watch --account <e> --interval 30s` polls `users.history.list` every 30 seconds.
2. For each new record, emits ndjson lines: `{event: "message_added", message_id: ..., thread_id: ..., from: ..., subject: ...}`.
3. Other events: `message_deleted`, `label_added`, `label_removed`, `thread_archived`.
4. Honors SIGINT for graceful shutdown.
5. `--once` runs a single poll and exits (useful for testing).
6. `--filter "label:INBOX"` filters events by Gmail-search-style predicate.

**Test scenarios:**
- Single poll with one mocked delta produces one `message_added` event.
- Multiple events in one delta produce multiple ndjson lines.
- `--once` exits after one poll.
- SIGINT during poll-wait terminates cleanly with exit code 0.
- `--filter "label:INBOX"` excludes events for non-inbox messages.
- `--interval 0` returns a usage error (must be > 0).
- History expiry during a watch triggers re-bootstrap and continues watching.

**Verification:** `watch --once --json` against a recent account returns at least one event when delta is non-empty.

---

### U15. `messages get-by-rfc822 <id>` — RFC822 lookup primitive

**Goal:** Lookup a message by its RFC822 ID (the universal cross-mail-system identifier).

**Requirements:** Composability primitive.

**Dependencies:** U2 (rfc822_id is now indexed in store).

**Files:**
- `internal/cli/messages_get_by_rfc822.go` — new file.
- `internal/cli/messages_get_by_rfc822_test.go` — new file.

**Approach:**
1. Query local store by `rfc822_id` column first.
2. On miss, fall back to Gmail search `q=rfc822msgid:<id>`.
3. Returns the message in the same shape as `messages get`.

**Test scenarios:**
- Local hit returns the message from SQLite (no Gmail call).
- Local miss falls back to Gmail search.
- Gmail miss returns "not found" exit code 3.
- `--no-fallback` skips Gmail and exits 3 on local miss.

**Verification:** `messages get-by-rfc822 <known-id>` from local store returns the expected message.

---

### U16. Auth refresh state surfacing in `doctor` and command errors

**Goal:** When tokens are expired, `doctor` reports it clearly and every read command fails fast with an actionable hint instead of an opaque HTTP error.

**Requirements:** Closes Problem Frame #6.

**Dependencies:** None.

**Files:**
- `internal/auth/refresh_state.go` — new file: `ClassifyRefreshState(account) (RefreshState, string)` where state is one of `ok`, `expired_access_can_refresh`, `expired_refresh_needs_relogin`.
- `internal/cli/doctor.go` — extend JSON output with `binary_age_days`, `tokens` block (per-account state), `auto_refresh_active`.
- `internal/cli/root.go` — when a read command errors due to auth, the wrapper detects the state and appends `Run 'auth login --chrome' to re-authenticate.` to stderr.

**Approach:**
1. `ClassifyRefreshState` inspects the auth Store for the active account; checks `access_expires_at` and `refresh_expires_at`.
2. `doctor --json` returns a top-level `tokens` field: `{<email>: {access_state: ok|expired, refresh_state: ok|expired, hint: ""}}`.
3. `binary_age_days` is `time.Since(binary mtime).Hours() / 24`. When > 14 days, doctor surfaces `binary_outdated_warning: true` with a hint to `go install ...@latest`.
4. When a command errors with HTTP 401 from the writeMessage path, root's error wrapper surfaces the auth hint above any other error context.

**Test scenarios:**
- Account with valid tokens: `access_state=ok refresh_state=ok`.
- Account with expired access + valid refresh: `expired_access_can_refresh`.
- Account with both expired: `expired_refresh_needs_relogin`, hint suggests `auth login --chrome`.
- `doctor --agent` includes the tokens block.
- `binary_age_days` is computed from binary mtime.
- A command failing with HTTP 401 prints the auth hint on stderr before the original error.

**Verification:** With an expired-refresh-token account, `superhuman-pp-cli threads list --type sent` prints the hint on stderr and exits non-zero.

---

### U17. Documentation, patches catalog, PII scrub gate

**Goal:** Update SKILL.md, README.md, agent-context, and `.printing-press-patches.json` to reflect new features. Scrub any PII from code/tests/fixtures before push.

**Requirements:** Ships the user-facing surface.

**Dependencies:** All previous units.

**Files:**
- `SKILL.md` — sections for: Folders (U1), Bootstrap (U2), Auto-Refresh (U4), Envelope (U5), Reminders (U6), Snippets (U7), Drafts New (U8), Schedule (U9), Search (U10), Composability (U11, U12, U15), Awaiting Reply (U13), Watch (U14), Auth State (U16).
- `README.md` — Auth Setup, new commands, bootstrap flow, freshness contract.
- `.printing-press-patches.json` — append entries for every `// PATCH:` comment introduced in U1, U4, U10, U11, U16.
- `tools-manifest.json` — regenerate to include new commands.
- `manifest.json`, `spec.yaml` — extend where needed.
- `.printing-press-pii-polish.json` — extend with any new PII patterns that need scrubbing.

**Approach:**
1. Run a `grep -rn` pre-merge scan for known PII tokens:
   - Active-author email patterns (e.g., personal `@gmail.com`, `@esperlabs.ai` work email — passed as a parameter to the scrub script, never committed in plaintext).
   - Known Google user ID strings (15+ digit numeric IDs).
   - Draft ID patterns from session output (`draft00[0-9a-f]{14}` that appear in fixtures).
   - Person names mentioned during the originating session (passed via the scrub-list config, never committed in plaintext).
   - Specific subject lines from the session's emails.
2. Any matches that are real PII get scrubbed (replaced with example placeholders like `alice@example.com`).
3. Any matches that are false positives (e.g., a regex example showing the draft-id pattern) get an allowlist entry in `.printing-press-pii-polish.json`.
4. Document all new commands in SKILL.md following the existing structure (Unique Capabilities → Command Reference → Recipes → Agent Mode).
5. Add a "Migrating from pre-overhaul" subsection noting: bootstrap is run automatically on first command, no flag changes break existing usage, envelope is opt-in.

**Patterns to follow:** PR #571's `library/productivity/granola/SKILL.md` and `README.md` diffs for the auto-refresh section.

**Test scenarios:**
- `grep -rEn '(known-PII-pattern-1|known-PII-pattern-2)' library/productivity/superhuman/` returns no matches.
- `tools-manifest.json` after regen lists every new command.
- `agent-context --json | jq .commands` includes every new command with its description.
- SKILL.md command coverage matches `--help` output (parity check).

**Verification:** Manual review of the diff with the PII scrub script; PR description includes the scrub script's all-clear output as evidence.

**Execution note:** The PII scrub is a hard gate — the PR must not push until the scrub returns clean.

---

## Risk Analysis and Mitigation

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Schema v2→v3 migration corrupts existing data | Low | High | Migration test covers stamp-and-continue, idempotency, and v2 fixture roundtrip. CI runs the test against a fixture v2 DB. |
| Auto-refresh adds visible latency to every command | Medium | Medium | Default skip list covers all metadata commands. Gmail delta poll typically <300ms. `--no-refresh` and the env var provide hard opt-out. Minimum-interval TTL (`SUPERHUMAN_AUTO_REFRESH_MIN_INTERVAL=2s`) deferred to a follow-up if a high-frequency caller emerges. |
| Gmail history expires (>7 days) mid-watch | Medium | Low | `HistoryExpiredError` triggers a partial re-bootstrap (last 50 messages per folder), watch continues. Stderr warning emitted. |
| PII leakage into upstream PR | Medium | High | U17 includes a hard grep gate over a configurable PII-token list. Reviewer must see the all-clear output in PR description. |
| Breaking change to existing `threads list` consumers | Low | Medium | Existing `--type` values continue to work. New values are additive. Default `--envelope=minimal` keeps the existing stdout shape. |
| `users.history.list` rate limits during initial heavy refresh | Low | Low | Gmail's history endpoint has generous quotas. Bootstrap is a one-time cost. Watch's default 30s interval is well within quota. |
| Snippet variable substitution interpreted differently by users (assumed Go template, got plain replace) | Low | Low | Documented explicitly in SKILL.md and command help: `{{key}}` is plain text replacement, no logic. |
| Auth refresh hint creates noise for accounts the user doesn't actively use | Low | Low | Hint is only printed when a command actually errors with 401. `doctor` surfaces the per-account state explicitly. |
| Watch mode left running indefinitely consumes battery | Low | Low | Documented as long-running; SIGINT terminates cleanly. No timer-on-by-default heartbeat. |

---

## Test Strategy

- **Unit tests** for every new file, covering scenarios enumerated per unit.
- **Integration tests** under `internal/cli/` using `httptest.Server` mocks for Gmail and the Superhuman backend (the existing pattern; see `messages_get_test.go`).
- **Migration tests** against a fixture v2 SQLite DB.
- **Smoke tests** in `_smoke_test.go` files marked with build tag `smoke`, run manually against a real account via `go test -tags=smoke ./...` (out of CI). Smoke fixtures use mocked sample addresses (`alice@example.com`, etc.), never real session data.
- **Coverage target:** ≥80% line coverage on new files. Existing files keep their current coverage floor.
- **`go vet ./...`** and **`govulncheck ./...`** clean.
- **`python3 .github/scripts/verify-skill/verify_skill.py --dir library/productivity/superhuman/`** passes.
- **PII scrub gate** (U17) returns all-clear before push.

---

## Rollout and Post-Merge Monitoring

- **PR description** lists every command added and every flag introduced, plus the PII-scrub all-clear output.
- **CHANGELOG.md** entry (if the repo uses one) summarizes the user-visible changes and notes the schema bump.
- **First indicator something is wrong:** users reporting `auto-refresh: gmail=failed` consistently — likely refresh-token expiry. Mitigation: `auth login --chrome`.
- **Second indicator:** a command hanging for >5s on cache-only setups. Mitigation: `--no-refresh` flag, or the deferred minimum-interval TTL knob.
- **Validation window:** 1 week post-merge. Watch for issues opened against this repo mentioning `auto-refresh`, `bootstrap`, `awaiting-reply`, or `watch`.
- **Owner:** the CLI maintainer.
- **Rollback trigger:** if multiple users report v3 migration failures, revert the PR and ship the schema migration separately with more conservative gating.

---

## Alternative Approaches Considered

- **Multiple stacked PRs.** Considered. User explicitly chose single PR for this work. Trade-off: review burden higher but no cross-PR sequencing complexity. See Phase 0 question round.
- **Bootstrap-on-first-run only, no auto-refresh.** Rejected: silent staleness was the explicit failure mode the session called out. Granola's PR #571 already proves auto-refresh is the right shape; replicating the pattern is lower-risk than reinventing.
- **Gmail Pub/Sub push for `watch` mode.** Deferred. Adds GCP project setup, complicates auth, not portable to non-Google deployments. History-list polling is good enough for the agent reactive use case.
- **Native semantic search (LLM-based)** instead of Gmail-search passthrough. Rejected: the live session showed `messages.query` and `ai --query` are unreliable. Gmail's native search syntax is deterministic, well-documented, and what every consumer who has used Gmail expects.
- **Wrapping every existing read command's output in the envelope unconditionally.** Rejected: breaking change. Opt-in via `--envelope=full` lets agents adopt at their pace.

---

## Success Metrics

- A user (or agent) can answer "which threads need a follow-up reply this week?" in one command: `awaiting-reply --since 7d --external-only`.
- A user can draft a new outbound email in one command: `drafts new --to <e> --subject <s> --body <b>` (no hand-rolled JSON).
- `threads list --type sent` returns the expected results without errors.
- `doctor --json` clearly distinguishes `ok` from `expired_refresh_needs_relogin` states.
- The PII scrub gate returns clean before merge.
- All new tests pass; existing tests continue to pass.
