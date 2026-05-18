---
title: Token exchange capture + CLI check-in
type: feat
status: active
created: 2026-05-11
target_repo: ~/printing-press/library/greatclips
depth: lightweight
---

# feat: Token exchange capture + CLI check-in

## The actual blocker (and why we missed it)

The captured customer-audience JWT from earlier sessions had:

```
iss: https://webservices.greatclips.com/customer
aud: https://webservices.greatclips.com/customer
```

`iss` (issuer) is `webservices.greatclips.com`, NOT Auth0
(`cid.greatclips.com`). That means GreatClips's backend mints those
JWTs itself by exchanging the Auth0-issued `/cmp` token for a
downstream token via some endpoint we haven't observed yet.

The v0.3 silent-mint port can only get the `/cmp` token directly
from Auth0 — that token is the seed for the exchange, not the
final credential.

**Until we capture the exchange endpoint, no amount of Go code
unlocks `checkin`.** Capturing it is a 10-minute DevTools task. We
should have done it on session one.

## Plan: 3 units, no fluff

### U1. Capture the token exchange endpoint

**Goal:** Identify the exact HTTP call the SPA makes to convert a
`/cmp`-aud Auth0 token into the `/customer`-aud and stylewaretouch
tokens.

**Files:** none — pure observation

**Approach:**
- Open `https://app.greatclips.com/` in Chrome, log out, log back in
  fresh so the full token-acquisition sequence is captured.
- Open DevTools Network tab. Filter to `XHR` or `Fetch`.
- Click a salon, then click Check In. This fires the full
  token-cascade.
- Identify any request whose response body contains a JWT (other
  than `cid.greatclips.com/authorize`). Candidates:
  - `webservices.greatclips.com/cmp/token-exchange` (hypothesis)
  - `webservices.greatclips.com/cmp/oauth/...`
  - Auth0 token-exchange grant at `cid.greatclips.com/oauth/token`
    with `grant_type=urn:ietf:params:oauth:grant-type:token-exchange`
- Copy: method, full URL, all request headers, request body, response
  body. Specifically need to know how the OUTPUT JWT differs from
  the INPUT.
- Repeat for stylewaretouch: when does the SPA mint THAT audience?
  Likely a second exchange call after the first.

**Verification:** A short text file at
`docs/exchange-capture-2026-05-11.md` (not committed) with:
  - The exchange URL pattern
  - Whether one call returns multiple audience tokens or each
    audience needs its own call
  - The exact input/output JWT audience pair(s)

If after 10 minutes of DevTools observation no second JWT is found
in any response body, the architecture hypothesis is wrong — fall
back to U1b.

### U1b. If no exchange endpoint exists, capture the actual SPA-side call shape

**Goal (fallback):** If the SPA isn't doing a token exchange, find
out what IS happening. Maybe the `/cmp` token works for everything
and the 401s we saw were from a different cause. Or maybe each
endpoint has its own JWT minted by a per-endpoint exchange.

**Approach:** Pull one successful request to
`webservices.greatclips.com/customer/salon-search/term` from the
Network tab. Inspect the Authorization header's JWT. Decode
the `aud` claim. Compare to the `/cmp` token we can mint. If `aud`
matches, our mint should already work and the 401 came from
something else (rate limit, missing header, stale token).

### U2. Replicate the exchange in Go

**Goal:** Add `Exchange(cmpToken string) (audToToken map[string]Token, error)`
to `internal/auth0silent/` that performs whatever U1 captured.

**Files:**
- `internal/auth0silent/exchange.go` (new)

**Approach:** Mirror U1's captured call exactly. Probably one POST
with the `/cmp` token in the Authorization header, returning a JSON
body with one or more downstream JWTs. The signature is whatever U1
discovered.

### U3. Wire and verify live check-in

**Goal:** `greatclips-pp-cli checkin --salon-number 8991 --guests 4 ...`
returns a real 200 response and Matt is on the Island Square
waitlist.

**Files:**
- `internal/cli/root.go` (modify `newClient` to call mint + exchange)
- `internal/cli/icssign_hook.go` (modify to attach per-host
  Authorization from a config map of audience→token)

**Approach:** On every command:
1. Read cached cookies from disk (or extract fresh via U2 of
   v0.3 plan).
2. Mint `/cmp` token if not cached.
3. Exchange `/cmp` → per-host tokens if not cached.
4. Attach the right token per host in the existing PreRequestHook.
5. On 401, refresh and retry once.

Then live-test sequence:
1. `greatclips-pp-cli status` → 200, no active check-in
2. **User confirmation gate**
3. `greatclips-pp-cli checkin --first-name Matt --last-name "Van Horn"
   --phone-number "(520) 907-6052" --salon-number 8991 --guests 4`
   → real 200, Matt on the waitlist
4. `greatclips-pp-cli status` → confirms the new check-in
5. Ask user: leave it active or cancel?
6. If cancel: `greatclips-pp-cli cancel` → 200, queue removed

## Out of scope

Everything from the v0.3 plan that isn't the exchange call. Cookie
extraction already works. Silent mint already works. The 200 lines
that remain are tiny IF U1 captures cleanly.

## Failure mode

If U1's capture reveals a multi-hop exchange (token A → call → token
B → call → token C) and replicating it in Go would take more than
~50 lines, stop. Report the architecture honestly. Either accept the
project as "read-only CLI for now" or commit to a longer build.

This plan exists because the previous three plans assumed Auth0
silent-mint was the whole story. It isn't. U1 settles that with
empirical evidence, not more guessing.
