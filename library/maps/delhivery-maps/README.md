# Delhivery Maps CLI

**Every Delhivery Maps endpoint, plus quota-aware address cleaning, trip costing and fleet planning.**

Run any of the 13 Delhivery Maps endpoints (India only) from the terminal, then batch work with 'address clean', cost a lane with 'trip quote', check service areas with 'coverage check' and plan routes from an orders sheet with 'fleet plan'. The gateway allows about 50 requests per 24 hours per token (route-plan has its own pool of 5), so the batch commands count their own calls, cache results, and mark work they cannot afford as deferred rather than failing.

## Install

The recommended path installs both the `delhivery-maps-pp-cli` binary and the `pp-delhivery-maps` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install delhivery-maps
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install delhivery-maps --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install delhivery-maps --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install delhivery-maps --agent claude-code
npx -y @mvanhorn/printing-press-library install delhivery-maps --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.11 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/maps/delhivery-maps/cmd/delhivery-maps-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/delhivery-maps-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install delhivery-maps --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-delhivery-maps --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-delhivery-maps --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install delhivery-maps --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/delhivery-maps-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `DELHIVERY_MAPS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/maps/delhivery-maps/cmd/delhivery-maps-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "delhivery-maps": {
      "command": "delhivery-maps-pp-mcp",
      "env": {
        "DELHIVERY_MAPS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Delhivery Maps uses a Bearer JWT. The docs site issues a 24-hour web-session token after phone-OTP login (the CLI does not refresh it; when calls start failing with exit code 4, set a fresh one); commercial keys are issued under an enterprise agreement. Set DELHIVERY_MAPS_TOKEN or run 'auth set-token'; 'auth status' reads the token's expiry locally without printing it and costs no quota, whereas 'doctor' makes a live request.

## Quick Start

```bash
# Confirm the install and find the right command; runs offline, no quota spent
delhivery-maps-pp-cli which "toll cost for a truck lane"

# Smallest real call: address to coordinates (spends 1 of your ~50 daily quota calls)
delhivery-maps-pp-cli geocode --address "flat 4b, lotus apt, sec 45, gurgaon, haryana 122003"

# Reverse geocode a coordinate to a structured address (1 quota call)
delhivery-maps-pp-cli rvg --lat 28.877074 --lng 76.619739 --req-id demo-1

# Route plus toll plus ETA in one record (up to 4 quota calls: two geocodes, route, toll)
delhivery-maps-pp-cli trip quote --from "Sector 45 Gurgaon" --to "Jaipur" --vehicle bus_truck

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Bulk address quality
- **`address clean`** — Clean a CSV of raw addresses into standardised, validated, geocoded rows: up to 3 calls per uncached address (about 16 a day per token); overflow rows are marked deferred.

  _Reach for this when you have more than a handful of addresses to standardise, validate or geocode; cached rows cost nothing on a rerun._

  ```bash
  delhivery-maps-pp-cli address clean --file orders.csv --out cleaned.csv --concurrency 4 --resume
  ```
- **`address report`** — See address quality and granularity broken down by pincode or city across everything you have cleaned. Local-only and free; empty until address clean has run.

  _Use this to find which pincodes produce the worst addresses._

  ```bash
  delhivery-maps-pp-cli address report --group-by pincode --agent
  ```
- **`address dupes`** — Find repeated delivery locations among addresses you have cleaned. Local-only and free; empty until address clean has run.

  _Use this to catch duplicate orders to the same doorstep._

  ```bash
  delhivery-maps-pp-cli address dupes --radius-m 30 --agent
  ```

### Dispatch and planning
- **`trip quote`** — Get distance, traffic ETA and toll plazas for a trip in one record. Costs up to 4 calls per lane (geocode from and to, route, toll).

  _Use this to cost a lane or answer 'how long and how much toll' without two separate calls._

  ```bash
  delhivery-maps-pp-cli trip quote --from "Sector 45 Gurgaon" --to "Jaipur" --vehicle bus_truck --agent
  ```
- **`coverage check`** — Flag orders that fall outside a hub's 45-minute (or N km) service area. Costs 1 isochrone call plus 1 call per uncached address.

  _Use this before dispatch to separate serviceable orders from out-of-area ones._

  ```bash
  delhivery-maps-pp-cli coverage check --hub 28.4599,77.0679 --minutes 45 --file orders.csv --out flagged.csv
  ```
