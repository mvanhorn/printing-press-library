---
title: "fix: contact-goat usability bugs and PP install freshness for all CLIs"
type: fix
status: active
date: 2026-04-19
---

# fix: contact-goat usability bugs and PP install freshness for all CLIs

## Overview

A real-user session on 2026-04-19 surfaced several defects in contact-goat beyond the Clerk /touch JWT-in-body regression fixed in #92/#93. This plan captures the remaining fixes. It also extends to the generator so every Printing Press CLI's install instructions stop quietly handing users stale post-merge binaries from the Go module proxy cache.

## Problem Frame

Real failure modes seen in a single session on freshly-installed `contact-goat-pp-cli`:

1. `coverage <company> --source li` hard-errors with `search_people ... limit: Unexpected keyword argument`. The MCP tool has no `limit` arg; contact-goat passes one anyway. 100% failure whenever the LinkedIn side is engaged.
2. `coverage <company>` relies on Happenstance's graph search, which routinely takes 2-5 minutes. The default poll timeout is 60s and `coverage` exposes no knob, so users see frequent "Thinking about who you're looking for" timeouts even on valid queries.
3. `hp people` has the same 60s default.
4. Post-merge installs via the documented `go install ...@latest` silently pull stale code for hours. This session confirmed an install of `@latest` right after a merge pulled a commit dated 8 days earlier. The documented install path doesn't guarantee freshness.

The tool quality bar: these failures should be invisible to a typical user running the documented commands.

## Requirements Trace

- R1. `contact-goat coverage <co> --source li` completes end-to-end when the user is logged in to the LinkedIn MCP.
- R2. `contact-goat coverage <co> --source hp` can complete Happenstance searches that take up to 3 minutes without requiring a manual flag.
- R3. `coverage` exposes a `--poll-timeout` knob consistent with `hp people --timeout`, propagated to the Happenstance client.
- R4. Default Happenstance poll timeout is raised from 60s to 180s across both `coverage` and `hp people` so the out-of-box experience matches real upstream latency.
- R5. Every PP CLI's install instructions give a user a command that reliably installs latest main, not a stale proxy-cache pseudo-version.

## Scope Boundaries

- Non-goal: Speed up Happenstance's backend when a search legitimately takes more than 3 minutes. The upstream is what it is; we only adjust client timeouts and expose a flag.
- Non-goal: Rework the LinkedIn MCP subprocess integration beyond removing the invalid `limit` kwarg.
- Non-goal: Change `coverage`'s dedup/ranking behavior, `prospect`, `warm-intro`, or any other cross-source command.
- Non-goal: Touch Deepline wiring or rate-limit defaults.
- Non-goal: Rewrite `tools/generate-skills` logic — just the template and a one-line change in `main.go` if needed for metadata symmetry.

### Deferred to Separate Tasks

- Integration test that actually talks to a live Happenstance test account: needs fixture cookies and a shared test user. Separate workstream.
- `auth login --chrome` UX for non-macOS (Linux/Windows) — tracked separately in the existing roadmap note in `auth_chrome.go`.
- `doctor` reporting `FAIL Auth: not configured` even when Chrome-cookie auth is actually healthy: misleading but not regression-caused by this session. Separate polish PR.

## Context & Research

### Relevant Code and Patterns

- `library/sales-and-crm/contact-goat/internal/cli/flagship_helpers.go:188-213` — `fetchLinkedInSearchPeople` is the single call site that sends `"limit": limit` to the `search_people` MCP tool. Same file has `runLIToolWithLimit` pattern (linkedin.go:79) which already does client-side truncation on the returned JSON array — reuse that pattern.
- `library/sales-and-crm/contact-goat/internal/cli/coverage.go:83,111,173` — `coverage` builds a flag set, calls `SearchPeopleByCompany`, then `fetchLinkedInSearchPeople(ctx, company, "", 25)`. Both call sites hardcode the limit; `SearchPeopleByCompany` uses `defaultSearchOptions()` with no timeout override.
- `library/sales-and-crm/contact-goat/internal/client/people_search.go:59-67,273-275` — `defaultSearchOptions()` returns `PollTimeout: 60 * time.Second`. `SearchPeopleByCompany` takes no options arg, so the default is the only path today; it needs a sibling or a new options-taking overload.
- `library/sales-and-crm/contact-goat/internal/cli/hp_people.go:108` — `cmd.Flags().IntVar(&timeoutSec, "timeout", 60, ...)` — existing pattern to mirror on coverage.
- `library/sales-and-crm/contact-goat/internal/cli/auth_chrome.go:59-72` — already respects `--no-input` and `--yes` for the consent prompt. No change required; confirms the root `--agent` flow works.
- `tools/generate-skills/skill-template.md:26-29,58-61` and `tools/generate-skills/main.go:532,549` — the two places the `@latest` install command is emitted. Both need matching guidance.
- `plugin/.claude-plugin/plugin.json` — AGENTS.md rule: SKILL content changes require manual version bump because `maybeUpdatePluginVersion` only auto-bumps on directory-set changes.

