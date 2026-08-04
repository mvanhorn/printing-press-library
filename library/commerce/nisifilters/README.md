# NiSi Italia CLI

**A read-only, offline-first mirror and search engine for the public NiSi Italia filter catalog and content.**

Sync every public product, category, attribute, post, page, and media item into a local SQLite database, then search and browse it offline with agent-clean output. Includes a filter-finder (filters) that surfaces available sizes/types, an enriched priced shop view (shop), a clean content reader (read), and a catalog digest (digest) — all without authentication.

## Install

The recommended path installs both the `nisifilters-pp-cli` binary and the `pp-nisifilters` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install nisifilters
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install nisifilters --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install nisifilters --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install nisifilters --agent claude-code
npx -y @mvanhorn/printing-press-library install nisifilters --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/nisifilters/cmd/nisifilters-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nisifilters-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install nisifilters --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-nisifilters --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-nisifilters --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install nisifilters --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nisifilters-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/nisifilters/cmd/nisifilters-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "nisifilters": {
      "command": "nisifilters-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirm the store is reachable before syncing
nisifilters-pp-cli doctor --dry-run

# Mirror the catalog and content into the local SQLite store
nisifilters-pp-cli sync

# Offline full-text search across products and content
nisifilters-pp-cli search "polarizzatore"

# Scan the catalog by price with categories and stock
nisifilters-pp-cli shop --sort price --json

# One-shot catalog and site overview
nisifilters-pp-cli digest

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local joins that compound
- **`shop`** — List filters joined with their categories — name, price, stock (in / backorder / out) — sorted by price.

  _Use to scan the filter catalog with prices and categories in one view._

  ```bash
  nisifilters-pp-cli shop --sort price --json
  ```
- **`variants`** — List a variable product's WooCommerce variations — variation id, attributes, price, stock — invisible via wp-json/wp/v2.

  _Use to get the exact variation_id and stock for a size (e.g. 82mm vs 95mm adapter ring) before building cart links._

  ```bash
  nisifilters-pp-cli variants 278495 --json
  ```
- **`filters`** — Discover available filter sizes/types from product attributes and find matching products.

  _Reach for this to answer 'which sizes exist' or 'which products are 82mm' without clicking the storefront._

  ```bash
  nisifilters-pp-cli filters --json
  ```
- **`digest`** — Summarize the mirror: product count, price range, in-stock vs out, top categories, and content counts.

  _Use for a one-shot overview of the catalog and site without paging every collection._

  ```bash
  nisifilters-pp-cli digest --json
  ```

### Agent-native plumbing
- **`read`** — Print one post, page, or product as title plus plain-text content, stripping the WordPress envelope.

  _Use to read Academy guides without burning context on WordPress envelope fields._

  ```bash
  nisifilters-pp-cli read 289989
  ```
- **`since`** — List posts and pages modified within a time window, across those types at once.

  _Use to answer 'what is new on the site' without checking each collection separately._

  ```bash
  nisifilters-pp-cli since 30d --json
  ```
- **`image`** — Resolve the featured image of a post or page, or a product's inline images, to full-resolution URLs.

  _Use when you have a content item or product and need its actual image URL._

  ```bash
  nisifilters-pp-cli image 289989 --type posts --json
  ```

## Recipes

### Mirror then search offline

```bash
nisifilters-pp-cli sync && nisifilters-pp-cli search "ND1000"
```

Sync once, then full-text search the catalog and content with no further network calls.

### Cheapest filters first

```bash
nisifilters-pp-cli shop --sort price --limit 10 --json
```

List the ten lowest-priced products joined with their categories and stock.

### Pick the right size variant before ordering

```bash
nisifilters-pp-cli variants 278495 --json
```

Variable products hide their sizes behind one parent id; this lists every variation with its own id, price, and stock — including backorder items that are still orderable.

### Narrow a verbose product to a few fields

```bash
nisifilters-pp-cli products list --per-page 5 --agent --select id,name,prices.price,categories.name
```

WooCommerce products are large; --select with dotted paths returns only the fields you need.

### Discover available filter sizes

```bash
nisifilters-pp-cli filters --json
```

Surface the product attributes (e.g. Dimensione) and their values from the local mirror.

### What changed this month

```bash
nisifilters-pp-cli since 30d
```

Show posts and pages modified in the last 30 days.

## Usage

Run `nisifilters-pp-cli --help` for the full command reference and flag list.

## Commands

### authors

Public authors

- **`nisifilters-pp-cli authors get`** - Get a single author by numeric ID
- **`nisifilters-pp-cli authors list`** - List public authors

### categories

Post categories

- **`nisifilters-pp-cli categories get`** - Get a single category by numeric ID
- **`nisifilters-pp-cli categories list`** - List post categories

### comments

Public comments

- **`nisifilters-pp-cli comments get`** - Get a single comment by numeric ID
- **`nisifilters-pp-cli comments list`** - List approved public comments

### find

Live global content search across the whole site (wp/v2 /search)

- **`nisifilters-pp-cli find`** - Search all public content live

### media

Media library (images / attachments)

- **`nisifilters-pp-cli media get`** - Get a single media item by numeric ID (resolves source_url)
- **`nisifilters-pp-cli media list`** - List media library items

### pages

Public site pages

- **`nisifilters-pp-cli pages get`** - Get a single page by numeric ID
- **`nisifilters-pp-cli pages list`** - List published pages

### posts

Public blog / Academy posts

- **`nisifilters-pp-cli posts get`** - Get a single post by numeric ID
- **`nisifilters-pp-cli posts list`** - List published posts

### product_categories

WooCommerce product categories — public Store API

- **`nisifilters-pp-cli product-categories`** - List product categories

### products

WooCommerce shop products (NiSi filters, holders, kits) — public Store API

- **`nisifilters-pp-cli products get`** - Get a single product by numeric ID
- **`nisifilters-pp-cli products list`** - List shop products

### tags

Post tags

- **`nisifilters-pp-cli tags get`** - Get a single tag by numeric ID
- **`nisifilters-pp-cli tags list`** - List post tags


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
nisifilters-pp-cli authors list

# JSON for scripting and agents
nisifilters-pp-cli authors list --json

# Filter to specific fields
nisifilters-pp-cli authors list --json --select id,name,status

# Dry run — show the request without sending
nisifilters-pp-cli authors list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
nisifilters-pp-cli authors list --agent
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
nisifilters-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/nisifilters-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **search or shop returns nothing** — Run 'nisifilters-pp-cli sync' first; these read the local mirror, not the live site.
- **image shows no URL for a post** — Sync media too: 'nisifilters-pp-cli sync' mirrors media so featured images resolve.
- **filters shows no sizes** — Sync products and product attributes first; the finder reads the local mirror.
