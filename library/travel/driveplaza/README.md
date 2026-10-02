# Drive Plaza CLI

**Plan expressway journeys with source toll estimates, directional rest stops and official advisories.**

Resolve interchanges and compare source route alternatives with explicit vehicle and JST schedule assumptions. Find directional SA/PA facilities and keep dated notices distinct from current road status.

## Install

```sh
go build -o driveplaza-pp-cli ./cmd/driveplaza-pp-cli
go build -o driveplaza-pp-mcp ./cmd/driveplaza-pp-mcp
```

Go 1.26.6 or newer. Source is locally built and promoted; this build has not been publicly published.

## Authentication

Public read-only English and Japanese pages; no account, key or browser runtime is required.

## Quick Start

```bash
# Check the local command setup.
driveplaza-pp-cli doctor --dry-run

# Resolve an exact English interchange name.
driveplaza-pp-cli interchanges --query nerima --language en --agent

# Compare source alternatives with explicit JST assumptions.
driveplaza-pp-cli route --from nerima --to sendai-minami --at 2026-10-10T08:00 --agent

# Find directional stops on Tohoku Expressway.
driveplaza-pp-cli sapa list --road 1040 --direction up --limit 5 --agent

```

## Agent Usage

Domain commands emit compact JSON with `meta` and `results`. `meta` contains source URLs, UTC retrieval time, `Asia/Tokyo`, request count and partial-enrichment warnings. Missing facts are `null`; empty collections are `[]`. `--select` projects the results payload while retaining provenance; a leading `results.` is optional.

```sh
./driveplaza-pp-cli sapa list --road 1040 --query hasuda --agent --select items.id,items.name_ja,items.direction,items.url
./driveplaza-pp-cli route --from nerima --to sendai-minami --at 2026-10-10T08:00 --agent --select alternatives.id,alternatives.standard_jpy,alternatives.etc_jpy,alternatives.distance_km
```

`--limit` defaults to 10, maximum 30. `--offset` paginates the current response locally; a later request may return changed source data. Road-specific SA/PA scans examine at most 500 records by default; `--max-scan-records` is independent of output size and capped at 2000. `scanned_items`, `scan_limited`, `note` and `next_offset` describe coverage. Each HTTP response is capped at 2 MiB and each command at 40 requests. Requests are paced at at most 2/second; `--timeout` bounds the whole domain command.

All planning commands declare MCP read-only hints. `driveplaza-pp-mcp` serves stdio and mirrors the Cobra commands. Framework learning commands write only local state; use `--no-learn` or `DRIVEPLAZA_NO_LEARN=true` for deterministic invocations. `recall`, `teach`, and `learnings` are optional local memory, not a cached source of current quotes. `reference` contains low-level page-inspection commands; use domain commands for planning facts.

## Health Check

```sh
./driveplaza-pp-cli doctor --json
./driveplaza-pp-cli agent-context --pretty
./driveplaza-pp-cli route --dry-run --agent
```

Dry runs make no provider request. `--data-source local` is rejected by live domain commands because there is no offline mirror. Online responses always carry their own retrieval time.

## Cookbook

```sh
# Source filters and directional detail
./driveplaza-pp-cli sapa facilities --agent
./driveplaza-pp-cli sapa list --road 1040 --direction up --facility 9010,5220 --limit 5 --agent
./driveplaza-pp-cli sapa detail --id 1040/1040021/1 --agent

# Preserve explicit arrival and ETC2.0 assumptions
./driveplaza-pp-cli route --from nerima --to sendai-minami --at 2026-10-10T08:00 --time-kind arrival --priority toll --payment etc2 --detail --agent

# Dated advisory notices and official map/restriction pages
./driveplaza-pp-cli notices --query 東北 --since 2026-09-01 --limit 5 --agent
./driveplaza-pp-cli handoff --agent
./driveplaza-pp-cli conditions --agent
```

The source form supports up to five `--via` English IC names, `--exclude-urban` and `--exclude-ordinary`. `--payment` selects a quoted column; all standard/ETC/ETC2.0 prices remain available. Prices use JPY, distances km, and durations minutes.

## Troubleshooting

- IC or route not found: resolve the exact English spelling and start/arrival role; inspect the attached official source URL. The bundle's English code-entry endpoints returned 404 and are not offered as working route entry.
- English SA/PA name lookup is case-sensitive upstream: use `sapa list --road 1040 --query hasuda`; this CLI filters bilingual names locally.
- Invalid time: use a real `YYYY-MM-DDTHH:MM` JST value with minutes 00,10,20,30,40,50. No silent rounding.
- HTTP 429: retry after the reported delay. The CLI returns a typed rate-limit error rather than an empty result.
- Unknown Japanese name: inspect `meta.warnings`; enrichment failure stays explicit and other facts remain available.

