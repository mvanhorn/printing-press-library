---
name: pp-kalshi
description: "Trade prediction markets, persist tick data, and answer category-level P&L questions Kalshi.com cannot. Trigger phrases: `kalshi market price`, `track prediction market`, `kalshi portfolio P&L`, `kalshi correlate markets`, `use kalshi`, `run kalshi`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - kalshi-pp-cli
    install:
      - kind: go
        bins: [kalshi-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/kalshi/cmd/kalshi-pp-cli
---

# Kalshi — Printing Press CLI

Every Kalshi feature, plus a local SQLite store that records market prices on every sync — so movers, correlations, and price history work offline. Honest about read-only vs read/write keys, with --dry-run on every mutator and a global --read-only safe-mode lock for newsroom and bot deployments.

## When to Use This CLI

Reach for kalshi-pp-cli when an agent needs price-over-time or category-level analytics on Kalshi markets — the public API only returns current prices and flat positions, so historical and aggregated questions require the local snapshot store. Use it for daily portfolio reconciliation, signal discovery via correlation across markets, and safe scripting against read-only API credentials.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`portfolio attribution`** — See your P&L broken down by market category and series over any time period
- **`markets history`** — Track how market odds moved over time with price progression charts
- **`portfolio winrate`** — Calculate your win/loss ratio, expected value, and ROI across all settled positions
- **`portfolio calendar`** — See upcoming settlements with your positions, expected payouts, and category breakdown
- **`markets movers`** — Find markets with the biggest price swings since your last sync
- **`markets correlate`** — Compare price histories of two markets to discover correlated events
- **`portfolio exposure`** — See your total risk broken down by category, with concentration warnings
- **`portfolio stale`** — Find positions in markets approaching expiry where you haven't acted recently

## Command Reference

**account** — Manage account

- `kalshi-pp-cli account get-api-limits` — Endpoint to retrieve the API tier limits associated with the authenticated user.
- `kalshi-pp-cli account get-endpoint-costs` — Lists API v2 endpoints whose configured token cost differs from the default cost. Endpoints that use the default...

**api-keys** — API key management endpoints

- `kalshi-pp-cli api-keys create` — Endpoint for creating a new API key with a user-provided public key. This endpoint allows users with Premier or...
- `kalshi-pp-cli api-keys delete` — Endpoint for deleting an existing API key. This endpoint permanently deletes an API key. Once deleted, the key can...
- `kalshi-pp-cli api-keys generate` — Endpoint for generating a new API key with an automatically created key pair. This endpoint generates both a public...
- `kalshi-pp-cli api-keys get` — Endpoint for retrieving all API keys associated with the authenticated user. API keys allow programmatic access to...

**communications** — Request-for-quote (RFQ) endpoints

- `kalshi-pp-cli communications accept-quote` — Endpoint for accepting a quote. This will require the quoter to confirm
- `kalshi-pp-cli communications confirm-quote` — Endpoint for confirming a quote. This will start a timer for order execution
- `kalshi-pp-cli communications create-quote` — Endpoint for creating a quote in response to an RFQ
- `kalshi-pp-cli communications create-rfq` — Endpoint for creating a new RFQ. You can have a maximum of 100 open RFQs at a time.
- `kalshi-pp-cli communications delete-quote` — Endpoint for deleting a quote, which means it can no longer be accepted.
- `kalshi-pp-cli communications delete-rfq` — Endpoint for deleting an RFQ by ID
- `kalshi-pp-cli communications get-id` — Endpoint for getting the communications ID of the logged-in user.
- `kalshi-pp-cli communications get-quote` — Endpoint for getting a particular quote
- `kalshi-pp-cli communications get-quotes` — Endpoint for getting quotes
- `kalshi-pp-cli communications get-rfq` — Endpoint for getting a single RFQ by id
- `kalshi-pp-cli communications get-rfqs` — Endpoint for getting RFQs

**events** — Event endpoints

- `kalshi-pp-cli events get` — Get all events. This endpoint excludes multivariate events. To retrieve multivariate events, use the GET...
- `kalshi-pp-cli events get-eventticker` — Endpoint for getting data about an event by its ticker. An event represents a real-world occurrence that can be...
- `kalshi-pp-cli events get-multivariate` — Retrieve multivariate (combo) events. These are dynamically created events from multivariate event collections....

**exchange** — Exchange status and information endpoints

- `kalshi-pp-cli exchange get-announcements` — Endpoint for getting all exchange-wide announcements.
- `kalshi-pp-cli exchange get-schedule` — Endpoint for getting the exchange schedule.
- `kalshi-pp-cli exchange get-status` — Endpoint for getting the exchange status.
- `kalshi-pp-cli exchange get-user-data-timestamp` — There is typically a short delay before exchange events are reflected in the API endpoints. Whenever possible,...

**fcm** — FCM member specific endpoints

- `kalshi-pp-cli fcm get-fcmorders` — Endpoint for FCM members to get orders filtered by subtrader ID. This endpoint requires FCM member access level and...
- `kalshi-pp-cli fcm get-fcmpositions` — Endpoint for FCM members to get market positions filtered by subtrader ID. This endpoint requires FCM member access...

**historical** — Manage historical

- `kalshi-pp-cli historical get-cutoff` — Returns the cutoff timestamps that define the boundary between **live** and **historical** data. ## Cutoff fields -...
- `kalshi-pp-cli historical get-fills` — Endpoint for getting all historical fills for the member. A fill is when a trade you have is matched.
- `kalshi-pp-cli historical get-market` — Endpoint for getting data about a specific market by its ticker from the historical database.
- `kalshi-pp-cli historical get-market-candlesticks` — Endpoint for fetching historical candlestick data for markets that have been archived from the live data set. Time...
- `kalshi-pp-cli historical get-markets` — Endpoint for getting markets that have been archived to the historical database. Filters are mutually exclusive.
- `kalshi-pp-cli historical get-orders` — Endpoint for getting orders that have been archived to the historical database.
- `kalshi-pp-cli historical get-trades` — Endpoint for getting all historical trades for all markets. Trades that were filled before the historical cutoff are...

**incentive-programs** — Incentive program endpoints

- `kalshi-pp-cli incentive-programs` — List incentives with optional filters. Incentives are rewards programs for trading activity on specific markets.

**kalshi-trade-manual-search** — Manage kalshi trade manual search

- `kalshi-pp-cli kalshi-trade-manual-search` — Retrieve available filters organized by sport. This endpoint returns filtering options available for each sport,...

**kalshi-trade-manual-search-2** — Manage kalshi trade manual search 2

- `kalshi-pp-cli kalshi-trade-manual-search-2` — Retrieve tags organized by series categories. This endpoint returns a mapping of series categories to their...

**live-data** — Live data endpoints

- `kalshi-pp-cli live-data get` — Get live data for multiple milestones
- `kalshi-pp-cli live-data get-by-milestone` — Get live data for a specific milestone.
- `kalshi-pp-cli live-data get-game-stats` — Get play-by-play game statistics for a specific milestone. Supported sports: Pro Football, College Football, Pro...

**markets** — Market data endpoints

- `kalshi-pp-cli markets batch-get-candlesticks` — Endpoint for retrieving candlestick data for multiple markets. - Accepts up to 100 market tickers per request -...
- `kalshi-pp-cli markets get` — Filter by market status. Possible values: `unopened`, `open`, `closed`, `settled`. Leave empty to return markets...
- `kalshi-pp-cli markets get-orderbooks` — Endpoint for getting the current order books for multiple markets in a single request. The order book shows all...
- `kalshi-pp-cli markets get-ticker` — Endpoint for getting data about a specific market by its ticker. A market represents a specific binary outcome...
- `kalshi-pp-cli markets get-trades` — Endpoint for getting all trades for all markets. A trade represents a completed transaction between two users on a...

**milestones** — Milestone endpoints

- `kalshi-pp-cli milestones get` — Minimum start date to filter milestones. Format: RFC3339 timestamp
- `kalshi-pp-cli milestones get-milestoneid` — Endpoint for getting data about a specific milestone by its ID.

**multivariate-event-collections** — Manage multivariate event collections

- `kalshi-pp-cli multivariate-event-collections create-market-in` — Endpoint for creating an individual market in a multivariate event collection. This endpoint must be hit at least...
- `kalshi-pp-cli multivariate-event-collections get` — Endpoint for getting data about multivariate event collections.
- `kalshi-pp-cli multivariate-event-collections get-multivariateeventcollections` — Endpoint for getting data about a multivariate event collection by its ticker.

**portfolio** — Portfolio and balance information endpoints

- `kalshi-pp-cli portfolio amend-order` — Endpoint for amending the max number of fillable contracts and/or price in an existing order. Max fillable contracts...
- `kalshi-pp-cli portfolio amend-order-v2` — Endpoint for amending the price and/or remaining count of an existing event-market order using the V2...
- `kalshi-pp-cli portfolio apply-subaccount-transfer` — Transfers funds between the authenticated user's subaccounts. Use 0 for the primary account, or 1-32 for numbered...
- `kalshi-pp-cli portfolio batch-cancel-orders` — Endpoint for cancelling a batch of orders. The maximum batch size scales with your tier's write budget — see [Rate...
- `kalshi-pp-cli portfolio batch-cancel-orders-v2` — Endpoint for cancelling a batch of event-market orders using the V2 response shape. The maximum batch size scales...
- `kalshi-pp-cli portfolio batch-create-orders` — Endpoint for submitting a batch of orders. The maximum batch size scales with your tier's write budget — see [Rate...
- `kalshi-pp-cli portfolio batch-create-orders-v2` — Endpoint for submitting a batch of event-market orders using the V2 request/response shape. The maximum batch size...
- `kalshi-pp-cli portfolio cancel-order` — Endpoint for canceling orders. The value for the orderId should match the id field of the order you want to...
- `kalshi-pp-cli portfolio cancel-order-v2` — Endpoint for cancelling event-market orders using the V2 response shape. Returns `{order_id, client_order_id,...
- `kalshi-pp-cli portfolio create-order` — Endpoint for submitting orders in a market. Each user is limited to 200 000 open orders at a time.
- `kalshi-pp-cli portfolio create-order-group` — Creates a new order group with a contracts limit measured over a rolling 15-second window. When the limit is hit,...
- `kalshi-pp-cli portfolio create-order-v2` — Endpoint for submitting event-market orders using the V2 request/response shape (single-book `bid`/`ask` side and...
- `kalshi-pp-cli portfolio create-subaccount` — Creates a new subaccount for the authenticated user. Subaccounts are numbered sequentially starting from 1. Maximum...
- `kalshi-pp-cli portfolio decrease-order` — Endpoint for decreasing the number of contracts in an existing order. This is the only kind of edit available on...
- `kalshi-pp-cli portfolio decrease-order-v2` — Endpoint for decreasing the remaining count of an existing event-market order using the V2 request/response shape....
- `kalshi-pp-cli portfolio delete-order-group` — Deletes an order group and cancels all orders within it. This permanently removes the group.
- `kalshi-pp-cli portfolio get-balance` — Endpoint for getting the balance and portfolio value of a member. Both values are returned in cents.
- `kalshi-pp-cli portfolio get-fills` — Endpoint for getting all fills for the member. A fill is when a trade you have is matched. Fills that occurred...
- `kalshi-pp-cli portfolio get-order` — Endpoint for getting a single order.
- `kalshi-pp-cli portfolio get-order-group` — Retrieves details for a single order group including all order IDs and auto-cancel status.
- `kalshi-pp-cli portfolio get-order-groups` — Retrieves all order groups for the authenticated user.
- `kalshi-pp-cli portfolio get-order-queue-position` — Endpoint for getting an order's queue position in the order book. This represents the amount of orders that need to...
- `kalshi-pp-cli portfolio get-order-queue-positions` — Endpoint for getting queue positions for all resting orders. Queue position represents the number of contracts that...
- `kalshi-pp-cli portfolio get-orders` — Restricts the response to orders that have a certain status: resting, canceled, or executed. Orders that have been...
- `kalshi-pp-cli portfolio get-positions` — Restricts the positions to those with any of following fields with non-zero values, as a comma separated list. The...
- `kalshi-pp-cli portfolio get-resting-order-total-value` — Endpoint for getting the total value, in cents, of resting orders. This endpoint is only intended for use by FCM...
- `kalshi-pp-cli portfolio get-settlements` — Endpoint for getting the member's settlements historical track.
- `kalshi-pp-cli portfolio get-subaccount-balances` — Gets balances for all subaccounts including the primary account.
- `kalshi-pp-cli portfolio get-subaccount-netting` — Gets the netting enabled settings for all subaccounts.
- `kalshi-pp-cli portfolio get-subaccount-transfers` — Gets a paginated list of all transfers between subaccounts for the authenticated user.
- `kalshi-pp-cli portfolio reset-order-group` — Resets the order group's matched contracts counter to zero, allowing new orders to be placed again after the limit...
- `kalshi-pp-cli portfolio trigger-order-group` — Triggers the order group, canceling all orders in the group and preventing new orders until the group is reset.
- `kalshi-pp-cli portfolio update-order-group-limit` — Updates the order group contracts limit (rolling 15-second window). If the updated limit would immediately trigger...
- `kalshi-pp-cli portfolio update-subaccount-netting` — Updates the netting enabled setting for a specific subaccount. Use 0 for the primary account, or 1-32 for numbered...

**series** — Manage series

- `kalshi-pp-cli series get` — Endpoint for getting data about a specific series by its ticker. A series represents a template for recurring events...
- `kalshi-pp-cli series get-fee-changes` — Get Series Fee Changes
- `kalshi-pp-cli series get-list` — Endpoint for getting data about multiple series with specified filters. A series represents a template for recurring...

**structured-targets** — Structured targets endpoints

- `kalshi-pp-cli structured-targets get` — Page size (min: 1, max: 2000)
- `kalshi-pp-cli structured-targets get-structuredtargets` — Endpoint for getting data about a specific structured target by its ID.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
kalshi-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Politics-category P&L this quarter

```bash
kalshi-pp-cli portfolio attribution --since 2026-01-01 --by category --agent --select 'rows.category,rows.realized_pnl,rows.fills'
```

Realized P&L attributed to Kalshi taxonomy; --select narrows the response to the three fields agents need.

### Track the Fed-cut market on a watchlist

```bash
kalshi-pp-cli watch add KXFEDFUNDS-26FEB && kalshi-pp-cli watch diff --since 24h --agent
```

Add ticker to local watchlist, then ask for the per-ticker delta since yesterday.

### Correlate inflation and rate-cut markets

```bash
kalshi-pp-cli markets correlate KXFEDFUNDS-26FEB KXCPI-26FEB --window 30d --agent
```

Pearson r computed locally over snapshot price series for both markets.

### Safe paper-trading session

```bash
KALSHI_READ_ONLY=1 kalshi-pp-cli portfolio create-order --ticker KXTEST-2026 --side yes --count 1 --yes-price 50 --action buy --dry-run
```

Both safety floors engaged: client-side read-only lock + dry-run; never reaches the API.

### Movers in sports markets, last 24h

```bash
kalshi-pp-cli markets movers --window 24h --category sports --limit 5 --agent
```

Computes price deltas from market_price_history snapshots; works offline after a sync.

## Auth Setup

Kalshi requires composed RSA-PSS signature auth: a UUID access key id (KALSHI_API_KEY) plus an RSA private key file (KALSHI_PRIVATE_KEY_PATH or KALSHI_PRIVATE_KEY). Kalshi issues two key tiers — read-only and read/write — and the CLI honors KALSHI_READ_ONLY=1 (or --read-only) as a client-side lock that blocks every POST/PUT/PATCH/DELETE before signing, regardless of which tier is loaded. Write commands run against a read-only key will surface a 403 from Kalshi; pair with --dry-run while debugging.

Run `kalshi-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  kalshi-pp-cli api-keys get --agent --select id,name,status
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
kalshi-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
kalshi-pp-cli feedback --stdin < notes.txt
kalshi-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.kalshi-pp-cli/feedback.jsonl`. They are never POSTed unless `KALSHI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `KALSHI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
kalshi-pp-cli profile save briefing --json
kalshi-pp-cli --profile briefing api-keys get
kalshi-pp-cli profile list --json
kalshi-pp-cli profile show briefing
kalshi-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `kalshi-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/kalshi/cmd/kalshi-pp-cli@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/other/kalshi/cmd/kalshi-pp-cli@main
   ```
3. Verify: `kalshi-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/kalshi/cmd/kalshi-pp-mcp@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/other/kalshi/cmd/kalshi-pp-mcp@main
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add kalshi-pp-mcp -- kalshi-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which kalshi-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   kalshi-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `kalshi-pp-cli <command> --help`.