### Institutional Learnings

- The existing memory `feedback_pp_go_install_goprivate.md` captures the sumdb 404 trap. This plan extends that guidance into a public doc so every user benefits, not just the one person who read the memory.
- PR #91 shipped a real fix (/tokens -> /touch) on top of a sniff that wrongly claimed `__session` came back via Set-Cookie. Regression landed because there was no capture-vs-implementation diff in CI. This plan does not add that CI check (out of scope), but #93 already corrected the sniff doc and doctor message so the next contributor has the right source of truth.

### External References

- Go modules docs on `GOPROXY=direct` vs `GOPRIVATE` resolution semantics: `GOPRIVATE` implies both `GONOPROXY` and `GONOSUMCHECK` for matching paths, so a `@latest` install under `GOPRIVATE` does resolve by `git ls-remote` and honors the default branch's HEAD. Users without `GOPRIVATE` still go through `proxy.golang.org`, which caches `@latest` independently of branch HEAD. This asymmetry is the root cause of R5's install-freshness problem.

## Key Technical Decisions

- **Drop `limit` from the MCP call entirely; client-side truncate.** The `search_people` MCP tool does not accept `limit`. Passing it errors out. Other LinkedIn subcommands (e.g. inbox, company-posts) do accept `limit` on their respective tools, so the fix is localized to `fetchLinkedInSearchPeople`. Rationale over alternatives below.
- **Coverage gets its own `--poll-timeout` flag, name-consistent with `hp people`'s `--timeout`.** A single name would be cleaner, but `--timeout` is already taken on the root for HTTP call timeouts in many places (see `runLIToolInternal`). Using `--poll-timeout` for the Happenstance-specific knob avoids shadowing.
- **Raise default `PollTimeout` to 180s (3 min) in `defaultSearchOptions()`.** Session evidence: Happenstance routinely takes 2-5 min. 180s covers the median case without dragging a fast failure out to the full upper bound. Users with slower queries can pass `--timeout 300` or `--poll-timeout 300`. Agents running with `--no-input` still get a bounded wait.
- **Install docs: keep `@latest` as the primary install, add a "freshness fallback" section pointing at `@main`.** Most users installing a PP CLI for the first time are fine with `@latest`; the proxy cache catches up. Changing the default to `@main` would cause every install to do a `git ls-remote` against GitHub, which is fine for us but surprises users who read go-install conventions. A documented fallback is the least-surprise path.
- **Regenerate all 21 `plugin/skills/pp-*/SKILL.md` in one commit and bump `plugin.json` once.** AGENTS.md calls this out explicitly: SKILL content changes need a manual version bump.

## Open Questions

### Resolved During Planning

- Should `auth login --chrome` auto-yes under `--no-input`? Resolved: already does (auth_chrome.go:61). Not a bug.
- Should we touch `prospect` or `warm-intro` too? Resolved: scope limited to `coverage` and `hp people` per the "smallest blast radius that fixes what this session saw" principle. Other commands don't hit the 60s ceiling in practice (they either compose these calls or use different Happenstance endpoints).
- Should `--poll-timeout 0` disable the timeout entirely? Resolved: no. Treat 0 as "use default". Unbounded polls are an agent-footgun. Users who want >5 min should pass the explicit number.

### Deferred to Implementation

- Exact flag name for coverage timeout (`--poll-timeout` vs `--search-timeout` vs reuse `--timeout`): default to `--poll-timeout` per decision above, but switch if the implementer finds an existing coverage-local `--timeout` collision.
- Whether to also bump `PollInterval` default from 1s to 2s to cut poll volume on longer waits: evaluate during implementation. If bumped, update help text.
- Whether the LinkedIn MCP needs a doctor check that `search_people` is callable without `limit`: add if trivial; skip if it requires spawning the subprocess every doctor run.

