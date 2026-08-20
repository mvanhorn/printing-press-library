---
title: Capture stylewaretouch JWT via real check-in action and complete live CLI check-in
type: feat
status: active
created: 2026-05-11
target_repo: ~/printing-press/library/greatclips
depth: lightweight
---

# feat: Capture stylewaretouch JWT and complete live CLI check-in

## Summary

The v0.2 plan landed signing infrastructure but never produced a live
check-in because the captured Auth0 JWT was scoped for
`webservices.greatclips.com/customer`, not `www.stylewaretouch.net`.
The GreatClips SPA mints separate tokens per audience and only fetches
the stylewaretouch token when a real check-in action is taken — not on
page load.

User-authorized fix: drive the SPA to trigger a stylewaretouch fetch
(by clicking "Check In" in their logged-in Chrome session), capture
the resulting Bearer token via the fetch interceptor that's already
installed, then submit the actual check-in from the CLI.

This delivers the user's killer flow tonight. The v0.3 Auth0 silent
flow port (so this works without browser action each session) remains
in scope as a follow-up.

## Problem Frame

v0.2 facts:
- CLI signing port is byte-identical to the SPA (golden vector test passing).
- CLI request pipeline correctly appends `?t=&s=` to stylewaretouch URLs.
- Captured JWT (`aud: webservices.greatclips.com/customer`) is rejected
  by stylewaretouch with HTTP 400 "Invalid request".

The block: getting a stylewaretouch-audience JWT into the CLI.

The user observed that the SPA only calls stylewaretouch on demand
(specifically when navigating to a check-in form or submitting). My
read-only browsing didn't trigger any stylewaretouch fetch. Clicking
the actual "Check In" CTA forces the SPA to either: (a) load the
check-in form which fetches `/api/customer/status` to check for an
active check-in, or (b) submit `/api/customer/checkIn`. Either fires
a stylewaretouch request whose `Authorization: Bearer` header the
interceptor will capture.

## Scope Boundaries

### In scope tonight
- Capture a stylewaretouch-audience JWT via a real SPA action
- Verify the JWT's `aud` claim is `www.stylewaretouch.net` (or whatever
  the audience string actually is — discovery during execution)
- Run live `greatclips-pp-cli checkin` for Matt + 3 kids at Island Square
- Verify the check-in succeeded via `greatclips-pp-cli status`
- Decide with the user whether to leave them on the waitlist or cancel

### Deferred to v0.3 (separate plan)
- Auth0 silent token flow (call `cid.greatclips.com/authorize?prompt=none&audience=<x>`
  with extracted HttpOnly session cookies) so the CLI mints tokens
  itself without browser action
- Auto-refresh on JWT expiry
- Per-audience token storage and routing in `internal/config/config.go`
  (a single `GREATCLIPS_TOKEN` works tonight because every command in
  the v0.2 set will hit at most one host per invocation)

## Requirements

| ID | Requirement |
|----|-------------|
| R1 | A stylewaretouch-audience JWT is captured from the user's logged-in Chrome session and written to a 0600 file in `/tmp` |
| R2 | The CLI's `checkin` command, run with that JWT, returns a 200 response from `www.stylewaretouch.net/api/customer/checkIn` containing the expected fields (success, position, etc.) |
| R3 | A subsequent `status` call confirms Matt is on the queue with party size 4 |
| R4 | The JWT and any captured tokens are scrubbed from `/tmp` after the session |
| R5 | If the live check-in succeeds, the user explicitly chooses whether to leave it active or cancel — agent does not unilaterally cancel a real action |

## Key Technical Decisions

### Trigger via "Check In" button click, not form-submit

The "Check In" button on the salon detail page navigates to
`/salon/<id>/check-in/` which renders the form. That page-load is
expected to call `/api/customer/status` to populate the form's "you
already have an active check-in" state. That GET fires from the SPA
with the proper stylewaretouch token, which the fetch interceptor
captures.

