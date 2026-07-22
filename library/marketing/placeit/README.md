# Placeit CLI

**Every one of Placeit's 164k mockup, logo, video, and design templates — searchable, sortable by real popularity, and cached offline from your terminal, with catalog analytics Placeit's own UI can't do.**

Placeit has no public API, no bulk search, and no offline catalog. This CLI mirrors the entire Algolia-indexed catalog into a local SQLite database, then layers on commands no Placeit tool has: rank results by actual purchase count (top), filter Printify-ready mockups for a POD pipeline (pod), assemble matched Twitch kits (kit), and cross-tabulate the catalog's tag facets (gaps). Search and browse need no login; account and saved-templates use your Placeit session.

Learn more at [Placeit](https://placeit.net).

Created by [@vcolombo](https://github.com/vcolombo) (Vincent Colombo).

## Install

The recommended path installs both the `placeit-pp-cli` binary and the `pp-placeit` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install placeit
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install placeit --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install placeit --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install placeit --agent claude-code
npx -y @mvanhorn/printing-press-library install placeit --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/placeit/cmd/placeit-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/placeit-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install placeit --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-placeit --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-placeit --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install placeit --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
placeit-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/placeit-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/placeit/cmd/placeit-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "placeit": {
      "command": "placeit-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Search, browse, sync, and template lookup use Placeit's public Algolia catalog and need no credentials. The account and bookmarks commands read your Placeit (Envato) session, which you import once from Chrome via the auth login step shown in Quick Start. Because Placeit sits behind Cloudflare, the CLI ships a browser-compatible HTTP transport for those calls.

## Quick Start

```bash
# Check catalog reachability before anything else (no login needed).
placeit-pp-cli doctor

# Search the live catalog by keyword and category.
placeit-pp-cli search "t-shirt mockup" --category mockups --limit 10

# Build a local offline mirror so search and analytics work without the network.
placeit-pp-cli sync --category mockups --max-pages 5

# Rank cached mockups by real purchase count.
placeit-pp-cli top "hoodie" --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Popularity intelligence
- **`top`** — Rank matching templates by real purchase count, not opaque relevance.

  _Reach for this when a seller needs the mockups that actually convert, not just the first search hits._

  ```bash
  placeit-pp-cli top "t-shirt" --category mockups --agent
  ```
- **`rank`** — Show a template's purchase percentile within its category and tag cohort.

  _Reach for this to judge whether a specific template is a strong pick before committing to it._

  ```bash
  placeit-pp-cli rank 41935 --agent
  ```

### Print-on-demand
- **`pod`** — Return only Printify-compatible mockups for a query with deep links for a listing pipeline.

  _Reach for this when batching POD listings so you never open a mockup that isn't print-ready._

  ```bash
  placeit-pp-cli pod "hoodie" --agent --select stage_link,editor_link
  ```

### Matched sets
- **`kit`** — Assemble a coherent streamer kit (overlay, panels, emotes, frame) and flag missing slots.

  _Reach for this when a streamer rebrand needs a visually consistent set, not mismatched single assets._

  ```bash
  placeit-pp-cli kit "neon gaming" --agent
  ```

### Taxonomy navigation
- **`industry-map`** — Map the 152-entry industry taxonomy with template counts from your synced mirror.

  _Reach for this to size a niche and find the industries with the most ready-made templates._

  ```bash
  placeit-pp-cli industry-map "coffee shop"
  ```

### Monitoring
- **`watch`** — Persist named searches and report templates newly matching since the last sync.

  _Reach for this to keep a client niche fresh without re-running searches by hand every week._

  ```bash
  placeit-pp-cli watch run --agent
  ```

### Catalog analytics
- **`gaps`** — Pivot the catalog across two tag facets to surface under-served template combinations.

  _Reach for this to spot mockup coverage gaps (e.g. a device with thin on-model diversity)._

  ```bash
  placeit-pp-cli gaps --facet device_tags --by ethnicity_tags
  ```

## Recipes


### Find the best-selling Printify t-shirt mockups

```bash
placeit-pp-cli pod "t-shirt" --agent --select name,stage_link,purchases
```

Filters to Printify-ready mockups and narrows the JSON to just the fields a listing pipeline needs.

### Rank cached logos by popularity in a niche

```bash
placeit-pp-cli top "coffee shop logo" --category logos --limit 20 --agent
```

Orders local-mirror matches by real purchase count for agent consumption.

### Audit on-model mockup diversity

```bash
placeit-pp-cli gaps --facet device_tags --by ethnicity_tags
```

Cross-tabulates two tag facets to reveal under-served mockup combinations.

### Narrow a deeply nested template record for an agent

```bash
placeit-pp-cli template 41935 --agent --select name,category_name,stage_link,large_thumb,device_tags
```

Template records carry 40+ fields and 9 tag arrays; --select keeps only what the agent needs.

### Watch a client niche for new templates

```bash
placeit-pp-cli watch add "halloween instagram" && placeit-pp-cli watch run --agent
```

Saves a named search, then reports templates newly published since the last sync.

## Usage

Run `placeit-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Your Placeit account and subscription status (requires a logged-in session)

- **`placeit-pp-cli account`** - Show the signed-in user's account type and subscription status

### bookmarks

Templates you've saved/bookmarked on Placeit (requires a logged-in session)

- **`placeit-pp-cli bookmarks`** - List the signed-in user's bookmarked templates

### campaigns

Active Placeit marketing campaigns and promotions (no login required)

- **`placeit-pp-cli campaigns`** - List active Placeit campaigns and promotions


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
placeit-pp-cli bookmarks --user-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
placeit-pp-cli bookmarks --user-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
placeit-pp-cli bookmarks --user-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
placeit-pp-cli bookmarks --user-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
placeit-pp-cli bookmarks --user-id 550e8400-e29b-41d4-a716-446655440000 --agent
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
placeit-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/placeit-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `placeit-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **search returns nothing offline** — search queries the live Placeit catalog by default and needs no sync; for offline use, run a sync first to build the local mirror.
- **account or bookmarks returns 403 / Cloudflare challenge** — Re-import a fresh Chrome session via the auth login step in Quick Start; Placeit sessions expire.
- **gaps or rank show 0 results** — gaps and rank read the local mirror; run a sync to populate it before using them.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
