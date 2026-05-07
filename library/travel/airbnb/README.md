# Airbnb CLI (`airbnb-pp-cli`)

**Skip the Airbnb platform fee. Find the host's direct booking site for any Airbnb listing.**

Search Airbnb from the terminal, run cheapest on a listing to extract the host's brand, web-search for their direct booking site, and report the lowest of three prices side-by-side. Cross-platform match, price-drop watchlist, host portfolio analysis, and trip planning all built on a local store that compounds over time.

> **VRBO is currently disabled.** VRBO's anti-bot protection (Akamai) blocks the scraper, and the previous fallback path returned hardcoded placeholder data with the queried city stamped on top — fake results labeled as if they were real. Every VRBO entry point now returns "vrbo is temporarily disabled — pending Akamai workaround". The VRBO source code stays in tree so re-enabling is a flag flip plus the Akamai workaround. Use this CLI for Airbnb work; expect VRBO to come back in a future release.

> **Renamed from `airbnb-vrbo-pp-cli` to `airbnb-pp-cli`.** Existing users get an automatic one-time state migration on first run (the `~/.airbnb-vrbo-pp-cli` directory is renamed in place to `~/.airbnb-pp-cli`; the same applies to the SQLite cache under `~/.local/share`). `AIRBNB_VRBO_*` env vars are still read but emit a deprecation warning; switch to `AIRBNB_PP_*`. The legacy paths will be dropped in a future release.

## Install

### Binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airbnb-pp-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

### Go

```
go install github.com/mvanhorn/printing-press-library/library/travel/airbnb/cmd/airbnb-pp-cli@latest
```

## Authentication

Public search and listing detail need no auth. Authenticated features (Airbnb wishlists, trip history) use cookie import via auth login --chrome. The web-search backend is pluggable: Parallel.ai (paid, best), DuckDuckGo HTML (free default), Brave Search API (free tier), or Tavily (free tier).

## Quick Start