- **`fleet plan`** — Turn an orders file of addresses or coordinates into per-vehicle stop sequences. Costs 1 call per 50 orders from a separate 5-per-day route-plan pool; about 30s each.

  _Use this when an orders sheet exceeds the 50-order route-plan limit or has only addresses._

  ```bash
  delhivery-maps-pp-cli fleet plan --orders orders.csv --vehicles 6 --depot 28.4599,77.0679 --out plan.csv
  ```

## Recipes

### Clean an order file

```bash
delhivery-maps-pp-cli address clean --file orders.csv --out cleaned.csv --resume
```

Standardises, validates and geocodes every row, caching results so a rerun skips finished rows.

### Cost a truck lane, narrowed output

```bash
delhivery-maps-pp-cli trip quote --from "Sector 45 Gurgaon" --to "Jaipur" --vehicle bus_truck --agent --select distance_km,total_toll_inr
```

Returns only distance and total toll.

### Service area as GeoJSON

```bash
delhivery-maps-pp-cli isochrone --origin "[28.6139,77.209]" --cost-type time --cost-value 600 --encode-geojson=false
```

Returns a decoded GeoJSON polygon ready for geojson.io or QGIS.

## Usage

Run `delhivery-maps-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `DELHIVERY_MAPS_CONFIG_DIR`, `DELHIVERY_MAPS_DATA_DIR`, `DELHIVERY_MAPS_STATE_DIR`, or `DELHIVERY_MAPS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `DELHIVERY_MAPS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export DELHIVERY_MAPS_HOME=/srv/delhivery-maps
delhivery-maps-pp-cli doctor   # note: makes a live request to the API
```

