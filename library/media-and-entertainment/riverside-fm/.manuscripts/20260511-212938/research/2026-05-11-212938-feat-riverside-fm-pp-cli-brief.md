# Riverside (riverside.fm / riverside.com) CLI Brief

> Run: 20260511-212938 · API slug: riverside-fm · Goal: download transcripts → audio → video from the user's own Riverside account.

## API Identity
- **Domain**: remote recording platform (podcasts, video interviews, webinars). Founders Gideon & Nadav Keyson; RiversideFM Inc, Palo Alto + Tel Aviv R&D. ~$77M raised through Series C (Dec 2024).
- **Users**: podcasters, video creators, remote-interview journalists, webinar hosts, enterprise teams.
- **Data profile**: hierarchical recording corpus — Production → Studio → Project → Recording → Track (per-participant) → File (raw_video, raw_audio, etc.) + Transcription (srt, txt). Plus workspace-level Exports (Magic-edited / Studio-edited renders).

## Reachability Risk
- **None for the official Business API** (`platform.riverside.fm`): Cloudflare in front, but unauth requests return structured 401 with documented rate-limit headers. No JS challenge or Turnstile on this host.
- **None for the studio app** (`riverside.fm`, fronted by CloudFront — not Cloudflare): live tested 200 OK on `/login`.
- **Per-call rate limit: 1 req/sec** on every Business API endpoint (sometimes 1 req/sec/unique-resource). Hard.
- v1 and v2 are **sunset 2026-02-24** — `410 Gone` already confirmed live. v3 is the only viable surface.

## Top Workflows (in priority order, per the user's stated goal)
1. **Download transcript for a recording** — `srt` or `txt`, one call, body returned directly.
2. **Download audio tracks for a recording** — per-participant `.wav` files; redirects to signed Cloudfront URL.
3. **Download video tracks for a recording** — per-participant `.mp4`; same redirect pattern.
4. **List productions / recordings** — discover what's downloadable, with cursor/page filters.
5. **Bulk export an entire studio or date range** — iterate recordings, download all preferred-tier assets locally (this is the killer workflow nobody else automates).

## Two API Surfaces (must support both)

| Surface | Host | Auth | Tier needed | Status |
|---|---|---|---|---|
| **Official Business API v3** | `platform.riverside.fm` ≡ `platform.riverside.com` | `Authorization: Bearer <api-key>` | **Business plan only** | Documented at docs.riverside.fm/endpoints-reference/v3 (12 endpoints). v1/v2 sunset 2026-02-24. |
| **Private frontend API** | `riverside.fm` | `sweetsesh` session cookie (HttpOnly, signed) | Any tier (Free/Pro/Live/Webinar/Business) | Undocumented. Used by the studio web app. **Must browser-sniff while logged-in to capture.** |

Business API access is custom-priced + CSM-gated (~$5,400/yr+). Most users will need the cookie path.

## Data Layer
- **Primary entities**:
  - `productions` (workspace teams)
  - `studios` (series of content)
  - `projects` (folders / episodes)
  - `recordings` (takes; nested tracks + transcription + files)
  - `tracks` (per-participant or screenshare/media-board)
  - `files` (raw_audio/raw_video/aligned_video/compressed_audio/cloud_recording)
  - `exports` (workspace-level edited renders)
- **Sync cursor**: `start_date` / `end_date` on `/api/v3/recordings`, page-based pagination (page 0 default, 20/page, newest first). `/api/v3/exports` uses page starting at 1.
- **FTS/search**: search across recording titles, project names, studio names, transcription content (the SRT/TXT we cache locally). Substring + token search are both useful.

## Codebase Intelligence
- Source: Phase 1 docs sweep + live curl probes
- **Auth (Business API)**: `Authorization: Bearer <opaque-key>`, env `RIVERSIDE_API_KEY` (canonical). Generated at `riverside.com/dashboard → Team settings → API key`.
- **Auth (private API)**: `sweetsesh` session cookie set on `.riverside.fm`. HttpOnly. Signed `s%3A...sig` shape (express-session compatible).
- **Data model**: see hierarchy above. Recording `status ∈ {uploading, processing, ready}` — least-baked-wins across tracks/transcription. Track `status ∈ {done, processing, uploading, stopped}`. Transcription `status ∈ {transcribing, done}`. Export `status ∈ {pending, processing, ready, failed}`.
- **Rate limiting**: 1 req/sec per endpoint, headers `x-ratelimit-limit / -remaining / -reset` populated on every response. Use `cliutil.AdaptiveLimiter`.
- **Architecture**: download endpoints return **HTTP 301** with `Location:` pointing to a short-lived signed URL on `storage.riverside.fm/signed-url/<obj>.{wav,mp4,mp4}?token=...`. CLI must follow redirects and download asset body promptly (treat URL TTL as ephemeral — re-fetch before downloading).

