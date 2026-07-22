# Boardgamegeek CLI

BoardGameGeek's official XMLAPI2 (https://boardgamegeek.com/xmlapi2), the
canonical database for board games, RPGs, and video games. Every endpoint
returns XML; the generated client normalizes it to JSON so --json/--select
and MCP tools work like any JSON API.

Authorization: as of 2025-07-02 BGG requires a registered application and an
Authorization: Bearer <token> header on every request (register at
https://boardgamegeek.com/applications, then create a token). Requests go to
boardgamegeek.com (no leading www).

Scope: read-only lookup across the BGG taste graph — search, full game/thing
detail with stats and rankings, the live "hot" list, user profiles, user game
collections, logged plays, families, and guilds. No write/mutating endpoints.

Quirk worth knowing: collection (and occasionally thing/plays) can answer with
HTTP 202 and a "your request is queued, retry shortly" body the first time a
large query is requested; repeat the call until the data is returned.

## Install

The recommended path installs both the `boardgamegeek-pp-cli` binary and the `pp-boardgamegeek` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install boardgamegeek
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install boardgamegeek --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install boardgamegeek --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install boardgamegeek --agent claude-code
npx -y @mvanhorn/printing-press-library install boardgamegeek --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/boardgamegeek/cmd/boardgamegeek-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/boardgamegeek-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install boardgamegeek --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-boardgamegeek --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-boardgamegeek --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install boardgamegeek --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/boardgamegeek-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BGG_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/boardgamegeek/cmd/boardgamegeek-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "boardgamegeek": {
      "command": "boardgamegeek-pp-mcp",
      "env": {
        "BGG_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
boardgamegeek-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export BGG_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
boardgamegeek-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
boardgamegeek-pp-cli collection --username example-resource
```

## Usage

Run `boardgamegeek-pp-cli --help` for the full command reference and flag list.

## Commands

### collection

A user's owned/rated/wishlisted game collection

- **`boardgamegeek-pp-cli collection`** - A user's collection by login name. Filter with the 0/1 status flags
(own, rated, played, wishlist, want, trade). `stats=1` adds per-game
rating and rank. Large collections may answer with HTTP 202 the first
time and need a retry.

### family

Manage family

- **`boardgamegeek-pp-cli family`** - A BGG family (a shared series, theme, or IP grouping related things) by
id. Comma-separate ids for a batch lookup.

### guild

Guild details and membership

- **`boardgamegeek-pp-cli guild`** - A BGG guild by id. Set `members=1` to include the member roster
(paginated with `page`).

### hot

The current BoardGameGeek "Hot" rankings

- **`boardgamegeek-pp-cli hot`** - The live BoardGameGeek Hot list. `type` selects the ranking
(boardgame is the default and most common).

### plays

Logged play sessions for a user or a game

- **`boardgamegeek-pp-cli plays`** - Logged play sessions. Provide `username` (a user's plays) or `id` (a
game's plays). Filter by date range and paginate with `page`.

### searches

Manage searches

- **`boardgamegeek-pp-cli searches`** - Full-text search across the BGG database. Returns matching things (id,
name, year). Narrow with `type` and set `exact=1` for exact-name matches.

### thing

Full detail for one or more things (games, expansions, accessories)

- **`boardgamegeek-pp-cli thing`** - Full record for a game/expansion/accessory by id (comma-separate ids for
a batch). Opt into extra sections with the 0/1 flags. `stats=1` adds the
rating, rank, and ownership statistics most callers want.

### user

Public user profiles, buddies, and guild membership

- **`boardgamegeek-pp-cli user`** - Public profile for a BGG user by login name. Opt into buddies, guilds,
and the user's personal hot/top lists with the 0/1 flags.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
boardgamegeek-pp-cli collection --username example-resource

# JSON for scripting and agents
boardgamegeek-pp-cli collection --username example-resource --json

# Filter to specific fields
boardgamegeek-pp-cli collection --username example-resource --json --select id,name,status

# Dry run — show the request without sending
boardgamegeek-pp-cli collection --username example-resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
boardgamegeek-pp-cli collection --username example-resource --agent
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
boardgamegeek-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/boardgamegeek-xml-api2-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BGG_TOKEN` | per_call | Yes | Set to your API credential. |
| `BOARDGAMEGEEK_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `boardgamegeek-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `boardgamegeek-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BGG_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
