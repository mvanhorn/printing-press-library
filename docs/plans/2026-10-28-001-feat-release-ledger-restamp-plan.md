---
title: "feat: release-ledger restamp mode"
date: 2026-10-28
type: feat
artifact_contract: ce-unified-plan/v1
product_contract_source: ce-plan-bootstrap
execution: code
---

# feat: release-ledger restamp mode

## Goal Capsule

- Objective: an operator can make a published CLI's downloadable binaries and pinned `go install` report the version its release ledger already records, without cutting a new release. First use: `ai/v0`, whose ledger says 2026.9.1 while its compiled `var version` is 2026.8.1, so build-mcpb.yml correctly refuses to publish it.
- Means: a `--restamp` mode in `tools/release-ledger` driven by a `restamp` workflow_dispatch input on `update-cli-release-ledger.yml`, plus registry pinning of restamp commits (KTD1, KTD3, KTD4).
- Authority: the user's request and settled decisions (session) > this plan > repo AGENTS.md conventions.
- Stop conditions: stop if restamp would need to change `version`, `released_at`, `source_commit`, or `CHANGELOG.md`; stop if the low-water-mark marker discovery cannot tolerate the new commit subject.
- Execution profile: Go tool change + tests, workflow YAML, one generate-registry change, docs paragraph. The post-merge dispatch for `ai/v0` is done by the orchestrator after merge, not by `ce-work`.

## Product Contract

### Summary

