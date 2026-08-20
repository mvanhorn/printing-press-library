---
title: "feat: Auto-refresh on every CLI invocation (granola-pp-cli + skill)"
type: feat
status: completed
created: 2026-05-14
depth: standard
target_repo: printing-press-library
target_package: library/productivity/granola
---

# feat: Auto-refresh on every CLI invocation (granola-pp-cli + skill)

## Summary

Make `granola-pp-cli` re-hydrate its local data store as the first action of every command invocation, with no user opt-in required. The current model leaves freshness as a manual responsibility (`sync` first), which silently produces stale results when agents or humans skip the step. Auto-refresh closes that gap by treating "make data current" as a CLI-level invariant, not a per-command discipline.

Auto-refresh must work for both authentication paths the CLI supports:

1. **Desktop encrypted cache path** (`sync`, default) — read Granola desktop's encrypted cache file and upsert into the local SQLite store.
2. **Public REST API path** (`sync-api`, when `GRANOLA_API_KEY` is set) — fetch from `public-api.granola.ai` and upsert into the local store.

When both are available, both refresh routines fire. The refresh is best-effort: a failure prints a one-line stderr warning and the underlying command proceeds against whatever data is already in the store. A `--no-refresh` flag and `GRANOLA_NO_AUTO_REFRESH=1` env var provide a hard opt-out for tight loops and tests.

The companion `pp-granola` agent skill is updated in lockstep: it drops the "run sync first" advisories that no longer apply, documents the new default in the Auth Setup and Direct Use sections, and tells agents not to dispatch `sync` manually before other commands.

---

## Problem Frame

**Current behavior.** `granola-pp-cli meetings list` reads only the local SQLite store. Its help text says outright: *"Reads from the synced SQLite store. Run 'granola-pp-cli sync' first."* The `--data-source auto` global flag is misleading for this subcommand — it's local-only regardless. The skill instructs agents to `sync` first, but compliance is inconsistent: every conversation that forgets returns stale meeting lists, panel reads against missing IDs, and follow-up extraction that silently misses today's recordings. The author hit this today in a live session: a weekly retrospective query returned 23 meetings sourced from whatever SQLite had cached at last manual sync, with no indication that data was stale.

**Why the manual model is wrong.** Granola's freshness chain has two stages (servers → desktop encrypted cache → CLI SQLite). The CLI controls the second hop. Forcing every command to call `sync` first is a process workaround for a missing system invariant. Agents are the dominant caller of this CLI; they are the worst kind of caller to depend on for ceremonial pre-steps because they don't share session state across invocations and don't remember what they should have run.

**Why not raise the staleness via flags only.** The existing `--data-source live` flag matters only on commands that have a live path (e.g., `panel get`). `meetings list` is local-only; no flag value changes that. Fixing only that one command pushes the problem around without removing the manual-sync requirement.

**What "auto-refresh" means here.** Re-hydrate the local store from whichever upstream sources the CLI has credentials for. It does **not** mean nudging Granola desktop to pull from servers (that was explicitly considered and deferred — see Scope Boundaries). The freshness ceiling is whatever Granola desktop has already pulled into its encrypted cache, plus whatever the public REST API returns.

---

## Goals & Non-Goals

### Goals

- Every CLI invocation re-hydrates the local store as its first data action, before the requested command runs.
- Auto-refresh fires for both auth paths (desktop encrypted cache; public API key) and runs both when both are configured.
- Refresh failures never block the requested command. Stale data with a one-line warning beats a broken CLI.
- A `--no-refresh` flag and `GRANOLA_NO_AUTO_REFRESH=1` env var give power users and CI a clean opt-out.
- Commands that don't read data (`auth`, `doctor`, `help`, `version`, `completion`, `agent-context`, `profile`, `feedback`, `which`, `sync`, `sync-api`) skip auto-refresh entirely.
- Provenance is visible: stderr shows one line summarizing what refreshed and how long it took. Suppressed under `--agent`, `--quiet`, and `--json` so machine consumers don't see chatter on stdout-or-stderr ambiguity. Stdout stays pure JSON.
- The companion `pp-granola` SKILL.md is updated in the same PR; "run sync first" guidance is removed, and the new contract is documented.

### Non-Goals

