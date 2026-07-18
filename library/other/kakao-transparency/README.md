# Kakao Transparency CLI

**Kakao's 2012-present government data-request statistics as one queryable archive — series, latest-report resolution, and workbook mirroring no other tool offers.**

Kakao publishes semiannual transparency reports (warrants, communication data, restriction measures for Kakao and Daum services) behind a one-period-at-a-time web page. This CLI turns that into an agent-friendly archive: `series` builds the full longitudinal table in one call, `latest` resolves the newest period without guessing, and `workbooks` indexes the official XLSX editions.

Learn more at [Kakao Transparency](https://privacy.kakao.com/transparency).

Created by [@krMaynard](https://github.com/krMaynard) (Kieran Maynard).

## Install

The recommended path installs both the `kakao-transparency-pp-cli` binary and the `pp-kakao-transparency` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install kakao-transparency
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install kakao-transparency --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install kakao-transparency --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install kakao-transparency --agent claude-code
npx -y @mvanhorn/printing-press-library install kakao-transparency --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/kakao-transparency/cmd/kakao-transparency-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/kakao-transparency-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install kakao-transparency --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-kakao-transparency --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-kakao-transparency --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install kakao-transparency --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/kakao-transparency-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/kakao-transparency/cmd/kakao-transparency-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "kakao-transparency": {
      "command": "kakao-transparency-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify connectivity — the API is public, no credentials needed
kakao-transparency-pp-cli doctor --dry-run

# Resolve and fetch the newest published half-year
kakao-transparency-pp-cli latest --agent

# Fetch one specific half-year report
kakao-transparency-pp-cli transparency --year 2025 --half-year-id 2

# Build the 2012-present warrant series as CSV
kakao-transparency-pp-cli series --category warrant --csv

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Whole-archive analytics
- **`series`** — One tidy table of every published half-year since 2012 — requests, processed, and affected accounts per service corporation and request category — instead of 28 separate report lookups.

  _Reach for this when a task needs trends over time (e.g. 'how did warrant volumes change since 2015') rather than a single period's numbers._

  ```bash
  kakao-transparency-pp-cli series --category warrant --service kakao --csv
  ```
- **`workbooks`** — List the official XLSX workbook download URLs (Korean and English editions) for every published half-year.

  _Use this when the deliverable needs the official source files (archival mirroring, citation) rather than the parsed numbers._

  ```bash
  kakao-transparency-pp-cli workbooks --since 2020 --agent
  ```

### Agent-native plumbing
- **`latest`** — Fetch the newest published half-year without knowing which period it is.

  _Use this as the first call whenever the task says 'current' or 'most recent' — it removes the guess-the-period failure mode._

  ```bash
  kakao-transparency-pp-cli latest --agent
  ```

## Recipes


### Warrant compliance trend for Kakao services

```bash
kakao-transparency-pp-cli series --service kakao --category warrant --agent --select year,halfYear,numberOfRequests,numberOfProcesses
```

One tidy JSON series of search-and-seizure warrant volumes and processing since 2012, narrowed to the four fields a chart needs.

### Newest report, narrative included

```bash
kakao-transparency-pp-cli latest --agent
```

Resolves the most recent half-year and returns its full payload including Kakao's own trend commentary.

### Mirror the official workbooks since 2020

```bash
kakao-transparency-pp-cli workbooks --since 2020 --agent
```

Lists the Korean and English XLSX download URLs per half-year for archival mirroring.

### One period as CSV

```bash
kakao-transparency-pp-cli transparency --year 2024 --half-year-id 1 --csv
```

The eight statistics rows (2 services x 4 categories) for 1H 2024 in spreadsheet-ready form.

## Usage

Run `kakao-transparency-pp-cli --help` for the full command reference and flag list.

## Commands

### transparency

Semiannual government data-request statistics (2012–present) for Kakao and Daum services.

- **`kakao-transparency-pp-cli transparency`** - Returns the transparency-report statistics for one half-year: eight
statistics rows (Kakao and Daum, each across the four government
request categories), the narrative summary, XLSX download links, and
prev/next availability flags. Published range starts at 1H 2012.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
kakao-transparency-pp-cli transparency --year 2025 --half-year-id 1

# JSON for scripting and agents
kakao-transparency-pp-cli transparency --year 2025 --half-year-id 1 --json

# Filter to specific fields
kakao-transparency-pp-cli transparency --year 2025 --half-year-id 1 --json --select id,name,status

# Dry run — show the request without sending
kakao-transparency-pp-cli transparency --year 2025 --half-year-id 1 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
kakao-transparency-pp-cli transparency --year 2025 --half-year-id 1 --agent
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
kakao-transparency-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/kakao-transparency-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **transparency returns 'response is not JSON' or an HTML page** — The year/half-year is outside the published range (before 1H 2012 or after the latest report) — run `latest` to find the newest period, or `series` to see the full range.
- **Numbers show -1 in some cells** — -1 is the API's encoding for N/A (Kakao stopped providing that data type); the site renders it as N/A. Filter it out before summing.
