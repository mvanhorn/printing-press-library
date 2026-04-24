# Scrape Creators CLI

Scrape public social media data from the terminal — profiles, posts, videos, comments, ads, and transcripts across TikTok, Instagram, YouTube, Twitter/X, LinkedIn, Facebook, Reddit, Threads, Bluesky, Pinterest, Snapchat, Twitch, Kick, Truth Social, and 15+ link-in-bio / creator link services.

Powered by the [Scrape Creators](https://scrapecreators.com) API. Read-only — this CLI fetches data, it does not post.
Each live API call burns credits. Use `--dry-run` to preview requests, `account budget` to watch spend, and the local SQLite sync for repeated analysis.

## Table of Contents

- [Install](#install)
- [Quick Start](#quick-start)
- [Key Features](#key-features)
- [What This Can't Do](#what-this-cant-do)
- [Interactive Wizard](#interactive-wizard)
- [Commands](#commands)
- [Output Formats](#output-formats)
- [Agent Usage](#agent-usage)
- [MCP Server](#mcp-server)
- [Cookbook](#cookbook)
- [Health Check](#health-check)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)

## Install

### Go

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-cli@latest
```

The MCP server installs from the same repo:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-mcp@latest
```

### Binary

Download from [Releases](https://github.com/mvanhorn/printing-press-library/releases).

## Quick Start

```bash
# 1. Get an API key at https://app.scrapecreators.com and export it
export SCRAPE_CREATORS_API_KEY="<your-key>"

# 2. Verify your setup
scrape-creators-pp-cli doctor

# 3. Check your credit balance and burn rate
scrape-creators-pp-cli account budget

# 4. Try a real scrape
scrape-creators-pp-cli tiktok profile --handle @charlidamelio
```

The CLI normalizes handles (strips leading `@`) and hashtags (strips `#`) automatically, so `@charlidamelio` and `charlidamelio` both work.

## Key Features

Commands that go beyond what the raw API returns, plus a local SQLite layer for repeat analysis:

- **`account budget`** — Show credit balance and project days remaining at your current burn rate.
- **`search trends`** — Search a hashtag and snapshot result count + top videos for growth tracking.
- **`tiktok spikes`** — Find videos that outperformed a creator's average engagement rate.
- **`tiktok transcripts`** — Fetch and search across all of a creator's video transcripts.
- **`tiktok compare`** — Compare multiple TikTok creators on followers, engagement, and posting cadence.
- **`tiktok cadence`** — Show a creator's posting frequency by day of week and hour of day.
- **`tiktok track`** — Record daily follower snapshots and chart a creator's growth trajectory.
- **`tiktok analyze`** — Rank a creator's videos by engagement rate (not raw likes).
- **`archive`** — Sync the CLI's current built-in archiveable set locally for offline search and analysis (today: `account` request history).

## What This Can't Do

- Post, comment, DM, like, follow, or otherwise write to social platforms. This CLI is read-only.
- Import or upsert records back into Scrape Creators. The upstream surface is read-only.
- Archive every single endpoint automatically. `archive` and bare `sync` cover only the current built-in archiveable set (today: `account` request history); endpoints that require a handle, URL, query, or other required args stay explicit commands.
- Guarantee a credential verdict from `doctor`. The upstream validation surface is inconsistent, so `doctor` reports missing or rejected auth as failures and otherwise labels credentials `inconclusive` unless the API clearly rejects them.

## Interactive Wizard

Running `scrape-creators-pp-cli` with no arguments on a TTY walks you through platform → action → required params, then executes the resolved command. Bypass the wizard with `--no-input`, `--agent`, `--yes`, or by piping stdin / running on a non-TTY.

## Commands

The command surface is organized as `<platform> <action>`. Running any platform by itself prints help. A hidden alias layer keeps older OpenAPI-style names (`list-post-2`, `list-user-5`, etc.) working for existing scripts.

### TikTok

```
scrape-creators-pp-cli tiktok <action>
```

| Action | What it does |
|--------|--------------|
| `profile` | Public profile: identity, bio, followers, follower/following/like/video counts |
| `profile-videos` | Recent videos for a profile |
| `user-audience` | Audience demographics |
| `user-followers` | Follower list (paginated) |
| `user-following` | Accounts the user follows (paginated) |
| `user-live` | Live status |
| `user-showcase` | User showcase products |
| `video` | Full video metadata |
| `video-comments` | Comments on a video |
| `video-comment-replies` | Replies to a specific comment |
| `video-transcript` | Transcript / captions (video must be < 2 min) |
| `search-hashtag` / `search-keyword` / `search-top` / `search-users` | Search surfaces |
| `songs-popular` / `song` / `song-videos` | Song discovery + songs-to-videos |
| `creators-popular` / `videos-popular` / `hashtags-popular` | Popular feeds |
| `trending-feed` | For-You-style trending feed |
| `shop-products` / `shop-search` / `product` | Shop browsing + product details |
| `analyze` / `compare` / `cadence` / `spikes` / `track` / `transcripts` | Local-analytics commands (see Key Features) |

### Instagram

```
scrape-creators-pp-cli instagram <action>
```

| Action | What it does |
|--------|--------------|
| `profile` | Public profile details, bio links, follower/following, recent posts, related profiles |
| `basic-profile` | Lightweight profile metadata |
| `post` | Single post or reel info |
| `post-comments` | Comments on a post or reel |
| `media-transcript` | AI-powered transcription for a reel (< 2 min) |
| `reels-search` | Search reels |
| `song-reels` | Reels using a specific song (deprecated upstream) |
| `user-posts` | Paginated feed of a user's posts |
| `user-embed` | Embed HTML for a user |
| `user-highlights` / `user-highlight-detail` | Story highlights |

### YouTube

```
scrape-creators-pp-cli youtube <action>
```

| Action | What it does |
|--------|--------------|
| `channel` | Channel details |
| `channel-videos` / `channel-shorts` | Videos or shorts from a channel |
| `community-post` | Community post details |
| `playlist` | All videos in a playlist |
| `video` | Video or short metadata |
| `video-comment-replies` | Comment replies |
| `video-transcript` | Timestamped transcript + plain text (video must be < 2 min) |
| `search` / `search-hashtag` | Search surfaces |
| `shorts-trending` | Trending shorts |

### Twitter / X

```
scrape-creators-pp-cli twitter <action>
```

| Action | What it does |
|--------|--------------|
| `profile` | Profile metadata and statistics |
| `user-tweets` | Recent tweets from a user |
| `tweet` | Single tweet details (includes AI transcript for video tweets < 2 min) |
| `community` | Community details |
| `community-tweets` | Tweets from a community |

### LinkedIn

```
scrape-creators-pp-cli linkedin <action>
```

| Action | What it does |
|--------|--------------|
| `profile` | Person's profile |
| `company` | Company page |
| `company-posts` | Posts from a company page |
| `post` | Single post / article with reactions, comments, related articles |
| `ad` | Ad details (Search Ads via `scrape-creators-pp-cli linkedin` with a keyword) |

### Facebook

```
scrape-creators-pp-cli facebook <action>
```

| Action | What it does |
|--------|--------------|
| `profile` | Public page details (category, contact, hours, ad library status) |
| `profile-posts` / `profile-photos` / `profile-reels` | Page content |
| `post` | Single public post or reel (optionally with comments + transcript) |
| `post-comments` | Comments on a post (feedback_id path is faster than url) |
| `post-transcript` | Video post transcript (< 2 min) |
| `group-posts` | Posts from a public Facebook group |
| `adlibrary-ad` / `adlibrary-company-ads` / `adlibrary-search-companies` | Meta Ad Library (searching via keyword caps around 1,500 results) |

### Reddit

```
scrape-creators-pp-cli reddit <action>
```

| Action | What it does |
|--------|--------------|
| `subreddit-details` | Subreddit metadata |
| `subreddit-search` | Search posts within a subreddit |
| `search` | Full-site post search with sort, timeframe, and pagination |
| `post-comments` | Comments on a post |
| `ad` / `ads-search` | Reddit ad lookup / search |

### Threads

```
scrape-creators-pp-cli threads <action>
```

| Action | What it does |
|--------|--------------|
| `profile` | Public profile (username, bio, followers, bio links) |
| `post` | Single post with comments + related posts |
| `search` / `search-users` | Keyword and user search |

### Pinterest

```
scrape-creators-pp-cli pinterest <action>
```

| Action | What it does |
|--------|--------------|
| `pin` | Single pin (multiple resolutions, annotations) |
| `search` | Pin search |
| `user-boards` | A user's boards |

### Bluesky

```
scrape-creators-pp-cli bluesky <action>
```

| Action | What it does |
|--------|--------------|
| `post` | Single post with replies |
| `user-posts` | Paginated post feed for a user (use `did` not handle for speed) |

### Truth Social

```
scrape-creators-pp-cli truthsocial <action>
```

| Action | What it does |
|--------|--------------|
| `profile` | Profile (prominent public figures only) |
| `user-posts` | Recent posts |

Running `scrape-creators-pp-cli truthsocial` with a URL fetches a single post.

### Twitch / Kick

```
scrape-creators-pp-cli twitch profile --handle <name>
scrape-creators-pp-cli twitch --url <clip-url>   # clip details
scrape-creators-pp-cli kick   --url <clip-url>   # clip details
```

### Google

```
scrape-creators-pp-cli google <action>
```

| Action | What it does |
|--------|--------------|
| `search` | Organic Google search results |
| `ad` | Ad details |
| `company-ads` | Advertiser company ads (the `google` shortcut is "Advertiser Search") |

### Single-endpoint platforms

These platforms expose one endpoint each — run the top-level command directly.

| Platform | Command | Returns |
|----------|---------|---------|
| Snapchat | `snapchat --handle <name>` | User profile + stories |
| Amazon Shop | `amazon --url <url>` | Amazon shop page |
| Linkbio | `linkbio --url <url>` | Linkbio (lnk.bio) page |
| Linktree | `linktree --url <url>` | Linktree page |
| Linkme | `linkme --url <url>` | Linkme profile + social links |
| Komi | `komi --url <url>` | Komi page |
| Pillar | `pillar --url <url>` | Pillar page |
| Detect age/gender | `detect-age-gender --url <image-url>` | Age and gender estimate |

### Account + infrastructure

| Command | What it does |
|---------|--------------|
| `account` (alias of `account list`) | Credit balance |
| `account api-usage` | Request history |
| `account daily-usage` | Daily usage |
| `account most-used-routes` | Most-used endpoints |
| `account budget` | Credit balance + projected days remaining (see Key Features) |
| `auth set-token` / `auth status` / `auth logout` | API-key management |
| `completion <shell>` | Generate shell completion (`bash`, `zsh`, `fish`, `powershell`) |
| `doctor` | Environment / auth / connectivity health check |
| `version` | Print version |

### Data layer — sync, search, export, analytics

The CLI ships a local SQLite layer. Bare `sync` and `archive` target the built-in archiveable set; explicit `sync --resources ...` lets you hit a canonical resource route directly when the upstream endpoint supports a no-input fetch.

| Command | What it does |
|---------|--------------|
| `sync` | Pull archiveable API data into local SQLite with resumable pagination; omit `--resources` to use the built-in archiveable set |
| `tail <resource>` | Stream live changes by polling one resource (NDJSON to stdout) |
| `search <query>` | FTS5 full-text search over synced data (falls back to API when available) |
| `search trends <hashtag>` | Snapshot hashtag result count + top videos for trend tracking |
| `analytics` | Count / group-by / top-N over synced data |
| `export` | Export a supported canonical API resource to JSONL or JSON (live API read, not a local-store export) |
| `api` | Browse every raw API endpoint by interface name (power-user escape hatch) |
| `archive` | One-shot sync of the current built-in archiveable set (currently `account`) |
| `archive status` | Local archive sync state |

## Output Formats

```bash
# Human-readable table (default on a TTY) / JSON when piped
scrape-creators-pp-cli account budget

# JSON always
scrape-creators-pp-cli account budget --json

# Keep only specific fields (dotted paths traverse arrays)
scrape-creators-pp-cli tiktok profile --handle charlidamelio --json --select user.nickname,stats.followerCount

# CSV / tab-separated / one-value-per-line
scrape-creators-pp-cli tiktok videos-popular --csv
scrape-creators-pp-cli tiktok videos-popular --plain
scrape-creators-pp-cli tiktok videos-popular --quiet

# Dry run — print the HTTP request without sending
scrape-creators-pp-cli tiktok profile --handle charlidamelio --dry-run

# Agent preset — JSON + compact + no prompts + no color + yes
scrape-creators-pp-cli tiktok profile --handle charlidamelio --agent
```

Successful data-layer commands use a `{"meta": {...}, "results": <data>}` envelope. Parse `.results` for payload and `.meta.source` for `live` vs `local`. Errors are plain stderr text and do not use the envelope. The `N results (live)` footer prints to stderr only when stdout is a TTY.

## Agent Usage

Designed for AI-agent consumption:

- **Non-interactive** — `--no-input` disables every prompt; `--agent` implies it.
- **Pipeable** — JSON on stdout, errors on stderr, NDJSON progress events to stderr for paginated ops.
- **Filterable** — `--select field1,field2` (dotted paths supported, arrays traverse element-wise).
- **Previewable** — `--dry-run` shows the HTTP request without sending.
- **Cacheable** — GET responses cached 5 min, bypass with `--no-cache`.
- **Rate-limitable** — `--rate-limit <rps>` caps requests per second.
- **Data-source switch** — `--data-source live|local|auto` chooses between API, local SQLite, or automatic fallback.

Example NDJSON progress stream:

```bash
scrape-creators-pp-cli sync --resources account --agent 2>progress.ndjson
```

Exit codes: `0` success · `2` usage error · `3` not found · `4` auth error · `5` API error · `7` rate limited · `10` config error.

## MCP Server

This CLI ships a companion MCP server (`scrape-creators-pp-mcp`) for Claude Code, Claude Desktop, Cursor, and Codex.

### One-liner install

```bash
# Claude Code (writes ~/.claude.json)
scrape-creators-pp-cli agent add claude-code

# Claude Desktop (writes ~/Library/Application Support/Claude/claude_desktop_config.json)
scrape-creators-pp-cli agent add claude-desktop

# Cursor (writes ~/.cursor/mcp.json)
scrape-creators-pp-cli agent add cursor

# Codex (writes ~/.codex/config.toml)
scrape-creators-pp-cli agent add codex

# Add --hosted to wire the hosted endpoint at https://api.scrapecreators.com/mcp instead of the local binary
# Add --force to overwrite an existing scrape-creators entry (a diff is printed by default)
```

All writes are `chmod 0600`. Existing entries are refused without `--force`. If no API key is present yet, `agent add` still writes the MCP entry and leaves credentials for you to add later.

### Manual config (Claude Desktop)

```json
{
  "mcpServers": {
    "scrape-creators": {
      "command": "scrape-creators-pp-mcp",
      "env": {
        "SCRAPE_CREATORS_API_KEY": "<your-key>"
      }
    }
  }
}
```

### Claude Code via `claude mcp add`

```bash
claude mcp add scrape-creators scrape-creators-pp-mcp -e SCRAPE_CREATORS_API_KEY=<your-key>
```

## Cookbook

```bash
# Find a creator's best-performing videos (2× their own engagement-rate average)
scrape-creators-pp-cli tiktok spikes --handle charlidamelio --threshold 2 --json

# Record a daily growth snapshot (run on a schedule) and show history
scrape-creators-pp-cli tiktok track --handle charlidamelio
scrape-creators-pp-cli tiktok track --handle charlidamelio --history

# Archive the built-in archiveable set, then inspect local status
scrape-creators-pp-cli archive
scrape-creators-pp-cli archive status

# Budget watch — alert when credits dip below 1000
scrape-creators-pp-cli account budget --agent

# Compare three creators side-by-side (repeat --handle once per creator)
scrape-creators-pp-cli tiktok compare \
  --handle charlidamelio --handle khaby.lame --handle addisonre --json

# Hashtag trend snapshot (re-run over time to detect growth)
scrape-creators-pp-cli search trends --hashtag fyp --json

# Search across all of a creator's transcripts
scrape-creators-pp-cli tiktok transcripts --handle charlidamelio --search "morning routine"

# Sync the built-in archiveable set into local SQLite
scrape-creators-pp-cli sync
```

## Health Check

```bash
scrape-creators-pp-cli doctor
```

Reports config path, auth status, base URL, CLI version, and a live connectivity probe.

## Configuration

Config file: `~/.config/scrape-creators-pp-cli/config.toml`

Required environment variable:

- `SCRAPE_CREATORS_API_KEY` — your API key from <https://app.scrapecreators.com>

Optional:

- `SCRAPE_CREATORS_BASE_URL` — override API base (defaults to `https://api.scrapecreators.com`)

## Troubleshooting

**Auth error (exit 4)**
- `scrape-creators-pp-cli doctor` to see what's set
- `echo $SCRAPE_CREATORS_API_KEY` to verify the variable is exported

**`doctor` says credentials are inconclusive**
- The CLI could not prove your credentials from the upstream validation endpoint, but it also did not see an explicit auth rejection
- Run a low-impact real command like `scrape-creators-pp-cli account budget --dry-run` to confirm request wiring without spending credits
- If a real request fails with `401` or `403`, rotate the key and retry

**Not found (exit 3)**
- Check the handle / URL is correct and the account is public
- Some platforms (Truth Social, older Snapchat profiles) only expose prominent public accounts

**`tail` fails immediately**
- Pass a resource name: `scrape-creators-pp-cli tail tiktok`
- Use `--follow=false` for a single poll instead of a continuous stream

**Rate limited (exit 7)**
- The CLI auto-retries with exponential backoff
- Lower concurrency with `--rate-limit 2` (2 requests per second)

**Transcript returns nothing**
- Video must be under ~2 minutes for AI transcription endpoints (Facebook, Instagram, TikTok, Twitter, Threads)

**MCP install before auth**
- `scrape-creators-pp-cli agent add <target>` works even before you have a key
- Re-run it later with `SCRAPE_CREATORS_API_KEY` exported, or edit the generated agent config to add the key manually

**Wizard did not launch**
- The wizard only runs on a TTY with no command path
- It is skipped automatically for pipes, CI, `--no-input`, `--agent`, and explicit subcommands

---

## Sources & Inspiration

- [**@scrapecreators/cli**](https://www.npmjs.com/package/@scrapecreators/cli) — official JS CLI (command naming mirrors v1)
- [**n8n-nodes-scrape-creators**](https://github.com/adrianhorning08/n8n-nodes-scrape-creators) — n8n integration
- [**scrape-creators-examples**](https://github.com/adrianhorning08/scrape-creators-examples) — JS examples

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press).
