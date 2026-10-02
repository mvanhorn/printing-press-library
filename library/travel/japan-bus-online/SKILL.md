---
name: pp-japan-bus-online
description: Discover Japan Bus Online route IDs and stops, check dated services and fares for a selected party, or read baggage and boarding conditions. Use for Japan Bus Online planning and canonical booking handoff.
author: zjsng
license: Apache-2.0
allowed-tools: Read Bash
---

# Japan Bus Online

This CLI uses anonymous public English pages over standard Go HTTP. A browser is unnecessary at runtime. Source sessions are temporary and memory-only.

Use the existing local binary or build from this directory with `go build -o build/japan-bus-online-pp-cli ./cmd/japan-bus-online-pp-cli`, then add the build directory to PATH. The distribution instructions below require this package to be merged and included in the public catalog. Use this local build until distribution is available.

## Prerequisites: Install the CLI

This skill drives the `japan-bus-online-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install japan-bus-online --cli-only
   ```
2. Verify: `japan-bus-online-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/japan-bus-online/cmd/japan-bus-online-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Workflow

Discover directional timetables, then check dated services and exact boarding-pair fares. Overnight dates and unknown capacity remain explicit.

1. Find the source course ID with `routes list`; retain its canonical URL.
2. Read `bus route` for direction IDs and the published timetable. Timetable rows do not prove seats are for sale.
3. Check `bus services` for an explicit YYYY-MM-DD JST service day. Retain service ID, both dates, availability and booking URL.
4. Run `bus quote --include-stops` to discover real stop IDs for that route, direction, service and fare plan. Repeat with selected stop IDs and party counts.
5. Read `bus conditions` and quote cancellation fees, then hand the canonical booking URL to the traveler.

Completion means source-grounded dates, stops, JPY fare assumptions and capacity uncertainty are clear. Booking, payment, cancellation and account history are outside this CLI.

## Anti-triggers

Use another workflow for bookings, payments, cancellations, account history, guaranteed seat adjacency or group discounts.

## Recipes

### Bound discovery

```bash
japan-bus-online-pp-cli routes list --query Hamamatsu --limit 5 --agent --select routes.id,routes.name,routes.url
```

Return a compact route shortlist.

## Evidence rules

- `bus route.kind=published_schedule_not_inventory` separates the timetable from saleable services. `schedule_row` is a display position, not a stop ID.
- Dates and timestamps use Asia/Tokyo. A departure at 25:00 boards at 01:00 on the next calendar day. Use `boarding.timestamp_jst`, rather than adding a second day to normalized times.
- `sold_out`, `not_on_sale`, `unavailable_unknown_reason`, no services and source date substitution have distinct meanings. An empty requested-day result does not imply sold out.
- Service availability describes the headline route. Quote party evidence uses the selected stop-pair fare table. Positive numeric seat counts are conservative lower bounds; a party larger than that count remains unknown. An explicit zero means capacity insufficient. A transaction limit is separate from capacity. Seat adjacency and gender or seat-plan feasibility remain unknown.
- Quote prices use source Adult and Child age labels in JPY for one way. The total is arithmetic, not a confirmed booking. Group discounts, roundtrip prices and unlabeled infant fares remain unknown. Missing cancellation fees are null; partial fetch failures are reported.
- Only `--language en` is verified. Names remain as the source supplies them; `name_ja` is null when no Japanese name exists.

## Runtime and output

Public searches need no credentials. Run `japan-bus-online-pp-cli doctor --json` for local setup, `which` to locate a capability, and command `--help` for current flags. Provider commands require live data and reject `--data-source local`.

`--agent` supplies JSON, compact output and noninteractive defaults. `--select` projects dotted fields before the agent envelope; `--csv` renders collection rows. `routes list` and `bus services` default to 20 rows, with `--limit` 1-100 and nonnegative `--offset`. The route catalog is one bounded source response filtered locally. `--timeout` bounds the whole command; individual requests also have a 25-second ceiling, a 6 MiB response bound and limited same-provider HTTPS redirects.

Retry later on HTTP 429; errors preserve the failure rather than returning empty inventory. Source HTML may change: report parser failures and use the canonical page. The CLI never advances the booking form, submits passenger information or creates a reservation.

For source maintenance, preserve the provider identity, fare/capacity and overnight-date contracts described above; run the repository verification checks after changes.

## Unique Capabilities

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

## Auth Setup

Public searches need no login. Each command creates a temporary memory-only session; no cookies or credentials are stored.

Run `japan-bus-online-pp-cli doctor` to verify setup.

`--deliver file:<path>` and `--deliver webhook:<url>` copy the full result to the selected sink and also print it to stdout. Redirect stdout when the result should appear only in the sink.

