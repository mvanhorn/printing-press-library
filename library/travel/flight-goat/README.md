# Flight Goat CLI

# Introduction
AeroAPI is a simple, query-based API that gives software developers access
to a variety of FlightAware's flight data. Users can obtain current or
historical data. AeroAPI is a RESTful API delivering accurate and
actionable aviation data. With the introduction of Foresight™, customers
have access to the data that powers over half of the predictive airline
ETAs in the US.

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

## Install

The recommended path installs both the `flight-goat-pp-cli` binary and the `pp-flight-goat` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install flight-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install flight-goat --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/flight-goat/cmd/flight-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/flight-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-flight-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-flight-goat --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-flight-goat skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-flight-goat. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export FLIGHT_GOAT_API_KEY_AUTH="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/flight-goat-pp-cli/config.toml`.

### 3. Verify Setup

```bash
flight-goat-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
flight-goat-pp-cli airports get mock-value
```

## Usage

Run `flight-goat-pp-cli --help` for the full command reference and flag list.

### Google Flights Currency

Google Flights price commands accept `--currency <ISO-4217-code>` for native
Google Flights prices in that currency. The default is USD when the flag is
omitted.

```bash
flight-goat-pp-cli flights MAN AGP 2026-05-10 --currency GBP --sort cheapest
flight-goat-pp-cli dates JFK CDG --from 2026-07-01 --to 2026-07-31 --currency EUR --sort
flight-goat-pp-cli compare SEA LHR 2026-06-15 --currency GBP
```

`--currency` is intentionally command-scoped. It is available on commands that
ask Google Flights for prices (`flights`, `dates`, `compare`, `gf-search`, and
`cheapest-longhaul`), not on AeroAPI or Kayak-only commands.

## Commands

### aircraft

Manage aircraft

- **`flight-goat-pp-cli aircraft get-flight-type`** - Returns information about an aircraft type, given an ICAO aircraft type designator string.
Data returned includes the description, type, manufacturer, engine type, and engine
count.

### airports

Manage airports

- **`flight-goat-pp-cli airports get`** - Returns information about an airport given an ICAO or LID airport code
such as KLAX, KIAH, O07, etc. Data returned includes airport name,
city, state (when known), latitude, longitude, and timezone.
- **`flight-goat-pp-cli airports get-all`** - Returns the ICAO identifiers of all known airports. For airports that
do not have an ICAO identifier, the FAA LID identifier will be used.
Links for further information about each airport are included.
- **`flight-goat-pp-cli airports get-delays-for-all`** - Returns a list of airports with delays. There may be multiple reasons
returned per airport if there are multiple types of delays reported at
an airport. Note that individual flights can be delayed without there
being an airport-wide delay returned by this endpoint.
- **`flight-goat-pp-cli airports get-nearby`** - Returns a list of airports located within a given distance from the
given location.

### alerts

AeroAPI alerting can be used to configure and receive real-time alerts on key flight
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

