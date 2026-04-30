# eBay CLI

Buyer-power-user CLI for eBay. Sold-comp intelligence, true sniper bidding, watchlist intelligence, saved-search feeds, and a local SQLite store for cross-listing analytics.

Learn more at [eBay](https://www.ebay.com).

## Unique Features

These capabilities aren't available in any other tool for this API.

### Buyer intelligence
- **`comp`** — Average sale price for any item over the last 90 days with smart matching, condition normalization, outlier trim, and percentile distribution.

  _When pricing a bid, you need the realistic distribution of recent sales, not a single anchor. Trim handles outliers; dedupe handles title variants._

  ```bash
  ebay-pp-cli comp "Cooper Flagg /50 Topps Chrome" --trim --json --select mean,median,sample_size
  ```
- **`snipe`** — Hold a max bid client-side, fire through the user's authenticated session at lead-seconds before close. Other bidders never see the max until it's too late to react.

  _Reveals max only at fire time, not on bid-form open. Other bidders cannot react inside 8 seconds._

  ```bash
  ebay-pp-cli snipe 123456789012 --max 50.00 --lead 8s
  ```
- **`auctions`** — Search active auctions filtered by bid count and ending window (e.g. "Steph Curry cards with at least 3 bids ending in next hour").

  _Finds price-discoverable competition windows where last-second bidding actually moves the price._

  ```bash
  ebay-pp-cli auctions "Steph Curry rookie" --has-bids --ending-within 1h --json --select item_id,price,bids,time_left
  ```
- **`comp`** — 1.5x IQR outlier trim on sold-comp results. Surfaces the realistic price band buyers should anchor on, with stddev and quartiles.

  _Tells you what a normal buyer actually paid versus a record sale or a fire-sale outlier._

  ```bash
  ebay-pp-cli comp "Rolex Submariner 116610LN" --trim --json --select p25,median,p75,std_dev
  ```
- **`comp --dedupe-variants`** — Collapse near-duplicate sold listings to one exemplar per fingerprint (token-bag, order-insensitive).

  _Without dedupe, the comp distribution is biased toward whichever seller listed the same card 5 times._

  ```bash
  ebay-pp-cli comp "Cooper Flagg /50" --dedupe-variants
  ```

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/commerce/ebay/cmd/ebay-pp-cli@latest
```

### Binary

Download from [Releases](https://github.com/mvanhorn/printing-press-library/releases).

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Authenticate

This CLI uses your browser session for authentication. Log in to .ebay.com in Chrome, then:

```bash
ebay-pp-cli auth login --chrome
```

Requires a cookie extraction tool. Install one:

```bash
pip install pycookiecheat          # Python (recommended)
brew install barnardb/cookies/cookies  # Homebrew
```

When your session expires, run `auth login --chrome` again.

### 3. Verify Setup

```bash
ebay-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Run the killer commands

```bash
# Sold-comp intelligence: average sale price for any item over the last 90 days
ebay-pp-cli comp "Cooper Flagg /50 Topps Chrome" --trim

# Has-bids + ending-soon auction discovery
ebay-pp-cli auctions "Steph Curry rookie" --has-bids --ending-within 1h

# Sniper bid: hold your max client-side, fire at T-25s
ebay-pp-cli snipe 123456789012 --max 50.00

# Pipe and filter for agents
ebay-pp-cli auctions "Rolex" --has-bids --ending-within 30m --agent --select item_id,price,bids,time_left
```

## Usage

Run `ebay-pp-cli --help` for the full command reference and flag list.

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

# Test a snipe without placing
ebay-pp-cli snipe 123456789012 --max 75 --simulate

# Place a real snipe with 8s lead time
ebay-pp-cli snipe 123456789012 --max 75 --lead 8s

# Bulk dry-run snipes from a list
echo -e "123456789012 50.00\n397872649101 25.00" | while read id max; do
  ebay-pp-cli snipe "$id" --max "$max" --simulate --json
done
```

## Known Gaps

- **Watchlist write paths, bid groups, saved-search CRUD, feed, and offer-hunter** ship as honest "not yet implemented" stubs that print their planned shape. The infrastructure (HTML scraper, bid client, local SQLite store) is fully built; only the per-command glue is deferred. Use `ebay-pp-cli history won --help` etc. to see the planned invocation.
- **Akamai bot manager** can throttle the CLI's IP under sustained load. Run `auth login --chrome` to import your logged-in cookies; this clears most challenges. The CLI returns a typed `RateLimitError` with explicit messaging when throttling is detected, never a silent empty result.
- **Forter token TTL** is unknown. Long-running snipes refresh the token at fire time by re-fetching the bid module; the simulator does not exercise this path, so always run a real `snipe --simulate` shortly before the auction's lead window for a sanity check.
- **`comp image <path>`** (search-by-image against sold comps) requires `EBAY_APP_ID` for Browse.searchByImage. Without an App ID, the command exits with an honest "requires App OAuth" message rather than failing silently.

## Commands

### bid

Place bids on auction listings (authenticated)

- **`ebay-pp-cli bid confirm`** - Place the actual bid (action=confirmbid). The endpoint that wins or loses an auction.
- **`ebay-pp-cli bid module`** - Load the bid form module to extract srt + forterToken (internal step)
- **`ebay-pp-cli bid trisk`** - Trust/risk pre-check before bid placement (action=trisk)

### deal

eBay Deals feed

- **`ebay-pp-cli deal list`** - Browse the eBay Deals feed

### item

Item details

- **`ebay-pp-cli item get`** - Get item detail by listing id

### listings

Active listing search (HTML scrape of /sch/i.html)

- **`ebay-pp-cli listings list`** - Search active eBay listings by keyword

### sold

Sold/completed listings (last 90 days, HTML scrape)

- **`ebay-pp-cli sold list`** - Search sold completed listings by keyword (90 day window)

### watch

Watchlist (authenticated)

- **`ebay-pp-cli watch list`** - List items in the user's watchlist


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ebay-pp-cli deal

# JSON for scripting and agents
ebay-pp-cli deal --json

# Filter to specific fields
ebay-pp-cli deal --json --select id,name,status

# Dry run — show the request without sending
ebay-pp-cli deal --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ebay-pp-cli deal --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
# Some tools work without auth. For full access, set up auth first:
ebay-pp-cli auth login --chrome

claude mcp add ebay ebay-pp-mcp
```

### Claude Desktop

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

## Health Check

```bash
ebay-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/ebay-pp-cli/config.toml`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ebay-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Capture coverage: 15 API entries from 25 total network entries
- Reachability: browser_clearance_http (95% confidence)
- Protocols: html_scraping (95% confidence), rest_json (90% confidence)
- Auth signals: — cookies: cid, s, nonsession, dp1, ebaysid, ds1, ds2, shs, npii
- Protection signals: akamai_bot_manager (90% confidence), forter_fraud (90% confidence), csrf_signed_request_token (95% confidence)
- Generation hints: requires_browser_auth, requires_protected_client, uses_chrome_cookie_import, has_per_request_csrf, has_per_request_fraud_token

Warnings from discovery:
- fraud_token_ttl: Forter token TTL is unknown; long-running snipes may need to refresh forterToken at fire time by re-fetching the bid module
- referrer_check: Direct nav to /bfl/placebid/<id> returns Oops error; bid module must be fetched via in-page click context
- akamai_active: Akamai bot manager active — Surf must use Chrome TLS fingerprint or stdlib HTTP will be blocked

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
