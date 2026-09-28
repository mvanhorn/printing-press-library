# Ikyu accommodation CLI

**Search Ikyu hotels and ryokan with dated room prices and cancellation details.**

Discover Ikyu hotels and ryokan, inspect exact room-plan conditions, and compare offers with honest price and bath distinctions. Compact results preserve Japanese names, source IDs and freshness.

## Install

```sh
npx -y @mvanhorn/printing-press-library install ikyu --cli-only
ikyu-pp-cli --version
```

Catalog installation becomes available after the library PR merges and catalog automation completes. To use a pending PR or a local checkout, build from source.

## Build locally

```sh
go build -o ikyu-pp-cli ./cmd/ikyu-pp-cli
export PATH="$PWD:$PATH"  # this shell only
./ikyu-pp-cli --version
```

Use the Go version declared in `go.mod`. Anonymous discovery requires no account setup. The build changes no shared agent configuration.

## Authentication

Verified public accommodation reads are anonymous and require no API key. Personalized member inventory and coupon eligibility are outside this CLI; final booking stays on Ikyu.

## Quick Start

```bash
# Check the local command surface.
ikyu-pp-cli doctor --dry-run

# Build a bounded dated shortlist.
ikyu-pp-cli stay search --destination tokyo --check-in 2026-11-17 --check-out 2026-11-18 --adults 2 --limit 3 --agent

# Inspect room and plan summaries.
ikyu-pp-cli stay rooms 00002889 --check-in 2026-11-17 --check-out 2026-11-18 --adults 2 --limit 3 --agent

# Check exact price and cancellation terms.
ikyu-pp-cli stay offer 00002889 10193727 11055986 --check-in 2026-11-17 --check-out 2026-11-18 --adults 2 --agent

```

## Prices, occupancy and baths

Adult and A–F child counts are **per room**. Multiple rooms use the same room type, plan and occupancy. Source amounts are JPY totals across requested rooms and nights; extra taxes may remain payable. Different occupancy per room requires Ikyu's booking screen.

Headline amount, conditional points-adjusted payable, earned/applied points, coupons and final checkout payable are separate. Earned points and immediately applied points describe alternative scenarios; never subtract both. The CLI preserves source amounts instead of reconstructing discounts from percentages. Unverified checkout payable and eligibility remain explicit.

An outdoor room bath does not necessarily use hot-spring water. A shared onsen does not prove the selected room has a private bath. Missing facts stay unknown. See [source semantics](docs/source-contract.md) for the source evidence and child/cancellation categories.

Comparisons report observed monetary differences separately from compatibility blockers; differing amounts alone do not make equal terms incompatible. Anonymous price eligibility stays unverified, so live comparisons do not assert equivalent-price savings. Different rooms, properties, dates, meals or cancellation terms are alternatives; a lower displayed amount alone is not equivalent-offer savings.

## Agent Usage

`stay` commands emit compact JSON by default; diagnostics go to stderr. Use `--agent` with `--select` to project fields, inspect pagination before continuing, and fetch full offer details only for chosen rooms. The local [agent skill](SKILL.md) describes this workflow.

Availability is a timestamped observation. Cache hits expose their age; `--refresh` fetches current data. Availability caches last five minutes and static details 24 hours; `--no-cache` disables reads/writes. `--allow-stale` permits labelled old observations only after an upstream failure. Use `IKYU_CACHE_DIR` or `--cache-dir` to keep the cache inside this checkout. Lists default to 10 and cap at 50; comparison/date scans and network retries have fixed bounds. Search scans one source page and filters its bounded previews. Unfiltered results retain unpriced candidates with explicit gaps; inspect exact offers before treating them as available. Budget and preference filters require observed matches. Continue at `pagination.offset + pagination.scanned`, rather than the number of returned matches. Room and plan offsets are independent; `--limit × --plan-limit` is capped at 100 summaries. Budget and meal filters remove nonmatching plans from each room while preserving source pagination counts.

## Cookbook

Compare selected offers, preserving their different meal/cancellation terms:

```sh
./ikyu-pp-cli stay compare --offers 00002889:10193727:11055986,00002889:10193727:11055981 --check-in 2026-11-17 --check-out 2026-11-18 --adults 2 --agent
```

Check explicit alternative dates for one exact room and plan:

```sh
./ikyu-pp-cli stay dates 00002889 10193727 11055986 --check-ins 2026-11-17,2026-11-18 --nights 1 --adults 2 --agent
```

## Health Check

```sh
./ikyu-pp-cli doctor --json
./ikyu-pp-cli stay offer --dry-run --json
```

The dry run performs no source request. Doctor checks the generated framework; live `stay` verification is documented separately.

## Access and limitations

Verified public reads require no API key, login or paid API subscription. Ikyu membership is optional and free, but anonymous discovery omits member-only inventory and cannot verify account coupons. This is an unofficial integration with undocumented JSON/SSR surfaces that can change or deny requests. Exact lookups use the upstream default points variant; room summaries retain other variant IDs, which require checking on Ikyu.

The scope is domestic accommodation discovery and booking handoff. Booking, payment, cancellation, restaurant/spa products, overseas stays, account actions and broad crawling are outside it. Confirm current availability and final payable on Ikyu through the returned canonical link. The generated raw `properties` HTML helper received HTTP 403 in the matrix; use the verified `stay` command surface for accommodation work.

## Troubleshooting

- Invalid date/party: use actual future Japan dates and per-room occupancy; source defaults are not accepted as your requested quote.
- No matching rooms: inspect filter coverage and pagination, then adjust the stay or preferences.
- Missing price or condition: treat it as unknown and inspect the selected offer/source link.
- Access or schema error: retain the error and retry later with `--refresh`; an error is not “no availability.”
- Stale result: refresh the exact offer before handoff.

## Verification

Run deterministic checks with `go test -count=1 ./...` and `go build ./...`.

```sh
python3 scripts/verify-live.py --output live-acceptance.json
python3 scripts/measure.py --output efficiency-metrics.json
```

The optional verification scripts need Python 3 and curl; measurements use `/usr/bin/time` on macOS/Linux. They default to future Japan dates and discover current source offers. Set explicit dates/selectors to reproduce a prior observation. The compiled CLI needs neither Python nor a running browser. See [validation and measurements](docs/validation.md) for results and limitations.

## Unique Features

| Command | Capability |
|---|---|
| `stay compare` | Compare exact offers with explicit compatible groups, differences and unknowns. |
| `stay dates` | Inspect bounded date alternatives for one selected room and plan. |
| `stay rooms` | Find rooms with source-backed space and bath attributes; keep missing evidence visible. |
| `stay offer` | Separate source prices, points scenarios, coupons and eligibility for an exact offer. |
| `stay search` | Return compact dated property candidates with pagination, freshness and detail gaps. |

Created by [@zjsng](https://github.com/zjsng) (zjsng).
