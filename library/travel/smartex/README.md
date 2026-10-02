# smartEX CLI

Created by [@zjsng](https://github.com/zjsng) (zjsng).

**Compare Shinkansen fares and constraints before booking**

Read-only dated adult fares, discount deadlines and baggage checks from smartEX and linked JR sources. Plan with Japanese station names and precise JST dates, then book through the official site.

## Install

Install the CLI and skill:

```sh
npx -y @mvanhorn/printing-press-library install smartex
```

Or build from a checkout with Go 1.26.6 or newer:

```sh
go build -o smartex-pp-cli ./cmd/smartex-pp-cli
./smartex-pp-cli version
```

## Authentication

Public planning commands need no login. Exact trains and seat inventory are available through the canonical member booking handoff.

## Quick Start

```bash
# Check installation without a network call
smartex-pp-cli doctor --dry-run

# Resolve a station before planning
smartex-pp-cli stations --query Osaka --agent

# Compare a public dated adult quote
smartex-pp-cli fare --from Tokyo --to Shin-Osaka --date 2026-10-02 --agent

```

## Planning Commands

| Command | Result |
|---|---|
| `stations` | Station identity, Japanese name and corridor order; bounded search |
| `route` | Tokaido/Sanyo/Kyushu coverage and categories; optional station sequence |
| `fare` | Fresh dated ordinary reserved, unreserved and Green adult fares |
| `products` | Basic and current Hayatoku rules, deadlines and unresolved eligibility |
| `window` | JST opening/cutoff, overnight party limits and confirmation uncertainty |
| `baggage` | Per-piece cm/kg limits and oversized area seat requirement |
| `policy` | Boarding, changes, refund, product and window guidance |
| `timetable` | Live official PDF links plus explicitly limited basic service examples |
| `handoff` | Canonical booking URL and exact travel checklist |
| `sources` | Source IDs, as-of dates and optional live HTTP page checks |

`reference <source> --agent` extracts fresh official page links, including fare PDFs. `products --detail` fetches current product-document links; summary rules remain explicitly dated.

## Boundaries and Freshness

Public fare navigator: current JST month and next two months, one adult one way. Adult party subtotals are arithmetic, with no seat guarantee. Child fares and totals involving children are null. EX Reservation prices require separate paid membership. Hayatoku price, route, exclusion-day and allocated-seat eligibility remain unverified until checked in the official product documents/booking flow. Conventional-line connections are extra for smartEX; station tickets may have different city-zone coverage.

Timetable rows are a curated subset of JR's2026-03-14 **basic** timetable, including the six named train types; they are not complete dated services. Default live lookup checks that the publication links still match and suppresses old examples if the links change. Intermediate departure-only rows retain null arrival and duration. Requested-date operation and inventory are always null. `--offline` serves the dated snapshot explicitly. An empty example result does not mean no trains.

Policy snapshot:2026-10-02. Japanese advance-reservation guidance says confirmation starts08:00JST one month before; the English page says14:00. `window` exposes both sources and null actual confirmation. A missing corresponding day in the previous month uses the first day of the travel month. Oversized one-year and overnight eligibility differs between current Japanese and English guidance: actual eligibility/opening stays unknown, with normal one-month sales retained as a known fallback. Oversized parties are limited to five ordinary or four Green passengers per operation; no cross-car grouping. Normal luggage over160cm through250cm needs a reserved oversized area; current JR summary controls the160cm boundary. Special equipment needs operator confirmation.

No real booking, payment, account or reservation mutation. Login maintenance was observed during discovery; exact inventory still requires the member booking flow. Public fare calculation works without login. Round-trip smartEX discount ended2026-03-31.

## Agent Usage

```sh
./smartex-pp-cli fare --from 東京 --to 新大阪 --agent --select quotes
./smartex-pp-cli products --date 2026-10-28 --agent --select products
./smartex-pp-cli route --from Hakata --to Kumamoto --json --compact
```

`--agent` gives compact JSON, noninteractive defaults and source metadata. `--select` applies to payload fields. Normal `--json` remains available; `--dry-run` performs no network work. Live commands honor `--timeout`; responses cap at512KiB and public requests are paced at at most2 per second. `--data-source local` prevents live work; for timetable it selects the offline snapshot. Invalid assumptions exit2, source failures5, throttling7. Partial comparisons retain explicit failures and successful classes, with no phantom zero fares.

## Health Check

```sh
./smartex-pp-cli doctor --json
./smartex-pp-cli sources --source timetable --check --limit 1 --agent
```

An HTTP200 source check proves page reachability, not that every dated policy fact was re-extracted.

## Troubleshooting

Use `stations --query` for supported names; use `shin-osaka` for Shinkansen access. A source drift, timeout, non200 or throttling error is explicit; retry a bounded query or follow `handoff`. For timetable publication changes, use fresh returned PDF links and booking confirmation. Unsupported child/product prices remain null.

## Cookbook

```sh
./smartex-pp-cli window --date 2026-10-31 --now 2026-10-01T09:00:00+09:00 --departure 09:00 --agent
./smartex-pp-cli baggage --length-cm 80 --width-cm 60 --height-cm 40 --weight-kg 20 --class reserved --agent
./smartex-pp-cli timetable --from Tokyo --to Shin-Osaka --train nozomi --limit 3 --agent
./smartex-pp-cli policy --topic refund --agent
```

## Verification

Consequential calendar, baggage, encoding, parser drift, unknown-price, timeout and throttle tests live in `internal/smartex`. Live proof covers all classes and corridors, reverse/through routes and official source checks. Run `go test ./...` and `go vet ./...`; no account is needed. Printing Press checks and the independent review are recorded in the build evidence.

Generated foundation: [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press). Source authorities: [smartEX](https://smart-ex.jp/en/), [JR basic timetables](https://global.jr-central.co.jp/en/info/timetable/), [JR luggage](https://global.jr-central.co.jp/en/info/oversized-baggage/), [JR-supported fare navigator](https://unchin-navi.jp/). Author: zjsng. Apache-2.0.

## Unique Features

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
