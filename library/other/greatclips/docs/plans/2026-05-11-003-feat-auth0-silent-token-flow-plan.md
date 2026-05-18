---
title: Auth0 silent token flow port for greatclips-pp-cli
type: feat
status: active
created: 2026-05-11
target_repo: ~/printing-press/library/greatclips
depth: standard
---

# feat: Auth0 silent token flow port — CLI mints its own per-audience JWTs

## Summary

v0.3 ports the GreatClips SPA's Auth0 silent-auth flow into the CLI so
`greatclips-pp-cli` can mint a fresh, audience-correct access token on
its own, with no browser-side help, using only the HttpOnly session
cookies in the user's Chrome cookie store.

This unblocks live `wait`, `checkin`, `status`, `cancel`, and
`customer profile` calls — the exact set v0.2 ships request shapes for
but can't authenticate today.

Target: roughly 200 lines of new Go in `internal/auth0silent/` plus
small additions to config and client wiring. No third-party crypto
deps; stdlib only.

---

## Problem Frame

### What we know from the v0.2 investigation

The GreatClips SPA calls Auth0 at `https://cid.greatclips.com` and
performs a "silent" token refresh by calling `/authorize?prompt=none`
with three Auth0 HttpOnly cookies attached. Auth0 returns a 302 with
the access token in the URL fragment of the redirect_uri. The SPA
parses the fragment and uses the token.

Critically, **Auth0 mints a different token per `audience` parameter**.
The SPA fetches at least three:

| Audience (confirmed via captured JWT) | Backend it unlocks |
|--|--|
| `https://webservices.greatclips.com/customer` | `salons search`, `salons get`, `geo`, `hours` |
| `https://webservices.greatclips.com/cmp2` (suspected) | `customer profile` |
| `https://www.stylewaretouch.net` or similar (suspected) | `wait`, `checkin`, `status`, `cancel` |

The exact audience strings for the cmp2 and stylewaretouch endpoints
are unverified; **first unit of this plan is discovering them**.

### What we already have

- **Chrome cookie decryption proven** (Python; needs Go port). PBKDF2-HMAC-SHA1
  with 1003 iterations, salt `saltysalt`, 16-byte AES-128-CBC key,
  16-space IV, with a 32-byte SHA-256-of-hostkey prefix skipped on
  decode. macOS Keychain password lives at service name `Chrome` and
  is fetched via `security find-generic-password -wa "Chrome"`.
- **Cookies extracted in v0.2 debug**: `auth0`, `auth0_compat`, `did`,
  `did_compat` on `cid.greatclips.com` (all HttpOnly, all the right
  scope).
- **The four endpoint URLs are correct** (verified by salons search
  succeeding live with the captured webservices token).
