# WeWork CLI — Autonomous Agent Auth (CDP live-read + auto-refresh)

## Goal
Make WeWork auth agent-runnable: no per-run human token paste. WeWork is a
browser-origin auth0 SPA (no CLI-direct OAuth: no localhost redirect, no device
grant), and the fresh token lives in browser memory (Chrome flushes to disk
lazily/unreliably). So autonomy requires reading the LIVE browser session + a
self-refresh path.

## Key finding (verified LIVE in-browser)
`POST https://idp.wework.com/oauth/token` with grant_type=refresh_token +
client_id (JWT azp) + refresh_token returns a fresh 12h access token (status 200)
AND a ROTATED refresh token. So:
- The CLI CAN mint its own access tokens from a stored refresh token (headless).
- Rotation means the CLI and the personal browser can't share one refresh chain —
  use a DEDICATED agent Chrome. (Design reflects this.)

## Built (durable, markerless, self-registering)
- `internal/config/wework_refresh.go` — SaveWeworkSession + RefreshWeworkTokenIfNeeded
  (auto-refresh when token expired) + doAuth0Refresh (pure, testable) + jwtIssAzp.
- `internal/cli/wework_cdp.go` — reads the LIVE session (access+refresh+uuid+member)
  from a Chrome started with --remote-debugging-port, via CDP (Runtime.evaluate on the
  members.wework.com tab). Uses gorilla/websocket.
- `internal/cli/wework_chrome.go` — `auth login --cdp [--cdp-port N]` path.
- `internal/cli/wework_refresh_hook.go` — registerClientHook that auto-refreshes before
  the first request (client reads Config.AuthHeader() per-request, so it takes effect).
- Unit tests: refresh req/parse, jwtIssAzp, skip-logic, CDP target-pick + eval-message +
  session parse.

## Verified
- Refresh grant: LIVE (browser) — works, rotates.
- CDP plumbing: LIVE against a throwaway headless debug Chrome — connects, lists targets,
  correct "no wework tab" fallback. (Only the logged-in-tab read is unit-tested, not live.)
- go vet clean; unit tests pass; shipcheck all legs PASS except scorecard live_api_verification.

## To run autonomously (agent runtime)
1. Launch a DEDICATED Chrome once:
   "Google Chrome" --remote-debugging-port=9222 --user-data-dir=<agent-profile>
2. Log in to members.wework.com in it (one-time).
3. `wework-pp-cli auth login --cdp` seeds token+refresh+uuid+member.
4. Thereafter the CLI auto-refreshes its access token headlessly; agent runs any command.

## Still not done
- Live booking via the CLI (book-desk) — blocked in THIS session only: no debug-Chrome with
  the session available, and the classifier blocks moving the live token into the CLI by hand.
  With the agent-Chrome setup above, `auth login --cdp` seeds the CLI and book-desk can run.