Runtime stamping only happens when the ledger cuts a release for a CLI. Fixes to stamping (for example nested-module support from #2388) never reach CLIs that do not change afterwards. Restamp re-syncs the stamp from the existing ledger.

### Problem Frame

`library/ai/v0/vzero/internal/cli/version.go` and `library/ai/v0/vzero/cmd/v0-pp-mcp/main.go` carry `var version = "2026.8.1"` while `library/ai/v0/.printing-press-release.json` says 2026.9.1. The build gate refuses a rebuild, so `v0-current` binaries are stale. Replaying the ledger is not clean because `updateCLI` rewrites `released_at` and adds a changelog entry.

### Requirements

- R1. `go run ./tools/release-ledger/main.go --restamp [<cli dir>...]` stamps `var version` (and `var commit` / `var date` where present) in each selected CLI's runtime files from its existing `.printing-press-release.json` (`version`, `source_commit`, `released_at`), using `resolveModuleDir`. No args means every CLI with a released ledger.
- R2. Restamp never writes `.printing-press-release.json` or `CHANGELOG.md`. Running it twice produces no diff.
- R3. `--restamp --check` exits non-zero when a restamp would change files and lists the CLIs.
- R4. A CLI with no release ledger, an unreleased skeleton (blank version), or no stampable var is skipped without error. A named CLI that does not exist is an error.
- R5. A restamp-only commit does not cut a release on the next ledger run, for root modules and nested modules, including `cmd/*-pp-mcp/main.go`.
- R6. `update-cli-release-ledger.yml` accepts a `restamp` workflow_dispatch input (space-separated `<category>/<slug>` list or `all`), runs restamp, commits to main via `push.sh`, and dispatches build-mcpb.yml for those CLIs at the restamp commit plus generate-registry.yml.
- R7. The restamp commit does not disturb the `Processed through:` low-water-mark discovery.
- R8. After a restamp, registry.json `release.source_commit` for that CLI points at the restamp commit, so a pinned `go install` reports the ledger version.
- R9. AGENTS.md "Per-CLI release ledger" gains one short paragraph on when and how to restamp.
- R10. No PR title, PR body, or authored commit body contains the literal skip-ci bracket marker. Only the workflow's bot commit subject carries it.

### Scope Boundaries

- Out: changing how releases are cut, CalVer rules, or build-mcpb.yml's gate.
- Out: running restamp across the whole library in this PR. The PR ships the capability; the `ai/v0` dispatch happens after merge.

### Acceptance Examples

- AE1. After merge, dispatching with `restamp=ai/v0` changes only the two vzero files to 2026.9.1; no CHANGELOG or ledger change.
- AE2. The dispatched build-mcpb run passes "Verify CLI version matches the release ledger" and publishes `v0-current`.
- AE3. The downloaded `v0-pp-cli` prints 2026.9.1 for `version`, and its `checksums.txt` line verifies.

## Planning Contract

### Key Technical Decisions

- KTD1. `--restamp` is a mode of the existing tool (session-settled, user-directed). CLI dirs come from positional args, accepted as `library/<cat>/<slug>` or `<cat>/<slug>`. It is exclusive with `--init-missing`, `--changed-from/--changed-to`, and `--list-first-parent`. It calls `stampRuntimeVersion(dir, existingManifest, check)` directly, so commit/date come from the release ledger's existing `source_commit` and `released_at` (the `.printing-press-release.json` read as `existingManifest`, not `.printing-press.json`). Lives in `main.go` because `go run` compiles that file alone.
- KTD2. `changedCLIKeys` already ignores stamp-only diffs by path suffix (`isRuntimeVersionPath`) and line content (`isRuntimeVersionStampLine`), which covers nested modules and MCP mains. Add a git-fixture test proving it for a nested `vzero` module with `version.go` and `cmd/v0-pp-mcp/main.go` (including `var commit`/`var date`). No code change unless the test fails.
- KTD3. Workflow: the `restamp` input switches the `update` job onto a restamp path. Ledger steps (meta, run) are skipped; a restamp step validates each token against `^[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*$` (or `all`) passed via env, runs the tool, and commits with subject `chore(releases): restamp CLI runtime versions [skip ci]` and no `Processed through:` marker. The meta step's loop only accepts the exact ledger subject, so this subject is skipped and discovery continues to the prior ledger commit (R7). Combining `restamp` with `changed_from`/`changed_to` fails fast. Targets: for a named list, every named CLI that has a Go module (rebuild even when the stamp already matched, so stale `-current` binaries can be republished); for `all`, only CLIs whose files the restamp commit changed. build-mcpb.yml is dispatched at the restamp commit when one was pushed, otherwise at the run's HEAD; generate-registry.yml is dispatched only when a commit was pushed. The existing concurrency group serializes restamp with ledger runs. In normal ledger mode, the "Select CLIs to rebuild" step must not re-select CLIs only because a restamp commit inside the processed range touched their stamp files: build its build-input diff per first-parent commit from `steps.meta.outputs.commits`, skipping commits whose subject is the restamp bot subject.
- KTD4. generate-registry's `releaseManifestCommits` also considers commits whose subject starts with `chore(releases): restamp CLI runtime versions` and that touched a CLI's runtime stamp files. Newest commit per CLI wins (git log order). Without this, R8 fails: the registry keeps pinning the last ledger commit, which for v0 still has 2026.8.1.

### Assumptions

- A restamp commit is tied to a CLI in the registry by two signals together: the restamp bot subject and the commit touching that CLI's runtime stamp files. The subject string is duplicated in the workflow and generate-registry with a cross-reference comment.
- Rebuilding a named CLI whose stamp already matched is cheap and safe; build-mcpb's "newer release" and version gates still apply.

## Implementation Units

### U1. Restamp mode in tools/release-ledger

- Files: `tools/release-ledger/main.go`, `tools/release-ledger/main_test.go`.
- Approach: add `restamp bool` to options; parse positional args; skip the changed-range requirement and HEAD resolution in restamp mode; add `restampCLIs(dirs, check)` that reads each ledger, skips missing/blank, and calls `stampRuntimeVersion`. Report changed CLI keys; reuse the `--check` drift exit with a restamp-specific message. Update the header usage comment.
- Test scenarios: nested module (v0 shape: catalog without go.mod, `vzero/go.mod`, `version.go` + MCP main) gets stamped to the ledger version; root-module CLI with `var version`/`var commit`/`var date` gets ledger version, source_commit, released_at; second run is a no-op and the ledger/CHANGELOG bytes are unchanged; CLI with no stampable var reports unchanged; CLI without a ledger or with blank version is skipped; `--check` reports drift without writing; unknown named CLI errors.

### U2. Stamp-only diffs never cut a release (verify)

- Files: `tools/release-ledger/main_test.go` (code only if the test fails).
- Test scenarios: git fixture where commit 2 changes only `var version/commit/date` in `library/ai/v0/vzero/internal/cli/version.go` and `library/ai/v0/vzero/cmd/v0-pp-mcp/main.go` → `changedCLIKeys` excludes `ai/v0`; same CLI with a non-stamp edit in the MCP main is selected.

### U3. Workflow restamp path

- Files: `.github/workflows/update-cli-release-ledger.yml`.
- Approach per KTD3. Commit step branches on mode; targets step branches on mode; dispatch job unchanged except that it already dispatches generate-registry when `pushed == true`.
- Verification: actionlint; supply-chain scan; read-through that a normal ledger run whose range contains a restamp commit does not re-dispatch the restamped CLIs.

### U4. Registry pins restamp commits

- Files: `tools/generate-registry/main.go`, `tools/generate-registry/main_test.go`.
- Test scenarios: a restamp commit after a ledger commit becomes the CLI's source commit; a non-restamp commit touching root.go does not; a later ledger commit wins over an earlier restamp.

### U5. AGENTS.md paragraph

- Files: `AGENTS.md`.

## Verification Contract

- `cd tools/release-ledger && go test ./...`
- `cd tools/generate-registry && go test ./...` and `go run ./tools/generate-registry/main.go --validate` (from repo root; `--validate` returns before `releaseManifestCommits`, so KTD4 is covered by U4's unit tests)
- `actionlint .github/workflows/update-cli-release-ledger.yml`
- `python3 .github/scripts/verify-supply-chain/scan.py --base-ref origin/main`
- Local smoke: `go run ./tools/release-ledger/main.go --restamp --check ai/v0` exits non-zero naming ai/v0 (then discard nothing: check mode writes nothing).

## Definition of Done

- R1-R10 hold; all verification commands pass; no edits to `var version`, CHANGELOG.md, `.printing-press-release.json`, registry.json, or cli-skills/ in the diff.
