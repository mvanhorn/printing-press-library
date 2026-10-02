# Japan Bus Online CLI

**Read-only bus routes, dated inventory and source-grounded fares.**

Discover directional timetables, then check dated services and exact boarding-pair fares. Overnight dates and unknown capacity remain explicit.

Created by [@zjsng](https://github.com/zjsng) (zjsng).

## Install

After this package is merged and the catalog workflows finish, install it with:

```bash
npx -y @mvanhorn/printing-press-library install japan-bus-online --cli-only
```

Until the package is available in the public catalog, build locally.

## Build locally

Requires Go 1.26.6 or newer.

```bash
go build -o build/japan-bus-online-pp-cli ./cmd/japan-bus-online-pp-cli
go build -o build/japan-bus-online-pp-mcp ./cmd/japan-bus-online-pp-mcp
./build/japan-bus-online-pp-cli doctor --json
```

Use `./build/japan-bus-online-pp-cli` below, or add the build directory to PATH. SKILL.md contains the agent workflow. Public installer and release URLs become available after this package is merged and the catalog workflows finish.

## Plan a trip

Replace the example date with the traveler's actual service day.

```bash
japan-bus-online-pp-cli routes list --query Hamamatsu --limit 5 --json --select routes.id,routes.name,routes.url
japan-bus-online-pp-cli bus route --route 12200160001 --agent
japan-bus-online-pp-cli bus services --route 12200160001 --direction 0 --date 2026-10-10 --agent
japan-bus-online-pp-cli bus quote --route 12200160001 --direction 0 --date 2026-10-10 --service 0001 --include-stops --agent
japan-bus-online-pp-cli bus quote --route 12200160001 --direction 0 --date 2026-10-10 --service 0001 --dep-stop 8 --arr-stop 9 --adults 2 --children 1 --agent
japan-bus-online-pp-cli bus conditions --route 12200160001 --direction 0 --date 2026-10-10 --agent
```

| Command | Source evidence |
| --- | --- |
| `routes list` | Course IDs, route names and canonical URLs; query, limit and offset |
| `bus route` | Direction IDs, advertised fares, timetable stops, maps and day offsets |
| `bus services` | Exact requested day, services, both overnight dates, headline availability and from fare |
| `bus quote` | Selected stop IDs and timestamps, source Adult/Child fares, party estimate, capacity, transaction cap and cancellation fees |
| `bus conditions` | Route/operator conditions and canonical general policy links |

Omitted service day defaults to seven days after today in JST. Quote defaults to the first service confirmed available, the first departure stop and last arrival stop, one adult, no children and fare plan 1. `--include-stops` exposes the complete stop selection for choosing IDs; they are scoped to route, direction, service and plan.

## Interpret results

A published timetable does not prove bookable inventory. Source `Not Available` stays an unknown-reason unavailability; explicit sold out and not on sale remain separate. Outside-window or silently substituted dates produce no inventory for the requested day.

Dates and normalized timestamps use Asia/Tokyo. For the example service, 2026-10-10 at 23:25 arrives on 2026-10-11. A boarding stop printed as 25:00 is 01:00 on that next day. `schedule_row` is not a booking stop ID.

Headline from fares are separate from actual selected-stop fares. Quote totals are one-way arithmetic in JPY using the source passenger age labels. Group discounts, roundtrip prices and unlabeled infant fares are unknown. A quote is fare evidence; booking confirmation remains necessary.

Party capacity comes from the selected fare table, rather than the headline route. Positive numeric counts are conservative lower bounds because the source may cap its display. Larger parties remain unknown unless the transaction cap prevents the request. Explicit zero reports no seats. Gender or seat-plan feasibility and adjacency remain unknown. Missing cancellation fees are null; failed optional fetches appear under `partial_failures`.

Only English is verified. `name_ja` is null when the source supplies no Japanese name. Each result includes source URL, freshness, request count, upstream response bytes and elapsed time. The canonical `booking_url` hands booking to the traveler. The CLI has no booking, payment, cancellation or account operations.

`--deliver file:<path>` and `--deliver webhook:<url>` copy the full result to the selected sink and also print it to stdout. Redirect stdout when the result should appear only in the sink.

## Output and limits

`--agent` enables JSON, compact output and noninteractive defaults. Use `--json` for the complete payload, dotted `--select` paths for field projection, or `--csv` for collection rows. Provider commands reject `--data-source local`; framework journals and learning data are local and do not cache provider inventory.

Route and service lists default to 20 rows; `--limit` is 1-100 and `--offset` is nonnegative. The catalog uses one source response and local substring filtering. Whole-command `--timeout`, 25-second request ceilings, 6 MiB response limits, rate pacing and same-provider HTTPS redirects bound resource use. HTTP 429 produces a typed failure; retry later. Empty responses or changed HTML fail explicitly.

Run command `--help` or `which` for current capability discovery. Read SKILL.md for agent usage and AGENTS.md for provider maintenance.

## MCP

The local MCP binary mirrors the Cobra command tree. Start `build/japan-bus-online-pp-mcp` with stdio using your client's server configuration. Provider tools carry read-only annotations. No browser-session credentials are required. `tools-manifest.json` is derived from real stdio tools/list; the original endpoint-generation input is labeled separately in `generator-endpoint-provenance.json`.

## Verify

```bash
go test ./...
go vet ./...
```

Real source proofs and final gate reports live in `evidence/`. Synthetic unit fixtures test consequential date, parser and capacity logic; they are separate from live source evidence. HTML layouts and live availability can change after fetching.

## Authentication

Public searches need no login. Each command creates a temporary memory-only session; no cookies or credentials are stored.

## Quick Start

```bash
# Check local setup
japan-bus-online-pp-cli doctor --dry-run

# Find a route
japan-bus-online-pp-cli routes list --query Hamamatsu --limit 5 --agent

# Read the schedule
japan-bus-online-pp-cli bus route --route 12200160001 --agent

```

## Unique Features

### Bus planning
- **`bus services`** — See service-day dates and inventory uncertainty explicitly.

  _Prevent timetable and midnight date mistakes._

  ```bash
  japan-bus-online-pp-cli bus services --route 12200160001 --direction 0 --date 2026-10-10 --agent
  ```
- **`bus quote`** — Read selected-stop Adult/Child fares, one-way party totals and capacity evidence.

  _Avoid treating the headline from-fare as the party total._

  ```bash
  japan-bus-online-pp-cli bus quote --route 12200160001 --direction 0 --date 2026-10-10 --service 0001 --adults 2 --children 1 --agent
  ```
- **`bus route`** — Read directional timetables and mapped stops.

  _Discover boarding location before checking availability._

  ```bash
  japan-bus-online-pp-cli bus route --route 12200160001 --agent
  ```

## Recipes

### Bound discovery

```bash
japan-bus-online-pp-cli routes list --query Hamamatsu --limit 5 --agent --select routes.id,routes.name,routes.url
```

Return a compact route shortlist.
