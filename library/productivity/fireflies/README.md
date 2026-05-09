# Fireflies.ai CLI

**Every Fireflies meeting feature, plus offline search, cross-meeting intelligence, and a local database no other tool has.**

Sync your entire meeting history once, then search, analyze, and correlate across every conversation without touching the API. Find stale action items, track topic escalation over weeks, reconstruct the full history with any person or account — all offline, all composable with jq and SQL.

## Install

The recommended path installs both the `fireflies-pp-cli` binary and the `pp-fireflies` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install fireflies
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install fireflies --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/fireflies/cmd/fireflies-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fireflies-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-fireflies --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-fireflies --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-fireflies skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-fireflies. The skill defines how its required CLI can be installed.
```

## Authentication

Requires a Fireflies API key set as FIREFLIES_API_KEY. API access requires a Business plan or higher. Get your key at app.fireflies.ai → Settings → Developer.

## Quick Start

```bash
# verify auth and API reachability
fireflies-pp-cli doctor


# pull all transcripts + summaries + sentences into local SQLite
fireflies-pp-cli sync --full


# see your recent meetings
fireflies-pp-cli transcripts list --mine --limit 10


# full-text search offline
fireflies-pp-cli search "action item" --from 7d --agent


# find dropped commitments
fireflies-pp-cli action-items stale --days 14 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`transcripts search`** — Full-text search across all synced meeting transcripts without consuming any API quota.

  _Use to find every time a specific topic was mentioned across all meetings without burning rate-limited API calls._

  ```bash
  fireflies-pp-cli transcripts search "pricing objection" --agent --select id,title,dateString
  ```
- **`action-items list`** — Aggregate action items from all meetings in a date range — weekly commitment audit in one command.

  _Use at the end of the week to harvest all commitments made in meetings and push to a TODO file._

  ```bash
  fireflies-pp-cli action-items list --from 7 --agent
  ```
- **`transcripts find`** — Find meetings by participant email, channel name, keyword, or date range — all client-side, no broken API date filters.

  _Use when you need meetings with a specific person or channel — the API's title-based search fails when meeting names don't contain participant names._

  ```bash
  fireflies-pp-cli transcripts find --participant danijel.latin@verybigthings.com --from 30 --processed-only --agent
  ```
- **`transcripts status`** — Show PROCESSED / PROCESSING / FAILED status for recent meetings upfront — know before you fetch.

  _Use when fetching same-day or next-morning meetings — avoids the loop of 'meeting not ready yet' failures._

  ```bash
  fireflies-pp-cli transcripts status --since 48h --agent
  ```
- **`topics list`** — Most frequent topics across all meetings in a date range — what is actually consuming meeting time.

  _Use during quarterly planning to identify recurring themes before deciding where to invest time._

  ```bash
  fireflies-pp-cli topics list --from 30 --top 15 --agent
  ```
- **`digest`** — Aggregate view of all recent meetings: titles, gists, topics, and action items in one structured output.

  _Use at session start or in a morning cron to orient on what happened yesterday before taking action._

  ```bash
  fireflies-pp-cli digest --since 24h --agent
  ```
- **`transcripts export`** — Export a transcript as markdown to a vault directory with auto-generated YYYY-MM-DD_title.md filename.

  _Use after a client meeting to save the formatted transcript directly to the right project folder._

  ```bash
  fireflies-pp-cli transcripts export abc123 --vault ~/vaults/VBT/Projects/1_Active/Ryder/transcripts/ --agent
  ```

### Person-centric intelligence
- **`person timeline`** — Chronological meeting history with a specific person — topics, action items, and talk ratio per meeting.

  _Use before a QBR or renewal call to reconstruct the full relationship history without reading every transcript._

  ```bash
  fireflies-pp-cli person timeline danijel.latin@verybigthings.com --from 90 --agent
  ```

## Usage

Run `fireflies-pp-cli --help` for the full command reference and flag list.

## Commands

### active-meetings

Manage active-meetings

- **`fireflies-pp-cli active-meetings get`** - Get a single activemeeting
- **`fireflies-pp-cli active-meetings update`** - Update a activemeeting

### analyticses

Manage analyticses

- **`fireflies-pp-cli analyticses get`** - Get a single analytics

### app-outputs

Manage app-outputs

- **`fireflies-pp-cli app-outputs get`** - Get a single appoutput

### ask-fred-responses

Manage ask-fred-responses

- **`fireflies-pp-cli ask-fred-responses create`** - Create a askfredresponse

### ask-fred-thread-summaries

Manage ask-fred-thread-summaries

- **`fireflies-pp-cli ask-fred-thread-summaries get`** - Get a single askfredthreadsummary

### ask-fred-threads

Manage ask-fred-threads

- **`fireflies-pp-cli ask-fred-threads get`** - Get a single askfredthread

### bites

Manage bites

- **`fireflies-pp-cli bites create`** - Create a bite
- **`fireflies-pp-cli bites get`** - Get a single bite

### channels

Manage channels

- **`fireflies-pp-cli channels get`** - Get a single channel

### contacts

Manage contacts

- **`fireflies-pp-cli contacts get`** - Get a single contact

### live-action-items

Manage live-action-items

- **`fireflies-pp-cli live-action-items create`** - Create a liveactionitem
- **`fireflies-pp-cli live-action-items get`** - Get a single liveactionitem

### mutation-results

Manage mutation-results

- **`fireflies-pp-cli mutation-results delete`** - Delete a mutationresult
- **`fireflies-pp-cli mutation-results update`** - Update a mutationresult

### transcripts

Manage transcripts

- **`fireflies-pp-cli transcripts get`** - Get a single transcript

### user-groups

Manage user-groups

- **`fireflies-pp-cli user-groups get`** - Get a single usergroup

### users

Manage users

- **`fireflies-pp-cli users get`** - Get a single user


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
fireflies-pp-cli active-meetings get

# JSON for scripting and agents
fireflies-pp-cli active-meetings get --json

# Filter to specific fields
fireflies-pp-cli active-meetings get --json --select id,name,status

# Dry run — show the request without sending
fireflies-pp-cli active-meetings get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
fireflies-pp-cli active-meetings get --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-fireflies -g
```

Then invoke `/pp-fireflies <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/fireflies/cmd/fireflies-pp-mcp@latest
```

Then register it:

```bash
claude mcp add fireflies fireflies-pp-mcp -e FIREFLIES_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fireflies-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FIREFLIES_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/fireflies/cmd/fireflies-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "fireflies": {
      "command": "fireflies-pp-mcp",
      "env": {
        "FIREFLIES_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
fireflies-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/fireflies-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FIREFLIES_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `fireflies-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FIREFLIES_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **403 Forbidden on any query** — Fireflies API requires Business plan or higher; verify at app.fireflies.ai/settings/billing
- **stale/search returns no results** — Run 'fireflies-pp-cli sync --full' first to populate the local store
- **rate limit 429 with retryAfter** — Free/Pro: 50 req/day; Business+: 60 req/min — the CLI retries with exponential backoff automatically
- **audio_url/video_url empty in transcript** — Audio URLs expire after 24h; re-fetch the transcript to get a fresh URL

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**bcharleson/fireflies-cli**](https://github.com/bcharleson/fireflies-cli) — TypeScript (120 stars)
- [**johntoups/mcp-fireflies**](https://github.com/johntoups/mcp-fireflies) — Python (40 stars)
- [**cassler/fireflies-mcp-server**](https://github.com/cassler/fireflies-mcp-server) — JavaScript (30 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