## Implementation Units

- [ ] **Unit 1: Drop invalid `limit` kwarg from `search_people` call; client-side truncate**

**Goal:** Make `coverage --source li` (and any other caller of `fetchLinkedInSearchPeople`) succeed end-to-end.

**Requirements:** R1

**Dependencies:** None

**Files:**
- Modify: `library/sales-and-crm/contact-goat/internal/cli/flagship_helpers.go`
- Test: `library/sales-and-crm/contact-goat/internal/cli/flagship_helpers_test.go` (create if missing)

**Approach:**
- Remove the `if limit > 0 { args["limit"] = limit }` block from `fetchLinkedInSearchPeople`.
- Apply the `limit` as a client-side truncation of the parsed `[]flagshipPerson` slice before returning (mirror `runLIToolWithLimit`'s pattern in `linkedin.go:379-397`).
- Keep the signature unchanged so callers (`coverage.go:111`, `prospect.go:77`) keep working without touching them.
- Defensive: if the MCP ever adds server-side `limit` later, a separate, smaller PR can re-introduce it behind a capability probe.

**Patterns to follow:**
- `runLIToolInternal` + `applyClientSideLimit` helper paradigm already in `linkedin.go`.

**Test scenarios:**
- Happy path: `fetchLinkedInSearchPeople(ctx, "Disney", "", 5)` returns no more than 5 results when the parsed payload has more, and does not include `"limit"` in the MCP args map.
- Edge case: `limit == 0` returns the full parsed list unmodified.
- Regression: assert the MCP args passed to `client.CallTool` contain only `keywords` (and `location` when non-empty) — no `limit` key. A mock MCP client or args-building refactor enables this assertion without spawning a real subprocess.
- Integration: `coverage --source li --limit 3` end-to-end returns a result object with `count <= 3` and empty `source_errors.li_search` (when the user has an active LinkedIn profile).

**Verification:**
- `go test ./...` green.
- Manual: `contact-goat-pp-cli coverage Disney --source li --limit 5 --agent` exits 0 with a populated results array.

- [ ] **Unit 2: Add `--poll-timeout` flag to `coverage`; thread through to Happenstance client**

**Goal:** Give `coverage` users the same explicit-timeout escape hatch `hp people` already has.

**Requirements:** R3

**Dependencies:** Unit 3 (the options struct change that lets callers override `PollTimeout`)

**Files:**
- Modify: `library/sales-and-crm/contact-goat/internal/cli/coverage.go`
- Modify: `library/sales-and-crm/contact-goat/internal/client/people_search.go` (add `SearchPeopleByCompanyWithOptions` or accept `*SearchPeopleOptions` on the existing method)
- Test: `library/sales-and-crm/contact-goat/internal/cli/coverage_test.go` (create if missing)

**Approach:**
- Add `cmd.Flags().IntVar(&pollTimeoutSec, "poll-timeout", 0, "Seconds to wait for Happenstance to return results (0 = use default 180s)")` to the coverage command builder, mirroring `hp_people.go:108`.
- Expose a variant of `SearchPeopleByCompany` that takes `*SearchPeopleOptions`, so the CLI can pass a non-nil `PollTimeout`. Preferred: change `SearchPeopleByCompany(company string)` to accept a trailing variadic `opts ...func(*SearchPeopleOptions)`, or add `SearchPeopleByCompanyWithOptions(company string, opts *SearchPeopleOptions)` and keep the zero-arg wrapper calling through it.
- In `coverage.go:83`, pass through the user's `--poll-timeout` when non-zero.
- Update help text / examples to include `--poll-timeout`.

**Patterns to follow:**
- `hp_people.go`'s flag plumbing into `SearchPeopleOptions`.

**Test scenarios:**
- Happy path: `coverage X --poll-timeout 240` results in the client receiving `PollTimeout == 240s`.
- Happy path: `coverage X` (no flag) results in `PollTimeout == 180s` (the new default, validated in Unit 3).
- Edge case: `--poll-timeout 0` falls back to the client default.
- Edge case: `--poll-timeout -5` is rejected with a clear usage error (or clamped to default — decide during implementation; document the choice).

**Verification:**
- `coverage X --poll-timeout 180` does not time out on queries that used to fail at 60s.
- Help text shows the new flag.

- [ ] **Unit 3: Raise default `PollTimeout` from 60s to 180s**

**Goal:** Match real upstream latency so the out-of-box path works without flags.

**Requirements:** R2, R4

**Dependencies:** None

**Files:**
- Modify: `library/sales-and-crm/contact-goat/internal/client/people_search.go`
- Modify: `library/sales-and-crm/contact-goat/internal/cli/hp_people.go` (bump the flag default and help text)
- Test: `library/sales-and-crm/contact-goat/internal/client/people_search_test.go` (create if missing)

**Approach:**
- Change `defaultSearchOptions()` `PollTimeout` from `60 * time.Second` to `180 * time.Second`.
- In `hp_people.go`, change `IntVar(&timeoutSec, "timeout", 60, ...)` to `180` and update the help text to reflect the new default.
- Optional: bump `PollInterval` from `1s` to `2s` to halve poll volume on longer waits. Keep out of scope if it changes UX perception (`hp people` currently prints nothing between polls; doubling the interval is invisible to the user). Implementer's call.

**Patterns to follow:**
- Minimal constant change — no new abstraction.

**Test scenarios:**
- Unit: `defaultSearchOptions().PollTimeout` equals `180 * time.Second`.
- Unit: `SearchPeopleByQuery` with `opts == nil` uses 180s.
- Unit: `hp_people`'s default flag value is 180.
- Regression: passing `opts.PollTimeout == 0` still collapses to the default via the existing zero-value guard at `people_search.go:233-236`.

**Verification:**
- Existing `go test ./library/sales-and-crm/contact-goat/...` continues to pass.
- `contact-goat-pp-cli hp people --help` shows `(default 180)`.

- [ ] **Unit 4: Install docs — document the `@latest` proxy-cache caveat and the `@main` fallback in the generator template**

**Goal:** Give every PP CLI user a documented, reliable way to get fresh code immediately after a merge.

**Requirements:** R5

**Dependencies:** None

**Files:**
- Modify: `tools/generate-skills/skill-template.md`
- Modify (regenerated output): `plugin/skills/pp-*/SKILL.md` — all 21 CLIs
- Modify: `library/sales-and-crm/contact-goat/README.md` (sync the user-facing install section)
- Modify: `plugin/.claude-plugin/plugin.json` (manual patch-version bump per AGENTS.md)

**Approach:**
- In `skill-template.md`, add a short "If install seems stale" paragraph directly after the primary `@latest` install command. It should name the exact failure signature (old code from proxy cache) and give the one-line `@main` fallback with the `GOPRIVATE` hint.
- The `metadata.openclaw.install` block in `main.go:532,549` stays on `@latest` (it's the primary install). The template prose is where the caveat lives.
- Run `go run ./tools/generate-skills/main.go` to regenerate all 21 `plugin/skills/pp-*/SKILL.md` files.
- Manually bump `plugin/.claude-plugin/plugin.json` `version` by one patch step.

**Patterns to follow:**
- `AGENTS.md` — "Keeping plugin/skills in sync" section dictates the exact regen + version-bump ritual.
- `tools/generate-skills/main_test.go` for template-rendering tests.

**Test scenarios:**
- Template: rendering with a sample CLI context emits both the primary `@latest` command and the `@main` fallback.
- Generator: running `go run ./tools/generate-skills/main.go` produces a clean diff limited to the install sections across all 21 `plugin/skills/pp-*/SKILL.md` files.
- Test expectation: no behavior change in generated CLIs; diff is doc-only.

**Verification:**
- All 21 SKILL.md files contain the same `@main` fallback paragraph.
- `plugin.json` version is bumped exactly once.
- `go test ./tools/generate-skills/...` green.

- [ ] **Unit 5: README sync for contact-goat**

**Goal:** Keep `library/sales-and-crm/contact-goat/README.md` in lockstep with the SKILL.md install changes.

**Requirements:** R5

**Dependencies:** Unit 4

**Files:**
- Modify: `library/sales-and-crm/contact-goat/README.md`

**Approach:**
- Append the `@main` fallback paragraph under the existing `go install ...@latest` line.
- Also: the existing `@latest` line in the README (at line 11) references `/library/contact-goat/cmd/...` instead of the correct `/library/sales-and-crm/contact-goat/cmd/...` — fix the path so copy-paste installs actually work.

**Patterns to follow:**
- Match the exact wording used in the generator template (Unit 4) so the two docs stay parallel.

**Test scenarios:**
- Test expectation: none — pure docs change.

**Verification:**
- `grep 'library/contact-goat' library/sales-and-crm/contact-goat/README.md` returns nothing (the broken path is gone).
- The install paragraph matches the generator template's paragraph verbatim.

## System-Wide Impact

- **Interaction graph:** `coverage` calls `SearchPeopleByCompany` (client) and `fetchLinkedInSearchPeople` (MCP subprocess). `prospect` and `warm-intro` also call `fetchLinkedInSearchPeople`. Unit 1's signature-preserving fix means those callers inherit the fix for free without edits.
- **Error propagation:** The LinkedIn `limit` bug currently surfaces as `source_errors.li_search` in coverage JSON and a stderr warning. After Unit 1, successes appear in the results array; no new error shapes. Poll-timeout changes keep the existing `happenstance poll: timeout after ...` error message but make it rarer.
- **State lifecycle risks:** None — no persistent state involved.
- **API surface parity:** `SearchPeopleByCompany` gets an options overload. Existing callers keep the zero-arg form. No breaking change.
- **Integration coverage:** Unit 1's assertion that the MCP arg map lacks `limit` is the key cross-layer test — unit tests alone can't prove it without a mock MCP client (or a pure args-builder function). Prefer the pure-function extraction so the test stays fast and hermetic.
- **Unchanged invariants:** `hp people`'s flag name (`--timeout`), `coverage`'s existing ranking/dedup, the Happenstance auth flow (PR #92), and all non-`search_people` LinkedIn tools keep working exactly as before.

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| The `@main` fallback in docs surprises users who expect `@latest` semantics (e.g. tagged releases). | Frame it explicitly as "for immediate post-merge freshness"; keep `@latest` as primary. Mention this in the template paragraph. |
| Raising the default poll to 180s makes legitimate failures feel slower. | Users can pass `--timeout 60` to restore the prior behavior; error message is unchanged. The tradeoff favors the common case (searches actually taking 2-5 min). |
| Regenerating 21 SKILL.md files creates a large diff that's easy to mis-review. | Split Unit 4 into two commits: "add template paragraph" (small diff) + "regen + version bump" (mechanical diff, reviewer can trust the generator). |
| The args-builder refactor in Unit 1 may require nudging `fetchLinkedInSearchPeople`'s testability. | Keep the refactor minimal: extract one pure function `searchPeopleArgs(keywords, location string) map[string]any`. Anything more is out of scope. |
| Unit 4 `plugin.json` version collides with an in-flight PR that also bumps the version. | Rebase on main immediately before pushing; AGENTS.md calls out this coordination requirement. |

## Documentation / Operational Notes

- Release note entry under contact-goat: "Fixed `coverage --source li` 100% failure; raised Happenstance poll default to 3 min; added `--poll-timeout` on `coverage`."
- Release note entry at the plugin level: "Install instructions now include a fresh-code fallback for immediately-post-merge installs."
- No migrations, feature flags, or rollout steps — all changes are backward-compatible.

## Sources & References

- This-session context: a single live session on 2026-04-19 that exercised `hp people` and `coverage` against Happenstance + LinkedIn.
- Related PRs shipped earlier in this session: [#92](https://github.com/mvanhorn/printing-press-library/pull/92) (Clerk /touch JWT-in-body), [#93](https://github.com/mvanhorn/printing-press-library/pull/93) (sniff doc + doctor message).
- Related code:
  - `library/sales-and-crm/contact-goat/internal/cli/flagship_helpers.go:188-213`
  - `library/sales-and-crm/contact-goat/internal/cli/coverage.go:83,111,173`
  - `library/sales-and-crm/contact-goat/internal/client/people_search.go:59-67,273-275`
  - `library/sales-and-crm/contact-goat/internal/cli/hp_people.go:108`
  - `tools/generate-skills/skill-template.md:26-29,58-61`
- Repo conventions: `AGENTS.md` — "Keeping plugin/skills in sync" and "Commit style".
