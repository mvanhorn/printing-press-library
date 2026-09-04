# Creative Fabrica CLI

**Search Creative Fabrica's 20M+ font, graphic, craft, and POD catalog from your terminal — with file-format and commercial-license filters the web UI lacks, plus designer tracking.**

Creative Fabrica's web UI is heavy, upsell-laden, and gives you only the facets it wants to. This CLI talks straight to the same catalog index the site uses and adds the filters crafters and print-on-demand sellers actually need: file format (svg/dxf/png/eps), commercial-license (POD), subscription-free, and real discount depth. It also tracks designers and surfaces what is new since your last run — all agent-native with `--json`, `--csv`, and `--select`.

## Why this exists

Creative Fabrica has **no official API**. Its site search is powered by a public Algolia index, and this CLI queries that same index directly over plain HTTPS — no browser, no scraping, no Cloudflare at runtime. That unlocks things the web UI simply does not offer:

- **File-format filtering.** Creative Fabrica has no "SVG only" facet. `find --format svg,dxf` matches the format in tags and titles, so Cricut/Silhouette crafters stop wading through subscription-locked previews of the wrong format.
- **Commercial-license sourcing.** Print-on-demand sellers can filter to `--pod` (commercial use) and `--no-subscription` (usable without an active plan) and export candidates to CSV.
- **Real discount depth.** `deals` ranks by the actual regular→sale percentage, cutting through discount theater.
- **Designer intelligence.** `designer-stats` and `designer-compare` profile a creator's whole catalog; `new-since` reports only what's new since your last run.

**Who it's for:** Cricut/Silhouette crafters hunting cut files, print-on-demand sellers sourcing license-safe assets, designers and budget crafters chasing genuine free/discounted items, and AI agents that need structured catalog data without parsing a heavy web page.

## Install

