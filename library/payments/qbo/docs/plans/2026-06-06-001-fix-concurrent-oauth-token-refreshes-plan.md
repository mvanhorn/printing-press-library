---
title: fix(auth): implement cross-process locking for concurrent OAuth token refreshes
type: fix
status: active
date: 2026-06-06
---

# fix(auth): implement cross-process locking for concurrent OAuth token refreshes

## Summary

This plan proposes implementing cross-process/file-based locking using `github.com/gofrs/flock` around the QuickBooks Online OAuth token refresh routine. When multiple CLI or MCP server processes run concurrently and try to refresh an expired token, only one process will acquire the lock and rotate the token. Other processes will wait for the lock to be released, reload the newly rotated token from the config file on disk, detect that the token is now valid, and safely short-circuit their own refresh.

---

## Problem Frame

QuickBooks Online immediately rotates and invalidates the previous OAuth refresh token on renewal. If multiple agent processes or parallel CLI commands invoke commands concurrently, both may see an expired token and attempt to refresh it at the same millisecond. The first process succeeds and updates the credentials, but the losing process receives a `400 Bad Request` from QBO because it presents the now-invalidated refresh token. This immediately invalidates the entire session and locks out the user.

---

## Assumptions

*This plan was authored without synchronous user confirmation. The items below are agent inferences that fill gaps in the input — un-validated bets that should be reviewed before implementation proceeds.*

- Concurrently executing commands run on the same local filesystem (or a shared volume) where they share access to the token config file (typically `~/.config/qbo-pp-cli/config.toml`).
- The directory containing the configuration file is writable, allowing us to create and update a `token.lock` file in the same folder.

---

## Requirements

- R1. Create a cross-process lock file using `github.com/gofrs/flock` at `token.lock` in the same directory as the config file.
- R2. Wrap token refresh in `flock.TryLockContext` to ensure only one process performs the rotation at a time.
- R3. Wait for the lock to be released if it is held by another process.
- R4. Reload the configuration file from disk immediately after acquiring the lock.
- R5. Short-circuit the refresh if the reloaded configuration shows the token has already been rotated (by verifying `TokenExpiry` is in the future).

---

## Scope Boundaries

- This plan does not change the core token storage format or config structure.
- Locking only applies to the token refresh routine; other API calls do not require lock coordination.
- We do not use OS keychain storage in this plan (Issue #13).

---

## Context & Research

### Relevant Code and Patterns

- `internal/client/client.go`: Defines the client HTTP round-trip logic and the `refreshAccessToken` method.
- `internal/config/config.go`: Defines the `Config` structure, loading, and saving tokens.

### External References

- `github.com/gofrs/flock`: A thread-safe, process-safe Go wrapper around standard file locking sys-calls.

---

## Key Technical Decisions

- **KTD1. Use file-based locking via github.com/gofrs/flock**: Provides portable, cross-process lock files suitable for macOS, Linux, and Windows.
- **KTD2. Dynamic config reload & short-circuit**: Waiting processes reload config from disk post-lock-acquisition, check if another process rotated the token, and return early to avoid invalidating the session.

---

## Open Questions

### Resolved During Planning

- *None*

### Deferred to Implementation

- *None*

---

## Implementation Units

### U1. Install flock dependency

**Goal:** Add `github.com/gofrs/flock` to `go.mod` and `go.sum`.

**Requirements:** R1

**Dependencies:** None

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

**Approach:**
- Run `go get github.com/gofrs/flock` to pull the dependency.

**Verification:**
- `go test ./...` succeeds and compilation check passes.

---

### U2. Implement locking and reloading in internal/client/client.go

**Goal:** Implement locking, config reloading, and short-circuit checks inside `refreshAccessToken`.

**Requirements:** R1, R2, R3, R4, R5

**Dependencies:** U1

**Files:**
- Modify: `internal/client/client.go`

**Approach:**
- In `refreshAccessToken`, resolve the lock path: `filepath.Join(filepath.Dir(c.Config.Path), "token.lock")`.
- Initialize `flock.New(lockPath)`.
- Call `TryLockContext` with retry interval (e.g., `100 * time.Millisecond`) to respect context cancellation.
- Upon lock acquisition, run `config.Load(c.Config.Path)` to reload config from disk.
- Update `c.Config` fields (`AccessToken`, `RefreshToken`, `TokenExpiry`, `AuthHeaderVal`) in memory with reloaded values.
- Check if `c.Config.TokenExpiry` is in the future. If so, return `nil` immediately (short-circuit).
- Release the lock via `defer lock.Unlock()`.

**Verification:**
- `go build ./...` compiles cleanly.

---

### U3. Write concurrency tests for OAuth token refresh

**Goal:** Verify concurrent token refresh attempts work safely without duplicate requests or session invalidation.

**Requirements:** R2, R3, R4, R5

**Dependencies:** U2

**Files:**
- Test: `internal/client/client_concurrency_test.go`

**Approach:**
- Set up a test server or round tripper representing the token endpoint.
- Simulate concurrent invocations of `refreshAccessToken` using goroutines.
- Verify that only one request is dispatched to the token server, and both callers get the refreshed token.

**Verification:**
- Run `go test -v ./internal/client -run TestClient_ConcurrentTokenRefresh` and ensure it passes.

---

## System-Wide Impact

- **Interaction graph:** `refreshAccessToken` is called from `authHeader` and HTTP round-trip on 401s.
- **Error propagation:** Failures to acquire the lock or reload the config propagate as errors to the caller.
- **State lifecycle risks:** Ensure lock is always unlocked (via `defer`) even if requests fail or context is cancelled.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Read-only configuration directory prevents lock file creation | Return descriptive error if lock file initialization fails, suggesting directory check. |
| Context timeout during wait | Log or return timeout error gracefully, unlocking the file descriptor. |

---

## Sources & References

- GitHub Issue #10: fix(auth): implement cross-process locking for concurrent OAuth token refreshes
