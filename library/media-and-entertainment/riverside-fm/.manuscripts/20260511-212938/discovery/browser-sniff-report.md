# Riverside.com — Browser-Sniff Discovery Report

> Captured: 2026-05-12 from user's logged-in Chrome (Person 1 profile) via `uvx browser-use`.
> Runtime: macOS, Chrome 147, riverside.com (post-domain-migration from riverside.fm).

## 1. User Goal Flow

- **Goal**: Download transcripts → audio → video from the user's own Riverside recordings (priority-fallback flow).
- **Steps completed**:
  1. Open `https://riverside.fm/dashboard` (redirects to `riverside.com/dashboard/home`) — captured studio-overview XHRs.
  2. Click into a take row ("Damien — Take 03" in "Damien Stevens's Studio") — navigated to project view with `?activeTab=recordings&hls=true`.
  3. Click project-level "Download" — captured `/api/v4/take/{sessionId}/assets` plus the Export menu surface (1080p/4K Grid MP4, audio-only WAV, Cloud MP4).
  4. Click per-recording "Download" (index 2353 from page state) — captured another Export menu render.
  5. Click "Exports" tab — captured `/api/v4/projects/{projectId}/clips/exports?offset=0&limit=200` (empty list for this user — no prior exports).
  6. Cookie + auth probe: dumped all cookies, identified HttpOnly auth tokens, validated cookie replay.
