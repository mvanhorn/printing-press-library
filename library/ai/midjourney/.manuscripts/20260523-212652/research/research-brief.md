# Midjourney CLI Research Brief

## Source

This printed CLI is based on browser-observed Midjourney web-app traffic. The relevant public-facing origin is `https://www.midjourney.com`.

Raw browser captures are intentionally not archived in the public package because HAR/CDP captures can contain account-specific identifiers, session-derived values, cookies, or rendered user content.

## Endpoint Shape

Observed read-only surfaces include generation history, queue, storage, folders, profiles, moodboards, explore feeds, and ranking/model-rating endpoints.

Observed image-generation behavior:

- The Create UI posts `POST /api/prompt-session-log` before generation.
- Image generation variants use `POST /api/submit-jobs`.
- Model and option selection is primarily encoded as prompt suffixes, including `--v`, `--niji`, `--style`, `--q`, `--draft`, `--tile`, `--weird`, references, and profile values.
- Rerun uses the same `/api/submit-jobs` endpoint with `t: "reroll"`.

## Auth Model

Midjourney web-app access requires an authenticated browser/session cookie. This CLI supports:

- browser-backed creation/export through Chrome DevTools Protocol;
- cookie-header-backed read-only calls through `MIDJOURNEY_COOKIE_HEADER`.

No real cookie headers, session tokens, local browser profile paths, or raw capture files are included in this package.

## Novel Commands

- `imagine`: submit Midjourney image jobs through the observed `/api/submit-jobs` flow.
- `rerun`: submit the observed reroll payload for an existing job ID.
- `download`: export rendered job images through Chrome CDP when direct CDN fetches are blocked by Cloudflare or CORS.
