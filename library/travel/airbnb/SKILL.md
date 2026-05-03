---
name: pp-airbnb
description: "Skip the Airbnb platform fee. Find the host's direct booking site for any Airbnb listing. Trigger phrases: `find the direct booking site`, `skip the airbnb fee`, `vacation rental cheapest`, `book direct`, `use airbnb-pp`, `run airbnb-pp`. NOTE: VRBO support is currently disabled — pending Akamai workaround."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["airbnb-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/travel/airbnb/cmd/airbnb-pp-cli@latest","bins":["airbnb-pp-cli"],"label":"Install via go install"}]}}'
---

# Airbnb — Printing Press CLI

Search Airbnb from the terminal, run cheapest on a listing to extract the host's brand, web-search for their direct booking site, and report the lowest of three prices side-by-side. Price-drop watchlist, host portfolio analysis, and trip planning all built on a local store that compounds over time.

> **VRBO support is currently disabled.** VRBO's Akamai bot challenge blocks the scraper. Every VRBO entry point (`vrbo-listing search/get`, `match` from a VRBO URL, `find-twin` from VRBO, the VRBO branch of `cheapest` and `plan`) returns the disabled error. The VRBO source code stays in the tree so re-enabling is a flag flip once an Akamai workaround lands.

> **Renamed from `airbnb-vrbo-pp-cli` to `airbnb-pp-cli` (this release).** State directories migrate automatically on first run. `AIRBNB_VRBO_*` env vars are still read but emit a one-time deprecation warning; use `AIRBNB_PP_*` going forward.

## When to Use This CLI

Reach for this CLI when an agent or user names an Airbnb listing URL, asks about Airbnb fees or direct-booking savings, mentions 'book direct,' or wants to plan a trip across listings. The cheapest command is the headline; plan and compare extend it. The local store accumulates listings, hosts, and price history across sessions, so re-running queries gets faster and richer over time.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 66 API entries from 299 total network entries
- Protocols: rpc_envelope (80% confidence), rest_json (75% confidence)
- Auth signals: api_key — headers: X-Airbnb-API-Key, X-Goog-Api-Key
- Generation hints: has_rpc_envelope, weak_schema_confidence
- Candidate command ideas: create_GetViewportInfo — Derived from observed POST /$rpc/google.internal.maps.mapsjs.v1.MapsJsInternalService/GetViewportInfo traffic.; create_StaysPdpSections — Derived from observed POST /api/v3/StaysPdpSections/{hash} traffic.; create_get_data_layer_variables — Derived from observed POST /api/v2/get-data-layer-variables traffic.; create_js — Derived from observed POST /js/ traffic.; create_marketing_event_tracking — Derived from observed POST /api/v2/marketing_event_tracking traffic.; create_messages — Derived from observed POST /tracking/jitney/logging/messages traffic.; create_realtimeconversion — Derived from observed POST /track/realtimeconversion traffic.; get_GetConsentFlagsQuery — Derived from observed GET /api/v3/GetConsentFlagsQuery/{hash} traffic.
- Caveats: empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

## Command Reference

**airbnb_listing** — Airbnb listings (search and detail) via SSR HTML scrape (openbnb pattern, no auth required).

- `airbnb-pp-cli airbnb_listing get` — Get full Airbnb listing detail (amenities, house rules, location, highlights, description, policies, host) via SSR...
- `airbnb-pp-cli airbnb_listing search` — Search Airbnb listings by location, dates, and guest count via the public SSR HTML page (openbnb pattern). Walks...

**airbnb_wishlist** — Airbnb wishlists (read user's saved listings; requires auth login --chrome).

- `airbnb-pp-cli airbnb_wishlist items` — Get items in a specific wishlist by listing IDs.
- `airbnb-pp-cli airbnb_wishlist list` — List the user's wishlists via Airbnb's GraphQL persisted query.

**host** — Host identity extraction (the linchpin of host-direct arbitrage).

- `airbnb-pp-cli host extract` — Extract the host's brand or display name from a listing URL across both platforms. Uses propertyManagement.name...
- `airbnb-pp-cli host portfolio` — List every known listing under one host or PMC across Airbnb and VRBO from the local store.

**vrbo_listing** — VRBO listings (search and detail) via /graphql with Akamai warmup pattern.

- `airbnb-pp-cli vrbo_listing get` — Get full VRBO property detail via the propertyDetail GraphQL operation (operation name discovered at runtime). Falls...
- `airbnb-pp-cli vrbo_listing search` — Search VRBO properties via the propertySearch GraphQL operation. Uses Akamai warmup (GET / first, wait 1.5s, then POST).


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `AIRBNB_PP_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `airbnb-pp-cli airbnb_wishlist`
- `airbnb-pp-cli airbnb_wishlist items`
- `airbnb-pp-cli airbnb_wishlist list`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
airbnb-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find the cheapest way to book a saved Airbnb listing

```bash
airbnb-pp-cli cheapest 'https://www.airbnb.com/rooms/37124493' --checkin 2026-05-16 --checkout 2026-05-19 --json --select savings,direct.url,direct.total,airbnb.total
```

Returns just the savings and price comparison fields, not the full host-extraction trace.

### Plan a 4-guest Tahoe trip across both platforms

```bash
airbnb-pp-cli plan 'Lake Tahoe' --checkin 2026-05-16 --checkout 2026-05-19 --guests 4 --budget 1500 --agent
```

Fans out across Airbnb + VRBO + direct discovery; ranks by total savings.

### Watch for price drops on saved listings

```bash
airbnb-pp-cli watch add 'https://www.airbnb.com/rooms/37124493' --max-price 350 && airbnb-pp-cli watch check
```

Add to watchlist, then check with cron — exits 7 when any drop is under threshold.

### Find a Vacasa property in Austin and book direct

```bash
airbnb-pp-cli host portfolio 'Vacasa' --city Austin --agent --select listings.title,listings.direct_url
```

Bulk discovery of one PMC's portfolio with direct URLs already resolved.

### Compare deeply nested response

```bash
airbnb-pp-cli airbnb get 37124493 --agent --select listing.title,listing.coordinate.latitude,listing.coordinate.longitude,pricingQuote.structuredStayDisplayPrice.primaryLine.price,pricingQuote.structuredStayDisplayPrice.secondaryLine.price
```

Use --select with dotted paths to narrow Airbnb's deeply nested SSR Apollo cache to just the high-gravity fields.

## Auth Setup

Public search and listing detail need no auth. Authenticated features (Airbnb wishlists, trip history) use cookie import via auth login --chrome. The web-search backend is pluggable: Parallel.ai (paid, best), DuckDuckGo HTML (free default), Brave Search API (free tier), or Tavily (free tier).

Run `airbnb-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  airbnb-pp-cli airbnb_listing get --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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
airbnb-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
airbnb-pp-cli feedback --stdin < notes.txt
airbnb-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.airbnb-pp-cli/feedback.jsonl`. They are never POSTed unless `AIRBNB_PP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AIRBNB_PP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
airbnb-pp-cli profile save briefing --json
airbnb-pp-cli --profile briefing airbnb_listing get
airbnb-pp-cli profile list --json
airbnb-pp-cli profile show briefing
airbnb-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `airbnb-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/airbnb/cmd/airbnb-pp-cli@latest
   ```
3. Verify: `airbnb-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/airbnb/cmd/airbnb-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add airbnb-pp-mcp -- airbnb-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which airbnb-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   airbnb-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `airbnb-pp-cli <command> --help`.
