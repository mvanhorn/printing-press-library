---
title: "fix: linear CLI agent-friendly issues list and single-issue get"
type: fix
status: active
date: 2026-04-20
---

# fix: linear CLI agent-friendly issues list and single-issue get

## Overview

Replace the broken promoted `issues <id>` path and add a first-class `issues list` command with common filter flags, so agents and humans can find and inspect Linear issues without falling back to `sql`, `today`, or raw `analytics` queries. Also tighten the error-classification path that currently returns misleading messages when the GraphQL endpoint rejects GET requests.

## Problem Frame

The generator shipped `linear-pp-cli` with two root causes of friction:

1. The promoted `issues <id>` command calls `resolveRead` against path `/graphql` via HTTP GET. Linear's GraphQL endpoint rejects non-POST requests as CSRF (observed `HTTP 400` with body `This operation has been blocked as a potential Cross-Site Request Forgery`). The CLI already has a working GraphQL POST path (`client.Query`), but the promoted command does not use it.

2. There is no `issues list` command at all. Users typing the first instinct command `linear-pp-cli issues list --assignee me` get `unknown command "issue"`, then `unknown flag: --assignee`, and eventually have to discover the workaround (`today`, `workload`, or raw `sql`). This forces multiple wrong attempts before a valid command is found.

A third friction comes from the fallback path. When `--data-source auto` catches a live failure, it passes the literal string `graphql` as an ID into the local store, yielding `resource "issues" with ID "graphql" not found in local store`, which is confusing and does not suggest the right next step (`sync` + `issues list`).

Evidence from this session:

- `linear-pp-cli issues ESP-1155` returns HTTP 400 CSRF.
- `linear-pp-cli doctor` already shows `Credentials: ok (HTTP 400 from base URL, but auth was accepted)`, confirming the systemic issue.
- `linear-pp-cli today` works because it uses the local sqlite store and a tiny `client.QueryInto` call to fetch `viewer { id }` via POST. The pattern is right there.

## Requirements Trace

- R1. `linear-pp-cli issues <ID>` returns the issue for `<ID>` against the live API via GraphQL POST, without CSRF errors.
- R2. `linear-pp-cli issues list` returns issues from the local store with filter flags for `--assignee` (including the shorthand `me`), `--state`, `--team`, `--project`, and `--limit`.
- R3. When the live API fails or when `--data-source local` is set, `issues get` falls back to the local store by identifier, with a clear error when the store has not been synced.
- R4. Error classification converts the raw CSRF/GraphQL GET failure and the `ID "graphql"` fallback error into actionable messages that name the right next step.
- R5. `SKILL.md`, `README.md`, and the plugin skill mirror reflect the new commands so the ppl router and agents surface them.

## Scope Boundaries

- Not changing mutation commands (create/update/delete) for issues.
- Not adding new analytics or aggregation views (`load`, `bottleneck`, `stale`, `workload` stay as-is).
- Not introducing an interactive picker or fuzzy selector.
- Not touching other promoted resources that hit the same GraphQL GET bug (projects, teams, cycles, etc.) in this plan. See deferred section.

### Deferred to Separate Tasks

- Generator-level fix so future GraphQL CLIs emit POST-based promoted commands: separate PR in `cli-printing-press`. This plan targets the already-shipped pp-linear CLI.
- Fixing the same GET-against-GraphQL bug for other promoted resources in pp-linear (`projects`, `teams`, `cycles`, etc.): follow-up PR in this repo, tracked once this one lands and the pattern is proven.

## Context and Research

### Relevant Code and Patterns

