# session_handshake token delivery, auto-refresh, referrer-locked API key, and test isolation

## Why this patch belongs in the printed tree

Found live during Phase 5 dogfooding against the real Flow API, in this order:

1. **Query-param delivery is wrong for Flow.** The generated `session_handshake`
   template (`internal/client/client.go`'s `doInternal`) attached the token as
   an `access_token` URL query parameter. A live captured `ya29.*` OAuth2
   access token is 500+ characters; on the wire this produced
   `http2: request header list larger than peer's advertised limit` and an
   HTTP 413. Live browser capture (monkey-patched `fetch`/`XMLHttpRequest` on
   a real Flow page) confirmed Flow always expects
   `Authorization: Bearer <token>` as a header, never a query parameter. This
   looks like a generator-template bug, not something Flow-specific -- worth a
   Printing Press retro on the `session_handshake` auth type.

2. **`EnsureToken()`'s automatic refresh can never work for Flow.**
   `internal/client/session.go`'s `sessionTokenURL` points at
   `https://labs.google/fx/tools/flow`, and with `sessionTokenFormat = "text"`,
   `EnsureToken()` GETs that URL and treats the *entire raw response body* as
   the plaintext token. That URL is a client-rendered SPA shell (a full HTML
   page); Flow has no server-side endpoint that mints a token over plain HTTP
   at all -- the real `ya29.*` token is minted client-side by Google Identity
   Services JS running in the browser. So every automatic-refresh call
   succeeds "successfully" at fetching *something*, caches that raw HTML page
   as if it were a token, and every subsequent request fails in confusing
   ways. This was caught directly: `~/.local/share/flow-pp-cli/session.json`
   was found on disk with `"token": "<!DOCTYPE html>...Google Flow - AI
   Creative Studio..."` cached as a literal token value.

3. **The generated test suite was the one calling this dead endpoint.**
   Three generated test files (`client_test.go`,
   `client_verify_short_circuit_test.go`) build a bare
   `New(&config.Config{BaseURL: server.URL}, ...)` with no auth material at
   all. Because `c.Session` is unconditionally non-nil for this auth type,
   every `c.do`/`c.doRead` call in those tests hit the
   `authHeader == "" && c.Session != nil` branch and called the real,
   internet-reaching `EnsureToken()` -- meaning `go test ./internal/client/...`
   made live HTTP calls to `https://labs.google/fx/tools/flow` and re-wrote
   the corrupted `session.json` above on every run, entirely by accident. A
   fourth generated test file, `platform_rate_limit_test.go`, already avoids
   this by setting `AuthHeaderVal: "Bearer synthetic"` on its config -- that
   is the established, correct pattern this print now applies everywhere.

4. **The `aisandbox-pa.googleapis.com` API key is HTTP-referrer-restricted.**
   After fixes 1-3, a live `credits` call still failed -- but this time inside
   `errNoSessionToken` on a *retry*, not the initial attempt: the real request
   went out with a valid header-delivered Bearer token, got back a real
   `403 PERMISSION_DENIED` from Google (`API_KEY_HTTP_REFERRER_BLOCKED`,
   `"Requests from referer <empty> are blocked"`), which matched
   `sessionInvalidationStatuses` and triggered a doomed retry. Confirmed with
   raw `curl`: the identical request with `-H "Authorization: Bearer ..."` and
   no `Referer` gets 403; adding `-H "Referer: https://labs.google/"` gets a
   real `200` with real account data (`{"credits":1035,...}`). Go's
   `http.Client` never sends a `Referer` header on its own, so every live call
   to this host was doomed regardless of the token being valid. This is a
   real Flow constraint (the API key is scoped to that origin on Flow's own
   GCP project), not a generator bug -- but it means this printed CLI cannot
   talk to `aisandbox-pa.googleapis.com` at all without spoofing the header a
   real Flow browser tab would send.

5. **Flow actually has two separate backend origins with two separate auth
   mechanisms, and this CLI is only wired for one.** Confirmed live with
   `credits` (200, real account data) once fixes 1-4 landed:
   `aisandbox-pa.googleapis.com` (the `sessionDataBaseURL` this CLI targets --
   credits, video status, and presumably generation-submit) accepts the
   harvested `ya29.*` Bearer token. But `scenes gaps`, `drive import
   --tag-scene`, `project`, and `projects` all call
   `labs.google/fx/api/trpc/project.*` (`fetchProjectContents` in
   `internal/cli/drive_folder.go`) -- Flow's own Next.js BFF, authenticated by
   a **NextAuth session cookie** (`__Secure-next-auth.session-token`), not the
   Bearer token at all. Verified directly: the identical `curl` with a valid
   Bearer token *and* a matching `Referer` still gets `401 UNAUTHORIZED` from
   `project.searchUserProjects` -- adding `Referer` (fix 4) only ever helped
   the `aisandbox-pa.googleapis.com` surface. `auth login --cookies-file`
   already imports cookies via `ImportSession`, but scopes them to
   `sessionDataBaseURL` (`aisandbox-pa.googleapis.com`) -- the wrong origin
   for this second auth surface. **Not fixed in this patch** -- wiring a
   second, cookie-based transport for the `labs.google` origin is a real
   scope expansion (a second base URL, a second credential-attachment path,
   and capturing real NextAuth cookies rather than the bearer token) rather
   than a bug-sized fix. Until that lands, `projects`/`project`/`scenes
   gaps`/`drive import --tag-scene` remain genuinely broken against the live
   API even with a valid `FLOW_SESSION_TOKEN` -- document this plainly rather
   than implying they work.

6. **The `errNoSessionToken` sentinel masked the fix-4 finding at first.** After
   fix 4's `Referer` addition was added but before this note, a live `credits`
   call reporting the 403 above still surfaced as `errNoSessionToken`
   ("no Flow session token cached") -- wrong and misleading, since a real
   token was sent and the server did respond, just with a rejection. Root
   cause: the retry loop's `ShouldInvalidate` branch clears `authHeader` and
   `continue`s on any 401/403, and the *next* iteration's
   `authHeader == "" && c.Session != nil` check unconditionally returned
   `errNoSessionToken`, discarding the real `lastErr` the previous iteration
   had just set. Fixed by returning `lastErr` when it is non-nil (a real
   request already went out and failed) and reserving `errNoSessionToken` for
   the case where no request was ever attempted at all (`lastErr == nil`).
   Verified against the source, not just inference: `scenes gaps` and `drive
   import --tag-scene` really do call `fetchProjectContents` /
   `flow.projectInitialData` at the absolute URL
   `https://labs.google/fx/api/trpc/flow.projectInitialData`
   (`internal/cli/drive_folder.go`), confirming item 5's labs.google/NextAuth
   diagnosis is correct and not an artifact of a shared error string.

## Reprint guard

On reprint, preserve these three changes:

- `internal/client/client.go`'s `doInternal`: attach `authHeader` via
  `req.Header.Set("Authorization", authHeader)`, never as a query parameter.
  Both places that used to call `c.Session.EnsureToken()` as an
  empty-token fallback (initial resolve, and the per-attempt re-resolve after
  a 401/403 invalidation) now return a fixed `errNoSessionToken` sentinel
  instead, telling the user to capture a fresh token from the browser and set
  `FLOW_SESSION_TOKEN`. Do not restore the `EnsureToken()` auto-refresh calls
  in `doInternal` -- they cannot succeed for Flow and will silently re-corrupt
  the on-disk session cache.
- `internal/cli/auth.go`'s `newAuthLoginCmd`: no longer calls
  `c.Session.EnsureToken()` after `ImportSession`. It imports cookies (kept
  for reference / potential future use) but tells the user plainly that Flow
  has no token-mint endpoint and they must still set `FLOW_SESSION_TOKEN`
  manually. Do not restore the old "Imported ... and refreshed the session
  token" success message -- it was never true.
- `internal/client/client_test.go` and
  `internal/client/client_verify_short_circuit_test.go`: their bare
  `config.Config{BaseURL: ...}` test fixtures now also set
  `AuthHeaderVal: "Bearer synthetic"`, matching `platform_rate_limit_test.go`.
  Without this, any test that exercises `c.do`/`c.doRead` against a
  no-auth-configured client will fail fast on `errNoSessionToken` instead of
  running the behavior it actually means to test -- or, pre-patch, will
  silently reach out to the real internet.
- `internal/client/client.go`'s `doInternal`: every request now sets
  `Referer: https://labs.google/` unconditionally, right after the
  Authorization header. Without it, any live call to
  `aisandbox-pa.googleapis.com` (credits, video status, project contents --
  every "data" endpoint this CLI's `sessionDataBaseURL` points at) gets a
  403 `API_KEY_HTTP_REFERRER_BLOCKED` no matter how valid the Bearer token
  is. Do not remove this on the theory that it looks unrelated to auth.
- `internal/client/client.go`'s retry loop: the empty-`authHeader` check
  inside the loop returns `lastErr` when it is non-nil, falling back to
  `errNoSessionToken` only when no request was ever attempted. Do not revert
  this to unconditionally returning `errNoSessionToken` -- that silently
  discards the server's real error on every 401/403 and actively misleads
  toward the wrong fix.

If a future reprint's `session_handshake` template gains a real,
Flow-specific token-mint mechanism, this whole patch (query-param fix
excepted -- that one's unconditionally correct) becomes obsolete and should
be re-evaluated rather than blindly re-applied.
