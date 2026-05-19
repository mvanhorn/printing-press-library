# Trend Hunter CLI

**Every Trend Hunter feature, plus a local SQLite corpus, FTS5 search, FAQ Q&A extraction, and agent-native JSON no other Trend Hunter tool offers.**

trendhunter-pp-cli scrapes the free public TrendHunter surface (RSS feed, sitemap, category and trend pages, search results) into a local SQLite store, then exposes corpus-diff, rising-keyword clusters, time-windowed author velocity, FAQ Q&A extraction, and a category-watch feed the site itself can't provide. No API key. No login. Stdlib HTTP transport - no headless browser.

Learn more at [Trend Hunter](https://www.trendhunter.com).

Printed by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).

## Install

The recommended path installs both the `trendhunter-pp-cli` binary and the `pp-trendhunter` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install trendhunter
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install trendhunter --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/trendhunter/cmd/trendhunter-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/trendhunter-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-trendhunter --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-trendhunter --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-trendhunter skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-trendhunter. The skill defines how its required CLI can be installed.
```

## Quick Start

```bash
# Pull the latest 30 trends + sitemap and write them to the local store.
trendhunter-pp-cli sync


# Search the local corpus first, with a flag to fan-out live.
trendhunter-pp-cli search "AI clone" --json


# Week-over-week digest with new vs repeat split.
trendhunter-pp-cli digest --since 7d --category eco --json


# Pull the FAQ JSON-LD from any /trends/<slug> page.
trendhunter-pp-cli faq ai-clone --json


# One-shot agent brief: ranked trends with FAQ Q&A.
trendhunter-pp-cli brief --category ai --top 10 --format markdown

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`digest`** — Generate a week-over-week digest of new vs repeat trends in a category with top rising keywords.

  _Agents pulling a weekly competitive scan can ask 'what's new in eco this week' without re-clicking 30 cards by hand._

  ```bash
  trendhunter-pp-cli digest --since 7d --category eco --json
  ```
- **`watch`** — Per-category new-only feed - fills the gap that TrendHunter's RSS is global-only.

  _Agents and indie scouts get a clean delta for one vertical without re-processing the global firehose._

  ```bash
  trendhunter-pp-cli watch --category gadgets --since 24h --json
  ```
- **`cluster`** — Surface rising keyword clusters via FTS5 co-occurrence with a rising-vs-prior-window delta.

  _Agents researching an emerging theme see which adjacent keywords are accelerating, not just the headline trends._

  ```bash
  trendhunter-pp-cli cluster --window 30d --min-count 3 --json
  ```
- **`authors`** — Rank TrendHunter authors and futurists by time-windowed publish rate, not lifetime count.

  _Find which trend hunters are currently active in your space, not just who has the biggest historical footprint._

  ```bash
  trendhunter-pp-cli authors --top 20 --since 30d --json
  ```
- **`inbox`** — Show trends seen since the last invocation of `inbox` - a per-machine cursor table.

  _Build it into a Monday-morning routine and only ever see things you haven't seen before._

  ```bash
  trendhunter-pp-cli inbox --json
  ```

### Agent-native extraction
- **`faq`** — Extract the FAQPage JSON-LD from a trend page into structured Q&A.

  _Agents get a ready-to-quote Q&A summary of each trend without prompting another LLM to summarize the article._

  ```bash
  trendhunter-pp-cli faq ai-clone --json
  ```
- **`brief`** — One-shot agent brief: top-N ranked trends in a category, each with FAQ Q&A and keywords, rendered as JSON or markdown.

  _Plugs straight into an agent prompt as ground truth for a market memo, no glue code needed._

  ```bash
  trendhunter-pp-cli brief --category ai --top 10 --format markdown
  ```
- **`scout`** — Pull top trends in a category, score each by relevance to a business profile, optionally route the scoring through a local LLM.

  _Agents researching a vertical get a ranked, business-scoped trend list ready to pipe into the next stage of their pipeline._

  ```bash
  trendhunter-pp-cli scout --category kitchen --business "We sell smart ovens for home cooks" --top 10 --llm
  ```

