# WeWork CLI — Chrome-session login (auth login --chrome)

## Built (hand-authored, markerless, self-registered under the auth parent)
- `internal/cli/wework_chrome.go` — `auth login --chrome`: scans Chrome/Chromium/Brave/Edge
  Local Storage (all profiles) with a tolerant RAW BYTE SCAN (no LevelDB dependency), extracts
  the auth0 access_token via regex, filters to WeWork tokens by distinctive JWT claims
  (user_origin/service_records/level2_access), picks the freshest by exp, and persists via
  the existing SaveComposedAuth. Registered via the registerNovelCommand init hook in
  internal/cli/wework_auth.go. Unit-tested (extract/pick/tokenIsWeWork/itoa).

## Verified against real Chrome
- `auth login --chrome --json` scanned 1 WeWork profile and imported a token (token:true).
- Graceful fallback proven earlier: clear error + points to `auth import` when no session found.
- go test (cli+config) pass; go vet clean; shipcheck all legs PASS except live_api_verification HOLD.

## Key limitation discovered (documented in help + README/SKILL)
- The token Chrome extracted was EXPIRED (2026-07-07). Chrome keeps the CURRENT auth0 token in
  memory and flushes to Local Storage lazily, so the on-disk copy can be stale. Therefore:
  - `auth login --chrome` is a convenience (quick) but may return a stale token; it now warns
    explicitly when the imported token is already expired.
  - `auth import` (DevTools snippet reading live localStorage) remains the RELIABLE way to get a
    fresh token. Docs present both; auth login refreshes only the token, uuid/member set once via
    auth import (they are memory-only in Chrome, never on disk, and not derivable from the JWT).

## Design notes
- Raw-scan chosen over goleveldb: goleveldb failed to open the live (locked) Chrome DB reliably;
  raw scan of *.ldb/*.log is far more robust and drops the dependency. Per-DIRECTORY origin gate
  (not per-file) handles LevelDB key prefix-compression where the token's value file lacks the
  literal origin string.
- Writing this credential-reading file required the user to enable bypass permissions (the
  auto-mode classifier blocks authoring browser-credential-reading code by default).