- **Steps skipped**: did NOT trigger an actual export render (would consume the user's plan quota and produce live work — out of scope for discovery).
- **Secondary flows attempted**: enumerated all UI tabs (Recordings / Made for You / Edits / Exports) and the visible navigation surface.
- **Coverage**: 6 of 6 planned discovery interactions completed; 0 destructive triggers fired.

## 2. Pages & Interactions

| # | URL | Action |
|---|-----|--------|
| 1 | `https://riverside.fm/dashboard` (→ riverside.com/dashboard/home) | Initial load. Captured studio overview + Magic AI status XHRs. |
| 2 | (SPA) `/dashboard/studios/damien-stevenss-studio/projects/{projectId}?activeTab=recordings&hls=true` | Clicked Take 03 thumbnail (index 140 in page state). Captured 86 XHRs hydrating the project view: studio overview, projects list, takes list, recording backup statuses, clip details + patches, transcriptions, VOD HLS manifests, production media. |
| 3 | Same URL | Clicked the project Download button (index 140-region). Capture menu opened with Export 1080p/4K Grid MP4 + Export audio-only WAV + Cloud MP4 + Custom export. Triggered `/api/v4/take/{sessionId}/assets`. |
| 4 | Same URL | Clicked per-recording Download (index 2353). Same Export menu rendered. |
| 5 | Same URL with `?activeTab=exports` (tab click) | Captured `/api/v4/projects/{projectId}/clips/exports?offset=0&limit=200` — returned `{clips:[], total:0}` (user has no exports). |
| 6 | Same URL | Probed `document.cookie` plus `browser-use cookies export`. Identified HttpOnly auth cookies. |

## 3. Browser-Sniff Configuration

- **Backend**: `uvx browser-use` (v0.12.3+) — CLI-driven mode, no LLM key needed.
- **Profile loaded**: `Person 1` (user's authenticated Chrome profile, default).
- **Headless**: yes (no UI shown; eval-driven from terminal).
- **Pacing**: 1-second sleeps between user-driven clicks; no rate-limit responses observed.
- **Proxy pattern detection**: NOT a proxy-envelope. Riverside uses direct REST paths (`/api/v4/<resource>/...`) with conventional method-on-path semantics. No GraphQL.
- **Interceptor**: fetch + XHR monkey-patched before any further navigation. Captured `method`, `url`, `request_headers` (credentials redacted at capture time), `response_status`, `response_headers`, `request_body`, first 4KB of `response_body`, `response_content_type`.
- **Total entries captured**: 153 (raw); 76 after filtering noise (Segment, Coralogix, OpenTelemetry, Google Analytics, Intercom, HubSpot, etc.).

## 4. Endpoints Discovered

All endpoints below are authenticated via session cookies (Auth column).

| Method | Path | Status | Content-Type | Auth |
|--------|------|--------|--------------|------|
| GET | `/user` | 200 | application/json | auth-required |
| GET | `/api/v3/studio/{slug}` | 200 | application/json | auth-required |
| GET | `/api/v4/studio/{slug}/overview` | 200 | application/json | auth-required |
| GET | `/api/v4/studio/{slug}/event/can-create-event` | 200 | application/json | auth-required |
| GET | `/api/v4/projects/studio/{slug}` | 200 | application/json | auth-required |
| GET | `/api/v4/projects/{projectId}` | 200 | application/json | auth-required |
| GET | `/api/v4/projects/{projectId}/ai-generation-status` | 200 | application/json | auth-required |
| GET | `/api/v4/projects/{projectId}/takes?offset=&limit=` | 200 | application/json | auth-required |
| GET | `/api/v4/projects/{projectId}/clips/exports?offset=&limit=` | 200 | application/json | auth-required |
| GET | `/api/v4/recording/{recordingId}/backup-status` | 200 | application/json | auth-required |
| GET | `/api/v4/clip/{clipId}` | 200 | application/json | auth-required |
| GET | `/api/v4/clip/{clipId}/patches` | 200 | application/json | auth-required |
| GET | `/api/v4/take/{sessionId}/assets` | 200 | application/json | auth-required |
| GET | `/api/v4/take/{sessionId}/clip/{clipId}/clip-assets` | 200 | application/json | auth-required |
| GET | `/api/v4/transcriptions/editableWithVoiceActivity/{sessionId}` | 200 | application/json | auth-required |
| GET | `/api/v4/vod/{sessionId}/{participantHandle}` | 200 | application/x-mpegurl | auth-required |
| GET | `/api/v4/production/{productionId}/media` | 200 | application/json | auth-required |
| GET | `/api/v4/recording-ai/{slug}/can-generate` | 200 | application/json | auth-required |
| GET | `/account-access` | 403 | application/json | auth-required (admin-gated) |
| POST | `/api/v4/global-search/migrate` | 200 | application/json | auth-required |
| GET | `/dialogr/api/history?resourceId=&threadId=&format=&projectId=` | 200 | application/json | auth-required (Magic AI assistant) |
| POST | `/api/logs` | 204 | (empty) | optional |

## 5. Traffic Analysis (from `traffic-analysis.json`)

- **Protocols**: `rest_json` (0.75 confidence) — straightforward REST/JSON.
- **Reachability mode**: `browser_http` (0.78 confidence). The `/api/logs` 204 carried a Cloudflare header marker; reading endpoints worked fine over plain HTTPS with a Chrome UA. Generator will be passed `http_transport: browser-chrome` (Surf with Chrome TLS fingerprint) to be safe; if standard HTTP turns out adequate in dogfood, downgrade later.
- **Auth signals (auto-detected)**: none, because all auth cookies are HttpOnly (not visible to the in-page interceptor). Auth was hand-validated below.
- **Generation hints**: `requires_browser_auth` (cookie-based login required), `protected_web=false` (no JS challenge / Turnstile).
- **Warnings**: none critical.

## 6. Coverage Analysis

**Exercised resources**: studios, projects, takes, recordings (backup-status), clips, transcriptions, vod (HLS), production media, AI generation status, user profile.

**Likely missed (not exercised in this browser-sniff)**:
- The Editor's **export-creation** POST (`POST /api/v4/.../exports`?) — discoverable by clicking "Export audio-only WAV" but skipped to avoid consuming the user's render quota.
- **Delete** endpoints for clips/recordings — destructive, not exercised.
- **Share/invite** endpoints — captured `/api/{slug}/invite` exists from prior research but not triggered.
- **Webinar registration** endpoints — would require navigating to webinar workflow.
- **Storage / media upload** endpoints — out of scope.

Coverage is sufficient for the user's stated goal (read + download). Write surfaces can be added in v0.2 by re-sniffing while triggering them.

## 7. Response Samples (truncated to 2KB each)

### `GET /user`
```json
{"user":{"_id":"REDACTED-OBJECTID","role":50,"isUserEmailVerified":true,"payingCustomer":false,"enterpriseCustomer":false,"enterprise":false,"enterpriseRole":"Editor","defaultOptions":{...}}}
```

### `GET /api/v4/projects/{projectId}/takes?offset=0&limit=5`
```json
{"takes":[{"_id":"REDACTED-OBJECTID","recordings":[{"_id":"REDACTED-OBJECTID","type":"local","filename":"accounts/.../studios/.../takes/{sessionId}/{participantHandle}/{participantHandle}-2026-5-7__14-31-59.webm","slug":"<studio-slug>","recordingType":"audioAndVideo","speakerName":"<name>","deviceName":"<device>","archiveId":"<participantHandle>","mimeType":"video/webm;codecs=avc1.640034,pcm","status":"done","clientStatus":"uploaded","resolution":"1920x1080","frameRate":24,"sessionId":"REDACTED-UUID","duration":2.49,...}],"clips":[...]}]}
```

### `GET /api/v4/transcriptions/editableWithVoiceActivity/{sessionId}`
```json
{"success":true,"data":{"speakers":[],"voiceActivity":{"speakers":[]}}}
```
> Note: this user's transcript was empty for one of the takes I sampled (likely a very short take). Real takes contain speakers[] and voiceActivity[].

### `GET /api/v4/production/{productionId}/media`
```json
{"success":true,"media":[{"_id":"REDACTED","name":"Cheering","url":"https://private-assets.riverside.com/.../mediashare/{uuid}.mkv?Policy=<encoded>&Key-Pair-Id=<...>&Signature=<...>&response-content-disposition=attachment","duration":36.0745,"hlsUrl":"/api/v4/public-media/{mediaId}/vod","type":"audio","wavPath":"...","emoji":"...",...}]}
```
> This is the CloudFront signed-URL pattern. Note `Key-Pair-Id`, `Policy`, `Signature` query parameters and `response-content-disposition=attachment` to force download.

### `GET /api/v4/take/{sessionId}/assets`
```json
{"success":true,"studio":{...,"slug":"<studio-slug>","shareToken":"<token>"},"take":{"_id":"REDACTED","title":"Damien — Take 03","slug":"<studio-slug>","sessionId":"REDACTED-UUID","tracks":[{"_id":"REDACTED","type":"local","filename":"accounts/.../{filename}.webm","recordingType":"audioAndVideo","speakerName":"<name>","archiveId":"<participantHandle>","mimeType":"video/webm;codecs=avc1.640034,pcm","resolution":"1920x1080","frameRate":24,"duration":2.49,...}]}}
```

### `GET /api/v4/clip/{clipId}`
```json
{"data":{"_id":"REDACTED","createdBy":"REDACTED","createdByMeta":{"role":"host"},"snapshots":[],"slug":"<studio-slug>","take":{...,"_id":"REDACTED","name":"<name>","sessionId":"REDACTED","recordings":[...],"exports":{"premiere":{"status":"none"},"finalcut":{"status":"none"},"davinci":{"status":"none"},"tracks":{"status":"none"}}}}}
```
> The `take.exports` object has status entries for premiere, finalcut, davinci, and tracks — these are the four export targets the editor supports.

## 8. Rate Limiting Events

Zero 429 responses encountered. Effective rate was approximately 0.5 req/sec (conservative). The site did not push back.

## 9. Authentication Context

- **Browser-sniff was authenticated**: yes, via the user's Chrome `Person 1` profile loaded by `browser-use`.
- **Transfer method**: profile-load (no manual login, no headed window).
- **Auth-only endpoints exercised**: all 18 above except `/account-access` (403, admin-only).
- **Auth header scheme discovered**: **none** — Riverside does NOT send an Authorization header. Auth flows entirely through cookies sent automatically by the browser.
- **Cookies (HttpOnly, Secure, domain=riverside.com)**:
  - `riverside_auth_access` — primary access token (HttpOnly + Secure)
  - `riverside_auth_refresh` — refresh token (HttpOnly + Secure)
  - `sweetsesh` — session cookie (HttpOnly; not Secure)
  - `cloudfront_signed_url` — CloudFront-signed-URL session marker (HttpOnly + Secure + SameSite=Strict)
  - `dialogr-session` + `dialogr-session.sig` — Magic AI assistant session
- **Non-HttpOnly auth metadata** (visible to JS):
  - `production_riverside_auth_access_expiration` — access-token expiry epoch
  - `production_riverside_auth_refresh_expiration` — refresh-token expiry epoch
  - `osid` — origin/session ID
- **Cookie replay validated**:
  - `curl -b cookies.txt https://riverside.com/user` → **200**, returns the user object.
  - `curl https://riverside.com/user` (no cookies) → **401** `{"error":"User is not authenticated","authenticated":false}`.
  - **Verdict: cookie auth replays cleanly** (no CSRF, no IP binding, no SameSite=Strict on auth cookies — `riverside_auth_access` has no SameSite, `sweetsesh` has no SameSite). Generated CLI can ship `auth login --chrome`.
- **Cross-test against Business API**:
  - `curl -b cookies.txt https://platform.riverside.fm/api/v3/recordings` → **401** `{"message":"The Api Key is required.","error":"Unauthorized","statusCode":401}`.
  - **Verdict: the Business API does NOT accept cookies**. The two API surfaces have completely separate auth and there is no shared session between them. For Pro/Live/Webinar users, the cookie-based `riverside.com/api/v4/*` surface is the ONLY viable path.
- **Session-state file**: not written (cookies were dumped to `discovery/cookies.txt` for the replay test only; deleted before archiving).

## 10. Bundle Extraction

Not run. The browser-sniff captured 18 distinct authenticated endpoints + dynamic micro-frontend manifests (`/builds/mfe/...`); coverage was already strong, and bundle extraction would have surfaced mostly editor-side endpoints unrelated to the read+download use case.