The recommended path installs both the `creativefabrica-pp-cli` binary and the `pp-creativefabrica` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install creativefabrica
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install creativefabrica --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install creativefabrica --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install creativefabrica --agent claude-code
npx -y @mvanhorn/printing-press-library install creativefabrica --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/cmd/creativefabrica-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/creativefabrica-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install creativefabrica --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-creativefabrica --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-creativefabrica --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install creativefabrica --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/creativefabrica-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/cmd/creativefabrica-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "creativefabrica": {
      "command": "creativefabrica-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirm the CLI is wired and the catalog index is reachable.
creativefabrica-pp-cli doctor --dry-run

# Search the catalog the way the website does.
creativefabrica-pp-cli find "watercolor flowers" --limit 10

# Find commercial-license SVG cut files — a filter the web UI can't express.
creativefabrica-pp-cli find "mandala" --format svg --pod --agent

# List the newest genuinely-free fonts.
creativefabrica-pp-cli free --type Fonts --limit 20

# Profile a designer's whole catalog at a glance.
creativefabrica-pp-cli designer-stats "DigiArt"

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Filters the web UI doesn't have
- **`find`** — Narrow craft results to a specific cut/print file format (SVG, DXF, PNG, EPS, PES) that Creative Fabrica has no facet for.

  _Reach for this when a crafter needs a machine-ready format the web UI can't filter on._

  ```bash
  creativefabrica-pp-cli find "mandala" --format svg,dxf --agent
  ```
- **`find`** — Keep only assets usable outside an active subscription.

  _Use this when you need standalone-licensable assets, distinct from free or POD._

  ```bash
  creativefabrica-pp-cli find "valentine svg" --no-subscription --agent
  ```

### Local synthesis
- **`deals`** — Rank on-sale items by their actual regular-to-sale price drop, not just whether they are flagged on sale.

  _Use this to surface the deepest genuine discounts instead of discount-theater._

  ```bash
  creativefabrica-pp-cli deals "font bundle" --agent
  ```
- **`designer-stats`** — Profile a designer's catalog: product-type mix, price band, free count, POD count, and newest release.

  _Pick this when you need a one-shot read on a creator instead of scrolling their page._

  ```bash
  creativefabrica-pp-cli designer-stats "DigiArt" --agent
  ```
- **`designer-compare`** — Compare two designers head-to-head: catalog size, type mix, price band, free/POD share, and popularity.

  _Use this for competitor research when sourcing or scouting creators._

  ```bash
  creativefabrica-pp-cli designer-compare "DigiArt" "CraftLab" --agent
  ```

### Local state that compounds
- **`new-since`** — Show only catalog items added since your last sync for a tracked query or designer.

  _Reach for this to track fresh drops without re-scanning the whole catalog._

  ```bash
  creativefabrica-pp-cli new-since "watercolor flowers" --agent
  ```
- **`tags`** — Show the top tags and categories that co-occur with a query, to refine the next search.

  _Use this to discover better refinement terms before a deep search._

  ```bash
  creativefabrica-pp-cli tags "christmas" --agent
  ```

## Recipes


### Commercial-license cut files

```bash
creativefabrica-pp-cli find "floral monogram" --format svg --pod --limit 25 --csv
```

POD-safe SVG candidates exported as CSV for a sourcing sheet.

### Newest free fonts

```bash
creativefabrica-pp-cli free --type Fonts --sort newest --limit 30 --agent
```

The latest genuinely-free fonts in machine-readable form.

### Deepest real discounts

```bash
creativefabrica-pp-cli deals "bundle" --agent --select name,price,regular_price,discount_pct
```

On-sale items ranked by actual discount depth, narrowed to the price fields.

### Track a designer's drops

```bash
creativefabrica-pp-cli new-since --designer 2880714 --agent
```

Only the items this designer added since your last sync.

### Refine a broad search

```bash
creativefabrica-pp-cli tags "christmas" --select tag,count --agent
```

Top co-occurring tags to sharpen a vague query before a deep search.

## Workflows

### Print-on-demand sourcing
Build a license-safe candidate list for a product idea, narrow it to a format and price, and export for a sourcing sheet:

```bash
# 1. See what tags a vague idea maps to
creativefabrica-pp-cli tags "summer t-shirt" --agent
# 2. Source POD-safe SVGs under $3, newest first, as CSV
creativefabrica-pp-cli find "summer beach" --pod --no-subscription --format svg --max-price 3 --sort newest --csv > candidates.csv
# 3. Vet a promising designer before buying in bulk
creativefabrica-pp-cli designer-stats "DigiArt" --agent
```

### Freebie hunting on a schedule
Track new free assets in a category without re-scanning the whole catalog:

```bash
# First run seeds the tracker; later runs report only what's new
creativefabrica-pp-cli new-since "watercolor florals" --agent
creativefabrica-pp-cli new-since --designer 2880714 --agent
# Browse today's newest free fonts
creativefabrica-pp-cli free --type Fonts --limit 30
```

### Deal scouting
Find the deepest genuine discounts across a theme and compare two designers' pricing:

```bash
creativefabrica-pp-cli deals "font bundle" --agent --select name,price,regular_price,discount_pct
creativefabrica-pp-cli designer-compare "DigiArt" "CraftLab" --agent
```

## Usage

Run `creativefabrica-pp-cli --help` for the full command reference and flag list.

## Commands

### Search & browse
- **`find [query]`** — Live catalog search with filters: `--type`, `--category`, `--designer`, `--format svg,dxf,...`, `--pod`, `--free`, `--no-subscription`, `--on-sale`, `--max-price`, `--sort relevance|newest`, `--page`, `--limit`.
- **`free [query]`** — Free assets, newest first (`--type`, `--category`).
- **`pod [query]`** — Print-on-demand / commercial-license assets (`--max-price`, `--free`).
- **`designer <id|name>`** — Browse one designer's catalog.
- **`product <objectID>`** — A single product's catalog metadata.

### Browse facets
- **`categories [query]`** — All 724 categories with counts (`--limit`).
- **`types [query]`** — Product types (Graphics, Fonts, Crafts, ...) with counts.

### Insight (only possible with the local store)
- **`deals [query]`** — On-sale items ranked by true regular→sale discount depth.
- **`designer-stats <id|name>`** — Aggregate a designer's catalog: type mix, price band, free/POD counts, newest.
- **`designer-compare <A> <B>`** — Two designers side-by-side.
- **`new-since [query|--designer <id>]`** — Only what the catalog added since your last run.
- **`tags <query>`** — Top tags/categories co-occurring with a query, to refine it.

### Setup & health
- **`auth set-key <key>`** / **`auth status`** — Configure the public catalog search key.
- **`doctor`** — Verify the CLI is wired and the catalog index is reachable.

The low-level `products` command is an Algolia multi-query passthrough; prefer the commands above.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
creativefabrica-pp-cli find "watercolor flowers"

# JSON for scripting and agents
creativefabrica-pp-cli find "watercolor flowers" --json

# Filter to specific fields (use the JSON field names: name, price, designer, url, ...)
creativefabrica-pp-cli find "logo" --json --select name,price,designer,url

# CSV for spreadsheets / sourcing sheets
creativefabrica-pp-cli pod "t-shirt" --csv

# Dry run — show the request without sending
creativefabrica-pp-cli find "x" --dry-run

# Agent mode — JSON + compact + no prompts in one flag
creativefabrica-pp-cli find "x" --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
creativefabrica-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/creativefabrica-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Search returns zero results for everything** — Run 'creativefabrica-pp-cli doctor' — the public search key may have rotated; the CLI re-discovers it automatically, or set CREATIVEFABRICA_ALGOLIA_API_KEY / CREATIVEFABRICA_ALGOLIA_APP_ID to override.
- **HTTP 403 from the search backend** — The catalog key is referer-restricted; the CLI sends the required Origin/Referer headers automatically — upgrade if you see this on an old build.
- **--format returns nothing** — Format is matched against tags and titles, not a server facet; broaden the query or try a sibling format (png vs svg).
