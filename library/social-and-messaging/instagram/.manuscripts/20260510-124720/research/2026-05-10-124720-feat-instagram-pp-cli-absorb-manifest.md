# Instagram CLI Absorb Manifest

CLI name: `igpress` (binary: `instagram-pp-cli`)

Slogan: *Single-binary Instagram archiver — local SQLite catalog, FTS over captions and IG-generated alt-text, agent-native `--json`, MCP server in the same binary.*

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Download single post/reel/IGTV by URL or shortcode | yt-dlp, instaloader URL-target, ig-dl | `igpress get <url-or-shortcode>` (carousel children, `--no-videos`, `--no-pictures`, `--geotags`, `--comments`, `--captions`) | `--json`, structured exit codes, SQLite dedup |
| 2 | Bulk profile archive (all posts) | instaloader `profile`, gallery-dl `InstagramPostsExtractor` | `igpress user <handle>` (`--include posts,reels,stories,highlights,tagged --limit N`) | SQLite-cursor incremental, `--fast-update` by default, `--dry-run` with size estimate |
| 3 | Per-kind profile media | gallery-dl per-extractor, instaloader `--reels --igtv --tagged` | `igpress user <handle> --kind posts,reels,igtv,tagged` | Composable, queryable after sync |
| 4 | Profile picture only | instaloader `--profile-pic`, gallery-dl `InstagramAvatarExtractor` | `igpress user <handle> --profile-pic-only` | Cached in SQLite, dedup by sha256 |
| 5 | Stories download | instaloader `--stories`, gallery-dl `InstagramStoriesExtractor`, ig-dl | `igpress stories <handle>` | Tracks expiry; safe before 24h cliff |
| 6 | Highlights download | instaloader `--highlights`, gallery-dl `InstagramHighlightsExtractor` | `igpress highlights <handle> [--id <highlight_id>]` | Per-highlight selection, joined to `highlight_items` |
| 7 | Tagged-in posts | instaloader `--tagged`, gallery-dl `InstagramTaggedExtractor` | `igpress user <handle> --kind tagged` | Same SQLite catalog |
| 8 | Hashtag download | instaloader `#tag`, gallery-dl `InstagramTagExtractor` | `igpress hashtag <tag> [--top|--recent] [--limit N]` | Auth-required; `--since-shortcode` cursor |
| 9 | Saved posts | instaloader `:saved`, gallery-dl `InstagramSavedExtractor`, ig-dl | `igpress saved` | Auth-required; agent-native `--json` |
| 10 | Liked posts | instaloader (custom filter) | `igpress liked` | Auth-required |
| 11 | Home feed | instaloader `:feed` | `igpress feed [--limit N]` | Auth-required |
| 12 | Following enumeration | gallery-dl `InstagramFollowingExtractor` | `igpress user <handle> --kind following` | SQLite stores edges for later graph queries |
| 13 | Location posts (numeric ID) | instaloader `%location_id` | `igpress location <id> [--top|--recent]` | Joined with `locations` table for offline reverse-lookup |
| 14 | Filter posts by date / count | instaloader `--post-filter`, `--latest-stamps`, `--count` | `igpress user <handle> --since <date> --until <date> --count N` | SQL-style filter expressions also supported |
| 15 | Skip videos / images / thumbnails | instaloader `--no-videos`, `--no-pictures`, `--no-video-thumbnails` | `igpress get/user --no-videos --no-pictures --no-thumbnails` | Per-command default, per-run override |
| 16 | Caption sidecar (.txt) | instaloader `--post-metadata-txt` | `igpress get/user --captions [--captions-format txt\|json]` | Caption is also stored in SQLite for FTS |
| 17 | Geotag sidecar | instaloader `--geotags` | `igpress get/user --geotags` | Reverse-lookup via `locations` table |
| 18 | Comments capture | instaloader `--comments` | `igpress get/user --comments [--comments-limit N]` | Stored in `comments` table |
| 19 | Slide selection (carousel slice) | instaloader `--slide 1-3` | `igpress get <url> --slide 1-3` | Validates against `media_items.carousel_index` |
| 20 | Filename / dirname patterns | instaloader `--filename-pattern`, `--dirname-pattern`, `--title-pattern`, `--sanitize-paths` | `igpress get/user --filename {shortcode}_{idx}.{ext} --dirname {profile}/{year}` | Go template strings |
| 21 | Resume / fast-update | instaloader `--fast-update`, `--resume-prefix`, `--no-resume`, `--commit-mode` | `igpress sync <handle>` (= fast-update over SQLite cursor); `--no-resume` to override | SQLite cursor obviates `iterator-*.json.xz` sidecars |
| 22 | Args from file | instaloader `+args.txt` | `igpress --args-file args.txt` | Same shape |
| 23 | Login by username/password | instaloader `--login -p`, instagrapi | NOT shipped (`igpress login --sessionid` instead) | Avoids the most ban-prone flow |
| 24 | Browser cookie import | instaloader `--load-cookies`, gallery-dl `--cookies-from-browser`, yt-dlp same, ig-dl CDP | `igpress login --chrome` (also `--firefox`, `--safari`, `--brave`, `--arc`, `--edge`) | Pure-Go cookie reader; no Python dep |
| 25 | Manual sessionid paste | gallery-dl `--cookies <file>`, instagrapi `login_by_sessionid` | `igpress login --sessionid <value>` | Headless/server fallback |
| 26 | Custom user-agent | instaloader `--user-agent`, `--no-iphone` | `igpress --user-agent <ua>` (root flag) | Defaults to a pinned recent Chrome UA |
| 27 | Configurable rate limiting / retries | instaloader `--max-connection-attempts`, `--request-timeout`, gallery-dl `sleep-request` | `igpress --rate <req-per-min> --max-attempts N --timeout S` | AdaptiveLimiter with 429-aware backoff |
| 28 | Dedup archive (skip already-fetched) | gallery-dl `--download-archive`, instaloader `--fast-update` | Built-in via `download_history` + `media_items.sha256` | Default behavior, not opt-in |
| 29 | Quiet / verbose modes | instaloader `--quiet`, root `-v` patterns | `igpress --quiet`, `--verbose` | Generator-provided |
| 30 | Single static binary | ig-dl, all Go alternatives | Pure Go, static, no Python or ffmpeg | No subprocesses to gallery-dl/yt-dlp |
| 31 | MCP server in same binary | ig-dl `ig-dl mcp` | `igpress mcp` (cobratree mirror) | Auto-mirrors all Cobra commands |
| 32 | MCP tools surface | ig-dl: `ig_download_url`, `ig_download_user`, `ig_download_saved`, `ig_session_status` | All Cobra commands → MCP tools automatically; read-only annotated where appropriate | Code-orchestration mode (search + execute pair) for >50 tools |
| 33 | Profile metadata-only fetch | gallery-dl `InstagramInfoExtractor` | `igpress user <handle> --info-only` | No downloads; just sync the row |