- `library/project-management/linear/internal/cli/promoted_issues.go` is the broken get command. Generated file with `DO NOT EDIT` header.
- `library/project-management/linear/internal/cli/today.go` is the working precedent: uses `flags.newClient()`, calls `client.QueryInto` with a GraphQL query, and reads from `store.ListIssues(filter, limit)`.
- `library/project-management/linear/internal/client/graphql.go` exposes `Client.Query(query, variables)` and `Client.QueryInto(query, variables, dest)`, both of which POST to `/graphql` with the correct content-type.
- `library/project-management/linear/internal/store/` exposes `Store.SearchIssues(query)` for FTS5 and `Store.ListIssues(filter, limit)` taking a `map[string]string` of column filters (for example, `assignee_id`). The `issues` table already has `identifier`, `title`, plus a `data` JSON blob.
- `library/project-management/linear/internal/cli/helpers.go` defines `classifyAPIError`. New patterns should extend this, not bypass it.
- `library/project-management/linear/internal/cli/data_source.go` defines `resolveRead`. The confusing `ID "graphql"` surface comes from here when live fails and the resource lookup uses the path as an ID.
- `library/project-management/linear/internal/cli/root.go` line 158 registers the promoted issues command. This is the single wire point to swap.

### Institutional Learnings

- From `AGENTS.md`: "For hand-editing published CLIs, prefer `printing-press patch` (AST-injects small changes) over re-running `printing-press generate`." This plan is a hand-edit, not a regeneration. The `promoted_issues.go` file carries `DO NOT EDIT` but it has already been shown to be broken, so we replace, not edit.
- From `AGENTS.md`: any change to `library/**/SKILL.md` or `library/**/internal/cli/**` requires running `go run ./tools/generate-skills/main.go` and manually bumping `plugin/.claude-plugin/plugin.json` version (semver patch). The generator does not auto-bump on content changes.
- The verifier `.github/scripts/verify-skill/verify_skill.py` checks every `--flag` in `SKILL.md` against declared flags in `internal/cli/*.go`, and checks positional args against cobra `Use:`. New flags must be declared on the cobra command before SKILL.md references them, or CI fails.

### External References

- None required for this change. The fix is grounded entirely in existing code patterns and the Linear GraphQL API's standard POST requirement, which is already honored in `today.go` and `me.go`.

## Key Technical Decisions

- Replace the promoted `issues` command in `root.go`, rather than hand-editing the `DO NOT EDIT` generated file. A new file `issues.go` owns both `issues <ID>` and `issues list`. This keeps the hand-fix isolated from generator output, making future regenerations safer: if the generator re-emits `promoted_issues.go` we simply ignore it at the wire point.
  - Rationale: smaller blast radius, aligns with the "prefer patch over regenerate" guidance in AGENTS.md.
- `issues list` is local-store backed, not live. Linear's GraphQL list endpoint is available, but users will have already synced (many commands require it), and the local path is faster, filterable via SQL, and does not burn API quota. Mirrors the precedent set by `today`, `stale`, and `workload`.
  - Rationale: agents typically need many fast list calls; the sync discipline is already established.
- `issues <ID>` tries local first when the store is fresh, then live GraphQL POST, then falls back to local-by-identifier on live failure. The order differs from other reads because this is the primary pain point and agents will already have synced.
  - Rationale: fast path for the common case, with live as the freshness escape hatch.
- The `--assignee me` shorthand resolves via a single `viewer { id }` GraphQL query and a process-local cache for the session. Do not persist viewer id to disk; `me` command already demonstrates the in-session pattern.
  - Rationale: avoids stale viewer identity after token rotation.
- Error classification for `CSRF` bodies and `ID "graphql"` fallbacks is added to `classifyAPIError` and `resolveRead`, not to the new command, so the improvement benefits every other affected promoted command (even though fixing those commands is out of scope here).
  - Rationale: one-line fixes with CLI-wide payoff.

## Open Questions

### Resolved During Planning

- Should `issues list` go live instead of local-backed? Resolved: local-backed, matches precedent and is faster.
- Should we delete the generated `promoted_issues.go`? Resolved: yes, delete it. Leaving a dead function that references broken paths invites future regressions if anyone re-wires it.
- Should `--assignee me` require sync? Resolved: no. The shorthand resolves via a live `viewer` query, so it works on a fresh machine as long as auth is set. Filtering still requires prior sync.

### Deferred to Implementation

