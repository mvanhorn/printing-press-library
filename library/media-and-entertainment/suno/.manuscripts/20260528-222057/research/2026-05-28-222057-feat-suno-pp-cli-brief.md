# Suno CLI Brief

## API Identity
- **Domain:** AI music generation. Suno (suno.com) turns text/lyrics+style into full songs; supports extend, cover, remaster, stems separation, voice personas, timestamped lyrics, library/playlists, credits/billing.
- **Official API:** None. Every client is reverse-engineered against `studio-api-prod.suno.com` with **Clerk** auth (`auth.suno.com`, migrated from `clerk.suno.com` Jan 2026) + **hCaptcha** gating on generation.
- **Users:** Musicians, content creators, hobbyists, agent/automation builders generating music programmatically.
- **Data profile:** Clips (songs) with audio/video/image URLs, tags, lyrics, duration, bpm, status, play/upvote counts; personas; lyrics jobs; billing/credits; playlists.

## Reachability Risk
- **HIGH for generation / Low–Medium for everything else.** Generation (`/api/generate/v2-web/`) requires a fresh hCaptcha token; captcha fires after sustained credit use and the auto-solver breaks on every Suno UI change. All read/library/metadata/lyrics/stems/persona endpoints work with just the Clerk-minted JWT.
- Evidence: gcui-art/suno-api issues — #277 (v5.5 UI dropped the captcha selector), #265 (Clerk domain move broke auth), #263/#269 (non-functional after cookie refresh), #262 maintainer asking for a new owner. The installed local `suno` CLI v0.5.7 confirms the read+auth surface works today on macOS.
- Probe-safe endpoint: `GET /api/billing/info/` (read-only; 401 without creds is expected).
- Tier/permission hints: captcha throttle ~after 200 credits of use; Pro/Premier unlock more stems and custom models.

## Top Workflows
1. **Generate a song** (custom lyrics+style+title, or inspiration-from-description, instrumental) — flagship.
2. **Manage a library** — list/search your songs, get clip info, download audio+embedded lyrics, set metadata/visibility, trash.
3. **Transform** — extend from a timestamp, cover in a new style, remaster to a newer model, separate stems.
4. **Lyrics workflow** — generate lyrics, fetch word-level timestamped (aligned) lyrics.
5. **Personas/voices** — reuse a captured vocal style; check credits/plan.

## Table Stakes (every competitor has these — we match + beat)
- generate (custom + inspiration + instrumental), extend, cover, remaster, concat, stems
- generate lyrics, get clip info, get aligned/timestamped lyrics
- list/library, search, get credits/limit, download
- set metadata, set visibility, delete/trash, personas

## Data Layer
- **Primary entities:** clips (songs), personas, lyrics jobs, billing snapshot, playlists.
- **Sync cursor:** `/api/feed/v3` opaque cursor — `cursor = next_cursor` from prior response, walk while `has_more`. (NOT numeric page — the reference gets this wrong.)
- **FTS/search:** local FTS over clip title, tags, lyrics/prompt. Enables offline search the live API can't match (server search is title-text only).

## Codebase Intelligence (from paperfoot/suno-cli Rust source)
- **Base URLs:** `https://studio-api-prod.suno.com` (app API), `https://auth.suno.com` (Clerk).
- **Auth:** `__client` cookie → `GET /v1/client?...` (session id from `last_active_session_id`) → `POST /v1/client/sessions/{sid}/tokens` (JWT) → `Authorization: Bearer <jwt>` on studio-api. Clerk query params `_clerk_js_version=5.117.0&__clerk_api_version=2025-11-10`. Clerk header `authorization: <raw __client value>` (not Bearer). JWT ~1h; treat expired when <30min remain; transparent single-retry refresh on `Token validation failed`.
- **Headers (every studio-api call):** `Authorization: Bearer <jwt>`, `device-id: <uuid>` (from `ajs_anonymous_id` cookie), `browser-token: {"token":"<b64 of {timestamp:ms}>"}` (dynamic per request), `origin: https://suno.com`, `referer: https://suno.com/`, Chrome UA.
- **Endpoints (relative to studio-api-prod):**
  - `GET /api/billing/info/` — credits/plan/models
  - `POST /api/generate/v2-web/` — generate (custom/inspiration/cover/remaster/extend all route here; needs hCaptcha `token`)
  - `GET /api/feed/?ids=<id,id>` — clips by id (batch 2 — Suno bug with 4+)
  - `POST /api/feed/v3` — list + search (opaque cursor; search via `filters.searchText`, `trashed:"False"`)
  - `POST /api/feed/trash` — delete/trash `{ids:[...]}`
  - `POST /api/generate/lyrics/` → `{id}`; `GET /api/generate/lyrics/{id}` — lyrics job
  - `POST /api/generate/concat/v2/` — finalize/concat `{clip_id}`
  - `POST /api/edit/stems/{clip_id}` — stems
  - `GET /api/gen/{clip_id}/aligned_lyrics/v2/` — word timestamps
  - `POST /api/gen/{clip_id}/set_metadata/` — title/lyrics/caption/cover
  - `POST /api/gen/{clip_id}/set_visibility/` — `{is_public}`
  - `GET /api/persona/get-persona-paginated/{persona_id}/?page=0` — persona
  - Documented-but-unimplemented (novel fuel): `GET /api/playlist/me`, `GET /api/trending/`, persona-creation pipeline.