Exit codes: 0 success, 2 invalid input, 3 source not found, 4 source access failure, 5 transport/parser failure, 7 rate limit, 10 local configuration failure. No purchases, account mutations or automatic browser navigation.

### API-specific
- **English SA/PA name search returns no results** — Use sapa list --road 1040 and filter locally with --query HASUDA.
- **Invalid planned time** — Use --at YYYY-MM-DDTHH:MM in JST with minutes00,10,20,30,40 or50.
- **ETC quote differs from a later drive** — Re-run route with your vehicle and planned time; quoted discounts depend on actual eligibility and travel.

## Data limits

Route prices are conditional source estimates, not guaranteed final tolls. Actual vehicle class, card/device eligibility, timing, route and driving can affect ETC/ETC2.0. “Considering traffic” is the provider's estimate for the requested schedule and is not a live-now claim. Per-route movement restrictions stay attached; `--detail` adds directional stop and predicted-traffic links.

SA/PA `up` and `down` identify distinct source records. Green icons are available, gray/`none` icons unavailable, missing icons unknown. A category such as rest/accommodation does not prove a hotel is present; use detail. `--open-24` selects a source 24-hour facility/service, not every shop. Hours are weekday source text; holidays and open-now status remain unknown. The provider explicitly dates non-NEXCO-East facility data to **2006-03-31**.

Notices include plans, postponements and releases. `active_restriction` is always null; the feed does not establish a comprehensive current closure inventory. `handoff` prints observed canonical URLs without fetching or navigating them. Review official construction, ETC-lane and live-map pages before travel. The English HASUDA page's breadcrumb/title refers to Higashi-Kanto while its detail header and road code identify Tohoku; this CLI uses the detail header and code.

Verification artifacts: [live acceptance](evidence/live/acceptance.json), [research](evidence/research.md), and `FINAL-REPORT.md` after gates close. Deterministic tests use sanitized captured public responses; live proof is independent of those fixtures.

Built with [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press). Source data: [Drive Plaza](https://en.driveplaza.com/dp/SearchTopEN), [SA/PA search](https://en.driveplaza.com/dp/SAPAServiceEN), [planned restrictions](https://www.driveplaza.com/traffic/roadinfo/schedule/).

## Unique Features


### Expressway planning
- **`route`** — Keep vehicle, JST schedule and conditional toll assumptions beside route alternatives.

  _Keep vehicle, JST schedule and conditional toll assumptions beside route alternatives._

  ```bash
  driveplaza-pp-cli route --from nerima --to sendai-minami --at 2026-10-10T08:00 --agent
  ```
- **`interchanges`** — Resolve English names to stable IDs and source Japanese names.

  _Resolve English names to stable IDs and source Japanese names._

  ```bash
  driveplaza-pp-cli interchanges --query nerima --language en --agent
  ```

### Rest-stop evidence
- **`sapa list`** — Compare directional stops using source facility availability.

  _Compare directional stops using source facility availability._

  ```bash
  driveplaza-pp-cli sapa list --road 1040 --direction up --limit 5 --agent
  ```

### Official handoffs
- **`notices`** — Read dated traffic notices without mistaking them for active restrictions.

  _Read dated traffic notices without mistaking them for active restrictions._

  ```bash
  driveplaza-pp-cli notices --limit 5 --agent
  ```
- **`handoff`** — Get official construction, ETC-lane and live-traffic URLs.

  _Get official construction, ETC-lane and live-traffic URLs._

  ```bash
  driveplaza-pp-cli handoff --agent
  ```

## Recipes

### Narrow directional stops

```bash
driveplaza-pp-cli sapa list --road 1040 --limit 5 --agent --select items.id,items.name_ja,items.direction,items.url
```

Keep stable identities and source links.

### Inspect facilities and hours

```bash
driveplaza-pp-cli sapa detail --id 1040/1040021/1 --agent
```

Read directional HASUDA-SA UP sections and weekday hours.

### Read traffic advisory notices

```bash
driveplaza-pp-cli notices --limit 5 --agent
```

Dated notices do not establish active road status.

### Get official restriction pages

```bash
driveplaza-pp-cli handoff --agent
```

Check the construction, ETC-lane and live-map pages.
