# Withings HealthMate — Browser-Sniff Discovery Report

**Target:** `https://healthmate.withings.com/<id>/timeline` (HealthMate web app SPA)
**Backend:** Withings privileged compatibility check via Spaceraccoon writeup
**Capture method:** chrome-MCP (Chrome extension), authenticated session, fresh isolated tab, closed after.
**Date:** 2026-06-14

> PII note: this report records *structure only* — no health values, account identifiers, names, emails, cookies, or token values. All were observed live but deliberately excluded.

## 1. User Goal Flow
- **Goal:** Load the logged-in HealthMate `/timeline` ("Journal") and discover the hosts, endpoints, auth mechanism, and response shapes the web app uses.
- **Steps completed:** Loaded timeline (authenticated, session active) → enumerated resource loads via Performance API → triggered XHR via dashboard/journal interaction → probed representative endpoints read-only.
- **Coverage:** Architecture + auth + endpoint inventory established. Exact per-endpoint action/param contract NOT fully captured (see §6).

## 2. Pages & Interactions
- `healthmate.withings.com/<id>/timeline` — loaded (session active; rendered dashboard widgets + Journal feed).
- Interacted with Journal/dashboard tiles to trigger data XHRs.
- Read-only probes to `scalews.withings.com/cgi-bin/v2/measure`, `/cgi-bin/measure`, `/cgi-bin/v2/timeline` (action=getmeas / getsummary). No mutations performed.

## 3. Browser-Sniff Configuration
- Backend: **chrome-MCP** (drives existing logged-in Chrome; fresh capture tab; auto-closed).
- Pacing: light, well under any rate limit. No 429s observed.
- Proxy pattern: **not** a proxy-envelope. Direct action-based POST endpoints.

## 4. Endpoints Discovered
Host counts this load: `scalews.withings.com` (41), `healthmate.withings.com` (20, mostly app shell/i18n), `static.withings.com` (assets), analytics hosts (excluded).

All data endpoints are **POST** to `scalews.withings.com/cgi-bin/...`, `Content-Type: application/x-www-form-urlencoded`, JSON responses, HTTP 200 with a `{status, body|error}` envelope.

| Method | Path | Status | Content-Type | Auth |
|---|---|---|---|---|
| POST | /cgi-bin/measure | 200 | application/json | session-cookie |
| POST | /cgi-bin/v2/measure | 200 | application/json | session-cookie |
| POST | /cgi-bin/v2/activity | 200 | application/json | session-cookie |
| POST | /cgi-bin/v2/aggregate | 200 | application/json | session-cookie |
| POST | /cgi-bin/v2/heart | 200 | application/json | session-cookie |
| POST | /cgi-bin/v2/summary | 200 | application/json | session-cookie |
| POST | /cgi-bin/v2/timeline | 200 | application/json | session-cookie |
| POST | /cgi-bin/v2/target | 200 | application/json | session-cookie |
| POST | /cgi-bin/v2/plan | 200 | application/json | session-cookie |
| POST | /cgi-bin/v2/feature | 200 | application/json | session-cookie |
| POST | /cgi-bin/v2/subcategory | 200 | application/json | session-cookie |
| POST | /cgi-bin/v2/account | 200 | application/json | session-cookie |
| POST | /cgi-bin/account | 200 | application/json | session-cookie |
| POST | /cgi-bin/association | 200 | application/json | session-cookie |

These mirror the official Withings API action verbs (measure→getmeas, activity, heart, etc.) but on the `scalews.withings.com/cgi-bin/` web backend rather than `wbsapi.withings.net`.

## 5. Traffic Analysis
- **Protocol:** action-based form-POST RPC returning `{status:int, body|error}` JSON envelope (same family as the official Withings API).
- **Auth signals:** `session_token` cookie present, **JS-readable (non-httpOnly)**, `.withings.com`-scoped so it is sent cross-origin to `scalews`. **No `Authorization` header** and no custom `X-*` auth headers were constructed by the SPA (captured request header name list was empty). Auth is the session cookie (possibly echoed into the body as a session id by some calls — not confirmed).
- **Protection signals:** none observed at the data endpoints (no Cloudflare/captcha/login-redirect on `scalews`). The page itself loaded fine in the authenticated session.
- **Web-only endpoints** (not in the official OAuth API): `aggregate`, `plan`, `target`, `feature`, `subcategory`, `timeline` (the Journal feed). Core data (`measure`, `activity`, `heart`, `summary`) overlaps the official API.

## 6. Coverage Analysis / Gaps
- Endpoint **inventory** captured (14 data paths).
- Exact **action names + required params per endpoint NOT captured.** Read-only probes with guessed params returned `503 "Invalid Params: Missing params"` and `2554 "Not implemented"`. The SPA caches responses in JS memory, so only the first interaction after a cold load fires real network calls; page-context interceptors installed after mount missed the cold request bodies. A **DevTools HAR** (captures from page load, bodies included) is the reliable way to recover the exact contract for this path.

## 7. Response Samples
Envelope shape only (values redacted):
- Success: `{ "status": 0, "body": { ... } }` where measure-family bodies carry `measuregrps[]` with per-group `measures[]` (`{type, unit, value}`); activity/summary carry per-day series.
- Error: `{ "status": <code>, "error": "<message>" }` (e.g., 503 "Invalid Params", 2554 "Not implemented"). HTTP is 200 even on logical error — status lives in the JSON `status` field.

## 8. Rate Limiting Events
None. No 429s. Light pacing.

## 9. Authentication Context
- **Authenticated** session used (logged-in Chrome via chrome-MCP).
- **Mechanism:** `session_token` cookie (web session). Importable from Chrome → a cookie-replay CLI is technically feasible.
- **Fragility:** web `session_token` is short-lived with no refresh-token mechanism; expires on logout/timeout and must be re-imported. Backend is undocumented and actively hardened (per Spaceraccoon 2024 security research).
- **Cookie-required proof:** inconclusive from probes — the `503 "Invalid Params"` param wall fires before the auth path, so with-cookie vs without-cookie both returned 503 on guessed params. The successful SPA calls (200) authenticated via the cookie with no other header.
- Session state was **not** written to any artifact (cookies/tokens excluded by design).

## 10. Bundle Extraction
Not run.

## Runtime assessment
- **Web cookie path:** viable but requires (a) a DevTools HAR to recover the undocumented `scalews` action/param contract, and (b) accepting a fragile, short-lived session cookie and an undocumented, actively-hardened backend.
- **Official OAuth2 API (`wbsapi.withings.net`):** exposes the same *core* health data (measure, activity, sleep, heart/ECG, workouts) with a documented contract and durable auto-rotating refresh tokens; lacks the web-only endpoints (aggregate/plan/target/feature/timeline-feed).
