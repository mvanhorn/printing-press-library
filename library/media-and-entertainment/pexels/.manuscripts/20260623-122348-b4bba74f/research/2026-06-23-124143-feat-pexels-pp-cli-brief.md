# Pexels CLI Brief

## API Identity
- Domain: Free stock photography & video. `api.pexels.com` — REST, JSON, key-authed.
- Users: developers, content creators, AI-video/Shorts pipelines, designers, ML dataset builders, wallpaper/automation hobbyists.
- Data profile: Photos (8 src sizes, photographer, avg_color, alt), Videos (multiple `video_files` by quality/fps + preview frames), Collections (featured, user's own, media). All read-only — Pexels has no write API.

## Reachability Gate
- Decision: **PASS**. `GET https://api.pexels.com/v1/curated?per_page=1` returned 200 with valid JSON + `x-ratelimit-limit: 25000`, `x-ratelimit-remaining: 24671` (2026-06-23).
- **Auth is OPTIONAL for read endpoints.** Probing with a deliberately-invalid key still returned 200 from `/v1/curated` — read endpoints (curated/search/photos/videos/featured) are public + IP-rate-limited. Only `/v1/collections` (my collections) hard-requires a key (401).
- Observed 401 body: `{"status":401,"code":"Unauthorized","message":"Missing API key"}` (richer than the SDK's `{"error":...}` type — model error handling on the observed shape).

## Reachability Risk
- **Low.** API healthy as of 2026-06. `status.pexels.com` clean; official JS wrapper has low issue churn, no "API down" clustering.
- 401s in the wild are user error: the deprecated `Bearer` prefix or the legacy `/videos/` path.
- **403s are policy, not outage** — wallpaper apps and "core-functionality clones" are deliberately gated by Pexels. We are a developer CLI, not a wallpaper clone, so this does not apply, but help text must respect attribution + the "may not replicate core functionality" guideline.
- **Cloudflare gotcha:** requests with NO `User-Agent` header get a 403. The generated client MUST send a User-Agent.
- Probe-safe endpoint for Phase 1.9: `GET /v1/curated?per_page=1` (auth-required; expect 401 without key, 200 with key).

## Auth (contract)
- Header: `Authorization: <RAW_API_KEY>` — **NO `Bearer` prefix.** Confirmed across the official SDK and all 9 MCP servers.
- Env var: `PEXELS_API_KEY` (canonical; community also uses `PEXELS_TOKEN`).
- Keys are free + instant from `pexels.com/api`.

## Base URL & Endpoints (all GET, all read-only)
Single base `https://api.pexels.com/v1/`. Target `/v1/videos/...` (legacy `/videos/` is deprecated).
- `GET /v1/search` — Search Photos. Params: `query`(req), `orientation`(landscape|portrait|square), `size`(large|medium|small = min size), `color`(named enum OR hex), `locale`(28-value enum), `page`, `per_page`(max 80).
- `GET /v1/curated` — Curated Photos. `page`, `per_page`.
- `GET /v1/photos/{id}` — Get Photo. `id`(int).
- `GET /v1/videos/search` — Search Videos. Same as photo search minus `color`.
- `GET /v1/videos/popular` — Popular Videos. `min_width`, `min_height`, `min_duration`, `max_duration`, `page`, `per_page`.
- `GET /v1/videos/videos/{id}` — Get Video. `id`(int). (Doubled `videos/videos` segment is correct.)
- `GET /v1/collections/featured` — Featured Collections. `page`, `per_page`.
- `GET /v1/collections` — My Collections (user's own). `page`, `per_page`.
- `GET /v1/collections/{id}` — Collection Media. `id`(string!), `type`(photos|videos), `sort`(asc|desc), `page`, `per_page`.

No real `random` endpoint — wrappers fake it via `/curated` + random page.

## Response Shapes (high-gravity fields)
- **Photo**: id, width, height, url, photographer, photographer_url, photographer_id, avg_color, alt, liked, src.{original,large2x,large,medium,small,portrait,landscape,tiny}. (large2x=940×650@2x, portrait=800×1200 crop, landscape=1200×627 crop, tiny=280×200 crop — crops change aspect ratio, medium/small scale.)
- **Video**: id, width, height, url, image, duration, user.{id,name,url}, video_files[].{id,quality(hd|sd|hls),file_type,width,height,link,fps}, video_pictures[].{id,picture,nr}.
- **Collection**: id(string), title, description, private, media_count, photos_count, videos_count.
- **Envelopes**: `{page, per_page, total_results, next_page, prev_page, photos[]|videos[]|collections[]|media[]}`. `next_page`/`prev_page` are **full URLs**, present only when a page exists. Collection `media[]` items carry a `type` ("Photo"|"Video").

## Rate Limits
- 200 req/hour, 20,000/month default (higher tiers manual-apply only).
- Headers `X-Ratelimit-Limit`, `X-Ratelimit-Remaining`, `X-Ratelimit-Reset`(UNIX ts) — **returned only on 2xx**, absent on the 429 you most want them on.

## Errors
- Body `{"error": "<message>"}`. 429 = rate-limit exceeded (no rate headers on it).

## Data Layer
- Primary entities: photos, videos, collections, collection_media. Plus a downloads/attribution ledger (local-only).
- Sync cursor: page-based; normalize the full-URL `next_page` into an int cursor.
- FTS/search: index synced photo `alt`/`photographer`/`query` and video `user.name`/tags for offline recall.

## Top Workflows
1. **Search → pick resolution → bulk download** with filename/folder templating. The single most common ecosystem workflow (whole cottage industry of download scripts).
2. **Scene-matched b-roll for AI/faceless video** (portrait for Shorts): search videos by orientation + duration, grab best `video_file`.
3. **License-compliant attribution** — produce `attribution`/`attribution_html` + per-file `.meta.json` sidecars / `SOURCES.md`.
4. **Curated/collection browsing** including the user's own collections (only 2 ecosystem tools cover user collections).
5. **Agent-native search** — structured `{data, meta}` JSON with field projection so agents don't drown in 8-size `src` blobs.

## Table Stakes (must match)
- Photo search w/ all filters (orientation/size/color/locale), curated, get-by-id.
- Video search, popular (duration filters), get-by-id.
- Collections: featured, mine, media (type/sort).
- `--json` output; download by chosen resolution; auth login/status.

## Codebase Intelligence (from MCP/wrapper source)
- afshinator MCP is the best LLM-facing output shape: renamed fields (creatorName, mediaUrl=large2x||medium), enforced attribution, best-video-file selector.
- pexels-media skill writes `X.jpg.meta.json` sidecars (id, URLs, photographer, avg_color, alt, license, attribution+attribution_html) — strongest attribution pattern in the ecosystem.
- agynio Rust CLI is the only tool with field-projection JSON (`--fields` dot-paths + `@ids @urls @files @all`) and a `{data,meta}` envelope.

## User Vision
- (none provided — user chose "Let's go")

## Product Thesis
- **Name:** pexels-pp-cli ("pexels")
- **Why it should exist:** No single tool covers the intersection of *full collections* + *rate-limit-aware bulk download* + *field-projection JSON* + *attribution sidecars* + *surfaced quota* + *a local store that makes search/dedup work offline*. Every competitor does a slice; none compounds them. We absorb all of it and add transcendence features only possible with a SQLite layer (cross-session dedup, attribution ledger, best-fit resolution picking, quota forecasting).

## Build Priorities
1. Data layer for photos/videos/collections + downloads/attribution ledger; sync/search/SQL path.
2. Full API parity: all 9 endpoints with every documented filter; raw-key auth + User-Agent; surfaced X-Ratelimit-* on every call.
3. Bulk download with resolution-picking, filename/folder templating, attribution sidecars, client-side dedup.
4. Transcendence: quota forecasting, best-fit resolution picker, attribution ledger/SOURCES export, cross-session dedup, b-roll shot-list assembler.
