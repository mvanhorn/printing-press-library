# Kalshi CLI

**Trade prediction markets, persist tick data, and answer category-level P&L questions Kalshi.com cannot.**

Every Kalshi feature, plus a local SQLite store that records market prices on every sync — so movers, correlations, and price history work offline. Honest about read-only vs read/write keys, with --dry-run on every mutator and a global --read-only safe-mode lock for newsroom and bot deployments.

## Install

### Binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/kalshi-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

### Go

```
go install github.com/mvanhorn/printing-press-library/library/other/kalshi/cmd/kalshi-pp-cli@latest
```

## Authentication

Kalshi requires composed RSA-PSS signature auth: a UUID access key id (KALSHI_API_KEY) plus an RSA private key file (KALSHI_PRIVATE_KEY_PATH or KALSHI_PRIVATE_KEY). Kalshi issues two key tiers — read-only and read/write — and the CLI honors KALSHI_READ_ONLY=1 (or --read-only) as a client-side lock that blocks every POST/PUT/PATCH/DELETE before signing, regardless of which tier is loaded. Write commands run against a read-only key will surface a 403 from Kalshi; pair with --dry-run while debugging.

## Quick Start

```bash
# verify your key id + private key are loaded; reports configured/source
kalshi-pp-cli auth status


# populate the local SQLite store including the market_price_history snapshot table
kalshi-pp-cli sync


# browse open markets from the live API (sync first for offline filtering)
kalshi-pp-cli markets get --status open --limit 10 --json


# render the captured price history with an inline sparkline
kalshi-pp-cli markets history KXPRES-2028-DJT --sparkline


# compute realized P&L by Kalshi taxonomy
kalshi-pp-cli portfolio attribution --by category --since 2026-01-01 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`portfolio attribution`** — See your P&L broken down by market category and series over any time period
- **`markets history`** — Track how market odds moved over time with price progression charts
- **`portfolio winrate`** — Calculate your win/loss ratio, expected value, and ROI across all settled positions
- **`portfolio calendar`** — See upcoming settlements with your positions, expected payouts, and category breakdown
- **`markets movers`** — Find markets with the biggest price swings since your last sync
- **`markets correlate`** — Compare price histories of two markets to discover correlated events
- **`portfolio exposure`** — See your total risk broken down by category, with concentration warnings
- **`portfolio stale`** — Find positions in markets approaching expiry where you haven't acted recently

## Usage

Run `kalshi-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Manage account

- **`kalshi-pp-cli account get-api-limits`** - Endpoint to retrieve the API tier limits associated with the authenticated user.
- **`kalshi-pp-cli account get-endpoint-costs`** - Lists API v2 endpoints whose configured token cost differs from the default cost. Endpoints that use the default cost are omitted.

### api-keys

API key management endpoints

- **`kalshi-pp-cli api-keys create`** - Endpoint for creating a new API key with a user-provided public key.  This endpoint allows users with Premier or Market Maker API usage levels to create API keys by providing their own RSA public key. The platform will use this public key to verify signatures on API requests.
- **`kalshi-pp-cli api-keys delete`** - Endpoint for deleting an existing API key.  This endpoint permanently deletes an API key. Once deleted, the key can no longer be used for authentication. This action cannot be undone.
- **`kalshi-pp-cli api-keys generate`** - Endpoint for generating a new API key with an automatically created key pair.  This endpoint generates both a public and private RSA key pair. The public key is stored on the platform, while the private key is returned to the user and must be stored securely. The private key cannot be retrieved again.
- **`kalshi-pp-cli api-keys get`** - Endpoint for retrieving all API keys associated with the authenticated user.  API keys allow programmatic access to the platform without requiring username/password authentication. Each key has a unique identifier and name.

### communications

Request-for-quote (RFQ) endpoints

