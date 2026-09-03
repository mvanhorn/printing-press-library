# MakerWorld CLI

**Every MakerWorld model, searchable offline — plus trend deltas, designer-watch, and multi-tag discovery no scraper or the web UI offers.**

A fast, agent-native CLI over MakerWorld's public catalog. Browse, search, and inspect 3D models from the terminal, mirror them into a local SQLite database for offline full-text search, then run queries the platform never exposes: what is newly rising (movers), which tracked designers shipped (designers deltas), and which models match a precise tag combination (tags). Reads need no account; an optional token unlocks 3MF downloads and your favorites.

## Install

The recommended path installs both the `makerworld-pp-cli` binary and the `pp-makerworld` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install makerworld
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install makerworld --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install makerworld --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install makerworld --agent claude-code
npx -y @mvanhorn/printing-press-library install makerworld --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/makerworld/cmd/makerworld-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/makerworld-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install makerworld --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-makerworld --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-makerworld --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install makerworld --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/makerworld-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/makerworld/cmd/makerworld-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "makerworld": {
      "command": "makerworld-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Browsing, search, and model details work with no credentials against Bambu Lab's public API. To download 3MF files or read your favorites, set MAKERWORLD_TOKEN to a Bambu Cloud token (the same JWT the Bambu Handy app uses).

## Quick Start

```bash
# Health check — confirms the CLI and API host are reachable; needs no auth.
makerworld-pp-cli doctor --dry-run

# See what is trending right now, straight from the public API.
makerworld-pp-cli designs list --nav Trending --limit 10

# Keyword-search the live catalog (designs search).
makerworld-pp-cli designs search "articulated dragon" --limit 10

# Mirror a slice of the catalog into local SQLite for offline search and deltas.
makerworld-pp-cli sync --resources designs --max-pages 5

# Surface high-quality models from your local mirror.
makerworld-pp-cli discover --sort quality --limit 20

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`discover`** — Find models that are both highly rated and printable on your setup, ranked by a composite quality score from your local mirror; constrain with --printable, --no-ams, --material, or --max-weight.

  _Reach for this when an agent needs models filtered by real quality and printer fit, not just popularity._

  ```bash
  makerworld-pp-cli discover --sort quality --no-ams --limit 20 --agent
  ```
- **`designers deltas`** — See which tracked designers posted new models or climbed in rank since your last sync.

  _Use this to answer 'what changed for the creators I follow' without re-scanning every profile._

  ```bash
  makerworld-pp-cli designers deltas --limit 50 --agent
  ```
- **`movers`** — Rank models by the biggest jump in likes, downloads, or prints between your two most recent syncs.

  _Pick this over 'browse --nav Trending' when the question is what is rising right now, not what is popular overall._

  ```bash
  makerworld-pp-cli movers --metric downloads --limit 25 --agent
  ```

### Service-specific structure
- **`tags`** — List the most common tags across your synced catalog, or find models matching ALL of several tags at once.

  _Use this to narrow to models matching a precise combination of tags, not just one._

  ```bash
  makerworld-pp-cli tags toy fidget --limit 15 --agent
  ```

## Recipes


### Inspect a model's printer requirements (deeply nested — narrow with --select)

```bash
makerworld-pp-cli designs get 2865269 --agent --select title,downloadCount,instances.weight,instances.needAms,instances.materialCnt
```

Pull just the fields that decide whether a model fits your printer, instead of the full multi-KB design payload.

### Find newly-rising household models

```bash
makerworld-pp-cli movers --metric downloads --limit 15 --agent
```

Rank models by download growth between your two most recent syncs.

### Track what your favorite designers shipped

```bash
makerworld-pp-cli designers deltas --limit 30 --agent
```

Roll up new uploads and rank rises across all synced designers.

### Only models printable without AMS

```bash
makerworld-pp-cli discover keychain --printable --no-ams --limit 20 --agent
```

Constrain discovery to single-material models that print on a base machine (no AMS).

## Usage

Run `makerworld-pp-cli --help` for the full command reference and flag list.

## Commands

### categories

List MakerWorld navigation categories

- **`makerworld-pp-cli categories`** - List navigation categories and their keys

### designers

Look up a designer's published models

- **`makerworld-pp-cli designers`** - List a designer's published designs by their numeric user ID

### designs

Browse, search, and inspect MakerWorld designs (3D models)

- **`makerworld-pp-cli designs get`** - Get full detail for one design (instances, creator, tags, counts)
- **`makerworld-pp-cli designs list`** - List designs by navigation category (Trending, For You, or a category key)
- **`makerworld-pp-cli designs ratings`** - List comments and star ratings for a design
- **`makerworld-pp-cli designs recommend`** - List recommended-for-you designs
- **`makerworld-pp-cli designs related`** - List designs related to a given design
- **`makerworld-pp-cli designs remixes`** - List designs that are remixes of a given design
- **`makerworld-pp-cli designs search`** - Keyword-search the live MakerWorld catalog


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
makerworld-pp-cli categories

# JSON for scripting and agents
makerworld-pp-cli categories --json

# Filter to specific fields
makerworld-pp-cli categories --json --select id,name,status

# Dry run — show the request without sending
makerworld-pp-cli categories --dry-run

# Agent mode — JSON + compact + no prompts in one flag
makerworld-pp-cli categories --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
makerworld-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/makerworld-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **the live keyword search returns no results for a query** — Some queries are rank-gated; retry with: designs search <kw> --order-by hotScore, or sync then use offline FTS: search <kw>.
- **movers or designers deltas returns empty** — These need at least two syncs to diff. Run 'sync' again later, then re-run.
- **download fails with 401/403** — Set MAKERWORLD_TOKEN to a current Bambu Cloud JWT; tokens expire after ~90 days.
