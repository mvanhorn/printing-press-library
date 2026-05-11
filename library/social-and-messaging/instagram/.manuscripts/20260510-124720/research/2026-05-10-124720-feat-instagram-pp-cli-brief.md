# Instagram CLI Brief

## API Identity
- Domain: `instagram.com` (web), `i.instagram.com` (mobile private). CDN: `*.cdninstagram.com`, `*.fbcdn.net` for media URLs.
- Users: Meta-owned social network; ~2B MAU. Read-only access to arbitrary public/private content is OFF-LIMITS per ToS — every realistic image-extraction tool operates on the reverse-engineered web/mobile API, not the official Graph API.
- Data profile: image (JPEG/HEIC), video (MP4), captions, EXIF stripped server-side, timestamp (`taken_at`), location ID, user ID (PK), shortcode, sidecar/carousel children, story expiry, highlight reels.
- Surfaces (three layers, only one is useful for image extraction):
  - **Graph API** (`graph.facebook.com/v21.0/`) — OAuth2 via Facebook Login + Business Verification. Reads ONLY the Instagram Business/Creator account the token belongs to (+ tagged/mentioned media on it, hashtag *search* for own account). **Cannot read arbitrary profiles/posts.** Useless for the user's stated goal.
  - **Basic Display API** — DEPRECATED September 2024 ([Meta announcement](https://developers.facebook.com/blog/post/2024/09/04/update-on-instagram-basic-display-api/)). Was OAuth for personal accounts to read own media; replaced by Instagram API with Instagram Login. Still business-account-scoped. Irrelevant.
  - **Reverse-engineered web/mobile API** (the only path that works for "download all photos from any public profile"):
    - Web endpoints: `GET /api/v1/users/web_profile_info/?username=<u>` (returns user object incl. `id`, profile pic, bio), `POST /graphql/query` with `doc_id` + `variables` (paginated media), `GET /api/v1/feed/user/<id>/username/<u>/`, `GET /api/v1/users/<id>/info/`, `GET /api/v1/media/<media_id>/info/`, `GET /api/v1/feed/reels_media/?reel_ids=<id>` (stories), `GET /api/v1/highlights/<user_id>/highlights_tray/`, `GET /api/v1/tags/web_info/?tag_name=<t>`, `GET /api/v1/feed/saved/`, `GET /api/v1/feed/liked/`.
    - Required headers: `User-Agent` (Chrome or pinned IG mobile UA like `Instagram 269.0.0.18.75 Android (...)`), `X-IG-App-ID: 936619743392459` (web) / `567067343352427` (iOS), `X-CSRFToken: <csrftoken cookie>`, `X-IG-WWW-Claim: <ws-claim from prior response>`, `X-ASBD-ID: 198387`, `X-Requested-With: XMLHttpRequest`, `Referer: https://www.instagram.com/`.
    - Required cookies: `sessionid` (auth), `csrftoken` (CSRF), `ds_user_id` (numeric uid), `mid`, `ig_did`. Anonymous fetches still work for some endpoints but get throttled to ~1–2 req / 30s and routinely 401 in 2025/2026.

## Reachability Risk
- **HIGH** — Instagram has aggressively tightened anti-scraping since Q4 2024. Evidence:
  - [instaloader/instaloader#2511](https://github.com/instaloader/instaloader/issues/2511) (Feb 2025): "401 Unauthorized 'Please wait a few minutes' — persists regardless of login status, IP, or account."
  - [instaloader/instaloader#2501](https://github.com/instaloader/instaloader/issues/2501): "401 back for all `get_posts()` requests."
  - [instaloader/instaloader#2445](https://github.com/instaloader/instaloader/issues/2445): "400/401 behavior since one month" — API locked down for unauthenticated Python clients in v4.14.1.
  - [instaloader/instaloader#2307](https://github.com/instaloader/instaloader/issues/2307): "Can only scrape 12 or so posts before `graphql/query` 401" — pagination wall.
  - [instaloader/instaloader#798](https://github.com/instaloader/instaloader/issues/798) + [#1787](https://github.com/instaloader/instaloader/issues/1787): `checkpoint_required` blocking login.
  - [subzeroid/instagrapi#2259](https://github.com/subzeroid/instagrapi/issues/2259): `cl.login()` hangs for minutes on 2FA/challenge with no callback.
  - [subzeroid/instagrapi#2287](https://github.com/subzeroid/instagrapi/issues/2287): "endless account login" — captcha/challenge wall.
  - [subzeroid/instagrapi#2267](https://github.com/subzeroid/instagrapi/issues/2267): requests taking ~45s (rate-limit shadow throttle).
  - [subzeroid/instagrapi Discussion #2224](https://github.com/subzeroid/instagrapi/discussions/2224): "Is There a Real Risk of Getting Banned" — yes, repeated fresh logins from new IPs trip account ban.
  - [yt-dlp/yt-dlp#16311](https://github.com/yt-dlp/yt-dlp/issues/16311), [#10129](https://github.com/yt-dlp/yt-dlp/issues/10129), [#8290](https://github.com/yt-dlp/yt-dlp/issues/8290): "Requested content is not available, rate-limit reached or login required" — recurring.
  - Hacker News [item 43398363](https://news.ycombinator.com/item?id=43398363): users report dormant accounts banned after light yt-dlp use against IG.
- **Mitigation table-stakes**: stable device-ID, persistent session (not re-login per run), one IP per account, residential proxy support, exponential backoff, treat anonymous endpoints as opportunistic. Brand-new accounts are the most blockable; aged accounts with browser-imported cookies survive longer.

## Top Workflows
1. Download all photos from a public profile by `<username>`.
2. Download all media (image + video + carousel children) for a single post/reel by URL or shortcode.
3. Download media for a hashtag (top + recent, capped) — requires login.
4. Download own saved (`:saved`) and liked posts — auth-only.
5. Download stories + highlights for a profile — auth-only.
6. Bulk archive a profile with caption sidecar (`.txt`), JSON metadata, geotag, comments, timestamps — incremental, resumable.
7. Re-download dedup (skip already-fetched shortcodes) for cron-style mirroring.
8. Tagged-in posts for a profile (`/<user>/tagged/`).

## Table Stakes

### instaloader (Python CLI — reference implementation)
Source: <https://instaloader.github.io/cli-options.html>

**Targets** (positional):
- `profile` — public, or private with login
- `"#hashtag"` — login required
- `@profile` — all profiles followed by `profile`
- `%location_id` — posts at numeric location ID, login required
- `:feed` — logged-in user's home feed
- `:stories` — followees' stories, login required
- `:saved` — own saved collection, login required
- URL of single post (`https://www.instagram.com/p/<shortcode>/`)

**What to download (per profile)**:
`--no-posts`, `--no-profile-pic`, `--stories/-s`, `--highlights`, `--tagged`, `--reels`, `--igtv`

**What to download (per post)**:
`--no-pictures`, `--no-videos/-V`, `--no-video-thumbnails`, `--geotags/-G`, `--comments/-C`, `--no-captions`, `--post-metadata-txt`, `--storyitem-metadata-txt`, `--slide` (sidecar slice e.g. `1-3`), `--no-metadata-json`, `--no-compress-json`

**Which posts** (filters):
`--fast-update/-F`, `--latest-stamps [FILE]`, `--post-filter EXPR / --only-if EXPR`, `--storyitem-filter EXPR`, `--count N`, `--abort-on STATUS_LIST`

**How to download** (layout/retry):
`--dirname-pattern`, `--filename-pattern`, `--title-pattern`, `--sanitize-paths`, `--resume-prefix`, `--no-resume`, `--user-agent`, `--max-connection-attempts N`, `--request-timeout N`, `--commit-mode`, `--no-iphone`

**Login**:
`--login/-l USER`, `--password/-p` (discouraged), `--load-cookies/-b BROWSER` (Arc/Brave/Chrome/Chromium/Edge/Firefox/LibreWolf/Opera/Opera_GX/Safari/Vivaldi), `--cookiefile PATH`, `--sessionfile FILE`, `--no-iphone` (web-UA mode)

**Misc**: `--quiet/-q`, `+args.txt` (read args from file), `--help`

### gallery-dl `--extractor instagram` (Python, multi-site)
Source: <https://github.com/mikf/gallery-dl/blob/master/gallery_dl/extractor/instagram.py>

Extractor subclasses (each is a URL-pattern handler):
- `InstagramUserExtractor` — dispatches to default subscope (`posts`)
- `InstagramInfoExtractor` — `/USER/info/` (profile metadata only)
- `InstagramAvatarExtractor` — `/USER/avatar/`
- `InstagramPostsExtractor` — `/USER/posts/`
- `InstagramReelsExtractor` — `/USER/reels/`
- `InstagramStoriesExtractor` — `/stories/USER/`
- `InstagramHighlightsExtractor` — `/USER/highlights/`
- `InstagramTaggedExtractor` — `/USER/tagged/`
- `InstagramFollowingExtractor` — `/USER/following/` (queues per-user)
- `InstagramTagExtractor` — `/explore/tags/<tag>/`
- `InstagramPostExtractor` — `/p/<shortcode>/`, `/reel/<shortcode>/`, `/tv/<shortcode>/`
- `InstagramSavedExtractor` — `/<user>/saved/`
- `InstagramCollectionExtractor` — saved collections
- `InstagramGuideExtractor` — `/guide/<id>/`

Per-extractor config keys: `api` (`rest`|`graphql`), `cookies`, `cookies-from-browser`, `include` (`posts,stories,reels,highlights,collections,avatar,tagged`), `order-files`, `order-posts`, `previews`, `sleep-request`, `videos`, `metadata`.

Global flags relevant: `--cookies <file>`, `--cookies-from-browser <browser>`, `--input-file <list>`, `--write-metadata`, `--write-info-json`, `--download-archive <file>` (dedup), `--range`, `--filter`.

### yt-dlp Instagram
Source: <https://github.com/yt-dlp/yt-dlp/tree/master/yt_dlp/extractor/instagram.py>

Supported URL shapes: `/p/<shortcode>/`, `/reel/<shortcode>/`, `/tv/<shortcode>/`, `/<user>/` (playlist), `/stories/<user>/<story_id>/`. **Reels are its strongest case; image-only posts are weak.** Auth flags: `--cookies <file>`, `--cookies-from-browser <browser>`, `--username/--password`, `--netrc instagram`. Recent issues (e.g. [#16311](https://github.com/yt-dlp/yt-dlp/issues/16311)) show frequent "rate-limit reached or login required" on un-cookied use.

### 4K Stogram (closed-source GUI, freemium)
Features: subscribe to user/hashtag/location, auto-update on schedule, paid concurrent-account limit, download stories+highlights+tagged, built-in viewer, export to JPG/MP4. No CLI, no scripting. Cookie-based auth.

### Go-native options (notable but immature)
- [`ahmdrz/goinsta`](https://github.com/ahmdrz/goinsta) — unofficial private-API client in Go, stdlib-only HTTP/2. Long-stale (last meaningful commit ~2021); auth flow likely broken vs 2025 IG.
- [`Vorkytaka/instagram-go-scraper`](https://github.com/Vorkytaka/instagram-go-scraper) — `GetAccountByUsername`, `GetMediaByCode`, `GetAccountMedia`, hashtag search. Anonymous-only, no session handling.
- [`siongui/instago`](https://pkg.go.dev/github.com/siongui/instago) — handles photos/videos/stories/highlights, function-level API, no CLI.
- [`jrudio/go-instagram-downloader`](https://github.com/jrudio/go-instagram-downloader) — toy/benchmark project.
- [`hcarriz/instago`](https://github.com/hcarriz/instago) — minimal user scrape.
- **[`ibrahimhajjaj/ig-dl`](https://github.com/ibrahimhajjaj/ig-dl)** — DIRECT COMPETITOR. Single Go binary that: captures session over Chrome DevTools Protocol (CDP, including Chrome M144 `chrome://inspect` toggle against real default profile), writes `cookies.txt`, then routes URLs — reels to `yt-dlp`, posts/stories/highlights/saved to `gallery-dl`. Subcommands: `ig-dl <url>`, `ig-dl user <handle>`, `ig-dl saved`, `ig-dl login [--import session.json]`, `ig-dl logout`, `ig-dl status`, `ig-dl browsers`, `ig-dl mcp`. Global `--out`, `--json`. MCP tools: `ig_download_url`, `ig_download_urls`, `ig_download_user`, `ig_download_saved`, `ig_session_status`, `ig_login`. **This is the bar to clear; ig-dl already wraps gallery-dl + yt-dlp and ships an MCP server.** A pure-native Go competitor (no Python deps) would be the meaningful differentiator.

## Data Layer
- **Primary entities** (SQLite tables):
  - `profiles` (pk=ig_user_id INTEGER, username TEXT UNIQUE, full_name, bio, is_private, follower_count, following_count, post_count, profile_pic_url, fetched_at)
  - `posts` (pk=shortcode TEXT, ig_media_pk INTEGER, owner_id, caption, taken_at, location_id, media_type (PHOTO/VIDEO/CAROUSEL), like_count, comment_count, raw_json BLOB)
  - `media_items` (pk=id, post_shortcode FK, carousel_index INT, kind (image/video), cdn_url, local_path, width, height, sha256, bytes, downloaded_at) — one row per actual file
  - `hashtags` (pk=tag, post_count, fetched_at)
  - `locations` (pk=location_id, name, lat, lng, slug)
  - `stories` (pk=story_id, owner_id, taken_at, expires_at, media_url, local_path)
  - `highlights` (pk=highlight_id, owner_id, title, cover_url) + `highlight_items` join to media
  - `sessions` (pk=username, cookies_json BLOB, device_id, user_agent, last_used_at, status enum)
  - `download_history` (shortcode, status, attempts, last_error, last_attempt_at) — supports `--fast-update` dedup
  - `rate_limit_events` (endpoint, status_code, ts, response_headers) — for client-side backoff modeling
- **Sync cursor**: per-profile `(max_taken_at, max_shortcode_seen)`. IG `graphql/query` returns posts in reverse-chronological with cursor pagination (`end_cursor` + `has_next_page`), so stop-when-shortcode-already-seen replicates instaloader's `--fast-update` cheaply. Stories use `expires_at`; highlights use `highlight_id` as primary cursor.
- **FTS/search** (FTS5 virtual tables):
  - `posts_fts(caption, hashtags, mentions, location_name, owner_username)` — most useful surface (search downloaded archive by text)
  - `profiles_fts(username, full_name, bio)`
  - Optional: alt-text accessibility caption (`accessibility_caption` field on IG media — auto-generated image description, useful for offline image search)

## Codebase Intelligence

### instaloader
- Source: <https://github.com/instaloader/instaloader> (Python, ~13k stars).
- Entry: `instaloader.__main__:main` → arg parse → `Instaloader.download_*` methods.
- Auth: pure-web flow. POSTs login to `/accounts/login/ajax/` with `csrftoken` cookie pre-fetched from `/`. Stores `sessionid`, `csrftoken`, `ds_user_id`, `mid`, `ig_did` to pickle at `~/.config/instaloader/session-<user>`. Browser import via `browser_cookie3` supports Arc/Brave/Chrome/Chromium/Edge/Firefox/LibreWolf/Opera/Opera_GX/Safari/Vivaldi.
- Headers: `X-IG-App-ID: 936619743392459` (hard-coded web app id), `X-Requested-With: XMLHttpRequest`, `X-CSRFToken`, `Referer: https://www.instagram.com/`, UA pinned to Chrome/142 on Linux unless `--user-agent` overrides. `--no-iphone` toggles between iPhone-UA private endpoints and web UA.
- Private endpoints used: `graphql/query` with hard-coded query hashes (`PROFILE_QUERY`, `PROFILE_FEED_QUERY`, `HASHTAG_QUERY`, etc.); `api/v1/users/web_profile_info/`; `api/v1/feed/reels_media/`; `api/v1/highlights/<uid>/highlights_tray/`; `api/v1/users/<id>/info/`.
- Rate limiting: `RateController` class — per-endpoint sliding window, retries with exponential backoff, sleeps when hitting 429/401-rate-limit.
- Architecture: `InstaloaderContext` (HTTP session + auth) → `Instaloader` (orchestrator + filename rendering) → `Post`/`Profile`/`StoryItem`/`Highlight`/`Hashtag` structures (lazy-load attributes from cached JSON).

### instagrapi
- Source: <https://github.com/subzeroid/instagrapi> (Python, ~4.7k stars, fork of `adw0rd/instagrapi`).
- Uses BOTH web flow and **iOS mobile private API** (`i.instagram.com/api/v1/`) — more endpoints, more data, more fragile.
- Auth: password → device-fingerprint generation (`device_settings` dict: model, android_version, etc., **persists per session** — re-login from scratch is the #1 ban trigger), `cl.login()` → handles 2FA + `challenge_required` via `challenge_code_handler` callback. `dump_settings()`/`load_settings()` persists device + cookies as JSON. Alt path: `login_by_sessionid("<sessionid>")` for cookie import.
- Headers: pinned Instagram app UA (e.g. `Instagram 269.0.0.18.75 Android (...)`), `X-IG-App-ID: 567067343352427` (iOS), `X-IG-Capabilities`, `X-IG-Connection-Type`, `X-IG-WWW-Claim`, `Authorization: Bearer IGT:2:<token>` for some endpoints, signed-body for sensitive endpoints (like/follow/login) — HMAC of body keyed by IG signature key.
- Cookies: `sessionid`, `csrftoken`, `ds_user_id`, `mid`, `ig_did`, `rur`.
- Data model: `Media`, `User`, `Story`, `Highlight`, `Collection`, `Comment`, `Location`, `Hashtag`, `DirectThread`, `DirectMessage` Pydantic models.
- Key download methods: `user_medias(user_id, amount)`, `photo_download(media_pk, folder)`, `video_download(media_pk)`, `album_download(media_pk)`, `story_download(story_pk)`, `highlight_pk_from_url`, `hashtag_medias_top/recent(name, amount)`, `location_medias_top/recent(location_id, amount)`, `user_stories(user_id)`, `user_saved_medias()`.
- Discovery surfaces (`Features` section): `chaining`, `fetch_suggestion_details`, `discover_recommended_accounts_for_category_v1`, `user_stream_*`, `user_web_profile_info_v1`. v2 search SERPs: `fbsearch_accounts_v2`, `fbsearch_reels_v2`, `fbsearch_topsearch_v2`, `fbsearch_typehead`. Fallback `media_info_v2` for ad-tagged/sponsored media the canonical endpoint refuses.
- Rate limiting: caller's job — library doesn't backoff for you. Best-practice doc explicitly warns: do not call `login()` from scratch each run, use one stable proxy/IP per account, persist device IDs.
- Architecture: `Client` god-object mixing in trait classes from `mixins/` per resource (user, media, story, direct, etc.).

## MCP / Plugin landscape
- **Most existing IG MCP servers are Graph-API based** (business-account only, can't fetch arbitrary public profiles — wrong tool for the user's goal):
  - [jlbadano/ig-mcp](https://github.com/jlbadano/ig-mcp) — Graph API, business accounts. Tools: profile info, media insights, publish media.
  - [leon-meta-mcp](https://lobehub.com/mcp/leon-meta-mcp) — Meta unified (FB/IG/WhatsApp), Graph API.
  - [your-username/news-instagram-mcp](https://lobehub.com/mcp/your-username-news-instagram-mcp) — trend scraping + post-generation, not extraction.
- **Scraping-capable MCP servers**:
  - [`anand-kamble/mcp-instagram`](https://github.com/anand-kamble/mcp-instagram) — view profiles/timelines/stories + engagement (like/follow). Implies cookie-session backend.
  - [`lupikovoleg/instagram-cli`](https://lobehub.com/mcp/lupikovoleg-instagram-cli) — terminal-first IG analytics + downloads via MCP. Search by topic, profile/reel stats, comments, likers.
  - **[`ibrahimhajjaj/ig-dl`](https://github.com/ibrahimhajjaj/ig-dl)** — already-shipping Go single-binary CLI + MCP server. Tools: `ig_download_url`, `ig_download_urls`, `ig_download_user`, `ig_download_saved`, `ig_session_status`, `ig_login`. Prompts: `download_url`, `archive_profile`, `session_health`.
- **Claude plugins / skills**: nothing in Anthropic's first-party skills marketplace targets IG image extraction as of May 2026 (sensitive ToS topic; Anthropic-distributed skills avoid it).
- **Ecosystem read**: sparse on the *extraction* side because of ToS; dense on the *publishing* side via Graph API. There's room for a credible Go-native extraction MCP — `ig-dl` is the only direct competitor and it's a wrapper, not a native client.

## Product Thesis
- **Name**: `igpress` (or `igarc`, `instarc`, `instamirror`) — leaning `igpress` to match the Printing Press house style.
- **Why it should exist** (one line): *Go single-binary Instagram archiver with local SQLite dedup, FTS over captions, and agent-native `--json` — install one binary, not Python + browser_cookie3 + ffmpeg.*
- **Differentiators vs instaloader, gallery-dl, ig-dl**:
  1. **Single static Go binary** — no `pip install`, no Python runtime, no `gallery-dl`/`yt-dlp` subprocess (ig-dl wraps them; `igpress` would speak the IG private API directly via Go HTTP).
  2. **Local SQLite catalog by default** — every download is a row; re-runs are O(diff) without a sidecar `iterator-*.json.xz`; supports SQL queries (`SELECT * FROM posts WHERE caption MATCH 'sunset'`).
  3. **FTS5 over captions + alt-text** — offline search across thousands of archived posts (instaloader requires you to grep `.txt` files manually).
  4. **Agent-native** — `--json` on every command, structured exit codes (0 ok, 64 rate-limited, 65 auth-required, 66 checkpoint, 67 not-found, 68 private, 69 network), MCP server in the same binary.
  5. **`--dry-run`** — list shortcodes that *would* be downloaded with size estimates; instaloader has no equivalent.
  6. **Browser-cookie import** (matching instaloader's `-b`) + CDP capture (matching ig-dl) + manual `sessionid` paste — three flavors, one flag.
  7. **Incremental sync as first-class** — `igpress sync <user>` is the same as `igpress user <user> --fast-update` but uses the SQLite cursor, not the directory.

## Build Priorities
1. **`igpress get <url>`** — single post/reel/IGTV by URL or shortcode. Highest ROI: matches yt-dlp's strongest case, validates the HTTP/session layer, ships value on day one. Includes carousel children.
2. **`igpress user <handle>`** — bulk profile archive with `--fast-update` (SQLite-cursor incremental), `--include posts,reels,stories,highlights,tagged`, `--limit N`. Replicates instaloader's bread-and-butter target.
3. **`igpress login`** — three modes: `--cookies-from-browser <name>` (Chrome/Firefox/Safari/Arc/Brave/Edge), `--cdp` (Chrome DevTools Protocol capture, ig-dl-style), `--sessionid <s>` (manual paste). Persists to SQLite `sessions` table. **Required for everything below.**
4. **`igpress search <query>` / `igpress list`** — FTS5 over the local catalog: captions, hashtags, alt-text, locations, owner. This is the *novel* differentiator nobody else ships.
5. **`igpress mcp`** — stdio MCP server exposing the above as tools (`ig_get_post`, `ig_archive_user`, `ig_search_local`, `ig_session_status`). Match ig-dl's tool surface so users can swap.

Deferred (P2): `hashtag <#tag>`, `saved`, `liked`, `location <id>`, `feed`, story watcher / cron mode.

## References
- Instaloader CLI: <https://instaloader.github.io/cli-options.html>
- Instaloader troubleshooting: <https://instaloader.github.io/troubleshooting.html>
- gallery-dl Instagram extractor: <https://github.com/mikf/gallery-dl/blob/master/gallery_dl/extractor/instagram.py>
- instagrapi: <https://github.com/subzeroid/instagrapi>
- instagrapi best practices: <https://subzeroid.github.io/instagrapi/usage-guide/best-practices.html>
- ig-dl (Go competitor): <https://github.com/ibrahimhajjaj/ig-dl>
- Meta Basic Display deprecation: <https://developers.facebook.com/blog/post/2024/09/04/update-on-instagram-basic-display-api/>
- Scrapfly 2026 IG scraping guide: <https://scrapfly.io/blog/posts/how-to-scrape-instagram>
- Reverse-engineered web client (reference): <https://instagram-private-api.readthedocs.io/>
