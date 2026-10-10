# Fooda CLI

**Your Fooda office lunch history, subsidy and this week's popups in a terminal, with vendor and spend analytics the app does not offer.**

Fooda has no public API. This CLI reads the logged-in web app with your Chrome session, syncs your order history locally, and answers questions like what you ate, where you keep ordering, and how much subsidy you have left today.

## Install

The recommended path installs both the `fooda-pp-cli` binary and the `pp-fooda` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install fooda
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install fooda --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install fooda --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install fooda --agent claude-code
npx -y @mvanhorn/printing-press-library install fooda --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/fooda/cmd/fooda-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fooda-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine fooda-pp-cli`. On Unix, mark it executable: `chmod +x fooda-pp-cli`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install fooda --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-fooda --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-fooda --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install fooda --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
fooda-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fooda-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/fooda/cmd/fooda-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "fooda": {
      "command": "fooda-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Fooda sits behind Cloudflare and has no API key. Log in to app.fooda.com in Chrome, then run `auth login --chrome` to import your session cookies, or use `auth login --cookies-file <file>` to import cookies from a JSON file. The CLI never logs in itself.

## Quick Start

```bash
# Check the install without needing a session
fooda-pp-cli doctor --dry-run

# See how much of today's subsidy is left
fooda-pp-cli subsidy-status

# See what is being served this week
fooda-pp-cli week-ahead

# Pull your order history into the local store
fooda-pp-cli sync

# See which vendors you order from most
fooda-pp-cli venue-rotation --since 120d

```

## Recipes

### Budget-optimized lunch planning

```bash
fooda-pp-cli order plan --event S609348 --budget 20 --anchor "Beef Bulgogi Bowl"
```

Suggests a budget-optimized add-on item from the event menu.

### Vendor fatigue check

```bash
fooda-pp-cli venue-rotation --since 120d --agent --select vendor,order_count,last_ordered
```

Shows only the fields needed to see repeat vendors.

### Monthly subsidy report

```bash
fooda-pp-cli spend-trends --since 6mo --group-by month
```

Splits covered and out-of-pocket spend per month.

### Vegan options today

```bash
fooda-pp-cli menu-search vegan
```

Searches every event menu for the term.

## Limitations & Anti-Triggers

- **Ordering Support**: Ordering is supported via `order add` and `order place` when using `--confirm`. Automated agents must never pass `--confirm` without explicit user intent.
- **Cancellation Limitation**: The `order cancel` command is not supported (please cancel manually in the Fooda app).
- **`whoami` Cloudflare Constraints**: The `whoami` command is heavily limited by Cloudflare challenges. It fallbacks to greeting pages; account and building IDs are extracted from the home page.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local history that compounds
- **`served-history`** — See every lunch you were actually served, with date, vendor, items and price.

  _Use when an agent needs a longitudinal record of what a person ate rather than today's menu._

  ```bash
  fooda-pp-cli served-history --since 90d --agent
  ```
- **`venue-rotation`** — Rank vendors by how often and how recently you ordered from them.

  _Use to spot vendor fatigue or under-used favorites before choosing lunch._

  ```bash
  fooda-pp-cli venue-rotation --since 120d --agent
  ```
- **`spend-trends`** — Spend per month or week, split into subsidy-covered and out-of-pocket.

  _Use for budget or subsidy-utilization reporting._

  ```bash
  fooda-pp-cli spend-trends --since 6mo --group-by month --agent
  ```

### Lunch planning
- **`week-ahead`** — One-line-per-day digest of upcoming events and restaurants at your building.

  _Use for a quick morning check of what is being served this week._

  ```bash
  fooda-pp-cli week-ahead --agent
  ```
- **`subsidy-status`** — Remaining dollars of today's subsidy and when it is valid.

  _Use before ordering to know how much of the meal is covered._

  ```bash
  fooda-pp-cli subsidy-status --agent
  ```
- **`menu-search`** — Search menu items across all of today's events.

  _Use for dietary or craving lookups without opening each vendor page._

  ```bash
  fooda-pp-cli menu-search vegan --agent
  ```
