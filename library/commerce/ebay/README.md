# eBay CLI

Discover, monitor, and analyze eBay listings, auctions, and sold comps from the terminal.

The killer feature is auctions filtered by bid count and ending window — the query the eBay site can no longer answer since the Finding API was retired in February 2025. Pair it with sold-comp pricing intelligence, watchlists, and saved searches.

Learn more at [eBay](https://www.ebay.com).

## What this CLI does well

- **`auctions`** — Search active auctions filtered by bid count and ending window. "Steph Curry rookies with at least 3 bids ending in the next hour" is a single command.
- **`comp`** — Sold-comp pricing intelligence over the last 90 days with smart matching, condition normalization, 1.5x IQR outlier trim, and percentile distribution.
- **`sold`** — Search sold completed listings by keyword.
- **`listings`** — Search active listings (auctions, Buy It Now, or both) with price and condition filters.
- **`watch`** — Inspect your watchlist (authenticated, read-only).
- **`saved-search`** — Local saved-search CRUD for repeatable queries.
- **`feed`** — Stream new listings matching a saved search, with sold-comp context appended.
- **Local SQLite store** — `sync` data once, run `search` queries offline.

See [Known Limitations](#known-limitations) for what doesn't work today.

![ebay-pp-cli discovery flow](docs/discovery-demo.gif)

## Install

### Go

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/ebay/cmd/ebay-pp-cli@latest
```

### Binary

Download from [Releases](https://github.com/mvanhorn/printing-press-library/releases).

## Quick Start

### 1. Authenticate

This CLI uses your Chrome browser session for authentication. Log in to eBay in Chrome, then:

```bash
ebay-pp-cli auth login --chrome
```

It needs a cookie extraction tool. Install one:

```bash
pip install pycookiecheat                # Python (recommended)
brew install barnardb/cookies/cookies    # Homebrew
```

When your session expires, run `auth login --chrome` (or `auth refresh`) again.

### 2. Verify Setup

```bash
ebay-pp-cli doctor
```

### 3. Run the killer commands

```bash
# The query the eBay site can't answer: bid-filtered, ending-soon auctions
ebay-pp-cli auctions "Steph Curry rookie" --has-bids --ending-within 1h

# Sold-comp pricing: what did this card actually go for in the last 90 days?
ebay-pp-cli comp "Cooper Flagg /50 Topps Chrome" --trim

# Active Buy It Now listings under $30
ebay-pp-cli listings --nkw "PSA Mariners Griffey" --lh-bin 1 --udlo 10 --udhi 30

# Pipe and filter for agents
ebay-pp-cli auctions "Rolex" --has-bids --ending-within 30m --agent --select item_id,price,bids,time_left
```

## Cookbook

Real workflows the CLI exists for.

```bash
# Comp before bidding: should I pay $X for this card?
ebay-pp-cli comp "PSA 10 Pikachu illustrator" --trim --json --select mean,median,p25,p75,sample_size

# Find under-priced auctions ending soon
ebay-pp-cli auctions "vintage Rolex Submariner" --has-bids --ending-within 2h --max-price 5000 --json | \
  jq '.[] | select(.price < 3000) | {item_id, price, bids, time_left, url}'

# Cross-condition comparison: how do graded vs raw cards sell?
ebay-pp-cli comp "Zion Williamson rookie" --condition raw --json
ebay-pp-cli comp "Zion Williamson rookie" --condition graded --json

# Find vintage cards with active bidding
ebay-pp-cli auctions "PSA Mariners 1980" --has-bids --max-price 30 --json | \
  jq '.[] | {title, price, bids, ends_at, url}'

# Save a search and re-run it
ebay-pp-cli saved-search create vintage-mariners --query "PSA Mariners Griffey" --max-price 30
ebay-pp-cli feed vintage-mariners --since 1h
```

## Known Limitations

Be honest with yourself before depending on this tool.

### Bid placement is experimental and currently fails

`bid`, `snipe`, and `bid-group` are wired up but cannot complete a bid end-to-end. eBay's `/bfl/placebid/<id>` endpoint redirects browser-cookie sessions to sign-in (step-up auth), even with cookies that pass the `/deals` validation handshake. The three-step flow (`bid module` → `bid trisk` → `bid confirm`) cannot extract the `srt` token because eBay never serves the bid module HTML to non-browser sessions.

These commands are hidden from `--help` and print a warning when invoked directly. They stay in the binary so a future browser-CDP rewrite (routing bid traffic through the user's actual Chrome process) can revive them. Until then, **bid in the browser**.

### Rate limiting under sustained use

eBay's Akamai bot manager throttles the CLI's IP under sustained scraping. Symptoms: discovery commands start returning empty results, or you see HTTP 403 in `--dry-run`-style debug output. Recovery:

```bash
ebay-pp-cli auth refresh
```

…opens Chrome to clear any challenge wall, then re-imports cookies. Backing off for ~15-30 minutes also helps.

### Other gaps

- **Watchlist write paths, bid groups, saved-search CRUD, feed, and offer-hunter** ship as honest "not yet implemented" stubs that print their planned shape. The infrastructure (HTML scraper, local SQLite store) is fully built; only the per-command glue is deferred.
- **`comp image <path>`** (search-by-image against sold comps) requires `EBAY_APP_ID` for `Browse.searchByImage`. Without an App ID, the command exits with a clear "requires App OAuth" message rather than failing silently.
- **Forter token TTL** is unknown for the bid flow. Moot until bid placement is rewritten.

## Usage

Run `ebay-pp-cli --help` for the full command reference. Hidden experimental commands (`bid`, `snipe`, `bid-group`) can be reached by name (e.g. `ebay-pp-cli snipe --help`) but are not listed in the default help output.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ebay-pp-cli auctions "Griffey rookie" --has-bids

# JSON for scripting and agents
ebay-pp-cli auctions "Griffey rookie" --has-bids --json

# Filter to specific fields
ebay-pp-cli auctions "Griffey rookie" --has-bids --json --select item_id,price,bids,time_left

# Dry run — show the request without sending
ebay-pp-cli auctions "Griffey rookie" --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ebay-pp-cli auctions "Griffey rookie" --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Offline-friendly** - sync/search commands use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set
- **Self-describing** - `agent-context` emits a JSON description of the CLI's current capabilities, including which commands are experimental

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-ebay -g
```

Then invoke `/pp-ebay <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/ebay/cmd/ebay-pp-mcp@latest
```

Some tools work without auth. For full access, set up auth first (the bundle reuses your local browser session):

```bash
ebay-pp-cli auth login --chrome
```

Then register it:

```bash
claude mcp add ebay ebay-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
ebay-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ebay-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle, install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/ebay/cmd/ebay-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ebay": {
      "command": "ebay-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
ebay-pp-cli doctor
```

Verifies configuration, credentials, and connectivity.

## Configuration

Config file: `~/.config/ebay-pp-cli/config.toml`

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `ebay-pp-cli doctor` to check credentials
- Run `ebay-pp-cli auth refresh` to re-import Chrome cookies

**Empty discovery results**
- Likely rate limiting; see [Known Limitations](#known-limitations)

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the relevant `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport (TLS fingerprint impersonation via `surf`) for browser-facing endpoints. It does not require a resident browser process for discovery commands.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), then hand-edited 2026-04-30 for capability honesty.
