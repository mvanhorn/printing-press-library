# Booking.com CLI

**Every Booking.com Demand API feature, plus offline search, price drift detection, and review-based property ranking that no other tool offers**

The only CLI for Booking.com's Demand API. Search accommodations, manage bookings, track price changes, and rank properties by review categories — all with offline SQLite storage, agent-native JSON output, and full MCP server support.

Printed by [@netnull](https://github.com/netnull) (netnull).

## Install

The recommended path installs both the `booking-com-demand-pp-cli` binary and the `pp-booking-com-demand` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install booking-com-demand
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install booking-com-demand --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/booking-com-demand-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-booking-com-demand --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-booking-com-demand --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-booking-com-demand skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-booking-com-demand. The skill defines how its required CLI can be installed.
```

## Authentication

Requires a Booking.com Demand API Bearer token and Affiliate ID. Get partner access at developers.booking.com. Set BOOKING_API_TOKEN and BOOKING_AFFILIATE_ID in your environment.

## Quick Start

```bash
# Configure your Bearer token and Affiliate ID
booking-com-demand-pp-cli auth set-token YOUR_TOKEN_HERE


# Verify API connectivity and auth
booking-com-demand-pp-cli doctor


# Search properties in the Netherlands
booking-com-demand-pp-cli accommodations search --country nl --checkin 2026-07-01 --checkout 2026-07-05


# Populate the local store for offline queries
booking-com-demand-pp-cli sync --full


# Rank by review quality and price
booking-com-demand-pp-cli reviews rank --destination Amsterdam --min-cleanliness 8 --sort price

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`accommodations drift`** — Flags properties where pricing moved beyond a threshold since your last sync

  _When monitoring property pricing for affiliate content or OTA inventory, this command replaces manual spreadsheet comparison with a single query_

  ```bash
  booking-com-demand-pp-cli accommodations drift --since 7d --threshold 10 --json
  ```
- **`reviews rank`** — Ranks properties by user-specified review sub-categories (cleanliness, location, staff) with price filtering

  _When curating top-N property lists for content or recommendations, this replaces hours of manual web browsing with a structured query_

  ```bash
  booking-com-demand-pp-cli reviews rank --destination Barcelona --min-cleanliness 8 --min-location 9 --sort price --json
  ```
- **`accommodations changelog`** — Shows field-level diffs for properties that changed since your last sync

  _When maintaining an OTA inventory, this command catches price and facility changes that would otherwise require manual API polling_

  ```bash
  booking-com-demand-pp-cli accommodations changelog --since 7d --fields price,facilities --json
  ```
- **`accommodations compare`** — Side-by-side comparison of multiple properties across facilities, review scores, and pricing

  _When narrowing down property options for a client or article, this replaces manual tab-switching with a structured diff_

  ```bash
  booking-com-demand-pp-cli accommodations compare --ids 123,456,789 --show facilities,scores,price --json
  ```
- **`locations intel`** — City-level analytics: property count, average scores, price ranges, top landmarks, district breakdown

  _When researching a new destination for content or travel planning, this produces a data-driven overview that replaces hours of browsing_

  ```bash
  booking-com-demand-pp-cli locations intel --city Barcelona --json
  ```

### Agent-native plumbing
- **`orders upcoming`** — Lists orders with imminent check-in dates, highlighting those with unresolved message threads

  _When managing active bookings, this single command replaces toggling between orders and messages screens_

  ```bash
  booking-com-demand-pp-cli orders upcoming --days 14 --with-messages --json
  ```

## Usage

Run `booking-com-demand-pp-cli --help` for the full command reference and flag list.

## Commands

### accommodations

This API collection is specific for the stay part of the connected trip. </br></br>Use these endpoints to search for stays such as hotels and apartments, check availability, retrieve reviews, and get detailed property information.

- **`booking-com-demand-pp-cli accommodations availability`** - Use this endpoint to return detailed product availability, price and charges of the accommodation matching a given search criteria. <br/>By default, only product availability and price is returned. To receive extended information use the `extras` parameter. <br/><br/>Note: It is mandatory to pass the input parameters: accommodation, booker, checkin, checkout and guest.
- **`booking-com-demand-pp-cli accommodations bulk-availability`** - Use this endpoint to retrieve detailed product availability, price and charges of a list of accommodations. By default, only product availability and price is returned. <br/>To receive extended information use the `extras` parameter. <br/>Note: It is mandatory to pass the input parameters: accommodations, booker, checkin, checkout and guests.
- **`booking-com-demand-pp-cli accommodations chains`** - Use this endpoint to retrieve a list of accommodation chains and their associated brands.<br/><br/> A chain-branded accommodation is part of a larger corporate group and operates under a recognised brand. The returned information can be used to filter search results by chain or brand across other endpoints. <br/> - To obtain the full list of chains, call this endpoint with an empty request body. <br/> - The `id` values returned are the codes used as input and output in other API requests, ensuring consistency across your integration. <br/> **Example of chain** - "Radisson Hotel Group" with brands such as "Radisson Blu" or "Park Inn by Radisson."
- **`booking-com-demand-pp-cli accommodations constants`** - Use this endpoint to retrieve standardised codes and names for accommodation-specific types, including facilities, room types, bed types, themes, and charges. <br/>These constants can be used to filter searches or populate reference data across other accommodation endpoints. <br/>You can request multiple languages using IETF language tags (see common/languages). <br/> Call with an empty body to retrieve all constants. The codes returned are canonical identifiers used across the API.
- **`booking-com-demand-pp-cli accommodations details`** - This endpoint returns detailed information on all accommodation properties matching a given search criteria. By default, only basic information is returned. <br/><br/>It is mandatory to pass one of the input parameters: accommodations, airport, city, country or region. <br/><br/>To receive extended information use the `extras` parameter.
- **`booking-com-demand-pp-cli accommodations details-changes`** - Use this endpoint to track accommodations that have opened, closed, or had relevant content updates since a specific timestamp. Changes can include updates to general information, facilities, rooms, photos, payments, and more. <br/>You can:<br/> - Filter results by country or city.<br/> - Use the "next" timestamp from the response to request further updates.<br/><br/>To keep your local accommodation cache up to date, use this endpoint in combination with [accommodations/details](/demand/docs/open-api/demand-api/accommodations/accommodations/details) <br/><br/>The maximum number of IDs returned is approximately 5000 per request.
- **`booking-com-demand-pp-cli accommodations reviews`** - This endpoint provides access to reviews for specified accommodations, allowing you to retrieve traveller feedback associated with a particular property.</br></br> ✅ The reviews returned can be [filtered and sorted](/demand/docs/accommodations/filter-sorting), with the option to limit the number of reviews per accommodation by specifying the `rows` parameter.</br></br> Please note that the ratings **score is based on all traveller traffic across Booking.com**, and may not necessarily reflect the experience of your own customers.</br></br> If you choose to display or use these ratings and reviews, you are responsible for ensuring that your travellers are properly informed about what these scores represent.
- **`booking-com-demand-pp-cli accommodations reviews-scores`** - This endpoint returns score distribution and score breakdown for the specified accommodations. The scores information can be filtered by reviewer parameters and languages.
- **`booking-com-demand-pp-cli accommodations search`** - This endpoint returns, by default, the cheapest available product for each accommodation that matches the specified search criteria. <br/><br/>When you apply location filters using parameters such as country or region id, the results are sorted by Booking.com popularity (top_picks) instead of price.<br/>In this case, accommodations are ranked in descending order of popularity, meaning higher-ranked listings will appear earlier in the response.

### cars

This API collection is specific to the car rentals part of the connected trip.</br></br> Use these endpoints to search for car rentals, check car details and look for depots and suppliers.

- **`booking-com-demand-pp-cli cars constants`** - This endpoint returns a list of relevant car constants names in the specified languages. For example, calling with the parameters {"languages":"en-us","fr"} will return the list in English (US) and French. To retrieve the full list, make the request with an empty body.
- **`booking-com-demand-pp-cli cars depots`** - Use this endpoint to retrieve the list of all available car rental depots.
- **`booking-com-demand-pp-cli cars depots-reviews-scores`** - Use this endpoint to return the score breakdown for the specified depots together with the overall number of reviews and score. </br></br>- Please note that the **ratings score is based on all traveller traffic across Booking.com/cars**, and may not necessarily reflect the experience of your own customers.</br></br> - If you choose to display or use these ratings, you are responsible for ensuring that your travellers are properly informed about what these scores represent.
- **`booking-com-demand-pp-cli cars details`** - Use this endpoint to fetch car details like bag capacity, number of doors, brand and model, etc.
- **`booking-com-demand-pp-cli cars search`** - Use this endpoint to retrieve the available car rentals matching the search criteria.
- **`booking-com-demand-pp-cli cars suppliers`** - Use this endpoint to fetch a list of car rental suppliers. <br/>You can use a supplier ID (or an array of them), to retrieve specific details. <br/>Alternatively, if you do not add any ID in the request, the response will include all suppliers.

### common

Manage common

- **`booking-com-demand-pp-cli common languages`** - This endpoint returns a list of human language codes and their names in the corresponding language. To get the full list call the endpoint passing an empty body. The language codes returned are what is used as input and output for other endpoints.
- **`booking-com-demand-pp-cli common locations-airports`** - This endpoint returns a list of airport codes and their names in the selected languages. The airports returned may be filtered by a location id. For example, you can get the list of airports in The Netherlands by passing: `{"country":"nl"}`. To get the full list call the endpoint passing an empty body. The airport codes returned are what is used as input and output for other endpoints. This endpoint implements pagination of the results.
- **`booking-com-demand-pp-cli common locations-cities`** - This endpoint returns a list of city codes and their names in the selected languages. The cities returned may be filtered by a location id. For example, you can get the list of cities in The Netherlands by passing: `{"country":"nl"}`. To get the full list call the endpoint passing an empty body. The city codes returned are what is used as input and output for other endpoints. This endpoint implements pagination of the results.
- **`booking-com-demand-pp-cli common locations-countries`** - This endpoint returns a list of country codes and their names in the selected languages. The countries returned may be filtered by a location id. For example, you can get the list of countries that are associated with the European Alps region by passing: `{"region":1199}`. <br/><br/>To get the full list call the endpoint passing an empty body. The returned country codes are used as input and output for other endpoints. <br/><br/>This endpoint implements pagination of the results.
- **`booking-com-demand-pp-cli common locations-districts`** - This endpoint returns a list of districts with translations in the selected languages. The districts returned may be filtered by a location id. For example, you can get the list of districts in Amsterdam by passing: `{"city":-2140479}`. <br/><br/>To get the full list call the endpoint passing an empty body. The district ids returned are what is used as input and output for other endpoints.<br/><br/> This endpoint implements pagination of the results.
- **`booking-com-demand-pp-cli common locations-landmarks`** - This endpoint returns a list of relevant geographical landmark codes and their names in the selected languages. The landmarks returned may be filtered by a location id. For example, you can get the list of landmarks that are associated with the city of Paris in France by passing: `{"city":-1456928}`. <br/><br/>To get the full list call the endpoint passing an empty body. The landmark codes returned are what is used as input and output for other endpoints. <br/><br/>This endpoint implements pagination of the results.
- **`booking-com-demand-pp-cli common locations-regions`** - This endpoint returns a list of regions with translations in the selected languages. The regions returned may be filtered by a location id. For example, you can get the list of regions in the Netherlands or that the Netherlands is a part of by passing: `{"country":"nl"}`.  To get the full list call the endpoint passing an empty body. The region ids returned are what is used as input and output for other endpoints. This endpoint implements pagination of the results.
- **`booking-com-demand-pp-cli common payments-cards`** - This endpoint returns a list of supported payment cards and their names in English. Examples of payment types are the different credit and debit cards. <br/><br/>To get the full list call the endpoint passing an empty body. The codes returned are what is used as input and output for other endpoints.
- **`booking-com-demand-pp-cli common payments-currencies`** - This endpoint returns a list of currency codes and their names in the selected languages. To get the full list call the endpoint passing an empty body. <br/><br/>The currency codes returned are what is used as input and output for other endpoints.

### messages

Provides endpoints for two-way post-booking communication between guests and properties. </br></br>Use these endpoints to send and retrieve messages, exchange images, and check conversation details.

- **`booking-com-demand-pp-cli messages confirm-receipt`** - Confirms receipt of specified messages.
This confirmation is required before receiving new messages from the POST /messages/latest endpoint.
- **`booking-com-demand-pp-cli messages download-attachment`** - Retrieves a file that was attached to a message. The response includes the file's content as a base64-encoded string.
- **`booking-com-demand-pp-cli messages fetch-latest`** - Retrieves up to **100 of the most recent messages** including messages from both property and guest.<br/>

- Messages are returned in reverse chronological order (newest first).
- Use this endpoint to sync message threads or poll for updates.

**Important:**  To retrieve the latest messages, send an empty POST request.  Any content in the request body will be ignored.
- **`booking-com-demand-pp-cli messages get-attachment-metadata`** - Returns metadata for a file uploaded in a message, including its name, type, and size.
- **`booking-com-demand-pp-cli messages retrieve-conversation`** - Retrieves a conversation accessible to the authenticated user, including message history and participants.
- **`booking-com-demand-pp-cli messages send`** - Sends a message within a conversation. The message body supports plain text.
Optionally, attach a file by referencing a previously uploaded attachment ID.
- **`booking-com-demand-pp-cli messages upload-attachment`** - Uploads a file to be used as a message attachment.
The response includes an attachment ID to reference when sending messages.

### orders

Enables management of booking orders within the Demand API. </br></br>Use these endpoints to preview and create new orders, check order details, cancel or modify existing orders. This collection is required to integrate booking and order management functionality.

- **`booking-com-demand-pp-cli orders cancel`** - Use this endpoint to process an order cancellation. Refer to the [Cancellations guide](/demand/docs/orders-api/cancel-order) for instructions, tips and examples.
- **`booking-com-demand-pp-cli orders create`** - Use this endpoint to confirm the booking and proceed the payment.
- **`booking-com-demand-pp-cli orders details`** - This endpoint returns basic information for orders filtered according to the input.
- **`booking-com-demand-pp-cli orders details-accommodations`** - This endpoint returns all information for given accommodation orders, sorted by `bookingDate` in descending order
- **`booking-com-demand-pp-cli orders details-cars`** - This endpoint returns car order details, sorted by `bookingDate` in descending order
- **`booking-com-demand-pp-cli orders details-flights`** - Use this endpoint to retrieve detailed information for one or more flight orders. - You can request car order details either by **order ID** or by **reservation ID**. - The response includes all relevant information for each order, such as booking and cancellation details, commission, pricing, and optional extras (for example, policies).   Results are sorted by `bookingDate` in descending order, with the most recently booked orders listed first. It returns structured order data, including pricing, itinerary segments with IATA airport codes, etc. </br></br>This endpoint is ideal for:</br></br> - Displaying flight booking details in traveller dashboards or confirmation pages.</br> - Generating post-booking communications and invoices. </br> - Performing reporting or reconciliation tasks that require accurate itinerary and pricing data.
- **`booking-com-demand-pp-cli orders modify`** - Use this endpoint to modify certain aspects of an accommodation order, such as credit card details, checkin/checkout dates, and room configurations (guest allocation, guest names, and smoking preferences). 
  - See the [Orders modification guide](/demand/docs/orders-api/order-modify) for examples and best practices.
- **`booking-com-demand-pp-cli orders preview`** - This endpoint returns the total final price with final charges, as well as the price breakdown and payment/cancellation policies for each product passed in the input.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
booking-com-demand-pp-cli accommodations search --checkin 2026-01-15

# JSON for scripting and agents
booking-com-demand-pp-cli accommodations search --checkin 2026-01-15 --json

# Filter to specific fields
booking-com-demand-pp-cli accommodations search --checkin 2026-01-15 --json --select id,name,status

# Dry run — show the request without sending
booking-com-demand-pp-cli accommodations search --checkin 2026-01-15 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
booking-com-demand-pp-cli accommodations search --checkin 2026-01-15 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-booking-com-demand -g
```

Then invoke `/pp-booking-com-demand <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add booking-com-demand booking-com-demand-pp-mcp -e BOOKING_COM_DEMAND_BEARER_AUTH=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/booking-com-demand-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BOOKING_COM_DEMAND_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "booking-com-demand": {
      "command": "booking-com-demand-pp-mcp",
      "env": {
        "BOOKING_COM_DEMAND_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
booking-com-demand-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/booking-com-demand-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BOOKING_COM_DEMAND_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `booking-com-demand-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BOOKING_COM_DEMAND_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every request** — Verify your Bearer token is set: booking-com-demand-pp-cli doctor --json
- **403 Forbidden** — Check your Affiliate ID is correct and your partner account has Demand API access enabled
- **429 Too Many Requests** — The CLI uses adaptive rate limiting. Wait 60 seconds or reduce --limit on bulk operations
- **Empty search results** — Verify date range is in the future and destination exists: booking-com-demand-pp-cli locations cities --query Amsterdam

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**hotels_mcp_server**](https://github.com/esakrissa/hotels_mcp_server) — Python
- [**bookingcomclient**](https://github.com/azaelcodes/bookingcomclient) — PHP
- [**bookingcom-client**](https://github.com/rybakit/bookingcom-client) — PHP
- [**hotels-skill**](https://github.com/Anmoldureha/hotels-skill) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