```bash
# Verify reachability and which search backend is active.
airbnb-pp-cli doctor


# Find listings on Airbnb.
airbnb-pp-cli airbnb search 'Lake Tahoe' --checkin 2026-05-16 --checkout 2026-05-19 --guests 4


# Same query, VRBO side.
airbnb-pp-cli vrbo search 'Lake Tahoe' --checkin 2026-05-16 --checkout 2026-05-19 --guests 4


# The headline command. Find the host's direct booking site and the cheapest of three sources.
airbnb-pp-cli cheapest 'https://www.airbnb.com/rooms/37124493?check_in=2026-05-16&check_out=2026-05-19'


# One call: search both platforms, find direct sites, return ranked-by-savings list.
airbnb-pp-cli plan 'Lake Tahoe' --checkin 2026-05-16 --checkout 2026-05-19 --guests 4 --budget 1500

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Host-direct arbitrage
- **`cheapest`** — Given an Airbnb or VRBO listing URL, find the host's direct booking site and report the cheapest of three sources.

  _When a user names an Airbnb/VRBO listing, this is the right tool to reach for. Returns a structured comparison of OTA fees vs direct booking with actionable URLs._

  ```bash
  airbnb-pp-cli cheapest 'https://www.airbnb.com/rooms/37124493?check_in=2026-05-16&check_out=2026-05-19' --agent
  ```
- **`plan`** — Search Airbnb and VRBO in parallel for a city/dates/budget, then run cheapest on the top results, return a ranked-by-savings list.

  _The agent-friendly trip planner. One call returns ranked results across both platforms with direct-booking URLs and savings amounts._

  ```bash
  airbnb-pp-cli plan 'Lake Tahoe' --checkin 2026-05-16 --checkout 2026-05-19 --guests 4 --budget 1500 --agent
  ```
- **`compare`** — Side-by-side: OTA total (with cleaning + service + tax fees) vs direct booking total, with dollar and percent savings.

  _Use when an agent needs to justify a booking recommendation with concrete savings numbers._

  ```bash
  airbnb-pp-cli compare 'https://www.airbnb.com/rooms/37124493' --checkin 2026-05-16 --checkout 2026-05-19 --json
  ```
- **`find-twin`** — Reverse image search a listing's photos to find the same property on direct booking sites or alternate platforms.

  _When host extraction fails (vague host name), reverse image search is the most reliable signal._

  ```bash
  airbnb-pp-cli find-twin 'https://www.airbnb.com/rooms/37124493' --json
  ```

### Cross-platform
- **`match`** — Given a listing on Airbnb (or VRBO), find the same property on the other platform via geocode + amenities + photo signal.

  _Cross-platform price discrimination is real; the same condo can cost 15 percent less on VRBO. This finds it._

  ```bash
  airbnb-pp-cli match 'https://www.airbnb.com/rooms/37124493' --json
  ```

### Local state that compounds
- **`watch`** — Add saved listings to a watchlist with target prices; daily sync checks for drops; cron-friendly exit codes signal hits.

  _Use when a user is shopping a specific listing and waiting for a price drop. Schedule watch check daily; act on exit code 7._

  ```bash
  airbnb-pp-cli watch add 'https://www.airbnb.com/rooms/37124493' --max-price 350 --checkin 2026-05-16 --checkout 2026-05-19
  ```
- **`host portfolio`** — Given a host or property management company name, list every known listing under them across Airbnb and VRBO.

  _Discover bulk patterns: which PMCs operate in this city, which have direct sites, where to focus arbitrage._

  ```bash
  airbnb-pp-cli host portfolio 'Vacasa' --json --select listings.title,listings.location
  ```
- **`wishlist diff`** — Track price changes on Airbnb wishlists over time; report which saved listings dropped, by how much, and over what window.

  _User saved a listing months ago and forgot. This surfaces price movement so they can act before booking._

  ```bash
  airbnb-pp-cli wishlist diff --since 2026-04-01 --json
  ```
- **`fingerprint`** — Stable hash from photos + amenities + host + city; used by match for dedupe; exposed for power-user export workflows.

  _Build your own external joins on listings; stable across sessions._

  ```bash
  airbnb-pp-cli fingerprint 'https://www.airbnb.com/rooms/37124493'
  ```

## Usage

Run `airbnb-pp-cli --help` for the full command reference and flag list.

## Commands

### airbnb_listing

Airbnb listings (search and detail) via SSR HTML scrape (openbnb pattern, no auth required).

- **`airbnb-pp-cli airbnb_listing get`** - Get full Airbnb listing detail (amenities, house rules, location, highlights, description, policies, host) via SSR HTML scrape.
- **`airbnb-pp-cli airbnb_listing search`** - Search Airbnb listings by location, dates, and guest count via the public SSR HTML page (openbnb pattern). Walks niobeClientData[0][1] Apollo cache to extract structured results.

### airbnb_wishlist

Airbnb wishlists (read user's saved listings; requires auth login --chrome).

- **`airbnb-pp-cli airbnb_wishlist items`** - Get items in a specific wishlist by listing IDs.
- **`airbnb-pp-cli airbnb_wishlist list`** - List the user's wishlists via Airbnb's GraphQL persisted query.

### host

Host identity extraction (the linchpin of host-direct arbitrage).

- **`airbnb-pp-cli host extract`** - Extract the host's brand or display name from a listing URL across both platforms. Uses propertyManagement.name (PMC) > host.displayName > description bio > photo signal.
- **`airbnb-pp-cli host portfolio`** - List every known listing under one host or PMC across Airbnb and VRBO from the local store.

### vrbo_listing

VRBO listings (search and detail) via /graphql with Akamai warmup pattern.

- **`airbnb-pp-cli vrbo_listing get`** - Get full VRBO property detail via the propertyDetail GraphQL operation (operation name discovered at runtime). Falls back to __PLUGIN_STATE__ SSR scrape for basic fields.
- **`airbnb-pp-cli vrbo_listing search`** - Search VRBO properties via the propertySearch GraphQL operation. Uses Akamai warmup (GET / first, wait 1.5s, then POST).


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
airbnb-pp-cli airbnb_listing get

# JSON for scripting and agents
airbnb-pp-cli airbnb_listing get --json

# Filter to specific fields
airbnb-pp-cli airbnb_listing get --json --select id,name,status

# Dry run — show the request without sending
airbnb-pp-cli airbnb_listing get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
airbnb-pp-cli airbnb_listing get --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `AIRBNB_PP_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `airbnb-pp-cli airbnb_wishlist`
- `airbnb-pp-cli airbnb_wishlist items`
- `airbnb-pp-cli airbnb_wishlist list`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-airbnb -g
```

Then invoke `/pp-airbnb <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/airbnb/cmd/airbnb-pp-mcp@latest
```

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
airbnb-pp-cli auth login --chrome

claude mcp add airbnb-pp airbnb-pp-mcp
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-airbnb --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-airbnb --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-airbnb skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-airbnb. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
airbnb-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airbnb-pp-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/airbnb/cmd/airbnb-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "airbnb-pp": {
      "command": "airbnb-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
airbnb-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/airbnb-pp-cli/config.toml`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `airbnb-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **VRBO returns 'Bot or Not?' challenge** — The CLI uses Surf with Chrome TLS impersonation and a warmup pattern by default. If you still see challenges: run airbnb-pp-cli doctor --probe vrbo to verify the warmup is firing; reduce request rate with --rate 0.3 (req/sec).
- **cheapest returns 'no direct site found'** — Try --search-backend parallel or --search-backend brave for higher-quality results; ensure PARALLEL_API_KEY or BRAVE_SEARCH_API_KEY is set in env.
- **Airbnb wishlist commands fail with auth error** — Run airbnb-pp-cli auth login --chrome to import your Chrome cookies; airbnb-pp-cli auth status verifies.
- **watch check exits with code 7 unexpectedly** — Code 7 means at least one watched listing dropped below threshold — that is the success path. Check with airbnb-pp-cli watch list --since 24h.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://a0.muscache.com/airbnb/static/packages/web/en/e67b.b1e4978dd5.js
- Capture coverage: 66 API entries from 299 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rpc_envelope (80% confidence), rest_json (75% confidence)
- Auth signals: api_key — headers: X-Airbnb-API-Key, X-Goog-Api-Key
- Generation hints: has_rpc_envelope, weak_schema_confidence
- Candidate command ideas: create_GetViewportInfo — Derived from observed POST /$rpc/google.internal.maps.mapsjs.v1.MapsJsInternalService/GetViewportInfo traffic.; create_StaysPdpSections — Derived from observed POST /api/v3/StaysPdpSections/{hash} traffic.; create_get_data_layer_variables — Derived from observed POST /api/v2/get-data-layer-variables traffic.; create_js — Derived from observed POST /js/ traffic.; create_marketing_event_tracking — Derived from observed POST /api/v2/marketing_event_tracking traffic.; create_messages — Derived from observed POST /tracking/jitney/logging/messages traffic.; create_realtimeconversion — Derived from observed POST /track/realtimeconversion traffic.; get_GetConsentFlagsQuery — Derived from observed GET /api/v3/GetConsentFlagsQuery/{hash} traffic.

Warnings from discovery:
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**openbnb-org/mcp-server-airbnb**](https://github.com/openbnb-org/mcp-server-airbnb) — TypeScript (443 stars)
- [**Apify ecomscrape/vrbo-property-search-scraper**](https://apify.com/ecomscrape/vrbo-property-search-scraper) — JavaScript
- [**Apify easyapi/vrbo-property-listing-scraper**](https://apify.com/easyapi/vrbo-property-listing-scraper) — JavaScript
- [**Apify jupri/vrbo-property**](https://apify.com/jupri/vrbo-property) — JavaScript
- [**markswendsen-code/mcp-vrbo**](https://github.com/markswendsen-code/mcp-vrbo) — TypeScript
- [**vedantparmar12/airbnb-mcp**](https://github.com/vedantparmar12/airbnb-mcp) — TypeScript
- [**Edioff/vrbo-scraper**](https://github.com/Edioff/vrbo-scraper) — Python
- [**Stevesie VRBO API**](https://stevesie.com/apps/vrbo-api) — Documentation

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
