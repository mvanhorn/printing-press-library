---
title: "fix(superhuman): follow-up to overhaul — autorefresh canonical parity, real snippets backend, live smoke evidence"
type: fix
status: active
created: 2026-05-15
depth: standard
target_repo: printing-press-library
target_package: library/productivity/superhuman
parent_plan: docs/plans/2026-05-15-001-feat-superhuman-cli-major-overhaul-plan.md
---

# fix(superhuman): follow-up to overhaul — autorefresh canonical parity, real snippets backend, live smoke evidence

## Summary

Close three audit-surfaced gaps in the superhuman-pp-cli overhaul branch (`feat/superhuman-overhaul`) before opening the PR:

1. **U4 autorefresh deviated from the canonical granola PR #571 pattern.** Codex implemented the dispatcher from plan description alone because the canonical file was not present in the worktree (granola PR #571 merged after `feat/superhuman` was cut). Result: 176 LOC vs granola's 346 LOC, missing `refreshPlan`/`refreshResult`/`refreshSurface` abstractions, missing typed `noRefreshCommands` map, half the test coverage. Works but breaks cross-CLI cognitive alignment — the next maintainer reading both will see two different shapes for the same concept.

2. **Snippets are local-only, not synced to Superhuman's actual Snippets folder.** Codex stored snippets at `~/.superhuman-pp-cli/snippets.json`, sidestepping the (undocumented) Superhuman backend snippets endpoint. The user's real snippets ("Esper Blurb", "scheduling", "Founder Dinner SF", etc., visible in the Superhuman UI sidebar) remain inaccessible from the CLI; anything created via CLI lives in a parallel local pool the UI never sees. Defensible coding rationale (don't invent undocumented APIs), but it fails the actual user workflow — the user wanted to USE their existing snippets from CLI, not maintain a duplicate set.

3. **Sent / starred / archived / spam / trash routing is untested against real Gmail.** The 474 passing tests are all against `httptest.Server` mocks. Routing code looks correct (`labelIDs=SENT|STARRED|SPAM|TRASH|IMPORTANT`, `q=in:anywhere -label:inbox` for done/archived), but whether a real Esper Labs account returns real data on each `--type` is unverified. A live smoke pass against the actual account would catch any wire-level mismatch and produces PR evidence reviewers will want.

All three land on the existing `feat/superhuman-overhaul` branch as fix-up commits before the PR opens. No new branch.

---

## Problem Frame

The parent plan (`docs/plans/2026-05-15-001-feat-superhuman-cli-major-overhaul-plan.md`) shipped 17 units, 62 files, +7281/-408 LOC, 474 passing tests, clean PII scrub. The audit pass surfaced three gaps that the plan's verification steps did not catch:

- **Autorefresh canonical drift.** The parent plan's U4 section explicitly referenced granola PR #571 as the template: "Mirror granola's autorefresh shape rather than inventing a new pattern." Codex could not access the file (branch base predates the merge) and proceeded from prose-level requirements only. The resulting implementation works but uses different type names, a different skip-list data structure, a slightly different provenance line format, and roughly half the test coverage.

- **Snippets behavioral mismatch.** The parent plan's U7 said: "Snippets in Superhuman are persistent user-created templates with a subject, body, and label. They are useful outside the send context..." This implied backend sync. Codex chose local-only storage, framed in the deviation log as: "the plan named snippets list/get/create/update/delete and send integration, but did not require a private Superhuman backend snippets endpoint." Both readings of the plan are defensible. The user's stated intent in this audit makes the backend-sync reading correct.

- **Unverified Gmail surface.** Mock-test green means request shape is correct against the mock. It does not prove SENT label routing returns SENT mail, that Gmail accepts `labelIds=SPAM`, or that `q=in:anywhere -label:inbox` is the right Gmail-search expression for "archived." These are checks only a live account can settle.

---

## Goals and Non-Goals

### Goals

- Bring superhuman's autorefresh dispatcher to structural parity with granola's autorefresh.go.
- Replace the local-only snippet store with Superhuman-backend-backed snippet sync, so CLI `snippets list` returns the same snippets the UI sidebar shows.
- Produce live-smoke evidence that each Gmail folder type (`sent`, `starred`, `archived`, `spam`, `trash`, `important`, `done`) returns real data from a real account.
- Land all three as fix-up commits on `feat/superhuman-overhaul` before opening the PR. PR body cites this plan and includes the smoke evidence.

### Non-Goals

- Rewriting any other unit from the parent plan. U1-U3, U5-U6, U8-U17 stand as-is.
- Pushing to origin or opening the PR before these fix-ups complete.
- Implementing a public Superhuman snippets API (we'll use whatever the web app uses, even if undocumented).
- Migrating existing snippet data — if the user has any local snippets from interim testing, document the manual migration in PR body; do not auto-migrate.

### Deferred to Follow-Up Work

- A `superhuman-pp-cli snippets sync` command for explicit re-pull from backend. The autorefresh hook covers it implicitly; explicit sync command is only useful if autorefresh misses a case.
- MCP server tool exposure for snippets (regenerates automatically from CLI on merge; no manual MCP edits needed).

---

## Key Technical Decisions

### D1. Use `git show origin/main:...` to read the canonical granola pattern

The granola autorefresh files exist on `origin/main` (commit `185f56b0`, PR #571) but not in the `feat/superhuman` worktree base. U1 reads them via `git show` rather than rebasing or merging main into the branch — this avoids pulling unrelated main-branch changes that would muddy the PR diff. U1 produces a working copy of the canonical files under `/tmp/granola-canonical/` (gitignored, not committed) that U2 ports from.

### D2. Surface labels stay short: "cache" and "gmail"

Granola uses "cache" (Granola desktop encrypted file) and "api" (public REST). Superhuman's two surfaces are "cache" (Superhuman backend writeMessage / userdata.read) and "gmail" (Gmail passthrough via users.history.list). Keeping "cache" gives one-word recognition for cross-CLI readers; renaming Superhuman's second surface from generic "api" to specific "gmail" tells the reader which underlying API at a glance.

### D3. Snippet backend discovery is a browser-sniff task, not a planning question

The Superhuman snippets endpoint isn't documented. The plan can't pin the URL or payload shape in advance. U3 explicitly runs as a discovery + implementation pair: open Chrome to the Snippets view, capture the Network tab, extract the endpoint + payload, then code against the captured shape. The plan documents the *workflow*; the actual endpoint is an implementation-time finding.

### D4. Snippet body preservation policy on first sync

When the user first runs `snippets list` against the new backend-backed implementation, any local-JSON snippets that exist at `~/.superhuman-pp-cli/snippets.json` are surfaced as a manual-migration prompt rather than auto-uploaded. Auto-upload risks duplicating snippets the user has already created in the UI. The CLI prints: "Local snippets found at <path>. To migrate, run `snippets create --name <n> --body <b>` for each." File stays in place so the user can review.

### D5. Live smoke runs from the user's machine, not CI

The smoke test exercises the real Esper Labs account (`mvh@esperlabs.ai`). Running it from CI would require leaking the user's auth token. The plan instead documents the smoke commands and asks the user to run them locally and paste the (PII-scrubbed) output into the PR body. U5 produces the command list and the scrub template.

---

## Implementation Units

### U1. Capture canonical granola autorefresh files

**Goal:** Produce a working copy of granola's autorefresh.go, sync_autorefresh.go, autorefresh_test.go, autorefresh_hook_test.go, and sync_autorefresh_test.go from `origin/main` for reference during U2.

**Requirements:** Closes Problem Frame #1 (foundation).

**Dependencies:** None.

**Files:** None committed. Working copies land under `/tmp/granola-canonical/` (gitignored).

**Approach:**
1. Run `git show origin/main:library/productivity/granola/internal/cli/autorefresh.go > /tmp/granola-canonical/autorefresh.go` and the same for the four sibling files.
2. Read all five files end-to-end.
3. Note: namespace, type names, skip-list contents, opt-out env var name, provenance line format, test coverage shape.

**Verification:** Files exist under `/tmp/granola-canonical/`, total LOC matches PR #571's stat block (346+161+303+187+88 = 1085 LOC across the five files). Reader can summarize the structural pieces (refreshPlan, refreshResult, refreshSurface, noRefreshCommands typed map, provenance line format) without re-reading.

**Test scenarios:** Not applicable — research-only unit.

---

### U2. Rewrite superhuman autorefresh to canonical parity

**Goal:** Replace `internal/cli/autorefresh.go` and `internal/cli/sync_autorefresh.go` (and their tests) with implementations that mirror the granola structure: typed `noRefreshCommands` map, `refreshSurface` constants, `refreshPlan` and `refreshResult` structs, parity provenance line format.

**Requirements:** Closes Problem Frame #1.

**Dependencies:** U1.

**Files:**
- `library/productivity/superhuman/internal/cli/autorefresh.go` — REPLACE
- `library/productivity/superhuman/internal/cli/sync_autorefresh.go` — REPLACE
- `library/productivity/superhuman/internal/cli/autorefresh_test.go` — REPLACE
- `library/productivity/superhuman/internal/cli/autorefresh_hook_test.go` — NEW (matches granola's split)
- `library/productivity/superhuman/internal/cli/sync_autorefresh_test.go` — REPLACE
- `library/productivity/superhuman/internal/cli/root.go` — update PersistentPreRunE wiring if the function signature shifted
- `library/productivity/superhuman/internal/cli/agent_context.go` — update the `auto_refresh` contract field shape if the surface naming changed

**Approach:**
1. Start from `/tmp/granola-canonical/autorefresh.go`. Rename:
   - Package stays `cli`
   - Env var: `GRANOLA_NO_AUTO_REFRESH` → `SUPERHUMAN_NO_AUTO_REFRESH` (already correct in codex's version; keep)
   - Surface labels: `refreshSurfaceCache` stays, `refreshSurfaceAPI` → `refreshSurfaceGmail`
   - Per-surface helpers: `runCacheSync` → `runSuperhumanBackendRefresh`, `runApiSync` → `runGmailHistoryRefresh`
2. Skip-list contents adapt to superhuman commands: `sync`, `auth`, `doctor`, `help`, `version`, `completion`, `agent-context`, `profile`, `feedback`, `which`. Same shape (typed `map[string]struct{}`).
3. Per-surface implementations replace granola's per-surface bodies:
   - `cache` surface: call into the existing superhuman backend sync path (the one driven by `sync.go` today, factored into a helper)
   - `gmail` surface: call `gmail.ListHistory(startHistoryId=history_state.last_history_id)` and apply deltas to the messages store (logic already in U3 of the parent plan)
4. Provenance line: byte-for-byte parity with granola's format where possible. Where the surface names differ ("gmail" vs "api"), keep the format string but substitute the surface label.
5. Tests: port granola's test scenarios scenario-by-scenario, adapting fixtures to superhuman's APIs. Match the LOC ballpark (490 LOC across the three test files).

**Patterns to follow:** Granola's autorefresh.go is the canonical reference. Match types, names, control flow, test fixtures shape.

**Execution note:** Implement test-first. Port one granola test at a time, watch it fail against the current codex implementation, then port the implementation hunk that makes it pass. This is the most reliable way to converge on parity without skipping subtle behaviors (e.g., the suppression matrix corners, the provenance-only-when-TTY branch).

**Test scenarios:**
- `noRefreshCommands` contains the documented superhuman skip list (sync, auth, doctor, help, version, completion, agent-context, profile, feedback, which) and no others.
- `shouldSkipAutoRefresh` returns true when a subcommand of a skip-list command runs (e.g., `auth login`).
- `shouldSkipAutoRefresh` returns false for non-skip commands.
- `--no-refresh` flag suppresses refresh; flag value beats profile and env.
- Profile `no-refresh: true` suppresses refresh; profile beats env when no flag set.
- `SUPERHUMAN_NO_AUTO_REFRESH=1` env suppresses refresh when neither flag nor profile set; truthy variants `true`, `1`, `yes` all parse.
- `detectRefreshPlan` returns empty plan when no auth surfaces configured; command proceeds with no provenance line.
- `detectRefreshPlan` returns cache-only when only Superhuman backend auth configured.
- `detectRefreshPlan` returns gmail-only when only Gmail auth configured.
- `detectRefreshPlan` returns both surfaces when both configured.
- Cache surface success emits `cache=ok (<duration>, <rows> rows)` fragment.
- Cache surface failure emits `cache=failed (<short-err>)` fragment.
- Gmail surface success emits `gmail=ok (<duration>, <rows> rows)` fragment.
- Gmail surface 401 emits `gmail=failed (run 'auth login --chrome' to re-authenticate)`.
- Gmail surface history-expired (HTTP 404) triggers fallback bootstrap; provenance line notes the fallback.
- Provenance line emits to stderr only when stderr is a TTY AND not under `--agent`/`--json`/`--compact`/`--quiet`.
- Provenance line suppressed under `--agent` flag.
- Provenance line suppressed when stderr is piped.
- `agent-context --json` includes the full `auto_refresh` contract: `default`, `flag`, `env`, `profile_field`, `surfaces=["cache","gmail"]`, `skip_list=[...]`.
- Best-effort: refresh failure does not block the original command. Original command exits with its own exit code.
- Best-effort: refresh failure emits one stderr warning line, no panic, no stack trace.

**Verification:** `wc -l` on the four new files lands within ~10% of granola's totals. `go test ./... -run TestAutoRefresh` passes. `go test ./...` passes. `superhuman-pp-cli agent-context --json | jq .auto_refresh` shape matches the granola equivalent (modulo the surface label difference).

---

### U3. Discover and capture the real Superhuman snippets endpoint

**Goal:** Identify the actual HTTP endpoint, request payload, and response shape the Superhuman web app uses for listing and creating snippets. Document the findings as an implementation note that U4 codes against.

**Requirements:** Closes Problem Frame #2 (discovery).

**Dependencies:** None — runs in parallel with U1.

**Files:** Findings recorded in `library/productivity/superhuman/docs/superhuman-snippets-discovery.md` (NEW, committed in U4).

**Approach:** The agent drives the sniff using the `claude-in-chrome` MCP tools — no manual user action required beyond ensuring Superhuman is signed in in Chrome.

1. `tabs_context_mcp` to discover open Chrome tabs; locate a Superhuman tab (`mail.superhuman.com`) or open a new one via `tabs_create_mcp` if none exists.
2. `navigate` to the Snippets folder URL (sidebar entry the user has open today, typically `mail.superhuman.com/...snippets` route).
3. `read_network_requests` with a Superhuman host filter to capture the list-snippets call as the view loads.
4. Drive the UI via `find` + `left_click` to:
   - Open one snippet for editing → capture the read call
   - Click "New snippet" / create a throwaway test snippet (`Test snippet from API sniff <timestamp>`) → capture the create call
   - Edit the test snippet → capture the update call
   - Delete the test snippet → capture the delete call
5. For each captured request, extract: method, URL path, request headers, request body shape, response status, response body shape.
6. Write `library/productivity/superhuman/docs/superhuman-snippets-discovery.md` with one section per endpoint.

**Patterns to follow:** The existing Superhuman backend client at `library/productivity/superhuman/internal/client/client.go` already handles `/v3/userdata.read`, `/v3/userdata.writeMessage`, `/v3/userdata.getThreads`, and `/v3/teams.suggest` — the snippets endpoint is almost certainly under `/v3/userdata.*`. Mirror the auth pattern (Bearer JWT) the rest of the client uses.

**Execution note:** Driven by agent browser tools, not the user. If the agent can't drive the UI (e.g., browser tier restricts clicks), fall back to `javascript_tool` for programmatic capture via `fetch()` overrides or `XMLHttpRequest` interception. Only escalate to "ask the user" if both paths fail.

**Test scenarios:** Not applicable — discovery-only unit.

**Verification:** `library/productivity/superhuman/docs/superhuman-snippets-discovery.md` exists with: list endpoint, get endpoint, create endpoint, update endpoint, delete endpoint — each carrying URL+method+request shape+response shape. Auth pattern noted.

---

### U4. Replace local-JSON snippet store with backend-backed implementation

**Goal:** Rewrite `internal/cli/snippets.go` to call the Superhuman backend (per U3's discovery) instead of reading/writing `~/.superhuman-pp-cli/snippets.json`. CLI `snippets list` returns the same snippets the UI sidebar shows.

**Requirements:** Closes Problem Frame #2 (implementation).

**Dependencies:** U3.

**Files:**
- `library/productivity/superhuman/internal/cli/snippets.go` — REPLACE
- `library/productivity/superhuman/internal/cli/snippets_test.go` — REPLACE
- `library/productivity/superhuman/internal/client/client.go` — extend with snippet helpers if the existing Superhuman backend client doesn't expose them
- `docs/plans/2026-05-15-002-superhuman-snippets-discovery.md` — NEW (committed here)
- `library/productivity/superhuman/SKILL.md` — UPDATE the Snippets section: snippets now sync with Superhuman UI; mention the local-snippet migration prompt
- `library/productivity/superhuman/README.md` — same UPDATE as SKILL.md

**Approach:**
1. Implement `snippets list / get / create / update / delete` against the U3-discovered endpoint using the existing Superhuman backend client (`internal/client/`).
2. Variable substitution (`--var key=value` substituting `{{key}}` in body) stays the same — it's a client-side transformation on top of the snippet body before sending.
3. Local-snippet migration: on first `snippets list` call, check for `~/.superhuman-pp-cli/snippets.json`. If present, print a one-time stderr hint suggesting manual migration via `snippets create`. File stays in place.
4. Remove the local-store code paths from snippets.go but keep the local file readable (for the migration hint) — no auto-deletion.

**Patterns to follow:**
- The existing Superhuman backend write path used by `drafts new` (U8 of parent plan) and `send` for the writeMessage payload structure.
- The typed error handling from `internal/client/client.go` (APIError, AuthError).

**Test scenarios:**
- `snippets list` against a mocked backend returns the parsed snippet array.
- `snippets get <name>` against a mocked backend returns the snippet body.
- `snippets create --name foo --body "Hi"` POSTs to the create endpoint with the discovered payload shape.
- `snippets update <name> --body "Hello"` PUTs/POSTs the update.
- `snippets delete <name>` calls the delete endpoint.
- `send --snippet foo --to a@b.com --subject test` resolves the snippet via backend `get` and uses the body.
- `send --snippet foo --var first_name=Alice` substitutes `{{first_name}}`.
- `send --snippet nonexistent` returns a "snippet not found" error from the backend, surfaced clearly.
- Migration hint: first call to `snippets list` after upgrade prints stderr migration message; subsequent calls do not.
- Migration hint: never auto-uploads, never deletes the local file.
- Backend 401 surfaces the standard auth hint (`Run 'auth login --chrome'`).

**Verification:** `superhuman-pp-cli snippets list --account mvh@esperlabs.ai --agent` returns at least one snippet matching the UI sidebar (verified by name match against the screenshot from the originating session — "Esper Blurb", "scheduling", "Founder Dinner SF Thurs May 7th 5:45 PM", "Esper Labs Seed Round / Close", "Esper Labs / Intro / Human Insight API"). Note: this verification is run locally by the user; the test scenarios above prove the wire shape but not real-account return.

---

### U5. Live smoke test of folder coverage + evidence capture for PR body

**Goal:** Confirm that every Gmail-routed `--type` (`sent`, `starred`, `archived`, `done`, `spam`, `trash`, `important`) actually returns real data when run against `mvh@esperlabs.ai`. Capture the (PII-scrubbed) output and add it to the PR body as evidence.

**Requirements:** Closes Problem Frame #3.

**Dependencies:** Branch must be in a buildable state. Best run after U2 + U4 land so the smoke evidence reflects the final state.

**Files:**
- `library/productivity/superhuman/docs/smoke-evidence-2026-05-15.md` — NEW (committed). Markdown doc with per-type smoke commands and scrubbed output samples.

**Approach:** Agent runs the smoke commands directly — `superhuman-pp-cli` on the agent's shell already has working auth for `mvh@esperlabs.ai` (verified via the parent overhaul session).

1. Agent runs the seven commands:
   ```
   superhuman-pp-cli threads list --type sent --limit 3 --json --account mvh@esperlabs.ai
   superhuman-pp-cli threads list --type starred --limit 3 --json --account mvh@esperlabs.ai
   superhuman-pp-cli threads list --type archived --limit 3 --json --account mvh@esperlabs.ai
   superhuman-pp-cli threads list --type done --limit 3 --json --account mvh@esperlabs.ai
   superhuman-pp-cli threads list --type spam --limit 3 --json --account mvh@esperlabs.ai
   superhuman-pp-cli threads list --type trash --limit 3 --json --account mvh@esperlabs.ai
   superhuman-pp-cli threads list --type important --limit 3 --json --account mvh@esperlabs.ai
   ```
2. For each, capture: exit code, response shape (no PII fields shown), thread count.
3. PII scrub: redact sender names, real email addresses, real subject lines from output before writing the evidence doc.
4. Document any failures with: HTTP status, error message, suspected cause, proposed fix-up.
5. If any failure surfaces a code bug, agent files a fix-up commit on the same branch before opening the PR.

**Execution note:** Agent runs the smoke, agent scrubs the output, agent writes the evidence doc. If auth tokens have expired since the parent overhaul session, agent runs `auth status` first and surfaces a clear `auth login --chrome` prompt rather than guessing.

**Test scenarios:**
- Each of the seven `--type` values returns exit 0.
- Each returns a non-empty thread array (or `[]` with no error if the folder is genuinely empty — confirmed against the UI).
- `--type done` and `--type archived` return overlapping or identical results (both target "in:anywhere -label:inbox"); document the relationship.
- `--type spam` and `--type trash` work even when the user's spam/trash folders are nearly empty.
- `--limit 3` is honored (no more than 3 threads returned).

**Verification:** Smoke evidence doc exists with all seven commands and their scrubbed outputs. PR body cites this doc.

---

### U6. Update PR body template with audit + fix-up summary

**Goal:** Refresh the PR description draft to reflect the parent plan + this follow-up plan + smoke evidence. Ready to paste when `gh pr create` runs.

**Requirements:** Closes the loop on the audit.

**Dependencies:** U2, U4, U5.

**Files:**
- `library/productivity/superhuman/docs/pr-body-draft-2026-05-15.md` — NEW (committed, but the file itself is meta — does not affect runtime behavior).

**Approach:**
1. Body summarizes: parent plan + this follow-up + commit count + test count + PII scrub all-clear + smoke evidence + audit notes (where codex deviated and how we fixed it).
2. Sections: Summary, What this PR adds, Test plan, Verification evidence (smoke), Plan references.
3. No process narrative (per `feedback_no_process_in_pr_body` memory): one-sentence AI-tool disclosure if needed; no review-round details; no findings list.

**Test scenarios:** Not applicable — documentation.

**Verification:** The draft is paste-ready; running `gh pr create --body-file <path>` would produce a complete PR description.

---

## Risk Analysis and Mitigation

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| U3 browser sniff turns up a snippets endpoint that requires WebSocket or SSE rather than plain HTTP | Low | Medium | If discovered, U4 still completes — adapt the client to whatever transport is needed. Existing CLI has SSE handling for `ai --query`. |
| U2 parity rewrite breaks tests U4-U17 already passed | Medium | Low | Run full suite after each commit in U2. Codex's existing test scenarios stay (additive); test count goes UP, not down. |
| Live smoke against real account hits rate limits | Low | Low | Only 7 read-only calls. Well within Gmail quota. |
| Snippets backend has different schema for "system snippets" (signature, knowledge base) vs user snippets | Medium | Low | U3 captures all snippet types in the discovery pass. U4 handles each via the discovered schema. |
| Local-JSON snippet migration confuses users who already created CLI snippets during U7 testing | Low | Low | Migration hint is stderr-only, non-blocking. Local file stays readable. |

---

## Success Metrics

- Codex's autorefresh.go grows from 176 LOC to ~340 LOC matching granola structure.
- `wc -l` on the autorefresh test files lands within 10% of granola's combined coverage.
- `snippets list` against `mvh@esperlabs.ai` returns at least 3 of the snippets visible in the user's UI sidebar (per the session screenshot: "Esper Blurb", "scheduling", "Founder Dinner SF", "Esper Labs Seed Round / Close", "Esper Labs / Intro / Human Insight API").
- All seven `threads list --type <gmail-folder>` commands return non-empty results (or empty with explicit confirmation against the UI).
- Smoke evidence doc exists and is referenced in the PR body.
- All existing tests still pass: `go test ./library/productivity/superhuman/...` green.
- PII scrub all-clear: zero matches against the known-PII token list after these fix-ups.

---

## Operational Notes

- The branch `feat/superhuman-overhaul` is local-only at plan-write time. Do not push to origin until U1-U6 complete and the smoke evidence is captured.
- The PR base is `feat/superhuman` (stacked PR — `feat/superhuman` is the initial-superhuman PR currently open).
- After all six units land, the PR description (from U6) gets pasted into `gh pr create --base feat/superhuman ...`.
- Local snippets at `~/.superhuman-pp-cli/snippets.json` are NOT touched by the fix-up commits. Migration is user-initiated post-merge.
