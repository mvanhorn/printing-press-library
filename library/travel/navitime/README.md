# NAVITIME CLI

Created by [@zjsng](https://github.com/zjsng) (zjsng).

**Check Japan journeys with dated routes, explicit fares and source-backed pass constraints.**

Resolve stations and places, compare source route alternatives, and inspect legs without losing dates or fare assumptions. Responses are bounded, compact JSON with source and freshness metadata.

## Authentication

The verified Japan Travel website surface requires no API key or subscription. Runtime uses Firefox-compatible HTTP; no running browser or imported cookies.

## Install

Catalog installation is available after this entry merges into the public library:

```sh
npx -y @mvanhorn/printing-press-library install navitime --cli-only
```

For a checkout, build locally:

## Build locally

Requires Go 1.26.6 or newer. From this checkout:

```sh
go build -o navitime-pp-cli ./cmd/navitime-pp-cli
```

Examples use `navitime-pp-cli`; use `./navitime-pp-cli` if it is not on PATH. Choose a future journey date. Offset-free date-times mean **Asia/Tokyo**; returned timestamps carry `+09:00`. Lookup returns candidates for you to resolve, including ambiguous names.

## Quick Start

```bash
# Inspect verified capabilities and access boundaries.
navitime-pp-cli capabilities --agent

# Resolve ambiguity before routing.
navitime-pp-cli places search 大久保 --type station --agent

# Fetch dated route summaries.
navitime-pp-cli routes search --from station:00006668 --to station:00001756 --depart-at 2026-10-01T09:00 --agent

```

## Agent Usage

The focused commands are `places search`, `routes search`, `routes show`, `routes compare`, `passes list`, and `capabilities`. Use their `--help` for current flags. Normal output is compact JSON; diagnostics and optional `--metrics` go to stderr.

```sh
./navitime-pp-cli routes search --from station:00006668 --to station:00001756 --arrive-by 2026-10-01T12:00 --agent --select routes.id,routes.departure_at,routes.arrival_at,routes.fare
./navitime-pp-cli capabilities
```

`--fields` aliases `--select`. Summaries are bounded; `routes show` expands a saved route without fetching it again. Source station/place IDs retain leading zeros. Route IDs identify local content snapshots, because the website supplies alternative positions rather than globally stable journey IDs.

## Health Check

```sh
./navitime-pp-cli doctor --json
./navitime-pp-cli version
```

A successful connection alone is insufficient: route commands also validate the returned query and reject challenges or malformed pages.

## Cookbook

```sh
# Last service for a Japan-local calendar date
./navitime-pp-cli routes search --from station:00006668 --to station:00001756 --last-on 2026-10-01

# Compare the returned alternatives with a walking limit in metres
./navitime-pp-cli routes compare --from station:00006668 --to spot:02301-1300539n --depart-at 2026-10-01T09:00 --sort walking --max-walk 1000

# Discover pass IDs, then ask NAVITIME to prefer one
./navitime-pp-cli passes list
./navitime-pp-cli routes search --from station:00006668 --to station:00001756 --depart-at 2026-10-01T09:00 --pass japan_rail_pass
```

Pass filtering may change services while the website still displays the normal cash fare. The CLI preserves that fare and source coverage/supplement text; it does not calculate pass-holder out-of-pocket cost. Scheduled routes do not establish live operation or seat availability. NAVITIME may also return Car/Taxi estimates; route transport kinds and timing basis distinguish them. Read [data semantics](docs/data-contract.md) when interpreting overnight dates, fare groups or passes.

## Cache and efficiency

Source fetches share the requested `--timeout` budget across attempts (CLI default: one minute) and are capped at 8 MiB, with one in-flight request per client and at most one bounded transient retry. Route query cache TTL is five minutes; lookup and pass-catalogue TTL is 24 hours. `--refresh` bypasses reads and replaces cached results; `--no-cache` bypasses reads and writes. `NAVITIME_CACHE_DIR` or `--cache-dir` isolates cache files. `routes show latest` follows the most recent successful route search, including cache hits. Snapshot details retain their fetch time; they are historical observations, not automatic refreshes.

`passes list` reuses the catalogue captured with route results. When a refresh is needed, it makes one reference Tokyo–Kyoto route request because the route entry page is less reliably accessible. Its source URL and request count make that request explicit. Lookup and route endpoints expose no verified remote pagination; the CLI bounds their returned sets. Pass-catalogue paging is local.

Measured results, request counts, source failures and reproducible commands are in [verification](docs/verification.md).

## Troubleshooting

- **Ambiguous location:** inspect lookup candidates and use the intended `station:` or `spot:` reference.
- **Access blocked / unexpected HTML:** pause before retrying with `--refresh`; rapid uncached requests can receive a temporary challenge. The CLI returns a source error instead of treating the page as a route. This undocumented website surface can change.
- **Missing route snapshot:** run `routes search` with caching enabled, then pass a returned route ID to `routes show`.
- **Unknown price or metric:** JSON `null` means unavailable. Comparisons do not turn unknown values into zero.
- **Date or mode rejected:** provide one dated departure, arrival deadline, first-service date or last-service date. Check the source URL if NAVITIME cannot serve that date.

## Access and boundaries

The shipped adapter uses NAVITIME only. [Access research](docs/access.md) distinguishes the website from the credential-gated official API and lists published API prices. Premium-only timetables/full stop lists, bookings, seat inventory and live disruption checks are outside this CLI's verified surface.

Local builds do not change shared tool or MCP configuration. An optional stdio MCP binary can be built from `./cmd/navitime-pp-mcp`.

Generated with Printing Press; see [SKILL.md](SKILL.md) for the agent workflow and [AGENTS.md](AGENTS.md) for maintenance rules. API-based comparison references: [mcp-navitime-rust](https://github.com/GHagui/mcp-navitime-rust), [navitime-transit-server](https://github.com/asterism45/mcp-servers/tree/main/navitime-transit-server). No source code was copied from those projects.

## Unique Features

### Itinerary evidence
- **`places search`** — Return bilingual candidates without silently choosing a station.

  _Return bilingual candidates without silently choosing a station._

  ```bash
  navitime-pp-cli places search 大久保 --type station --agent
  ```
- **`routes search`** — Check dated departures, deadlines and first/last services.

  _Check dated departures, deadlines and first/last services._

  ```bash
  navitime-pp-cli routes search --from station:00006668 --to station:00001756 --depart-at 2026-10-01T09:00 --agent
  ```
- **`routes show`** — Expand a stored route with full dates and fare assumptions.

  _Inspect overnight boundaries and optional fare supplements before accepting an itinerary._

  ```bash
  navitime-pp-cli routes show --latest --agent
  ```
- **`routes compare`** — Compare the returned journeys by time, fare, walking and transfers.

  _Compare the returned journeys by time, fare, walking and transfers._

  ```bash
  navitime-pp-cli routes compare --from station:00006668 --to station:00001756 --arrive-by 2026-10-01T12:00 --agent
  ```
- **`passes list`** — Discover advertised passes and their verification status.

  _Discover advertised passes and their verification status._

  ```bash
  navitime-pp-cli passes list --agent
  ```

## Recipes

### Resolve a station

```bash
navitime-pp-cli places search 大久保 --type station --agent
```

Choose a source station ref from the candidates.

### Compact route summaries

```bash
navitime-pp-cli routes search --from station:00006668 --to station:00001756 --depart-at 2026-10-01T09:00 --agent --select routes.id,routes.departure_at,routes.arrival_at,routes.fare
```

Keep output focused on the travel window and displayed price.

### Compare a deadline

```bash
navitime-pp-cli routes compare --from station:00006668 --to station:00001756 --arrive-by 2026-10-01T12:00 --agent
```

Compare returned alternatives before an appointment.

### Discover passes

```bash
navitime-pp-cli passes list --agent
```

Use an advertised ID with routes search --pass.
