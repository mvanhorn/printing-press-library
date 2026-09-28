# tenki.jp CLI

Created by [@zjsng](https://github.com/zjsng) (zjsng).

**Choose Japan sightseeing dates with traceable weather and seasonal evidence.**

Resolve supported places, inspect bounded forecasts, and compare explicit criteria while keeping source time, forecast level and seasonal year visible.

## Install

After the public-library entry is merged and the catalog refresh completes:

```bash
npx -y @mvanhorn/printing-press-library install tenki --cli-only
tenki-pp-cli --version
```

The installer uses a per-user binary directory; add the directory it reports to `$PATH`. A direct Go install requires Go 1.26.6 or newer:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/tenki/cmd/tenki-pp-cli@latest
```

Add `$GOPATH/bin` (default `$HOME/go/bin`) to `$PATH` for that fallback. To work from a checkout before publication, use the local build below.

## Authentication

No credentials are required for public HTML pages. Access is undocumented and subject to tenki.jp terms; there is no verified supported public API.

## Quick Start

```bash
# Resolve bounded candidates and keep ward identity.
tenki-pp-cli places search --query 千代田区 --limit 3

# Inspect source dates, units and freshness.
tenki-pp-cli forecast daily --place https://tenki.jp/forecast/3/16/4410/13101/ --days 3

# Evaluate explicit thresholds with source evidence.
tenki-pp-cli compare --place https://tenki.jp/forecast/3/16/4410/13101/ --days 2 --max-pop 40

```

## Local build

Build this checkout with Go 1.26.6 or newer:

```bash
go build -o bin/tenki-pp-cli ./cmd/tenki-pp-cli
bin/tenki-pp-cli --help
```

Run the examples as `bin/tenki-pp-cli` when the local `bin` directory is not on your shell's path.

## Agent Usage

Product commands return one compact JSON document on stdout; warnings and errors go to stderr. The envelope has `meta` (provider, source transport and request/cache metrics) and `results` (source evidence). Add `--agent` for noninteractive defaults. `--select` uses comma-separated dotted paths and traverses arrays:

```bash
bin/tenki-pp-cli forecast daily --place https://tenki.jp/forecast/3/16/4410/13101/ --days 3 --agent --select results.periods.date,results.periods.weather,results.periods.precip_probability_pct,results.source
bin/tenki-pp-cli seasonal show --kind kouyou --place https://tenki.jp/kouyou/3/13/30139.html --agent --select results.spot,results.status,results.source
```

Read [SKILL.md](SKILL.md) before interpreting suitability, mountain levels, seasonal years or hourly windows. It is the authoritative interpretation guide. Current commands and bounds are available through `--help`; parent groups display help only.

`--dry-run --json` succeeds without required inputs or source requests. Real missing or invalid inputs exit `2`; acquisition failures exit `5`; exhausted rate limits exit `7`. Product unavailability is an explicit successful JSON status with empty arrays, rather than fabricated weather.

## Cache and efficiency

Use `--refresh` to bypass fresh cache, `--data-source local` for cache-only reads, and `--allow-stale` to explicitly allow marked stale cache. Stale evidence cannot earn a positive comparison. `--data-source live` fetches again. Local mode and refresh are incompatible.

`--cache-dir PATH` selects the product cache. Otherwise generated path resolution uses `TENKI_CACHE_DIR`, `--home`, `TENKI_HOME`, XDG variables, then platform defaults. `--timeout` bounds the entire command. Each response includes actual HTTP request, cache-hit and response-byte counts under `meta.metrics`; response bytes are decoded source bodies, not network transfer bytes.

The CLI caches weather/seasonal/model pages for one hour and directories for seven days. These are cache policies; mountain and seasonal provider update cadences are unverified. No hidden polling or nationwide crawl runs.

Source pacing caps each provider client at one HTTP request per second. CLI invocations and MCP command calls use separate clients; concurrent invocations do not coordinate and can exceed that rate in aggregate. Serialize source calls. `--rate-limit` only lowers the per-client ceiling, not a shared or global budget.

## Health Check

```bash
bin/tenki-pp-cli doctor --json
bin/tenki-pp-cli agent-context --pretty
bin/tenki-pp-cli places show --place https://tenki.jp/forecast/3/16/4410/13101/ --refresh
```

`doctor` checks generated configuration/connectivity. A product request checks the current HTML contract. No credentials are required for public pages.

## Cookbook

```bash
# Separate a facility from its forecast municipality.
bin/tenki-pp-cli places show --place https://tenki.jp/leisure/6/29/189/7327/

# Keep six-hour rain intervals separate from temperature/wind instants.
bin/tenki-pp-cli forecast daily --place https://tenki.jp/forecast/3/16/4410/13101/ --days 2 --detail

# Select an actual mountain-model altitude; unsupported levels remain unavailable.
bin/tenki-pp-cli mountain show --place https://tenki.jp/mountain/famous100/5/25/150.html --level 3000 --limit 8

# Search a retained Sakura prefecture directory; the scan has bounded coverage.
bin/tenki-pp-cli seasonal list --kind sakura --directory https://tenki.jp/sakura/3/16/ --query 上野 --year 2026 --limit 5

# Retained Sakura evidence remains labelled with its season year and ended status.
bin/tenki-pp-cli seasonal show --kind sakura --place https://tenki.jp/sakura/3/16/54401.html --year 2026

# Compare preferences using actual source periods; no universal score.
bin/tenki-pp-cli compare --place https://tenki.jp/forecast/3/16/4410/13101/ --days 3 --max-pop 30 --max-temp 30 --sort max-pop

# Limit the comparison to a supported hourly activity window.
bin/tenki-pp-cli compare --place https://tenki.jp/forecast/3/16/4410/13101/ --days 2 --hours 09:00-17:00 --max-rain 0 --max-wind 5 --sort max-rain
```

## Troubleshooting

- A leisure search miss covers only its reported directory and scanned links. Choose a narrower readable directory or use a known canonical URL.
- Daily `partial_horizon`/`out_of_horizon` includes `unsupported_dates` and actual source coverage. `truncated` separately identifies an output limit. Choose source-covered dates; changing `--days` cannot extend the forecast.
- Missing issue time, stale pages, absent values and incompatible forecast levels remain explicit. Inspect `results.source`, warnings and per-criterion evidence.
- `--select` with a wholly missing path returns an error. Inspect an unprojected response or `--help` before choosing fields.
- Source `403`/changed HTML errors are acquisition failures. Do not substitute zeros or route around denials.

## Access and limitations

The CLI reads unauthenticated public HTML, not a supported public API. [tenki.jp terms Article 8(6)](https://tenki.jp/docs/rule/) restrict obtaining or using information through tools other than browsers/RSS readers. Successful HTTP access and public distribution of this CLI do not establish provider authorization. HTML markup can change, and unavailable or retained seasonal pages cannot supply unpublished future-year forecasts.

The optional MCP HTTP server uses a ten-second request-header timeout. The dated scanner finding and publication fix are described in [VERIFICATION.md](VERIFICATION.md). The CLI does not start that server; the live acceptance matrix validates CLI workflows rather than HTTP hosting.

## Sources & Inspiration

Source semantics: [tenki.jp](https://tenki.jp/) and its official help service. Ecosystem references: [algon-320/tenki](https://github.com/algon-320/tenki) and [rrencanno/tenki_scraper](https://github.com/rrencanno/tenki_scraper).

Generated foundation: [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press).

## Unique Features

### Evidence-based planning
- **`compare`** — Compare selected Japan destinations and dates against explicit weather thresholds.

  _Compare selected Japan destinations and dates against explicit weather thresholds._

  ```bash
  tenki-pp-cli compare --place https://tenki.jp/forecast/3/16/4410/13101/ --days 2 --max-pop 40 --agent
  ```
- **`compare`** — Evaluate a specific JST outing window with complete hourly evidence.

  _Evaluate a specific JST outing window with complete hourly evidence._

  ```bash
  tenki-pp-cli compare --place https://tenki.jp/forecast/3/16/4410/13101/ --hours 09:00-17:00 --max-pop 40 --agent
  ```
- **`compare`** — Pair seasonal spot status with its actual forecast municipality.

  _Pair seasonal spot status with its actual forecast municipality._

  ```bash
  tenki-pp-cli compare --place https://tenki.jp/kouyou/4/19/30314.html --season kouyou --max-pop 40 --agent
  ```
- **`compare`** — Explain missing, stale, unsupported or mismatched evidence before recommending.

  _Explain missing, stale, unsupported or mismatched evidence before recommending._

  ```bash
  tenki-pp-cli compare --place https://tenki.jp/forecast/3/16/4410/13101/ --days 14 --max-wind 8 --agent
  ```

## Verification

See [VERIFICATION.md](VERIFICATION.md) for live coverage, separate fixture tests, efficiency measurements, access limits and reproducible checks.
