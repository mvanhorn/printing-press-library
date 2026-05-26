# LoopNet CLI

**LoopNet shows you today — this CLI remembers, building the price-cut, days-on-market, and supply trends LoopNet never exposes.**

A SQLite-backed, agent-native CLI for LoopNet, the largest US commercial real estate marketplace. Search inventory and pull full listing detail like any scraper — then sync a submarket on a schedule and the local store accumulates the time series LoopNet hides. price-cuts finds every drop, dom computes true days-on-market, velocity tracks absorption, distress surfaces motivated sellers. Built to feed CRE market-intelligence pipelines: every command emits clean JSON or CSV.

Learn more at [LoopNet](https://www.loopnet.com).

Printed by [@melanson633](https://github.com/melanson633).

## Install

The recommended path installs both the `loopnet-pp-cli` binary and the `pp-loopnet` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install loopnet
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install loopnet --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install loopnet --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install loopnet --agent claude-code
npx -y @mvanhorn/printing-press-library install loopnet --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/loopnet/cmd/loopnet-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/loopnet-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-loopnet --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-loopnet --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-loopnet skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-loopnet. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/loopnet-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "loopnet": {
      "command": "loopnet-pp-mcp"
    }
  }
}
```

</details>

## Authentication

LoopNet's data pages are protected by Akamai Bot Manager, which blocks plain HTTP clients. The CLI fetches them by replaying short-lived clearance cookies from a real browser session. Run 'loopnet-pp-cli auth refresh' to mint cookies — it briefly drives the browser-use tool — or 'auth set --cookies' to paste a Cookie header copied from your browser's DevTools. Cookies last a few hours; refresh when a fetch fails with a bot-challenge error. No API key is required.

## Quick Start

```bash
# Mint Akamai clearance cookies (briefly opens a browser). Required before any live fetch.
loopnet-pp-cli auth refresh

# Live search — confirm cookies work and listings parse.
loopnet-pp-cli inventory worcester-ma --type industrial --listing for-sale --json

# Pull the submarket into the local store (fetches are paced ~10s for Akamai).
loopnet-pp-cli sync worcester-ma --type industrial --listing for-sale

# Cap-rate and yield distribution, computed offline from the store.
loopnet-pp-cli caprate worcester-ma --type industrial --agent

# After a later sync, see which listings dropped their asking price.
loopnet-pp-cli price-cuts worcester-ma --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Change tracking only the local store enables
- **`price-cuts`** — Surfaces every synced listing whose asking price dropped between syncs, with old price, new price, percent cut, and days on market at the cut.

  _Reach for this to find motivated sellers and re-priced assets — a price cut is the single strongest deal-sentiment signal LoopNet itself never exposes._

  ```bash
  loopnet-pp-cli price-cuts worcester-ma --type industrial-properties --agent
  ```
- **`dom`** — Computes true days on market for every live listing from the date the CLI first saw it, and flags aged inventory past a threshold.

  _Use this to tell a fresh comp from a stale one and to find listings that have languished — both invisible on LoopNet._

  ```bash
  loopnet-pp-cli dom worcester-ma --min-days 90 --agent
  ```
- **`velocity`** — Reports absorption for a submarket: new listings, delistings, median days on market, and net supply change per period.

  _Reach for this to gauge whether a submarket is heating up or cooling — the supply-and-demand pulse for a market-intelligence brief._

  ```bash
  loopnet-pp-cli velocity worcester-ma --agent
  ```
- **`delisted`** — Lists listings present in a prior sync but absent now — sold, withdrawn, or expired.

  _Use this to track which assets cleared the market — a proxy for transaction velocity LoopNet never publishes._

  ```bash
  loopnet-pp-cli delisted worcester-ma --since 30d --agent
  ```

### Pricing, yield and distress intelligence
- **`caprate`** — Reports the cap-rate, NOI, and price-per-square-foot distribution (count, min, median, quartiles, max) for synced for-sale listings in a submarket, and flags listings whose cap rate falls outside the interquartile range.

  _Use this to benchmark a single asset's yield against its submarket and spot mispriced cap rates._

  ```bash
  loopnet-pp-cli caprate worcester-ma --type industrial-properties --agent
  ```