- **Models (`mv`):** chirp-fenix(v5.5), chirp-crow(v5), chirp-bluejay(v4.5+), chirp-auk(v4.5), chirp-v4(v4), chirp-v3-5(v3.5), chirp-v3-0(v3), chirp-v2-xxl-alpha(v2). Remaster: chirp-flounder(v5.5)/chirp-carp(v5)/chirp-bass(v4.5+).

## Corrections over the reference (unmerged-fix intelligence — the user's specific ask)
1. **Feed pagination:** use opaque `next_cursor` + `has_more`, not numeric page (issue #1 / PR #2). Implement real cursor-walking + `--all`.
2. **Cover:** Suno added a now-required `title` field to cover params → HTTP 422 in mainline (issue #3). We send `title`.
3. **Billing schema drift:** `plan`, `total_credits_left`, `period` now optional → mainline parse fails and auth *looks* broken (PR #4). Tolerant parsing + `total_credits_left → credits` fallback.
4. **`--jwt` direct auth** should clear stale Clerk state (PR #4).
5. **Captcha cookie wipe** forces SSO popup every generate (issue #3) → preserve the Clerk handshake cookie in our `auth login --chrome`.

## Competitive landscape
| Tool | Lang | Stars | State |
|---|---|---|---|
| gcui-art/suno-api | TS | ~2960 | Abandoned; owner wanted (#262) |
| SunoAI-API/Suno-API | Py | ~1780 | Stale (Dec 2024) |
| yihong0618/SunoSongsCreator | Py | ~349 | Stale (Aug 2024) |
| Suno-API/Suno-API | Go | ~140 | Stale (Dec 2024); the notable Go client |
| Malith-Rukshan/SunoAI | Py | ~124 | Stale (Jul 2024) |
| AceDataCloud/SunoMCP, CodeKeanu/suno-mcp | Py | ~7/6 | MCP servers (relay-backed / direct) |
| Local installed `suno` v0.5.7 | Rust | — | Most complete reference; 26 commands |

## User Vision (briefing)
- Use `paperfoot/suno-cli` (Rust) as API ground truth.
- Mine its issues/PRs for proposed-but-unmerged fixes and bake them in (done — see Corrections above).
- Auth via logged-in browser session (`auth login --chrome`, Clerk cookie capture). `AUTH_SESSION_AVAILABLE=true`.

## Product Thesis
- **Name:** suno-pp-cli ("Suno CLI") — *The correct, offline-first Suno CLI.*
- **Why it should exist:** Every existing Suno client is abandoned and wrong in ways that matter today — broken pagination, broken cover, stale auth, no local persistence. This one is built from the current contract (post-Clerk-move), bakes in the community's unmerged fixes, persists your whole library to local SQLite for offline search/SQL/agent workflows, and is agent-native (`--json`, `--select`, `--dry-run`, typed exit codes) out of the box.

## Build Priorities
1. **Foundation:** Clerk cookie→JWT auth (`auth login --chrome`), local SQLite store for clips, `sync` (opaque-cursor walk), FTS `search`, `sql`.
2. **Absorb:** every competitor command — generate (custom/inspiration/instrumental), extend, cover (with title fix), remaster, concat, stems, lyrics submit+poll, aligned-lyrics, clip info, list, search, download (with ID3 lyric embed), set metadata, set visibility, trash, persona, credits, models.
3. **Transcend:** offline library analytics + agent-native compounds the live API/competitors can't do (see manifest).
