# 1688 CLI

**The free, offline, agent-native 1688 wholesale sourcing CLI — keyword search with the signals the web UI buries (回头率 reorder rate, factory-verified status, supplier trade scores, price drift), persisted to a local store no scraper tracks over time.**

Search 1688.com's China-domestic wholesale catalog and get structured offers (tiered price, MOQ, supplier, region, transaction volume) for free, with no paid scraper API and no API key. Every search persists to a local SQLite store, so `drift` shows how prices and reorder rates moved since last week, `factory-find` ranks real manufacturers above resellers, and `supplier-report` rolls up a shop's full reliability footprint. Read-only sourcing research.

## Install

The recommended path installs both the `1688-pp-cli` binary and the `pp-1688` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install 1688
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install 1688 --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install 1688 --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install 1688 --agent claude-code
npx -y @mvanhorn/printing-press-library install 1688 --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/1688/cmd/1688-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/1688-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install 1688 --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-1688 --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-1688 --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install 1688 --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/1688-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/1688/cmd/1688-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "1688": {
      "command": "1688-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# health check; confirms the mtop endpoint is reachable, no key needed
1688-pp-cli doctor --dry-run

# live wholesale search (translate English terms to Chinese first for rich results)
1688-pp-cli search 手机壳 --limit 10

# persist the result set into the local store to build drift history
1688-pp-cli sync 手机壳

# rank likely real factories above resellers
1688-pp-cli factory-find 手机壳 --top 10

# after a later sync, see which offers moved on price or reorder rate
1688-pp-cli drift 手机壳

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Sourcing signals competitors don't rank
- **`factory-find`** — Rank wholesale offers by how likely the seller is the real factory, not a reseller, and label each trader / likely-factory / verified-factory.

  _Reach for this when an agent must pick a manufacturer over a middleman among dozens of near-identical listings._

  ```bash
  1688-pp-cli factory-find 蓝牙耳机 --top 10 --json
  ```
- **`repurchase-top`** — Rank synced offers and suppliers by 回头率 (buyer reorder rate), with a minimum-transaction floor to suppress low-volume noise.

  _Use it to surface suppliers buyers actually come back to, instead of trusting a one-off star rating._

  ```bash
  1688-pp-cli repurchase-top 手机壳 --min-tx 100 --json
  ```
- **`region-spread`** — Group stored offers for a keyword by Chinese province and report min, median, and max price plus transaction count per region.

  _Reach for this to spot whether a product is meaningfully cheaper out of one manufacturing cluster before narrowing suppliers._

  ```bash
  1688-pp-cli region-spread 手机壳 --json
  ```

### Local state that compounds
- **`drift`** — Show how an offer's price, reorder rate, and 30-day transaction count moved across your stored snapshots.

  _Reach for this before a reorder to see whether a 'limited-time' price actually dropped or a supplier's reliability is trending down._

  ```bash
  1688-pp-cli drift 手机壳 --json
  ```
- **`compare`** — Render a side-by-side table of price, MOQ, tier, reorder rate, transactions, factory flags, and trade scores for several offers of the same product.

  _Use it to make the final buy decision between a handful of shortlisted suppliers in one view._

  ```bash
  1688-pp-cli compare 927875250705 836112681124 --json
  ```
- **`supplier-report`** — Aggregate one shop across all its stored offers: trade-service scores, average reorder rate, total transactions, verification badges, offer count, and price range.

  _Reach for this to vet or audit a supplier before committing volume, instead of judging from one listing._

  ```bash
  1688-pp-cli supplier-report b2b-2850655109d72ea --json
  ```
- **`watch`** — Re-run a saved search, store a fresh snapshot, and print only what changed since last run: price and reorder-rate moves plus newly appeared offers and suppliers.

  _Use it on a schedule to catch new entrants and price moves in a category without re-reading the whole result set._

  ```bash
  1688-pp-cli watch 手机壳 --json
  ```

## Recipes


### Find verified factories for a product

```bash
1688-pp-cli factory-find 蓝牙耳机 --top 10 --json
```

Ranks likely manufacturers above traders using factory flags, reorder rate, and trade scores.

### Narrow a verbose search payload for an agent

```bash
1688-pp-cli search 手机壳 --agent --select offers.title,offers.price_cny,offers.repurchase_rate,offers.supplier_name
```

Search responses are large and nested; --select pulls only the fields an agent needs so it does not burn context.

### Compare shortlisted suppliers head to head

```bash
1688-pp-cli compare 927875250705 836112681124 --json
```

Side-by-side price/MOQ/reorder/factory signals for specific offers you already synced.

### Check price and reorder-rate drift before a reorder

```bash
1688-pp-cli drift 手机壳 --json
```

Diffs the latest stored snapshot against prior ones so you see real movement, not marketing 'limited-time' labels.

### Rank suppliers by buyer reorder rate

```bash
1688-pp-cli repurchase-top 手机壳 --min-tx 100 --json
```

Surfaces shops buyers actually come back to, ignoring low-volume noise.

## Usage

Run `1688-pp-cli --help` for the full command reference and flag list.

## Commands

### offers

Inspect 1688 wholesale offer detail pages

- **`1688-pp-cli offers <offer_id>`** - Fetch a 1688 offer's public detail page by offer ID


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
1688-pp-cli offers mock-value

# JSON for scripting and agents
1688-pp-cli offers mock-value --json

# Filter to specific fields
1688-pp-cli offers mock-value --json --select id,name,status

# Dry run — show the request without sending
1688-pp-cli offers mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
1688-pp-cli offers mock-value --agent
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
1688-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/1688-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **search returns very few or zero results for an English keyword** — 1688's corpus is Mandarin; translate the term to Simplified Chinese (e.g. 'phone case' -> 手机壳) and retry
- **FAIL_SYS_TOKEN_EMPTY or FAIL_SYS_TOKEN_EXPIRED from the API** — the signed-request token expired; the client re-bootstraps automatically on the next call, so just retry once
- **requests blocked or punished (action=deny) from a cloud/CI host** — Alibaba blocks data-center IPs (cloud_ip_bl); run from a residential IP or a residential proxy, not a cloud egress
- **a transcendence command returns an empty result** — run 'sync <keyword>' first to populate the local store; drift/compare/region-spread read stored data, not the live API

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**oxylabs/1688-scraper**](https://github.com/oxylabs/1688-scraper) — Python
- [**jeff2go/1688-Crawler**](https://github.com/jeff2go/1688-Crawler) — Python
- [**krautsdubisq1g/1688-product-search-scraper**](https://github.com/krautsdubisq1g/1688-product-search-scraper) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