Under `DELHIVERY_MAPS_HOME=/srv/delhivery-maps`, the four dirs resolve to `/srv/delhivery-maps/config`, `/srv/delhivery-maps/data`, `/srv/delhivery-maps/state`, and `/srv/delhivery-maps/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "delhivery-maps": {
      "command": "delhivery-maps-pp-mcp",
      "env": {
        "DELHIVERY_MAPS_HOME": "/srv/delhivery-maps"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `DELHIVERY_MAPS_DATA_DIR` overrides an explicit `--home` for that kind. Use `DELHIVERY_MAPS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `DELHIVERY_MAPS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `delhivery-maps-pp-cli doctor --fail-on warn` (live request) to check path and credential-location warnings in automation.

## Commands

### autosuggest

Manage autosuggest

- **`delhivery-maps-pp-cli autosuggest`** - Returns ranked location suggestions for a text query across India.
Optional `--lat` and `--lng` bias ranking toward the user's location.

Special handling of `--query`:
- a 6-digit pin code filters to pin code polygons
- a lat/long pair returns a single synthetic coordinate result

### geocode

Manage geocode

- **`delhivery-maps-pp-cli geocode`** - Accepts a raw or semi-structured Indian address and returns geographic
coordinates with an error radius indicating positional confidence.

### isochrone

Reachability polygons — time-based (isochrone) and distance-based (isodistance).

- **`delhivery-maps-pp-cli isochrone`** - Returns a GeoJSON polygon covering everywhere reachable from an origin within a single cost budget.

One endpoint serves both products; `--cost-type` selects which:

| `--cost-type` | Product | `--cost-value` unit | Question answered |
| --- | --- | --- | --- |
| `time` | IsoChrone | seconds | "How far can I get in 10 minutes?" |
| `distance` | IsoDistance | metres | "How far can I get within 5 km of road travel?" |

Everything else (flags, validation, response shape, error codes) is identical between the two modes. The response echoes the mode back in `features[0].properties.cost_type`.

`--direction` flips the direction of travel: `outbound` (default) is where you can reach from the origin; `inbound` is who can reach the origin. The two differ on one-ways, restricted turns and time-dependent costing.

Exactly one contour is requested per call, so the response always contains exactly one feature.

**SERVICE LIMITS**

| `--cost-type` | Unit | Min (exclusive) | Max |
| --- | --- | --- | --- |
| `time` | seconds | 0 | 7,200 |
| `distance` | metres | 0 | 100,000 |

`--travel-mode` accepts `auto`, `motorcycle`, `truck` and `pedestrian`.

`--cost-value` must satisfy `0 < value <= max`. A value of `0`, a negative value, or a value above the max returns HTTP 400. A larger value explores a larger area and costs more latency. `--origin` must lie inside the India boundary.

**RESPONSE GEOMETRY FORMAT**

By default the response uses encoded polyline strings to reduce its size. Pass `--encode-geojson=false` to get standard GeoJSON coordinate arrays in `[longitude, latitude]` order.

| `--encode-geojson` | Response geometry |
| --- | --- |
| omitted or `true` (default) | Encoded polyline strings |
| `=false` | Standard GeoJSON numeric coordinate arrays in `[longitude, latitude]` order |

### matrix

Manage matrix

- **`delhivery-maps-pp-cli matrix`** - Computes distance (km) and duration (seconds) for every source→target pair.
Returns a 2D matrix indexed as `[source_index][target_index]`.

**SERVICE LIMITS**

| Travel Mode | Max Location Pairs | Max Distance |
| --- | --- | --- |
| auto | 40,000 | 6,000 km |
| motorcycle | 40,000 | 6,000 km |
| truck | 40,000 | 6,000 km |
| pedestrian | 40,000 | 200 km |

### raster-tiles

Manage raster tiles

- **`delhivery-maps-pp-cli raster-tiles <size> <z> <x> <y>`** - Serves pre-rendered WebP map tiles of India (XYZ slippy map convention).

**Zoom coverage:** 0–4 country · 5–7 state · 8–10 district · 11–13 city ·
14–16 neighborhood · 17–20 street level.

### route

Manage route

- **`delhivery-maps-pp-cli route`** - Calculates the optimal route between 2+ waypoints within India.
Supports multiple travel modes, alternate routes, and route modifiers.

Geometry is **encoded** by default: both `geometry` and every `legs[].shape` come back as encoded polyline strings (precision 6). Pass `--output-format-encode-polyline=false` to get decoded coordinates instead. Maneuvers are **off** by default; pass `--output-format-maneuvers=true` to include them.

**SERVICE LIMITS**

| Travel Mode | Max Locations |
| --- | --- |
| auto | 20 |
| motorcycle | 50 |
| truck | 20 |
| pedestrian | 50 |

Requests exceeding the max location count return HTTP 400. There is no pre-check for maximum distance.

### route-plan

Manage route plan

- **`delhivery-maps-pp-cli route-plan`** - Solves the Vehicle Routing Problem (VRP) by assigning delivery orders across a fleet of vehicles from a central depot to generate optimized, stop-by-stop routes.

Accepts location data as exact coordinates or text addresses with automatic geocoding. The engine minimizes total transit distance and duration while enforcing per-vehicle weight and volume limits.

**ORDER LOCATION**

Each order must specify either a order coordinate pair (`lat, long`) or an order address. If an address is provided, it is automatically geocoded, and the resolved coordinates are returned in the resulting route stop payload.

**SERVICE LIMITS**

| Constraint | Description |
| --- | --- |
| Orders | Must be between 1 and 50. |
| Order Address | Order address must be located in India. |
| Order Location | Order Location must be within Indian territory. |

### rvg

Manage rvg

- **`delhivery-maps-pp-cli rvg`** - Converts a latitude and longitude into a structured address (Google Geocoding format).
Returns a precise `ROOFTOP` result or multiple `APPROXIMATE` results at descending granularity.

### standarize

Manage standarize

- **`delhivery-maps-pp-cli standarize`** - Accepts a raw or semi-structured Indian address and returns a cleaned,
standardised version with structured components and applied corrections.

### toll-charges-by-od

Manage toll charges by od

- **`delhivery-maps-pp-cli toll-charges-by-od`** - Calculates toll charges for a journey between an origin and destination coordinate pair.
The system computes the route internally and identifies toll plazas along the route.
Supports multiple vehicle types and optionally returns alternate routes with their toll breakdowns.

**SERVICE LIMITS**

| Constraint | Value |
| --- | --- |
| Waypoints (`--waypoints`) | Optional ordered intermediate stops between origin & destination |
| Vehicle types | `motorcycle`, `car_jeep_van`, `lcv`, `bus_truck`, `upto_3_axle`, `axle_4_to_6`, `hcm_eme`, `axle_7_plus` |
| Max routes returned | 1 recommended + alternates |

> When `--waypoints` is supplied, the routing engine returns a single stitched route (no alternates), regardless of `--include-alternates`.

### toll-charges-by-route

Manage toll charges by route

- **`delhivery-maps-pp-cli toll-charges-by-route`** - Calculates toll charges for one or more route polylines provided by the user.

**SERVICE LIMITS**

| Constraint | Value |
| --- | --- |
| Max polylines per request | 3 |
| Min polylines per request | 1 |
| Minimum Polyline Coordinates | 2 coordinate pairs per route |
| Polyline format | Array of `[lat, lon]` pairs |
| Vehicle types | `motorcycle`, `car_jeep_van`, `lcv`, `bus_truck`, `upto_3_axle`, `axle_4_to_6`, `hcm_eme`, `axle_7_plus` |

### validate

Manage validate

- **`delhivery-maps-pp-cli validate`** - Validates an Indian delivery address against a database of historically delivered addresses. Returns quality assessment, granularity, corrected address, and inline diff.

### verify

Manage verify

- **`delhivery-maps-pp-cli verify`** - Checks whether a confirmed delivery has reached a specific address within the last N months. Address validation runs first — the history lookup only proceeds on a valid, premise-level match.

In the response, `is_verified=false` can mean: address could not be resolved, no delivery recorded in the window, address not specific enough (PREMISE level required), or no delivery records found at this location.

**SERVICE LIMITS**

| Parameter | Range |
| --- | --- |
| `--months` min | 1 |
| `--months` max | 24 |


### address

- **`delhivery-maps-pp-cli address clean`** - Clean a CSV or JSONL file of raw addresses into standardised, validated, geocoded rows. Costs up to 3 API calls per uncached address (standardize, validate, geocode), so a token covers roughly 16 addresses a day; rows that do not fit the budget are marked `status=deferred`. Takes `--max-calls`, `--max-rows`, `--force`, `--resume`.
- **`delhivery-maps-pp-cli address report`** - Address quality and granularity grouped by pincode, city, state, quality or granularity; the percentages use only rows that have a validate result. Local-only and free; empty until `address clean` has run.
- **`delhivery-maps-pp-cli address dupes`** - Repeated delivery locations among cleaned addresses. Only rows with granularity PREMISE or STREET_LANDMARK and a known error radius of at most 100 m are compared. Local-only and free; empty until `address clean` has run.
- **`delhivery-maps-pp-cli address purge`** - Local-only maintenance: deletes cached API responses and cleaned addresses from the local database, by age (`--older-than 30d`, the default) or everything (`--all` or `--older-than 0`, which need `--yes`). Never calls the API. See [PII and local storage](#pii-and-local-storage).

### coverage

- **`delhivery-maps-pp-cli coverage check`** - Flag orders outside a hub's 45-minute (or N km) service area. Costs 1 isochrone call plus 1 call per uncached address; addresses beyond the budget are marked `deferred`. Takes `--max-calls`, `--max-rows`, `--force`. `--minutes` is at most 120 and `--km` at most 100.

### fleet

- **`delhivery-maps-pp-cli fleet plan`** - Turn an orders file of addresses or coordinates into per-vehicle stop sequences. Costs 1 call per chunk of up to 50 orders from the separate route-plan pool (about 5 per 24 hours), and each call takes about 30 seconds; chunks beyond the budget are reported as `deferred_chunks`. `--depot lat,lng` is required. Takes `--max-calls`, `--max-rows`, `--force`.

### trip

- **`delhivery-maps-pp-cli trip quote`** - Distance, traffic ETA and toll plazas for a trip in one record. Costs up to 4 calls per lane (geocode from and to if they are addresses, route, toll); lanes beyond the budget are marked `deferred`. Takes `--max-calls`, `--max-rows`, `--force`.

### Self-learning loop (optional, local)

This CLI includes a small local teach/recall loop backed by the SQLite store: `teach` records a question-to-command note and `recall` looks it up later. Every invocation is journaled locally, and nothing leaves your machine. It is optional; this API is stateless, so most users can ignore it. See the `## Automatic learning` section in `SKILL.md` for the agent protocol.

