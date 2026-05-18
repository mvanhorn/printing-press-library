# Suno Browser-Sniff Discovery Report

**Target:** https://suno.com (web app) -> https://studio-api-prod.suno.com (API host)
**Tool:** browser-use v0.12.5 (headed mode, fresh Chromium profile)
**Date:** 2026-05-14
**Auth mode:** authenticated (Google OAuth -> Clerk session)
**Primary goal:** Generate a song from a prompt (the user's #1 workflow)

## Outcome

API surface discovered, generate endpoint and response shape confirmed, auth model fully understood. Real cost: 10 Suno credits (one test song with prompt "donkey love"). Ready for spec generation.

## Auth Model

Suno uses **Clerk** for authentication, layered:

| Layer | Cookie | Domain | HttpOnly | Purpose |
|-------|--------|--------|----------|---------|
| Long-lived session | `__client` | auth.suno.com | yes | Refresh source. JWT regenerator. |
| Short-lived JWT | `__session` | .suno.com | no | THE access token. Sent verbatim as `Authorization: Bearer <__session>`. ~1 hour TTL. |
| Session pointer | `clerk_active_context` | suno.com | no | `session_<id>` reference for refresh endpoint URL. |
| Device fingerprint | `suno_device_id` | suno.com | no | Sent as both `browser-token` and `device-id` headers on every API call. |

**Auth header observed on every API call:** `Authorization: Bearer <JWT>`. Token is RS256 (`eyJhbGciOiJSUzI1NiIs...`).

**Refresh flow (for printed CLI):** when `__session` expires, POST to `https://clerk.suno.com/v1/client/sessions/<clerk_active_context value>/tokens` with the `__client` cookie. Response is a fresh `__session` JWT.

**Generated spec:** `auth.type: composed`, format `Bearer {__session}`, with `__session` from the user's Chrome cookie jar (via Printing Press's standard `auth login --chrome` command), and `__client` for refresh.

## Endpoints Discovered

Combined from interactive capture (browser-use interceptor) and page-load Performance API (URLs that fired before interceptor was installed).

| Method | Path | Description | Source | Auth |
|--------|------|-------------|--------|------|
| POST | `/api/generate/v2-web/` | **Generate a new song** (primary workflow) | capture | required |
| POST | `/api/generate/concat/v2/` | Concatenate clip extensions | community-known | required |
| POST | `/api/generate/lyrics/` | Generate lyrics only (FREE, 0 credits) | community-known | required |
| GET | `/api/generate/lyrics/{id}` | Poll lyrics generation status | community-known | required |
| GET | `/api/video/generate/{id}/status/` | Poll video render status for a clip | capture | required |
| POST | `/api/feed/v3` | List user's clips (paginated library feed) | capture | required |
| GET | `/api/clip/{id}` | Get a single clip by ID | community-known | required |
| GET | `/api/clips/{id}/attribution` | Attribution info / lineage | capture | required |
| GET | `/api/clips/parent?clip_id={id}` | Parent clip (for extends/covers) | capture | required |
| GET | `/api/clips/direct_children_count?clip_id={id}` | Count of extends/covers | capture | required |
| GET | `/api/clips/get_similar/?id={id}` | Similar clips | capture | required |
| GET | `/api/gen/{id}/aligned_lyrics/v2/` | Word-aligned lyrics with timestamps | community-known | required |
| GET | `/api/gen/{id}/comments` | Comments on a clip | capture | required |
| POST | `/api/gen/{id}/set_visibility` | Toggle public/private/unlisted | community-known | required |
| POST | `/api/gen/{id}/set_metadata` | Edit title/tags/lyrics | community-known | required |
| POST | `/api/gen/trash/` | Trash clips | community-known | required |
| GET | `/api/persona/get-persona-paginated/{id}/` | Persona detail | community-known | required |
| GET | `/api/persona/me` | List user's personas | community-known | required |
| GET | `/api/billing/info/` | Credits, plan, renewal | capture | required |
| GET | `/api/billing/eligible-discounts` | Available discounts | Performance API | required |
| GET | `/api/billing/usage-plan-web-table-comparison/` | Plan comparison table | capture | required |
| GET | `/api/billing/usage-plan-faq/` | Plan FAQ | capture | required |
| GET | `/api/user/user_config/` | User config + feature flags | Performance API | required |
| GET | `/api/personalization/settings` | Personalization | Performance API | required |
| GET | `/api/personalization/memory` | Personalization memory | Performance API | required |
| GET | `/api/project/me` | User's project memberships | Performance API | required |
| GET | `/api/project/default` | Default workspace details | Performance API | required |
| GET | `/api/project/default/pinned-clips` | Pinned clips in workspace | capture | required |
| GET | `/api/notification/v2` | Notifications | capture | required |
| GET | `/api/notification/v2/badge-count` | Unread count | capture | required |
| GET | `/api/custom-model/pending/` | Pending custom-model jobs | Performance API | required |

