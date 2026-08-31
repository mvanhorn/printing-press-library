# labs.google NextAuth cookie auth surface

## Why this patch belongs in the printed tree

Flow has two independent backend origins with two independent auth
mechanisms, and the generated CLI was only ever wired for one of them (see
`2026-08-30-session-handshake-header-delivery.md`, finding 5). This patch
wires up the second.

**Confirmed live, step by step:**

1. Clicking through Flow's own sign-in ("Create with Google Flow" on the
   logged-out landing page) redirects to
   `accounts.google.com/v3/signin/...&redirect_uri=https%3A%2F%2Flabs.google%2Ffx%2Fapi%2Fauth%2Fcallback%2Fgoogle&scope=...auth/aisandbox+...userinfo.profile+...userinfo.email...`.
   `/api/auth/callback/google` is NextAuth.js's (Auth.js) exact standard OAuth
   callback path with Google as the provider -- this is not a guess, it's the
   literal convention the library uses.
2. After completing that sign-in, the browser holds
   `__Secure-next-auth.session-token` (HttpOnly, Secure, SameSite=Lax, a large
   opaque JWE -- confirmed ~1000+ chars) on the `labs.google` origin, alongside
   `__Host-next-auth.csrf-token` and `__Secure-next-auth.callback-url` (present
   on *any* visit, logged in or not -- these are not sufficient on their own).
3. A raw `curl` to `labs.google/fx/api/trpc/project.searchUserProjects` with
   *only* `Cookie: __Secure-next-auth.session-token=<value>` and
   `Referer: https://labs.google/` returned a real `200` with real project
   data. The same request with a valid `ya29.*` Bearer token instead (no
   cookie) returns `401 UNAUTHORIZED` -- confirms the two surfaces are
   genuinely independent, not two names for the same credential.
4. The server rotates `__Secure-next-auth.session-token` on every response
   (fresh `Set-Cookie` each time, standard NextAuth JWT sliding-session
   behavior) -- handled for free by Go's stdlib `cookiejar`, which already
   updates the jar from `Set-Cookie` on every response with no CLI-specific
   code needed.

**What was already correct, and didn't need to change:** `internal/client/client.go`'s
`New()` already sets `httpClient.Jar = sess.CookieJar()` unconditionally, and
Go's `net/http/cookiejar` is inherently multi-domain -- once a cookie for
`labs.google` is in the jar, any request to `https://labs.google/...` via
`c.HTTPClient` gets it attached automatically, no per-request code required.
`internal/client/session.go`'s `saveToDisk()` also already snapshots cookies
scoped to `labs.google` to disk, as an accidental side effect of
`sessionTokenURL` (`https://labs.google/fx/tools/flow`) sharing that host.

**What was actually broken -- four separate things:**

1. `internal/cli/auth.go`'s `newAuthLoginCmd` called
   `loadSessionCookiesFromFile(cookiesFile, ".aisandbox-pa.googleapis.com")`
   and `ImportSession("https://aisandbox-pa.googleapis.com/v1", ...)` -- the
   domain filter silently dropped every `labs.google`-scoped cookie (including
   the NextAuth session token) from an imported storage-state file, and even
   without the filter the default `cookieDomain` fallback pointed at the wrong
   origin. `aisandbox-pa.googleapis.com` never needed cookie auth at all (it
   uses the Bearer token exclusively) -- scoping cookie import there was
   always pointless.
2. `internal/client/client.go`'s `doInternal` required `authHeader != ""`
   (the Bearer token) before allowing *any* request, including ones targeting
   `labs.google` that only need the cookie (already attached by the jar).
   Added `requiresSessionBearerToken(targetURL)` -- true unless the target
   host is `labs.google` -- and gated both the initial check and the
   in-retry-loop check on it.
3. `internal/cli/promoted_projects.go`'s response path was
   `result.data.json.projects`; the real shape (confirmed via `curl`) is
   `result.data.json.result.projects` -- one extra `result` level than
   `flow.projectInitialData`'s shape (`result.data.json`, no extra nesting --
   that one was already correct and needed no change).
4. Three separate, disconnected copies of `projects`' path/response-path
   config all had both of the above bugs independently:
   `internal/cli/resource_paths.go`'s `resourceReadPaths`/`resourceReadConfigs`
   (feeds `export projects`), and `internal/cli/sync.go`'s
   `syncResourcePath`/`responsePathForResource` (feeds
   `sync --resources projects`). Fixed all three to the absolute
   `https://labs.google/fx/api/trpc/project.searchUserProjects` URL and the
   corrected response path.