- Exact SQL shape for multi-filter composition in `store.ListIssues`. The current signature takes `map[string]string`. If the existing implementation cannot express "state type != completed" or "team key = ESP", a small extension to the store may be needed. Implementer should inspect the current query builder before adding new filters.
- Whether `--project` filter should accept a project key, name, or ID. Resolve based on what the synced `projects` table stores as a human-queryable slug. Mirror the `--team` filter behavior for consistency.
- Whether a `--mine` shortcut is worth adding on top of `--assignee me`. Defer until we see agents using the long form.

## High-Level Technical Design

> This illustrates the intended command shape and is directional guidance for review, not implementation specification.

```
linear-pp-cli issues <ID>                         # single-issue get
linear-pp-cli issues list                         # list, local-backed
  --assignee <me|user-id|user-name>               # me shorthand resolves via live viewer query
  --state <started|backlog|completed|canceled|all>
  --team <key-or-id>
  --project <key-or-id>
  --limit <int>
  --json
  --data-source <auto|live|local>                 # inherited global

Single-issue get resolution order (default --data-source auto):
  1. local store lookup by identifier
  2. live GraphQL POST issue(id: $id)
  3. on live failure, fall back to local with a clear "run sync" hint
```

## Implementation Units

- [ ] Unit 1: Add `issues` command replacing the broken promoted version

Goal: ship a working single-issue get and a new `issues list` with common filter flags.

Requirements: R1, R2, R3

Dependencies: none

Files:
- Create: `library/project-management/linear/internal/cli/issues.go`
- Modify: `library/project-management/linear/internal/cli/root.go`
- Delete: `library/project-management/linear/internal/cli/promoted_issues.go`
- Test: `library/project-management/linear/internal/cli/issues_test.go`

Approach:
- `newIssuesCmd(flags)` returns a cobra command with `Use: "issues <ID>"` and a `list` subcommand.
- The bare command calls a helper `getIssueByIdentifier(flags, id)` that implements the three-stage resolution order (local, live POST, local fallback).
- The `list` subcommand calls `store.ListIssues(filterMap, limit)` and renders the same table layout as `today.go` for consistency. JSON output matches the `today --json` shape.
- Live viewer resolution uses `c.QueryInto("query { viewer { id } }", nil, &dest)`, identical to `today.go`.
- Replace line 158 in `root.go` so `rootCmd.AddCommand(newIssuesCmd(&flags))` wires the new command.

Patterns to follow:
- `library/project-management/linear/internal/cli/today.go` for client usage, sqlite pull, and rendering.
- `library/project-management/linear/internal/cli/stale_custom.go` for `--team` flag handling.

Test scenarios:
- Happy path: `issues list` with no filters returns all active issues from a seeded store.
- Happy path: `issues list --assignee me` resolves viewer id and returns only issues whose `assignee_id` matches.
- Happy path: `issues ESP-1155` returns the matching issue from the seeded store.
- Edge case: `issues list` on an empty store prints "No issues found" and exits 0.
- Edge case: `issues list --state all` returns completed and canceled issues in addition to active ones.
- Error path: `issues NOPE-999` returns a not-found error with exit code matching the existing not-found convention.
- Error path: `issues list --data-source local` on an unsynced store returns a "run `linear-pp-cli sync` first" message, not a raw sqlite error.
- Integration: `issues list --assignee me --state started --team ESP` applies all three filters; the test confirms the store query includes each filter key.

Verification:
- `go build ./...` succeeds with `promoted_issues.go` removed.
- Running `linear-pp-cli issues list --assignee me --json` returns a valid JSON array after a sync.
- Running `linear-pp-cli issues ESP-1155` returns the issue without HTTP 400.
- `.github/scripts/verify-skill/verify_skill.py` passes locally when run against the updated SKILL.md.

- [ ] Unit 2: Improve error classification for GraphQL GET failures

Goal: turn raw CSRF HTTP 400 errors and the "ID graphql" fallback error into actionable guidance, even for promoted commands this plan does not rewrite.

Requirements: R4

Dependencies: none

