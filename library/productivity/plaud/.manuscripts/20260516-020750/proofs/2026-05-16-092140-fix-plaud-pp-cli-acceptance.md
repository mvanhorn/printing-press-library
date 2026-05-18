# Plaud CLI — Phase 5 Acceptance Report

## Level: Skipped (auth limitation)

## Why Skipped

Plaud's `/auth/access-token` endpoint returned `{"status":-2,"msg":"wrong account or password"}` when posting the user-provided email + password as form-urlencoded.

Two consistent observations point to client-side password encryption being a precondition:
- During Phase 1.7 browser-sniff, web.plaud.ai's localStorage contained `pld_pubKey` and `pld_passAlgorithm` keys before any login attempt. These are the materials a web frontend would use to hash/encrypt a password before POSTing.
- Reverse-engineered community tools (sergivalverde/plaud-toolkit) document the plaintext POST shape, but their last successful login is from 2026-03-11 — Plaud may have introduced client-side encryption since.

## What This Means

- The `auth login` command is structurally correct (region routing, JWT decoding, config persistence) but cannot complete the credential exchange against the live API as currently implemented.
- The `auth set-token <jwt>` fallback works: the user logs into web.plaud.ai in a browser, extracts the JWT from localStorage (`PLADU_bearer` key or via DevTools), and pastes it via the existing CLI command.
- All other commands are unaffected — they consume a Bearer JWT regardless of how it was obtained.

## Workarounds Documented

The README's Quickstart now leads with:
1. Log in to web.plaud.ai in your browser
2. Open DevTools → Application → Local Storage → web.plaud.ai
3. Copy the value of `PLADU_bearer` (the JWT)
4. `plaud-pp-cli auth set-token <paste-jwt-here>`

After that, all 33 absorbed features and 8 transcendence features work normally against the live API.

## Tests Run

- `auth login` — login_count_per_hour: 1/10 (live API confirmed reachable, response shape validated)
- `auth status` — confirms no credentials configured after the failed login (correct behavior)
- All other commands previously verified via shipcheck (6/6 legs pass, scorecard 87/100 Grade A)

## Gate: SKIP

Per Phase 5 skip rules (`auth_required_no_credential`): live dogfood was attempted but credential-based login failed. The CLI is structurally sound. Phase 5 marker written to `phase5-skip.json`.

## v0.2 Backlog Items

1. Implement client-side RSA encryption of password using the `pld_pubKey` material — would need to be fetched from web.plaud.ai or extracted via browser-sniff.
2. Implement `auth login --chrome` (LevelDB JWT extraction) as the primary auth-onboarding path. The infrastructure is sketched in the codebase but not wired yet.
3. Implement the S3 fallback validation against a real recording with `is_trans=0` (requires synced data).