**Verified live, after all four fixes, using a real captured session cookie
in an isolated `--home`:** `projects` (returns the real project list),
`project --input '{"json":{"projectId":"<real-id>"}}'` (returns real project
contents), and `scenes gaps --project <real-id>` (returns real character/media
gap data) all now work end-to-end with zero code changes beyond what's listed
above. `drive import --tag-scene --project <real-id>` also authenticates
correctly (tested against a project whose one character happens to have an
empty display name, so tagging itself had nothing to match -- a separate,
minor, unrelated finding, not an auth failure).

**New finding, NOT fixed by this patch -- flag for the user, don't
silently claim it works:** `sync --resources projects` and `export projects`
still fail, but for a different, unrelated reason: `project.searchUserProjects`
is a tRPC GET endpoint that requires its query params wrapped in a single
JSON-encoded `input=` envelope (`{"json":{"pageSize":20,"toolName":"PINHOLE",
"cursor":null},...}`), not flat query params. The generic sync/export engine
(`syncResource`/`export.go`) has no mechanism to build that envelope --
it only knows flat cursor/limit-style params. `promoted_projects.go`'s hand-
written command works because it hardcodes a full default `--input` flag
value; the generic list/sync/export path has no equivalent. This is a
structural gap in the generator's sync/export engine for tRPC-shaped APIs
generally, not specific to Flow or to auth -- worth a Printing Press retro,
separate from this patch.

**Dogfooded end-to-end after the fact, using the actual documented workflow**
with a *second*, independently-captured session (fresh Google sign-in, fresh
cookie capture) rather than reusing the first verification's state:

5. `browser-use cookies export` -- a realistic stand-in for "any cookie-export
   tool," per the docs' own wording -- produces a **bare top-level JSON array**
   of cookie objects, not Playwright's `storage_state()` shape
   (`{"cookies": [...], "origins": [...]}`). `loadSessionCookiesFromFile` only
   handled the wrapped shape, so a very common export format (also matches
   Puppeteer's `page.cookies()` and most DevTools cookie-export browser
   extensions) failed with a confusing `cookies file is empty` -- the JSON
   parse into the wrapped struct fails silently, falls through to the
   raw-`Cookie:`-header parser, which naturally finds no `k=v` pairs in JSON
   array text. Fixed by trying the wrapped shape first, then falling back to
   a bare array of the same per-cookie fields, before giving up on JSON.
6. `scenes gaps --project <real-id>` reproducibly returned
   `"characters_missing_image": [""]` -- a CHARACTER entity with no
   `displayName` was still appended to the missing-image list, as a useless
   empty string. Fixed in `computeSceneGaps` to only append characters with a
   non-empty `DisplayName`; the character is still counted in the
   `characters` total.

`projects`, `project`, `scenes gaps`, and `drive import --tag-scene` were all
re-verified live end-to-end using this second, independent capture, and
`sync --resources projects` / `export projects` were re-confirmed to fail
with the exact same, expected `BAD_REQUEST` (missing `input` envelope) --
matching the documented gap precisely, not some new failure mode.

**Reviewed and hardened afterward** (no git history existed yet to run the
usual diff-based review against, so this was a direct manual pass covering
correctness, security, testing, and maintainability):