Files:
- Modify: `library/project-management/linear/internal/cli/helpers.go`
- Modify: `library/project-management/linear/internal/cli/data_source.go`
- Test: `library/project-management/linear/internal/cli/helpers_test.go`

Approach:
- Extend `classifyAPIError` to detect response bodies containing `potential Cross-Site Request Forgery` or `Please either specify a 'content-type' header` and return a formatted error suggesting the user try `linear-pp-cli issues list`, `linear-pp-cli today`, or `linear-pp-cli sync` as appropriate.
- In `resolveRead`, when the resource name is a promoted GraphQL resource and the extracted `id` equals `graphql`, return a specific error explaining that the live path is not yet supported for this resource and pointing to `issues list` or the local store.
- Keep the original error wrapped so `--json` callers still get structured detail.

Patterns to follow:
- Existing `classifyAPIError` branches (for 401, 403, 404, 429) already map to hint strings.

Test scenarios:
- Happy path: a 400 response with a CSRF body produces the new classified message referencing `issues list`.
- Edge case: a 400 response without CSRF content passes through unchanged.
- Edge case: `resolveRead` called with id `graphql` returns the "not yet supported" message, not the confusing "not found in local store" message.
- Error path: other resource IDs continue to produce the existing not-found error.

Verification:
- Unit tests pass.
- Manually reproducing `linear-pp-cli projects` (another promoted resource with the same bug) now produces the classified error rather than the raw CSRF body.

- [ ] Unit 3: Update README, SKILL.md, and command help text

Goal: make the new commands discoverable from every surface a user or agent might consult.

Requirements: R5

Dependencies: Unit 1 (flags must exist before SKILL.md references them, or the verifier fails)

Files:
- Modify: `library/project-management/linear/README.md`
- Modify: `library/project-management/linear/SKILL.md`
- Modify: `library/project-management/linear/internal/cli/issues.go` (example text, long description)

Approach:
- README gets a "Finding issues" section with three examples: `issues list`, `issues list --assignee me --state started`, and `issues ESP-1155`. Remove or rewrite any existing example that still recommends the broken `issues <ID>` path.
- SKILL.md adds a trigger section for "find my tickets", "what's assigned to me", "look up issue ABC-123". Include the same three examples.
- Cobra `Example:` strings on `issues` and `issues list` show realistic ID format (`ESP-1155`) and the `--assignee me` shorthand.

Patterns to follow:
- Existing SKILL.md examples for `today` and `stale`.

Test scenarios:
- Test expectation: none for README text changes.
- Integration: `.github/scripts/verify-skill/verify_skill.py` passes against the new SKILL.md, confirming every referenced flag is declared on the command it is shown on.

Verification:
- `python .github/scripts/verify-skill/verify_skill.py library/project-management/linear` exits 0.
- `linear-pp-cli issues list --help` and `linear-pp-cli issues --help` both show helpful examples.

- [ ] Unit 4: Regenerate plugin skills and bump plugin version

Goal: keep `plugin/skills/pp-linear/` and `plugin/commands/pp-linear.md` in lockstep with the library SKILL.md, per AGENTS.md.

Requirements: R5

Dependencies: Unit 3

Files:
- Run generator: `tools/generate-skills/main.go`
- Modify: `plugin/skills/pp-linear/SKILL.md` (regenerated)
- Modify: `plugin/commands/pp-linear.md` (regenerated)
- Modify: `plugin/.claude-plugin/plugin.json` (version bump, semver patch)

Approach:
- Run `go run ./tools/generate-skills/main.go` from the repo root.
- Inspect the diff and confirm only the pp-linear skill and command files changed, plus the version field.
- Bump the patch version manually if the generator did not auto-bump (content changes do not trigger auto-bump per AGENTS.md).

Patterns to follow:
- Prior commits with `chore(plugin): regenerate pp-* skills + bump to X.Y.Z` message.

Test scenarios:
- Test expectation: none, this is generator output. The behavioral guarantee is verified by CI running `verify-skills.yml`.

