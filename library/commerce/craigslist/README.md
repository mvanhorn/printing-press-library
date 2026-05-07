# Craigslist CLI

**The local-first Craigslist watcher and triage tool that knows what's a repost, what's a scam, and what just dropped in price.**

craigslist-pp-cli wraps Craigslist's own undocumented JSON endpoints (sapi, rapi, reference) — the same ones the Craigslist mobile app uses — and layers a local SQLite snapshot history on top. That powers cross-city search, saved-search alerts that distinguish true new listings from edits and reposts, price-drift detection, and cross-city duplicate / scam scoring. Free, scriptable, no PII selling, no proxies, no API key.

Learn more at [Craigslist](https://www.craigslist.org).

## Install

### Binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/craigslist-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

### Go

```
go install github.com/mvanhorn/printing-press-library/library/commerce/craigslist/cmd/craigslist-pp-cli@latest
```

## Authentication

No authentication required. Craigslist's read endpoints (sapi.craigslist.org/web/v8, rapi.craigslist.org/web/v8, reference.craigslist.org) are public. The CLI sets a polite User-Agent and reuses the cl_b cookie Craigslist auto-issues. Posting and account management are intentionally out of scope for v1.

## Quick Start

```bash
# Pull the 178-category and 707-area reference taxonomy into the local store; refreshed at most every 30 days per Craigslist's own Cache-Control.
craigslist-pp-cli catalog refresh


# Single-city search with a price cap; default human table output.
craigslist-pp-cli search 'ipad' --site sfbay --category sss --max-price 300


# Cross-city hunt for a rare item — fans out parallel sapi calls, source-attributes results.
craigslist-pp-cli search 'leica m6' --sites sfbay,nyc,seattle,losangeles,chicago --category pho --json


# Populate the local store before running snapshot-driven commands (drift, scam-score, dupe-cluster, median, reposts).
craigslist-pp-cli cl-sync --site sfbay --category apa --since 7d


# Save a smart search with negative keywords Craigslist's own search doesn't support.
craigslist-pp-cli watch save apartments --query 1BR --negate furnished,sublet --sites sfbay --category apa --max-price 2500


# Stream new-listing events as JSON lines: [NEW] (new posts) and [PRICE-DROP] (sellers dropped the price).
craigslist-pp-cli watch tail apartments --interval 5m --json


# Rule-based scam triage on a listing (requires `cl-sync` to have synced the listing first).
craigslist-pp-cli scam-score 7915891289 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Compounding watch surface
- **`watch run`** — Periodic poll for a saved search that emits NEW, PRICE-DROP, and SEED events instead of any-listing-I-haven't-seen — so true new listings stand out from edits and reposts. Cross-city repost detection lives in the separate `reposts` command; cross-city dup detection lives in `dupe-cluster`.

  _Pick this over a generic search call when an agent is alerting a user — only true-new listings are worth pinging on, and the typed diff event tells the agent how to phrase the alert._

  ```bash
  craigslist-pp-cli watch save apartments --query 1BR --negate furnished,sublet --sites sfbay --category apa --max-price 2500 && craigslist-pp-cli watch run apartments --seed-only && craigslist-pp-cli watch run apartments --json
  ```
- **`watch tail`** — Long-running tail that prints one JSON event per new diff result so an agent or shell pipeline can react in real time. Emits NEW and PRICE-DROP events.

  _When you want a continuous stream of new-listing events to pipe into a Slack/email sender, jq filter, or downstream agent loop._

  ```bash
  craigslist-pp-cli watch tail apartments --interval 5m --json
  ```

### Local snapshot history
- **`drift`** — Show the price timeline for a single listing across every snapshot we've captured.

  _Agents negotiating on behalf of a buyer can quote concrete price-drop history — "this listing was $50, now $35, posted 14 days ago."_

  ```bash
  craigslist-pp-cli drift 7915891289 --json
  ```
- **`dupe-cluster`** — Find listings whose body fingerprint and image hash match across cities. Surfaces cross-city scams and aggregator reposts.

  _Use before driving across town to view a rental — a cluster of 6+ cities for one apartment is almost always a scam._

  ```bash
  craigslist-pp-cli dupe-cluster --category apa --min-cluster-size 3 --json
  ```
- **`reposts`** — Find listings that have been reposted N or more times in the last X days, by body fingerprint clustering.

  _Reposts signal motivated sellers (negotiation leverage) or spam flooders (skip). Telling them apart is the value._

  ```bash
  craigslist-pp-cli reposts 'eames lounge' --min-reposts 3 --window 30d --json
  ```
- **`cities heat`** — Across cities, count fresh listings per category over a window. Surfaces which markets are hot for what.

  _When an agent is hunting cross-city for a specific item, knowing which 3 cities have the most fresh activity tells it where to look first._

  ```bash
  craigslist-pp-cli cities heat --category sss --since 24h --top 20 --json
  ```

### Triage and scoring
- **`scam-score`** — Rule-based 0-100 score for a listing using brand-new-account, below-median-price, wire-transfer keywords, and cross-city duplicate signals.

  _Agents triaging "is this listing legit" questions get an actionable number plus the per-rule contributions, instead of a vibes-based answer._

  ```bash
  craigslist-pp-cli scam-score 7915891289 --json
  ```
- **`median`** — p25/p50/p75 of prices for a query, optionally over a time window or split by city.

  _When an agent needs a fair-price benchmark before suggesting an offer, or when a reseller is sizing up a market._

  ```bash
  craigslist-pp-cli median 'iphone 15' --category mob --since 30d --by-city --json
  ```

### Search beyond CL
- **`search --negate`** — Search with NOT-keyword filtering that Craigslist's own search doesn't support natively.

  _Apartment-hunting and job-search personas live and die by exclusion terms; this is the difference between scanning 50 results and scanning 5._

  ```bash
  craigslist-pp-cli search '1BR' --category apa --site sfbay --negate furnished,sublet,studio --json
  ```
- **`since`** — Ad-hoc "what's new in this category in this city since X duration ago" without setting up a saved search.

  _The first command an agent should run for "what hit while I was sleeping" — no per-watch setup, no state, just a window._

  ```bash
  craigslist-pp-cli since 24h --site sfbay --category sss --query ipad --json
  ```

## Usage

Run `craigslist-pp-cli --help` for the full command reference and flag list.

## Commands

The full command tree is discoverable via `craigslist-pp-cli --help`. Headline groups:

- **Reference taxonomy** — `categories list`, `areas list`, `catalog refresh`
- **Search & fetch** — `search`, `postings`, `listing get|get-by-pid|images`, `filters show`
- **Local store population** — `cl-sync`
- **Saved-search watches** — `watch save|list|show|delete|run|tail`
- **Snapshot analytics** — `drift`, `dupe-cluster`, `reposts`, `median`, `cities heat`, `since`, `geo within|bbox`
- **Triage** — `scam-score`
- **Local-state CRUD** — `favorite add|list|remove`
- **Framework helpers** — `doctor`, `version`, `which`, `agent-context`, `analytics`, `export`, `import`, `feedback`, `profile`, `tail`, `sync`, `workflow`


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
craigslist-pp-cli postings

# JSON for scripting and agents
craigslist-pp-cli postings --json

# Filter to specific fields
craigslist-pp-cli postings --json --select id,name,status

# Dry run — show the request without sending
craigslist-pp-cli postings --dry-run

# Agent mode — JSON + compact + no prompts in one flag
craigslist-pp-cli postings --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `CRAIGSLIST_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `craigslist-pp-cli postings`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-craigslist -g
```

Then invoke `/pp-craigslist <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/craigslist/cmd/craigslist-pp-mcp@latest
```

Then register it:

```bash
claude mcp add craigslist craigslist-pp-mcp
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-craigslist --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-craigslist --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-craigslist skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-craigslist. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/craigslist-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/craigslist/cmd/craigslist-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "craigslist": {
      "command": "craigslist-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
craigslist-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/craigslist-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Confirm the site hostname is a valid Craigslist abbreviation: `craigslist-pp-cli areas list --grep <name>`
- Confirm the category abbreviation: `craigslist-pp-cli categories list --grep <name>`
- For listings, the integer post id (URL form) and the rapi UUID are different identifiers — use `listing get-by-pid` if you only have the integer.

### API-specific

- **HTTP 403 with body 'Your request has been blocked'** — You hit Craigslist's anti-bot threshold. Wait 30 minutes, then resume with longer poll intervals (--interval 5m or higher). Avoid running parallel watches across more than 3-5 sites at once.
- **Search returns 0 items but the website shows results** — Confirm the site abbreviation with `craigslist-pp-cli areas list --grep <name>`; sapi rejects requests against unknown hostnames silently. Hostname is the short slug (sfbay, nyc, losangeles), not the city name.
- **`watch run` keeps reporting the same listings as NEW** — The local store is missing or got reset. Run `craigslist-pp-cli watch run <name>` once with `--seed-only` to populate the seen-listing baseline without alerting.
- **RSS feeds 403 — "the URL has ?format=rss in it"** — Craigslist hard-blocks RSS as of 2024. The CLI uses sapi/rapi/sitemap-by-date instead — never call CL's RSS URL directly.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**juliomalegria/python-craigslist**](https://github.com/juliomalegria/python-craigslist) — Python (1100 stars)
- [**irahorecka/pycraigslist**](https://github.com/irahorecka/pycraigslist) — Python (54 stars)
- [**sa7mon/craigsfeed**](https://github.com/sa7mon/craigsfeed) — Go (5 stars)
- [**node-craigslist**](https://github.com/brozeph/node-craigslist) — JavaScript
- [**ecnepsnai/craigslist**](https://github.com/ecnepsnai/craigslist) — Go
- [**meub/craigslist-for-sale-alerts**](https://github.com/meub/craigslist-for-sale-alerts) — Python
- [**3d-logic/craigslist-automation**](https://github.com/3d-logic/craigslist-automation) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
