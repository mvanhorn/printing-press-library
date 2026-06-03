# Perfume NZ CLI

**The full authentic Perfume NZ catalog, with powerful note-based search and offline intelligence no website offers.**

Perfume NZ (perfumenz.co.nz) is New Zealand's largest wholesale distributor of genuine designer and niche fragrances. This CLI syncs their public catalog into a local SQLite store with parsed Top/Heart/Base notes, then gives you filters, FTS, and novel features (note overlap, similarity, price-per-ml, discovery sets) that the Shopify frontend simply cannot provide.

Learn more at [Perfume NZ](https://www.perfumenz.co.nz).

Created by [@AhjinGuild12](https://github.com/AhjinGuild12) (Jan Medina).

## Install

The recommended path installs both the `perfumenz-pp-cli` binary and the `pp-perfumenz` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install perfumenz
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install perfumenz --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install perfumenz --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install perfumenz --agent claude-code
npx -y @mvanhorn/printing-press-library install perfumenz --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/perfumenz/cmd/perfumenz-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/perfumenz-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-perfumenz --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-perfumenz --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-perfumenz skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-perfumenz. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/perfumenz-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/perfumenz/cmd/perfumenz-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "perfumenz": {
      "command": "perfumenz-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify the public catalog JSON is reachable (no key needed).
perfumenz doctor --dry-run


# Pull the current ~250 authentic items (brands, prices, explicit notes) into the local store.
perfumenz sync


# Find matching perfumes with structured note filters + machine output.
perfumenz search --notes "citrus,woody" --max-price 100 --json --select title,price,vendor


# Discover similar profiles using local note overlap (the feature the website is missing).
perfumenz similar "nocturno-elixir-by-rayhaan-100ml-edp" --limit 5

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Note intelligence

- **`search --notes`** — Find perfumes matching specific Top/Heart/Base notes (with exclude).

  _Website faceted search is weak on note combinations; this is the main reason power users will install the CLI._

  ```bash
  perfumenz search --notes "vanilla,oud" --exclude "patchouli" --max-price 120 --json
  ```
- **`similar`** — List perfumes with overlapping note profiles to a given one.

  _Agents and users want 'more like this but cheaper or fresher' — the site has no equivalent._

  ```bash
  perfumenz similar "wolf-by-rayhaan-100ml-edp" --limit 8 --json
  ```

### Value & stock intelligence

- **`value`** — Rank current stock by price per ml (or per 100ml) with filters.

  _Real NZ buyers care about value; this turns the catalog into a shopping optimizer._

  ```bash
  perfumenz value --notes "citrus,woody" --gender unisex --sort ppm --json
  ```

### Catalog intelligence

- **`stats notes`** — Show the most common notes across in-stock items (overall or filtered).

  _See what accords are actually available right now from an authentic NZ source._

  ```bash
  perfumenz stats notes --gender mens --limit 15 --json
  ```

### Agent & shopping workflows

- **`recommend`** — Build a small set of perfumes that together cover requested notes under a budget.

  _The killer workflow: 'give me a 5-perfume discovery box that hits these notes without breaking the bank'._

  ```bash
  perfumenz recommend --notes "vanilla,cedar,ginger" --budget 350 --count 5 --json
  ```

## Recipes


### Fresh summer unisex under $100 with citrus top

```bash
perfumenz search --notes "lemon,grapefruit,mint" --gender unisex --max-price 100 --json
```

Combines note filters + gender + price in one structured query that the website search does not support at this precision.

### Best value woody scents right now

```bash
perfumenz value --notes "sandalwood,cedar,woody" --sort ppm --limit 10
```

Computes price-per-ml on the synced catalog and ranks — pure website cannot do this dynamically.

### Agent building a discovery box

```bash
perfumenz recommend --notes "vanilla,oud,spicy" --budget 400 --count 5 --json
```

The coverage + budget recommendation the enthusiast has always wanted; uses the local parsed notes + prices.

## Usage

Run `perfumenz-pp-cli --help` for the full command reference and flag list.

## Commands

### collections

Manage collections

- **`perfumenz-pp-cli collections`** - List collections

### products

Manage products

- **`perfumenz-pp-cli products <handle>`** - Get single product by handle

### products-json

Manage products json

- **`perfumenz-pp-cli products-json`** - List all perfumes from public feed


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
perfumenz-pp-cli collections

# JSON for scripting and agents
perfumenz-pp-cli collections --json

# Filter to specific fields
perfumenz-pp-cli collections --json --select id,name,status

# Dry run — show the request without sending
perfumenz-pp-cli collections --dry-run

# Agent mode — JSON + compact + no prompts in one flag
perfumenz-pp-cli collections --agent
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
perfumenz-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/perfumenz-public-catalog-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **sync returns very few items** — The public /products.json may be paginated or the 'all' collection limited; raise --limit or hit the products endpoint directly in a future sync.
- **notes look empty after sync** — The Fragrance Notes section lives in body_html as 'Top Notes: ... Heart Notes: ... Base Notes: ...'. The parser may need a tweak for this store's exact HTML; check a raw product in the store.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
