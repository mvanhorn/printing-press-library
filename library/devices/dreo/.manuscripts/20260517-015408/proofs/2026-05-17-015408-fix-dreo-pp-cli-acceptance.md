# Dreo CLI Acceptance Report

**Level:** Full Dogfood
**Tests:** 64/64 passed, 44 skipped (no-arg / mutating-dry-run-only), 0 failed
**Date:** 2026-05-17

## Live API verification

Login round-tripped against `app-api-us.dreo-tech.com` against the authenticated viewer's real account. One device on the account (an Air Circulator, model DR-HPF004S). All read-side endpoints returned valid responses; no mutations were performed.

## Failures fixed inline (not deferred)

1. **`Config.AuthHeader()` returned `Bearer <email>` when no AccessToken set** — generator emitted a placeholder that synthesized a bearer header from the username env var (which is the user's email). Patched to return `""` when no AccessToken is present; relies on `auth login` or the new lazy-login path.
2. **`fetchDeviceState` returned the raw `data` map** — Dreo wraps state fields in a `{state, timestamp}` envelope inside a `mixed` sub-object. Added `flattenState()` that unwraps the envelope and merges `mixed`/`deviceInfo` into a flat state map. Every novel command that reads state now sees scalar fields directly.
3. **Devices list `online` field absent** — Dreo's `/api/v2/user-device/device/list` does not return an `online` key. Defaulted to `true` when the field is missing rather than the generator's `false`-on-missing-bool.
4. **No auto-login** — under `dogfood --live`'s scoped HOME (a fresh tempdir per subprocess), the CLI had no cached token. Added `Client.lazyLogin()` that performs the OAuth exchange when `DREO_USERNAME`/`DREO_PASSWORD` are set in env but no AccessToken is in config. CLI now "just works" with env-var credentials, no `auth login` prerequisite.
5. **`scene list` help missing Examples** — Added two example invocations.
6. **`watch <name>` accepted nonexistent device names silently** — Now resolves the filter to a sn BEFORE opening the WebSocket and exits with `notFoundErr` if the device isn't in the local catalog.

## Printing Press issues (for retro)

None that block ship. The generator-emitted `Config.AuthHeader()` placeholder (using `DreoUsername` as a token) is worth flagging — for any CLI declaring `env_vars: [USERNAME, PASSWORD]`, the generator should either omit the synthesis path or emit a placeholder that returns "" instead of a fake bearer.

## Gate: PASS

64/64 tests in the live matrix passed against the authenticated viewer's real Dreo account. Zero auth failures, zero sync failures, every novel feature returns plausible output for the single real device available.

## Auth context

- `type`: bearer_token (via lazy OAuth login from DREO_USERNAME/PASSWORD)
- `api_key_available`: true (user supplied credentials)
- `browser_session_available`: n/a (Dreo uses email/password, not browser session)