- **Signing port is correct** (verified golden vector against the
  SPA's `generateICSSignature`).

### What's blocking

Only one thing: getting the right JWT for the host being called.

---

## Scope Boundaries

### In scope for v0.3

- Discover the audience strings for cmp2 and stylewaretouch endpoints
- Go implementation of Chrome cookie extraction on macOS
- Go implementation of the Auth0 silent-auth flow per audience
- Per-host token storage and routing in the CLI's config
- A `auth login --chrome` subcommand that performs the full ceremony
  once and writes tokens to disk
- Auto-refresh: when a request returns 401, mint a fresh token for
  that audience using cached cookies and retry once
- Live end-to-end test: `wait`, `checkin`, `status`, `cancel`,
  `customer profile` against the real API

### Deferred to Follow-Up Work

- Linux / Windows Chrome cookie extraction (macOS only in v0.3; other
  platforms get a clear "paste your token via `auth set-token`" error)
- Firefox / Safari / other-browser cookie extraction
- Refresh-token-only flow (silent auth handles refresh implicitly by
  re-running `/authorize` with the cookies; we do not need to parse or
  store the Auth0 refresh token)
- A first-class background daemon that pre-refreshes before expiry
  (auto-refresh on 401 is enough for v0.3)

### Outside this product's identity

- Any flow that requires the user to enter a password or 2FA into the
  CLI. The CLI only reads existing session cookies the user established
  through their normal browser login.
- Captcha solving, anti-bot evasion, or any work to defeat Auth0's
  security boundaries. If Auth0 starts requiring an interaction (the
  silent flow returns `login_required`), the CLI surfaces the error
  clearly and tells the user to log in via their browser.

---

## Requirements

| ID | Requirement |
|----|-------------|
| R1 | `greatclips-pp-cli auth login --chrome` extracts the relevant HttpOnly cookies from the user's macOS Chrome cookie store and writes them encrypted-at-rest into config |
| R2 | The CLI mints a separate access token per audience by calling Auth0's `/authorize?prompt=none&audience=<x>` and parsing the token from the redirect Location header's fragment |
| R3 | Each minted token is stored in config with its `exp` claim parsed and used to decide refresh timing |
| R4 | When a request returns 401, the CLI mints a fresh token for that audience using cached cookies and retries the request once. A second 401 surfaces honestly to the user with a hint to re-run `auth login --chrome` |
| R5 | `wait --store-number 8991` returns parsed wait time JSON against the live API |
| R6 | `checkin --first-name Matt --last-name "Van Horn" --phone-number ... --salon-number 8991 --guests 4` succeeds against the live API and returns a parsed response |
| R7 | `status` and `cancel` work end-to-end for an active check-in |
| R8 | `customer profile` returns the parsed profile object against the live cmp2 endpoint |
| R9 | The Chrome keychain password, decrypted cookies, and minted tokens never appear in logs, `--json` output, error messages, dry-run output, or any committed artifact |

---

## Key Technical Decisions

### Cookie storage: encrypted-at-rest in the config file, not env vars

Tokens rotate every ~60 minutes; cookies are long-lived (typically 30
days). Env vars are wrong for both because they leak through `ps`
output and shell history.

The config file at `~/.config/greatclips-pp-cli/config.toml` is
already 0600. We encrypt the cookie blob with a key derived from the
macOS Keychain itself (same approach Chrome uses) so a stolen config
file alone is not enough to replay. Tokens go beside the cookies, same
encryption.

### Audience discovery: read from JS bundle, verify via DevTools capture

The audience strings are constants in the SPA's JS bundle. They're
also visible as URL parameters every time `/authorize` is called. The
first implementation unit runs both checks in parallel and reconciles
— pasting an `/authorize` URL from the user's DevTools is the
quickest path to ground truth.

### Silent auth as a single Go function, not a state machine

`POST cid.greatclips.com/oauth/token` is the underlying call but the
SPA wraps it via `/authorize?response_type=token` because that's the
shape Auth0's JS SDK uses. We follow the SPA pattern:

1. Build URL: `https://cid.greatclips.com/authorize?client_id=X&audience=Y&prompt=none&response_type=token&redirect_uri=Z&scope=openid&state=<random>&nonce=<random>`
2. GET it with Cookie header set to the four Auth0 cookies, with
   `http.Client.CheckRedirect` set to return `http.ErrUseLastResponse`
   so we receive the 302 instead of following it
3. Parse `Location` response header
4. Extract `access_token` from the URL fragment
5. Parse JWT payload for `exp`

One function, ~50 lines.

### Per-host token routing in the client's existing PreRequestHook

v0.2 added a single `PreRequestHook` to the client. v0.3 generalizes
it slightly: the hook reads the request URL's host, picks the right
token from a per-host map in config, and sets the Authorization
header before signing (which only fires on stylewaretouch hosts
anyway). The existing icssign hook stays.

### 401 retry path is built in, not a feature flag

Auth0 access tokens expire every ~60 minutes by default. A long
session would hit 401s constantly without auto-refresh. We refresh on
the first 401, retry once, and bubble a clean error to the user on a
second 401. This isn't optional v0.4 work; it's part of "live
check-in works from the CLI."

### Failure mode: `login_required` from Auth0

If the user's Auth0 session has expired (cookies are still present
but server-side session is gone), `/authorize?prompt=none` returns a
302 to `error=login_required` instead of an access token. We detect
this, surface it clearly, and tell the user to log in via
`https://app.greatclips.com` and re-run `auth login --chrome`. We do
not attempt to silently re-authenticate via password.

