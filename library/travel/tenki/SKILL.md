---
name: pp-tenki
description: Resolve Japan sightseeing places, inspect tenki.jp daily/hourly forecasts, compare user weather thresholds, or check mountain and sakura/foliage evidence with source time and year.
author: zjsng
license: Apache-2.0
argument-hint: '<command> [flags]'
allowed-tools: 'Read Bash'
---

# tenki.jp

## Prerequisites: Install the CLI

This skill drives the `tenki-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install tenki --cli-only
   ```
2. Verify: `tenki-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/tenki/cmd/tenki-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Local checkout

Before a public-library merge and catalog refresh, build the checkout with Go 1.26.6 or newer:

```bash
go build -o bin/tenki-pp-cli ./cmd/tenki-pp-cli
bin/tenki-pp-cli --version
```

Use `bin/tenki-pp-cli` instead of the bare command prefix in examples when that verified local binary is not on `$PATH`.

Use command `--help` for current flags and bounds. Product commands read public HTML and write only local cache. Do not use this CLI for summit or route safety, historical weather observations, or unpublished future seasons.

## Choose the branch

- Resolve a name/postal code: `places search`; select the canonical URL explicitly. Leisure searches cover the reported bounded directory, with optional `--directory`, rather than nationwide keyword search.
- Inspect identity and forecast reference: `places show`.
- Inspect dates: `forecast daily`; add `--detail` for separate six-hour intervals and instants.
- Inspect a supported activity window: `forecast hourly --date YYYY-MM-DD --hours HH:00-HH:00`.
- Evaluate supplied thresholds: `compare`; require at least one criterion and choose its deterministic `--sort` key when ranking.
- Inspect mountain evidence: `mountain show`; `--level` matches an actual source altitude.
- Inspect seasonal reports: `seasonal list` or `seasonal show --kind sakura|kouyou --year YYYY`. For ended Sakura name discovery, choose a retained prefecture `--directory`; a miss covers the reported scan.

## Examples

```bash
tenki-pp-cli places search --query 金閣寺 --kind leisure --directory https://tenki.jp/leisure/6/29/ --limit 5 --agent --select results.places.name,results.places.url,results.search_scope
tenki-pp-cli places show --place https://tenki.jp/leisure/6/29/189/7327/ --agent
tenki-pp-cli forecast daily --place https://tenki.jp/forecast/3/16/4410/13101/ --days 3 --agent --select results.periods.date,results.periods.weather,results.periods.precip_probability_pct,results.source
tenki-pp-cli forecast hourly --place https://tenki.jp/forecast/3/16/4410/13101/ --hours 09:00-17:00 --agent --select results.periods.valid_at,results.periods.kind,results.periods.precip_rate_mm_h,results.periods.temperature_c
tenki-pp-cli compare --place https://tenki.jp/forecast/3/16/4410/13101/ --days 3 --max-pop 30 --max-temp 30 --sort max-pop --agent
tenki-pp-cli mountain show --place https://tenki.jp/mountain/famous100/5/25/150.html --level 3000 --limit 8 --agent
tenki-pp-cli seasonal show --kind kouyou --place https://tenki.jp/kouyou/3/13/30139.html --agent --select results.spot,results.source,results.status,results.year
tenki-pp-cli seasonal list --kind sakura --directory https://tenki.jp/sakura/3/16/ --query 上野 --year 2026 --limit 5 --agent
tenki-pp-cli seasonal show --kind sakura --place https://tenki.jp/sakura/3/16/54401.html --year 2026 --agent
```

## Interpret evidence

Read the source timestamps, freshness, actual coverage and destination's `forecast_reference_url` before interpreting values. `meta.source` is the actual transport (`live`/`local`, or `mixed`/`unavailable` for comparisons); `meta.metrics` counts HTTP requests, cache hits and decoded source-response bytes, not network transfer bytes. Comparison decisions carry `meta.data_origin: computed`, a separate axis from acquisition. Missing numeric values are `null`; an explicit provider zero stays zero.

Weather dates/times use Asia/Tokyo. Hourly precipitation describes the preceding `start`–`end` hour; temperature/wind belong to `valid_at`. A 09:00–17:00 comparison uses eight complete rain intervals ending 10:00–17:00 and temperature/wind instants from 09:00 through 17:00, inclusive. The hourly inspection command also shows the 09:00 endpoint and its preceding rain interval. Probabilities are per source interval: their maximum is not the probability of any rain over the outing. Rain thresholds use daily amount in mm or hourly rate in mm/h; wind uses m/s; temperature uses °C. Estimated past hours are `estimated_actual`, not forecasts or municipal station observations.

Daily temperature minima are morning lows and maxima are daytime highs. Today's temperature fields can mix forecasts with estimated actual values; their per-field type stays explicit and cannot certify a forecast-only criterion. Today's daily period can be partial, with remaining weather/probability coverage beginning at issuance. Four six-hour rain intervals coexist with five instant labels; keep their boundaries distinct. Daily values cannot certify an hourly window. Daily wind is `max_of_five_source_instants` at 00/06/12/18/24; it is not a continuous daily maximum. Days 12–14 may contain confidence letters and fewer fields; retain unknown values and source confidence letters as supplied.

Municipal weather refers to the government-office vicinity; leisure and seasonal weather refers to the linked municipality. Preserve requested destination identity separately. Mountain main weather is foothill weather. Altitude tables are `nearby_model_guidance`, with their own `model_initial_at`; they do not establish summit weather, trail condition or ascent safety. A mountain destination cannot earn a summit suitability result from municipal/foothill evidence.

Seasonal spot reports, predictions and normal periods are distinct evidence. Keep `requested_year`, actual `year`, update state, report date and weather issue time separate. A fresh forecast does not refresh seasonal facts. Ended Sakura, a year mismatch or future-date current foliage cannot be positive seasonal evidence. `--season` pairs evidence without a hidden peak-quality criterion. With no condition preference, report that explicitly and retain the raw condition. `--season-condition` supplies an exact raw state preference. If the source supplies no applicable exact-date prediction, future seasonal suitability remains unknown while weather criteria can still be evaluated.

Comparison returns `meets_criteria`, `fails_criteria` or `insufficient_data`, with per-criterion `pass`/`fail`/`unknown`, thresholds, source values and coverage. Missing, stale, unknown-freshness, past-estimated, partial or out-of-horizon evidence cannot produce `meets_criteria`. A definite failed criterion can produce `fails_criteria` while other criteria remain explicitly unknown. Ties are shown; there is no universal sightseeing score. Thresholds describe the user's preferences.

## Output and bounded acquisition

Serialize source calls, including MCP command calls. Pacing is capped at one request per second per provider client/CLI or MCP invocation. Separate concurrent invocations do not coordinate; `--rate-limit` only lowers the per-invocation ceiling and does not enforce an aggregate or global limit.

`--agent` sets JSON, compact, noninteractive defaults. Product stdout is one compact `{meta,results}` document; diagnostics use stderr. `--select` projects comma-separated dotted paths, including array fields. A wholly absent path is an error. Required-input `--dry-run --json` succeeds without inputs/network; real invalid inputs exit `2`, source errors `5`, exhausted rate limits `7`. Structured unavailable statuses are ordinary product outcomes.

Search returns at most 50 candidates across at most two pages. Compare accepts up to five places, fourteen dates and seventy cells; hourly output caps at 72 source rows. Daily `unsupported_dates` identifies absent requested dates before the output limit; `partial_horizon` distinguishes mixed coverage and `truncated` identifies row limiting. Dates outside returned source coverage stay unavailable. `--refresh`/`--data-source live` fetch again; `--data-source local` reads cache only; `--allow-stale` explicitly permits marked stale cache without positive suitability. `--timeout` bounds the whole command. `--cache-dir` selects a local page cache. Cache TTL is one hour for weather/seasonal/model pages and seven days for directories, a CLI policy rather than a verified seasonal/model provider cadence.

## Access

No credentials are needed for public HTML. There is no verified supported public tenki.jp API. [Terms Article 8(6)](https://tenki.jp/docs/rule/) restrict non-browser/RSS acquisition. Successful HTTP access and public distribution do not establish provider authorization. Respect source denials and propagate rate limits. Preserve missing evidence instead of substituting another provider, invented future-season facts or zero-valued failed fetches.

## Unique Capabilities

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

## Auth Setup

No credentials are required for public HTML pages. Access is undocumented and subject to tenki.jp terms; there is no verified supported public API.

Run `tenki-pp-cli doctor` to verify setup.