## Transcendence (only possible with our approach)

Survivors are scored Domain Fit (0-3) + User Pain (0-3) + Build Feasibility (0-2) + Research Backing (0-2).

| # | Feature | Command | Score | How It Works | Persona | Evidence |
|---|---------|---------|-------|--------------|---------|----------|
| 1 | Local-corpus FTS search | `igpress search "<q>" [--owner h] [--since d] [--kind post\|reel\|story] [--alt]` | 10/10 | FTS5 virtual table `posts_fts(caption, hashtags, mentions, location_name, owner_username, alt_text)` joined to `posts`/`media_items` — pure SQL, no API call at query time. | Mara, Kai | Brief Data Layer §FTS5 explicitly lists this; instaloader has no equivalent (sidecar `.txt` files only). |
| 2 | Alt-text capture + search | (column: `media_items.alt_text`; query path: `igpress search --alt "..."`) | 9/10 | `media_info.accessibility_caption` is captured into `media_items.alt_text` on every download and indexed in `posts_fts`. | Sasha, Mara | Brief Data Layer flags `accessibility_caption` as auto-generated and useful for offline visual search; no extractor surveyed exposes it. |
| 3 | Cross-profile watchlist + sync | `igpress watch add/rm/list`, `igpress sync` | 9/10 | `watchlist(entry_kind, entry_value, added_at, last_synced_at)` table; `sync` iterates entries under one shared rate budget, reusing `user`/`hashtag` walkers. | Mara, Kai, Dev | instaloader requires shell-looping; no surveyed tool ships first-class watchlists. Brief Workflows §1+§3+§7. |
| 4 | Highlight-tray diff | `igpress highlights diff <h>` | 8/10 | Snapshot `highlights_tray` + `highlight_items` per sync; diff reports added/removed/renamed items vs prior snapshot. | Dev | Brief Top Workflows §5 + Data Layer `highlights`; gallery-dl/instaloader treat each pull as fresh — no diff. |
| 5 | What's-new since last sync | `igpress whatsnew [--since <run-id>]` | 8/10 | One SQL over `download_history` + `posts` (`downloaded_at > :last_run_at`), grouped by owner. | Mara, Dev, Kai | Brief Workflow §6+§7 (incremental, resumable); instaloader's `--fast-update` reports nothing, just stops. |
| 6 | Dry-run with size + rate-budget estimate | `igpress get/user --dry-run` | 8/10 | Cursor-walk shortcodes; sum estimated bytes from `media_info.original_dimensions`; divide request count by configured `--rate` for wall-clock estimate. | Mara, Dev | Brief Product Thesis §5 names `--dry-run` as a differentiator vs instaloader; #2511 / #2307 show users hitting walls they wish they'd predicted. |
| 7 | Mirror-diff (remote vs local) | `igpress diff <h>` | 7/10 | Cursor-walk shortcodes from IG; set-diff against `download_history` rows; report `added`/`deleted` lists. | Kai, Mara | Brief Product Thesis §2 (re-runs are O(diff)); deleted-evidence capture is core for DMCA persona. |
| 8 | Carousel slide-kind filter | `igpress get/user --slide-kind image\|video` | 6/10 | At child-write time, skip rows whose `media_items.kind != requested`. | Sasha | Brief Workflow §2 (carousel children); existing `--slide 1-3` filters by index not kind — gap in instaloader/gallery-dl. |
| 9 | Sync-health doctor | `igpress doctor [--handle h]` | 6/10 | Joins `sessions` + `rate_limit_events` + `download_history`; reports last-sync per watchlist entry, 429 backoff state, session validity, cursor-stall warnings. | Dev | Brief Reachability Risk + Codebase Intel §RateController; instagrapi explicitly doesn't backoff for you. |
| 10 | LLM-rerank theme search (opt-in, BYO key) | `igpress search "<q>" --rerank [--rerank-top N]` | 6/10 | FTS5 returns top-N candidates → CLI sends their captions + alt-text + first 5 comments to whichever LLM env is set (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, or `OLLAMA_HOST`/`OLLAMA_MODEL`) → LLM returns a relevance order → CLI emits ranked results. No model bundled. Flag rejects with a helpful error when no env is set. | Sasha, Mara | User briefing — explicit request for theme/image-type search; rubric-compliant pipe-friendly LLM hook (no bundled model, no required dependency). |

## Stub disclosures

None. Every row above will ship as a real implementation against the local SQLite and (where applicable) the reverse-engineered web/private API.

## Infra (cross-cutting, not user-facing features)

- **AdaptiveLimiter persisted to `rate_limit_events`** — sliding-window 429-aware backoff shared across `get`, `user`, `hashtag`, `sync`. Persists across process exits.
- **Per-account device fingerprint** — `sessions.device_id` + pinned UA, regenerated only on explicit `igpress login --rotate-device`. Borrows instagrapi best practice.
- **Browser-cookie import** — pure-Go reader for Chrome/Firefox/Safari/Brave/Arc/Edge sqlite-and-keychain stores.

## Out-of-scope deliberately

- Username+password login (banned-account magnet — user is steered to cookie import or sessionid paste).
- Posting / following / unfollowing / liking / DMs (this is an extraction CLI; mutating endpoints are out of scope).
- Live broadcasts (transient, ToS-fragile).
- Anti-detection device-fingerprint randomization beyond the pinned per-account UA (any further device-spoofing crosses from "polite client" to "evasion tooling").