Verification:
- `git status` shows only expected file changes.
- `.github/workflows/verify-skills.yml` passes on the PR.

- [ ] Unit 5: Smoke-test against a real Linear workspace

Goal: confirm the fix works end to end against a live workspace before merging.

Requirements: R1, R2, R3, R4

Dependencies: Unit 1, Unit 2

Files: none (manual verification only)

Approach:
- `go install ./library/project-management/linear/cmd/linear-pp-cli`
- With `LINEAR_API_KEY` set, run each command in the verification list below and capture outputs. Paste into the PR description as evidence.

Test scenarios:
- Test expectation: none, this is manual smoke coverage.

Verification:
- `linear-pp-cli sync` succeeds.
- `linear-pp-cli issues list` returns a table.
- `linear-pp-cli issues list --assignee me --json | jq length` returns a non-zero count if the user has assigned issues.
- `linear-pp-cli issues ESP-1155` (or any real ID) returns the issue with no CSRF error.
- `linear-pp-cli issues ESP-1155 --data-source live` hits the API and returns the issue via POST.
- `linear-pp-cli projects --data-source live` still fails (expected, not in scope), but the error now names the GraphQL-GET cause and suggests `sync`, confirming Unit 2 benefits other promoted resources.

## System-Wide Impact

- Interaction graph: the new `issues` command touches the shared `rootFlags`, `Client`, `Store`, and `classifyAPIError`. No new middleware or callback surfaces.
- Error propagation: `classifyAPIError` is the single funnel; extending it benefits every read. `resolveRead` fallback string is user-facing and touches every promoted read that targets `/graphql` as its path, not just `issues`.
- State lifecycle risks: none; the change is read-only for Linear state and only reads the local sqlite store.
- API surface parity: `issues list` flags (`--assignee`, `--state`, `--team`, `--project`, `--limit`) should align with `stale` and `workload` flag names so agents can transfer muscle memory.
- Integration coverage: the three-stage resolution order in single-issue get is the main place unit tests alone will not prove. Unit 5's manual smoke is how we confirm the live path.
- Unchanged invariants: all other promoted commands continue to work the same way they do now (including when they are broken by the same CSRF bug). This plan explicitly does not fix them; it only improves the error message they produce.

## Risks and Dependencies

| Risk | Mitigation |
|------|------------|
| Removing `promoted_issues.go` causes a regen conflict if the generator is re-run. | Document in the PR description. The generator fix belongs in `cli-printing-press` and is listed as deferred. |
| `--assignee me` depends on a live `viewer` query, which fails if auth is invalid. | Return a clear error if viewer resolution fails and suggest `linear-pp-cli auth` and `linear-pp-cli doctor`. |
| Store schema does not express `state type != completed` in a single `ListIssues` filter call. | Implementer inspects the current store query builder in Unit 1; if a small extension is needed, add it before the command ships. Do not fork the store. |
| `verify_skill.py` rejects a flag referenced in SKILL.md that is not declared on the command. | Sequence Unit 3 after Unit 1 so flags exist before docs describe them. Run the verifier locally. |

## Documentation and Operational Notes

- No rollout, feature flag, or monitoring work required. This is a CLI-only hand-fix.
- After landing, update the pp-linear CHANGELOG.md entry for the next release to call out the agent-friendly improvements. Not a blocking item for the PR.
- If this fix proves the pattern, file an issue in `cli-printing-press` to teach the generator to emit POST-based GraphQL reads by default.

## Sources and References

- Broken get command: `library/project-management/linear/internal/cli/promoted_issues.go`
- Working precedent for GraphQL POST: `library/project-management/linear/internal/cli/today.go`
- GraphQL client: `library/project-management/linear/internal/client/graphql.go`
- Store methods: `library/project-management/linear/internal/store/` (`ListIssues`, `SearchIssues`, `GetByID`)
- Command wiring: `library/project-management/linear/internal/cli/root.go`
- Plugin regeneration conventions: `AGENTS.md` in this repo
- CI verifier: `.github/scripts/verify-skill/verify_skill.py`