- **`delhivery-maps-pp-cli recall <query>`** - Look up locally taught notes for a question
- **`delhivery-maps-pp-cli teach`** - Record a question-to-command note (silent on success, safe to background with `&`)
- **`delhivery-maps-pp-cli learnings list`** - Inspect taught rows
- **`delhivery-maps-pp-cli learnings forget <query>`** - Undo a teach
- **`delhivery-maps-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`delhivery-maps-pp-cli learnings stats`** - Local loop metrics
- **`delhivery-maps-pp-cli teach-pattern`** - Install a query template up front
- **`delhivery-maps-pp-cli teach-lookup`** - Add an entity alias mapping for pattern substitution

Pass `--no-learn` or set `DELHIVERY_MAPS_NO_LEARN=true` to disable the loop for deterministic flows. Do not teach addresses or other personal data.

The local store's schema version stamp is one-way: once this version of `delhivery-maps-pp-cli` opens the database, older binaries refuse it with a version error. Upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
delhivery-maps-pp-cli geocode --address "flat 4b, lotus apt, sec 45, gurgaon, haryana 122003"

# JSON for scripting and agents
delhivery-maps-pp-cli geocode --address "flat 4b, lotus apt, sec 45, gurgaon, haryana 122003" --json

# Filter to specific fields by name (dotted paths reach nested fields, e.g. metadata.pincode)
delhivery-maps-pp-cli geocode --address "flat 4b, lotus apt, sec 45, gurgaon, haryana 122003" --agent --select lat,lng,error_radius

# Dry run: show the request without sending it (spends no quota)
delhivery-maps-pp-cli raster-tiles 256 5 22 13 --dry-run

# Save a map tile to a file (1 quota call)
delhivery-maps-pp-cli raster-tiles 256 5 22 13 --deliver file:tile.webp

# Agent mode: JSON + compact + no prompts in one flag
delhivery-maps-pp-cli geocode --address "flat 4b, lotus apt, sec 45, gurgaon, haryana 122003" --agent
```

