---
name: pp-jma
description: "Read Japan Meteorological Agency official daily or weekly forecasts, municipality weather warnings and typhoon uncertainty for Japan trip planning. Use for JMA area or forecast station resolution, checking JMA warning lifecycle, or inspecting JMA cyclone analysis versus forecast. Trigger phrases include \"JMA forecast for Tokyo\", \"JMA warnings for my destination\", \"check typhoon forecast uncertainty\", \"use jma-cli\", and \"run jma-cli\"."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - jma-pp-cli
    install:
      - kind: go
        bins: [jma-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/jma/cmd/jma-pp-cli
---

# JMA source workflow

## Prerequisites: Install the CLI

This skill drives the `jma-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install jma --cli-only
   ```
2. Verify: `jma-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/jma/cmd/jma-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.


1. Resolve the destination with `jma-pp-cli areas search --query Kyoto --kind municipality`, then `areas resolve --area 2610000`. Use the returned source ID; exact name ambiguity is an error. Stations are forecast temperature references; use `stations search` for a station request.
2. Read `forecast get --area 130010 --period all --days 3`. Preserve district/station IDs, issued and valid times, units, source confidence and missing null values. Short extrema time markers are not instantaneous temperatures.
3. Read `warnings get --area 1310100`. Summaries cover all resolved municipalities before pagination. Report source Japanese wording and lifecycle; inspect `--detail` for source hazard properties. `none_reported` concerns retrieved municipality weather products only. `incomplete`, transport failure, missing records and unknown codes do not establish no warnings.
4. Discover current cyclone IDs with `typhoons list`, then fetch selected detail with `typhoons get --id TC2632`. Obtain a current ID before running the example. Distinguish JMA analysis, estimate and forecast. A 70% center probability circle is forecast uncertainty; it does not represent cyclone size or a guaranteed path. Historical analysis tracks in `--detail` have no individual timestamps.
5. Check `meta.coverage`, `meta.sources`, issue ages and `page.next_offset`. Use `--refresh` for a fresh network request. `inventory refresh` is explicit; source inventory reads are offline and do not silently refresh. `--offline` fails on stale/missing weather cache. Finish when the requested source evidence is retrieved and its geographic/temporal limits are reported, or a retrieval failure is clearly reported.

## Agent output

```sh
jma-pp-cli forecast get --area 130010 --agent --select results.series
jma-pp-cli warnings get --area 1340100 --detail --select state,municipalities.events
jma-pp-cli typhoons list --select results.id,results.issued_at
jma-pp-cli stations search --query Tokyo --limit 5
```

JSON is compact by default. Dotted field projection traverses arrays and always retains provenance/pagination. Exit 2 indicates input/projection/resolution error; 4 transport/cache/offline error; 5 incomplete/incompatible source data. Diagnostics go to stderr. Source absence stays null. Use `--cache-dir` to isolate this CLI's cache and an absolute `--home` to isolate framework state. No API key or paid account is required.

## Boundaries

Read [README.md](README.md#forecast-interpretation) when explaining source validity, station coverage or warning completeness. Municipality weather warnings exclude joint river flood bulletins and coastal-zone supplements; warning valid-until is unknown. Issue age is not warning expiry. Present JMA wording accurately without personal safety clearance or evacuation advice. Choose another source for earthquake, tsunami, volcano, radar, observation/history, long-range forecasts, geocoding, transport operation status or weather outside JMA's scope. No purchases, bookings or provider writes.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Source correctness
- **`areas resolve`** — Resolve source IDs and reject ambiguous destination names.

  _Resolve source IDs and reject ambiguous destination names._

  ```bash
  jma-pp-cli areas resolve --area 1310100 --agent
  ```
- **`warnings get`** — Separate applicable no-warning, not-applicable, lifted and incomplete source records.

  _Separate applicable no-warning, not-applicable, lifted and incomplete source records._

  ```bash
  jma-pp-cli warnings get --area 1340100 --agent
  ```
- **`typhoons get`** — Join matching JMA issued/valid-time geometry and intensity documents.

  _Join matching JMA issued/valid-time geometry and intensity documents._

  ```bash
  jma-pp-cli typhoons get --id TC2633 --agent
  ```
- **`typhoons list`** — Discover current source cyclone IDs without detail requests.

  _Discover current source cyclone IDs without detail requests._

  ```bash
  jma-pp-cli typhoons list --agent
  ```
- **`inventory refresh`** — Validate first-party source catalogs before atomic local replacement.

  _Validate first-party source catalogs before atomic local replacement._

  ```bash
  jma-pp-cli inventory refresh --agent
  ```

## Recipes

### Tokyo warnings

```bash
jma-pp-cli warnings get --area 1310100 --agent --select results
```

Municipality-specific source status

### Typhoon discovery

```bash
jma-pp-cli typhoons list --agent
```

Lazy active cyclone inventory

## Auth Setup

Public anonymous HTTPS; no credentials or fees.

Run `jma-pp-cli doctor` to verify setup.
