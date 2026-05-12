# Kickstarter CLI

**Every Kickstarter scraper, plus the first programmatic Magazine path and a Surf transport that clears Cloudflare without a paid service.**

Surfaces newly launched campaigns, trending projects, and Magazine editorial in one binary. Local SQLite mirror enables compound queries no API exposes — funding velocity, vertical classification, category-rotation signals. Built for agent ingestion, terse JSONL output, Surf-fronted to clear Cloudflare without a clearance cookie.

Learn more at [Kickstarter](https://www.kickstarter.com).

## Install

The recommended path installs both the `kickstarter-pp-cli` binary and the `pp-kickstarter` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install kickstarter
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install kickstarter --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/kickstarter/cmd/kickstarter-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/kickstarter-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-kickstarter --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-kickstarter --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-kickstarter skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-kickstarter. The skill defines how its required CLI can be installed.
```

## Authentication

No authentication required. Public Discover, project detail, and Magazine surfaces work over Surf-Chrome HTTP. The api.kickstarter.com OAuth API is partner-only and not used by this CLI.

## Quick Start

```bash
# Unified roll-up of new launches + trending + Magazine, filtered to AI vertical, in agent-ready JSONL.
kickstarter-pp-cli latest-news --vertical ai-agents --limit 20 --jsonl


# Newly launched AI projects in the last week, ranked by funding velocity.
kickstarter-pp-cli tech-radar --subcategory ai --days 7 --json


# Mirror live tech projects into the local SQLite store for compound queries.
kickstarter-pp-cli sync --category technology --status live


# Identify breakout projects before they hit the trending list.
kickstarter-pp-cli funding-rank --window 24h --category technology --limit 25


# FTS over Kickstarter's own editorial coverage.
kickstarter-pp-cli magazine search "ai" --limit 10 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agent-native aggregation
- **`latest-news`** — Fan out across Discover new-launches, Trending/Most-Funded, and Kickstarter Magazine editorial in one call — single ranked stream of fresh signal.

  _When an agent needs one call for 'what's worth knowing on Kickstarter right now,' this returns the smallest useful payload without the agent having to plan three queries._

  ```bash
  kickstarter-pp-cli latest-news --vertical ai-agents --limit 20 --jsonl
  ```
- **`magazine search`** — Full-text search over locally-mirrored Kickstarter Magazine articles (editorial posts from KS staff covering trends, creator spotlights, Maker Monday roll-ups).

  _Magazine articles are KS's own editorial signal about what they amplify. Useful when an agent needs 'editorial coverage' separate from raw project listings._

  ```bash
  kickstarter-pp-cli magazine search "ai" --limit 10 --json
  ```

### Local state that compounds
- **`tech-radar`** — Newly launched Technology/Design projects ranked by funding velocity over a time window. Filter to AI/Hardware/Software/Robots subcategories.

  _Agents tracking emerging tech want 'what's growing fastest,' not 'what's biggest right now.' Local snapshots compute the derivative._

  ```bash
  kickstarter-pp-cli tech-radar --subcategory ai --days 7 --json
  ```
- **`funding-rank`** — Rank live projects by funding velocity (% of goal raised per hour-since-launch) over a configurable window.

  _Identifies breakouts before they top the charts. Trending lists are lagging indicators; velocity is leading._

  ```bash
  kickstarter-pp-cli funding-rank --window 24h --category technology --limit 25
  ```
- **`category velocity`** — Aggregate launches-per-day and total-pledged-per-day for each top-level category over a time window. Surfaces which categories are accelerating.

  _Sector rotation signal. When tech launches double in a week, that's a market shift worth flagging upstream._

  ```bash
  kickstarter-pp-cli category velocity --window 14d --json
  ```

### Scout pipeline integration
- **`vertical`** — Score each project against a named vertical (ai-agents, frontier-ai, smb-saas, geopolitics, aus-tech, india-tech) using keyword + description + tag matching. Stored in vertical_match table for SQL composition.

  _Scout's vertical-mapping stage consumes this directly. One CLI call replaces a query + a downstream classifier._

  ```bash
  kickstarter-pp-cli vertical ai-agents --score-threshold 0.6 --since 7d --jsonl
  ```
- **`sync --since-last --diff`** — Periodic sync emits only the delta (new + changed projects) since the last successful sync, as JSONL one-per-line.

  _Lets Scout's weekly run pull only what's new without re-downloading the entire week's launch corpus._

  ```bash
  kickstarter-pp-cli sync --since-last --diff delta --json
  ```

## Usage

Run `kickstarter-pp-cli --help` for the full command reference and flag list.

## Commands

### creators

Creator (user) profile and project history

- **`kickstarter-pp-cli creators get`** - Fetch creator profile and project history

### discover

Public Kickstarter Discover JSON endpoint

- **`kickstarter-pp-cli discover list`** - Search and discover Kickstarter projects with filters, sort, and pagination

### magazine

Kickstarter Magazine editorial articles (HTML)

- **`kickstarter-pp-cli magazine get`** - Fetch a Magazine article (HTML; parsed for title, author, body, tags)
- **`kickstarter-pp-cli magazine list`** - Fetch the Magazine index page (HTML; parsed for article links)

### oembed

Lightweight oEmbed metadata for any Kickstarter project URL

- **`kickstarter-pp-cli oembed get`** - Fetch oEmbed metadata for a project URL

### projects

Individual project detail and updates

- **`kickstarter-pp-cli projects get`** - Fetch full project detail (creator, rewards, video, funding state)
- **`kickstarter-pp-cli projects updates`** - List creator-posted updates for a project


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
kickstarter-pp-cli creators --username example-resource

# JSON for scripting and agents
kickstarter-pp-cli creators --username example-resource --json

# Filter to specific fields
kickstarter-pp-cli creators --username example-resource --json --select id,name,status

# Dry run — show the request without sending
kickstarter-pp-cli creators --username example-resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
kickstarter-pp-cli creators --username example-resource --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-kickstarter -g
```

Then invoke `/pp-kickstarter <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/kickstarter/cmd/kickstarter-pp-mcp@latest
```

Then register it:

```bash
claude mcp add kickstarter kickstarter-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/kickstarter-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/kickstarter/cmd/kickstarter-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "kickstarter": {
      "command": "kickstarter-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
kickstarter-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/kickstarter-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Empty results from `latest-news` or any Discover command** — Re-run with `--verbose` to confirm the Surf-Chrome request returned 200. If status is 403, Cloudflare protection has changed shape; run `printing-press probe-reachability https://www.kickstarter.com/` to confirm runtime mode.
- **Funding-velocity is always 0** — Funding velocity requires at least two synced snapshots. Run `kickstarter-pp-cli sync` at least twice with a gap before invoking `funding-rank` or `tech-radar`.
- **Magazine commands return empty** — Magazine has no JSON endpoint; HTML parser depends on KS's current Magazine template. If they restructure /magazine, re-run with `--debug-html` and file an issue.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.kickstarter.com/
- Capture coverage: 2 API entries from 2 total network entries
- Reachability: browser_http (85% confidence)
- Protocols: rest-json (90% confidence), html-scrape (80% confidence)
- Protection signals: cloudflare-challenge (95% confidence)
- Generation hints: Ship Surf with Chrome TLS fingerprint as the default HTTP transport (no chrome cookie import needed; no browser sidecar at runtime), Magazine endpoints are HTML-only; emit response_format: html parsers, not JSON unmarshal, category_id values: 1=art, 3=comics, 6=dance, 7=design, 9=fashion, 10=food, 11=film-video, 12=games, 13=journalism, 14=music, 15=photography, 16=technology, 17=theater, 18=publishing, 26=crafts, Discover JSON returns a 'projects' array with name, slug, blurb, goal, pledged, backers_count, state, deadline, launched_at, creator (id+name+slug), category (id+name+slug), location

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**markolson/kickscraper**](https://github.com/markolson/kickscraper) — Ruby (280 stars)
- [**rabidlogic/PyKickstarter**](https://github.com/rabidlogic/PyKickstarter) — Python (50 stars)
- [**mikeminutillo/Kickstarter.Api**](https://github.com/mikeminutillo/Kickstarter.Api) — C# (30 stars)
- [**gippy/kickstarter-search**](https://github.com/gippy/kickstarter-search) — JavaScript (15 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