Boolean flags that default to true are turned off with `--flag=false` (for example `--encode-geojson=false`), not `--flag false`.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select field1,field2` returns only the fields you need (dotted paths reach nested fields)
- **Previewable** - `--dry-run` shows the request without sending it
- **Read/compute only** - nothing here creates, updates or deletes remote data; the only deletion is `address purge`, which touches the local database only
- **Quota-aware** - the batch commands (`address clean`, `coverage check`, `trip quote`, `fleet plan`) count their own calls; see [Quota and call costs](#quota-and-call-costs)
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only where a command asks for it (for example `address purge --all`)
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

### Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Success. Batch commands (`address clean`, `coverage check`, `trip quote --file`, `fleet plan`) also exit 0 when a 429 or the local call budget stops the run: the JSON then has `rate_limited: true` and the deferred rows, lanes or chunks |
| `2` | Usage error (bad flags or input, for example a negative `--max-calls`, an over-limit `--minutes`, empty stdin, `--stdin` together with `--file`, or an `--out` that already exists without `--force`) |
| `3` | Not found |
| `4` | Auth error (401/403). Batch commands stop at once and still write partial output |
| `5` | API error. Batch commands also exit 5 when every row, lane or order failed for a reason other than quota; partial output is still written |
| `7` | Rate limited. Applies to single calls, a single-lane `trip quote`, and `coverage check` when its one required isochrone call is blocked by the quota |
| `10` | Config error |

## Quota and call costs

The gateway allows about **50 requests per 24 hours per token**, shared by every endpoint except one: `POST /route-plan` (used by `fleet plan`) has its **own limit of about 5 requests per 24 hours** (the live header `x-ratelimit-limit` reports 5). These are daily limits, not per-minute ones, so plan batch work around them. The four batch commands keep a local call ledger, record a 24-hour lockout after a 429, and defer work they cannot afford instead of failing. The pools are tracked separately: a 429 on one does not lock out the other. The ledger, lockout and cache are scoped per token and base URL.

| Command | Calls spent | Pool |
| --- | --- | --- |
| `address clean` | up to 3 per uncached address (standardize, validate, geocode): about 16 addresses a day per token. Rows that do not fit are written with `status=deferred` | shared pool of 50 |
| `trip quote` | up to 4 per lane (geocode from, geocode to, route, toll); `lat,lng` inputs skip the geocodes | shared pool of 50 |
| `coverage check` | 1 isochrone call plus 1 per uncached address | shared pool of 50 |
| `fleet plan` | 1 per chunk of up to 50 orders, each taking about 30 seconds | separate route-plan pool of 5 |
| `address report`, `address dupes`, `address purge` | none (local-only) | none |
| any single endpoint command (`geocode`, `route`, `validate`, and so on) | 1 each, sent for you but not recorded in the batch ledger (see Limitations) | shared pool of 50 (the `route-plan` endpoint uses the route-plan pool) |

Controls:

- `--max-calls N` exists on `address clean`, `coverage check`, `trip quote` and `fleet plan` and caps the live calls for that run. It can only lower the budget, never raise it. Omit it to use the whole remaining budget; `--max-calls 0` means make no calls at all; a negative value is a usage error. On `fleet plan` it applies to the route-plan pool.
- `DELHIVERY_MAPS_DAILY_BUDGET` changes the assumed shared quota (default 50) and `DELHIVERY_MAPS_PLAN_BUDGET` the assumed route-plan quota (default 5), for example if you have a commercial key.
- `DELHIVERY_MAPS_IGNORE_LOCKOUT=1` ignores the 24-hour lockout recorded after a 429.
- Fully cached rows, lanes and addresses are still served after a 429 or a cap; only items that need a live call are deferred.
- `address clean --resume` skips addresses that already completed and re-runs only the steps still missing for a partially stored row.

Limitations to weigh before relying on the numbers:

- The call counter only sees calls made through the four batch commands; it cannot see other tools or people using the same token, so the real remaining quota can be lower.
- Standalone endpoint commands (`geocode`, `route`, `validate` and the other endpoint commands) and typed MCP endpoint tools spend quota without writing to the ledger. After you run them, the batch commands still count that quota as available, so a batch can start on a budget that is too high. A 429 then locks out the pool and defers the rest of the batch, as described above. Keep a rough count of standalone calls on a shared token, or set `--max-calls` on the batch.
- The 24-hour lockout length is a local policy: the client cannot read the server's reset time.
- For `fleet plan`, orders the solver leaves unassigned are reported as `unassigned_orders`. A response with `plan_status: false` could not be verified against the live service.

## Piping rows in (`--stdin`) and optional `--out`

`address clean`, `coverage check`, `trip quote` and `fleet plan` can read their input rows from standard input with `--stdin` (the same as `--file -`, or `--orders -` for `fleet plan`). Input via `--stdin` needs no files, so agents and scripts can pipe rows straight in. The format (CSV, JSON Lines or a JSON array of objects) is detected automatically, and the same `--max-rows` and file-size caps apply. Empty stdin, or `--stdin` together with a real path, is a usage error (exit 2). `--stdin` is not offered as an MCP tool argument, because an MCP call cannot supply standard input; MCP callers pass a server-local `--file` path instead.

`--out` is optional on all four commands. Without it nothing is written to disk and the rows come back in the output instead: `cleaned` (`address clean`), `flagged` (`coverage check`), `stops` (`fleet plan`) or the lane records (`trip quote`), capped by `--max-rows`. `--csv` prints them as CSV (formula-neutralised) and human mode prints a table. With `--out` the behaviour is unchanged: no overwrite without `--force`, and so on.

```bash
# file form: read a file, write a CSV
delhivery-maps-pp-cli address clean --file orders.csv --out cleaned.csv --resume

