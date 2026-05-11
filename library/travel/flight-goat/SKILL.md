---
name: pp-flight-goat
description: "Search Google Flights, scan Kayak long-haul routes, and join FlightAware AeroAPI reliability, alerts, and tracking from one CLI."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - flight-goat-pp-cli
    install:
      - kind: go
        bins: [flight-goat-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/flight-goat/cmd/flight-goat-pp-cli
---

# Flight Goat — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `flight-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install flight-goat --cli-only
   ```
2. Verify: `flight-goat-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/flight-goat/cmd/flight-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Categories
AeroAPI is divided into several categories to make things easier to
discover.
- Flights: Summary information, planned routes, positions and more
- Foresight: Flight positions enhanced with FlightAware Foresight™
- Airports: Airport information and FIDS style resources
- Operators: Operator information and fleet activity resources
- Alerts: Configure flight alerts and delivery destinations
- History: Historical flight access for various endpoints
- Miscellaneous: Flight disruption, future schedule information, and aircraft owner information

## Google Flights Currency

Use `--currency <ISO-4217-code>` when the user wants Google Flights prices in a
specific currency. Omit it for the default USD behavior.

```bash
flight-goat-pp-cli flights MAN AGP 2026-05-10 --currency GBP --sort cheapest --agent
flight-goat-pp-cli dates JFK CDG --from 2026-07-01 --to 2026-07-31 --currency EUR --sort --agent
flight-goat-pp-cli compare SEA LHR 2026-06-15 --currency GBP --agent
```

The flag is only valid on Google Flights price commands: `flights`, `dates`,
`compare`, `gf-search`, and `cheapest-longhaul`. Do not add it to AeroAPI or
Kayak-only commands.

## Watch a Purchased Flight for Price Drops

`watch` monitors a flight the user has *already booked* and alerts when the
same exact itinerary (airline + flight number + date + route + cabin) reappears
on Google Flights below `paid - threshold`. Use it when the user says "did my
flight get cheaper?" or "watch DL 669 for me."

State is persisted in a local SQLite file (default
`~/.local/share/flight-goat-pp-cli/watches.db`, override with `--watch-db` or
`$FLIGHT_GOAT_WATCH_DB`). Subcommands: `add`, `list`, `show`, `check`,
`remove`, `alert-test`.

```bash
# Register a watch (all required fields)
flight-goat-pp-cli watch add \
  --from SFO --to JFK --date 2026-06-21 \
  --airline DL --flight-number 669 --cabin economy \
  --paid 428.20 --threshold 50 --currency USD \
  --notify webhook:https://hooks.example.com/flight-drops --agent

# Stricter watch with departure-time guard, explicit fare brand, and
# basic-economy excluded so a $300 basic fare won't false-match your
# $700 main-cabin ticket
flight-goat-pp-cli watch add \
  --from SFO --to JFK --date 2026-06-21 --departure-time 07:30 \
  --airline DL --flight-number 668 --cabin economy --fare-brand "Main Cabin" \
  --paid 700 --threshold 50 --agent

# Add a watch on a basic-economy ticket you actually paid for (opt-in)
flight-goat-pp-cli watch add \
  --from LAX --to ATL --date 2026-07-10 \
  --airline DL --flight-number 1234 --cabin economy --fare-brand "Basic Economy" \
  --include-basic --paid 198 --threshold 30 --agent

# List, inspect, re-check
flight-goat-pp-cli watch list --agent
flight-goat-pp-cli watch show watch_5a25c89c2966 --agent
flight-goat-pp-cli watch check watch_5a25c89c2966 --agent     # one watch
flight-goat-pp-cli watch check --agent                        # all active watches

# Verify the alert path without burning a Google Flights request
flight-goat-pp-cli watch alert-test watch_5a25c89c2966 --agent

# Force a re-alert after a previous one fired (dedup is on by default)
flight-goat-pp-cli watch check watch_5a25c89c2966 --force-alert --agent

# Re-alert only when the price drops by at least 25 more
flight-goat-pp-cli watch check --repeat-delta 25 --agent

flight-goat-pp-cli watch remove watch_5a25c89c2966 --agent
```

`watch check` returns a stable JSON envelope (also POSTed verbatim to
`--notify webhook:`):

```json
{
  "schema": "flight-goat.watch.check.v1",
  "watch_id": "watch_…",
  "origin": "SFO", "destination": "JFK",
  "departure_date": "2026-06-21", "departure_time": "07:30",
  "airline": "DL", "flight_number": "668",
  "cabin": "economy", "fare_brand": "Main Cabin",
  "original_price": 700, "threshold": 50, "currency": "USD",
  "booking_url": "https://www.google.com/travel/flights?q=Flights+from+SFO+to+JFK+on+2026-06-21+economy&curr=USD",
  "found_price": 609, "route_cheapest_price": 280, "delta": 91,
  "confidence": "high",
  "match_reason": "exact match: same airline DL, flight 668; date 2026-06-21; route SFO→JFK; cabin economy; basic-economy excluded; departure 07:25 within ±30 min of your 07:30",
  "threshold_crossed": true,
  "alert_dispatched": true,
  "matched_flight": {
    "airline": "DL", "flight_number": "668", "price": 609,
    "cabin": "economy", "fare_brand": "Main Cabin",
    "departure_time": "2026-06-21T07:25:00",
    "arrival_time":   "2026-06-21T15:55:00",
    "duration_minutes": 330, "stops": 0
  },
  "safety_notice": "Same flight appears cheaper. Verify fare rules, refundability, cancellation fees, credits, and seat/bag differences before canceling or rebooking."
}
```

When relaying an alert to the user, always include:

1. `match_reason` — the chain of constraints the matcher verified, so the user can sanity-check the conclusion themselves.
2. `matched_flight.departure_time` / `arrival_time` / `duration_minutes` — so the user can confirm this is the same operating itinerary, not a different flight that happens to share a number.
3. `booking_url` — the Google Flights search URL pre-filled with the route + date + cabin + currency. One tap to verify the live fare.
4. `safety_notice` verbatim.

**Fare-class safety:** alerts fire only on the cabin the user registered (Google Flights filters by cabin at search time), and `exclude_basic` defaults to **on** so a $300 basic-economy result never false-matches a $700 main-cabin ticket. Override with `--include-basic` only when the user actually purchased basic economy.

Match-confidence rules — read carefully, this is the safety property of the
feature:

- `high` — the cheaper itinerary is the user's exact flight (airline + flight
  number + date + route + cabin all match). Only `high` matches trigger
  alerts.
