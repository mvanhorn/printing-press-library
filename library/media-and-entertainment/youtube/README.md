# YouTube CLI

**Search YouTube in bulk, grab transcripts, get embed snippets — for the photo-keywords-to-blog-post workflow.**

A trimmed read-only YouTube Data API v3 CLI built for local research workflows. Bulk search from stdin, transcript fetching via timedtext (no OAuth needed), embed snippet generation, and a heuristic-based `videos related` that still works despite the 2023 deprecation.

Learn more at [YouTube Data API v3](https://developers.google.com/youtube/v3).

**Sample use case:** [justinwfu/pictovideo](https://github.com/justinwfu/pictovideo) — photo-keywords → candidate-video → embedded blog draft pipeline using this CLI.

Printed by [@justinwfu](https://github.com/justinwfu) (Justin).

## Install

The recommended path installs both the `youtube-pp-cli` binary and the `pp-youtube` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install youtube
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install youtube --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install youtube --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install youtube --agent claude-code
npx -y @mvanhorn/printing-press-library install youtube --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/cmd/youtube-pp-cli@latest
```

This installs the CLI only — no skill.

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

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/youtube-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `YOUTUBE_API_KEY` when Claude Desktop prompts you.

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
        "YOUTUBE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

API-key only — set `YOUTUBE_API_KEY` and you're done. Read-only public-data operations only (10,000 quota units/day default). Write operations are not configured; this CLI is for discovery and research, not channel management.

## Quick Start

```bash
# Verify the API key and reachability.
youtube-pp-cli doctor

# Single-term search.
youtube-pp-cli youtube search-list --q "sourdough scoring" --max-results 5 --json

# Bulk search from positional args — the photo-keywords -> videos shape (use --stdin to read from a file/pipe instead).
youtube-pp-cli youtube search-bulk "sourdough scoring" "latte art" --top 3 --json

# Fetch the transcript to verify topic relevance before picking the video.
youtube-pp-cli youtube videos-transcript dQw4w9WgXcQ --lang en

# Get a markdown embed snippet for the blog draft.
youtube-pp-cli youtube videos-embed dQw4w9WgXcQ --format markdown

# Find more candidates similar to a video you've already picked.
youtube-pp-cli youtube videos-related dQw4w9WgXcQ --limit 5

```

## Known Gaps

- **Enum flag values aren't validated client-side.** Flags like `--order`, `--safe-search`, `--video-duration`, and similar carry a "one of:" hint in `--help`, but the CLI passes whatever string you supply straight to the API. An invalid value comes back as an HTTP 400 from YouTube rather than a local "unknown value" error. Use the documented values from `--help` to avoid the round-trip.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Blog-post composition
- **`youtube search-bulk`** — Take a list of search terms from stdin or args, return top-N YouTube videos per term in one JSON document with titles, channels, embed URLs, and thumbnails.

  _When you have N search terms from an upstream pipeline (image labels, photo tags, scraped keywords), reach for this instead of looping single searches._

  ```bash
  youtube-pp-cli youtube search-bulk "sourdough scoring" "latte art" --top 3
  ```
- **`youtube videos-transcript`** — Fetch the spoken-content transcript of a YouTube video using the timedtext endpoint. Works for auto-generated and manual captions on any public video. Caches into the local store.

  _Read the transcript before deciding whether a candidate video actually fits the topic of the blog post or photo._

  ```bash
  youtube-pp-cli youtube videos-transcript dQw4w9WgXcQ --lang en --json
  ```
- **`youtube videos-embed`** — Print embed HTML, iframe, or markdown-style embed for a video ID. Direct copy-paste into a blog draft.

  _Once you've picked a video for the blog, get the embed snippet without remembering the exact iframe URL pattern._

  ```bash
  youtube-pp-cli youtube videos-embed dQw4w9WgXcQ --format markdown
  ```

### Reachability mitigation
- **`youtube videos-related`** — Find videos related to a target video using topicDetails + same-channel + tag overlap from the local store. Best-effort replacement for the deprecated relatedToVideoId parameter.

  _When one candidate video is on-topic but you want more like it, this is the working stand-in for the deprecated parameter._

  ```bash
  youtube-pp-cli youtube videos-related dQw4w9WgXcQ --limit 10 --json
  ```

### Audience signal
- **`youtube videos-comments`** — Fetch top comments on a video, ranked locally by likeCount. Pulls up to 5 pages from commentThreads.list and sorts so most-liked floats regardless of API order.

  _Use to gauge whether a video is actually well-received before embedding, or to surface viewer-supplied context the description doesn't mention._

  ```bash
  youtube-pp-cli youtube videos-comments dQw4w9WgXcQ --top 10
  ```

### Channel discovery
- **`youtube channel-uploads`** — List a channel's most recent uploads in one call. Resolves @handle or channelId, walks the auto-generated uploads playlist, returns video IDs + titles + publish times.

  _Replaces the manual two-step lookup. Pairs naturally with --for-handle workflows._

  ```bash
  youtube-pp-cli youtube channel-uploads @veritasium --top 10
  ```

## Usage

Run `youtube-pp-cli --help` for the full command reference and flag list.

## Commands

### YouTube Data API v3 endpoints

- **`youtube-pp-cli youtube channels-list`** - List channels by `--id`, `--for-handle`, `--for-username`, `--mine`, or `--managed-by-me`.
- **`youtube-pp-cli youtube comment-threads-list`** - List top-level comment threads, filterable by video, channel, or thread id.
- **`youtube-pp-cli youtube playlist-items-list`** - List items in a playlist by `--playlist-id` or `--id`.
- **`youtube-pp-cli youtube playlists-list`** - List playlists by `--channel-id`, `--id`, or `--mine`.
- **`youtube-pp-cli youtube search-list`** - Search videos, channels, and playlists with full filter support (`--q`, `--order`, `--type`, etc.).
- **`youtube-pp-cli youtube videos-list`** - List videos by `--id`, `--chart`, or `--my-rating`.

### Novel commands (not in the API)

- **`youtube-pp-cli youtube search-bulk`** - Search multiple terms in one call; top-N per term.
- **`youtube-pp-cli youtube videos-transcript`** - Fetch a video's transcript via timedtext (no OAuth).
- **`youtube-pp-cli youtube videos-embed`** - Print HTML, iframe, or markdown embed snippet for a video.
- **`youtube-pp-cli youtube videos-related`** - Find related videos via topic + channel + tag overlap (replaces deprecated `relatedToVideoId`).
- **`youtube-pp-cli youtube videos-comments`** - Top comments on a video, ranked locally by likeCount.
- **`youtube-pp-cli youtube channel-uploads`** - List a channel's most recent uploads in one call (resolves `@handle` → uploads playlist).

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
youtube-pp-cli youtube channels-list

# JSON for scripting and agents
youtube-pp-cli youtube channels-list --json

# Filter to specific fields
youtube-pp-cli youtube channels-list --json --select id,name,status

# Dry run — show the request without sending
youtube-pp-cli youtube channels-list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
youtube-pp-cli youtube channels-list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
youtube-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/youtube-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `YOUTUBE_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `youtube-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $YOUTUBE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`videos transcript` returns RequestBlocked** — Run from a residential IP (your laptop). YouTube blocks AWS/GCP/Azure IPs from the timedtext endpoint.
- **`videos related` returns no results for a fresh video ID** — Run `youtube-pp-cli youtube videos-list --id <id> --parts topicDetails,snippet` first to populate topic/tag data in the local store, then retry.
- **`search` returns HTTP 403 quotaExceeded** — Daily quota (10,000 units) is exhausted. search.list costs 100 units per call, videos.list costs 1. Switch to cached results via `youtube-pp-cli sql \"SELECT * FROM search_results WHERE query='...'\"`.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Bin-Huang/youtube-data-cli**](https://github.com/Bin-Huang/youtube-data-cli) — JavaScript
- [**pauling-ai/youtube-mcp-server**](https://github.com/pauling-ai/youtube-mcp-server) — Python
- [**jdepoix/youtube-transcript-api**](https://github.com/jdepoix/youtube-transcript-api) — Python
- [**djthorpe/ytapi**](https://github.com/djthorpe/ytapi) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