7. **`auth login --cookies-file` imported every cookie for the domain, not
   just the ones needed.** A real storage-state export for labs.google also
   carries `email`/`EMAIL` (the user's own email address, HttpOnly) and three
   Google Analytics `_ga*` cookies alongside the actual auth cookies. All of
   it was getting persisted to `session.json` for no functional benefit.
   Fixed with an explicit allowlist (`nextAuthCookieNames`:
   `__Secure-next-auth.session-token`, `__Host-next-auth.csrf-token`,
   `__Secure-next-auth.callback-url`) applied in both the JSON-shape and
   raw-`Cookie:`-header import paths.
8. **The Bearer token was sent to labs.google too, untested.** Every request
   unconditionally attached `Authorization: <Bearer token>` when one was
   cached, regardless of target host -- so a user with *both* credentials
   configured (the documented normal end-state) would send the
   aisandbox-pa-scoped token alongside the labs.google cookie on every
   request there, a combination never actually exercised live. Fixed by
   gating both the live-request header attachment and the `--dry-run`
   preview on `requiresSessionBearerToken(targetURL)`, symmetric with the
   auth-check gating already in place -- removes the untested-combination
   risk architecturally instead of requiring a live re-test.
9. **Zero unit test coverage on any of this.** Everything above (and the
   original four items) had been verified entirely by live dogfooding against
   a real account -- real and valuable, but not durable or CI-checkable.
   Added: `internal/client/session_handshake_test.go`
   (`requiresSessionBearerToken`, `SessionManager.HasCookieFor`, including
   nil-jar and malformed-URL edge cases), `internal/cli/auth_test.go`
   (`loadSessionCookiesFromFile` across the wrapped/bare-array/raw-header
   shapes, domain filtering, and the new name allowlist), a case in
   `scenes_gaps_test.go` for the empty-`DisplayName` fix, and
   `internal/cli/resource_paths_test.go` locking in the
   `result.data.json.result.projects` response path against a synthetic
   payload shaped like the real one.
10. **Pagination for `projects`/`sync --resources projects` was flagged as
    "unverified," then resolved by reading the code rather than needing more
    live data.** `resourceReadConfigs["projects"].cursorParam` is `""`, and
    `extractPaginationFromEnvelope` short-circuits to `("", false)`
    immediately whenever `cursorParam` is empty -- so `--all`/pagination
    continuation is a documented no-op for this resource by design, not a
    silent truncation risk. This is the same tRPC input-envelope gap already
    flagged (finding 5 above) surfacing a second way: even if `hasMore`/
    `cursor` extraction worked, there's no way to build the next request's
    `input=` envelope with an updated cursor anyway. `TestProjectsPagination
    ContinuationIsANoOpByDesign` locks in this behavior directly rather than
    asserting an unverifiable assumption about where a real multi-page
    response nests `cursor`/`hasMore`.
11. **Root-cause note for retro, not a fix:** the *original* traffic capture
    (`discovery/traffic-analysis.json`) already correctly recorded
    `project.searchUserProjects` as `"observed_auth": ["cookie"]`. The
    correct auth classification was captured from the start -- it got
    collapsed into the single `session_handshake`/Bearer-token auth type
    somewhere in the generation pipeline, not lost at capture time. Worth a
    Printing Press retro: multi-endpoint APIs whose traffic capture already
    distinguishes auth mechanisms per-endpoint shouldn't get flattened to one
    auth type in the generated spec.

## Reprint guard

On reprint, preserve:
- `auth login --cookies-file` importing to `"https://labs.google"` with
  `loadSessionCookiesFromFile(cookiesFile, "labs.google", nextAuthCookieNames)`
  (not `aisandbox-pa.googleapis.com`, and not without the name allowlist --
  see finding 7), and its success/status output describing both auth
  surfaces independently (see `auth status`'s `labs_google_session` field
  and `SessionManager.HasCookieFor`).
- `nextAuthCookieNames` and `sessionCookieNameAllowed` in `auth.go`, applied
  in both `loadSessionCookiesFromFile`'s JSON-shape branch and
  `parseSessionCookieHeaderCookies`'s raw-header branch. Do not revert to
  importing every cookie matching the domain -- that re-introduces
  unnecessary PII (email) and analytics cookies into `session.json`.
- The `requiresSessionBearerToken(targetURL)` gate on the `Authorization`
  header attachment in *both* `doInternal`'s live-request path and
  `dryRun`'s preview -- not just the auth-check gates from finding 2. Do not
  let the header attachment go back to unconditional; that reintroduces the
  untested cross-surface-credential combination from finding 8.
- `requiresSessionBearerToken` in `internal/client/client.go`, and both call
  sites gating on it. Do not go back to unconditionally requiring
  `authHeader != ""` for every request -- that breaks the entire
  cookie-only surface.
- The `result.data.json.result.projects` response path in all three places
  (`promoted_projects.go`, `resource_paths.go`, `sync.go`) and the absolute
  `https://labs.google/fx/api/trpc/project.searchUserProjects` URL in the
  latter two. Do not revert to a relative `/project.searchUserProjects` path
  -- that silently resolves against the wrong `BaseURL`.
- Do not attempt to "fix" `sync --resources projects` / `export projects` by
  copying `promoted_projects.go`'s hardcoded default `--input` value into the
  generic engine as a one-off hack; that's a symptom of the engine's real
  gap (no tRPC input-envelope support), not a one-line fix. Treat it as a
  separate, unresolved limitation.
- `loadSessionCookiesFromFile` in `internal/cli/auth.go` tries Playwright's
  `{"cookies": [...]}` shape first, then falls back to a bare top-level JSON
  array of the same per-cookie fields (`jsonCookieEntry`), before falling
  back further to raw-`Cookie:`-header parsing. Do not narrow this back to
  the wrapped shape only -- a bare array is what `browser-use cookies
  export`, Puppeteer, and most DevTools cookie-export extensions actually
  produce, and was confirmed to fail with a confusing "cookies file is
  empty" before this fix.
- `computeSceneGaps` in `internal/cli/scenes_gaps.go` only appends a
  character to `characters_missing_image` when its `DisplayName` is
  non-empty (still counts it in `characters` either way). Reproduced live
  twice against the same real project: a CHARACTER entity with no display
  name was polluting the list with a useless `""` entry.
- The four new/extended test files (`internal/client/session_handshake_test.go`,
  `internal/cli/auth_test.go`, `internal/cli/resource_paths_test.go`, and the
  added case in `internal/cli/scenes_gaps_test.go`). These are the only
  regression net on this entire feature; a reprint that drops them silently
  returns to "verified by hand once, unguarded thereafter."