Endpoints intentionally excluded from the spec:
- `POST /api/c/check` -- captcha pre-flight check (not user-facing)
- `POST /api/unified/feed` -- returns 404 (deprecated)
- `GET /api/challenge/progress` -- onboarding challenges (not core workflow)
- `GET /api/statsig/experiment/*` -- feature flag experimentation (not user-facing)

## Generate Response Shape (the headline endpoint)

POST `/api/generate/v2-web/` returns:

```json
{
  "id": "<generation UUID>",
  "clips": [
    {
      "id": "<clip UUID>",
      "status": "submitted | streaming | complete | error",
      "title": "...",
      "audio_url": "https://cdn1.suno.ai/<clip-id>.mp3",
      "video_url": "https://cdn2.suno.ai/<clip-id>.mp4",
      "image_url": "https://cdn2.suno.ai/image_<id>.jpeg",
      "major_model_version": "v5.5",
      "model_name": "chirp-fenix",
      "metadata": {
        "prompt": "<custom lyrics or empty>",
        "gpt_description_prompt": "<description-mode prompt>",
        "tags": "...",
        "type": "gen",
        "stream": true,
        "make_instrumental": false,
        "can_remix": true,
        "is_remix": false
      },
      "user_id": "<user UUID>",
      "display_name": "...",
      "handle": "...",
      "created_at": "<ISO8601>",
      "is_public": false,
      "is_trashed": false,
      "is_hidden": false,
      "is_liked": false,
      "play_count": 0,
      "upvote_count": 0,
      "action_config": {
        "actions": [
          {"action_type": "add_to_playlist", "disabled": false, "visible": true},
          {"action_type": "like_song", ...},
          {"action_type": "share_song", ...},
          {"action_type": "delete_song", ...},
          {"action_type": "edit_song_details", ...},
          {"action_type": "download_song", ...},
          {"action_type": "download_as_video", ...},
          {"action_type": "remix_extend", "disabled": true},
          {"action_type": "remix_cover", "disabled": true},
          {"action_type": "create_hook", ...}
        ]
      }
    }
  ]
}
```

`audio_url` is empty until `status: complete`. Polling: GET `/api/feed/v3` filtering by this clip ID, or repeatedly fetch the clip detail. `remix_extend` / `remix_cover` are disabled until generation completes.

## Required Headers (anti-bot)

Every API request to `studio-api-prod.suno.com` carries these custom headers in addition to `Authorization`:

- `browser-token` -- value of `suno_device_id` cookie
- `device-id` -- value of `suno_device_id` cookie
- `content-type: application/json` for POSTs
- `accept: application/json`

A printed CLI that does not send `browser-token` and `device-id` will likely receive 403 / 429 from Suno's anti-bot layer.

## Reachability

- Direct HTTP probe of `https://api.suno.com` returns `browser_clearance_http` (cf-mitigated)
- `https://studio-api-prod.suno.com` (the real API host) is reachable with a Chrome-fingerprint user agent + bearer token + the custom headers above. No clearance cookie needed at the transport layer.
- The Printing Press auto-analyzer flagged `browser_required` due to `POST /api/c/check` being treated as a CAPTCHA marker. **This is a false positive** -- `/api/c/check` is a captcha *pre-flight check* (returned 200 with body 40 bytes, equivalent to "no challenge required"), not a transport barrier. The printed CLI does not need a resident browser.
- Recommended runtime: `browser-chrome` Surf transport (Chrome TLS fingerprint via Surf) + custom headers + Bearer JWT.

## Replayability Verdict

**PASS** -- every captured endpoint round-trips through Surf with the right headers. No persistent-browser dependency for the printed CLI. Browser-sniff was discovery only; the printed CLI ships HTTP-only runtime.

## Test artifacts

- `discovery/sniff-capture.json` -- raw capture entries (27 unique API requests, response bodies up to 8KB each)
- `discovery/suno-capture.har` -- HAR-formatted capture with credentials redacted
- `discovery/session-cookies.json` -- exported cookie jar (will be cleaned up in Phase 5.6 archive)
- `discovery/traffic-analysis.json` -- printing-press browser-sniff analyzer output (reachability flagged `browser_required` -- overridden as documented above)
- `discovery/create-page.png` -- screenshot of the create UI

## Cost

10 Suno credits spent on one test song ("donkey love"). The clip is in the user's library at clip ID `6b055eee-3b1c-4a74-9aa9-1f16c0818fba`. The user authorized 20-30 credits but the Remix flow was blocked by a privacy modal so extend/concat were not exercised at the live API. Those endpoints are documented in the spec from community-wrapper knowledge and will be verified in Phase 5.
