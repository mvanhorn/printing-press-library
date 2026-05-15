---
name: pp-booking-com-demand
description: "Every Booking.com Demand API feature, plus offline search, price drift detection, and review-based property ranking... Trigger phrases: `search hotels in`, `find accommodation`, `check booking availability`, `compare hotel prices`, `booking.com properties`, `manage travel bookings`, `use booking`."
author: "netnull"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - booking-com-demand-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/travel/booking-com-demand/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Booking.com — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `booking-com-demand-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install booking-com-demand --cli-only
   ```
2. Verify: `booking-com-demand-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/booking-com-demand/cmd/booking-com-demand-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The only CLI for Booking.com's Demand API. Search accommodations, manage bookings, track price changes, and rank properties by review categories — all with offline SQLite storage, agent-native JSON output, and full MCP server support.

## When to Use This CLI

Use this CLI when you need programmatic access to Booking.com's accommodation search, booking management, or property data. Ideal for travel affiliates automating content creation, OTA engineers syncing inventory, and travel agents managing bookings. The offline store and review ranking make it particularly valuable for recurring workflows like weekly price monitoring or destination research.

## Unique Capabilities

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

## Command Reference

**accommodations** — This API collection is specific for the stay part of the connected trip. </br></br>Use these endpoints to search for stays such as hotels and apartments, check availability, retrieve reviews, and get detailed property information.

- `booking-com-demand-pp-cli accommodations availability` — Use this endpoint to return detailed product availability, price and charges of the accommodation matching a given...
- `booking-com-demand-pp-cli accommodations bulk-availability` — Use this endpoint to retrieve detailed product availability, price and charges of a list of accommodations. By...
- `booking-com-demand-pp-cli accommodations chains` — Use this endpoint to retrieve a list of accommodation chains and their associated brands.<br/><br/> A chain-branded...
- `booking-com-demand-pp-cli accommodations constants` — Use this endpoint to retrieve standardised codes and names for accommodation-specific types, including facilities,...
- `booking-com-demand-pp-cli accommodations details` — This endpoint returns detailed information on all accommodation properties matching a given search criteria. By...
- `booking-com-demand-pp-cli accommodations details-changes` — Use this endpoint to track accommodations that have opened, closed, or had relevant content updates since a specific...
- `booking-com-demand-pp-cli accommodations reviews` — This endpoint provides access to reviews for specified accommodations, allowing you to retrieve traveller feedback...
- `booking-com-demand-pp-cli accommodations reviews-scores` — This endpoint returns score distribution and score breakdown for the specified accommodations. The scores...
- `booking-com-demand-pp-cli accommodations search` — This endpoint returns, by default, the cheapest available product for each accommodation that matches the specified...

**cars** — This API collection is specific to the car rentals part of the connected trip.</br></br> Use these endpoints to search for car rentals, check car details and look for depots and suppliers.

- `booking-com-demand-pp-cli cars constants` — This endpoint returns a list of relevant car constants names in the specified languages. For example, calling with...
- `booking-com-demand-pp-cli cars depots` — Use this endpoint to retrieve the list of all available car rental depots.
- `booking-com-demand-pp-cli cars depots-reviews-scores` — Use this endpoint to return the score breakdown for the specified depots together with the overall number of reviews...
- `booking-com-demand-pp-cli cars details` — Use this endpoint to fetch car details like bag capacity, number of doors, brand and model, etc.
- `booking-com-demand-pp-cli cars search` — Use this endpoint to retrieve the available car rentals matching the search criteria.
- `booking-com-demand-pp-cli cars suppliers` — Use this endpoint to fetch a list of car rental suppliers. <br/>You can use a supplier ID (or an array of them), to...

**common** — Manage common

- `booking-com-demand-pp-cli common languages` — This endpoint returns a list of human language codes and their names in the corresponding language. To get the full...
- `booking-com-demand-pp-cli common locations-airports` — This endpoint returns a list of airport codes and their names in the selected languages. The airports returned may...
- `booking-com-demand-pp-cli common locations-cities` — This endpoint returns a list of city codes and their names in the selected languages. The cities returned may be...
- `booking-com-demand-pp-cli common locations-countries` — This endpoint returns a list of country codes and their names in the selected languages. The countries returned may...
- `booking-com-demand-pp-cli common locations-districts` — This endpoint returns a list of districts with translations in the selected languages. The districts returned may be...
- `booking-com-demand-pp-cli common locations-landmarks` — This endpoint returns a list of relevant geographical landmark codes and their names in the selected languages. The...
- `booking-com-demand-pp-cli common locations-regions` — This endpoint returns a list of regions with translations in the selected languages. The regions returned may be...
- `booking-com-demand-pp-cli common payments-cards` — This endpoint returns a list of supported payment cards and their names in English. Examples of payment types are...
- `booking-com-demand-pp-cli common payments-currencies` — This endpoint returns a list of currency codes and their names in the selected languages. To get the full list call...

**messages** — Provides endpoints for two-way post-booking communication between guests and properties. </br></br>Use these endpoints to send and retrieve messages, exchange images, and check conversation details.

- `booking-com-demand-pp-cli messages confirm-receipt` — Confirms receipt of specified messages. This confirmation is required before receiving new messages from the POST...
- `booking-com-demand-pp-cli messages download-attachment` — Retrieves a file that was attached to a message. The response includes the file's content as a base64-encoded string.
- `booking-com-demand-pp-cli messages fetch-latest` — Retrieves up to **100 of the most recent messages** including messages from both property and guest.<br/> - Messages...
- `booking-com-demand-pp-cli messages get-attachment-metadata` — Returns metadata for a file uploaded in a message, including its name, type, and size.
- `booking-com-demand-pp-cli messages retrieve-conversation` — Retrieves a conversation accessible to the authenticated user, including message history and participants.
- `booking-com-demand-pp-cli messages send` — Sends a message within a conversation. The message body supports plain text. Optionally, attach a file by...
- `booking-com-demand-pp-cli messages upload-attachment` — Uploads a file to be used as a message attachment. The response includes an attachment ID to reference when sending...

**orders** — Enables management of booking orders within the Demand API. </br></br>Use these endpoints to preview and create new orders, check order details, cancel or modify existing orders. This collection is required to integrate booking and order management functionality.

- `booking-com-demand-pp-cli orders cancel` — Use this endpoint to process an order cancellation. Refer to the [Cancellations...
- `booking-com-demand-pp-cli orders create` — Use this endpoint to confirm the booking and proceed the payment.
- `booking-com-demand-pp-cli orders details` — This endpoint returns basic information for orders filtered according to the input.
- `booking-com-demand-pp-cli orders details-accommodations` — This endpoint returns all information for given accommodation orders, sorted by `bookingDate` in descending order
- `booking-com-demand-pp-cli orders details-cars` — This endpoint returns car order details, sorted by `bookingDate` in descending order
- `booking-com-demand-pp-cli orders details-flights` — Use this endpoint to retrieve detailed information for one or more flight orders. - You can request car order...
- `booking-com-demand-pp-cli orders modify` — Use this endpoint to modify certain aspects of an accommodation order, such as credit card details, checkin/checkout...
- `booking-com-demand-pp-cli orders preview` — This endpoint returns the total final price with final charges, as well as the price breakdown and...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
booking-com-demand-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find top-rated budget hotels

```bash
booking-com-demand-pp-cli reviews rank --destination Barcelona --min-cleanliness 8 --max-price 120 --sort price --agent --select name,reviewScore,price
```

Ranks properties by cleanliness score, filtered to budget range, with agent-friendly narrow output

### Monitor price changes

```bash
booking-com-demand-pp-cli accommodations drift --since 7d --threshold 10 --json
```

Detects properties where pricing moved more than 10% since last sync

### Compare shortlisted properties

```bash
booking-com-demand-pp-cli accommodations compare --ids 123,456,789 --show facilities,scores,price --agent --select name,overallScore,price
```

Side-by-side comparison with narrowed output for agent consumption

### Weekly booking review

```bash
booking-com-demand-pp-cli orders upcoming --days 14 --with-messages --json
```

Lists imminent check-ins with unresolved message threads

### Destination research

```bash
booking-com-demand-pp-cli locations intel --city Amsterdam --json
```

City-level analytics aggregated from the local store

## Auth Setup

Requires a Booking.com Demand API Bearer token and Affiliate ID. Get partner access at developers.booking.com. Set BOOKING_API_TOKEN and BOOKING_AFFILIATE_ID in your environment.

Run `booking-com-demand-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  booking-com-demand-pp-cli accommodations search --checkin 2026-01-15 --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
booking-com-demand-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
booking-com-demand-pp-cli feedback --stdin < notes.txt
booking-com-demand-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.booking-com-demand-pp-cli/feedback.jsonl`. They are never POSTed unless `BOOKING_COM_DEMAND_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BOOKING_COM_DEMAND_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
booking-com-demand-pp-cli profile save briefing --json
booking-com-demand-pp-cli --profile briefing accommodations search --checkin 2026-01-15
booking-com-demand-pp-cli profile list --json
booking-com-demand-pp-cli profile show briefing
booking-com-demand-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `booking-com-demand-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add booking-com-demand-pp-mcp -- booking-com-demand-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which booking-com-demand-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   booking-com-demand-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `booking-com-demand-pp-cli <command> --help`.