Why this is enough: a single Bearer header capture proves the
audience and gives the CLI everything it needs. We do not need to
submit through the SPA — that step happens through the CLI.

If the form page doesn't actually call stylewaretouch (only the submit
does), we fall back to having the user click submit in the SPA, which
puts them on the waitlist via the SPA's submission. In that fallback
case we still capture the token from the submit fetch, and the CLI's
role becomes verifying status + offering cancel — which still validates
the v0.2 signing pipeline end-to-end on the live API.

### One JWT per session is enough for tonight

The v0.2 client already supports a single `GREATCLIPS_TOKEN` env var.
For tonight's flow we only need stylewaretouch tokens — every command
in the check-in lifecycle (`status`, `checkin`, `cancel`) hits
stylewaretouch. We do not need per-host token routing tonight.

Tomorrow, if the user wants `customer profile` and `salons search` in
the same shell session, they'll need separate tokens — that's the
real v0.3 work.

### JWT scrub immediately after verification

Per the v0.1 cardinal-rule pattern, the captured JWT is written to
`/tmp/gc-sw-token.txt` with 0600, used for the live check-in and
status calls, then `rm -f`'d. It never lands in shell history, logs,
or any committed artifact.

## Implementation Units

### U1. Capture stylewaretouch JWT via real Check-In action

**Goal:** A valid stylewaretouch-audience JWT lands at `/tmp/gc-sw-token.txt`
with 0600 perms.

**Requirements:** R1

**Dependencies:** none

**Files:** none — pure interactive capture flow, no code changes

**Approach:**
1. Install a fresh fetch interceptor in the user's Chrome tab that
   captures the first `Authorization: Bearer ...` header on any
   `stylewaretouch.net` URL.
2. Navigate the tab to `https://app.greatclips.com/` and have the
   user click "Check In" on the Island Square card (or drive the
   click via the Claude-in-Chrome MCP — the user has authorized this).
3. The check-in form page should fire `GET /api/customer/status`
   with the stylewaretouch JWT in the Authorization header.
4. The interceptor captures the JWT. Download via Blob trick to
   `~/Downloads/gc-sw-token.txt`, then `mv` to `/tmp/gc-sw-token.txt`
   with `chmod 600`.
5. Decode the JWT payload (base64url middle section) and verify the
   `aud` claim matches the stylewaretouch audience. If `aud` is still
   `webservices.greatclips.com/customer`, the SPA reused the wrong
   token and we need to go deeper (fall back: have user click the
   form submit button to force the stylewaretouch POST).

**Patterns to follow:** the v0.1 JWT capture flow already proven in
the prior session (interceptor + Blob download + 0600 mv).

**Test scenarios:**
- The downloaded file has 3 dot-delimited parts (valid JWT structure).
- The base64-decoded payload contains an `aud` field that is NOT
  `https://webservices.greatclips.com/customer`. If it IS that, the
  capture failed and we fall back to step 5's plan-B.
- File permissions are 0600.

**Verification:** `python3 -c "import base64,json; ..."` against the
file prints an `aud` claim referencing stylewaretouch.

---

### U2. Live check-in for Matt + 3 kids at Island Square (salon 8991)

**Goal:** The CLI's `checkin` command submits a real check-in and the
user is on the actual waitlist at Island Square.

**Requirements:** R2, R3, R5

**Dependencies:** U1

**Files:** none — uses the existing v0.2 binary at
`~/printing-press/library/greatclips/greatclips-pp-cli`

**Approach:**
1. `export GREATCLIPS_TOKEN="$(cat /tmp/gc-sw-token.txt)"`
2. **Pre-check (read-only):** `greatclips-pp-cli status` should
   succeed (not 401, not 500). Either returns "no active check-in"
   or shows an existing one. If it returns 401, U1 captured the
   wrong token; re-run U1's plan-B fallback.
3. **Re-confirm with user before submitting** — `checkin` is a real
   mutation. Show the exact command and request body, ask "Submit
   for real?" with a clear yes/no.