- `medium` — same airline + cabin + date + route, but the provider didn't
  return a flight number. Surfaced as `found_price` for context but never
  alerts.
- `low` — the cheapest fare on the route belongs to a different airline or
  flight number. Returned only as `route_cheapest_price`; never alerts. A
  cheaper-on-route flight does NOT help users who can't move their ticket
  without losing it.

**Always pass the `safety_notice` through to the user when relaying a watch
alert.** The notice covers fare rules, refundability, change fees, credits,
and seat/bag differences — the four most common ways a "the same flight is
cheaper now" message becomes an expensive mistake.

`--notify` accepts `stdout` (default), `json` (machine-readable stdout), or
`webhook:<https-url>` (POSTs the JSON envelope above). Validation happens at
`watch add` time so a bad webhook URL is caught before the first check.

## Development Tools
AeroAPI is defined using the OpenAPI Spec 3.0, which means it can be easily
imported into tools like Postman. To get started try importing the API
specification using
[Postman's instructions](https://learning.postman.com/docs/integrations/available-integrations/working-with-openAPI/).
Once imported as a collection only the "Value" field under the collection's
Authorization tab needs to be populated and saved before making calls.

The AeroAPI OpenAPI specification is located at:\
https://flightaware.com/commercial/aeroapi/resources/aeroapi-openapi.yml

Our [open source AeroApps project](/aeroapi/portal/resources)
provides a small collection of services and sample applications to help
you get started.

The Flight Information Display System (FIDS) AeroApp is an example of a
multi-tier application using multiple languages and Docker containers.
It demonstrates connectivity, data caching, flight presentation, and leveraging flight maps.

The Alerts AeroApp demonstrates the use of AeroAPI to set, edit, and
receive alerts in a sample application with a Dockerized Python backend
and a React frontend.

Our AeroAPI push notification [testing interface](/commercial/aeroapi/send.rvt)
provides a quick and easy way to test the delivery of customized alerts via AeroAPI push.

## Command Reference

**aircraft** — Manage aircraft

- `flight-goat-pp-cli aircraft <type>` — Returns information about an aircraft type, given an ICAO aircraft type designator string. Data returned includes...

**airports** — Manage airports

- `flight-goat-pp-cli airports get` — Returns information about an airport given an ICAO or LID airport code such as KLAX, KIAH, O07, etc. Data returned...
- `flight-goat-pp-cli airports get-all` — Returns the ICAO identifiers of all known airports. For airports that do not have an ICAO identifier, the FAA LID...
- `flight-goat-pp-cli airports get-delays-for-all` — Returns a list of airports with delays. There may be multiple reasons returned per airport if there are multiple...
- `flight-goat-pp-cli airports get-nearby` — Returns a list of airports located within a given distance from the given location.

**alerts** — AeroAPI alerting can be used to configure and receive real-time alerts on key flight
events. With customizable alerting offered by our alert endpoints, AeroAPI empowers
users to selectively pick various types of events/filters to alert on. By doing so,
you can receive specially tailored alerts delivered to you for events such as flight plan
filed, flight departure (out and off), flight arrival (on and in), and more!

To get started with alerting, the **PUT /alerts/endpoint** endpoint must first be used
to set up the account-wide default URL that alerts will be delivered to. This step must
be done before any alerts can be configured and will serve as the fallback URL that all
alerts will be sent to for the account if a specific delivery URL is not designated on a
particular alert. If this is not performed before configuring alerts, then you will
receive a 400 error with an error message reminding you of this step when trying to interact
with the **POST /alerts** endpoint. Once a URL is set via the **PUT /alerts/endpoint** endpoint,
then alerts can be configured using the **POST /alerts** endpoint. The **GET /alerts** endpoint
can also be used to retrieve all currently configured alerts associated with your AeroAPI key.
The **GET /alerts** endpoint will allow you to easily retrieve the id of any specific alerts of
interest configured for the account which can let you use the **GET** **PUT** and **DELETE**
**/alerts/{id}** endpoints to retrieve, update, and delete specific alerts.

When configuring an individual alert, the *target_url* field can be set to a URL that’s
different than the account-wide target endpoint set via the **PUT /alerts/endpoint**. If
the *target_url* field is set on an alert, then that specific alert will be delivered to
the specified *target_url* rather than the default account-wide one. If this field is not
configured for the alert, then the alert will be delivered to the default account-wide endpoint.
By setting this field, one can easily target different alerts to be received by different endpoints
which can be useful for configuring per-application alerts or sending alerts to an alternate
development environment without having to adjust a production alert configuration.

For each alert configured, one-to-many ‘events’ can be set for alert delivery. While most
events will result in one alert delivery, both the *arrival* and the *departure* events can
result in multiple alerts delivered (referred to as bundled). The *departure* event bundles the
departure (actual OFF the ground) alert, along with the flight plan filed alert and up to 5
per-departure changes which can include alerts for significant departure delays of over
30 minutes, gate changes, and airport delays. FlightAware Global customers will
also receive *Power on* and *Ready to taxi* alerts as part of the departure bundle. The *arrival* event
bundles the arrival (actual ON the ground) alert, along with up to 5 en-route changes (including delays
of over 30 minutes and excluding diversions) identified. FlightAware Global customers will also receive
*taxi stop* times as part of the *arrival* bundle. Setting a bundled type and unbundled type for an
On/Off will only result in a single alert in the case where events may overlap.

If there is a need to change the alert configurations, updating an alert using the **PUT /alerts/{id}**
endpoint and a unique alert identifier (id) is preferred rather than creating an additional alert.
By doing so, you can avoid duplicate alerts being delivered which could create unnecessary noise
if they are not of interest anymore.

If at any point there is a need to delete an alert, the **DELETE alerts/{id}** endpoint can be
leveraged to delete an alert so that it won’t be delivered anymore. As a reminder, specific alert
IDs can be retrieved from the **GET /alerts** endpoint.

- `flight-goat-pp-cli alerts create` — Create a new AeroAPI flight alert. When the alert is triggered, a callback mechanism will be used to notify the...
- `flight-goat-pp-cli alerts delete` — Deletes specific alert with given ID
- `flight-goat-pp-cli alerts delete-endpoint` — Remove the default account-wide URL that will be POSTed to for alerts that are not configured with a specific URL....
- `flight-goat-pp-cli alerts get` — Returns the configuration data for an alert with the specified ID.
- `flight-goat-pp-cli alerts get-all` — Returns all configured alerts for the FlightAware account (this includes alerts configured through other means by...
- `flight-goat-pp-cli alerts get-endpoint` — Returns URL that will be POSTed to for alerts that are delivered via AeroAPI.
- `flight-goat-pp-cli alerts set-endpoint` — Updates the default URL that will be POSTed to for alerts that are delivered via AeroAPI. This sets the account-wide...
- `flight-goat-pp-cli alerts update` — Modifies the configuration for an alert with the specified ID. If a target URL address is provided, then the alert...

**disruption-counts** — Manage disruption counts

- `flight-goat-pp-cli disruption-counts get` — Returns flight cancellation/delay counts in the specified time period for a particular airline or airport.
- `flight-goat-pp-cli disruption-counts get-all` — Returns overall flight cancellation/delay counts in the specified time period for either all airlines or all airports.

**flights** — Manage flights

- `flight-goat-pp-cli flights get` — Returns the flight info status summary for a registration, ident, or fa_flight_id. If a fa_flight_id is specified...
- `flight-goat-pp-cli flights get-by-advanced-search` — Returns currently or recently airborne flights based on geospatial search parameters. Query parameters include a...
- `flight-goat-pp-cli flights get-by-position-search` — Returns flight positions based on geospatial search parameters. This allows you to locate flights that have ever...
- `flight-goat-pp-cli flights get-by-search` — Search for airborne flights by matching against various parameters including geospatial data. Uses a simplified...
- `flight-goat-pp-cli flights get-count-by-search` — Full search query documentation is available at the /flights/search endpoint.

**foresight** — Foresight endpoints provide access to FlightAware's Foresight predictive models and
predictions for key events. Our advanced machine learning (ML) models identify key
influencing factors for a flight to forecast future events in real-time, providing
unprecedented insight to improve operational efficiencies and facilitate better
decision-making in the air and on the ground. To learn more about the power of Foresight,
visit https://www.flightaware.com/commercial/foresight/

These endpoints each mirror a non-Foresight equivalent endpoint of similar functionality,
with the addition of all the ML 'predicted' values included in the Foresight response. The
respective non-Foresight endpoint response includes a flag, 'foresight_predictions_available',
which can optionally be used as a trigger to obtain and leverage Foresight predictions on an
as-needed basis and manage cost. Foresight is only available to Premium tier customers.
Contact integrationsales@flightaware.com for more information, pricing details, and to have
your account enabled for Foresight.

- `flight-goat-pp-cli foresight get-flight-position-with` — Get flight's current position, including Foresight data
- `flight-goat-pp-cli foresight get-flight-with` — Returns the flight info status summary for a registration, ident, or fa_flight_id, including all available predicted...
- `flight-goat-pp-cli foresight get-flights-by-advanced-search-with` — Returns currently or recently airborne flights based on geospatial search parameters. If available, flights'...

**history** — Manage history

- `flight-goat-pp-cli history get-aircraft-last-flight` — Returns flight info status summary for an aircraft's last known flight given its registration. The search is limited...
- `flight-goat-pp-cli history get-flight` — Returns historical flight info status summary for a registration, ident, or fa_flight_id. If a fa_flight_id is...
- `flight-goat-pp-cli history get-flight-map` — Returns a historical flight's track as a base64-encoded image. Image can contain a variety of additional data layers...
- `flight-goat-pp-cli history get-flight-route` — Returns information about a historical flight's filed route including coordinates, names, and types of fixes along...
- `flight-goat-pp-cli history get-flight-track` — Returns the track for a historical flight as an array of positions. Data is available from now back to...

**operators** — Manage operators

- `flight-goat-pp-cli operators get` — Returns information for an operator such as their name, ICAO/IATA codes, headquarter location, etc.
- `flight-goat-pp-cli operators get-all` — Returns list of operator references (ICAO/IATA codes and URLs to access more information).

**schedules** — Manage schedules

- `flight-goat-pp-cli schedules` — Returns scheduled flights that have been published by airlines. These schedules are available for up to three months...

**watch** — Monitor purchased-flight prices for drops (see *Watch a Purchased Flight for Price Drops* above for full semantics).

- `flight-goat-pp-cli watch add` — Register a purchased flight. Required: `--from`, `--to`, `--date`, `--airline`, `--flight-number`, `--paid`, `--threshold`. Optional: `--departure-time HH:MM` (matcher rejects candidates ±30 min outside), `--cabin`, `--fare-brand`, `--include-basic` (off by default), `--passengers`, `--currency`, `--notify`, `--booking-ref`, `--notes`.
- `flight-goat-pp-cli watch list` — List registered watches; filter by `--status active|paused|archived`.
- `flight-goat-pp-cli watch show <watch_id>` — Show one watch's full state.
- `flight-goat-pp-cli watch check [watch_id]` — Re-check the live price for one watch (or all active watches if omitted), dispatch alert on threshold-crossing exact matches, persist `last_seen_price`/`last_alerted_price`. Flags: `--force-alert`, `--repeat-delta`.
- `flight-goat-pp-cli watch remove <watch_id>` — Delete a watch.
- `flight-goat-pp-cli watch alert-test <watch_id>` — Send a synthetic alert through the watch's `--notify` sink without hitting Google Flights.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
flight-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Set your API key via environment variable:

```bash
export FLIGHT_GOAT_API_KEY_AUTH="<your-key>"
```

Or persist it in `~/.config/flight-goat-pp-cli/config.toml`.

Run `flight-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  flight-goat-pp-cli airports get mock-value --agent --select id,name,status
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
flight-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
flight-goat-pp-cli feedback --stdin < notes.txt
flight-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.flight-goat-pp-cli/feedback.jsonl`. They are never POSTed unless `FLIGHT_GOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FLIGHT_GOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
flight-goat-pp-cli profile save briefing --json
flight-goat-pp-cli --profile briefing airports get mock-value
flight-goat-pp-cli profile list --json
flight-goat-pp-cli profile show briefing
flight-goat-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `flight-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)
## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/flight-goat/cmd/flight-goat-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add flight-goat-pp-mcp -- flight-goat-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which flight-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   flight-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `flight-goat-pp-cli <command> --help`.
