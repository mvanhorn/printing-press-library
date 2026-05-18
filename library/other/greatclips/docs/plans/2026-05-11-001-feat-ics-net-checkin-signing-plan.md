---
title: ICS Net Check-In request signing for greatclips-pp-cli
type: feat
status: active
created: 2026-05-11
target_repo: ~/printing-press/library/greatclips
depth: standard
---

# feat: ICS Net Check-In request signing for greatclips-pp-cli

## Summary

v0.1 of `greatclips-pp-cli` ships every endpoint shape and the right
Authorization header, but live calls to `www.stylewaretouch.net` return HTTP
500 because the server requires HMAC-signed query parameters that the v0.1
client does not produce. This plan ports the JS signing primitives from the
GreatClips SPA into Go, hooks them into the HTTP client for stylewaretouch
hosts only, and verifies the four ICS endpoints (`wait`, `checkin`, `status`,
`cancel`) work end-to-end against the live API.

A parallel concern, per-host JWT audience scope (webservices.greatclips.com
returns 401 with a stylewaretouch-scoped token), is captured as a separate
unit gated on whether the user wants the customer-profile endpoint to work
in v0.2.

## Problem Frame

### What we know (from the 2026-05-11 browser-sniff session)

The signing scheme is fully captured in the SPA's JS chunk
`01000ffd9a85230f.js`. The complete recipe:

```js
const body = JSON.stringify([{storeNumber: "8991"}]);     // request body
const timestamp = new Date().getTime().toString();        // ms epoch as string
const sig = await generateICSSignature(`${timestamp}${body}`);
const url = `https://www.stylewaretouch.net/api/store/waitTime?t=${timestamp}&s=${sig}`;
// then: fetch(url, {method:"POST", body: body, headers:{"Content-Type":"application/json","Authorization":"Bearer <jwt>"}})
```

`generateICSSignature(input)` is built from four pure functions:

```
g(bytes)       = SHA-256(bytes), returned as byte array
y(key, msg)    = HMAC-SHA-256 with manual ipad/opad (0x36, 0x5c) and SHA-256
                 as the inner hash function
_(seed, label) = seed XOR HMAC-SHA-256("", label), 32 bytes
v(input)       = derived = _(SEED, "Online Check-In")
                 sig = HMAC-SHA-256(derived, input)
                 return base64url(sig)
```

`SEED` is a hardcoded 32-byte array in the JS bundle:

```
[-72, -109, 68, -63, 74, 27, -112, -69, -125, 48, -82, 82, 51, -104, 114, -20,
 -67, 107, -87, 42, -98, -13, -72, -88, -27, -23, -32, -79, -100, -31, -47, 76]
