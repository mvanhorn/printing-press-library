# Rakuten Travel CLI

**Inspect real Japan room offers with explicit dates, party and price units.**

Search Rakuten Travel public pages, inspect property details and compare bounded date alternatives. Keep room and plan identities, source price labels and booking links intact.

## Authentication

The public website backend requires no API credentials or browser. The separately documented official APIs require an application ID and access key and are not used by these commands.

## Quick Start

```bash
# Check local CLI wiring.
rakuten-travel-pp-cli doctor --dry-run

# Resolve source hotel IDs.
rakuten-travel-pp-cli hotels search --query 品川 --limit 3

# Inspect actual dated room offers.
rakuten-travel-pp-cli offers search --hotel 51870 --checkin 2026-11-08 --checkout 2026-11-10 --rooms 1 --adults-per-room 2 --limit 3

```

## Agent Usage

Default output is compact JSON on stdout; diagnostics use stderr. Search returns bounded summaries; `show` inspects one property or exact room/plan. Read [SKILL.md](SKILL.md) for the agent workflow.

```sh
rakuten-travel-pp-cli hotels search --query Shinagawa --limit 3 --agent --select results.hotel_id,results.name
rakuten-travel-pp-cli offers search --hotel 51870 --checkin 2026-11-08 --checkout 2026-11-10 --rooms 1 --adults-per-room 2 --infant-none 1 --limit 3
```

Adults and all child counts are **per room**. Multiple rooms use the same party in each room. The six child categories are upper/lower elementary, infant with meal+bed, meal only, bed only, and neither; unequal room allocations are unsupported. English keywords are literal matches, with different coverage from Japanese queries.

Follow returned page/offset metadata to continue; `--limit` alone does not mean all inventory was examined. See [data contract](docs/data-contract.md) for price units, freshness and failure meanings.

## Unique Features

- **Date alternatives** (`compare`): an explicit hotel/date matrix with equal stay length and party, capped at nine cells.
- **Whole-stay quote evidence** (`offers search`): source-labelled per-room totals; no extrapolation from a first-night price.
- **Child-aware occupancy** (`offers search`): every source child category remains distinct.
- **Policy and fee inspection** (`offers show`): property notes and unknown plan policy stay separately labelled.
- **Exact offer handoff** (`offers show`): hotel, plan and room IDs stay attached to the dated source page.


## Cookbook

Browse areas, then search one source area:

```sh
rakuten-travel-pp-cli areas list --limit 10
rakuten-travel-pp-cli areas list --parent tokyo --limit 10
rakuten-travel-pp-cli hotels search --area tokyo/E --limit 5
```

Inspect an offer using IDs from `offers search`:

```sh
rakuten-travel-pp-cli offers show --hotel 51870 --plan 3951989 --room s-double- --checkin 2026-11-08 --checkout 2026-11-10 --rooms 1 --adults-per-room 2
```

Compare two dates for a selected hotel:

```sh
rakuten-travel-pp-cli compare --hotels 51870 --checkins 2026-11-08,2026-11-09 --nights 2 --rooms 1 --adults-per-room 2
```

The minimum reported for a cell is limited to the inspected source page. Open the returned Rakuten link to confirm conditions and complete booking; this CLI never submits a reservation.

## Health Check

```sh
rakuten-travel-pp-cli doctor --json
rakuten-travel-pp-cli offers search --help
go test -count=1 ./...
```

`doctor` checks generated runtime setup; a successful live offer search is the inventory check. Tests and measured live evidence are documented in [verification](docs/verification.md).

## Troubleshooting

- **No availability:** try another explicit date or hotel; unavailable catalog entries are excluded from bookable results.
- **Only part of a page returned:** continue with the returned offset before moving to the next source page.
- **Throttling or page-contract error:** retry later for throttling; inspect/update the parser for changed HTML. Neither means sold out.
- **Unexpected English results:** use the Japanese property/destination name or a source area ID.
- **Missing fees:** inspect property notes and the booking page. Consumption-tax-inclusive prices can exclude accommodation tax and optional services.

Requests are serial and paced, with bounded retries, response size and per-command request budgets. Metadata is cached; inventory is fresh by default. `--refresh` bypasses stored responses; `--no-cache` disables persistent cache reads and writes. Inventory caching is explicit and at most 60 seconds. Use `--help` for current controls.

## Access and limitations

This delivery uses the public Japanese Rakuten Travel website. HTML structure may change, and returned inventory is only what the queried source exposes. No radius search or unverified coordinate conversion; coordinates remain unknown where datum/units are unavailable. Member/coupon promotional amounts are not treated as baseline prices.

The official API was investigated first. It currently requires a Rakuten account, application registration, application ID and access key; it is not an active backend in this CLI. Its documented availability charge fields cover the first night, and its ranking data is stale. See [access research](docs/access-research.md), the [official guide](https://webservice.rakuten.co.jp/guide) and [availability documentation](https://webservice.rakuten.co.jp/documentation/vacant-hotel-search).

Data source: [Rakuten Travel](https://travel.rakuten.co.jp/). Unaffiliated with Rakuten. Built with [Printing Press](https://github.com/mvanhorn/cli-printing-press); agent documentation follows writing-for-agents. Workflow references: [rakuten-mcp](https://github.com/mrslbt/rakuten-mcp), [rakuten_travel_mcp](https://github.com/hirochachacha/rakuten_travel_mcp), and [rakuten-travel-skill](https://github.com/Tomatio13/Temple-of-Skills/tree/main/rakuten-travel-skill).

## Recipes

### Compact candidates

```bash
rakuten-travel-pp-cli hotels search --query 品川 --limit 3 --agent --select results.hotel_id,results.name
```

Resolve IDs with a bounded payload.

### Property notes

```bash
rakuten-travel-pp-cli hotels show --hotel 51870
```

Inspect amenities, access and property fee notes.

### Date alternatives

```bash
rakuten-travel-pp-cli compare --hotels 51870 --checkins 2026-11-08,2026-11-09 --nights 2 --rooms 1 --adults-per-room 2
```

Compare explicit equal-length stays with the same party.
