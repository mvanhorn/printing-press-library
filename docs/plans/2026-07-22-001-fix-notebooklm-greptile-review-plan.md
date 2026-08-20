---
title: "fix: Resolve NotebookLM Greptile review findings"
date: 2026-07-22
type: fix
depth: standard
execution: code
target_repo: printing-press-library
origin: Greptile PR #1562 review (confidence 2/5, policy gate requires ≥4/5)
---

# fix: Resolve NotebookLM Greptile review findings

**Target repo:** printing-press-library (branch `feat/notebooklm`, PR #1562)

## Summary

Greptile flagged four issues on the NotebookLM publish PR. The policy gate requires confidence ≥4/5; current score is 2/5. This plan removes the local `go.mod` replace, hardens `sync` error and filter semantics, and fixes Studio artifact wait behavior so `--wait` polls until terminal status.

## Problem Frame

The NotebookLM CLI publish PR passes Verify and other CI checks but fails the **Greptile policy gate** because Greptile's confidence score is 2/5 (threshold: 4/5). Four inline review comments describe concrete bugs: a local filesystem `replace` in `go.mod`, sync swallowing auth failures, sync ignoring empty resource filters, and artifact wait returning pending artifacts as complete.

## Requirements

| ID | Requirement |
|----|-------------|
| R1 | `go.mod` must build in CI without author-local `replace` directives |
| R2 | `sync` must return a non-zero exit when client/bootstrap fails (not success with zero count) |
| R3 | `sync --resources <unknown-id>` must sync zero notebooks, not the full batch |
| R4 | `WaitForArtifact` with a specific ID must poll until ready, fail on terminal failure, or timeout — never return pending as success |
| R5 | Greptile policy gate confidence ≥4/5 after push |

## Key Technical Decisions

1. **Remove `replace` entirely** — Other library CLIs (e.g. midjourney) use the monorepo module path without `replace`. The notebooklm module is already `github.com/mvanhorn/printing-press-library/library/ai/notebooklm`; CI builds from the repo root and resolves imports natively.

2. **Propagate sync client errors** — `newAPIClient` already returns typed `cliError` values (`authErr`, `configErr`, etc.). Match other commands (`notebook list`, `studio list`) and return the error directly instead of printing a warning and exiting 0.

3. **Always apply resource filter** — When `--resources` is resolved (including default `notebooks`), assign `batch = filtered` unconditionally. An empty filter result means zero notebooks to sync.

4. **Artifact wait: delete early-return bypass** — The first loop already returns when status is ready/complete. The second loop (lines 61–66) incorrectly returns any matching ID regardless of status; remove it. Add explicit failed-status detection so terminal failures return an error instead of polling until timeout.

5. **Extract testable helpers** — Pull resource-filter and artifact-status logic into small unexported helpers with unit tests, following existing test files under `internal/nlm/`.

## Scope Boundaries

### In scope
- Four Greptile findings on PR #1562
- Unit tests for new/changed behavior
- Push to `feat/notebooklm` and verify CI

### Out of scope
- Other Greptile style suggestions not in the four findings
- Scorecard/dogfood polish beyond what fixes require
- Resolving Greptile review threads manually (auto-resolves on re-review after push)

## Implementation Units

### U1. Remove local go.mod replace

**Goal:** Make the module reproducible in CI and GoReleaser.

**Requirements:** R1

**Files:**
- Modify: `library/ai/notebooklm/go.mod`

**Approach:** Delete the `replace github.com/mvanhorn/printing-press-library => /Users/apple/...` line. Run `go mod tidy` if needed.

**Test expectation:** none — config-only change; build verification in U4.

**Verification:** `go build ./...` from `library/ai/notebooklm` succeeds without local replace.

---

### U2. Fix sync auth failure exit code

**Goal:** Sync returns non-zero when authentication or bootstrap fails.

**Requirements:** R2

**Dependencies:** U1

**Files:**
- Modify: `library/ai/notebooklm/internal/cli/sync.go`

**Approach:** Replace the warn-and-return-nil block (lines 48–55) with `return err`. `newAPIClient` already wraps errors with correct exit codes.

**Patterns to follow:** `internal/cli/studio.go`, `internal/cli/notebooks.go` — both `return err` from `newAPIClient`.

**Test scenarios:**
- Error path: simulate `newAPIClient` failure via extracted helper or command test — expect non-nil error propagated (integration via `RunE` with invalid auth is execution-time; unit test the error path if extractable).

**Verification:** Expired/missing cookie causes sync to exit with code 4 (auth), not 0.

---

### U3. Fix sync empty resource filter

**Goal:** Unknown resource IDs sync nothing.

**Requirements:** R3

**Dependencies:** U1

**Files:**
- Modify: `library/ai/notebooklm/internal/cli/sync.go`
- Create: `library/ai/notebooklm/internal/cli/sync_test.go`

**Approach:** Extract `filterNotebooksByResources(batch []nlm.Notebook, resources []string) []nlm.Notebook`. Always assign `batch = filtered` when filtering runs. Remove `len(filtered) > 0` guard.

**Test scenarios:**
- Happy path: `resources=["notebooks"]` with 3 notebooks → returns all 3
- Happy path: `resources=["id-a"]` with matching notebook → returns 1
- Edge case: `resources=["nonexistent"]` with 3 notebooks → returns empty slice (not all 3)
- Edge case: empty batch input → returns empty

**Verification:** Unit tests pass; `sync --resources nonexistent-id` syncs 0.

---

### U4. Fix artifact wait pending bypass

**Goal:** `--wait` polls until ready or fails on terminal failure.

**Requirements:** R4

**Dependencies:** U1

**Files:**
- Modify: `library/ai/notebooklm/internal/nlm/artifacts.go`
- Create: `library/ai/notebooklm/internal/nlm/artifacts_test.go`

**Approach:**
- Add `artifactReady(status string) bool` and `artifactFailed(status string) bool` helpers
- In `WaitForArtifact`, when a specific `artifactID` is found: return error if failed, return artifact if ready, otherwise continue polling
- Remove the second loop that returns matching ID without status check

**Test scenarios:**
- Happy path: `artifactReady("ready")` → true; `artifactReady("complete")` → true
- Edge case: `artifactReady("pending")` → false; `artifactReady("")` → true (existing behavior)
- Error path: `artifactFailed("failed")` → true; `artifactFailed("error")` → true

**Verification:** Unit tests pass; `studio generate-quiz --wait` does not return immediately on pending artifact.

---

### U5. Verify and push

**Goal:** Confirm fixes locally and unblock Greptile policy gate.

**Requirements:** R5

**Dependencies:** U1, U2, U3, U4

**Approach:** Run `go test ./...` and `go build ./...` in notebooklm; commit; push to `feat/notebooklm`.

**Verification:** Local tests green; Greptile re-review shows confidence ≥4/5 and policy gate passes.

## Risks & Dependencies

- **Risk:** Removing `replace` may break local dev builds outside the monorepo. **Mitigation:** Standard pattern for all published library CLIs; builds run from repo checkout.
- **Risk:** Greptile may not immediately re-score on push. **Mitigation:** Policy gate workflow waits for new review; nudge if needed.

## Deferred to Implementation

- Exact failed-status strings from live NotebookLM API (use conservative `fail`/`error` substring matching).
