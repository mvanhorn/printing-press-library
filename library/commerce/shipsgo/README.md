# ShipsGo CLI

**Every ShipsGo endpoint, plus an RFQ workspace, lane analytics, and an offline shipment book no other tool offers.**

shipsgo wraps the full ShipsGo v2 API (ocean + air, 16 endpoints, all CRUD verbs) and layers on top a freight-RFQ workflow that lives entirely in your local SQLite store. Use `rfq compare` to score carriers against your own shipment history, `eta-drift` to catch slipping voyages before customers do, and `credit-budget` to never burn a free-tier credit on a duplicate container.

## Install

The recommended path installs both the `shipsgo-pp-cli` binary and the `pp-shipsgo` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install shipsgo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install shipsgo --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install shipsgo --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install shipsgo --agent claude-code
npx -y @mvanhorn/printing-press-library install shipsgo --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shipsgo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-shipsgo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-shipsgo --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-shipsgo skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-shipsgo. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shipsgo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SHIPSGO_USER_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "shipsgo": {
      "command": "shipsgo-pp-mcp",
      "env": {
        "SHIPSGO_USER_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authenticate by setting `SHIPSGO_USER_TOKEN` to an API token created at shipsgo.com/dashboard/air/integrations/api-tokens. The token sits in a single `X-Shipsgo-User-Token` header on every request — no OAuth, no refresh flow. Use `shipsgo-pp-cli doctor` to confirm the token is loaded and reachable.

## Quick Start

```bash
# Verify SHIPSGO_USER_TOKEN is set and api.shipsgo.com responds.
shipsgo-pp-cli doctor

# Mirror your shipments locally for sub-second queries (a built-in `sync`
# command is planned for v0.2 — for now, append list output to the store).
shipsgo-pp-cli ocean shipments list --json > ~/.shipsgo-pp-cli/store/shipments.jsonl

# See every shipment arriving in the next week across ocean + air.
shipsgo-pp-cli book --eta-within 7d --json

# Open an RFQ workspace for a Shanghai → LA Less-than-Container-Load quote.
shipsgo-pp-cli rfq new Q3-LCL-Asia --lane SHA-LAX

# Attach an existing tracked container to the RFQ (mirrors as a ShipsGo tag).
shipsgo-pp-cli rfq add Q3-LCL-Asia MSCU1234567

# See carrier-by-carrier transit + ETA delta for this RFQ.
shipsgo-pp-cli rfq compare Q3-LCL-Asia --format md

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### RFQ workflow
- **`rfq new`** — Create, populate, and inspect an RFQ workspace that groups multiple tracked shipments under one quote slug.

  _Reach for this when an agent needs to evaluate a freight quote against actual shipments — the RFQ slug becomes the join key for compare, digest, and follower fan-out._

  ```bash
  shipsgo-pp-cli rfq new Q3-LCL-Asia --lane SHA-LAX --carriers COSCO,ONE,MSC
  ```
- **`rfq compare`** — For one RFQ slug, group its shipments by carrier and emit mean actual transit days, ETA-vs-actual delta, and shipment count.

  _This is the literal RFQ question: which carrier actually performs best on this lane. Use when an agent has to pick or recommend a carrier from a quote._

  ```bash
  shipsgo-pp-cli rfq compare Q3-LCL-Asia --format md --agent
  ```
- **`digest`** — Emit a structured table of shipment id, last milestone, current ETA, and ETA-delta since last digest, scoped to an RFQ or a time window.

  _Use Friday afternoons to assemble the importer's weekly status update without screenshotting the dashboard._

  ```bash
  shipsgo-pp-cli digest --rfq Q3-LCL-Asia --since 7d --format md
  ```
- **`followers-broadcast`** — Add or remove one follower email across every shipment in an RFQ in one command, gated by credit-budget and --dry-run.

  _Use when a new stakeholder joins an RFQ — saves dozens of dashboard clicks._

  ```bash
  shipsgo-pp-cli followers-broadcast Q3-LCL-Asia importer@example.com --dry-run
  ```

### Analytics
- **`lane stats`** — Across all locally tracked shipments on an origin→destination lane, report p50/p90 transit days with per-carrier breakdown.

  _Lane analytics are paywalled on commercial alternatives (Freightify, GoComet). Use when planning carrier mix on a specific port pair._

  ```bash
  shipsgo-pp-cli lane stats SHA LAX --since 90d --json
  ```
- **`eta-drift`** — List shipments whose ETA shifted by more than N days since the last sync — sorted by drift magnitude.

  _Drift is the earliest signal of voyage trouble. Use as a watch query before customer-facing status digests._

  ```bash
  shipsgo-pp-cli eta-drift --threshold 2 --since 7d --json
  ```
- **`reliability`** — Median(actual_delivery − original_eta) and on-time percentage for a carrier, optionally scoped to a specific lane.

  _This is the single number that should decide every RFQ. Use when ranking carriers in a quote response._

  ```bash
  shipsgo-pp-cli reliability COSCO --lane SHA-LAX --json
  ```

### Reachability mitigation
- **`credit-budget`** — Read ShipsGo X-RateLimit-* headers from the last response, combine with local dedupe state, and refuse new POSTs when the reserve would be breached.

  _Free tier is 3 credits and the rate limit is 100/min collective. Use before any batch POST loop._

  ```bash
  shipsgo-pp-cli credit-budget --reserve 5 --json
  ```
- **`dupe-check`** — Look up a container number or AWB in the local store and exit non-zero with the existing shipment id if already tracked.

  _Use in any script that may POST a new shipment — saves a credit on every re-run._

  ```bash
  shipsgo-pp-cli dupe-check MSCU1234567 --json
  ```

### Local state that compounds
- **`book`** — Single view across ocean + air shipments, filterable by ETA window, status, tag, port, or carrier — backed by FTS5 and local indices.

  _Replaces the dashboard's daily 'what's arriving this week' query with a sub-second, agent-pipeable command._

  ```bash
  shipsgo-pp-cli book --eta-within 7d --status sailing --json
  ```
- **`webhook tail`** — Tail the locally stored webhook_events table chronologically; supports --since, --shipment, and --watch.

  _Use to audit milestone history when a customer disputes an ETA change._

  ```bash
  shipsgo-pp-cli webhook tail --since 24h --shipment ship_abc123 --json
  ```

## Recipes


### Score an RFQ before accepting a quote

```bash
shipsgo-pp-cli rfq compare Q3-LCL-Asia --json --select carrier,mean_transit_days,on_time_pct,sample_size
```

Returns one row per carrier in the RFQ; the on_time_pct column ranks them.

### Pre-flight before posting a new container

```bash
shipsgo-pp-cli dupe-check MSCU1234567 --json
```

Exit code 3 means already tracked locally. Wrap your batch POST loops with this guard to save free-tier credits.

### Catch slipping voyages each morning

```bash
shipsgo-pp-cli eta-drift --threshold 2 --since 24h --agent --select shipment_id,container,old_eta,new_eta,drift_days
```

Lists only shipments whose ETA shifted ≥ 2 days since yesterday — narrowed to the columns an agent actually needs for an alert.

### Lane health check for a customer call

```bash
shipsgo-pp-cli lane stats SHA LAX --since 90d --json
```

Returns p50/p90 transit days plus per-carrier breakdown for the SHA→LAX lane over the last quarter.

## Usage

Run `shipsgo-pp-cli --help` for the full command reference and flag list.

## Commands

### air

Manage air

- **`shipsgo-pp-cli air create`** - This endpoint allows you to create a new shipment by providing the necessary shipment details. Once
the shipment is successfully created, you need to save the given shipment identifier in your internal system
to use it across the related endpoints.

### Duplicate Shipments

If there is an another shipment with the same `reference` and `awb_number`, the shipment will not be created
(there is no cost), and the system will return a response (`409` - `ALREADY_EXISTS`) indicating details of the
existing shipment. If you don't provide a `reference` field on your request, the system will only check with
the `awb_number`.

**Note:** If you want to add a follower or tag to the existing shipment, you must make new request(s)
to related endpoints using the existing shipment's `id`.

### Concurrent Requests

We process creation requests one by one for your company to prevent race conditions, such as misusing your credits
or creating unnecessary duplicate shipments. You don't need to take any action, this process is handled entirely by
ShipsGo system. However, sending too many concurrent requests can result in longer wait times and errors
(`429` - `TOO_MANY_CONCURRENT_REQUESTS`). If you plan to create a large number of shipments at once, ensure that
requests are sent synchronously.
- **`shipsgo-pp-cli air create-shipments`** - This endpoint allows users to add a new follower to an existing air shipment.
- **`shipsgo-pp-cli air create-shipments-2`** - This endpoint allows users to add a tag to an existing air shipment.
- **`shipsgo-pp-cli air delete`** - This endpoint allows users to delete an existing air shipment.
- **`shipsgo-pp-cli air delete-shipments`** - This endpoint allows users to remove an existing follower from an existing air shipment.
- **`shipsgo-pp-cli air delete-shipments-2`** - This endpoint allows users to remove an existing tag from an existing air shipment.
- **`shipsgo-pp-cli air get`** - This endpoint retrieves the details of an existing air shipment. It returns comprehensive information
about the shipment. This allows users to track and monitor the shipment's progress and status in real-time.
- **`shipsgo-pp-cli air get-shipments`** - This endpoint provides a GeoJSON FeatureCollection with all map-related data for a shipment, including airport
locations, the aircraft’s current position, and past/future paths.

GeoJSON ([RFC 7946](https://datatracker.ietf.org/doc/html/rfc7946)) is a format used to describe geographic data and can
be directly used with most mapping libraries (Leaflet, Mapbox, Google Maps, etc.).

In this endpoint, only `Point` and `LineString` geometry types are used. Each geometry is returned within a Feature as
defined below:

-   Each Feature has a `geometry` (`Point` or `LineString`).
-   Each Feature has a set of `properties` according to its `geometry`.
-   Each Feature has a `status` property, which can be:
    -   `PAST` – The shipment was at this airport/route in the past.
    -   `CURRENT` – The shipment is at this airport/route now.
    -   `FUTURE` – The shipment is expected to be at this airport/route in the future.

#### Point Features

Features with `Point` geometry represent airports. Each `Point` Feature includes the airport’s name, IATA code,
timezone, and country information.

#### LineString Features

Features with `LineString` geometry represent routes between two locations. Each `LineString` Feature includes cargo
information, flight number, related events (`DEP`, `ARR`), and current position (when `status` is `CURRENT`).
- **`shipsgo-pp-cli air list`** - The shipments list endpoint allows you to retrieve a list of airlines. This includes options to apply various
filters and sorting.

**Example #1:** To filter the trackable airlines.

```plain
/air/airlines?filters[status]=eq:ACTIVE
```

**Example #2:** To filter airlines that contain `ARABIA` in their name.

```plain
/air/airlines?filters[name]=contains:ARABIA
```
- **`shipsgo-pp-cli air list-shipments`** - The shipments list endpoint allows you to retrieve a list of your shipments (**basic information**). This includes
options to apply various filters and sorting. To access detailed information (`status_extended`, `movements`,
`followers`, etc.) about a shipment, you should use the **AIR - Details of the Shipment** endpoint.

**Example #1:** **Ongoing** (`EN_ROUTE`) shipments carried by **TURKISH CARGO** (`TK`) or **LUFTHANSA CARGO** (`LH`)
with upcoming arrivals in order.

```plain
/air/shipments
  ?filters[status]=eq:EN_ROUTE
  &filters[airline]=in:TK,LH
  &order_by=date_of_rcf,asc
```

**Example #2:** Shipments that were shipped to **India** (`IN`), between **September 1, 2024**, and
**October 1, 2024** in order.

```plain
/air/shipments
  ?filters[destination_country]=eq:IN
  &filters[date_of_dep]=between:2024-09-01...2024-10-01
  &order_by=date_of_dep,asc
```

**Example #3:** Shipments tagged with `COMPANY_ABC` and created by **John Doe** (`john-doe@example.com`).

```plain
/air/shipments
  ?filters[tags]=with:COMPANY_ABC
  &filters[creator]=eq:john-doe@example.com
```
- **`shipsgo-pp-cli air update`** - This endpoint allows users to update the fields of an existing air shipment.

### ocean

Manage ocean

- **`shipsgo-pp-cli ocean create`** - This endpoint allows you to create a new shipment by providing the necessary shipment details. Once
the shipment is successfully created, you need to save the given shipment identifier in your internal system
to use it across the related endpoints.

### Duplicate Shipments

If a shipment already exists with the same `reference` and `booking_number` or `container_number`, the system will not
create a new shipment (no cost will be incurred) and will return a (`409 - ALREADY_EXISTS`) response containing the
details of the existing shipment.

- If the `reference` is provided in the request, it will always be considered during the duplicate check.
- If both `booking_number` and `container_number` are provided, only the `booking_number` will be considered for the duplicate check.
- If the `booking_number` is not provided, the system will perform the duplicate check using the `container_number`.

**Note:** To add a follower or tag to an existing shipment, you must send a new request to the relevant endpoints using
the existing shipment’s `id`.

### Concurrent Requests

We process creation requests one by one for your company to prevent race conditions, such as misusing your credits
or creating unnecessary duplicate shipments. You don't need to take any action, this process is handled entirely by
ShipsGo system. However, sending too many concurrent requests can result in longer wait times and errors
(`429` - `TOO_MANY_CONCURRENT_REQUESTS`). If you plan to create a large number of shipments at once, ensure that
requests are sent synchronously.
- **`shipsgo-pp-cli ocean create-shipments`** - This endpoint allows users to add a new follower to an existing ocean shipment.
- **`shipsgo-pp-cli ocean create-shipments-2`** - This endpoint allows users to add a tag to an existing ocean shipment.
- **`shipsgo-pp-cli ocean delete`** - This endpoint allows users to delete an existing ocean shipment.
- **`shipsgo-pp-cli ocean delete-shipments`** - This endpoint allows users to remove an existing follower from an existing ocean shipment.
- **`shipsgo-pp-cli ocean delete-shipments-2`** - This endpoint allows users to remove an existing tag from an existing ocean shipment.
- **`shipsgo-pp-cli ocean get`** - This endpoint retrieves the details of an existing ocean shipment. It returns comprehensive information
about the shipment. This allows users to track and monitor the shipment's progress and status in real-time.
- **`shipsgo-pp-cli ocean get-shipments`** - This endpoint provides a GeoJSON FeatureCollection with all map-related data for a shipment, including port locations,
the vessel’s current position, and past/future paths.

GeoJSON ([RFC 7946](https://datatracker.ietf.org/doc/html/rfc7946)) is a format used to describe geographic data and can
be directly used with most mapping libraries (Leaflet, Mapbox, Google Maps, etc.).

In this endpoint, only `Point` and `LineString` geometry types are used. Each geometry is returned within a Feature as
defined below:

-   Each Feature has a `geometry` (`Point` or `LineString`).
-   Each Feature has a set of `properties` according to its `geometry`.
-   Each Feature has a `status` property, which can be:
    -   `PAST` – The shipment was at this port/route in the past.
    -   `CURRENT` – The shipment is at this port/route now.
    -   `FUTURE` – The shipment is expected to be at this port/route in the future.

#### Point Features

Features with `Point` geometry represent ports. Each `Point` Feature includes the port’s name, unlocode, timezone, and
country information.

#### LineString Features

Features with `LineString` geometry represent routes between two locations. Each `LineString` Feature includes vessel
information, voyage number, related events (`DEPA`, `ARRV`), and current position (when `status` is `CURRENT`).
- **`shipsgo-pp-cli ocean list`** - The shipments list endpoint allows you to retrieve a list of carriers. This includes options to apply various
filters and sorting.

**Example #1:** To filter the trackable carriers.

```plain
/ocean/carriers?filters[status]=eq:ACTIVE
```

**Example #2:** To filter carriers that contain `CMA` in their name.

```plain
/ocean/carriers?filters[name]=contains:CMA
```
- **`shipsgo-pp-cli ocean list-shipments`** - The shipments list endpoint allows you to retrieve a list of your shipments (**basic information**) including options
for applying various filters and sorting. To access detailed information (such as `route details`, `container movements`,
`followers`, etc.) about a shipment, use the **OCEAN - Details of the Shipment** endpoint.

**Example #1:** Ongoing (SAILING) shipments carried by **MSC** (MSCU) or **OOCL** (OOLU) with upcoming
arrivals, listed in order.

```plain
/ocean/shipments
  ?filters[status]=eq:SAILING
  &filters[carrier]=in:MSCU,OOLU
  &order_by=date_of_discharge,asc
```

**Example #2:** Shipments loaded from **India** (IN), between **September 1, 2024**, and **October 1, 2024**, listed in
order of loading date.

```plain
/ocean/shipments
  ?filters[port_of_loading_country]=eq:IN
  &filters[date_of_loading]=between:2024-09-01...2024-10-01
  &order_by=date_of_loading,asc
```

**Example #3:** Shipments tagged with `COMPANY_ABC` and created by **John Doe** (`john-doe@example.com`).

```plain
/ocean/shipments
  ?filters[tags]=with:COMPANY_ABC
  &filters[creator]=eq:john-doe@example.com
```
- **`shipsgo-pp-cli ocean update`** - This endpoint allows users to update the fields of an existing ocean shipment.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
shipsgo-pp-cli air list

# JSON for scripting and agents
shipsgo-pp-cli air list --json

# Filter to specific fields
shipsgo-pp-cli air list --json --select id,name,status

# Dry run — show the request without sending
shipsgo-pp-cli air list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
shipsgo-pp-cli air list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
shipsgo-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/shipsgo-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SHIPSGO_USER_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `shipsgo-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `shipsgo-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SHIPSGO_USER_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 TOKEN_MISSING** — export SHIPSGO_USER_TOKEN=<token-from-dashboard>; then re-run.
- **429 rate limited** — shipsgo-pp-cli credit-budget shows the live counter; sleep or back off when X-RateLimit-Remaining is low.
- **Free trial credits exhausted** — shipsgo-pp-cli dupe-check <container> before every POST; new credits require a paid plan upgrade.
- **Empty rfq compare table** — Sample size is too small — wait for shipments to complete, or scope the RFQ wider with `rfq add`.
- **Webhook tail empty** — Webhook payloads must be piped to `shipsgo-pp-cli webhook ingest` from your own receiver. Configure the URL in the ShipsGo dashboard.

## Roadmap (v0.2)

The full REST surface (22 endpoints across ocean + air shipments, carriers, airlines, followers, tags, and GeoJSON routes) ships in v0.1.

The following **transcendence commands** are scaffolded in v0.1 but currently print a `v0.2` placeholder message. They all depend on a local SQLite shipment mirror that v0.1 does not include:

| Command | Purpose | Depends on |
|---------|---------|------------|
| `book` | Unified shipment book across ocean + air with ETA/status/tag filters | SQLite mirror |
| `credit-budget` | Read `X-RateLimit-*` headers + local dedupe before write calls | SQLite mirror + last-response cache |
| `digest --rfq <slug>` | Last-milestone + ETA-delta digest across all RFQ members | SQLite mirror + RFQ tables |
| `dupe-check <ref>` | Non-zero exit if container/AWB already tracked | SQLite mirror |
| `eta-drift --since <dur>` | Shipments whose ETA shifted > threshold | SQLite mirror + `eta_history` table |
| `followers-broadcast` | Bulk POST followers across every shipment in an RFQ | SQLite mirror + RFQ tables |
| `lane stats <orig> <dest>` | p50/p90 transit days per lane, per-carrier breakdown | SQLite mirror |
| `reliability <carrier>` | Median ETA-vs-actual delta + on-time % | SQLite mirror + completed shipments |
| `rfq new/compare` | RFQ workspace + per-carrier transit comparison | SQLite mirror + RFQ tables |
| `webhook tail` | Chronological view of locally stored webhook events | SQLite mirror + `webhook_events` table |

### Why deferred

The v0.1 generator did not scaffold a local SQLite store. Building these features requires:

1. `internal/store/` with SQLite schema migrations
2. `sync` command to mirror shipments from `/ocean/shipments` + `/air/shipments` into the store
3. FTS5 search index
4. `eta_history` + `rfqs` + `rfq_members` + `webhook_events` tables
5. Per-command SQL aggregations

Roughly 1500–2500 lines of Go. Tracked for v0.2.

### What v0.1 IS good for

- Calling every ShipsGo REST endpoint with `--json` / `--select` / `--dry-run` / typed exit codes
- Running as an MCP server (`shipsgo-pp-mcp`) — every endpoint becomes an MCP tool automatically
- Operator/debug use: list shipments, check a single track, query carrier metadata, validate API key with `doctor`
