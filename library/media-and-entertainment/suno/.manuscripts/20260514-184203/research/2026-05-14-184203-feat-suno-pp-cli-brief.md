# Suno CLI Brief

## API Identity

- Domain: AI music generation. Text/lyrics + style → audio clips (MP3 + MP4 + cover art + word-aligned lyrics).
- Users: Indie musicians, content creators, agent builders, jingle/podcast producers. The user prompting this run is a logged-in Suno account holder who wants prompt-to-song from the terminal.
- Data profile: User has a `clips` library (songs they've generated). Each clip carries title, lyrics, tags, persona, model, audio_url, video_url, image_url, duration, created_at, like_count, is_public, parent_id (for extends/concats), generation params. Auxiliary entities: personas, models, credits/quota.

## Reachability Risk

**Moderate to elevated.**

- Suno has **no official public API**. All access is reverse-engineered through suno.com's web app, which uses Clerk authentication.
- Top wrapper `gcui-art/suno-api` has 5+ open issues in the last 90 days reporting CAPTCHA failures (#258, #263), auth/session expiration (#269), and one explicit "still maintaining?" inquiry (#270). Project owner asked for a takeover (#262).
- CAPTCHA fires more often on Linux/Windows than macOS — 2Captcha integration is the common workaround.
- Auth tokens (Clerk JWTs) expire in ~1 hour; require automatic refresh from a longer-lived `__client` session cookie.
- **Mitigation:** ship Surf transport (Chrome TLS fingerprint) by default + Chrome cookie import so the CLI inherits the user's logged-in browser session instead of re-authenticating from scratch. Validate reachability before generating.

## Top Workflows (ordered by user value)

1. **Generate a song from a prompt** (the user's stated #1 goal). `suno generate "a synthwave anthem about debugging at 3am"` → song appears in account, audio downloads locally. Custom mode adds title, lyrics, tags, persona, model.
2. **Browse / search my library**. List recent, search by title/tags/lyrics, get detailed clip info, check generation status.
3. **Extend or remix an existing clip**. Continue a clip past its 4-minute cap, swap models (remaster), generate cover, concat segments into a full song.
4. **Lyrics-only generation** (free, no credits). Iterate on lyrics before committing credits to a full song.
5. **Download with embedded metadata**. MP3 + ID3 + USLT (plain lyrics) + SYLT (word-aligned timestamps) + cover art.

## Table Stakes

(Every competing tool ships these. The CLI MUST match all of them.)

- `generate` with all 4 modes (custom, description, lyrics-only, instrumental)
- `extend`, `concat`, `cover`, `remaster`, `stems`
- `list`, `search`, `info`, `status`, `credits`, `models`
- `download` with ID3 embedding (USLT + SYLT), cover art
- `delete`, `set` (rename / re-lyric / re-caption), `publish`/`unpublish`
- `timed-lyrics` (LRC export)
- `persona` browse
- Cookie / JWT / browser-extraction auth flow

## Data Layer

- **Primary entities:** `clips`, `personas`, `models`, `credits_snapshots`, `generations` (the request that produced one or more clips).
- **Sync cursor:** `created_at` descending from `POST /api/feed/v3`; persist last-seen `clip_id` to resume.
- **FTS5/search:** `clips_fts` over `title`, `tags`, `prompt`, `lyrics`. Persona FTS over `name` + `description`. Cross-entity search via existing `resources_fts` framework.
- **Computed columns for transcendence:** clip age, days-since-last-extended, credit-cost-class, lyrics-word-count, time-of-day-generated.

## Codebase Intelligence

(Pulled from MCP server source + community wrapper analysis; no DeepWiki entry exists yet for Suno wrappers.)

- **Auth:** Clerk session cookie (`__client` from `clerk.suno.com` domain) → exchange for JWT via Clerk's `/v1/client/sessions/{sid}/tokens` endpoint → `Authorization: Bearer <jwt>` on every API call. JWT expires ~1 hour; cookie lasts weeks.
- **Data model:** Clips own everything. Generations are short-lived metadata records that point to one or more clips. Persona is a UUID-keyed voice prompt.
- **Rate limiting:** Soft — empirical 429s on bursts > ~3 simultaneous generations. Free tier ≈ 10 credits/day; paid plans 2500/month. Each full song = 10 credits, lyrics-only = 0.
- **Architecture insight:** Suno's web app drives a versioned set of endpoints under `/api/generate/v2-web/`, `/api/feed/v3`, `/api/gen/{id}/aligned_lyrics/v2/`. Older wrappers still call `/api/generate` and `/api/custom_generate` — likely still proxied internally but the v2-web path is what the live web client uses today.

## User Vision

The user has an active Suno account and is logged in to suno.com in Chrome. They want to "send prompts and generate songs from the CLI" — i.e., the headline experience is `suno generate "..."` → song produced and downloaded. There is no official API, so the spec must come from browser-sniffing the live web app.

## Product Thesis

- **Name:** `suno-pp-cli`
- **Headline:** Every Suno feature, plus a local SQLite library, offline FTS5 search, MCP-native agent surface, and a single-binary Go install.
- **Why it should exist:**
  1. Every existing Suno CLI ships in Python or Node — venv hassle, pip dependency hell, no static binary. `suno-pp-cli` is one Go binary.
  2. **No Suno tool today persists your library to a local queryable DB.** Search across your entire generation history, run SQL, find drift, run analytics — only possible with our store-first architecture.
  3. **Native MCP server with safety annotations.** The existing Suno MCPs (`AceDataCloud/SunoMCP`, `mcp-suno` on PyPI) all proxy through paid 3rd-party gateways. None talk to the real Suno API with your real account.
  4. Agent-native by default: every command produces `--json`, supports `--select` field paths, has typed exit codes (0/2/3/4/5/7), and `--dry-run` on mutations.
  5. **Reachability hardened:** Surf transport (Chrome TLS fingerprint) + Chrome cookie import + automatic JWT refresh from the stored Clerk cookie — survives CAPTCHA/Cloudflare challenges that kill curl-based wrappers.

## Build Priorities

1. **Browser-sniff suno.com with the user's logged-in session** to capture the live endpoint surface. Validate that `/api/generate/v2-web/`, `/api/feed/v3`, `/api/gen/{id}/aligned_lyrics/v2/`, and the persona endpoint are still the active routes.
2. **Cookie-based auth** with Chrome import (`auth login --chrome`) and JWT refresh from the Clerk session. Honor `SUNO_COOKIE` env var as fallback.
3. **Generate path first** — single command `suno generate "<prompt>"` must work end-to-end (submit, poll status, download MP3 with embedded lyrics).
4. **Local SQLite store** with `sync` to pull `/api/feed/v3` into the `clips` table, `search` (FTS5), `sql` (read-only ad-hoc).
5. **Full absorbed feature set:** generate (custom + description + lyrics + instrumental), extend, concat, cover, remaster, stems, list, info, status, credits, models, download (ID3 + USLT + SYLT), delete, set, publish, timed-lyrics, persona.
6. **Transcendence features** that compound on the local store (see absorb manifest).