- **`flight-goat-pp-cli alerts create`** - Create a new AeroAPI flight alert. When the alert is triggered, a
callback mechanism will be used to notify the address set via the
/alerts/endpoint endpoint. Each callback will be charged as a query and
count towards usage for the AeroAPI key that created the alert. If this key
is disabled or removed, the alert will no longer be available.
If a target_url is provided, then this specific alert will be delivered
to that address regardless of the adress set via the /alerts/endpoint endpoint.
- **`flight-goat-pp-cli alerts delete`** - Deletes specific alert with given ID
- **`flight-goat-pp-cli alerts delete-endpoint`** - Remove the default account-wide URL that will be POSTed to for alerts that are
not configured with a specific URL. This means that any alerts that are not configured
with a specific URL will not be delivered.
- **`flight-goat-pp-cli alerts get`** - Returns the configuration data for an alert with the specified ID.
- **`flight-goat-pp-cli alerts get-all`** - Returns all configured alerts for the FlightAware account (this
includes alerts configured through other means by the FlightAware user
owning the AeroAPI account like FlightAware's website or mobile apps).
- **`flight-goat-pp-cli alerts get-endpoint`** - Returns URL that will be POSTed to for alerts that are delivered via AeroAPI.
- **`flight-goat-pp-cli alerts set-endpoint`** - Updates the default URL that will be POSTed to for alerts that are delivered via AeroAPI.
This sets the account-wide default URL that all alerts will be delivered to unless
the specific alert has a different delivery address configured for it.
- **`flight-goat-pp-cli alerts update`** - Modifies the configuration for an alert with the specified ID. If a target
URL address is provided, then the alert will be delivered to that address
even if it is different than the default account-wide address set through
the alerts/endpoint endpoint. Updating an alert that was created with a
different AeroAPI key is possible, but will not change the AeroAPI key that
the alert is associated with for usage.

### disruption-counts

Manage disruption counts

- **`flight-goat-pp-cli disruption-counts get`** - Returns flight cancellation/delay counts in the specified time period
for a particular airline or airport.
- **`flight-goat-pp-cli disruption-counts get-all`** - Returns overall flight cancellation/delay counts in the specified time
period for either all airlines or all airports.

### flights

Manage flights

- **`flight-goat-pp-cli flights get`** - Returns the flight info status summary for a registration, ident, or
fa_flight_id.  If a fa_flight_id is specified then a maximum of 1
flight is returned, unless the flight has been diverted in which case
both the original flight and any diversions will be returned with a
duplicate fa_flight_id. If a registration or ident is specified,
approximately 14 days of recent and scheduled flight information is
returned, ordered by `scheduled_out` (or `scheduled_off` if
`scheduled_out` is missing) descending. Alternately, specify a start
and end parameter to find your flight(s) of interest, including up to
10 days of flight history.
- **`flight-goat-pp-cli flights get-by-advanced-search`** - Returns currently or recently airborne flights based on geospatial
search parameters.

Query parameters include a latitude/longitude box, aircraft ident with
wildcards, type with wildcards, prefix, origin airport,
destination airport, origin or destination airport, groundspeed, and
altitude. It takes search terms in a single string comprising of
{operator key value} elements and returns an array of flight
structures. Each search term must be enclosed in curly braces. Multiple
search terms can be combined in an implicit boolean "and" by separating
the terms with at least one space. This function only searches flight
data representing approximately the last 24 hours. Codeshares and
alternate idents are NOT searched when matching against the ident key.

The supported operators include (note that operators take different numbers of arguments):

* false - results must have the specified boolean key set to a value of false. Example: {false arrived}
* true - results must have the specified boolean key set to a value of true. Example: {true lifeguard}
* null - results must have the specified key set to a null value. Example: {null waypoints}
* notnull - results must have the specified key not set to a null value. Example: {notnull aircraftType}
* = - results must have a key that exactly matches the specified value. Example: {= aircraftType C172}
* != - results must have a key that must not match the specified value. Example: {!= prefix H}
* < - results must have a key that is lexicographically less-than a specified value. Example: {< arrivalTime 1276811040}
* \> - results must have a key that is lexicographically greater-than a specified value. Example: {> speed 500}
* <= - results must have a key that is lexicographically less-than-or-equal-to a specified value. Example: {<= alt 8000}
* \>= - results must have a key that is lexicographically greater-than-or-equal-to a specified value.
* match - results must have a key that matches against a case-insensitive wildcard pattern. Example: {match ident AAL*}
* notmatch - results must have a key that does not match against a case-insensitive wildcard pattern. Example: {notmatch aircraftType B76*}
* range - results must have a key that is numerically between the two specified values. Example: {range alt 8000 20000}
* in - results must have a key that exactly matches one of the specified values. Example: {in orig {KLAX KBUR KSNA KLGB}}
* orig_or_dest - results must have either the origin or destination key exactly match one of the specified values. Example: {orig_or_dest {KLAX KBUR KSNA KLGB}}
* airline - results will only include airline flight if the argument is 1, or will only include GA flights if the argument is 0. Example: {airline 1}
* aircraftType - results must have an aircraftType key that matches one of the specified case-insensitive wildcard patterns. Example: {aircraftType {B76* B77*}}
* ident - results must have an ident key that matches one of the specified case-insensitive wildcard patterns. Example: {ident {N123* N456* AAL* UAL*}}
* ident_or_reg - results must have an ident key or was known to be operated by an aircraft registration that matches one of the specified case-insensitive wildcard patterns. Example: {ident_or_reg {N123* N456* AAL* UAL*}}

The supported key names include (note that not all of these key names are returned in the result structure, and some have slightly different names):

* actualDepartureTime - Actual time of departure, or null if not departed yet. UNIX epoch timestamp seconds since 1970
* aircraftType - aircraft type ID (for example: B763)
* alt - altitude at last reported position (hundreds of feet or Flight Level)
* altChange - altitude change indication (for example: "C" if climbing, "D" if descending, and empty if it is level)
* arrivalTime - Actual time of arrival, or null if not arrived yet. UNIX epoch timestamp seconds since 1970
* arrived - true if the flight has arrived at its destination.
* cancelled - true if the flight has been cancelled. The meaning of cancellation is that the flight is no longer being tracked by FlightAware. There are a number of reasons a flight may be cancelled including cancellation by the airline, but that will not always be the case.
* cdt - Controlled Departure Time, set if there is a ground hold on the flight. UNIX epoch timestamp seconds since 1970
* clock - Time of last received position. UNIX epoch timestamp seconds since 1970
* cta - Controlled Time of Arrival, set if there is a ground hold on the flight. UNIX epoch timestamp seconds since 1970
* dest - ICAO airport code of destination (for example: KLAX)
* edt - Estimated Departure Time. Epoch timestamp seconds since 1970
* eta - Estimated Time of Arrival. Epoch timestamp seconds since 1970
* fdt - Field Departure Time. UNIX epoch timestamp seconds since 1970
* firstPositionTime - Time when first reported position was received, or 0 if no position has been received yet. Epoch timestamp seconds since 1970
* fixes - intersections and/or VORs along the route (for example: SLS AMERO ARTOM VODIR NOTOS ULAPA ACA NUXCO OLULA PERAS ALIPO UPN GDL KEDMA BRISA CUL PERTI CEN PPE ALTAR ASUTA JLI RONLD LAADY WYVIL OLDEE RAL PDZ ARNES BASET WELLZ CIVET)
* fp - unique identifier assigned by FlightAware for this flight, aka fa_flight_id.
* gs - ground speed at last reported position, in kts.
* heading - direction of travel at last reported position.
* hiLat - highest latitude travelled by flight.
* hiLon - highest longitude travelled by flight.
* ident - flight identifier or registration of aircraft.
* lastPositionTime - Time when last reported position was received, or 0 if no position has been received yet. Epoch timestamp seconds since 1970.
* lat - latitude of last reported position.
* lifeguard - true if a "lifeguard" rescue flight.
* lon - longitude of last reported position.
* lowLat - lowest latitude travelled by flight.
* lowLon - lowest longitude travelled by flight.
* ogta - Original Time of Arrival. UNIX epoch timestamp seconds since 1970
* ogtd - Original Time of Departure. UNIX epoch timestamp seconds since 1970
* orig - ICAO airport code of origin (for example: KIAH)
* physClass - physical class (for example: J is jet)
* prefix - A one or two character identifier prefix code (common values: G or GG Medevac, L Lifeguard, A Air Taxi, H Heavy, M Medium).
* speed - ground speed, in kts.
* status - Single letter code for current flight status, can be S Scheduled, F Filed, A Active, Z Completed, or X Cancelled.
* updateType - data source of last position (P=projected, O=oceanic, Z=radar, A=ADS-B, M=multilateration, D=datalink, X=surface and near surface (ADS-B and ASDE-X), S=space-based).
* waypoints - all of the intersections and VORs comprising the route
- **`flight-goat-pp-cli flights get-by-position-search`** - Returns flight positions based on geospatial search parameters.  This
allows you to locate flights that have ever flown within a specific a
latitude/longitude box, groundspeed, and altitude. It takes search
terms in a single string comprising of {operator key value} elements
and returns an array of flight structures. Each search term must be
enclosed in curly braces. Multiple search terms can be combined in an
implicit boolean "and" by separating the terms with at least one space.
This function only searches flight data representing approximately the
last 24 hours.

The supported operators include (note that operators take different numbers of arguments):

* false - results must have the specified boolean key set to a value of false. Example: {false preferred}
* true - results must have the specified boolean key set to a value of true. Example: {true preferred}
* null - results must have the specified key set to a null value. Example: {null waypoints}
* notnull - results must have the specified key not set to a null value. Example: {notnull aircraftType}
* = - results must have a key that exactly matches the specified value. Example: {= fp C172}
* != - results must have a key that must not match the specified value. Example: {!= prefix H}
* < - results must have a key that is lexicographically less-than a specified value. Example: {< arrivalTime 1276811040}
* \> - results must have a key that is lexicographically greater-than a specified value. Example: {> speed 500}
* <= - results must have a key that is lexicographically less-than-or-equal-to a specified value. Example: {<= alt 8000}
* \>= - results must have a key that is lexicographically greater-than-or-equal-to a specified value.
* match - results must have a key that matches against a case-insensitive wildcard pattern. Example: {match ident AAL*}
* notmatch - results must have a key that does not match against a case-insensitive wildcard pattern. Example: {notmatch aircraftType B76*}
* range - results must have a key that is numerically between the two specified values. Example: {range alt 8000 20000}
* in - results must have a key that exactly matches one of the specified values. Example: {in orig {KLAX KBUR KSNA KLGB}}

The supported key names include (note that not all of these key names are returned in the result structure, and some have slightly different names):

* alt - Altitude, measured in hundreds of feet or Flight Level.
* altChange - a one-character code indicating the change in altitude.
* cid - a three-character cid code
* clock - UNIX epoch timestamp seconds since 1970
* fp - unique identifier assigned by FlightAware for this flight, aka fa_flight_id.
* gs - ground speed, measured in kts.
* lat - latitude of the reported position.
* lon - longitude of the reported position
* updateType - source of the last reported position (P=projected, O=oceanic, Z=radar, A=ADS-B, M=multilateration, D=datalink, X=surface and near surface (ADS-B and ASDE-X), S=space-based)
- **`flight-goat-pp-cli flights get-by-search`** - Search for airborne flights by matching against various parameters including
geospatial data. Uses a simplified query syntax compared to
/flights/search/advanced.
- **`flight-goat-pp-cli flights get-count-by-search`** - Full search query documentation is available at the /flights/search
endpoint.

### foresight

Foresight endpoints provide access to FlightAware's Foresight predictive models and
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

- **`flight-goat-pp-cli foresight get-flight-position-with`** - Get flight's current position, including Foresight data
- **`flight-goat-pp-cli foresight get-flight-with`** - Returns the flight info status summary for a registration, ident, or
fa_flight_id, including all available predicted fields.  If a
fa_flight_id is specified then a maximum of 1 flight is returned,
unless the flight has been diverted in which case both the original
flight and any diversions will be returned with a duplicate fa_flight_id.
- **`flight-goat-pp-cli foresight get-flights-by-advanced-search-with`** - Returns currently or recently airborne flights based on geospatial
search parameters. If available, flights' predicted OOOI fields will be
set.

### history

Manage history

- **`flight-goat-pp-cli history get-aircraft-last-flight`** - Returns flight info status summary for an aircraft's last known flight
given its registration. The search is limited to flights flown since
January 1, 2011. On a successful response, the body will contain a
flights array with only 1 element. If a user queries a registration with
it's last known flight before January 1, 2011, an empty flights array will
be returned.
- **`flight-goat-pp-cli history get-flight`** - Returns historical flight info status summary for a registration, ident,
or fa_flight_id. If a fa_flight_id is specified then a maximum of 1
flight is returned, unless the flight has been diverted in which case
both the original flight and any diversions will be returned with a
duplicate fa_flight_id. If a registraion or ident is specified then a
start_date and end_date must be specified. The span between start_date
and end_date can be up to 7 days. No more than 40 pages may be requested
at once. Data is available from now back to 2011-01-01 00:00:00 UTC.

The field `inbound_fa_flight_id` will not be populated by this resource.
- **`flight-goat-pp-cli history get-flight-map`** - Returns a historical flight's track as a base64-encoded image. Image can contain a
variety of additional data layers beyond just the track. Data is available from now back to
2011-01-01T00:00:00Z.
- **`flight-goat-pp-cli history get-flight-route`** - Returns information about a historical flight's filed route including
coordinates, names, and types of fixes along the route.  Not all flight
routes can be successfully decoded by this endpoint, particularly if the
flight is not entirely within the continental U.S. airspace, since this function
only has access to navaids within that area. If data on a waypoint is
missing then the type will be listed as "UNKNOWN". Data is available from now back to
2011-01-01T00:00:00Z.
- **`flight-goat-pp-cli history get-flight-track`** - Returns the track for a historical flight as an array of positions.
Data is available from now back to 2011-01-01T00:00:00Z.

### operators

Manage operators

- **`flight-goat-pp-cli operators get`** - Returns information for an operator such as their name, ICAO/IATA
codes, headquarter location, etc.
- **`flight-goat-pp-cli operators get-all`** - Returns list of operator references (ICAO/IATA codes and URLs to access
more information).

### schedules

Manage schedules

- **`flight-goat-pp-cli schedules get-by-date`** - Returns scheduled flights that have been published by airlines. These
schedules are available for up to three months in the past as well as
one year into the future.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
flight-goat-pp-cli airports get mock-value

# JSON for scripting and agents
flight-goat-pp-cli airports get mock-value --json

# Filter to specific fields
flight-goat-pp-cli airports get mock-value --json --select id,name,status

# Dry run — show the request without sending
flight-goat-pp-cli airports get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
flight-goat-pp-cli airports get mock-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-flight-goat -g
```

Then invoke `/pp-flight-goat <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/flight-goat/cmd/flight-goat-pp-mcp@latest
```

Then register it:

```bash
claude mcp add flight-goat flight-goat-pp-mcp -e FLIGHT_GOAT_API_KEY_AUTH=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/flight-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FLIGHT_GOAT_API_KEY_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/flight-goat/cmd/flight-goat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "flight-goat": {
      "command": "flight-goat-pp-mcp",
      "env": {
        "FLIGHT_GOAT_API_KEY_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
flight-goat-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/flight-goat-pp-cli/config.toml`

Environment variables:
- `FLIGHT_GOAT_API_KEY_AUTH`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `flight-goat-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FLIGHT_GOAT_API_KEY_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
