---
title: "fix(superhuman): browser-sniff reminder API and patch CLI payload + minimum"
created: 2026-05-15
type: fix
depth: lightweight
status: active
target_repo: printing-press-library
target_path: library/productivity/superhuman
related:
  - PR: https://github.com/mvanhorn/printing-press-library/pull/595 (feat/superhuman-overhaul, in flight)
  - Session: 2026-05-15 found bugs while sending donkey-test email
---

# fix(superhuman): browser-sniff reminder API and patch CLI payload + minimum

## Problem Frame

While sending a test email (`donkey` subject/body, mvh@esperlabs.ai) with a 5-minute no-reply reminder, three CLI defects surfaced:

1. `superhuman-pp-cli send --remind-in 5m` is rejected with `--remind-in must be at least 1h`, but the Superhuman web app accepts 5-minute reminders. The 1h floor is a CLI-side validation, not an API limit.
2. `superhuman-pp-cli reminders create --thread-id <id> --trigger-at <ts>` posts `{"thread_id": "...", "trigger_at": "..."}` (snake_case), and the Superhuman backend rejects it with `400 {"code":400,"error":"missing threadId"}`. The same shape exists in `reminders cancel` (sends `reminder_id`, `thread_id`).
3. `reminders create` with a correctly camelCased body via `--stdin` (verified by `--dry-run`) still returns `400 missing threadId` for a freshly-sent thread ID. Unknown root cause: API ingestion timing, missing required field, or a different ID format. The browser sniff resolves this.

Cause for 1 and 2 is visible in source. Cause for 3 is unknown until we capture the real web client's request. Without the sniff, we'd be guessing about whether the freshly-sent-thread case is a real API contract gap or just propagation timing.

## Scope

In scope:
- Run a browser sniff against `https://mail.superhuman.com` to capture the real wire shape of `POST /reminders/create` (and `cancel`) when triggered from the web app.
- Patch `library/productivity/superhuman/internal/cli/reminders_create.go` and `reminders_cancel.go` to send the captured body shape.
- Lower or remove the `--remind-in` floor in `library/productivity/superhuman/internal/cli/send.go` based on the sniffed `triggerAt` minimum (likely none, or 1 minute).
- Record the changes in `library/productivity/superhuman/.printing-press-patches.json` per published-library conventions.
- File a generator-side companion issue or PR against `cli-printing-press` if the snake_case body emission is template-shaped.

Out of scope:
- Re-architecting auth, the send command, or the reminder data model.
- Implementing native `--remind-in 5m` via the `send` compound command (the existing flow goes through `buildSendReminder` which uses `triggerAt` already — once the floor is lowered the send path likely works for sub-hour values too, but verifying that is part of testing, not redesign).
- Resolving the "freshly-sent thread returns 400" mystery beyond what the sniff teaches us. If the sniff shows it's an ingestion delay, that becomes either a retry/backoff helper or a documented limitation, decided during /ce-work.

### Deferred to Follow-Up Work

- A `send` workflow recipe that does `send → wait for ingestion → reminders create` as one command, if the sniff confirms ingestion is the blocker.
- Aligning `messages`, `drafts`, `threads`, and any other endpoints that may have the same snake_case generator bug — touched only if the sniff or a quick `grep '"<field>_id"' library/productivity/superhuman/internal/cli/*.go` shows real consumers fail in the same way.

## Success Criteria

- `superhuman-pp-cli reminders create --thread-id <existing-ingested-id> --trigger-at <iso>` succeeds against the real API (HTTP 200, reminder visible in Superhuman UI).
- `superhuman-pp-cli reminders cancel --reminder-id <id>` succeeds against the real API.
- `superhuman-pp-cli send --remind-in 5m --if-no-reply` returns nil from the CLI validation step (no `--remind-in must be at least 1h`) for any duration the sniff shows the API accepts.
- `.printing-press-patches.json` carries an entry naming the three changes, files touched, and the validated outcome.
- Either a generator fix is co-landed or a `cli-printing-press` issue exists describing the template defect with a pointer to this patch.

## Approach Overview

Three lanes, sequenced:

1. **Discover** what the real wire shape is via browser-sniff. The Network panel in the Superhuman web app is the only place we know it works — what the app sends when the user picks "Remind me" is the source of truth.
2. **Patch** the three confirmed local bugs (`--remind-in` floor, snake_case in `reminders_create`, snake_case in `reminders_cancel`) using the sniffed contract.
3. **Verify** end-to-end against the live API with a real account, then record the patches per published-library conventions.

