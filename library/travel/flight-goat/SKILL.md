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