### Graph and synthesis
- **`megatrend-map`** — Walk the related-trend graph two levels deep from a starting slug, returning depth-1 and depth-2 related slugs so you can see which free trends cluster around a given concept.

  _Evaluators deciding whether a paid megatrend report is worth $295 can see which free trends ladder up to it._

  ```bash
  trendhunter-pp-cli megatrend-map ai-clone --json
  ```

## Usage

Run `trendhunter-pp-cli --help` for the full command reference and flag list.

## Commands

### category

Category index pages (tech, fashion, food, eco, ai, marketing, design, ...)

- **`trendhunter-pp-cli category list`** - Fetch a TrendHunter category index page (~30 trend cards). The CLI's 'category <name>' command parses this.

### megatrends

Megatrend / pattern index pages.

- **`trendhunter-pp-cli megatrends list`** - Fetch the megatrend index page.

### popular

Curated 'popular right now' index.

- **`trendhunter-pp-cli popular get`** - Fetch the popular-trends index page (~30 cards). Run 'popular' to get parsed JSON.

### reports

Trend report index.

- **`trendhunter-pp-cli reports list`** - Fetch the trend-reports landing page (titles, descriptions, paid PDF links).

### results

Site search-results endpoint (the framework already provides a hand-built `search` command; this exposes the raw HTML page).

- **`trendhunter-pp-cli results query`** - Run a TrendHunter search and get the result-page HTML. The CLI's 'search <term>' parses this; pass --raw to get HTML.

### rss

Global RSS feed of the 30 latest trends across the whole site.

- **`trendhunter-pp-cli rss latest`** - Fetch the global RSS 2.0 feed (~30 newest trends). Use 'sync' to land these in the local store; this raw endpoint is for one-off inspection.

### scoreboard

Top contributors and trends scoreboard.

- **`trendhunter-pp-cli scoreboard get`** - Fetch the scoreboard page with top contributor handles and recent trends. Run 'scoreboard' to get parsed JSON.

### sitemap

Master sitemap with 430+ URLs (categories, futurists, contributors, hubs).

- **`trendhunter-pp-cli sitemap get`** - Fetch the sitemap.xml. The CLI's 'catalog refresh' uses this; users should use that instead.

### trends

Individual trend pages with full metadata, FAQ Q&A, related trends, author, category.

- **`trendhunter-pp-cli trends get`** - Fetch the full trend page HTML for a slug. The CLI's 'trend show <slug>' parses this; users should use that instead of calling this directly.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
trendhunter-pp-cli category mock-value

# JSON for scripting and agents
trendhunter-pp-cli category mock-value --json

# Filter to specific fields
trendhunter-pp-cli category mock-value --json --select id,name,status

# Dry run — show the request without sending
trendhunter-pp-cli category mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
trendhunter-pp-cli category mock-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-trendhunter -g
```

Then invoke `/pp-trendhunter <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/trendhunter/cmd/trendhunter-pp-mcp@latest
```

Then register it:

```bash
claude mcp add trendhunter trendhunter-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/trendhunter-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/trendhunter/cmd/trendhunter-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "trendhunter": {
      "command": "trendhunter-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
trendhunter-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/trendhunter-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Search returns no results after install** — Run `trendhunter-pp-cli sync` first - search is local-first and needs a corpus.
- **HTTP 403 on a fetch** — Update the CLI; the default Chrome UA may be too old for the site's filter. `go install ...@latest` and try again.
- **Trend page redirects to a vertical microsite (cleanthesky.com etc)** — The CLI follows redirects; the canonical /trends/<slug> URL still resolves and is what the local store records.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.trendhunter.com/
- Capture coverage: 11 API entries from 30 total network entries
- Reachability: standard_http (95% confidence)
- Protocols: html (100% confidence), rss (100% confidence)
- Generation hints: Use stdlib HTTP transport (http_transport: standard)., Default UA must imitate Chrome - curl default UA is blocked., Default Accept header must be Chrome-style 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8' - bare '*/*' is blocked., RSS feed is global only. Per-category RSS returns valid-but-empty channels; do not call them., Trend pages occasionally redirect to vertical microsites (cleanthesky.com, etc.). Follow redirects; record the canonical /trends/<slug> URL., FAQPage JSON-LD on /trends/<slug> is the highest-density agent-summary surface; extract with the faq command., No clearance cookie, no Surf, no browser sidecar.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
