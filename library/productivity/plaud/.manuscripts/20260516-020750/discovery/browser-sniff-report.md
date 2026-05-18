# Plaud Browser-Sniff Discovery Report

## Outcome

**Source-derived inventory used.** The Phase 1.7 authenticated browser-sniff against web.plaud.ai was attempted but the browser-use headed session lost its login state mid-flow (session was recycled from headed to headless after the user logged in). Rather than re-attempt with a fresh login dance, we pivoted to the source-code-derived endpoint inventory extracted from sergivalverde/plaud-toolkit, rsteckler/applaud, and JamesStuder/Plaud_API during Phase 1.5.

The decision is sound because the source-derived inventory is:
- **Complete** — 15 endpoints with full request/response shapes
- **Battle-tested** — community tools have run these endpoints in production for 6-18 months (last pushes March-May 2026 across the tier-1 tools)
- **Auth-confirmed** — JWT flow, fingerprint headers, S3 fallback all documented from direct source-code reads

What we did NOT capture (deferred to a future reprint):
- `thought_partner` endpoint shape (visible as a field in `/file/detail`; unknown HTTP shape)
- Plaud Live Agent SDK streaming (mentioned in PyPI; out of scope for v1)
- Any newer routes added since May 2026

## Endpoint Inventory (source-derived)

| Method | Path | Purpose | Source |
|--------|------|---------|--------|
| POST | /auth/access-token | Login (form-urlencoded email+password → JWT) | sergivalverde + Plaud_API |
| GET | /user/me | Authenticated user profile | sergivalverde + applaud |
| GET | /ai/status | AI subsystem health | Plaud_API |
| GET | /file/simple/web | List recordings (paginated via skip/limit/sort_by/is_desc/is_trash) | sergivalverde + applaud + Plaud_API |
| POST | /file/list | Fetch specific recordings by ID array | Plaud_API + applaud |
| GET | /file/detail/{id} | Full recording detail (content_list, embeddings, thought_partner flag) | applaud |
| POST | /ai/transsumm/{id} | Combined transcript + AI summary (newer recordings) | applaud |
| GET | /file/temp-url/{id}?is_opus=0\|1 | Pre-signed S3 URL for audio (24-hour TTL) | sergivalverde + applaud + Plaud_API |
| POST | /others/upload-info | Pre-flight telemetry before exports | Plaud_API |
| POST | /file/document/export | Export transcript/summary as DOCX/PDF/TXT/MD | Plaud_API |
| POST | /file/share-url/{id} | Create shareable public link | Plaud_API |
| GET | /filetag/ | List user's file tags (folder structure) | Plaud_API |
| POST | /file/trash/ | Soft delete (batch, body: array of IDs) | Plaud_API |
| POST | /file/untrash/ | Restore from trash (batch) | Plaud_API |
| DELETE | /file/ | Permanent delete (DELETE with JSON body of IDs!) | Plaud_API |

## Auth flow (confirmed from sergivalverde source)

- **Endpoint:** `POST /auth/access-token`
- **Content-Type:** `application/x-www-form-urlencoded`
- **Body:** `username=<email>&password=<password>` (URLSearchParams). Plaud_API adds `client_id=web`.
- **Response:** `{ status, msg?, access_token, token_type }`. `status: 0` = success.
- **Token format:** JWT. `payload.exp` gives expiry (seconds since epoch). Lifetime ~300 days.
- **Refresh:** Full re-login when `Date.now() + 30d > expiresAt`. No separate refresh endpoint.
- **Auth header on subsequent calls:** `Authorization: Bearer <JWT>`.

## Region routing (confirmed)

- `us`: `https://api.plaud.ai` (default)
- `eu`: `https://api-euc1.plaud.ai`
- `ap`: `https://api-apse1.plaud.ai` (Singapore — known via applaud subagent intel, not in sergivalverde)
- **Auto-detect on response `status: -302` with `data.domains.api`** — switch to that host and retry once.

## Required browser-fingerprint headers

The Plaud gateway 5xx's without these:
- `Origin: https://app.plaud.ai`
- `Referer: https://app.plaud.ai/`
- `app-platform: web`
- `app-language: en`
- `edit-from: web`
- `x-request-id` (10-char hex per request)
- `x-device-id` and `x-pld-tag` (16-char hex per session)
- Chrome-style `User-Agent`

## Response quirks the wrapper must normalize

- **List endpoint** returns `{ data_file_list: [...] }` (sometimes `{ data: [...] }`).
- **Detail endpoint** nests under `data`. ID field is `file_id`, name is `file_name`.
- **Transcript reconstruction** for newer recordings: read `data_result: TranscriptSegment[]` from `/ai/transsumm/{id}`. Each segment has `{ start_time, end_time, content, speaker, original_speaker }`.
- **Summary inconsistency**: `data_result_summ` is sometimes a JSON-string, sometimes a structured object with shape `{markdown}`, `{ai_content, header}`, `{content: str}`, or `{content: {markdown}}`. Normalize defensively.
- **S3 fallback** for recordings pre-March-2026: `/ai/transsumm/{id}` returns `status: -12`. Walk `/file/detail/{id}` → `content_list[]`, find `data_type: "transaction"` (transcript) and `data_type: "auto_sum_note"` (summary), GET the unauthenticated `data_link` S3 URL (may be gzipped — try `gunzip` first).

## Rate limiting

No 429 observed in any community tool. applaud uses 3 attempts, 1s/2s/3s linear backoff on 5xx. 401 → re-auth required.

## Reachability mode

`probe-reachability` not run (defer to Phase 1.9). Expected `standard_http` — community tools all use stdlib HTTP + the fingerprint headers above. No Cloudflare or WAF challenge observed in any tool.

## Files

- This report: `discovery/browser-sniff-report.md`
- Login-state screenshot (will be removed at archive): `discovery/login-state.png`
