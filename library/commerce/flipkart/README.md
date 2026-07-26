# Flipkart CLI

**Every Flipkart scraper's features, plus a local price-history and deal-tracking layer no single-shot script can offer.**

Search, product detail, and the official Affiliate API in one consistent schema — with local SQLite turning every fetch into price history, watchlist digests, and cross-product deal arbitrage.

Learn more at [Flipkart](https://www.flipkart.com).

Printed by [@urbanlotusai](https://github.com/urbanlotusai) (Rajiv Lokare).

## Install

The recommended path installs both the `flipkart-pp-cli` binary and the `pp-flipkart` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install flipkart
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install flipkart --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install flipkart --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install flipkart --agent claude-code
npx -y @mvanhorn/printing-press install flipkart --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/shopping/flipkart/cmd/flipkart-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/flipkart-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-flipkart --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-flipkart --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-flipkart skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-flipkart. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/flipkart-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/shopping/flipkart/cmd/flipkart-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "flipkart": {
      "command": "flipkart-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No login required for search and product lookup — the CLI replays browser-sniffed direct-HTTP requests against the public site. Users with an approved Flipkart Affiliate account can additionally set FLIPKART_AFFILIATE_ID and FLIPKART_AFFILIATE_TOKEN to unlock the official category/delta feed endpoints.

## Quick Start

```bash
# Start with a keyword search — no auth needed.
flipkart-pp-cli catalog "wireless earbuds" --page 1


# Pull full detail on one result; this also records a price-history snapshot.
flipkart-pp-cli product get <product-url>


# Track a product so future syncs build a price history.
flipkart-pp-cli watch add <product-url> --threshold 1999


# Refresh watched products and update local price history.
flipkart-pp-cli sync


# See everything that changed across your watchlist since last check.
flipkart-pp-cli watch digest

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`watch digest`** — See everything that changed across your whole watchlist since you last checked — price drops, new offers, stock changes — in one command.

  _Reach for this instead of re-checking N product pages by hand — it answers 'what changed' in one call._

  ```bash
  flipkart-pp-cli watch digest --json
  ```
- **`compare`** — Line up 2 or more products side by side — price, rating, discount, key specs — in one table instead of copy-pasting across tabs.

  _Use when deciding between a shortlist of similar products rather than fetching each individually._

  ```bash
  flipkart-pp-cli compare "https://www.flipkart.com/oppo-a3x-8-gb-ram-128-gb-storage-lake-green/p/itmc050065f36601?pid=MOBH4ZCFEQZFQBFB" "https://www.flipkart.com/samsung-galaxy-m06-5g-mint-green-128-gb/p/itmc0501a8c1f5e5?pid=MOBH8QSVDVBRZFNJ" --json
  ```
- **`catalog diff`** — Re-run a saved search and see exactly what's new, removed, or price-changed since the last time you ran it.

  _Use to track a category of interest over time without hand-comparing result pages._

  ```bash
  flipkart-pp-cli catalog diff "wireless earbuds" --json
  ```
- **`deals category`** — Scan an entire category for products above a discount threshold, and keep the results queryable offline afterward.

  _Use during sale windows to shortlist deals once, then re-query them offline without re-hitting the API._

  ```bash
  flipkart-pp-cli deals category electronics --min-discount 40 --json
  ```
- **`offers best-card`** — Across a set of products you're considering, find the single bank card that maximizes your total stacked savings.

  _Use before checkout when weighing which of your cards to use across a multi-item cart._

  ```bash
  flipkart-pp-cli offers best-card "https://www.flipkart.com/oppo-a3x-8-gb-ram-128-gb-storage-lake-green/p/itmc050065f36601?pid=MOBH4ZCFEQZFQBFB" "https://www.flipkart.com/samsung-galaxy-m06-5g-mint-green-128-gb/p/itmc0501a8c1f5e5?pid=MOBH8QSVDVBRZFNJ" --json
  ```

### Agent-native plumbing
- **`feed digest`** — Pull the official Delta Feed API without manually tracking the last fromVersion — the CLI remembers it — and see deltas ranked by discount-percentage change.

  _Use for a weekly affiliate-catalog refresh instead of hand-tracking version numbers across runs._

  ```bash
  flipkart-pp-cli feed digest electronics --json
  ```

## Usage

Run `flipkart-pp-cli --help` for the full command reference and flag list.

## Commands

### catalog

Search Flipkart's live product catalog by keyword.

- **`flipkart-pp-cli catalog <q>`** - Search Flipkart products by keyword.

### feed

Official Flipkart Affiliate API — category and delta product feeds (requires an approved affiliate account).

- **`flipkart-pp-cli feed category`** - Fetch the product feed for one category via the official Affiliate API.
- **`flipkart-pp-cli feed delta`** - Fetch only products changed since a given feed version via the official Affiliate API.
- **`flipkart-pp-cli feed product`** - Look up a single product by Flipkart product ID via the Affiliate API.
- **`flipkart-pp-cli feed search`** - Search the Affiliate API catalog by keyword.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
flipkart-pp-cli catalog mock-value

# JSON for scripting and agents
flipkart-pp-cli catalog mock-value --json

# Filter to specific fields
flipkart-pp-cli catalog mock-value --json --select id,name,status

# Dry run — show the request without sending
flipkart-pp-cli catalog mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
flipkart-pp-cli catalog mock-value --agent
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
flipkart-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **search or product get returns an HTTP 403 or empty result** — Flipkart's anti-bot detection may be triggering; run 'flipkart-pp-cli doctor' to check transport health, and retry after a short delay.
- **feed/delta feed commands fail with an auth error** — Set FLIPKART_AFFILIATE_ID and FLIPKART_AFFILIATE_TOKEN — these require an approved Flipkart Affiliate account, separate from normal search/product commands.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Known Gaps

- **`sync` prints a per-resource error for `feed` when Affiliate credentials aren't set.** The `feed` resource (official Affiliate API) is optional and requires `FLIPKART_AFFILIATE_ID`/`FLIPKART_AFFILIATE_TOKEN`. Without them, `sync` still attempts the fetch, gets an HTTP 401, and logs a `sync_error` line for that resource — but `sync` still exits `0` and treats it as a non-critical warning (`exit_policy_default_changed`), so scripts checking the exit code are unaffected. This was not fixable in this session without hand-editing generated code; a future regeneration of the Printing Press generator could pre-check tier auth before attempting the fetch.
- **`feed digest` and `deals category` require an approved Flipkart Affiliate account** and could not be tested against live, credentialed traffic in this session (no affiliate account was available). Both commands correctly return an actionable error naming the two required env vars when credentials are absent.
- **MRP / discount % / bank-card offers are extracted from page text, not a stable JSON API** — Flipkart doesn't expose these fields in structured JSON-LD. If Flipkart changes its page layout, `product offers` may need its extraction patterns updated.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**flipkart-scraper (dvishal485, Rust)**](https://github.com/dvishal485/flipkart-scraper) — Rust
- [**flipkart-scraper (mdvenukumar)**](https://github.com/mdvenukumar/flipkart-scraper) — Python
- [**flipkart-scraper (atharao)**](https://github.com/atharao/flipkart-scraper) — Python
- [**Web_Scraping_Flipkart_Product_Data**](https://github.com/Sunil-nith/Web_Scraping_Flipkart_Product_Data) — Python
- [**Flipkart-Price-Tracker (muhammedashharps)**](https://github.com/muhammedashharps/Flipkart-Price-Tracker) — Python
- [**PriceTrackerBot (nuhmanpk)**](https://github.com/nuhmanpk/PriceTrackerBot) — Python
- [**flipkart-affiliate-client**](https://www.npmjs.com/package/flipkart-affiliate-client) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
