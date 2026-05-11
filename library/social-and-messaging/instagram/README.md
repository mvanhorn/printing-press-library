# Instagram CLI

**Single-binary Instagram archiver with a local SQLite catalog, full-text search across captions and Instagram-generated alt-text, and an MCP server in the same binary.**

This CLI is a pure-Go alternative to instaloader and gallery-dl for archiving public and authenticated Instagram pages. Every download lands in a local SQLite catalog, so search, watchlists, highlight-tray diff, and remote-vs-local mirror-diff are SQL queries — not shell loops over .txt sidecars. One static binary, no Python, no ffmpeg, no subprocesses.

## Install

The recommended path installs both the `instagram-pp-cli` binary and the `pp-instagram` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install instagram
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install instagram --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/instagram-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-instagram --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-instagram --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-instagram skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-instagram. The skill defines how its required CLI can be installed.
```

## Authentication

Instagram's reachable surface is the reverse-engineered web/private API, gated by session cookies. Authenticate by importing cookies from your logged-in Chrome/Firefox/Safari/Arc/Brave/Edge profile (`auth login --chrome`) or by pasting a sessionid value (`auth login --sessionid <value>`). Username+password login is not shipped — it is the most reliable way to trip a checkpoint or get an account banned. After login, the device fingerprint and cookies persist in the local store so the next run reuses them. Run `doctor` to confirm the session is still good before a long sync.

## Quick Start

```bash
# Import the sessionid + csrftoken + ds_user_id cookies from your logged-in Chrome profile
instagram-pp-cli auth login --chrome


# Confirm the session is healthy before spending the rate budget
instagram-pp-cli doctor


# Smoke-test the auth and HTTP layer on one known-good post
instagram-pp-cli get https://www.instagram.com/p/Cabc123/ --json


# Estimate request count, bytes, and wall-clock before committing to a profile sync
instagram-pp-cli user natgeo --dry-run --rate 30


# Register a watchlist of profiles for incremental sync
instagram-pp-cli watch add natgeo nasa rei


# Walk the whole watchlist under one shared rate budget; only fetches new shortcodes
instagram-pp-cli sync watchlist

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`search`** — Full-text search across every caption, hashtag, mention, location, owner handle, and IG-generated alt-text in your local archive — no API call.

  _When the archive is the corpus, agents can answer cross-profile questions ("every post that mentions person X across these 40 accounts") without re-fetching from Instagram._

  ```bash
  instagram-pp-cli search "sunset golden hour" --owner natgeo --kind post --json --select results.shortcode,results.owner,results.taken_at,results.caption_snippet
  ```
- **`search`** — Captures Instagram's auto-generated accessibility_caption ("may contain: two people, sunglasses, outdoors") into the local catalog and exposes it via --alt search.

  _Lets an agent pick representative images by visual content from the local archive without OCR, vision models, or extra API calls._

  ```bash
  instagram-pp-cli search --alt "sunglasses outdoors" --json --select results.shortcode,results.alt_text,results.local_path
  ```
- **`sync`** — Register profiles and hashtags as a watchlist, then run one sync that walks the whole list under one shared rate budget instead of looping a shell over instaloader runs.

  _One predictable command handles the recurring weekly sync, so an agent can be tasked with "sync the watchlist" without managing per-target state._

  ```bash
  instagram-pp-cli watch add natgeo nasa rei && instagram-pp-cli sync watchlist --json --select summary.profile,summary.added,summary.skipped
  ```
- **`highlights diff`** — Compares the current highlight tray and items for a profile against the prior snapshot in the local store, reporting added, removed, and renamed items.

  _Brand and rights teams need the previous version of a highlight when something gets pulled — only a local snapshot can answer this._

  ```bash
  instagram-pp-cli highlights diff natgeo --json --select changes.tray,changes.kind,changes.shortcode
  ```
- **`whatsnew`** — Lists every shortcode added to the local catalog since the prior sync run, grouped by owner, with caption snippets.

  _Gives agents and researchers a delta after every sync without re-walking IG._

  ```bash
  instagram-pp-cli whatsnew --since last-run --json --select results.owner,results.shortcode,results.caption_snippet,results.taken_at
  ```
- **`diff`** — Cursor-walks shortcodes from Instagram, set-diffs against the local download_history, and reports both new posts and posts that were deleted upstream since the last sync.

  _Deleted-evidence preservation is the only way to answer "which posts disappeared since last week" — Instagram itself doesn't tell you._

  ```bash
  instagram-pp-cli diff natgeo --json --select added.shortcode,deleted.shortcode,deleted.taken_at
  ```
- **`search`** — Opt-in --rerank flag pipes the top FTS candidates' captions, alt-text, and first comments to whichever LLM env is set (ANTHROPIC_API_KEY, OPENAI_API_KEY, or OLLAMA_HOST) and returns the LLM's relevance ranking.

  _Lets an agent or designer search by theme even when captions and alt-text don't use the same words; no model is bundled and no LLM call happens unless --rerank is passed._

  ```bash
  instagram-pp-cli search "warm indoor light" --rerank --rerank-top 50 --json --select results.shortcode,results.score,results.alt_text,results.caption_snippet
  ```

### Reachability mitigation
- **`user`** — Walks the cursor without downloading, sums estimated bytes from each media_info, and reports request count plus wall-clock under the configured --rate.

  _Agents can preview cost before committing the rate budget; humans can decide whether to run now or split across days._

  ```bash
  instagram-pp-cli user natgeo --dry-run --rate 30 --json --select estimate.posts,estimate.bytes,estimate.requests,estimate.wall_clock_seconds
  ```