```

(Java signed-byte representation; cast to uint8 for Go.)

The response from a properly signed request is plain JSON via `a.json()` —
no client-side decryption. The hex-blob response observed via unsigned curl
is the server's error envelope when the signature is missing or invalid.

### What is broken in v0.1

The generated client at `internal/client/client.go` builds the POST body
correctly and attaches the Bearer header, but never appends `?t=<timestamp>&s=<signature>`
to the URL. The server rejects with 500 and a Java NullPointerException
leaked from `String.replace(char, char)` on the missing `s` parameter.

Three of the four ICS endpoints are affected the same way:
`POST /api/store/waitTime`, `POST /api/customer/checkIn`,
`POST /api/customer/cancel`. The `GET /api/customer/status` endpoint also
fails with 500 — likely the same signing requirement on GET URLs.

### Why this fix is worth doing

The user's two killer flows ("wait at Mercer Island", "add me + 3 kids to
the wait list") both terminate at stylewaretouch. Without signing, every
write operation and the wait-time read are inert. With signing, the v0.1
CLI becomes useful for its stated purpose.

The signing seed is hardcoded and publicly visible in a JS bundle served to
every visitor, so reproducing it in a Go client is replicating a published
contract, not bypassing a security control.

## Scope Boundaries

### In scope for v0.2

- Port the four JS primitives (`g`, `y`, `_`, `v`) to a Go package
- Compute `t` (timestamp) and `s` (signature) for every stylewaretouch.net
  request
- Hook the signer into the existing HTTP client by host
- Live-verify all four stylewaretouch endpoints (wait, checkin, status, cancel)
- Document the v0.2 auth flow in README and SKILL.md

### Deferred to Follow-Up Work

- **Per-host JWT audience scope** (v0.3): `webservices.greatclips.com` returns
  401 with a stylewaretouch-scoped JWT. The SPA fetches different tokens per
  audience. Fixing this enables `customer profile` and `salons search` against
  the real API. Tracked as a separate unit in this plan (U5) but gated on user
  opt-in — the killer flows do not need it.
- **Auth0 silent token mint from Chrome cookies** (v0.3+): so the user does not
  have to paste a JWT each hour. Reads Chrome's encrypted cookie store, mimics
  the SPA's `getAccessTokenSilently()` call. Significant work.
- **The remaining 10 transcendence commands** (watch, drift, plan, compare,
  next-open, recommend, history, vs-typical, --favorite, auto-checkin
  --when-under). Still scoped in the v0.1 absorb manifest; deferred to v0.3.
- **wait array-wrap fix** is already shipped in v0.1's library directory
  (`internal/cli/promoted_wait.go`); no re-do needed.

### Outside this product's identity

- Repackaging or republishing the ICS Net Check-In service itself
- Any operation that would let an unauthenticated caller place check-ins
- Bypassing rate limits or fingerprint checks beyond what the SPA already does

## Requirements

| ID | Requirement |
|----|-------------|
| R1 | `greatclips-pp-cli wait --store-number 8991` against the live API returns parsed wait time, not 500 |
| R2 | `greatclips-pp-cli checkin ...` against the live API returns a check-in confirmation, not 500 |
| R3 | `greatclips-pp-cli status` and `cancel` work for an active check-in |
| R4 | Existing `--dry-run` output for stylewaretouch endpoints shows the signed URL with `?t=...&s=...` so users can verify before sending |
| R5 | The Go signer is deterministic and tested against at least one captured browser request as a golden vector |
| R6 | Signing only fires for stylewaretouch.net hosts — `webservices.greatclips.com` requests are not affected |
| R7 | The seed and derived key never appear in any log output, error message, or `--json` payload |

## Key Technical Decisions

### Signer is a sibling package, not a generated artifact

The signing logic goes in `internal/icssign/` rather than as a patch to the
generator-emitted client. `internal/client/client.go` carries a "DO NOT EDIT"
header and gets regenerated whenever someone runs `printing-press generate`
again. By keeping the signer in a sibling package and importing it from the
client's pre-request hook (a regenerable extension point), the signer
survives regeneration.

Why not in `internal/cliutil/`? `cliutil` is generator-reserved per the
Printing Press AGENTS.md ("Generator-reserved namespaces"). The signer needs
to be in a package the generator does not own.

### One pre-request hook in the client, not per-command patches

The client's `Do` loop already iterates per request. The minimum diff is to
add a hook function variable (`PreRequestHook func(*http.Request)`) populated
at construction time from a sibling registry that knows about stylewaretouch.
The hook reads request body, computes signature, mutates `req.URL.RawQuery`.
Pre-existing query params (if any) are preserved.

This is one client edit (the hook field plus its invocation site) rather than
modifying every promoted command's body construction.

### Seed encoded as Go byte literal, not string-parsed at runtime

The JS bundle parses a comma-separated string. The Go port encodes the seed
as a `[32]byte{0xb8, 0x93, ...}` literal so there is no allocation, no parse
step, and no test for parse correctness needed. The 32 raw byte values are
written directly into source as hex.

### Timestamp source: `time.Now().UnixMilli()`

The JS uses `new Date().getTime()` which is millisecond epoch as a base-10
string. Go's `time.Now().UnixMilli()` returns int64 in the same units. We
format as `strconv.FormatInt(ms, 10)` for stable byte-for-byte equivalence
with the SPA.

### `--dry-run` shows the signed URL but masks the signature

Per R7, signatures must not appear in logs. The existing `--dry-run` output
will be updated to show the URL shape with `?t=<timestamp>&s=****` to make
the signing visible without exposing the actual signature value. Same masking
convention the client already uses for Authorization headers.

## High-Level Technical Design

```
                ┌────────────────────────┐
   command ─────▶ client.Do(req)         │
                │                        │
                │  PreRequestHook?       │
                │    │                   │
                │    ▼                   │
                │  icssign.Sign(req)     │  (only for stylewaretouch.net)
                │    │                   │
                │    │  reads body       │
                │    │  computes:        │
                │    │    t = unix ms    │
                │    │    s = HMAC(...)  │
                │    │  appends:         │
                │    │    ?t=...&s=...   │
                │    ▼                   │
                │  req.URL.RawQuery+="..." │
                │                        │
                │  http.Client.Do(req)   │
                │                        │
                │  return resp.json()    │
                └────────────────────────┘