---

## High-Level Technical Design

```
                                ┌──────────────────────┐
   auth login --chrome  ──────▶ │ Chrome cookie store  │  (macOS SQLite +
                                │ + macOS Keychain     │   Keychain-derived
                                └──────────┬───────────┘   PBKDF2 + AES key)
                                           │ decrypt
                                           ▼
                                ┌──────────────────────┐
                                │ Auth0 cookies        │
                                │ (auth0, did, etc.)   │
                                └──────────┬───────────┘
                                           │
                       ┌───────────────────┼────────────────────┐
                       │                   │                    │
                       ▼                   ▼                    ▼
              for each audience: GET https://cid.greatclips.com/authorize
                           ?prompt=none&audience=<aud>&response_type=token
                       │                   │                    │
                       ▼                   ▼                    ▼
                  Location:                                Location:
                  redirect_uri#                            redirect_uri#
                  access_token=...                         access_token=...
                       │                   │                    │
                       ▼                   ▼                    ▼
              ┌──────────────────────────────────────────────────────┐
              │ Config (encrypted):                                  │
              │   cookies: {auth0, did, ...}                         │
              │   tokens:  {                                         │
              │     "webservices.greatclips.com/customer": {jwt, exp},│
              │     "webservices.greatclips.com/cmp2":     {jwt, exp},│
              │     "www.stylewaretouch.net":               {jwt, exp},│
              │   }                                                  │
              └──────────────────────────────────────────────────────┘
                                           │
                              client.PreRequestHook
                                           │
                                           ▼
                           Match req.URL.Hostname() to a stored token
                           Set Authorization: Bearer <token>
                           If stylewaretouch host: also sign (v0.2 icssign)
                           On 401: refresh THIS audience, retry once
```

This illustrates the intended approach and is directional guidance
for review, not implementation specification. The implementing agent
should treat it as context, not code to reproduce.

---

## Output Structure

```
~/printing-press/library/greatclips/
  internal/
    auth0silent/                          # NEW
      audiences.go                        # discovered audience constants
      cookies_darwin.go                   # macOS Chrome cookie extraction
      cookies.go                          # cross-platform interface + stubs
      silent.go                           # /authorize call, Location parser
      jwt.go                              # parse exp claim
      audiences_test.go
      cookies_darwin_test.go
      silent_test.go
    config/
      config.go                           # MODIFIED: encrypted cookies + per-audience tokens
    cli/
      icssign_hook.go                     # MODIFIED: read token per host
      auth_login_chrome.go                # NEW: subcommand
  docs/plans/
    2026-05-11-003-feat-auth0-silent-token-flow-plan.md   # this plan
```

---

## Implementation Units

### U1. Discover the three audience strings

**Goal:** Three confirmed audience strings written to
`internal/auth0silent/audiences.go` as named constants.

**Requirements:** R2

**Dependencies:** none

**Files:**
- `internal/auth0silent/audiences.go` (new)
- `internal/auth0silent/audiences_test.go` (new)

**Approach:**
- The webservices/customer audience is confirmed:
  `https://webservices.greatclips.com/customer`.
- For cmp2 and stylewaretouch, run both checks in parallel:
  1. Grep the SPA's JS chunk `01000ffd9a85230f.js` for `audience`
     string literals near the relevant URL patterns.
  2. Have the user open DevTools, click around (favorites,
     check-in, profile), and copy any `/authorize?...` URL from the
     network tab. The `audience` query param is in the URL.
- Pin each as a const with a constant for the corresponding host
  prefix so the routing table is one map.
- The test asserts the string format (HTTPS URLs, no trailing slash,
  no whitespace).

**Patterns to follow:** `internal/icssign/seed.go` carries the SPA's
hardcoded HMAC seed as a 32-byte const literal — same pattern: a
small file with one or two constants and a comment explaining when
to rotate them.

**Test scenarios:**
- Each audience constant is a non-empty HTTPS URL.
- No audience contains a `?`, `#`, or trailing `/`.
- A small lookup map keyed by hostname returns the right audience
  for each of `webservices.greatclips.com`,
  `webservices.greatclips.com` (with path-based dispatch for cmp2
  vs customer if needed), and `www.stylewaretouch.net`.

