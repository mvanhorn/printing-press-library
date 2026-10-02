# Yamato luggage planning CLI

Created by [@zjsng](https://github.com/zjsng) (zjsng).

**Plan Japan luggage forwarding with source rates, calendar deadlines and acceptance evidence.**

Measure luggage, compare distinct TA-Q-BIN products, and query current public rates and estimated delivery dates. Airport terminal and limited same-day evidence preserve cutoff and acceptance uncertainty.

## Install

From a checked-out source directory, use the Go version specified by `go.mod`:

```sh
go build -o yamato-pp-cli ./cmd/yamato-pp-cli
./yamato-pp-cli --version
go install ./cmd/yamato-pp-cli
```

Optional MCP server: `go build -o yamato-pp-mcp ./cmd/yamato-pp-mcp`, then configure your MCP host to run that absolute binary path. Its tools mirror the current Cobra commands.

## Authentication

No login or API key is required for supported public planning commands.

## Quick Start

```bash
# Check the public CLI setup and connectivity.
yamato-pp-cli doctor --json

# Determine the chargeable parcel category.
yamato-pp-cli parcel --length 70 --width 45 --height 30 --weight 23

# Replace the example date with today or a future JST date; read estimated arrival and source list rates.
yamato-pp-cli quote --origin 1000005 --destination 6008216 --date 2026-10-08 --size 160

```

## Cookbook

Resolve the terminal before calculating its airport tariff:

```sh
yamato-pp-cli airports --query Narita --limit 10 --json
yamato-pp-cli quote --service airport --origin 1000005 --airport 006 --date 2026-10-08 --date-kind boarding --size 160 --json
yamato-pp-cli quote --service airport-roundtrip --origin 1000005 --airport 006 --date 2026-10-08 --date-kind boarding --size 160 --detail --json
```

Lodging roundtrip is a separate two-leg tariff. `use` means the intended check-in/use day:

```sh
yamato-pp-cli quote --service roundtrip --origin 1000005 --destination 6008216 --date 2026-10-08 --date-kind use --size 160 --json
yamato-pp-cli quote --origin 1000005 --destination 6008216 --date 2026-10-08 --date-kind delivery --size 160 --json
```

Inspect counter and same-day evidence:

```sh
yamato-pp-cli counters --query narita2_send --json
yamato-pp-cli same-day --query Narita --limit 10 --json
yamato-pp-cli products --service airport --json
```

`quote --date-kind ship` returns an estimated arrival for standard TA-Q-BIN, an earliest boarding date for airport service, or an earliest use date for lodging roundtrip. `delivery`, `boarding` and `use` return a shipping deadline date for their matching product. All calendar dates and displayed cutoff/hour values use JST.

## Rates, eligibility and source limits

- Parcel limits are 200 cm total dimensions and 30 kg. The longest side is at most 170 cm, or 100 cm when the parcel must stay upright. `--size` must also account for weight; use `parcel` first. Contents, packaging and accepting/receiving locations remain unconfirmed.
- Rates are source list tariffs per parcel, including tax. Airport outbound 660 JPY and roundtrip reductions are already included in the relevant source table. Conditional dropoff, member and digital discounts are not automatically deducted. Packaging and other optional charges are excluded. Kansai return airport charges require confirmation against the source conditions.
- Standard TA-Q-BIN and limited-area same-day service have separate eligibility. Same-day covers nine verified Narita/Haneda airport-to-hotel rows. Other routes, counters and directions are handed off to the complete first-party PDF/directory. Published price examples are not quotes for those schedule rows.
- Counter business hours do not establish the dispatch cutoff. Most airport counter hours are image-only and are returned as unknown. Narita Terminal 2 sending hours are transcribed from a verified image and returned only while current image bytes match. Selected same-day schedules are likewise suppressed if the current PDF changes.
- Hotels, local offices, convenience stores, special contents, holidays, islands, weather and transport disruption require source/counter confirmation. No shipment, payment, private tracking or account operation is exposed.

## Agent Usage

```sh
yamato-pp-cli parcel --length 70 --width 45 --height 30 --weight 23 --agent
yamato-pp-cli airports --query Narita --json --select results.items
```

Planning output is compact JSON `{meta,results}`. `meta` includes source URLs, fetch timestamps, SHA-256, response bytes, actual upstream request count and elapsed milliseconds. Optional-source failures remain explicit in `meta.fetch_failures` and stderr. `--detail` expands a quote to all eight source size tariffs. `--select` projects fields; `--limit` (1–50) and `--offset` page airport/counter matches. `same-day --limit` bounds its nine selected rows. Empty collections are `[]`.

`--agent` sets machine-friendly output and avoids prompts. `--dry-run --json` emits a preview without source requests. `--timeout` bounds the entire planning command. Live commands accept `--data-source auto|live`; unsupported local-source requests fail before network access. `parcel` accepts auto/local because its rules are a dated local snapshot.

Exit codes: `0` success, `2` planning-command input validation, `5` source/network/parser failure, `7` source rate limit. Generated root/flag parser errors can exit `1` (including an unrecognized `--data-source` value). A source failure never becomes a guessed price or a confirmed delivery date.

## Health Check

```sh
yamato-pp-cli doctor --json
yamato-pp-cli agent-context --json
```

Public planning requires no API key. Public calculator session cookies are held in memory for one command, then discarded. Source discovery used native Browser Use in an isolated Chrome tab; an earlier Playwright session verified the postal workflow after the in-app browser was unavailable. Both normal browser workflows were reproduced by the HTTP client.

## Troubleshooting

Use `airports` for exact terminal IDs, then `--help` for flag meanings. Seven-digit postal codes may include one hyphen. Dates must be real current/future JST dates and match the chosen product. Source errors name the source/handoff URL; retry or inspect it in a browser. If image/PDF fingerprints change, inspect the current source instead of relying on the prior snapshot.

`--no-learn` or `YAMATO_NO_LEARN=true` disables optional local invocation learning. The generated local store schema is forward-only; an older binary may refuse a store opened by a newer binary. Local-state paths can be relocated with `--home` or `YAMATO_HOME`; inspect `doctor` for resolved paths.

See `.manuscripts/` for source references, typed live assertions, resource measurements, completed reviews and final checks. Generated framework utilities can be discovered with `--help` and `which`.

## Recipes

### parcel

```bash
yamato-pp-cli parcel --length 70 --width 45 --height 30 --weight 23 --agent
```

Classify both size and weight and preserve acceptance unknowns.

### quote

```bash
yamato-pp-cli quote --origin 1000005 --destination 6008216 --date 2026-10-08 --size 160 --agent
```

Use a current/future JST date and correct date-kind. Read current source tariffs and an estimated arrival or shipping deadline; acceptance remains unknown.

### airports

```bash
yamato-pp-cli airports --query 成田 --agent --select results
```

Use live upstream airport codes and Japanese terminal names.

### counters

```bash
yamato-pp-cli counters --query Narita --agent
```

Inspect public send/pickup/floor evidence and fingerprint-checked image hours.

### same-day

```bash
yamato-pp-cli same-day --query Narita --agent
```

Inspect limited-area cutoff schedules and separate example prices.

### products

```bash
yamato-pp-cli products --service airport --agent
```

Review live service policy evidence for standard, airport, roundtrip and same-day products.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Luggage planning
- **`parcel`** — Classify both size and weight and preserve acceptance unknowns.

  _Classify both size and weight and preserve acceptance unknowns._

  ```bash
  yamato-pp-cli parcel --length 70 --width 45 --height 30 --weight 23 --agent
  ```
- **`quote`** — Fetch current source rates and date-specific arrival or shipping deadline.

  _Fetch current source rates and date-specific arrival or shipping deadline._

  ```bash
  yamato-pp-cli quote --origin 1000005 --destination 6008216 --date 2026-10-08 --size 160 --agent
  ```
- **`airports`** — Use live upstream airport codes and Japanese terminal names.

  _Use live upstream airport codes and Japanese terminal names._

  ```bash
  yamato-pp-cli airports --query 成田 --agent
  ```
- **`counters`** — Inspect public send/pickup/floor evidence and fingerprint-checked image hours.

  _Inspect public send/pickup/floor evidence and fingerprint-checked image hours._

  ```bash
  yamato-pp-cli counters --query Narita --agent
  ```
- **`same-day`** — Inspect limited-area cutoff schedules and separate example prices.

  _Inspect limited-area cutoff schedules and separate example prices._

  ```bash
  yamato-pp-cli same-day --query Narita --agent
  ```
- **`products`** — Review live service policy evidence for standard, airport, roundtrip and same-day products.

  _Review live service policy evidence for standard, airport, roundtrip and same-day products._

  ```bash
  yamato-pp-cli products --service airport --agent
  ```