```

Directional only — implementer should treat the boundaries as guidance and
adjust the hook signature, registration mechanism, and signer API as the
concrete code shape demands.

## Output Structure

```
~/printing-press/library/greatclips/
  internal/
    icssign/                    # NEW
      seed.go                   # 32-byte seed literal
      sign.go                   # HMAC-SHA256, key derivation, Sign(input) string
      sign_test.go              # primitives + golden vector tests
    client/
      client.go                 # MODIFIED: PreRequestHook field + invocation
    cli/
      icssign_hook.go           # NEW: wires icssign into client for stylewaretouch hosts
      promoted_wait.go          # MODIFIED: ensure --dry-run shows signed URL
  README.md                     # MODIFIED: v0.2 auth & signing section
  SKILL.md                      # MODIFIED: same
  docs/plans/
    2026-05-11-001-feat-ics-net-checkin-signing-plan.md   # this plan
```

## Implementation Units

### U1. Port HMAC primitives to Go

**Goal:** Pure-Go implementation of `g`, `y`, `_`, `v` that produces
byte-for-byte identical output to the JS bundle for the same input.

**Requirements:** R5, R7

**Dependencies:** none

**Files:**
- `internal/icssign/seed.go` (new) — 32-byte literal
- `internal/icssign/sign.go` (new) — primitives and `Sign(input string) string`
- `internal/icssign/sign_test.go` (new) — table-driven tests

**Approach:**
- `seed.go` exports `var seed = [32]byte{0xb8, 0x93, 0x44, 0xc1, 0x4a, 0x1b, 0x90, 0xbb, 0x83, 0x30, 0xae, 0x52, 0x33, 0x98, 0x72, 0xec, 0xbd, 0x6b, 0xa9, 0x2a, 0x9e, 0xf3, 0xb8, 0xa8, 0xe5, 0xe9, 0xe0, 0xb1, 0x9c, 0xe1, 0xd1, 0x4c}` (converted from JS signed-byte form; verify against the bundle once)
- `sign.go` uses `crypto/sha256` and `crypto/hmac` from stdlib. No third-party crypto deps.
- `deriveKey()` returns `seed XOR HMAC-SHA256(empty_key, "Online Check-In")` (the JS `_(seed, "Online Check-In")` flow)
- `Sign(input string) string` returns base64url-no-padding of `HMAC-SHA-256(derivedKey, input)`
- Use `base64.URLEncoding.WithPadding(base64.NoPadding)` for the URL-safe variant matching JS `.replaceAll("+","-").replaceAll("/","_")` with no `=` padding
- Cache `deriveKey()` result in a `sync.Once`-protected package var; it never changes

**Patterns to follow:** existing pure-Go internal package conventions in the
codebase; `cliutil` shape is a good reference (single-purpose package, no
external deps, fully tested).

**Test scenarios:**
- SHA-256 of a known input (e.g., `"abc"`) matches the standard NIST vector
- HMAC of `(key="key", msg="The quick brown fox jumps over the lazy dog")`
  matches the RFC 4231 test vector
- The derived key (`seed XOR HMAC("", "Online Check-In")`) has length 32 and
  has at least one byte different from `seed` and from the HMAC output
  (sanity check that XOR happened)
- `Sign("1778520000000[{\"storeNumber\":\"8991\"}]")` produces a
  deterministic, non-empty, URL-safe base64 string of expected length (43
  chars for 32-byte HMAC-SHA-256, base64url-no-padding)
- **Golden vector:** Given a captured browser request (timestamp + body + signature),
  `Sign(timestamp + body)` reproduces the captured signature exactly. Capture
  one real (timestamp, body, signature) triple from a live browser call
  during U4 verification; bake it into the test as a frozen fixture.
- The seed and derived key are never logged; running tests with
  `GODEBUG=allocfreetrace` or similar does not surface either value

**Verification:** `go test ./internal/icssign/...` passes; golden vector test
proves byte-identity with the JS implementation.

---

### U2. Add PreRequestHook to the generated HTTP client

**Goal:** Give the client a regen-stable extension point for per-request mutation.

**Requirements:** R1, R2, R3, R6

**Dependencies:** U1 (semantically, though U1 and U2 can be implemented in
parallel)

**Files:**
- `internal/client/client.go` (modify) — add `PreRequestHook func(*http.Request) error` field; invoke after URL composition, before `http.Client.Do`

**Approach:**
- Add a single field to the client struct
- After `req` is constructed and the auth header is set, but before `c.HTTPClient.Do(req)`, call the hook if non-nil
- If the hook returns an error, surface it as the request error
- The hook may freely mutate `req.URL.RawQuery`, headers, or body
- This is a 5-10 line patch to a file the generator owns. When the generator
  regens, the patch is lost — accepted cost for v0.2. U6 documents the
  regen-survival plan.

**Patterns to follow:** existing middleware-style hook patterns in the
client (the `Headers` map and `headerOverrides` already follow a similar
"per-request mutation" shape).

**Test scenarios:**
- A hook that sets a custom header on every request fires for both GET and POST
- A hook that returns an error short-circuits the request and surfaces the
  error to the caller
- A nil hook is a no-op (no panic, no extra latency)
- Existing tests for the client still pass (auth, headers, retries, dry-run)

**Verification:** `go test ./internal/client/...` passes; manual unit
exercise of a no-op hook returns the same response as no hook.

---

### U3. Wire icssign into the client for stylewaretouch.net hosts

**Goal:** Every outbound request to `www.stylewaretouch.net` gets `?t=&s=`
appended; other hosts are untouched.

**Requirements:** R1, R2, R3, R4, R6, R7

**Dependencies:** U1, U2

**Files:**
- `internal/cli/icssign_hook.go` (new) — `func NewICSSignHook() func(*http.Request) error`
- `internal/cli/root.go` or wherever the client is constructed (modify) — register the hook
- `internal/cli/promoted_wait.go` (modify) — update the `--dry-run` printer to mask the signature

**Approach:**
- The hook checks `req.URL.Hostname() == "www.stylewaretouch.net"` and
  short-circuits to no-op for anything else
- For POST/PUT/PATCH: read `req.Body`, buffer it (must replace `req.Body`
  with a fresh `io.NopCloser(bytes.NewReader(...))` after reading because
  the body is single-read)
- For GET/DELETE: signing input is just the timestamp string (no body)
- Compute `t = strconv.FormatInt(time.Now().UnixMilli(), 10)`
- Compute `s = icssign.Sign(t + body_string)`
- Build new RawQuery: existing query params (if any) plus `t=...&s=...` —
  use `url.Values` to compose cleanly
- For `--dry-run` output (the client already has a dry-run branch): print
  the URL with `s=****` substituted so the signature is not exposed in logs

**Patterns to follow:** the existing `dryRunOK(flags)` short-circuit at the
top of every command's RunE; the `maskToken` helper already used for
Authorization header masking.

**Test scenarios:**
- A POST to `www.stylewaretouch.net/api/store/waitTime` with body
  `[{"storeNumber":"8991"}]` gets `?t=<num>&s=<43-char-b64url>` appended
- A GET to `www.stylewaretouch.net/api/customer/status` gets `?t=<num>&s=<...>`
  appended (signing input is timestamp only)
- A POST to `webservices.greatclips.com/customer/salon-search/term` is
  unchanged — no `?t=&s=` appended (R6)
- A request with pre-existing query params (e.g.,
  `/api/customer/status?foo=bar`) retains `foo=bar` and appends the signing
  pair
- `--dry-run` for a `wait` command shows the URL with `s=****` masking
- Signature values do not appear in any error message when the request fails
  (network error simulated via `httptest.NewServer` returning 500)

**Verification:** unit tests above pass; `--dry-run` output for
`greatclips-pp-cli wait --store-number 8991 --dry-run` includes the
expected URL shape with masked signature.

---

### U4. Live verification against the real ICS endpoints

**Goal:** Prove the four stylewaretouch endpoints work end-to-end with a
real, current JWT.

**Requirements:** R1, R2, R3

**Dependencies:** U1, U2, U3

**Files:**
- `internal/icssign/sign_test.go` (modify) — add the captured golden vector
- `docs/plans/2026-05-11-001-feat-ics-net-checkin-signing-plan.md` (this
  file, modify) — append acceptance evidence as a sub-section

**Approach:** This unit is half-test, half-evidence-capture. The sequence:

1. **Re-capture a fresh JWT** via the Claude-in-Chrome flow used in v0.1
   (one-time token grab from a logged-in session, written to a 0600 file)
2. **Capture one signed request from the browser** for the golden vector —
   intercept `fetch`, log the URL's `?t=` and `?s=` values plus the body, copy
   them to disk. This becomes the golden test fixture in U1's test file.
3. **`wait` test:** `greatclips-pp-cli wait --store-number 8991 --json`
   should return parsed JSON `{stores: [{storeNumber: "8991", storeName:
   "Island Square", estimatedWaitMinutes: <int>, ...}]}`
4. **`status` test:** `greatclips-pp-cli status` against a no-active-check-in
   state should return a documented empty-state response (likely 404 or an
   empty body — verify the exact shape and surface honestly)
5. **`checkin` test:** With explicit user re-confirmation (real mutation!),
   `greatclips-pp-cli checkin --first-name Matt --last-name "Van Horn"
   --phone-number "(520) 907-6052" --salon-number 8991 --guests 4` should
   succeed with a documented success payload, AND a subsequent
   `greatclips-pp-cli status` should show the active check-in
6. **`cancel` test:** Immediately after #5, `greatclips-pp-cli cancel`
   should remove the check-in, AND a subsequent `status` should reflect that
7. **Failure tests:** verify a stale JWT produces a 401 with the existing
   helpful hint (not a 500 from the server), and verify an invalid
   signature (e.g., wrong seed) produces 500 (signing rejected) — this
   confirms the signing is what unlocks the endpoint

**Patterns to follow:** the existing v0.1 dry-run smoke pattern from the
prior shipcheck report.

**Test scenarios:** the seven live tests above. Each one named, with input,
action, and expected outcome documented.

**Test expectation:** these are real-network integration tests gated behind
an env var (e.g., `GREATCLIPS_LIVE_TEST=1`); they do not run in normal CI.

**Verification:** all seven tests pass; their outputs are summarized in this
plan's Acceptance Evidence section (added during U4 execution).

---

### U5. (Opt-in) Per-host JWT audience scope

**Goal:** Make `customer profile` and `salons search` work against the real
API by minting a webservices-audience JWT in addition to the stylewaretouch
one.

**Requirements:** none of R1-R7 directly — this enables a feature outside
the killer-flow scope. Gated on user choice during plan execution.

**Dependencies:** U1, U2 (the same client hook can mint both audience tokens)

**Files:**
- `internal/icssign/audience.go` (new) — typed audience enum
  (`AudWebservices`, `AudStyleware`)
- `internal/cli/icssign_hook.go` (modify) — pick the right token per host
- `internal/config/config.go` (modify) — add `WebservicesToken` field
  alongside `GreatclipsToken`; populate from `GREATCLIPS_WEBSERVICES_TOKEN`
  env var
- `internal/cli/auth_login.go` (modify) — accept `--audience webservices` or
  similar flag on token-paste

**Approach:** The SPA calls `getAccessTokenSilently({audience: "..."})` with
different audience values. We do not replicate the silent flow in v0.2 —
just accept two paste-in tokens (one per audience) and dispatch by request
host. The naming in the Auth0 tenant is unknown without further capture;
U5 starts by sniffing the SPA's two `/oauth/token` request bodies to learn
both audience strings, then plumbs them.

**Patterns to follow:** the existing `GREATCLIPS_TOKEN` env-var convention;
v0.1's auth subcommands (`auth set-token`, `auth status`, `auth logout`).

**Test scenarios:**
- A request to `webservices.greatclips.com` uses the webservices token,
  not the stylewaretouch token
- A request to `www.stylewaretouch.net` continues to use the stylewaretouch
  token (no regression)
- `auth status` shows the presence/absence of both tokens with appropriate
  masking
- `doctor` reports `Auth: ok` only when at least the relevant token is
  configured for the host the user is about to call

**Verification:** `greatclips-pp-cli customer profile --json` returns the
parsed profile object (firstName, email, favorited salons, etc.); existing
stylewaretouch tests from U4 still pass.

---

### U6. Documentation and regeneration-survival notes

**Goal:** Future maintainers (and future regen runs) understand what
landed and where the patches live.

**Requirements:** indirect — supports R4 (visible signed URL in dry-run is
documented), and supports keeping U2's client patch from getting silently
overwritten

**Dependencies:** U1, U3, optionally U5

**Files:**
- `README.md` (modify) — add an "Auth (v0.2)" section explaining the JWT
  paste flow and the automatic signing for stylewaretouch endpoints
- `SKILL.md` (modify) — same content, agent-shaped
- `docs/REGEN-NOTES.md` (new) — short note: "When regenerating, U2's
  PreRequestHook field must be re-added to `internal/client/client.go`.
  Consider upstreaming the hook into the generator template."
- `research.json` (modify) — update narrative.troubleshoots to reflect v0.2
  reality (signing now happens automatically; the v0.1 "wait body should
  be a JSON array" troubleshoot can be removed)

**Approach:** Write the docs to match what v0.2 actually does, not what
v0.1 promised. Specifically remove the "v0.1 emits cookie-aware request
shapes but does not yet attach cookies on the wire" language and replace
with "v0.2 signs every stylewaretouch request automatically; paste a JWT
once per session."

**Patterns to follow:** existing README/SKILL structure from v0.1.

**Test scenarios:** documentation; no behavioral test. Verified by reading.

**Test expectation:** none -- documentation unit, no behavior to assert.

**Verification:** README and SKILL render cleanly; a fresh reader can
follow the auth flow without prior context.

---

## System-Wide Impact

- **Generated client**: U2 patches a "DO NOT EDIT" file. Documented in U6
  as a regen-survival concern. The right long-term home for the hook is in
  the Printing Press generator template, but that is out of scope here.
- **Existing v0.1 commands**: `auth login`, `auth set-token`, `doctor`, all
  framework commands continue to work unchanged. The signing hook is a
  no-op for non-stylewaretouch hosts (R6).
- **MCP server**: the embedded MCP server uses the same client, so MCP tool
  invocations for `wait`, `checkin`, `status`, `cancel` will also start
  working live after this plan lands.
- **Verify/shipcheck**: existing shipcheck verdict (6/6 legs PASS,
  scorecard 58/100) should not regress. The new package adds a small
  amount of dead-code-detector surface but is fully tested.

## Risks

- **Seed value misread from JS**: if the 32-byte literal is off by a single
  bit, every signature is wrong and U4's golden vector test catches it
  immediately. Mitigation: U1 verifies the seed byte-for-byte against the
  bundle as the first step.
- **JWT lifetime**: Auth0 access tokens typically expire in 60 minutes. U4
  must capture a fresh token; live tests cannot be deterministic across
  long delays. Mitigation: U4 documents this honestly; v0.3 (silent token
  mint) closes the gap.
- **ICS rotates the seed**: if GreatClips redeploys with a new SEED, every
  v0.2 install breaks at once. Mitigation: this is a known risk of any
  reverse-engineered API and is documented in README under "Known Risks".
  No mitigation in scope here.
- **Rate limiting**: U4's live tests exercise the real API. Pacing matters.
  Mitigation: U4 runs sequentially with brief delays between tests; not in
  parallel.
- **`String.replace` server-side bug**: the 500 response with a leaked Java
  stack trace suggests the ICS server is not hardened. We avoid triggering
  edge cases by sending only well-formed bodies that the SPA also produces.
- **Real-mutation tests in U4**: `checkin` puts the user on a real
  waitlist. Mitigation: U4 explicitly re-confirms with the user before
  step 5 and pairs each `checkin` with an immediate `cancel` (step 6).

## Verification

The plan is complete when:

- `go test ./internal/icssign/...` and `go test ./internal/client/...`
  both pass
- All seven U4 live tests pass and their outputs are appended to this plan
  under an "Acceptance Evidence" section
- `greatclips-pp-cli doctor` continues to report config and auth status
  honestly
- `printing-press shipcheck` continues to exit 0 with no new failures
- README and SKILL accurately describe the v0.2 auth+signing flow

The killer flow ("how long is the wait at Mercer Island, and add me + 3
kids to the list") is reachable in one command sequence:

```
greatclips-pp-cli wait --store-number 8991
greatclips-pp-cli checkin --first-name Matt --last-name "Van Horn" \
  --phone-number "(520) 907-6052" --salon-number 8991 --guests 4
greatclips-pp-cli status
```