4. **Submit:** `greatclips-pp-cli checkin --first-name Matt
   --last-name "Van Horn" --phone-number "(520) 907-6052"
   --salon-number 8991 --guests 4 --json`
5. **Verify:** `greatclips-pp-cli status --json` should return Matt's
   active check-in at salon 8991 with `positionInLine` and
   `estimatedWaitMinutes` populated.
6. **Ask the user:** leave them on the waitlist (real visit) or
   cancel? If cancel: `greatclips-pp-cli cancel`. If leave: scrub
   tokens and report success.

**Patterns to follow:** the v0.1 dry-run smoke pattern. Live calls
go through the same path; only `--dry-run` is removed.

**Test scenarios:**
- `status` returns 200 with parsed JSON (proves token audience is right).
- `checkin` returns 200 with a parsed JSON response object (proves
  signing + audience are both correct end-to-end).
- The response from `checkin` contains some indicator of success
  (the actual field names will be discovered at execution time —
  candidates include `success`, `checkInId`, `positionInLine`,
  `estimatedWaitMinutes`, or a JSON envelope shaped like the
  `WaitTime` response).
- Post-check-in `status` shows the new check-in with `salonNumber:
  "8991"` and `partySize: 4` (or whatever the field name turns out
  to be).
- If `cancel` runs, the next `status` shows no active check-in.

**Verification:** the user is actually checked in at Island Square,
reachable via the GreatClips app or web UI to confirm visually.

---

### U3. Acceptance evidence and v0.3 next-steps note

**Goal:** Capture the live outputs into a plan addendum so v0.3 has
honest evidence to build on; surface remaining v0.3 work cleanly.

**Requirements:** indirectly supports R5 — documents what actually
happened so the user can decide next steps with full context

**Dependencies:** U2

**Files:**
- `docs/plans/2026-05-11-002-feat-stylewaretouch-token-capture-plan.md`
  (this file, modify) — append `## Acceptance Evidence` section with
  the JSON shapes of the live responses
- `README.md` (modify) — flip the "Not yet working live" block to
  reflect the partial unlock; note "wait/status/checkin/cancel work
  end-to-end with a per-session JWT paste; v0.3 will automate the
  paste"

**Approach:** straightforward documentation update. The acceptance
evidence section is where the field names and response shapes
discovered during U2 land — this is also the spec input for v0.3's
typed response models.

**Test scenarios:** documentation; no behavioral test.

**Test expectation:** none -- documentation unit.

**Verification:** the README accurately reflects the partial unlock;
the plan carries the response shapes for future use.

## Risks

- **The "Check In" button click might not fire a stylewaretouch
  fetch.** Mitigation: the plan-B fallback has the user click the
  actual submit button, which forces the stylewaretouch POST. The
  captured token then comes from the submit fetch.
- **The check-in actually puts the user on a real waitlist.** This
  is the intended behavior — the user explicitly asked for it. The
  cancel command is available and tested as part of the same unit.
- **Token expires mid-flow.** Auth0 access tokens are typically valid
  for 60 minutes. If status passes but checkin fails with 401, the
  token expired between calls; re-capture and retry.
- **The captured token still has the wrong audience.** Possible if
  the SPA aggressively caches tokens and reuses the webservices one
  for stylewaretouch URLs (unlikely but possible). The audience
  check in U1's verification catches this; fallback is plan-B.

## Verification

The plan is complete when:

- The user is actually checked in at Island Square (or has explicitly
  declined and chosen to cancel after verification).
- The acceptance evidence section in this plan captures the live
  response JSON shapes.
- All captured tokens are scrubbed from `/tmp`.
- The README accurately describes what works live and what's still
  blocked on v0.3.

The killer flow is reachable:

```
greatclips-pp-cli status              # 200, parsed JSON
greatclips-pp-cli checkin --salon-number 8991 --guests 4 ...   # 200, on the waitlist
greatclips-pp-cli status              # shows the check-in we just made
greatclips-pp-cli cancel              # (optional) removes it
```