**Verification:** `go test ./internal/auth0silent -run TestAudiences`
passes. Discovery evidence is recorded in this plan's Acceptance
Evidence section (added during execution).

---

### U2. Chrome cookie extraction on macOS

**Goal:** Go function `ExtractAuth0Cookies(host string) (map[string]string, error)`
that returns the four Auth0 cookies for `cid.greatclips.com` from the
user's default Chrome profile, decrypted in-memory.

**Requirements:** R1

**Dependencies:** none

**Files:**
- `internal/auth0silent/cookies.go` (new) — cross-platform interface
- `internal/auth0silent/cookies_darwin.go` (new) — macOS implementation
- `internal/auth0silent/cookies_other.go` (new) — non-Darwin stub that
  returns a clear "platform not supported in v0.3" error
- `internal/auth0silent/cookies_darwin_test.go` (new)

**Approach:**
- Read Chrome's cookie SQLite file at
  `~/Library/Application Support/Google/Chrome/Default/Cookies`
  (read-only, in a temp copy so we don't lock Chrome's database).
- Use `database/sql` with `modernc.org/sqlite` (pure Go, no CGO) for
  the query. Avoid `mattn/go-sqlite3` to keep the binary CGO-free.
- Fetch the macOS Keychain password by shelling to
  `security find-generic-password -wa Chrome` (no Go library for
  generic-password lookup that doesn't need CGO).
- Derive the AES key with PBKDF2-HMAC-SHA1 (`crypto/pbkdf2` since Go
  1.24, otherwise `golang.org/x/crypto/pbkdf2`): 1003 iterations, salt
  `saltysalt`, 16-byte output.
- For each cookie row matching `host_key = 'cid.greatclips.com'`,
  decrypt the `encrypted_value` blob (strip the 3-byte `v10` prefix,
  AES-128-CBC with 16-space IV, PKCS7 unpad, strip the 32-byte
  SHA-256-of-host-key prefix from the plaintext).
- Return a map of cookie name to plaintext value.

**Patterns to follow:** the working Python decryption pipeline used
during the v0.2 debug session (visible in the transcript) is the
authoritative reference for parameter values.

**Test scenarios:**
- **Happy path:** When run on a machine with Chrome installed and a
  logged-in greatclips.com session, returns at least the four cookies
  `auth0`, `auth0_compat`, `did`, `did_compat`, all non-empty.
- **Edge case:** Chrome not installed — returns a wrapped error that
  names the missing file path so the user can diagnose.
- **Edge case:** Keychain access denied (the OS prompts the user for
  permission to read Chrome's keychain item) — returns a clear error
  with the OS error message wrapped.
- **Edge case:** Cookie value with the `v10` prefix but malformed
  ciphertext (e.g., truncated) — returns a decode error, not a panic.
- **Error path:** Keychain lookup returns empty — clear error about
  Chrome not being set up or the keychain item being deleted.
- **Integration:** The full pipeline (Keychain → PBKDF2 → AES decrypt
  → SHA-256 prefix strip → return) is exercised end-to-end against
  a real Chrome database. Stub-only tests are insufficient because
  every step matters; one parameter wrong and every cookie is
  garbage.

**Verification:** `go test ./internal/auth0silent -run TestExtractAuth0Cookies`
passes against a real Chrome profile in CI-style integration test
(gated behind `GREATCLIPS_LIVE_TEST=1` env var).

---

### U3. Auth0 silent token mint

**Goal:** Go function `Mint(audience string, cookies map[string]string) (Token, error)`
that calls Auth0's `/authorize?prompt=none` and returns a parsed
access token with its expiry time.

**Requirements:** R2, R3

**Dependencies:** U2 (for the cookies it consumes)

**Files:**
- `internal/auth0silent/silent.go` (new)
- `internal/auth0silent/jwt.go` (new) — minimal `parseExp` helper
- `internal/auth0silent/silent_test.go` (new)

**Approach:**
- Build the URL: `https://cid.greatclips.com/authorize?client_id=<X>&audience=<aud>&prompt=none&response_type=token&redirect_uri=<redirect>&scope=openid&state=<random>&nonce=<random>`.
- `client_id` and `redirect_uri` are discoverable from the SPA's
  Auth0 SDK init call in the JS bundle (look for `domain:`,
  `clientId:`, `authorizationParams:` near a `cid.greatclips.com`
  string). Capture during U1's discovery.
- Use `crypto/rand` for state and nonce; both are required by the
  Auth0 protocol and validated server-side.
- Construct a `http.Client` with `CheckRedirect` returning
  `http.ErrUseLastResponse` so the 302 response body is returned to
  us instead of being followed.
- Set the `Cookie` header on the request manually (each `name=value`
  joined by `; `).
- On 302, parse `Location` header. The token is in the URL fragment
  (after `#`), URL-encoded. Pull `access_token`, `token_type`,
  `expires_in`.
- If `Location` contains `error=` instead of `access_token=`, parse
  the error and surface it. The two we expect: `login_required`
  (cookies are stale) and `consent_required` (the user revoked
  consent in their Auth0 dashboard).
- Decode the JWT's middle segment to extract `exp` as a Unix
  timestamp. (Just base64-decode + json-unmarshal `{"exp": int64}` —
  do not validate the signature; we trust the token because the
  Bearer is what we'll send.)
- Return a struct `{Token string, ExpiresAt time.Time, Audience string}`.

**Patterns to follow:** `internal/client/client.go` for the existing
`http.Client` construction and the conventions around error
wrapping.

**Test scenarios:**
- **Happy path:** Given a known audience and fake-but-valid Auth0
  cookies (from a test fixture or stubbed response), the function
  returns a token whose `aud` claim matches the requested audience
  and whose `ExpiresAt` is approximately 60 minutes in the future.
- **Edge case:** Auth0 returns `error=login_required` — the function
  returns a typed `*LoginRequiredError` so the caller can route to
  "tell the user to re-login" instead of generic error handling.
- **Edge case:** Auth0 returns `error=consent_required` — returns a
  typed `*ConsentRequiredError` with the same routing behavior.
- **Edge case:** Network failure mid-call — returns a wrapped error
  identifying the host so the user can diagnose.
- **Edge case:** `Location` header missing entirely (Auth0 returned
  200 with HTML, suggesting cookie auth failed and Auth0 is trying
  to render the login page) — returns a clear "authentication
  failed" error.
- **Error path:** Token in `Location` is malformed (e.g., not three
  dot-delimited parts) — returns a JWT parse error, not a panic.
- **Integration:** End-to-end against the real Auth0 tenant using
  cookies from U2. Run gated behind `GREATCLIPS_LIVE_TEST=1`.

**Verification:** `go test ./internal/auth0silent -run TestMint`
passes; the live integration test successfully mints a token for the
webservices/customer audience.

---

### U4. Per-audience token storage in config

**Goal:** Config can hold the encrypted Auth0 cookies plus a
per-audience token map keyed by audience string.

**Requirements:** R1, R3, R9

**Dependencies:** U2, U3

**Files:**
- `internal/config/config.go` (modify)
- `internal/config/config_test.go` (new if not present)

**Approach:**
- Add two new fields to `Config`:
  - `EncryptedCookies map[string]string` — cookie name → base64 of
    the encrypted blob. Keyed by cookie name so an attacker who
    steals the file can't immediately replay; the decryption key is
    fetched from macOS Keychain on each load.
  - `Tokens map[string]TokenEntry` — audience → token entry. The
    entry holds `Token string`, `ExpiresAt time.Time`, `MintedAt time.Time`.
- Helper methods on `Config`:
  - `SaveCookies(map[string]string) error` — encrypts and writes
  - `LoadCookies() (map[string]string, error)` — decrypts on read
  - `GetTokenForHost(host string) (string, bool)` — looks up the
    right audience for the host (using the table from U1) and returns
    the token if present and not within 60s of expiry; otherwise
    returns false
  - `SaveToken(audience string, t Token) error` — persists the new
    token
- Encryption: AES-256-GCM with a key derived from the same macOS
  Keychain entry Chrome uses. We do not invent our own keychain
  entry — we ride on the existing one so the user does not see a
  second permission prompt. The IV is per-blob, prepended to the
  ciphertext.

**Patterns to follow:** `internal/config/config.go` already
demonstrates TOML serialization and file-permission handling. The
existing `SaveTokens` / `ClearTokens` methods are the model.

**Test scenarios:**
- **Happy path:** `SaveCookies` then `LoadCookies` returns the exact
  same map.
- **Edge case:** Config file exists but `EncryptedCookies` is empty
  — `LoadCookies` returns an empty map and nil error (not "missing
  cookies" error).
- **Edge case:** Token within 60 seconds of expiry — `GetTokenForHost`
  returns `(_, false)` so the caller refreshes.
- **Edge case:** Token expired — same behavior, returns `(_, false)`.
- **Edge case:** Unknown host (e.g., `example.com`) —
  `GetTokenForHost` returns `(_, false)` with no error.
- **Error path:** Decryption key fetch fails (Keychain access
  denied) — error is surfaced clearly, not silently dropped.

**Verification:** `go test ./internal/config -run TestCookieStorage`
and `TestTokenStorage` pass. Token-routing test covers all three
known hosts.

---

### U5. Wire per-host token routing into the client's PreRequestHook

**Goal:** Every outbound API call from the CLI picks the right
Bearer token for the destination host. On 401, the hook refreshes
the token for that audience and retries once.

**Requirements:** R4, R5, R6, R7, R8

**Dependencies:** U1, U2, U3, U4

**Files:**
- `internal/cli/icssign_hook.go` (modify) — extend to also set the
  per-host Authorization header
- `internal/client/client.go` (modify) — add the 401 retry path
- `internal/cli/icssign_hook_test.go` (new)

**Approach:**
- The existing icssign hook fires for all hosts but only mutates the
  URL for stylewaretouch. v0.3 extends it: before signing, the hook
  also looks up the token for the request's host and sets
  `Authorization: Bearer <token>`. This replaces the v0.2 behavior
  where the client read from `config.AuthHeader()`.
- The 401 retry: when `c.HTTPClient.Do(req)` returns a response with
  status 401, the client invokes a hook-provided refresh callback
  (`RefreshTokenForHost(host string)`) which calls the silent-mint
  flow for that audience and updates config. The client then rebuilds
  the request (because body is single-read; need to use the buffered
  body from the icssign hook) and retries once. A second 401 surfaces
  as the existing 401 error.
- The refresh callback is a function pointer on the client struct,
  set by `newClient()` the same way the PreRequestHook is.

**Patterns to follow:** the existing v0.2 PreRequestHook invocation
in `internal/client/client.go`. The buffered body trick from the
icssign hook (`io.NopCloser(bytes.NewReader(...))` + `req.GetBody`)
is the model for retry-safe body handling.

**Test scenarios:**
- **Happy path:** A request to
  `https://webservices.greatclips.com/customer/salon-search/term`
  gets the customer-audience Bearer attached.
- **Happy path (cross-host):** A request to
  `https://www.stylewaretouch.net/api/store/waitTime` gets the
  stylewaretouch Bearer AND the v0.2 `?t=&s=` signing query params.
  Both hooks compose cleanly.
- **Edge case:** No token cached for the host — Authorization header
  is not set; request fires unauthenticated (probably 401, then
  refresh path engages).
- **Error/retry path:** Stale token returns 401; refresh succeeds;
  retry succeeds with the new token. Verified by a test that returns
  401 once and 200 the second time and asserts both Authorization
  values differ.
- **Error path:** Refresh itself fails (Auth0 returns
  `login_required`) — the second 401 surfaces to the caller with a
  clear "re-run `auth login --chrome`" hint.
- **Integration:** `wait`, `checkin`, `status`, `cancel`, and
  `customer profile` all work end-to-end against the real API.
  Verified by U7's live test matrix.

**Verification:** `go test ./internal/client ./internal/cli` passes.

---

### U6. `auth login --chrome` subcommand

**Goal:** A single CLI command that performs the full one-time
onboarding: extracts cookies, mints all three tokens, writes them to
config.

**Requirements:** R1, R2, R3, R9

**Dependencies:** U1, U2, U3, U4

**Files:**
- `internal/cli/auth_login_chrome.go` (new)
- `internal/cli/auth.go` (modify) — register the new subcommand
- `internal/cli/auth_login_chrome_test.go` (new)

**Approach:**
- Subcommand under the existing `auth` group: `greatclips-pp-cli auth login --chrome`.
- Behavior:
  1. Print "Extracting Chrome cookies for cid.greatclips.com..." to
     stderr so the user knows what's happening (the OS may prompt
     for Keychain access on the first run).
  2. Call `auth0silent.ExtractAuth0Cookies("cid.greatclips.com")`.
  3. Call `config.SaveCookies(cookies)`.
  4. For each of the three audiences, call `auth0silent.Mint(aud, cookies)` and
     `config.SaveToken(aud, token)`. Print "Minted token for <host>
     (expires in <duration>)" per audience.
  5. On any failure, print a clear error explaining which step
     failed and what the user can do. Specifically: cookie extract
     failure → "make sure Chrome is closed or try again with sudo";
     mint failure with `login_required` → "log in at
     https://app.greatclips.com and re-run".
- `--force` flag re-mints all tokens even if cached ones are still
  valid. Useful for debugging.
- Does NOT print the cookie values or tokens. The output is
  human-readable counts and durations only.

**Patterns to follow:** the existing `auth` subcommands in
`internal/cli/auth.go` (`set-token`, `status`, `logout`).

**Test scenarios:**
- **Happy path:** First-time invocation extracts cookies and mints
  all three tokens; subsequent `greatclips-pp-cli doctor` reports
  auth as configured.
- **Edge case:** Cookies extract but one audience mint fails (e.g.,
  Auth0 doesn't recognize that audience because the user revoked
  scope) — the other two tokens still get saved; the failed audience
  is reported clearly.
- **Edge case:** `--force` on an already-configured config —
  re-mints and overwrites existing tokens. Asserts the new tokens
  differ from the old ones.
- **Error path:** No Chrome installed — surfaces a clear error from
  U2 without panicking.

**Verification:** `greatclips-pp-cli auth login --chrome` succeeds
on a logged-in macOS machine and `greatclips-pp-cli doctor` reports
all auth slots populated.

---

### U7. Live verification matrix

**Goal:** Every commit-blocked v0.2 endpoint works live against the
real GreatClips and stylewaretouch APIs.

**Requirements:** R5, R6, R7, R8

**Dependencies:** U1 through U6

**Files:**
- `docs/plans/2026-05-11-003-feat-auth0-silent-token-flow-plan.md`
  (this file, modify) — append Acceptance Evidence section with the
  live response shapes

**Approach:** The test matrix mirrors v0.2's plan-U4 but extended
to the audience-blocked endpoints. The sequence:

1. `greatclips-pp-cli auth login --chrome` — onboarding works.
2. `greatclips-pp-cli doctor --json` — reports all three audience
   slots configured.
3. `greatclips-pp-cli customer profile --json` — returns Matt's
   actual profile data (firstName, email, favoriteSalons including
   8991). Proves cmp2-audience routing.
4. `greatclips-pp-cli salons search --term 98040 --radius 5 --json`
   — already works in v0.2; confirms no regression.
5. `greatclips-pp-cli wait --store-number 8991 --json` — returns
   `{stores: [{storeNumber: "8991", storeName: "Island Square",
   estimatedWaitMinutes: <int>, ...}]}`. Proves stylewaretouch
   audience + v0.2 signing both work end-to-end.
6. `greatclips-pp-cli status --json` — returns either an active
   check-in or an empty-state envelope. Proves stylewaretouch
   audience on a GET.
7. **User-confirmed:** `greatclips-pp-cli checkin --salon-number
   8991 --guests 4 ...` succeeds and `status` confirms the new
   check-in. Mutation gated on explicit user yes/no.
8. `greatclips-pp-cli cancel` removes the check-in; subsequent
   `status` confirms no active check-in.
9. Mid-flight 401 test: artificially set a token's `ExpiresAt` to
   the past, run any command, confirm the auto-refresh path fires
   and the request succeeds.

**Test scenarios:** the nine live tests above. Each one named, with
input, action, and expected outcome documented in the Acceptance
Evidence section after execution.

**Test expectation:** these are real-network integration tests gated
behind `GREATCLIPS_LIVE_TEST=1`; they do not run in default CI.

**Verification:** the user is actually checked in at Island Square
via the CLI, then cancelled via the CLI. Visible in the GreatClips
web UI throughout.

---

## System-Wide Impact

- **Existing v0.2 commands**: `salons search`, `salons get`, `hours`,
  `geo` continue to work unchanged after the audience routing change
  — they were already pointed at the customer audience.
- **MCP server**: the embedded MCP server uses the same client, so
  every MCP tool inherits the auth fix automatically. Particularly
  notable: agent integrations that previously failed silently on
  `wait`/`checkin` start working.
- **Generated client**: `internal/client/client.go` continues to
  carry the v0.2 patches plus the new 401 retry path. Document the
  retry path in `docs/REGEN-NOTES.md`.
- **macOS-only assumption**: v0.3 ships with a clear-error stub on
  Linux/Windows. Cross-platform expansion is its own follow-up plan.

---

## Risks

- **Auth0 client_id rotation**: if GreatClips reissues the SPA with a
  different Auth0 client_id, the silent-auth call fails with
  `invalid_client`. Mitigation: discover client_id during U1 by
  parsing the JS bundle; document the rotation procedure in
  REGEN-NOTES.md.
- **Audience strings rotate**: same risk as client_id. Same
  mitigation: U1 documents the discovery procedure.
- **Chrome database lock**: if Chrome is running, the cookie SQLite
  file is locked. Mitigation: copy the file to a temp location
  before opening (the file copy is allowed; only opening the
  original is locked). The Python proof-of-concept already used
  this pattern.
- **Chrome encryption format change**: Chrome has historically
  changed cookie encryption (v10 → v11 prefixes). If it changes
  again, decryption breaks. Mitigation: surface the error clearly
  ("unrecognized cookie format prefix"), don't try to silently fix.
- **macOS Keychain access prompt**: the user sees a permission
  prompt the first time. Mitigation: U6 prints clear messaging
  before triggering the read; the prompt is unavoidable and
  intentional.
- **Auth0 silent-flow rate limits**: Auth0 free tier has per-tenant
  rate limits. The CLI mints three tokens per `auth login --chrome`
  and one per audience-401-retry. Realistic CLI usage doesn't
  approach the limit, but a runaway test loop could. Mitigation:
  document the per-mint cost in the silent.go comments; do not loop
  on persistent 401s.
- **Token leakage via shell history**: the user might accidentally
  `echo` config contents. Mitigation: tokens are encrypted at rest;
  even reading the file gives ciphertext, not the JWT.
- **Refresh storms on multiple-concurrent-401s**: if the user runs
  N commands in parallel and all 401, all N would trigger refresh.
  Mitigation: use a per-audience `sync.Once`-style guard so only one
  refresh runs per audience at a time; the others wait and reuse the
  result.

---

## Verification

The plan is complete when:

- `go test ./internal/auth0silent/... ./internal/config/... ./internal/cli/...` passes
- The full nine-test live matrix in U7 succeeds end-to-end
- `greatclips-pp-cli doctor` reports all three audience slots configured
- The user can check in (Matt + 3 kids at salon 8991) via a single
  CLI command and cancel via a single CLI command
- README and SKILL are updated to describe the v0.3 auth onboarding
  (`auth login --chrome` once per ~30 days; token refresh is
  automatic)
- All captured tokens, cookies, and the Keychain-derived encryption
  key are scrubbed from `/tmp` after development

The killer flow is reachable, end-to-end, no SPA involvement:

```
greatclips-pp-cli auth login --chrome     # one time, ~30 days
greatclips-pp-cli wait --store-number 8991
greatclips-pp-cli checkin --first-name Matt --last-name "Van Horn" \
  --phone-number "(520) 907-6052" --salon-number 8991 --guests 4
greatclips-pp-cli status
greatclips-pp-cli cancel                  # optional
```