- **`kalshi-pp-cli communications accept-quote`** - Endpoint for accepting a quote. This will require the quoter to confirm
- **`kalshi-pp-cli communications confirm-quote`** - Endpoint for confirming a quote. This will start a timer for order execution
- **`kalshi-pp-cli communications create-quote`** - Endpoint for creating a quote in response to an RFQ
- **`kalshi-pp-cli communications create-rfq`** - Endpoint for creating a new RFQ. You can have a maximum of 100 open RFQs at a time.
- **`kalshi-pp-cli communications delete-quote`** - Endpoint for deleting a quote, which means it can no longer be accepted.
- **`kalshi-pp-cli communications delete-rfq`** - Endpoint for deleting an RFQ by ID
- **`kalshi-pp-cli communications get-id`** - Endpoint for getting the communications ID of the logged-in user.
- **`kalshi-pp-cli communications get-quote`** - Endpoint for getting a particular quote
- **`kalshi-pp-cli communications get-quotes`** - Endpoint for getting quotes
- **`kalshi-pp-cli communications get-rfq`** - Endpoint for getting a single RFQ by id
- **`kalshi-pp-cli communications get-rfqs`** - Endpoint for getting RFQs

### events

Event endpoints

- **`kalshi-pp-cli events get`** - Get all events. This endpoint excludes multivariate events.
To retrieve multivariate events, use the GET /events/multivariate endpoint.
All events are accessible through this endpoint, even if their associated markets are older than the historical cutoff.
- **`kalshi-pp-cli events get-eventticker`** - Endpoint for getting data about an event by its ticker. An event represents a real-world occurrence that can be traded on, such as an election, sports game, or economic indicator release.
Events contain one or more markets where users can place trades on different outcomes.
All events are accessible through this endpoint, even if their associated markets are older than the historical cutoff.
- **`kalshi-pp-cli events get-multivariate`** - Retrieve multivariate (combo) events. These are dynamically created events from multivariate event collections. Supports filtering by series and collection ticker.

### exchange

Exchange status and information endpoints

- **`kalshi-pp-cli exchange get-announcements`** - Endpoint for getting all exchange-wide announcements.
- **`kalshi-pp-cli exchange get-schedule`** - Endpoint for getting the exchange schedule.
- **`kalshi-pp-cli exchange get-status`** - Endpoint for getting the exchange status.
- **`kalshi-pp-cli exchange get-user-data-timestamp`** - There is typically a short delay before exchange events are reflected in the API endpoints. Whenever possible, combine API responses to PUT/POST/DELETE requests with websocket data to obtain the most accurate view of the exchange state. This endpoint provides an approximate indication of when the data from the following endpoints was last validated: GetBalance, GetOrder(s), GetFills, GetPositions

### fcm

FCM member specific endpoints

- **`kalshi-pp-cli fcm get-fcmorders`** - Endpoint for FCM members to get orders filtered by subtrader ID.
This endpoint requires FCM member access level and allows filtering orders by subtrader ID.
- **`kalshi-pp-cli fcm get-fcmpositions`** - Endpoint for FCM members to get market positions filtered by subtrader ID.
This endpoint requires FCM member access level and allows filtering positions by subtrader ID.

### historical

Manage historical

- **`kalshi-pp-cli historical get-cutoff`** - Returns the cutoff timestamps that define the boundary between **live** and **historical** data.