## User Vision
- User wants to **download transcripts, audio, or video (in priority order) from THEIR riverside account**. "Priority order" is the killer — they want a single command that fetches the most useful asset that exists for each recording without ceremony.
- User is logged in to riverside.com in Chrome — session cookie capture is available immediately.

## Product Thesis
- **Name**: `riverside-fm-pp-cli` (canonical: "Riverside CLI")
- **Why it should exist**: There is no other tool that automates bulk download from Riverside. The official Make.com integration covers single-recording downloads only and is Business-tier-only. The `dlt source: riverside` library is Business-tier-only and stale (still hits v2). For 99% of Riverside users (Pro / Live / Webinar / Free), there is NO programmatic export today — they manually click Download per recording in the UI. This CLI is the first programmatic export tool for non-Business Riverside users, and a better one for Business users (priority-fallback download, batch, local FTS, transcript reformatting).

## Build Priorities
1. **Foundation**: dual-auth client (Bearer for Business, sweetsesh cookie for everyone else), SQLite store for productions/studios/projects/recordings/tracks/files/exports/transcriptions, sync command.
2. **Absorbed (parity with every existing tool)**: list recordings/productions/exports/studios, get individual recording, download single file, download transcript, delete recording, delete export, registrant create/list, asset polling.
3. **Transcendence (only we have these)**:
   - `dl <recording-id>` — priority-fallback download (transcript first, then audio, then video) — the user's stated goal as one command.
   - `bulk <studio|date-range>` — download every recording in a studio/timeframe with progress + resume.
   - `tx <recording-id> --format vtt|json` — convert SRT to formats Riverside doesn't natively expose (VTT, timecoded JSON, plain).
   - `search "<query>"` — offline FTS across recording titles AND transcript bodies (the only way to grep your own podcast catalog).
   - `wait <recording-id>` — block until tracks finish processing (no UI to poll); useful for editor-side automation.

## Source Priority
- Single source (Riverside). No multi-source priority gate.

## Reachability Probe
- `https://riverside.com/` — standard_http, 200 (marketing site)
- `https://app.riverside.com/` — browser_clearance_http, 403 (Cloudflare; user-app shell)
- `https://api.riverside.com/` — browser_clearance_http, 404 (Cloudflare)
- `https://platform.riverside.fm/` — Business API host (Cloudflare → Envoy, Bearer-gated; structured 401)
- `https://riverside.fm/` — studio app (CloudFront, NOT Cloudflare; sweetsesh-cookie-gated)
- **Implication**: the CLI ships **standard HTTP transport** (no Surf needed) because the Business API answers vanilla curl with a proper 401 envelope. The cookie-based frontend will be discovered via browser-sniff; if its surface is also vanilla HTTP plus the cookie, we use the same transport.

## Open Questions for Browser-Sniff
1. Does the studio dashboard hit `platform.riverside.fm/api/v3/...` with the session cookie? Or a parallel `riverside.fm/api/v4/...` surface? **Load-bearing for client shape.**
2. What's the exact request the **Download high-quality tracks** button issues for a recording?
3. What's the exact request the Transcript **Download → text / SRT** button issues?
4. Does any download flow bypass `platform.*` and serve direct signed CloudFront URLs from the studio app?
5. Has the 2023-vintage `/login-react` path been renamed? (Sniff to confirm current login POST — only needed if we'd want to script email/password login, which we don't because `auth login --chrome` cookie import is simpler.)

## Sources
- docs.riverside.fm/endpoints-reference/v3/ (12 endpoints, object reference, llms.txt)
- github.com/PodInvite/riverside-api-docs (2023 OpenAPI of private surface — stale but directional)
- dlthub.com/context/source/riverside (Business API wrapper, stale paths)
- apps.make.com/riverside-goh4mb (community Make.com integration)
- gist.github.com/zephraph/...export-db (browser-only IndexedDB recovery — out-of-scope)
- cleanvoice.ai/blog/riverside-api-review/ (third-party review of API + pricing)
- riverside.com/pricing (live extraction)
- Live curl probes on `platform.riverside.fm`, `riverside.fm`, `riverside.com/faq` (2026-05-12)