- Nudging Granola desktop to refresh from Granola servers (AppleScript path). Considered, declined for v1 — macOS-only, GUI-dependent, adds seconds per call, doesn't compose with headless agents.
- A staleness TTL or stale-while-revalidate cache. User asked for "every single time" as the explicit default. A minimum-interval env knob is listed as a deferred enhancement, not a v1 feature.
- Auto-refresh for the `granola-pp-mcp` server. MCP per-tool refresh has different ergonomics (long-running server, per-tool latency budget). Deferred to follow-up.
- Changing the existing `sync` or `sync-api` command surfaces. The hook reuses their core logic; their user-facing commands remain.
- Changing the `--data-source` flag semantics. The flag continues to mean what it means today on commands that have a live path.

---

## Key Technical Decisions

**Hook location: `PersistentPreRunE` in `internal/cli/root.go`.** Cobra's pre-run hook is the only point that fires for every command exactly once before its `RunE`. It already handles deliver-spec parsing, profile application, and `--agent` expansion. Auto-refresh is appended after agent-mode expansion (so `--no-refresh` from a profile, env, or flag is honored) and after data-source validation (which is unchanged). Origin reference: `internal/cli/root.go:130-184`.

**Skip-list approach: command-name allowlist, evaluated at hook entry.** Walk `cmd.Name()` and the names of its ancestors up to root; if any segment is in the no-refresh list, skip. Names rather than annotations because (a) the list is short and stable, (b) annotations require touching every skipped command, (c) Cobra's `Hidden` flag is unrelated. Skip list: `sync`, `sync-api`, `auth` and any subcommand, `doctor`, `help`, `version`, `completion`, `agent-context`, `profile` and any subcommand, `feedback` and any subcommand, `which`. Rationale per entry is documented inline in code.

**Dual-path detection.** Build a small `refreshPlan` struct at hook entry that contains two booleans: `runCacheSync` and `runApiSync`. `runCacheSync` is true when the encrypted cache file exists at its expected path AND a usable token source is detected (reuses logic from `doctor_encrypted_store.go`). `runApiSync` is true when `GRANOLA_API_KEY` is set (or whichever env vars `sync-api` recognizes today). When both are true, run both — they hydrate different rows and don't conflict. When neither is true, auto-refresh is a silent no-op (no warning; this is a legitimate "auth not yet configured" state and the underlying command should produce its own auth error).

**Failure mode: best-effort with stderr warning.** Refresh errors are wrapped, logged to stderr in human-friendly mode, and otherwise dropped. The requested command always proceeds. Rationale: a flaky network or a transient Keychain hiccup should not break read-only data exploration. If the user's command itself needs live data, it surfaces its own error.

**Opt-out surface: `--no-refresh` + `GRANOLA_NO_AUTO_REFRESH=1` + profile-aware.** A persistent flag covers ad-hoc invocations. An env var covers shells, agents, and CI. A `no_refresh` profile field covers Beacon-style scheduled callers. Precedence (highest first): explicit flag → profile → env var → default (refresh on).