The sniff is non-optional even if the snake_case fix looks obviously right — it also tells us what the minimum `triggerAt` window is (informs the `--remind-in` floor), whether `condition: "if-no-reply"` vs `condition: "always"` is the right shape (the CLI's existing `buildSendReminder` already emits this; need to confirm naming), and what the API does on a freshly-sent thread.

## Key Technical Decisions

- **Agent does the browser sniff** via the `claude-in-chrome` MCP tools, not a draft asking the user to open DevTools. The user's logged-in Superhuman session is already attached to Chrome; we drive it.
- **Patch the published library first** per the catalog repo's AGENTS.md rule ("A broken published CLI gets patched here first, regardless of root cause"). Mark each site with `// PATCH(...)` source comments and record in `.printing-press-patches.json`.
- **Generator-side fix in parallel only if the bug is multi-CLI.** A quick `grep '"\w*_id"' library/**/internal/cli/*_create.go` will tell us whether the snake_case body emission shows up in other published CLIs. If it does, file an issue against `cli-printing-press` linking back to this patch. If only superhuman shows it, treat as a one-off published-library repair.
- **No new test fixtures from the live API.** Replay the captured request as a test fixture so we don't need a logged-in account in CI.

## Implementation Units

### U1. Browser-sniff reminder API contract from Superhuman web

**Goal:** Capture the real wire shape of `POST /reminders/create` and `POST /reminders/cancel` as the Superhuman web app emits them, plus enough surrounding behavior to inform the `--remind-in` minimum and the freshly-sent-thread 400 question.

**Dependencies:** none

**Files:**
- `library/productivity/superhuman/.manuscripts/<run-id>/discovery/reminders-sniff-report.md` (new) — captured request/response pairs, header set, observed minimums, freshness-of-thread behavior

**Approach:**
- Use `mcp__claude-in-chrome__tabs_context_mcp` to find or open the Superhuman tab (`https://mail.superhuman.com`).
- Pick an existing inbox thread (not freshly-sent) and trigger "Remind me" → "In 5 minutes" via the web UI.
- Capture the network request to `/reminders/create` (or wherever the real endpoint is — confirm path) via `mcp__claude-in-chrome__read_network_requests`. Record:
  - Method, full URL, all `x-superhuman-*` headers, content-type, full request body
  - Response status and body
- Repeat with "In 1 minute" if the UI exposes it (or the smallest selectable interval) to find the real minimum.
- Repeat the same flow for "Remove reminder" / "Un-snooze" to capture `/reminders/cancel` shape.
- Replay the freshly-sent-thread case: send a test email via the web app, immediately try to add a reminder. Observe whether the web app waits, retries, or succeeds immediately.
- Write everything captured into the manuscripts file as the durable record. Plain markdown — request bodies as fenced JSON blocks, no narration of process.

**Patterns to follow:**
- Existing `.manuscripts/<run-id>/discovery/browser-sniff-report.md` from past printing-press runs (see `.manuscripts/20260512-165241/discovery/browser-sniff-report.md` in this CLI).
- Memory: "Agent does the browser sniff" — agent drives chrome MCP, does not draft user-action steps.

**Test scenarios:**
Test expectation: none — this unit produces a discovery artifact, not behavior. Verification is human-readable: the manuscript exists, has three captured exchanges (create, cancel, freshly-sent retry), and the request bodies are concrete JSON not paraphrased prose.

**Verification:**
- Manuscript file exists with the three captured exchanges.
- The captured `reminders/create` body explicitly shows whether keys are `threadId`/`triggerAt` (camelCase, confirming our hypothesis) or some other shape.
- The captured minimum reminder interval is documented (1 minute, 5 minutes, or whatever the UI allows).

---

### U2. Patch reminders_create + reminders_cancel payload to match API contract

**Goal:** Replace the snake_case body keys in `reminders_create.go` and `reminders_cancel.go` with whatever U1's sniff confirmed the API expects.

**Dependencies:** U1

**Files:**
- `library/productivity/superhuman/internal/cli/reminders_create.go` (modify lines 51-59 — the snake_case block)
- `library/productivity/superhuman/internal/cli/reminders_cancel.go` (modify analogous lines 51-59)
- `library/productivity/superhuman/internal/cli/reminders_create_test.go` (new — or extend existing test file if one exists)
- `library/productivity/superhuman/internal/cli/reminders_cancel_test.go` (new — or extend existing)

**Approach:**
- For `reminders_create.go`: change `body["thread_id"] = bodyThreadId` to the sniff-confirmed key (expected `threadId`); same for `trigger_at` → `triggerAt`. Add a `// PATCH(reminders-api-contract): API uses camelCase body keys, generator default of snake_case is wrong here` comment block above the changed lines so the next regen leaves it intact.
- For `reminders_cancel.go`: same change for `reminder_id` and `thread_id`.
- Update `Example:` strings on both commands if the help text leaks the old shape.
- Tests assert the final HTTP request body shape using a fake transport (existing pattern — look at `messages_get_test.go` or `drafts_discard_test.go` for the http-client mocking idiom used in this CLI).

**Patterns to follow:**
- Existing `// PATCH(...)` annotation convention from AGENTS.md and `library/payments/kalshi/` for what the comment + manifest entry should look like.
- HTTP-mock test idiom used in `internal/cli/messages_get_test.go` (or whichever existing test in this package mocks `c.Post`).

**Test scenarios:**
- `reminders create --thread-id abc --trigger-at 2026-05-16T00:00:00Z` results in a POST with body `{"threadId":"abc","triggerAt":"2026-05-16T00:00:00Z"}` (no snake_case keys present).
- `reminders create --stdin` with a camelCase body passes the body through unchanged (existing stdin path must still work).
- `reminders cancel --reminder-id xyz --thread-id abc` posts `{"reminderId":"xyz","threadId":"abc"}` (or whatever shape U1 confirmed for cancel).
- Required-flag validation still fires when `--thread-id` is omitted and not `--dry-run`.

**Verification:**
- All four tests pass under `go test ./internal/cli/...`.
- Manual smoke: against a non-freshly-sent inbox thread (one the API has ingested), `superhuman-pp-cli reminders create --thread-id <id> --trigger-at <future-iso>` returns HTTP 200 and the reminder appears in the Superhuman UI.

---

### U3. Lower or remove --remind-in floor in send.go

**Goal:** Replace the hard-coded 1-hour minimum on `--remind-in` with whatever the API actually accepts (per U1).

**Dependencies:** U1

**Files:**
- `library/productivity/superhuman/internal/cli/send.go` (modify lines 642-644 — the `if d < time.Hour` block)
- `library/productivity/superhuman/internal/cli/send_test.go` (extend with new sub-hour scenarios)

**Approach:**
- If U1 confirms the API has no minimum, remove the block entirely (the parse already rejects non-positive durations via `parseReminderDuration`).
- If U1 confirms a minimum exists (e.g. 1 minute), replace `time.Hour` with the real value and update the error message.
- Add `// PATCH(reminders-floor): generator default was 1h, real API minimum is <X>`.
- The `--remind-on` path on line 651 (`if !parsed.After(now)`) is already correct for absolute timestamps and needs no change.

**Patterns to follow:**
- The existing `parseReminderDuration` already supports `5m`, `30m`, `1h`, etc. via `time.ParseDuration`. No flag re-plumbing needed — just lift the artificial floor.

**Test scenarios:**
- `send --remind-in 5m` produces a `sendReminder` with `TriggerAt = now+5min` (no validation error).
- `send --remind-in 1m` succeeds if the API minimum is ≤1m, or returns the new minimum-bound error if not.
- `send --remind-in -5m` still errors via the existing parse-time check.
- `send --remind-in 5m --remind-on 2026-05-16T00:00:00Z` still errors as mutually exclusive.
- `send --remind-in 5m --if-no-reply` sets `condition = "if-no-reply"` (existing behavior, regression check).

**Verification:**
- `go test ./internal/cli/...` passes.
- Manual smoke: `superhuman-pp-cli send --to mvh@esperlabs.ai --subject test --body test --remind-in 5m --if-no-reply` sends the email and 5 minutes later the reminder fires in the Superhuman UI if no reply has arrived.

---

### U4. Record patches and file generator-side companion if multi-CLI

**Goal:** Record the U2 and U3 changes in `.printing-press-patches.json` per published-library conventions, and either co-land or file a generator-side fix if the snake_case bug is template-shaped.

**Dependencies:** U2, U3

**Files:**
- `library/productivity/superhuman/.printing-press-patches.json` (create if absent, else extend)

**Approach:**
- Check whether `.printing-press-patches.json` already exists in the superhuman directory. If not, create one with the schema from AGENTS.md (`schema_version: 1`, `applied_at`, `base_run_id`, `base_printing_press_version`, `patches: []`). Copy `base_run_id` and `base_printing_press_version` from the existing `.printing-press.json`.
- Add three patch entries: `reminders-create-payload-casing`, `reminders-cancel-payload-casing`, `send-remind-in-floor`. Each entry: short `summary`, `reason` (one sentence on why), `files: [...]`, optional `validated_outcome` (the manual smoke result).
- Run `grep -l '"\w\+_id"' library/**/internal/cli/*_create.go library/**/internal/cli/*_cancel.go` from the catalog repo root. If two or more other published CLIs show the same snake_case body emission, this is template-shaped → file an issue against `mvanhorn/cli-printing-press` describing the defect, the affected CLIs, and a pointer to this patch. If only superhuman shows it, treat as one-off and skip the upstream issue.
- For each patch entry that has an upstream issue, set the `upstream_issue` field on the entry.

**Patterns to follow:**
- `library/payments/kalshi/.printing-press-patches.json` (referenced by AGENTS.md as the worked example).

**Test scenarios:**
Test expectation: none — this unit produces a manifest file and (conditionally) an upstream issue, not runtime behavior.

**Verification:**
- `.printing-press-patches.json` exists, validates as JSON, lists three patches with `files` arrays matching what U2 and U3 actually touched.
- If filed, the `cli-printing-press` issue URL is in `upstream_issue` on at least the two casing-related patches.

---

## System-Wide Impact

- **Other published CLIs:** if the generator emits snake_case body keys as a default, U4's grep step is the canary. Other CLIs with `POST /<resource>/create` endpoints that take ID-shaped body fields are candidate victims. Not in scope to fix here, but worth surfacing in the upstream issue.
- **MCP server:** `library/productivity/superhuman/internal/mcp/tools.go` exposes a `reminders.create` tool. If it constructs the body itself rather than delegating to the CLI internals, it has the same bug. Sanity-check during U2 and patch in the same commit if so.
- **In-flight PR #595 (`feat/superhuman-overhaul`):** land these patches as additional commits on the existing `feat/superhuman-overhaul` branch and push to PR #595. User decision 2026-05-15: fold in rather than open a standalone fix PR since #595 has not merged yet. Commits should be scoped tightly (one per unit) so the reviewer can see them as a coherent followup block.

## Risks

- **Sniff captures the wrong endpoint.** Superhuman's web client may use a non-obvious gateway (e.g. `userdata.writeMessage` for sends, similar for reminders). If `/reminders/create` doesn't appear in Network when "Remind me" is clicked, walk back from the UI action and capture whatever POST actually fires — and rename the CLI's path if necessary. Mitigation: U1's manuscript records the real URL, not the assumed one.
- **Freshly-sent-thread 400 turns out to be unfixable from the CLI.** If U1 shows the web app also fails (or waits) for freshly-sent threads, the right CLI response is documented limitation + retry helper, not a code fix. Mitigation: scope explicitly excludes "fix the freshly-sent 400" — we only commit to documenting what U1 finds.
- **Patch comments get stripped on regen.** If the generator overwrites these files on the next regen, the `// PATCH(...)` annotations plus `.printing-press-patches.json` are how the next agent finds the customization. Mitigation: this is exactly the catalog repo's standard convention; we follow it.

## Verification Strategy

- Unit-level: tests under `internal/cli/` assert the wire shape of request bodies and the absence of the 1h validation.
- Integration: manual smoke against the live Superhuman API using `superhuman-pp-cli send` followed by `superhuman-pp-cli reminders create` on an ingested thread. Done by the implementing agent during /ce-work.
- Regression: existing `send_test.go` / reminder tests continue to pass.

## Open Questions Deferred to Implementation

- Exact field names: confirmed by U1's sniff, not by reading the API. The plan assumes `threadId`/`triggerAt`/`reminderId` but the sniff is authoritative.
- Whether `condition: "if-no-reply"` is the exact API value or whether it's named differently in the wire shape (e.g. `triggerOn: "no_reply"`). U1 captures it.
- The MCP server path's relationship to the CLI internals — settled by reading `internal/mcp/tools.go` during U2.
