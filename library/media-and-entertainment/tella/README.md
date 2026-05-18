# Tella CLI

**Every Tella API operation behind one CLI, with a local SQLite store, FTS5 transcript search, and webhook tooling that replaces ngrok.**

Tella ships an API and an official MCP server; this CLI gives you both surfaces in one binary, plus a local-first store that makes cross-video transcript search, view-milestone rollups, and webhook replay actually fast. Every endpoint is a Cobra command, every command emits structured JSON, and every mutation supports --dry-run.

Printed by [@gregce](https://github.com/gregce) (Greg Ceccarelli).

## Install

The recommended path installs both the `tella-pp-cli` binary and the `pp-tella` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install tella
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install tella --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tella-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tella --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tella --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-tella skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-tella. The skill defines how its required CLI can be installed.
```

## Authentication

Set TELLA_API_KEY to your Tella account API key (Account → Settings → API). Sent as Authorization: Bearer on every call. No OAuth, no refresh — it's a single static token.

## Quick Start

```bash
# Save your Tella API key to local config; used by every command.
tella-pp-cli auth set-token


# Verify token + reachability before doing anything else.
tella-pp-cli doctor


# Sanity-check that you can see your workspace.
tella-pp-cli videos list --json --limit 5


# Populate the local SQLite store; required for transcript search and milestone digest.
tella-pp-cli sync


# Offline FTS5 search across every cached transcript.
tella-pp-cli transcripts search "pricing" --json


# Stream new webhook events to stdout; useful during integration development.
tella-pp-cli webhooks tail --follow

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local-first transcendence
- **`transcripts search`** — FTS5 search across every cached clip transcript in your workspace; returns video, clip, and timecode hits in milliseconds.

  _Use this when an agent or human needs to find every video that mentioned a topic without rehydrating the workspace from the API._

  ```bash
  tella-pp-cli transcripts search "pricing change" --json --limit 10
  ```
- **`videos viewed`** — Roll up webhook view-milestone events into a per-video summary over a window (e.g. who crossed 75% in the last 7 days).

  _Use this in a sales follow-up loop to triage prospects by engagement without scanning the dashboard._

  ```bash
  tella-pp-cli videos viewed --since 7d --milestone 75 --json
  ```
- **`workspace stats`** — Local aggregate of video count, clip count, total duration, transcript word count, and export count by month across the cached workspace.

  _Use this for monthly creator-economy reports without dashboard scraping._

  ```bash
  tella-pp-cli workspace stats --json
  ```

### Webhook tooling
- **`webhooks tail`** — Stream new webhook events from the inbox to stdout, and replay any prior message to a local URL with valid HMAC headers — no public tunnel needed.

  _Use this when developing a webhook handler against Tella without exposing localhost via a tunnel._

  ```bash
  tella-pp-cli webhooks tail --follow --json
  ```

### Bulk operations
- **`clips edit-pass`** — Apply a chained set of edits (remove-fillers, trim-silences-gt N, blur preset) across every clip in a playlist in one command.

  _Use this to apply a creator's standard edit pass across an entire playlist without per-clip clicking._

  ```bash
  tella-pp-cli clips edit-pass --playlist plst_42 --remove-fillers --trim-silences-gt 1s --dry-run
  ```
- **`exports wait`** — Kick off exports for one or more videos and block until each is ready, short-circuiting on the Export ready webhook event.

  _Use this in batch publishing scripts that need export URLs for downstream uploads._

  ```bash
  tella-pp-cli exports wait --video vid_1 --video vid_2 --timeout 10m --json
  ```

### Transcript tooling
- **`clips transcript-diff`** — Diff a clip's cut transcript against its uncut transcript to surface every word that editing removed (filler, silence, hand-edit) with timecodes.

  _Use this to audit what an automated edit pass actually changed before publishing._

  ```bash
  tella-pp-cli clips transcript-diff clp_abc --json
  ```
- **`clips captions`** — Format a clip's cut transcript as an SRT or VTT subtitle file ready to attach to an embed or upload.

  _Use this to ship caption files alongside video embeds without round-tripping through a separate caption tool._

  ```bash
  tella-pp-cli clips captions clp_abc --format srt > captions.srt
  ```

## Usage

Run `tella-pp-cli --help` for the full command reference and flag list.

## Commands

### playlists

Playlist operations

- **`tella-pp-cli playlists create`** - Create a new playlist for the authenticated user
- **`tella-pp-cli playlists delete`** - Permanently delete a playlist. Videos in the playlist are not deleted.
- **`tella-pp-cli playlists get`** - Returns detailed information about a playlist including its videos
- **`tella-pp-cli playlists list`** - Returns a list of all playlists for the authenticated user
- **`tella-pp-cli playlists update`** - Update a playlist's name and/or description

### videos

Video operations

- **`tella-pp-cli videos delete`** - Permanently delete a video
- **`tella-pp-cli videos get`** - Returns detailed information about a video including chapters, transcript, and thumbnails
- **`tella-pp-cli videos list`** - Returns a paginated list of all videos for the authenticated user. Use playlistId query parameter to filter videos by playlist.
- **`tella-pp-cli videos update`** - Update a video's settings including viewer options, download permissions, access controls, and metadata. Some features require Premium plan.

### webhooks

Webhook endpoint management

- **`tella-pp-cli webhooks create-endpoint`** - Creates a new webhook endpoint to receive events. Returns the endpoint ID and signing secret.
- **`tella-pp-cli webhooks delete-endpoint`** - Permanently deletes a webhook endpoint
- **`tella-pp-cli webhooks get-endpoint-secret`** - Retrieves the signing secret for a webhook endpoint. Use this to verify incoming webhook payloads.
- **`tella-pp-cli webhooks get-message`** - Returns details of a specific webhook message by ID
- **`tella-pp-cli webhooks list-messages`** - Returns a list of recently sent webhook messages for debugging purposes


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tella-pp-cli playlists list

# JSON for scripting and agents
tella-pp-cli playlists list --json

# Filter to specific fields
tella-pp-cli playlists list --json --select id,name,status

# Dry run — show the request without sending
tella-pp-cli playlists list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tella-pp-cli playlists list --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-tella -g
```

Then invoke `/pp-tella <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add tella tella-pp-mcp -e TELLA_API_KEY=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tella-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TELLA_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "tella": {
      "command": "tella-pp-mcp",
      "env": {
        "TELLA_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
tella-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/tella-public-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TELLA_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `tella-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TELLA_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every call** — Re-fetch your API key at tella.tv → Account → Settings → API and run `tella-pp-cli auth set-token` again.
- **Empty results from `transcripts search --data-source local`** — Run `tella-pp-cli sync` first to populate the local transcripts table.
- **`webhooks tail` shows no events** — Confirm a webhook endpoint is registered (`tella-pp-cli webhooks endpoint create`); the inbox only collects events for registered endpoints.
- **`clips edit-pass --dry-run` shows zero planned mutations** — The playlist may be empty or the silence threshold (--trim-silences-gt) is larger than every gap; run `clips silences --playlist <id>` to inspect raw ranges.
- **Export wait times out** — Default timeout is 10m. Long videos can exceed; pass `--timeout 30m` and ensure the Export ready webhook endpoint is registered for early termination.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
