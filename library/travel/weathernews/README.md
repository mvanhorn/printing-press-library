# Weathernews CLI

**Japan weather and seasonal travel evidence with explicit dates, freshness and coverage.**

Resolve Japanese places, inspect bounded forecasts and seasonal evidence, and compare caller criteria. Public first-party access without a paid account.

Created by [@zjsng](https://github.com/zjsng) (zjsng).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Authentication

Selected public products need no credentials. Member-only products and commercial WxTech are outside scope.

## Quick Start

```bash
# Check local readiness
weathernews-pp-cli doctor --dry-run

# Resolve place identity
weathernews-pp-cli places resolve --query 京都

# Read bounded Kyoto forecast
weathernews-pp-cli weather forecast --lat 35.01167 --lon 135.76806

```

## Cookbook

```sh
# Caller criteria, compared independently in input order
./weathernews-pp-cli weather compare --point Kyoto=35.01167,135.76806 --point Tokyo=35.681,139.767 --date 2026-10-01 --max-pop 40 --max-temp 30
# Caller tolerance from a published peak prediction, never a destination score
./weathernews-pp-cli season compare --product koyo --ids 26101,26111 --date 2026-11-26 --max-days-from-peak 3
# Ended reports remain useful historical evidence
./weathernews-pp-cli season show --product sakura --id 384
# Explicit inventory refresh, followed by a bounded local page
./weathernews-pp-cli season search --product koyo --area kyoto --refresh --limit 5 --offset 0
# Source-horizon miss is a product state
./weathernews-pp-cli weather forecast --lat 35.01167 --lon 135.76806 --date 2030-01-01
```

Replace example dates with your trip dates. `meets` means the supplied numerical criteria pass; it does not promise suitable weather or colorful leaves. `unknown` is never treated as a pass. Seasonal comparisons use absolute day distance from the source's peak prediction. A historical normal never substitutes for a missing prediction.

## Agent Usage

Focused commands emit one compact JSON document by default. Errors and optional `--metrics` diagnostics go to stderr. `--agent` adds a `meta` / `results` envelope; source/cache provenance remains explicit.

```sh
./weathernews-pp-cli season search --product koyo --area kyoto --agent --select items.id,items.name_ja,season
./weathernews-pp-cli weather forecast --lat 35.01167 --lon 135.76806 --agent --select daily,location,horizon,source
```

Search defaults to 10 rows, capped at 50. Follow `next_offset` against the same cached snapshot. Prefecture/region searches retrieve one inventory page; nationwide keyword searches retrieve one larger inventory and filter all literal terms against names, kana, city, address and prefecture. Use Japanese terms for the most reliable source match. Summaries never fetch spot details. `season show` fetches exactly one spot. Comparisons accept 2–5 candidates, run serially, and preserve input order. Duplicate candidate names/IDs are rejected.

## Evidence and Coverage

Every public fetch includes its source URL, retrieval time, cache hit and age. Forecast issue times are **null** because this public response does not supply them; the first valid time is not an issue time. Observation `valid_at` comes from the source's `observation.ISSUE`, separately from predictions. Times use JST (`+09:00` / `Asia/Tokyo`). Seasonal dates have day precision; their year comes from the source page title.

Temperature is Celsius, precipitation millimeters, wind meters/second and daily precipitation probability percent. Hourly precipitation uses the source interval stride; the hour label is preserved without inventing start/end alignment. Observation precipitation interval is unknown. Weather/direction codes are preserved without invented descriptions.

Locations retain requested coordinates and source location/grid identity. Elevation and public grid resolution remain null when absent. Mountain-place matches are distinct from mountain-top measurements; do not infer elevation or apply lapse-rate corrections.

Reports, forecasts and historical normals carry separate `kind` fields. Report age is separate from fetch age. Season year, `updates_ended`, closure message and prediction availability are explicit. Missing dates/values remain null. Weather dates outside the returned horizon emit `out_of_horizon`; ended seasons or missing peak predictions produce unknown comparison evidence.

As verified on **2026-10-01 JST**, 2026 sakura updates have ended; current foliage reports and predictions are available. Active next-season sakura predictions cannot be live-verified until the provider publishes them. Member radar, gated/extended products and commercial WxTech are excluded; the CLI never silently requires a subscription. Public website contracts are undocumented and may change; parsing failures use an error, not guessed data.

## Health Check

`doctor --dry-run` checks local readiness without network. `doctor --json` is the framework health report. A live focused check is `places resolve --query 京都 --metrics`. Use `--home /absolute/path` to isolate all CLI state, or `--cache-dir /absolute/path` for the focused HTTP cache. `--no-learn` disables optional framework learning/journaling.

## Runtime Bounds

Focused HTTP responses are capped at 4 MiB. Cache: 128 entries / 64 MiB, atomic writes, no expired-data fallback. Place cache TTL 24 hours, forecasts 10 minutes, seasonal inventory/detail 1 hour. `--refresh` explicitly bypasses and replaces cache; `--no-cache` bypasses reads and writes. `--data-source local` requires fresh cached evidence and makes no network request.

At most two attempts per GET, one request at a time, default pacing 2 requests/second (explicit `--rate-limit` capped at 5; 0 disables pacing), per-request timeout capped at 20 seconds, overall command deadline no longer than 90 seconds and honoring the global `--timeout`. No background inventory crawl. No auth-session capture. Hosts are restricted to weathernews.jp and site.weathernews.jp for focused commands.

## Troubleshooting

- Exit 2: invalid arguments, missing criteria, bad projection or unsupported coordinates. Check command `--help`.
- Exit 3: missing spot or fresh local cache. Resolve/search first, or refresh online.
- Exit 4: provider access restriction; selected scope is public only.
- Exit 5: request/schema failure or partial comparison. Partial JSON retains each successful candidate and each error.
- Exit 7: provider rate limit. Retry later or use fresh cache.
- Exit 10: local configuration/path problem. Use a writable absolute `--home`.

An empty search is an ordinary result with `total:0`. It is distinct from access failure. `--select` requests missing fields fail predictably. Raw generated replay/framework tools remain available for diagnostics; focused travel commands carry the normalized coverage contract above.

## Local Build

From this source checkout, use `go build -o weathernews-pp-cli ./cmd/weathernews-pp-cli` (Go 1.26.6 or newer). Remote installer commands become available after the library PR is merged and its catalog workflows complete.

## Build and Evidence

Run `go test -count=1 ./...` and `go vet ./...`. The tests cover consequential parsing, dates, missing values, criteria and cache states; they are separate from live proof. See [evidence/final-report.md](evidence/final-report.md) for source agreement, runtime measurements, independent review and Press acceptance. Fresh publication acceptance, test results and source-parity evidence are retained alongside the final report.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Travel evidence
- **`season search`** — Bounded literal Japanese seasonal search with lazy details

  _Bounded literal Japanese seasonal search with lazy details_

  ```bash
  weathernews-pp-cli season search --product koyo --area kyoto --query 嵐山 --agent
  ```
- **`season show`** — Separate observation, forecast, norm, season year and ended state

  _Separate observation, forecast, norm, season year and ended state_

  ```bash
  weathernews-pp-cli season show --product koyo --id 26102 --agent
  ```
- **`weather compare`** — Compare caller daily thresholds across bounded exact coordinates

  _Compare caller daily thresholds across bounded exact coordinates_

  ```bash
  weathernews-pp-cli weather compare --points 'Kyoto=35.01167,135.76806|Tokyo=35.681,139.767' --date 2026-10-01 --max-pop 40 --agent
  ```
- **`season compare`** — Compare caller date proximity to published peak predictions

  _Compare caller date proximity to published peak predictions_

  ```bash
  weathernews-pp-cli season compare --product koyo --ids 26101,26111 --date 2026-11-26 --max-days-from-peak 3 --agent
  ```
- **`places resolve`** — Preserve source names, URLs and coordinates before forecast selection

  _Preserve source names, URLs and coordinates before forecast selection_

  ```bash
  weathernews-pp-cli places resolve --query 高尾山 --agent
  ```

## Recipes

### Compact seasonal search

```bash
weathernews-pp-cli season search --product koyo --area kyoto --agent --select items.id,items.name_ja,season
```

Find source IDs before lazy details
