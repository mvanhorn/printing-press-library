# YouTube CLI

**Every YouTube Data API v3 endpoint, plus quota-aware planning, plus offline transcripts (no quota), plus an SQLite-backed store so channel and trending analysis cost zero quota after sync.**

Existing tools split the surface — yt-dlp owns downloads, MCP servers own metadata reads, no one lets an operator do all three plus track quota plus query a cached corpus. youtube-pp-cli mirrors all 23 Data API resources, shells out to yt-dlp for transcripts, and ships a local SQLite store with FTS5 across video metadata, transcripts, and comments. Use `quota plan` before running expensive sweeps and `corpus search` to query everything you've already synced without touching the API.

Learn more at [YouTube](https://google.com).

## Install

The recommended path installs both the `youtube-pp-cli` binary and the `pp-youtube` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install youtube
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install youtube --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/youtube-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-youtube --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-youtube --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-youtube skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-youtube. The skill defines how its required CLI can be installed.
```

## Authentication

Read commands need a YouTube Data API key (10K-unit/day quota by default). Set `YOUTUBE_API_KEY` or pass `--api-key`. Write commands (playlist edits, comments, ratings, uploads, channel section CRUD, subscriptions, captions, thumbnails) require OAuth2 — run `youtube-pp-cli auth login` to harvest a token into `~/.config/youtube-pp-cli/oauth.json`. Transcripts use yt-dlp and require neither.

## Quick Start

```bash
# Verify YOUTUBE_API_KEY is set, yt-dlp is on PATH, and the API is reachable
youtube-pp-cli doctor


# Search the API directly; JSON-piped to jq or stdout
youtube-pp-cli youtube search-list --q 'go generics' --part snippet --type video --max-results 25 --json


# Mirror a channel's uploads + stats into the local SQLite store
youtube-pp-cli sync-channel UCBJycsmduvYEL83R_U4JriQ --max-videos 50


# FTS across everything you've synced — zero quota
youtube-pp-cli corpus search 'goroutine leak' --json


# Preview projected quota cost before running real commands
youtube-pp-cli quota plan search.list videos.list --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Quota awareness
- **`quota plan`** — Dry-run any command sequence and see projected unit cost vs. your remaining 10K-unit daily budget before pressing enter.

  _Pick this when you need to know if a planned scrape will fit today's budget. Returns total units, per-endpoint breakdown, and remaining budget._

  ```bash
  youtube-pp-cli quota plan search.list --times 5 --include-ledger --json --agent
  ```
- **`cost ledger`** — Show the rolling per-API-key ledger of every command's quota spend, aggregated by command, endpoint, and day.

  _Use after a quota-exceeded surprise to find the offending command, or daily to budget across multiple agents sharing one key._

  ```bash
  youtube-pp-cli cost ledger --last 24h --json --agent
  ```

### Local-state advantage
- **`corpus search`** — FTS5 search across all synced video metadata, transcripts, and comments in one query — no quota cost.

  _Reach for this when an agent needs to find which videos discuss a topic across a synced corpus without burning search.list quota (100 units per call)._

  ```bash
  youtube-pp-cli corpus search 'inference latency' --since 30d --json --select hits.video_id,hits.channel,hits.snippet --agent
  ```
- **`digest`** — For a tracked channel, list new uploads since a timestamp joined with transcript-FTS hits for a keyword set.

  _Use this for weekly newsletter sweeps, agent research loops, or any 'what did this creator say about X this week' question._

  ```bash
  youtube-pp-cli digest UCBJycsmduvYEL83R_U4JriQ --since 7d --keywords inference,RAG,Sora --json --agent
  ```

### Trend tracking
- **`trending diff`** — Diff today's `mostPopular` snapshot against an earlier date and show entries that entered, exited, or moved.

  _Pick this to detect rising or falling trending entries between two dates — useful for content trend reports and sentiment shifts._

  ```bash
  youtube-pp-cli trending diff --region US --since 2026-04-01 --json --agent
  ```
- **`velocity`** — Compute Δviews, Δlikes, and Δcomments per day per video over a rolling window from your local snapshot history.

  _Use when you need 'which uploads from the last 30 days are accelerating or dying' — a question Studio shows piecemeal but agents can't extract._

  ```bash
  youtube-pp-cli velocity UCBJycsmduvYEL83R_U4JriQ --window 30d --json --agent
  ```
- **`topic crossover`** — Find videos in trending snapshots whose title, description, or transcript mention a keyword, ranked by trending position.

  _Use to spot when a niche topic crosses into mainstream trending lists — a signal for editorial timing._

  ```bash
  youtube-pp-cli topic crossover 'AI safety' --regions US,GB --json --select results.video_id,results.title,results.trending_rank --agent
  ```

### Subscription mirror
- **`subscriptions sweep`** — OAuth-gated: rebuild the subscription feed locally — list subscribed channels' new uploads since a timestamp in chronological order.

  _Pick this for an agent-readable replacement of the in-app subscription feed: filterable, sortable, replayable._

  ```bash
  youtube-pp-cli subscriptions sweep --since 7d --json --agent
  ```

## Usage

Run `youtube-pp-cli --help` for the full command reference and flag list.

## Commands

### youtube

Manage youtube

- **`youtube-pp-cli youtube abuse-reports-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube activities-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube captions-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube captions-download`** - Downloads a caption track.
- **`youtube-pp-cli youtube captions-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube captions-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube captions-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube channel-banners-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube channel-sections-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube channel-sections-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube channel-sections-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube channel-sections-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube channels-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube channels-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube comment-threads-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube comment-threads-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube comments-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube comments-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube comments-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube comments-mark-as-spam`** - Expresses the caller's opinion that one or more comments should be flagged as spam.
- **`youtube-pp-cli youtube comments-set-moderation-status`** - Sets the moderation status of one or more comments.
- **`youtube-pp-cli youtube comments-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube i18n-languages-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube i18n-regions-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube live-broadcasts-bind`** - Bind a broadcast to a stream.
- **`youtube-pp-cli youtube live-broadcasts-delete`** - Delete a given broadcast.
- **`youtube-pp-cli youtube live-broadcasts-insert`** - Inserts a new stream for the authenticated user.
- **`youtube-pp-cli youtube live-broadcasts-insert-cuepoint`** - Insert cuepoints in a broadcast
- **`youtube-pp-cli youtube live-broadcasts-list`** - Retrieve the list of broadcasts associated with the given channel.
- **`youtube-pp-cli youtube live-broadcasts-transition`** - Transition a broadcast to a given status.
- **`youtube-pp-cli youtube live-broadcasts-update`** - Updates an existing broadcast for the authenticated user.
- **`youtube-pp-cli youtube live-chat-bans-delete`** - Deletes a chat ban.
- **`youtube-pp-cli youtube live-chat-bans-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube live-chat-messages-delete`** - Deletes a chat message.
- **`youtube-pp-cli youtube live-chat-messages-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube live-chat-messages-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube live-chat-moderators-delete`** - Deletes a chat moderator.
- **`youtube-pp-cli youtube live-chat-moderators-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube live-chat-moderators-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube live-streams-delete`** - Deletes an existing stream for the authenticated user.
- **`youtube-pp-cli youtube live-streams-insert`** - Inserts a new stream for the authenticated user.
- **`youtube-pp-cli youtube live-streams-list`** - Retrieve the list of streams associated with the given channel. --
- **`youtube-pp-cli youtube live-streams-update`** - Updates an existing stream for the authenticated user.
- **`youtube-pp-cli youtube members-list`** - Retrieves a list of members that match the request criteria for a channel.
- **`youtube-pp-cli youtube memberships-levels-list`** - Retrieves a list of all pricing levels offered by a creator to the fans.
- **`youtube-pp-cli youtube playlist-items-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube playlist-items-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube playlist-items-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube playlist-items-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube playlists-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube playlists-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube playlists-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube playlists-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube search-list`** - Retrieves a list of search resources
- **`youtube-pp-cli youtube subscriptions-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube subscriptions-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube subscriptions-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube super-chat-events-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube tests-insert`** - POST method.
- **`youtube-pp-cli youtube third-party-links-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube third-party-links-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube third-party-links-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube third-party-links-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube thumbnails-set`** - As this is not an insert in a strict sense (it supports uploading/setting of a thumbnail for multiple videos, which doesn't result in creation of a single resource), I use a custom verb here.
- **`youtube-pp-cli youtube update-comment-threads`** - Updates an existing resource.
- **`youtube-pp-cli youtube video-abuse-report-reasons-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube video-categories-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube videos-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube videos-get-rating`** - Retrieves the ratings that the authorized user gave to a list of specified videos.
- **`youtube-pp-cli youtube videos-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube videos-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube videos-rate`** - Adds a like or dislike rating to a video or removes a rating from a video.
- **`youtube-pp-cli youtube videos-report-abuse`** - Report abuse for a video.
- **`youtube-pp-cli youtube videos-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube watermarks-set`** - Allows upload of watermark image and setting it for a channel.
- **`youtube-pp-cli youtube watermarks-unset`** - Allows removal of channel watermark.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
youtube-pp-cli youtube abuse-reports-insert --part example-value

# JSON for scripting and agents
youtube-pp-cli youtube abuse-reports-insert --part example-value --json

# Filter to specific fields
youtube-pp-cli youtube abuse-reports-insert --part example-value --json --select id,name,status

# Dry run — show the request without sending
youtube-pp-cli youtube abuse-reports-insert --part example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
youtube-pp-cli youtube abuse-reports-insert --part example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-youtube -g
```

Then invoke `/pp-youtube <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add youtube youtube-pp-mcp -e YOUTUBE_DATA_OAUTH2C=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/youtube-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `YOUTUBE_DATA_OAUTH2C` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "youtube": {
      "command": "youtube-pp-mcp",
      "env": {
        "YOUTUBE_DATA_OAUTH2C": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
youtube-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/youtube-data-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `YOUTUBE_DATA_OAUTH2C` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `youtube-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $YOUTUBE_DATA_OAUTH2C`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **quotaExceeded error mid-sweep** — Run `youtube-pp-cli cost ledger --last 24h` to find the offending command. Use `--data-source local` on read commands to skip the API entirely after sync.
- **search.list returning no results** — search.list won't surface videos older than ~6 months or unindexed regions. Try `youtube-pp-cli channels videos <id>` instead — it crawls the uploads playlist directly.
- **yt-dlp not found** — Install yt-dlp via `brew install yt-dlp` (macOS) or `pip install yt-dlp`. The CLI prints an `oauth required` style error when it shells out and yt-dlp is missing.
- **OAuth-gated command rejects** — Run `youtube-pp-cli auth login` to refresh the OAuth token. The token is stored at `~/.config/youtube-pp-cli/oauth.json`.
- **transcripts get returns empty** — The video may have no captions. Try `--lang auto` to let yt-dlp pick auto-generated captions, or check `youtube-pp-cli captions list <video>` for available language codes.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**dannySubsense/youtube-mcp-server**](https://github.com/dannySubsense/youtube-mcp-server) — Python
- [**anaisbetts/mcp-youtube**](https://github.com/anaisbetts/mcp-youtube) — TypeScript
- [**nattyraz/youtube-mcp**](https://github.com/nattyraz/youtube-mcp) — TypeScript
- [**mourad-ghafiri/youtube-mcp-server**](https://github.com/mourad-ghafiri/youtube-mcp-server) — Python
- [**yt-dlp/yt-dlp**](https://github.com/yt-dlp/yt-dlp) — Python
- [**happycod3r/YouTube-Data-API-v3-Tools**](https://github.com/happycod3r/YouTube-Data-API-v3-Tools) — PHP
- [**danvega/youtube-cli**](https://github.com/danvega/youtube-cli) — Java
- [**0xPr0xy/youtube-cli**](https://github.com/0xPr0xy/youtube-cli) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
