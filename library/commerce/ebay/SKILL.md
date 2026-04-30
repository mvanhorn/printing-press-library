---
name: pp-ebay
description: "Printing Press CLI for eBay. Sold-comp intelligence (average sale price over 90 days with outlier trim), true sniper bidding (max held client-side, fired at T-N seconds via the user's session), has-bids+ending-window auction discovery, and a local SQLite store for cross-listing analytics. Trigger phrases: 'comp this card', 'snipe this auction', 'what did this sell for'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["ebay-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/commerce/ebay/cmd/ebay-pp-cli@latest","bins":["ebay-pp-cli"],"label":"Install via go install"}]}}'
---

# eBay — Printing Press CLI

Buyer-power-user CLI for eBay. Sold-comp intelligence, true sniper bidding, watchlist intelligence, saved-search feeds, and a local SQLite store for cross-listing analytics.

## Unique Capabilities

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

## When to use

- User asks "what did this card / watch / item sell for" → `ebay-pp-cli comp "<title>" --trim`
- User asks "find auctions ending soon with bids" → `ebay-pp-cli auctions "<query>" --has-bids --ending-within 1h`
- User asks to bid programmatically without revealing max → `ebay-pp-cli snipe <itemId> --max <amount>`

## Anti-triggers

This CLI is NOT the right tool for:
- Listing items as a seller (use the eBay Sell APIs / Seller Hub directly).
- Order fulfillment or shipping label generation.
- Bulk inventory management for sellers.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 15 API entries from 25 total network entries
- Protocols: html_scraping (95% confidence), rest_json (90% confidence)
- Auth signals: — cookies: cid, s, nonsession, dp1, ebaysid, ds1, ds2, shs, npii
- Generation hints: requires_browser_auth, requires_protected_client, uses_chrome_cookie_import, has_per_request_csrf, has_per_request_fraud_token
- Caveats: fraud_token_ttl: Forter token TTL is unknown; long-running snipes may need to refresh forterToken at fire time by re-fetching the bid module; referrer_check: Direct nav to /bfl/placebid/<id> returns Oops error; bid module must be fetched via in-page click context; akamai_active: Akamai bot manager active — Surf must use Chrome TLS fingerprint or stdlib HTTP will be blocked

## Command Reference

**bid** — Place bids on auction listings (authenticated)

- `ebay-pp-cli bid confirm` — Place the actual bid (action=confirmbid). The endpoint that wins or loses an auction.
- `ebay-pp-cli bid module` — Load the bid form module to extract srt + forterToken (internal step)
- `ebay-pp-cli bid trisk` — Trust/risk pre-check before bid placement (action=trisk)

**deal** — eBay Deals feed

- `ebay-pp-cli deal` — Browse the eBay Deals feed

**item** — Item details

- `ebay-pp-cli item <itemId>` — Get item detail by listing id

**listings** — Active listing search (HTML scrape of /sch/i.html)

- `ebay-pp-cli listings` — Search active eBay listings by keyword

**sold** — Sold/completed listings (last 90 days, HTML scrape)

- `ebay-pp-cli sold` — Search sold completed listings by keyword (90 day window)

**watch** — Watchlist (authenticated)

- `ebay-pp-cli watch` — List items in the user's watchlist


**Hand-written commands**

- `ebay-pp-cli comp <query>` — Sold-comp intelligence: average sale price, distribution, trendline for items matching the query over the last 90 days.
- `ebay-pp-cli snipe <itemId> --max <amount>` — True sniper bid: hold a max client-side, fire at T-N seconds with the user's session. Max stays hidden from other...
- `ebay-pp-cli bid-group` — Coordinated multi-item snipe groups (single-win, multi-win=N, contingency).
- `ebay-pp-cli auctions <query>` — Search active auctions filtered by bid count, ending window, condition. The 'has bids ending in next hour' query.
- `ebay-pp-cli feed <saved-search>` — Stream new listings matching a saved search, with sold-comp context appended to each item.
- `ebay-pp-cli offer-hunter <saved-search>` — Auto-submit best offers across a saved search at a percentage of asking price.
- `ebay-pp-cli history` — Buying history (won, lost, paid) over a configurable window.
- `ebay-pp-cli saved-search` — Local saved-search CRUD.


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
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.25+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/ebay/cmd/ebay-pp-cli@latest
   ```
3. Verify: `ebay-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

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
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ebay-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ebay-pp-cli <command> --help`.