# piped form: no files at all (spends up to 3 quota calls per uncached address)
printf 'address\n"flat 4b, lotus apt, sec 45, gurgaon, haryana 122003"\n' | delhivery-maps-pp-cli address clean --stdin --json

# lanes as JSON Lines (up to 4 quota calls per lane)
printf '{"from":"28.5954,77.0319","to":"28.4089,77.3178"}\n' | delhivery-maps-pp-cli trip quote --stdin --json

# orders piped into the route planner (1 call from the separate route-plan pool per 50 orders)
printf 'order_id,lat,lng\nA1,28.48,77.09\nA2,28.50,77.10\n' | delhivery-maps-pp-cli fleet plan --stdin --vehicles 1 --depot 28.4599,77.0679 --json
```

## Input and output safety

- **Overwrite protection.** File-writing commands refuse to overwrite an existing `--out`, or the input file itself (including through a symlink or hard link), unless you pass `--force`. The `--out` parent directory is checked before any API call is made, so a bad path never wastes quota.
- **Spreadsheet formula neutralisation.** CSV, `--csv` and `--plain` output prefix cells that start with `=`, `+`, `-` or `@` with a single apostrophe (plain numbers are kept as numbers). `coverage check` applies the same quoting to the passthrough order columns it copies.
- **Input limits.** `--max-rows` (default 100000, hard cap 200000) is enforced while parsing. Input files must be regular files of at most 64 MiB. Accepted formats are CSV, JSONL (`.jsonl`, `.ndjson`) and `.json` (a JSON array of objects); from `--stdin` the format is detected automatically. `coverage check` accepts `--minutes` up to 120 and `--km` up to 100.
- **Cache.** Responses are cached locally, keyed by base URL plus a fingerprint of the token (the token itself is never stored in the key). TTL is 30 days for geocode, standardize and validate, and 24 hours for route, toll and isochrone; route-plan is never cached. A response that fails validation is not cached. `--no-cache` bypasses the cache.
- **Token safety.** The novel commands send the bearer token only to `https://gateway-maps-pub-int.delhivery.com` (or a loopback host for local tests). Any other base URL is refused with a usage error.

