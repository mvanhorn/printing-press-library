# YouTube Studio (Kami creator analytics) CLI

**Every YouTube creator metric that matters, offline-queryable, with framework-audit binding to your scripts. The CLI that turns retention curves into belief-shift evidence.**

yt-studio-pp-cli is a hybrid YouTube Data API + Analytics API + Studio Innertube creator-side CLI. It caches channel videos, daily metrics, retention curves, demographics, and thumbnail CTR into a local SQLite. The killer command, framework-audit, joins retention drops to script Signal/Belief-Shift/Action-CTA lines so you have measurable evidence of which structural beat lost the audience.

Learn more at [YouTube Studio (Kami creator analytics)](https://studio.youtube.com).

Printed by [@kamiops](https://github.com/kamiops) (Kamibot Upgrade Automation).

## Install

The recommended path installs both the `yt-studio-pp-cli` binary and the `pp-yt-studio` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install yt-studio
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install yt-studio --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/cmd/yt-studio-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/yt-studio-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-yt-studio --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-yt-studio --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-yt-studio skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-yt-studio. The skill defines how its required CLI can be installed.
```

## Authentication

One-time interactive login captures OAuth refresh tokens (Data API + Analytics API, scopes youtube.readonly + yt-analytics.readonly) and the Studio web session cookies. OAuth refresh is automatic on 401. Studio session expiry surfaces as typed exit 4 with a re-login hint. Tokens live in ~/.openclaw/state/yt-studio/ mode 600.

## Quick Start

```bash
# One-time interactive: opens browser for OAuth + harvests Studio cookies
yt-studio-pp-cli login


# Pulls own channel videos + watchlist + daily metrics into local SQLite
yt-studio-pp-cli sync --full


# Quickest health check; should return your channel stats
yt-studio-pp-cli channels info --self --json --compact


# ASCII sparkline of the retention curve with 3 sharpest drops auto-annotated
yt-studio-pp-cli retention <video_id> --ascii


# The killer command: joins retention to script structure
yt-studio-pp-cli framework-audit <video_id> --script-dir ~/.openclaw/workspace/data

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`framework-audit`** — Cross-check whether a published video hits the Signal / Belief-Shift / Action-CTA structure by joining retention buckets against script lines from your local content directory.

  _When an agent reviews a published video, this is the only command that tells it which structural beat lost the audience._

  ```bash
  yt-studio-pp-cli framework-audit dQw4w9WgXcQ --script-dir ~/.openclaw/workspace/data --json
  ```

### Per-video signal extraction

- **`retention`** — Return the 100-bucket retention curve for a video as JSON or an ASCII sparkline, with the three sharpest drops auto-annotated.

  _Quickest way for an agent to identify which second of a video lost the audience._

  ```bash
  yt-studio-pp-cli retention dQw4w9WgXcQ --ascii
  ```
- **`retention-cohort`** — Average retention across videos whose title matches a regex (build-guide cohort vs rework cohort vs tier-list cohort).

  _Tells an agent whether a content format is structurally stronger than another._

  ```bash
  yt-studio-pp-cli retention-cohort --pattern "Rework" --days 30 --ascii
  ```
- **`ctr-decay`** — Compare first-72h CTR to day-30 CTR for a video; flags fast-decaying thumbnails.

  _Identifies which thumbnails earned their early lift and which are dying._

  ```bash
  yt-studio-pp-cli ctr-decay dQw4w9WgXcQ --json --compact
  ```

### Competitive context

- **`vs-watchlist`** — Compare own channel against the watchlist on CTR, retention, or upload cadence — normalized scale across all channels.

  _When an agent is planning what to publish next, this tells it which axis the channel is losing on._

  ```bash
  yt-studio-pp-cli vs-watchlist --metric ctr,retention,upload-cadence --period 28d
  ```
- **`title-patterns`** — Token-level analysis of words/phrases that correlate with above-median CTR (winners) or below-median (losers).

  _Drives an agent's title brainstorm with empirical evidence from this channel._

  ```bash
  yt-studio-pp-cli title-patterns --winners --losers --json
  ```
- **`idea-gap`** — Topics the watchlist covered in the last N days that the own channel has not.

  _Cheapest way for an agent to find what's underserved on this channel._

  ```bash
  yt-studio-pp-cli idea-gap --days 14 --json
  ```
- **`watchlist suggest`** — Auto-discover competitor channels for the watchlist via Search API; ranks by relevance and recent upload volume.

  _Saves an agent from manually curating a competitor list._

  ```bash
  yt-studio-pp-cli watchlist suggest --niche "poe2,last-epoch" --top 20 --json
  ```

### Agent-native plumbing

- **`sync`** — Incremental sync of own channel + watchlist with daily-quota budgeting; caps concurrency and prefers cache.

  _Prevents an agent from accidentally exhausting daily quota with one bad call._

  ```bash
  yt-studio-pp-cli sync --since 2026-05-01 --data-source auto --concurrency 2
  ```

## Usage

Run `yt-studio-pp-cli --help` for the full command reference and flag list.

## Commands

### captions

Caption tracks (read-only; transcript fetch deferred)

- **`yt-studio-pp-cli captions list`** - List caption tracks for a video

### categories

YouTube video category metadata

- **`yt-studio-pp-cli categories list`** - List video categories for a region

### channels

YouTube channels (own channel + competitor watchlist)

- **`yt-studio-pp-cli channels info`** - Get the authenticated user's channel (--self) or a single channel by id
- **`yt-studio-pp-cli channels list`** - List channel resources by id, mine=true, or forHandle

### comments

Video comment threads (read-only)

- **`yt-studio-pp-cli comments list`** - List top-level comment threads for a video

### discover

YouTube search.list API (100 quota units per request — use sparingly; named 'discover' because 'search' is a built-in framework command for offline FTS)

- **`yt-studio-pp-cli discover list`** - Search for videos, channels, or playlists via the YouTube Data API search.list endpoint. WARNING: 100 quota units per call.

### playlistItems

Items within a playlist

- **`yt-studio-pp-cli playlistItems list`** - List items in a playlist

### playlists

YouTube playlists

- **`yt-studio-pp-cli playlists get`** - Get a single playlist by ID
- **`yt-studio-pp-cli playlists list`** - List playlists by channelId or id

### videos

YouTube videos (own + watchlist)

- **`yt-studio-pp-cli videos get`** - Get a single video by ID
- **`yt-studio-pp-cli videos list`** - List video metadata by id


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
yt-studio-pp-cli captions --video-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
yt-studio-pp-cli captions --video-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
yt-studio-pp-cli captions --video-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
yt-studio-pp-cli captions --video-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
yt-studio-pp-cli captions --video-id 550e8400-e29b-41d4-a716-446655440000 --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-yt-studio -g
```

Then invoke `/pp-yt-studio <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/cmd/yt-studio-pp-mcp@latest
```

Then register it:

```bash
# Set up auth first:
yt-studio-pp-cli auth login

claude mcp add yt-studio yt-studio-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local OAuth tokens — authenticate first if you haven't:

```bash
yt-studio-pp-cli auth login
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/yt-studio-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `YT_STUDIO_OAUTH_CLIENT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/cmd/yt-studio-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "yt-studio": {
      "command": "yt-studio-pp-mcp",
      "env": {
        "YT_STUDIO_OAUTH_CLIENT_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
yt-studio-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/yt-studio-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `YT_STUDIO_OAUTH_CLIENT_ID` | auth_flow_input | Yes | OAuth 2.0 Client ID from your Google Cloud Console Desktop Client |
| `YT_STUDIO_OAUTH_CLIENT_SECRET` | auth_flow_input | Yes | Set during initial auth setup. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `yt-studio-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $YT_STUDIO_OAUTH_CLIENT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Exit 4 on every command** — Run `yt-studio-pp-cli login` to refresh OAuth + capture a new Studio session
- **Exit 5 on Studio-dependent commands (real-time, A/B thumbnails)** — Studio Innertube schema drift; run `yt-studio-pp-cli sniff-doctor` for the suspected mismatch, then file an issue
- **Exit 7 on sync** — Daily quota exhausted (10K units). Wait until midnight PT or run `--data-source local` until reset
- **framework-audit returns 'no script binding'** — Run `yt-studio-pp-cli script-link <video_id> <script_path>` to manually bind the script, or add a `**Video ID:** <id>` line to ~/.openclaw/workspace/data/content-registry.md
- **Empty results from `search`** — Run `yt-studio-pp-cli sync --full` first; offline FTS needs synced data

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**google-api-go-client**](https://github.com/googleapis/google-api-go-client) — Go (4000 stars)
- [**mcp-youtube**](https://github.com/anaisbetts/mcp-youtube) — TypeScript (300 stars)
- [**mcp-youtube**](https://github.com/adhikasp/mcp-youtube) — Python (100 stars)
- [**youtube-mcp-server**](https://github.com/dannySubsense/youtube-mcp-server) — Python (50 stars)
- [**youtube-mcp**](https://github.com/nattyraz/youtube-mcp) — TypeScript (30 stars)
- [**youtube-mcp-server**](https://github.com/mourad-ghafiri/youtube-mcp-server) — Python (20 stars)
- [**yt-mcp**](https://github.com/space-cadet/yt-mcp) — TypeScript (15 stars)
- [**youtube-cli**](https://github.com/danvega/youtube-cli) — Java (10 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