- **`distress`** — Flags listings carrying motivation signals: price-reduced and must-sell keyword hits in the description, Ten-X auction listings, and recent price cuts.

  _Reach for this to find distressed and motivated-seller assets in one sweep instead of reading every listing's free text._

  ```bash
  loopnet-pp-cli distress worcester-ma --agent
  ```

### Analyst and pipeline workflows
- **`digest`** — Rolls a synced submarket into one report: live supply count, recent price cuts, median days on market, new and delisted counts, and distress hits.

  _Reach for this for a one-command market-intelligence snapshot of a submarket instead of running five separate analysis commands._

  ```bash
  loopnet-pp-cli digest worcester-ma --type industrial-properties --agent
  ```
- **`feed`** — Exports the latest synced submarket as a run-stamped JSON or CSV file, with records mapped to the six CRE market-intelligence data categories.

  _Reach for this to drop LoopNet data straight into a downstream CRE pipeline (e.g. a data/raw ingest folder) without glue code._

  ```bash
  loopnet-pp-cli feed worcester-ma --format csv --out ./loopnet-worcester.csv
  ```

## Usage

Run `loopnet-pp-cli --help` for the full command reference and flag list.

## Commands

### inventory

Search LoopNet commercial real estate inventory by location, property type, and sale/lease.

- **`loopnet-pp-cli inventory <location> [--type <property_type>] [--listing for-sale|for-lease]`** - Search LoopNet listings for a location, property type, and sale-or-lease. Returns a server-rendered results page carrying a schema.org CollectionPage JSON-LD block with one RealEstateListing per result.

### property

Fetch the full detail record for a single LoopNet listing.

- **`loopnet-pp-cli property <id>`** - Fetch one LoopNet listing's detail page. The page carries a schema.org RealEstateListing/Product JSON-LD block plus a property-facts table (price, cap rate, building class, zoning, year built, parcel numbers, tax assessments).

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
loopnet-pp-cli inventory worcester-ma --type industrial --listing for-sale

# JSON for scripting and agents
loopnet-pp-cli inventory worcester-ma --type industrial --listing for-sale --json

# Filter to specific fields
loopnet-pp-cli inventory worcester-ma --type industrial --listing for-sale --json --select id,name,status

# Dry run — show the request without sending
loopnet-pp-cli inventory worcester-ma --type industrial --listing for-sale --dry-run

# Agent mode — JSON + compact + no prompts in one flag
loopnet-pp-cli inventory worcester-ma --type industrial --listing for-sale --agent
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
loopnet-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/loopnet-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Fetch fails: 'Akamai bot-challenge page' / clearance cookies expired** — Run 'loopnet-pp-cli auth refresh' to mint fresh cookies (or 'auth set --cookies "..."' to paste them). Cookies last a few hours; check state with 'auth status'.
- **price-cuts, dom, velocity, or delisted return nothing** — These read the local store and need at least two syncs of the same submarket over time. Run sync again after a gap, then re-run the command.
- **sync is slow** — Each LoopNet fetch is paced ~10 seconds to stay under Akamai's bot-velocity threshold — this is expected. Lower --limit and --pages for a faster partial sync.
- **inventory or sync returns 0 results for a location** — Use LoopNet's slug form: a city-state like worcester-ma, a state like ma, or a 5-digit zip. Type accepts office, industrial, retail, multifamily, land, hospitality, health-care.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.loopnet.com/search/office/los-angeles-ca/for-sale/
- Capture coverage: 0 API entries from 3 total network entries
- Reachability: browser_http (85% confidence)
- Protocols: ssr_embedded_data (85% confidence)
- Protection signals: cloudflare (90% confidence), captcha (85% confidence), akamai (75% confidence)
- Generation hints: requires_protected_client

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**ln-scraper**](https://github.com/Spareo/ln-scraper) — Python (6 stars)
- [**LoopnetMCP**](https://github.com/johnstenner/LoopnetMCP) — Python
- [**commercial-realestate-crawler-v3**](https://github.com/BenNormann/commercial-realestate-crawler-v3) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