- **`doctor`** — Joins sessions, rate_limit_events, and download_history to report last-sync per watchlist entry, current 429 backoff state, session validity, and cursor-stall warnings.

  _Agents can decide whether to attempt a sync now or wait, and humans can spot a session that is one bad request from a checkpoint._

  ```bash
  instagram-pp-cli doctor --json --select sessions.username,sessions.status,backoff.until,stalled.handles
  ```

### Service-specific patterns
- **`get`** — Filter carousel children by media kind so a sync keeps only stills (skipping reels and video children) or only videos.

  _Lets agents scope a download to a media type without inspecting each carousel manually._

  ```bash
  instagram-pp-cli get https://www.instagram.com/p/Cabc123/ --json --select media_items.kind,media_items.local_path
  ```

## Usage

Run `instagram-pp-cli --help` for the full command reference and flag list.

## Commands

### accounts

Look up Instagram profiles by username or user ID.

- **`instagram-pp-cli accounts get`** - Get a profile's web info by username (id, full_name, biography, follower_count, profile_pic_url, is_private).
- **`instagram-pp-cli accounts user`** - Get a user's full mobile-API profile by numeric user ID (richer fields than web info).

### feed

Fetch authenticated user feeds: saved, liked, timeline, and per-user media.

- **`instagram-pp-cli feed liked`** - Fetch the authenticated user's liked posts.
- **`instagram-pp-cli feed reels`** - Fetch active stories (reels media) for a list of user IDs. Stories expire 24 hours after posting.
- **`instagram-pp-cli feed saved`** - Fetch the authenticated user's saved (bookmarked) posts.
- **`instagram-pp-cli feed timeline`** - Fetch the authenticated user's home feed.
- **`instagram-pp-cli feed user`** - Fetch a profile's media feed by numeric user ID. Pagination cursor is max_id.

### hashtag

Look up hashtag pages and download tagged media.

- **`instagram-pp-cli hashtag info`** - Get a hashtag's web info (post count, top posts, recent posts cursor).

### highlights

Fetch profile highlight trays and per-highlight items.

- **`instagram-pp-cli highlights reel`** - Fetch a single highlight reel's items by highlight reel ID (prefix with 'highlight:', e.g., highlight:17855928765404872).
- **`instagram-pp-cli highlights tray`** - Fetch the highlight tray for a profile (cover images, titles, ordered list of highlight reels).

### location

Fetch geotagged location pages and the posts at them.

- **`instagram-pp-cli location sections`** - Fetch posts at a location ID (top, recent, ranked sections).

### media

Fetch single posts, reels, and IGTV by media ID.

- **`instagram-pp-cli media info`** - Get full media JSON for a single post, reel, or IGTV by media ID, including carousel children, captions, alt-text, comments, and CDN URLs.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
instagram-pp-cli accounts get mock-value

# JSON for scripting and agents
instagram-pp-cli accounts get mock-value --json

# Filter to specific fields
instagram-pp-cli accounts get mock-value --json --select id,name,status

# Dry run — show the request without sending
instagram-pp-cli accounts get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
instagram-pp-cli accounts get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-instagram -g
```

Then invoke `/pp-instagram <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
instagram-pp-cli auth login --chrome

claude mcp add instagram instagram-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
instagram-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/instagram-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "instagram": {
      "command": "instagram-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
instagram-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/instagram-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `instagram-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run `instagram-pp-cli --help` to see available commands or `instagram-pp-cli which "<capability>"` to find a specific command

### API-specific

- **401 Unauthorized 'Please wait a few minutes' on every request** — Run `instagram-pp-cli doctor`; if backoff.until is in the past, your session was invalidated — re-import cookies with `auth login --chrome`. If backoff is active, wait it out — re-logging in from a new IP escalates the ban risk.
- **checkpoint_required during login** — Stop and resolve the challenge in your normal browser session, then re-run `auth login --chrome` to import the refreshed cookies. Do NOT retry login from the CLI — it makes the checkpoint sticky.
- **Cursor stalls at ~12 posts when paginating a profile** — Run with `--rate 20` (or lower) to slow the pagination, and confirm cookies are imported (`doctor`); anonymous access is throttled aggressively in 2025+.
- **`get` fetches succeed but `user` returns no posts** — Profile is private and your session does not follow it, OR the cursor cache is stale. Run `instagram-pp-cli user <handle> --no-resume` to walk from scratch.
- **Watchlist sync skips a profile every run** — Inspect `instagram-pp-cli doctor --full --json`; a `cursor_stalled` flag means the last cursor token is rejected — clear it with `--no-resume`.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**yt-dlp**](https://github.com/yt-dlp/yt-dlp) — Python (90000 stars)
- [**gallery-dl**](https://github.com/mikf/gallery-dl) — Python (13500 stars)
- [**instaloader**](https://github.com/instaloader/instaloader) — Python (13000 stars)
- [**instagrapi**](https://github.com/subzeroid/instagrapi) — Python (4700 stars)
- [**goinsta**](https://github.com/ahmdrz/goinsta) — Go (2500 stars)
- [**ig-dl**](https://github.com/ibrahimhajjaj/ig-dl) — Go
- [**instagram-go-scraper**](https://github.com/Vorkytaka/instagram-go-scraper) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