**Provenance line goes to stderr only.** Format: `auto-refresh: cache=ok (1.2s, 47 docs)  api=skipped (no GRANOLA_API_KEY)`. Stdout stays pure JSON for agent piping. Suppressed under `--agent`, `--quiet`, `--json`, and when stderr is not a TTY (so CI logs don't fill with refresh lines).

**No new lock primitive.** The existing sync code path already serializes inside `granola.SyncFromCache` and `sync-api` against the SQLite store. Two concurrent CLI invocations are rare in practice; if they collide, sqlite's WAL handles it. Revisit if Beacon-style high-concurrency callers emerge.

---

## High-Level Technical Design

This illustrates the intended approach and is directional guidance for review, not implementation specification. The implementing agent should treat it as context, not code to reproduce.

### Control flow at hook entry

```
PersistentPreRunE(cmd, args):
  ... existing deliver/profile/agent-mode setup ...
  ... existing data-source validation ...

  if shouldSkipAutoRefresh(cmd) { return nil }
  if optedOut(flags, env, profile)  { return nil }

  plan := detectRefreshPlan()
  if plan.empty() { return nil }     # no auth surfaces present; silent no-op

  start := now()
  results := plan.run(ctx)            # best-effort; collects errors but never returns them
  emitProvenanceLine(stderr, results, elapsed=now()-start)
  return nil                          # always nil; command proceeds regardless
```

### Refresh dispatch shape

| Auth surface present | Action |
|---|---|
| Encrypted cache only | Call shared `runCacheSync(ctx)` (extracted from `newSyncCacheCmd`'s `RunE`) |
| `GRANOLA_API_KEY` only | Call shared `runApiSync(ctx)` (extracted from `newSyncCmd`'s `RunE`) |
| Both | Call both, sequentially (cache first, API second; cache is faster) |
| Neither | No-op, no warning |

### Skip-list decision

| Command path | Refreshes? | Reason |
|---|---|---|
| `meetings list`, `panel get`, `attendee timeline`, `folder stream`, etc. | yes | These read data and benefit from freshness |
| `sync`, `sync-api` | no | Recursion / pointless |
| `auth login`, `auth status`, `auth set-token`, `auth logout` | no | Cannot refresh without auth |
| `doctor` | no | Doctor's job is to report *current* state, not change it |
| `help`, `version`, `completion`, `agent-context`, `which` | no | No data dependency |
| `profile *`, `feedback *` | no | Config/local operations |

### Opt-out matrix

| Source | Wins over | Loses to |
|---|---|---|
| `--no-refresh` flag | profile, env, default | — |
| `no_refresh: true` in profile | env, default | flag |
| `GRANOLA_NO_AUTO_REFRESH=1` | default | flag, profile |
| Default (auto-refresh on) | — | all above |

---

## Output Structure

No new directories. All changes land in existing paths:

```
library/productivity/granola/
├── internal/cli/
│   ├── autorefresh.go             # NEW: hook + plan detection + dispatch
│   ├── autorefresh_test.go        # NEW: unit + integration tests
│   ├── root.go                    # MODIFY: invoke autorefresh in PersistentPreRunE; add --no-refresh flag
│   ├── sync_cache.go              # MODIFY: extract RunE body into exported runCacheSync(ctx) so the hook can reuse it
│   ├── sync.go                    # MODIFY: same extraction for sync-api's core
│   └── profile.go                 # MODIFY: add no_refresh field
├── SKILL.md                       # MODIFY: drop "run sync first" advisories; document auto-refresh contract
├── README.md                      # MODIFY: same as SKILL.md
└── .printing-press-patches.json   # MODIFY: record the SKILL.md edits per repo convention
```

---

## Implementation Units

### U1. Extract reusable sync cores from `sync` and `sync-api` commands

**Goal:** Make the cache-sync and api-sync logic callable from a non-command context (the auto-refresh hook) without going through Cobra's `cmd.RunE`.

**Dependencies:** none.

**Files:**
- `library/productivity/granola/internal/cli/sync_cache.go`
- `library/productivity/granola/internal/cli/sync.go`
- `library/productivity/granola/internal/cli/sync_cache_test.go` (extend if present, otherwise add)
- `library/productivity/granola/internal/cli/sync_test.go` (extend if present, otherwise add)

**Approach:**
- In `sync_cache.go`, factor the body of `newSyncCacheCmd`'s `RunE` into an exported function (package-internal) `runCacheSync(ctx, flags) (CacheSyncResult, error)`. The `RunE` becomes a thin wrapper that calls it and formats output. `CacheSyncResult` carries doc count, duration, and decrypt status — fields the provenance line and existing JSON output both need.
- Same refactor for `sync.go` and the api path: `runApiSync(ctx, flags) (ApiSyncResult, error)`.
- No behavior change to existing commands. Output, JSON shape, exit codes, and Keychain prompts must be preserved verbatim.

**Patterns to follow:** the existing `newSyncCacheCmd` already builds a `granola.SyncState` struct and calls `WriteSyncState`. Keep that pattern inside the extracted function so auto-refresh writes sync state the same way manual sync does.

**Test scenarios:**
- `runCacheSync` returns success with non-zero doc count when the encrypted cache is present and decryptable. Result struct's `Count` matches what `SyncFromCache` returned. `SyncState` is written with `LastSyncAt` updated.
- `runCacheSync` returns the existing `ErrRefreshRefused` sentinel unchanged when the token source is `TokenSourceEncryptedSupabase` and a refresh is attempted (regression guard for the D6 read-only invariant in patch 41).
- `runCacheSync` returns a wrapped error when the encrypted cache decrypt fails; `recordSyncDecryptStatus` is still called.
- `runApiSync` returns success when `GRANOLA_API_KEY` is set and the public API is reachable (mock the HTTP layer).
- `runApiSync` returns a "no auth" error when `GRANOLA_API_KEY` is unset.
- Existing `sync` and `sync-api` end-to-end output is byte-identical before and after the refactor (snapshot test if one exists; otherwise a targeted comparison).

**Verification:** `go test ./internal/cli/...` passes; running `granola-pp-cli sync` and `granola-pp-cli sync-api` against a populated dev environment produces the same stdout, stderr, and exit codes as before.

---

### U2. Add `autorefresh.go` with plan detection and dispatch

**Goal:** Centralize the auto-refresh logic in one file so the hook in `root.go` stays minimal and the logic is unit-testable in isolation.

**Dependencies:** U1.

**Files:**
- `library/productivity/granola/internal/cli/autorefresh.go` (new)
- `library/productivity/granola/internal/cli/autorefresh_test.go` (new)

**Approach:**
- Define `type refreshPlan struct { cache, api bool }` and a `detectRefreshPlan(flags) refreshPlan` function that:
  - Sets `cache = true` if the encrypted cache file exists at the expected path AND a token source is detected (reuse the helper already used by `doctor_encrypted_store.go`; do not duplicate).
  - Sets `api = true` if `GRANOLA_API_KEY` (or whatever the existing api auth detection uses; mirror it exactly) is set.
- Define `type refreshResult struct { surface string; ok bool; count int; duration time.Duration; err error }` and `func (p refreshPlan) run(ctx) []refreshResult` that calls `runCacheSync` and/or `runApiSync` in order, collecting results without returning errors. Each failure becomes a `refreshResult{ ok: false, err: ... }`.
- Define `shouldSkipAutoRefresh(cmd *cobra.Command) bool` that walks ancestors and matches against the skip-list constants in the same file. List the constants explicitly (string slice) so the test can lock them down.
- Define `emitProvenanceLine(w io.Writer, results []refreshResult, total time.Duration)` that formats and writes the single stderr line.

**Technical design (directional):**

```
// pseudo-code, not implementation
package cli

var noRefreshCommands = []string{
  "sync", "sync-api", "auth", "doctor", "help", "version",
  "completion", "agent-context", "profile", "feedback", "which",
}

func runAutoRefresh(cmd, flags) {
  if shouldSkipAutoRefresh(cmd) { return }
  if optedOut(flags) { return }
  plan := detectRefreshPlan(flags)
  if !plan.cache && !plan.api { return }
  results := plan.run(cmd.Context())
  if shouldEmitProvenance(flags) { emitProvenanceLine(stderr, results) }
}
```

**Patterns to follow:** Other small helpers like `ParseDeliverSink` in `deliver.go` and `GetProfile` in `profile.go` — single-purpose files, exported types where the hook needs them, no Cobra dependencies inside the helper functions themselves.

**Test scenarios:**
- `shouldSkipAutoRefresh` returns true for each command name in the skip list, plus deep subcommands like `auth login`, `profile save`, `feedback list`.
- `shouldSkipAutoRefresh` returns false for representative data commands: `meetings list`, `panel get`, `attendee timeline`, `folder stream`, `recipes coverage`, `talktime`, `calendar overlay`, `export`, `notes-show`, `transcript`.
- `detectRefreshPlan` with no auth surfaces returns `{cache: false, api: false}`.
- `detectRefreshPlan` with encrypted cache file present and token source set returns `{cache: true, api: false}` when `GRANOLA_API_KEY` is empty.
- `detectRefreshPlan` with `GRANOLA_API_KEY` set and no encrypted cache returns `{cache: false, api: true}`.
- `detectRefreshPlan` with both auth surfaces returns `{cache: true, api: true}`.
- `refreshPlan.run` with `cache=true, api=true` calls `runCacheSync` first, then `runApiSync`, even when the first errors. Results slice has two entries with the correct `surface` labels.
- `refreshPlan.run` returns a non-empty results slice on error — error is captured in the result struct, not returned from `run`. Confirms best-effort contract.
- `emitProvenanceLine` writes the expected format to its writer and writes nothing when results is empty.

**Verification:** unit tests above pass; the file compiles without importing Cobra at the helper-function level (only the dispatcher function takes `*cobra.Command`).

---

### U3. Wire the hook into `PersistentPreRunE` and add opt-out surfaces

**Goal:** Make auto-refresh actually fire at the right moment in the existing cobra setup, with all three opt-out paths working.

**Dependencies:** U1, U2.

**Files:**
- `library/productivity/granola/internal/cli/root.go`
- `library/productivity/granola/internal/cli/profile.go`
- `library/productivity/granola/internal/cli/root_test.go` (extend if present, otherwise add)

**Approach:**
- Add a persistent flag in `newRootCmd`: `--no-refresh` (bool, default false), bound to `flags.noRefresh`. Help text: `"Skip the auto-refresh that runs before every command"`.
- At the end of `PersistentPreRunE` (after data-source validation), call `runAutoRefresh(cmd, flags)`. This is one line.
- Add `NoRefresh` field to the profile struct in `profile.go` and wire it into `ApplyProfileToFlags` so saved profiles can disable refresh per-profile.
- Implement `optedOut(flags) bool` precedence: explicit `--no-refresh` flag wins; then `flags.noRefresh` from profile; then `os.Getenv("GRANOLA_NO_AUTO_REFRESH")` is one of `1`, `true`, `yes` (matching the project's existing env-boolean convention — check how `GRANOLA_FEEDBACK_AUTO_SEND` is parsed and mirror it).
- Update `agent_context.go` so the agent-context JSON exposes `auto_refresh: { default: "on", flag: "--no-refresh", env: "GRANOLA_NO_AUTO_REFRESH", profile_field: "no_refresh" }` so introspecting agents discover the contract.

**Patterns to follow:** the existing `--agent` flag wiring in `PersistentPreRunE` (lines 157-174) is the precedent for "expand flags into other flag state inside the pre-run hook." Do not invoke `runAutoRefresh` until *after* that block — `--no-refresh` may itself be set by a profile via `ApplyProfileToFlags`.

**Test scenarios:**
- `granola-pp-cli meetings list --no-refresh` does not invoke auto-refresh (mock the dispatcher; assert call count is zero).
- `GRANOLA_NO_AUTO_REFRESH=1 granola-pp-cli meetings list` does not invoke auto-refresh.
- A profile with `no_refresh: true` applied via `--profile foo` does not invoke auto-refresh.
- `granola-pp-cli meetings list` (no flag, no env, no profile) does invoke auto-refresh.
- `--no-refresh=false` on the CLI overrides a profile that has `no_refresh: true` (flag wins precedence test).
- Running `granola-pp-cli sync` does not recursively invoke auto-refresh (skip-list integration test).
- Running `granola-pp-cli auth status` does not invoke auto-refresh.
- `granola-pp-cli --help` does not invoke auto-refresh.
- A simulated `runCacheSync` failure does not cause the requested command to fail — `meetings list` still returns its result with exit 0.
- `agent-context` JSON output includes the auto-refresh capability descriptor.

**Verification:** `go test ./internal/cli/...` passes; manual smoke test of the matrix above against a populated dev environment.

---

### U4. Provenance line, quiet-mode suppression, and TTY detection

**Goal:** Make the auto-refresh visible to interactive users without polluting agent pipes, JSON output, or CI logs.

**Dependencies:** U2, U3.

**Files:**
- `library/productivity/granola/internal/cli/autorefresh.go` (extend)
- `library/productivity/granola/internal/cli/autorefresh_test.go` (extend)
- `library/productivity/granola/internal/cli/helpers.go` (if a TTY helper exists, reuse it)

**Approach:**
- `shouldEmitProvenance(flags) bool` returns false when any of the following are true: `flags.asJSON`, `flags.compact`, `flags.quiet`, `flags.agent`, stderr is not a TTY. Returns true otherwise.
- Provenance line format: `auto-refresh: cache=<status> (<duration>, <count> docs)  api=<status> (<duration>, <count> docs)`. Statuses: `ok`, `skipped`, `failed: <short-reason>`. Skip the api fragment entirely when api wasn't in the plan (don't print `api=skipped` if user has no API key — too noisy). Duration shown as `1.2s` or `230ms`, not seconds-with-six-decimals.
- Write to `cmd.ErrOrStderr()` rather than `os.Stderr` directly so tests can capture it.

**Patterns to follow:** the response envelope in the `Agent Mode` section of SKILL.md describes the "summary on stderr only when stdout is a terminal" rule for live/local provenance lines. Apply the same rule here.

**Test scenarios:**
- Provenance line is emitted on a TTY in default mode.
- Provenance line is suppressed under `--agent`.
- Provenance line is suppressed under `--json`.
- Provenance line is suppressed under `--quiet`.
- Provenance line is suppressed under `--compact`.
- Provenance line is suppressed when stderr is piped (non-TTY).
- Provenance line for cache-only plan omits the api fragment.
- Provenance line for api-only plan omits the cache fragment.
- Failure status renders as `cache=failed: keychain timeout` with the wrapped error's short reason, not a full stack.
- Duration formatting: 1.234s renders as `1.2s`; 230ms renders as `230ms`; 50ms renders as `50ms`.

**Verification:** unit tests above pass; manual smoke test confirms agent pipelines see no extra stderr chatter and interactive users see a tidy one-liner.

---

### U5. Update SKILL.md, README.md, and patches manifest

**Goal:** Document the new contract so agents and humans understand it without reading code, and lock the changes into the repo's patch-tracked SKILL workflow.

**Dependencies:** U1-U4 (so the documentation matches shipped behavior).

**Files:**
- `library/productivity/granola/SKILL.md`
- `library/productivity/granola/README.md`
- `library/productivity/granola/.printing-press-patches.json`
- `.claude/skills/pp-granola/SKILL.md` (the distributed copy installed in the user's `~/.claude/skills/` — note the absolute path is outside the repo; this is referenced for parity but updated via the existing skill-install pipeline, not edited directly in the repo)

**Approach:**
- In SKILL.md and README.md, drop the standalone "Run `granola-pp-cli sync` first" advisories under `meetings list` and similar local-only commands. Replace with a short note that data is auto-refreshed.
- Add a new top-level subsection under "Agent Mode" or just after it titled **"Auto-Refresh"** that documents:
  - The default: every command refreshes the local store as its first action.
  - The two auth paths that get refreshed (encrypted cache, `GRANOLA_API_KEY`).
  - The freshness ceiling (we don't pull from Granola servers; the desktop app controls that hop).
  - The three opt-out mechanisms (`--no-refresh`, `GRANOLA_NO_AUTO_REFRESH=1`, profile `no_refresh: true`).
  - The provenance line and when it's emitted.
- In the existing "When Not to Use This CLI" section, add nothing — this is purely a freshness-discipline change, not a capability change.
- In the existing "Recipes" section, scrub any recipe that explicitly chains `sync && command`. Replace with the direct command.
- Add a patch entry to `.printing-press-patches.json` documenting the SKILL.md rewrite, in the same format as patches 41 and 70 already present. Summary line states what changed and why.

**Patterns to follow:** patches 41 and 70 in `.printing-press-patches.json` are the precedent for SKILL.md mutations tracked in patches. Mirror their `summary` / `reason` structure.

**Test scenarios:** `Test expectation: none -- documentation-only changes; the skill-install pipeline's existing render/verify step catches malformed patches.`

**Verification:** run the existing patch verification step (`workflow-verify-report.json` is the artifact; the make target is in `Makefile`). Confirm the rendered SKILL.md still parses through the install pipeline. Confirm `npx -y @mvanhorn/printing-press install granola --cli-only` re-renders the user-facing skill correctly (do this against a scratch directory, not the user's real `~/.claude/skills`).

---

## Scope Boundaries

### In scope

- All CLI commands except the explicit skip list (see Key Technical Decisions).
- Both auth paths: encrypted desktop cache and `GRANOLA_API_KEY`.
- The opt-out flag, env var, and profile field.
- The provenance line and its suppression rules.
- SKILL.md and README.md updates.

### Out of scope

- `granola-pp-mcp` auto-refresh hook. The MCP server has its own latency budget per tool call; treat as separate follow-up plan.
- Nudging Granola desktop to refresh from Granola servers (AppleScript path).
- A staleness TTL or stale-while-revalidate behavior. User asked for "every single time" as default.
- Changing the semantics of `--data-source auto | live | local`. That flag continues to work as before on commands that have a live path.

### Deferred to Follow-Up Work

- **MCP server auto-refresh.** Same idea, different harness. Track separately so the CLI ships first and the MCP doesn't gate on it.
- **Minimum-interval env knob (`GRANOLA_AUTO_REFRESH_MIN_INTERVAL=2s`).** If a Beacon-style high-frequency agent emerges, add a "don't refresh if last refresh was within N seconds" floor. Not needed for v1; current refresh duration is small enough that "every single time" is fine.
- **Desktop nudge as opt-in deep refresh.** Add `--refresh=deep` and `GRANOLA_DEEP_REFRESH=1` once there's evidence users need it. Requires AppleScript path and a probe for whether Granola is running.
- **Concurrency lock.** If two parallel invocations start producing visible problems beyond what SQLite's WAL handles, add a per-process file lock under `~/.granola-pp-cli/`. Punt until evidence shows up.

---

## System-Wide Impact

| Surface | Impact |
|---|---|
| `granola-pp-cli` binary | Every command (except skip list) gains a pre-run refresh step. Typical added latency: tens to a few hundred ms for cache sync; ~1-3s for api sync when present. |
| `granola-pp-mcp` binary | No change in v1. Same package; the hook lives in CLI-only code. |
| `pp-granola` agent skill | Drops "run sync first" affordances. New auto-refresh section documents the contract. |
| Existing scripts that pre-pend `granola-pp-cli sync &&` | Continue to work but become redundant. The redundant `sync` is a no-op refresh in effect. Document this in SKILL.md so users can clean up but aren't forced to. |
| CI pipelines | Likely want `GRANOLA_NO_AUTO_REFRESH=1` set globally to keep CI runs deterministic against fixture data. Add this to README in a "CI usage" line. |
| Local SQLite store | Hit at the start of every command instead of once per session. Existing SQLite WAL handles repeated upsert load. |
| Granola desktop app | Unchanged. Auto-refresh only reads the encrypted cache file; it does not talk to the app. |
| Granola public REST API | Hit on every command when `GRANOLA_API_KEY` is set. Confirm the API's rate limit accommodates per-command calls; if rate limiting becomes a problem, the deferred minimum-interval knob is the answer. |

---

## Verification

- All unit tests above pass.
- Manual smoke test against a real Granola desktop install:
  - `granola-pp-cli meetings list --last 24h` shows the same meetings as `granola-pp-cli sync && granola-pp-cli meetings list --last 24h` did before the change. Provenance line appears on stderr.
  - `granola-pp-cli meetings list --no-refresh --last 24h` produces no provenance line and runs measurably faster (within a few ms of stale-only baseline).
  - `granola-pp-cli sync` does not produce a provenance line (skip list works) and behaves identically to today.
  - `granola-pp-cli --help` does not produce a provenance line.
  - `granola-pp-cli doctor` runs against current state without auto-refresh interfering with its diagnostic output.
- `granola-pp-cli agent-context --json` exposes the auto_refresh contract object.
- The `pp-granola` SKILL.md, after running through the install pipeline into a scratch directory, contains the new Auto-Refresh section and has no remaining "run sync first" advisories.

---

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Refresh adds latency to commands that don't need it. | Skip list covers the obvious cases. Add the minimum-interval knob if real evidence emerges. |
| Refresh fails silently and user sees stale data with no warning. | Provenance line on stderr by default in interactive mode shows failure status. `doctor` continues to be the source of truth for sync status. |
| Agents that pipe stderr-to-stdout pick up the provenance line as data. | Provenance is suppressed under `--agent`, `--json`, `--quiet`, `--compact`, and non-TTY stderr. Document in SKILL.md. |
| `--no-refresh` is forgotten in long-running CI jobs and hits the public API on every command. | Default to suggesting `GRANOLA_NO_AUTO_REFRESH=1` for CI in README's CI section. |
| Existing scripts that pre-pend `sync &&` now sync twice. | Cosmetic at worst; the second sync is fast and idempotent. Note in changelog. |
| Keychain prompt fires on first auto-refresh and confuses users. | First-run prompt is unchanged from today's manual-sync first-run. Doctor already documents the behavior. |
| The encrypted cache check turns out to be too expensive to run on every command. | The check is a file stat plus a token-source read. If profiling shows it's a problem, cache the result for the lifetime of the process. |

---

## Origin

Solo invocation (no upstream `*-requirements.md`). Planning context surfaced today from a live debug session: `granola-pp-cli meetings list` returned stale results during a weekly retrospective; debug revealed the manual-sync requirement; user requested an auto-refresh design covering both CLI and skill, working for both the encrypted-cache and public-API-key auth paths.
