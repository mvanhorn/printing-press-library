# Notion CLI

**Every Notion database queryable offline — cross-workspace SQL joins, stale detection, and relation graph traversal the UI can never give you.**

notion-pp-cli syncs your Notion workspace into a local SQLite store and exposes commands that answer compound questions the Notion UI cannot: cross-database joins, status drift, dead links, workspace health, and who owns what across every client. Works offline and ships a full MCP server so agents can load rich project context in a single call instead of 30.

## Install

The recommended path installs both the `notion-pp-cli` binary and the `pp-notion` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install notion
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install notion --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/notion/cmd/notion-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/notion-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-notion --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-notion --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-notion skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-notion. The skill defines how its required CLI can be installed.
```

## Authentication

Requires a Notion Internal Integration token. Create one at notion.so/my-integrations, then share your databases with it. Set NOTION_TOKEN in your environment. Run `notion-pp-cli sync` once to populate the local store.

## Quick Start

```bash
# Paste your NOTION_TOKEN from notion.so/my-integrations
notion-pp-cli auth set-token


# Mirror your entire workspace to local SQLite — takes 30-120 seconds depending on workspace size
notion-pp-cli sync --full


# Find everything untouched for 30+ days
notion-pp-cli stale --days 30 --json


# Get a hygiene scorecard for the whole workspace
notion-pp-cli workspace-health


# Raw SQL against the local store for custom queries
notion-pp-cli sql "SELECT title, last_edited_time FROM pages ORDER BY last_edited_time DESC LIMIT 20" --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`changed`** — Show everything in the workspace added, edited, or deleted since your last sync or a given timestamp.

  _Use at the start of an agent session to orient on what has changed before taking action._

  ```bash
  notion-pp-cli changed --since 2h --json
  ```

### Local state that compounds
- **`stale`** — List pages and records not edited in N days, filterable by database, parent, or tag.

  _Use to identify dead pages before workspace cleanup or to flag deliverables overdue for review._

  ```bash
  notion-pp-cli stale --days 30 --db ProjectDB --agent
  ```

## Usage

Run `notion-pp-cli --help` for the full command reference and flag list.

## Commands

### blocks

Block endpoints

- **`notion-pp-cli blocks delete-a`** - Delete a block
- **`notion-pp-cli blocks query-meeting-notes`** - Query meeting notes
- **`notion-pp-cli blocks retrieve-a`** - Retrieve a block
- **`notion-pp-cli blocks update-a`** - Update a block

### comments

Comment endpoints

- **`notion-pp-cli comments create-a`** - Create a comment
- **`notion-pp-cli comments delete-a`** - Delete a comment
- **`notion-pp-cli comments list`** - List comments
- **`notion-pp-cli comments retrieve`** - Retrieve a comment
- **`notion-pp-cli comments update-a`** - Update a comment

### custom-emojis

Custom emoji endpoints

- **`notion-pp-cli custom-emojis list`** - List custom emojis

### data-sources

Data source endpoints

- **`notion-pp-cli data-sources create-a-database`** - Create a data source
- **`notion-pp-cli data-sources retrieve-a`** - Retrieve a data source
- **`notion-pp-cli data-sources update-a`** - Update a data source

### databases

Database endpoints

- **`notion-pp-cli databases create`** - Create a database
- **`notion-pp-cli databases retrieve`** - Retrieve a database
- **`notion-pp-cli databases update`** - Update a database

### file-uploads

File upload endpoints

- **`notion-pp-cli file-uploads create-file`** - Create a file upload
- **`notion-pp-cli file-uploads list`** - List file uploads
- **`notion-pp-cli file-uploads retrieve`** - Retrieve a file upload

### notion-search

Manage notion search

- **`notion-pp-cli notion-search post`** - Search by title

### oauth

OAuth endpoints (basic authentication)

- **`notion-pp-cli oauth create-a-token`** - Exchange an authorization code for an access and refresh token
- **`notion-pp-cli oauth introspect-token`** - Introspect a token
- **`notion-pp-cli oauth revoke-token`** - Revoke a token

### pages

Page endpoints

- **`notion-pp-cli pages patch`** - Update page
- **`notion-pp-cli pages post`** - Create a page
- **`notion-pp-cli pages retrieve-a`** - Retrieve a page

### users

User endpoints

- **`notion-pp-cli users get`** - List all users
- **`notion-pp-cli users get-self`** - Retrieve your token's bot user
- **`notion-pp-cli users get-userid`** - Retrieve a user

### views

View endpoints

- **`notion-pp-cli views create`** - Create a view
- **`notion-pp-cli views delete`** - Delete a view
- **`notion-pp-cli views list`** - List views
- **`notion-pp-cli views retrieve-a`** - Retrieve a view
- **`notion-pp-cli views update-a`** - Update a view


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
notion-pp-cli comments list --block-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
notion-pp-cli comments list --block-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
notion-pp-cli comments list --block-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
notion-pp-cli comments list --block-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
notion-pp-cli comments list --block-id 550e8400-e29b-41d4-a716-446655440000 --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-notion -g
```

Then invoke `/pp-notion <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/notion/cmd/notion-pp-mcp@latest
```

Then register it:

```bash
claude mcp add notion notion-pp-mcp -e NOTION_BEARER_AUTH=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/notion-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `NOTION_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/notion/cmd/notion-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "notion": {
      "command": "notion-pp-mcp",
      "env": {
        "NOTION_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
notion-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/notion-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `NOTION_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `notion-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $NOTION_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **sync returns 'object not found' for some databases** — Share those databases with your integration at notion.so/my-integrations — the token only sees what you've explicitly shared
- **stale/cross-db query returns empty results** — Run `notion-pp-cli sync --full` first to populate the local store
- **NOTION_TOKEN not recognized** — Token format is `ntn_...` for internal integrations — do not use your Notion account password or OAuth tokens

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**notion-sdk-js**](https://github.com/makenotion/notion-sdk-js) — TypeScript (5598 stars)
- [**notion-mcp-server (official)**](https://github.com/makenotion/notion-mcp-server) — TypeScript (4307 stars)
- [**notion-sdk-py**](https://github.com/ramnes/notion-sdk-py) — Python (2171 stars)
- [**mcp-notion-server (suekou)**](https://github.com/suekou/mcp-notion-server) — TypeScript (887 stars)
- [**notion2md**](https://github.com/echo724/notion2md) — Python (758 stars)
- [**notionapi (Go)**](https://github.com/jomei/notionapi) — Go (574 stars)
- [**notion-cli (4ier)**](https://github.com/4ier/notion-cli) — Go (192 stars)
- [**notion-cli (kris-hansen)**](https://github.com/kris-hansen/notion-cli) — Python (132 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