## PII and local storage

The batch commands store data in the local SQLite database (`data.db` in the data directory, file mode 0600): the raw addresses you cleaned, their coordinates, and the raw API responses (tables `dm_cache` and `dm_address`). That data is kept until you delete it. Nothing is uploaded anywhere except to Delhivery's own API as part of the calls themselves.

To remove it:

```bash
# delete cached responses and cleaned addresses older than 30 days (the default)
delhivery-maps-pp-cli address purge --older-than 30d

# delete everything (requires --yes)
delhivery-maps-pp-cli address purge --all --yes
```

`--older-than` takes a day count such as `30d` (at most `36500d`) or a Go duration such as `72h`. `--older-than 0` and `--all` delete everything and require `--yes`. The purge really erases the data (secure delete, WAL checkpoint and VACUUM), never calls the API, and never touches the quota ledger, so the budget guard stays accurate. `address report` and `address dupes` read the same tables, so they are empty after a purge. Avoid `teach` on queries that contain addresses or personal data.

## Health Check

```bash
delhivery-maps-pp-cli auth status       # local only: shows whether a token is set and when it expires
delhivery-maps-pp-cli doctor --dry-run  # prints "would run doctor" and sends nothing; it verifies nothing
delhivery-maps-pp-cli doctor            # LIVE: makes a real request to the API
```

`doctor` verifies configuration, credentials, and connectivity, which means it issues a live request and may count against your daily quota. Use `auth status` for a free local check.

## Configuration

Run `delhivery-maps-pp-cli doctor` (a live request; see Health Check) to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/delhivery-maps-pp-cli/config.toml`; `--home`, `DELHIVERY_MAPS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `DELHIVERY_MAPS_TOKEN` | per_call | Yes | Bearer JWT for the API. The token from the docs site is a 24-hour web-session token and the CLI does not refresh it: when calls start failing with 401/exit 4, run `auth status` and set a fresh token. |
| `DELHIVERY_MAPS_DAILY_BUDGET` | local | No | Assumed shared daily call quota for the local counter used by the batch commands (default 50). |
| `DELHIVERY_MAPS_PLAN_BUDGET` | local | No | Assumed separate daily quota for route-plan, used by `fleet plan` (default 5). |
| `DELHIVERY_MAPS_IGNORE_LOCKOUT` | local | No | Set to `1` to ignore the 24-hour lockout recorded after a 429. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `delhivery-maps-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `delhivery-maps-pp-cli auth status` to check whether a token is set and whether it has expired (a live `doctor` request also works but uses quota)
- Verify the environment variable is set: `test -n "$DELHIVERY_MAPS_TOKEN" && echo set`
**Not found errors (exit code 3)**
- Check the coordinate, address or file argument is correct (there are no list commands; this API is stateless)

### API-specific
- **401 Unauthorized partway through a job** — The 24h web-session token expired; run 'delhivery-maps-pp-cli auth status' then set a fresh DELHIVERY_MAPS_TOKEN
- **429 Rate limit exceeded** — The gateway allows about 50 requests per 24 hours per token, and fleet plan (route-plan) has a separate pool of about 5 per 24 hours. A 429 makes the batch commands lock out that pool for 24 hours (override with DELHIVERY_MAPS_IGNORE_LOCKOUT=1) and defer the rest of the run with exit 0 and rate_limited:true; fully cached items are still served. Rerun after the window with --resume (address clean) so cached rows cost no calls