## Cutoff fields
- `market_settled_ts` : Markets that **settled** before this timestamp, and their candlesticks, must be accessed via `GET /historical/markets` and `GET /historical/markets/{ticker}/candlesticks`.
- `trades_created_ts` : Trades that were **filled** before this timestamp must be accessed via `GET /historical/fills`.
- `orders_updated_ts` : Orders that were **canceled or fully executed** before this timestamp must be accessed via `GET /historical/orders`. Resting (active) orders are always available in `GET /portfolio/orders`.
- **`kalshi-pp-cli historical get-fills`** - Endpoint for getting all historical fills for the member. A fill is when a trade you have is matched.
- **`kalshi-pp-cli historical get-market`** - Endpoint for getting data about a specific market by its ticker from the historical database.
- **`kalshi-pp-cli historical get-market-candlesticks`** - Endpoint for fetching historical candlestick data for markets that have been archived from the live data set. Time period length of each candlestick in minutes. Valid values: 1 (1 minute), 60 (1 hour), 1440 (1 day).
- **`kalshi-pp-cli historical get-markets`** - Endpoint for getting markets that have been archived to the historical database. Filters are mutually exclusive.
- **`kalshi-pp-cli historical get-orders`** - Endpoint for getting orders that have been archived to the historical database.
- **`kalshi-pp-cli historical get-trades`** - Endpoint for getting all historical trades for all markets. Trades that were filled before the historical cutoff are available via this endpoint. See [Historical Data](https://kalshi.com/docs/getting_started/historical_data) for details.

### incentive-programs

Incentive program endpoints

- **`kalshi-pp-cli incentive-programs get`** - List incentives with optional filters. Incentives are rewards programs for trading activity on specific markets.

### kalshi-trade-manual-search

Manage kalshi trade manual search

- **`kalshi-pp-cli kalshi-trade-manual-search get-filters-for-sports`** - Retrieve available filters organized by sport.

This endpoint returns filtering options available for each sport, including scopes and competitions. It also provides an ordered list of sports for display purposes.

### kalshi-trade-manual-search-2

Manage kalshi trade manual search 2

- **`kalshi-pp-cli kalshi-trade-manual-search-2 get-tags-for-series-categories`** - Retrieve tags organized by series categories.

This endpoint returns a mapping of series categories to their associated tags, which can be used for filtering and search functionality.

### live-data

Live data endpoints

- **`kalshi-pp-cli live-data get`** - Get live data for multiple milestones
- **`kalshi-pp-cli live-data get-by-milestone`** - Get live data for a specific milestone.
- **`kalshi-pp-cli live-data get-game-stats`** - Get play-by-play game statistics for a specific milestone. Supported sports: Pro Football, College Football, Pro Basketball, College Men's Basketball, College Women's Basketball, WNBA, Soccer, Pro Hockey, and Pro Baseball. Returns null for unsupported milestone types or milestones without a Sportradar ID.

### markets

Market data endpoints

- **`kalshi-pp-cli markets batch-get-candlesticks`** - Endpoint for retrieving candlestick data for multiple markets.

- Accepts up to 100 market tickers per request
- Returns up to 10,000 candlesticks total across all markets
- Returns candlesticks grouped by market_id
- Optionally includes a synthetic initial candlestick for price continuity (see `include_latest_before_start` parameter)
- **`kalshi-pp-cli markets get`** - Filter by market status. Possible values: `unopened`, `open`, `closed`, `settled`. Leave empty to return markets with any status.
 - Only one `status` filter may be supplied at a time.
 - Timestamp filters will be mutually exclusive from other timestamp filters and certain status filters.

 | Compatible Timestamp Filters | Additional Status Filters| Extra Notes |
 |------------------------------|--------------------------|-------------|
 | min_created_ts, max_created_ts | `unopened`, `open`, *empty* | |
 | min_close_ts, max_close_ts | `closed`, *empty* | |
 | min_settled_ts, max_settled_ts | `settled`, *empty* | |
 | min_updated_ts | *empty* | Incompatible with all filters besides `mve_filter=exclude` |

 Markets that settled before the historical cutoff are only available via `GET /historical/markets`. See [Historical Data](https://kalshi.com/docs/getting_started/historical_data) for details.
- **`kalshi-pp-cli markets get-orderbooks`** - Endpoint for getting the current order books for multiple markets in a single request. The order book shows all active bid orders for both yes and no sides of a binary market. It returns yes bids and no bids only (no asks are returned). This is because in binary markets, a bid for yes at price X is equivalent to an ask for no at price (100-X). For example, a yes bid at 7¢ is the same as a no ask at 93¢, with identical contract sizes. Each side shows price levels with their corresponding quantities and order counts, organized from best to worst prices. Returns one orderbook per requested market ticker.
- **`kalshi-pp-cli markets get-ticker`** - Endpoint for getting data about a specific market by its ticker. A market represents a specific binary outcome within an event that users can trade on (e.g., "Will candidate X win?"). Markets have yes/no positions, current prices, volume, and settlement rules.
- **`kalshi-pp-cli markets get-trades`** - Endpoint for getting all trades for all markets. A trade represents a completed transaction between two users on a specific market. Each trade includes the market ticker, price, quantity, and timestamp information. This endpoint returns a paginated response. Use the 'limit' parameter to control page size (1-1000, defaults to 100). The response includes a 'cursor' field - pass this value in the 'cursor' parameter of your next request to get the next page. An empty cursor indicates no more pages are available.

### milestones

Milestone endpoints

- **`kalshi-pp-cli milestones get`** - Minimum start date to filter milestones. Format: RFC3339 timestamp
- **`kalshi-pp-cli milestones get-milestoneid`** - Endpoint for getting data about a specific milestone by its ID.

### multivariate-event-collections

Manage multivariate event collections

- **`kalshi-pp-cli multivariate-event-collections create-market-in`** - Endpoint for creating an individual market in a multivariate event collection. This endpoint must be hit at least once before trading or looking up a market. Users are limited to 5000 creations per week.
- **`kalshi-pp-cli multivariate-event-collections get`** - Endpoint for getting data about multivariate event collections.
- **`kalshi-pp-cli multivariate-event-collections get-multivariateeventcollections`** - Endpoint for getting data about a multivariate event collection by its ticker.

### portfolio

Portfolio and balance information endpoints

- **`kalshi-pp-cli portfolio amend-order`** - Endpoint for amending the max number of fillable contracts and/or price in an existing order. Max fillable contracts is `remaining_count` + `fill_count`.
- **`kalshi-pp-cli portfolio amend-order-v2`** - Endpoint for amending the price and/or remaining count of an existing event-market order using the V2 request/response shape.
- **`kalshi-pp-cli portfolio apply-subaccount-transfer`** - Transfers funds between the authenticated user's subaccounts. Use 0 for the primary account, or 1-32 for numbered subaccounts.
- **`kalshi-pp-cli portfolio batch-cancel-orders`** - Endpoint for cancelling a batch of orders. The maximum batch size scales with your tier's write budget — see [Rate Limits and Tiers](/getting_started/rate_limits).
- **`kalshi-pp-cli portfolio batch-cancel-orders-v2`** - Endpoint for cancelling a batch of event-market orders using the V2 response shape. The maximum batch size scales with your tier's write budget — see [Rate Limits and Tiers](/getting_started/rate_limits).
- **`kalshi-pp-cli portfolio batch-create-orders`** - Endpoint for submitting a batch of orders. The maximum batch size scales with your tier's write budget — see [Rate Limits and Tiers](/getting_started/rate_limits).
- **`kalshi-pp-cli portfolio batch-create-orders-v2`** - Endpoint for submitting a batch of event-market orders using the V2 request/response shape. The maximum batch size scales with your tier's write budget — see [Rate Limits and Tiers](/getting_started/rate_limits).
- **`kalshi-pp-cli portfolio cancel-order`** - Endpoint for canceling orders. The value for the orderId should match the id field of the order you want to decrease. Commonly, DELETE-type endpoints return 204 status with no body content on success. But we can't completely delete the order, as it may be partially filled already. Instead, the DeleteOrder endpoint reduce the order completely, essentially zeroing the remaining resting contracts on it. The zeroed order is returned on the response payload as a form of validation for the client.
- **`kalshi-pp-cli portfolio cancel-order-v2`** - Endpoint for cancelling event-market orders using the V2 response shape. Returns `{order_id, client_order_id, reduced_by}` rather than a full order object.
- **`kalshi-pp-cli portfolio create-order`** - Endpoint for submitting orders in a market. Each user is limited to 200 000 open orders at a time.
- **`kalshi-pp-cli portfolio create-order-group`** - Creates a new order group with a contracts limit measured over a rolling 15-second window. When the limit is hit, all orders in the group are cancelled and no new orders can be placed until reset.
- **`kalshi-pp-cli portfolio create-order-v2`** - Endpoint for submitting event-market orders using the V2 request/response shape (single-book `bid`/`ask` side and fixed-point dollar prices). The legacy `/portfolio/orders` endpoint will be deprecated no earlier than May 6, 2026 — clients should migrate to this path.
- **`kalshi-pp-cli portfolio create-subaccount`** - Creates a new subaccount for the authenticated user. Subaccounts are numbered sequentially starting from 1. Maximum 32 subaccounts per user.
- **`kalshi-pp-cli portfolio decrease-order`** - Endpoint for decreasing the number of contracts in an existing order. This is the only kind of edit available on order quantity. Cancelling an order is equivalent to decreasing an order amount to zero.
- **`kalshi-pp-cli portfolio decrease-order-v2`** - Endpoint for decreasing the remaining count of an existing event-market order using the V2 request/response shape. Only `reduce_to` is supported.
- **`kalshi-pp-cli portfolio delete-order-group`** - Deletes an order group and cancels all orders within it. This permanently removes the group.
- **`kalshi-pp-cli portfolio get-balance`** - Endpoint for getting the balance and portfolio value of a member. Both values are returned in cents.
- **`kalshi-pp-cli portfolio get-fills`** - Endpoint for getting all fills for the member. A fill is when a trade you have is matched.
Fills that occurred before the historical cutoff are only available via `GET /historical/fills`. See [Historical Data](https://kalshi.com/docs/getting_started/historical_data) for details.
- **`kalshi-pp-cli portfolio get-order`** - Endpoint for getting a single order.
- **`kalshi-pp-cli portfolio get-order-group`** - Retrieves details for a single order group including all order IDs and auto-cancel status.
- **`kalshi-pp-cli portfolio get-order-groups`** - Retrieves all order groups for the authenticated user.
- **`kalshi-pp-cli portfolio get-order-queue-position`** - Endpoint for getting an order's queue position in the order book. This represents the amount of orders that need to be matched before this order receives a partial or full match. Queue position is determined using a price-time priority.
- **`kalshi-pp-cli portfolio get-order-queue-positions`** - Endpoint for getting queue positions for all resting orders. Queue position represents the number of contracts that need to be matched before an order receives a partial or full match, determined using price-time priority.
- **`kalshi-pp-cli portfolio get-orders`** - Restricts the response to orders that have a certain status: resting, canceled, or executed.
Orders that have been canceled or fully executed before the historical cutoff are only available via `GET /historical/orders`. Resting orders will always be available through this endpoint. See [Historical Data](https://kalshi.com/docs/getting_started/historical_data) for details.
- **`kalshi-pp-cli portfolio get-positions`** - Restricts the positions to those with any of following fields with non-zero values, as a comma separated list. The following values are accepted: position, total_traded
- **`kalshi-pp-cli portfolio get-resting-order-total-value`** - Endpoint for getting the total value, in cents, of resting orders. This endpoint is only intended for use by FCM members (rare). Note: If you're uncertain about this endpoint, it likely does not apply to you.
- **`kalshi-pp-cli portfolio get-settlements`** - Endpoint for getting the member's settlements historical track.
- **`kalshi-pp-cli portfolio get-subaccount-balances`** - Gets balances for all subaccounts including the primary account.
- **`kalshi-pp-cli portfolio get-subaccount-netting`** - Gets the netting enabled settings for all subaccounts.
- **`kalshi-pp-cli portfolio get-subaccount-transfers`** - Gets a paginated list of all transfers between subaccounts for the authenticated user.
- **`kalshi-pp-cli portfolio reset-order-group`** - Resets the order group's matched contracts counter to zero, allowing new orders to be placed again after the limit was hit.
- **`kalshi-pp-cli portfolio trigger-order-group`** - Triggers the order group, canceling all orders in the group and preventing new orders until the group is reset.
- **`kalshi-pp-cli portfolio update-order-group-limit`** - Updates the order group contracts limit (rolling 15-second window). If the updated limit would immediately trigger the group, all orders in the group are canceled and the group is triggered.
- **`kalshi-pp-cli portfolio update-subaccount-netting`** - Updates the netting enabled setting for a specific subaccount. Use 0 for the primary account, or 1-32 for numbered subaccounts.

### series

Manage series

- **`kalshi-pp-cli series get`** - Endpoint for getting data about a specific series by its ticker.  A series represents a template for recurring events that follow the same format and rules (e.g., "Monthly Jobs Report", "Weekly Initial Jobless Claims", "Daily Weather in NYC"). Series define the structure, settlement sources, and metadata that will be applied to each recurring event instance within that series.
- **`kalshi-pp-cli series get-fee-changes`** - Get Series Fee Changes
- **`kalshi-pp-cli series get-list`** - Endpoint for getting data about multiple series with specified filters.  A series represents a template for recurring events that follow the same format and rules (e.g., "Monthly Jobs Report", "Weekly Initial Jobless Claims", "Daily Weather in NYC"). This endpoint allows you to browse and discover available series templates by category.

### structured-targets

Structured targets endpoints

- **`kalshi-pp-cli structured-targets get`** - Page size (min: 1, max: 2000)
- **`kalshi-pp-cli structured-targets get-structuredtargets`** - Endpoint for getting data about a specific structured target by its ID.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
kalshi-pp-cli api-keys get

# JSON for scripting and agents
kalshi-pp-cli api-keys get --json

# Filter to specific fields
kalshi-pp-cli api-keys get --json --select id,name,status

# Dry run — show the request without sending
kalshi-pp-cli api-keys get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
kalshi-pp-cli api-keys get --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-kalshi -g
```

Then invoke `/pp-kalshi <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/kalshi/cmd/kalshi-pp-mcp@latest
```

Then register it:

```bash
claude mcp add kalshi kalshi-pp-mcp -e KALSHI_API_KEY=<your-key>
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-kalshi --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-kalshi --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-kalshi skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-kalshi. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/kalshi-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `KALSHI_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/kalshi/cmd/kalshi-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "kalshi": {
      "command": "kalshi-pp-mcp",
      "env": {
        "KALSHI_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
kalshi-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/kalshi-pp-cli/config.toml`

Environment variables:
- `KALSHI_API_KEY`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `kalshi-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $KALSHI_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports `auth_status: missing private key`** — export KALSHI_PRIVATE_KEY_PATH=~/.kalshi/private_key.pem (RSA PEM from Kalshi.com → Account → API Keys)
- **Loaded a read-only key but want to script safely** — export KALSHI_READ_ONLY=1 (or pass --read-only) — the client blocks every POST/PUT/PATCH/DELETE before signing
- **portfolio create-order returns 403 Permission Denied** — your loaded API key is read-only. Issue a read/write key from Kalshi → Account → API Keys, or run write commands with --dry-run while debugging
- **markets movers returns no rows or no movement** — run kalshi-pp-cli sync at least twice an hour apart — movers reads from market_price_history snapshots
- **rate-limit errors during sync** — run kalshi-pp-cli account get-api-limits --json to inspect your tier; tune --workers down

---

## Known Gaps

These are documented limitations that ship with this version. They do not affect day-to-day use; they only surface in synthetic test environments.

- **Sync warnings on bare-path resources.** A handful of resource paths (e.g., `/account`, `/api-keys`, `/communications`) emit HTTP 404 warnings during `sync` because Kalshi exposes them under nested paths (e.g., `/portfolio/account`). Sync correctly skips them and continues with usable resources (markets, events, series, milestones, incentive-programs). This is a generator-level naming gap; the affected commands work normally when called directly.
- **Write-side endpoint mirrors require valid bodies.** Commands like `api-keys create`, `communications create-quote`, `portfolio batch-create-orders`, `portfolio apply-subaccount-transfer` need valid Kalshi-domain request bodies (real market tickers, valid pricing, etc.). Calling them with placeholder values returns Kalshi's HTTP 400/404. This is expected — these endpoints work when given real input.
- **Auth tier detection is client-side only.** The CLI does not actively probe whether your loaded API key is read-only or read/write. Use `KALSHI_READ_ONLY=1` or `--read-only` to enforce a client-side lock; the API will return 403 on writes if your key tier is read-only.
- **Resource name pollution.** Two paths (`/search/filters_by_sport` and `/search/tags_by_categories`) collided with the framework `search` command and were renamed to `kalshi-trade-manual-search` and `kalshi-trade-manual-search-2`. These are intentionally ugly to surface the collision.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**OctagonAI/kalshi-deep-trading-bot**](https://github.com/OctagonAI/kalshi-deep-trading-bot) — TypeScript (168 stars)
- [**austron24/kalshi-cli**](https://github.com/austron24/kalshi-cli) — Python (14 stars)
- [**newyorkcompute/kalshi**](https://github.com/newyorkcompute/kalshi) — TypeScript (3 stars)
- [**fsctl/go-kalshi**](https://github.com/fsctl/go-kalshi) — Go (2 stars)
- [**JThomasDevs/kalshi-cli**](https://github.com/JThomasDevs/kalshi-cli) — Python (1 stars)
- [**9crusher/mcp-server-kalshi**](https://github.com/9crusher/mcp-server-kalshi) — TypeScript
- [**yakub268/kalshi-mcp**](https://github.com/yakub268/kalshi-mcp) — TypeScript
- [**JamesANZ/prediction-market-mcp**](https://github.com/JamesANZ/prediction-market-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
