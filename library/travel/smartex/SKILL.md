---
name: pp-smartex
description: Read-only Japan Shinkansen planning with smartEX: dated adult fares, Tokaido/Sanyo/Kyushu stations, discount deadlines, luggage limits and booking handoff. Use for smartEX planning, compare Shinkansen fares, smartEX booking deadline, oversized baggage, use smartex or run smartex. Exact seat inventory and bookings use the official site.
author: zjsng
license: Apache-2.0
allowed-tools: Read Bash
---

# smartEX planning

## Prerequisites: Install the CLI

This skill drives the `smartex-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install smartex --cli-only
   ```
2. Verify: `smartex-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/smartex/cmd/smartex-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read-only dated adult fares, discount deadlines and baggage checks from smartEX and linked JR sources. Plan with Japanese station names and precise JST dates, then book through the official site.

## When to Use This CLI

Resolve Tokyo–Kagoshima-Chuo Shinkansen stations and compare public fares or booking constraints. For exact trains and seats, generate a canonical `handoff` and use member booking. No booking, payment, account or reservation mutation is supported.

## Choose the Command

- Resolve Japanese names and station IDs with `stations`; Osaka and Shin-Osaka remain distinct.
- Use `fare` for dated basic adult one-way prices, current JST month and next two months. All-class comparison uses four public requests. Child fares and party totals involving children are null.
- Use `products` for current smartEX/Hayatoku rules and sale-window checks; listed routes, exclusion dates, allocated seats and discount prices still require source/booking confirmation. `--detail` fetches current fare-document links.
- Use `window` for precise JST calendar/overnight rules. Confirmation time is null because current Japanese08:00 and English14:00 guidance conflict.
- Use `window --oversized-baggage` with `--class reserved` or `--class green` for class-dependent five/four-person limits. Oversized one-year and overnight eligibility remain unknown because official language editions differ; use the known one-month daytime fallback.
- Use `baggage` for per-piece cm/kg limits. More than160cm through250cm requires a reserved oversized area; special equipment needs operator confirmation.
- Use `timetable` for fresh official publications and a limited2026-03-14 basic example catalog. Operating date and seats are unknown; null arrival means the source provided departure only.
- Use `policy` for boarding, changes and refund conditions; `handoff` supplies a booking checklist.

Policy as-of2026-10-02; embedded rules are a snapshot, not current inventory. An empty timetable example list is incomplete coverage, not no service. Paid-member EX Reservation and free-membership smartEX are distinct. Round-trip smartEX discounts ended2026-03-31.

## Recipes

### fare

```bash
smartex-pp-cli fare --from Tokyo --to Shin-Osaka --date 2026-10-02 --agent --select quotes
```

Compare fresh dated adult fares and membership restrictions

### window

```bash
smartex-pp-cli window --date 2026-10-28 --now 2026-10-02T12:00:00+09:00 --agent
```

Compute precise JST advance request and confirmation boundaries

### baggage

```bash
smartex-pp-cli baggage --length-cm 80 --width-cm 60 --height-cm 40 --weight-kg 20 --agent
```

Check baggage size and required reserved seat class

## Health Check and Recovery

```sh
smartex-pp-cli doctor --json
smartex-pp-cli sources --source timetable --check --limit 1 --agent
smartex-pp-cli reference service --agent
```

Unknown station or invalid input exits2; source/parse/network failure5; throttle7. A source HTTP200 check is page reachability only. On a source change, inspect current reference links or use official handoff. On partial fare failure, successful quotes retain explicit `fetch_failures`; do not total missing classes or unknown child prices.

## Unique Capabilities

These commands return compact, sourced planning output for agent workflows.

### Shinkansen planning
- **`stations`** — Resolve station names and corridor order

  _Use for compact auditable Shinkansen planning before booking_

  ```bash
  smartex-pp-cli stations --query Osaka --agent
  ```
- **`route`** — Identify corridors and train categories with booking links

  _Use for compact auditable Shinkansen planning before booking_

  ```bash
  smartex-pp-cli route --from Tokyo --to Hakata --agent
  ```
- **`fare`** — Compare fresh dated adult fares and membership restrictions

  _Use for compact auditable Shinkansen planning before booking_

  ```bash
  smartex-pp-cli fare --from Tokyo --to Shin-Osaka --date 2026-10-02 --agent
  ```
- **`products`** — Compare discount deadlines and unresolved eligibility conditions

  _Use for compact auditable Shinkansen planning before booking_

  ```bash
  smartex-pp-cli products --date 2026-10-28 --now 2026-10-02T12:00:00+09:00 --agent
  ```
- **`window`** — Compute precise JST advance request and confirmation boundaries

  _Use for compact auditable Shinkansen planning before booking_

  ```bash
  smartex-pp-cli window --date 2026-10-28 --now 2026-10-02T12:00:00+09:00 --agent
  ```
- **`baggage`** — Check baggage size and required reserved seat class

  _Use for compact auditable Shinkansen planning before booking_

  ```bash
  smartex-pp-cli baggage --length-cm 80 --width-cm 60 --height-cm 40 --weight-kg 20 --agent
  ```
- **`policy`** — Retrieve practical boarding, changes and refunds guidance

  _Use for compact auditable Shinkansen planning before booking_

  ```bash
  smartex-pp-cli policy --topic refund --agent
  ```
- **`timetable`** — Find current official basic timetables and validated regular service examples

  _Use for compact auditable Shinkansen planning before booking_

  ```bash
  smartex-pp-cli timetable --from Tokyo --to Shin-Osaka --agent
  ```
- **`handoff`** — Prepare canonical booking links and an exact travel checklist

  _Use for compact auditable Shinkansen planning before booking_

  ```bash
  smartex-pp-cli handoff --from Tokyo --to Shin-Osaka --date 2026-10-28 --agent
  ```
- **`sources`** — Inspect source freshness and planning versus inventory boundaries

  _Use for compact auditable Shinkansen planning before booking_

  ```bash
  smartex-pp-cli sources --agent
  ```

## Auth Setup

Public planning commands need no login. Exact trains and seat inventory are available through the canonical member booking handoff.

Run `smartex-pp-cli doctor` to verify setup.
