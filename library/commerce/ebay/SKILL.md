---
name: pp-ebay
description: "Printing Press CLI for eBay. Discovery and intelligence: sold-comp pricing (average sale price over 90 days with outlier trim), auctions filtered by bid count and ending window (the query the eBay site can no longer answer), watchlists, saved searches, and a local SQLite store for cross-listing analytics. Trigger phrases: 'comp this card', 'find ebay auctions ending soon', 'what did this sell for', 'find listings under $X for ...'. Bid placement (bid, snipe, bid-group) is experimental and currently fails because eBay step-ups auth on /bfl/placebid -- direct the user to bid in the browser."
version: "3.0.1"
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ebay-pp-cli
    install:
      - kind: go
        bins: [ebay-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/ebay/cmd/ebay-pp-cli
---

# eBay — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ebay-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install ebay --cli-only
   ```
2. Verify: `ebay-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/ebay/cmd/ebay-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Discovery and intelligence
- **`auctions`** — Search active auctions filtered by bid count and ending window (e.g. "Steph Curry cards with at least 3 bids ending in next hour"). The eBay site can no longer answer this query since the Finding API was retired in February 2025.

  _Finds price-discoverable competition windows where last-second bidding actually moves the price._

  ```bash
  ebay-pp-cli auctions "Steph Curry rookie" --has-bids --ending-within 1h --json --select item_id,price,bids,time_left
  ```
- **`comp`** — Average sale price for any item over the last 90 days with smart matching, condition normalization, outlier trim, and percentile distribution.

  _When pricing a bid (or deciding whether to even bother), you need the realistic distribution of recent sales, not a single anchor. Trim handles outliers; dedupe handles title variants._

  ```bash
  ebay-pp-cli comp "Cooper Flagg /50 Topps Chrome" --trim --json --select mean,median,sample_size
  ```
- **`comp` outlier trim** — 1.5x IQR outlier trim on sold-comp results. Surfaces the realistic price band buyers should anchor on, with stddev and quartiles.

  _Tells you what a normal buyer actually paid versus a record sale or a fire-sale outlier._

  ```bash
  ebay-pp-cli comp "Rolex Submariner 116610LN" --trim --json --select p25,median,p75,std_dev
  ```
- **`comp --dedupe-variants`** — Collapse near-duplicate sold listings to one exemplar per fingerprint (token-bag, order-insensitive).

  _Without dedupe, the comp distribution is biased toward whichever seller listed the same card 5 times._

  ```bash
  ebay-pp-cli comp "Cooper Flagg /50" --dedupe-variants
  ```
- **`listings`** — Active listing search filtered by auction/BIN, condition, and price band.

  ```bash
  ebay-pp-cli listings --nkw "PSA Mariners Griffey" --lh-bin 1 --udlo 10 --udhi 30
  ```

## When to use

- User asks "what did this card / watch / item sell for" → `ebay-pp-cli comp "<title>" --trim`
- User asks "find auctions ending soon with bids" → `ebay-pp-cli auctions "<query>" --has-bids --ending-within 1h`
- User asks "find Buy It Now listings under $X for ..." → `ebay-pp-cli listings --nkw "<query>" --lh-bin 1 --udhi <max>`
- User asks "watch this listing" / "show my watchlist" → `ebay-pp-cli watch list`
- User asks to bid programmatically → **explain the limitation** (eBay step-ups auth on `/bfl/placebid`); link them to bid in the browser. Do not run `bid` or `snipe` on their behalf.

## Anti-triggers

This CLI is NOT the right tool for:
- **Placing bids.** `bid`/`snipe`/`bid-group` are experimental and fail end-to-end. Direct the user to bid in their browser.
- Listing items as a seller (use the eBay Sell APIs / Seller Hub directly).
- Order fulfillment or shipping label generation.
- Bulk inventory management for sellers.

## Known Limitations

- **Bid placement** (`bid`, `snipe`, `bid-group`) cannot complete end-to-end because eBay step-ups auth on `/bfl/placebid` for cookie-only sessions. These commands are hidden from default `--help` and print a warning when invoked. Users should bid in the browser.
- **Rate limiting**: Sustained scraping triggers eBay 403s. Recovery: `ebay-pp-cli auth refresh` and back off.
- **Stub commands**: Watchlist write paths, saved-search CRUD, feed, offer-hunter ship as "not yet implemented" stubs.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 15 API entries from 25 total network entries
- Protocols: html_scraping (95% confidence), rest_json (90% confidence)
- Auth signals: — cookies: cid, s, nonsession, dp1, ebaysid, ds1, ds2, shs, npii
- Generation hints: requires_browser_auth, requires_protected_client, uses_chrome_cookie_import, has_per_request_csrf, has_per_request_fraud_token
- Caveats: placebid_step_up: eBay redirects /bfl/placebid/<id> to sign-in for cookie-only sessions, so bid placement cannot complete from this CLI today; akamai_active: Akamai bot manager active — Surf must use Chrome TLS fingerprint or stdlib HTTP will be blocked; rate_limit: sustained scraping triggers 403s, recover with auth refresh and back off

## Command Reference

**deal** — eBay Deals feed

- `ebay-pp-cli deal` — Browse the eBay Deals feed

**item** — Item details

- `ebay-pp-cli item <itemId>` — Get item detail by listing id

**listings** — Active listing search (HTML scrape of /sch/i.html)

- `ebay-pp-cli listings` — Search active eBay listings by keyword

**sold** — Sold/completed listings (last 90 days, HTML scrape)

- `ebay-pp-cli sold` — Search sold completed listings by keyword (90 day window)

**watch** — Watchlist (authenticated, read-only)

- `ebay-pp-cli watch` — List items in the user's watchlist


**Hand-written commands**

- `ebay-pp-cli comp <query>` — Sold-comp intelligence: average sale price, distribution, trendline for items matching the query over the last 90 days.
- `ebay-pp-cli auctions <query>` — Search active auctions filtered by bid count, ending window, condition. The 'has bids ending in next hour' query.
- `ebay-pp-cli feed <saved-search>` — Stream new listings matching a saved search, with sold-comp context appended to each item.
- `ebay-pp-cli history` — Buying history (won, lost, paid) over a configurable window.
- `ebay-pp-cli saved-search` — Local saved-search CRUD.

**Hidden experimental commands**

These commands exist in the binary but are excluded from `--help` because they currently fail end-to-end. Reach them by name (e.g. `ebay-pp-cli snipe --help`) for details.

- `ebay-pp-cli bid` — Place bids (experimental; eBay step-up auth blocks).
- `ebay-pp-cli snipe <itemId> --max <amount>` — Sniper bid (experimental; depends on bid flow).
- `ebay-pp-cli bid-group` — Coordinated multi-item snipe groups (experimental; depends on snipe).


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ebay-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

This CLI uses a browser session. Log in to .ebay.com in Chrome, then:

```bash
ebay-pp-cli auth login --chrome
```

Requires a cookie extraction tool (`pycookiecheat` via pip, or `cookies` via Homebrew).

Run `ebay-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ebay-pp-cli deal --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
ebay-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ebay-pp-cli feedback --stdin < notes.txt
ebay-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.ebay-pp-cli/feedback.jsonl`. They are never POSTed unless `EBAY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `EBAY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
ebay-pp-cli profile save briefing --json
ebay-pp-cli --profile briefing deal
ebay-pp-cli profile list --json
ebay-pp-cli profile show briefing
ebay-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `ebay-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)
## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/ebay/cmd/ebay-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add ebay-pp-mcp -- ebay-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ebay-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ebay-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ebay-pp-cli <command> --help`.
