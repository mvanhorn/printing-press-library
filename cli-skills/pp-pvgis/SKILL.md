---
name: pp-pvgis
description: "The free, single-binary PVGIS CLI — JRC solar-radiation and PV-yield estimates with offline cache, multi-site... Trigger phrases: `estimate solar production at`, `what's the optimal tilt for`, `get TMY weather for`, `PVGIS yield`, `use pvgis`."
author: "Roberto Bissanti"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pvgis-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/pvgis/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# PVGIS — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `pvgis-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install pvgis --cli-only
   ```
2. Verify: `pvgis-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/pvgis/cmd/pvgis-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for pvgis-pp-cli when an agent or pipeline needs ground-truth solar-yield estimates from the JRC PVGIS service without subscribing to commercial APIs. Best when the workflow involves multiple candidate sites, repeated parameter sweeps, or downstream consumers (PVSyst, SAM, custom models) that want clean JSON or CSV.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`sites rank`** — Rank a CSV of candidate sites by annual yield/kWp under the same system spec.

  _Use when an agent has a list of candidate roofs/sites and needs to recommend the highest-yield one without N round trips._

  ```bash
  pvgis-pp-cli sites rank --input sites.csv --tilt 30 --pnom 5 --system-loss 14 --agent
  ```
- **`production sweep`** — Compute a 2D grid of yields across tilt and azimuth, cache it, and serve future point queries from local interpolation.

  _Use before recommending a fixed-mount orientation when the user can choose tilt and azimuth independently._

  ```bash
  pvgis-pp-cli production sweep --lat 45 --lon 9 --pnom 5 --system-loss 14 --tilt-min 0 --tilt-max 60 --azimuth-min -90 --azimuth-max 90 --agent
  ```
- **`sites diff`** — Compare annual production at a baseline site against a target site under identical system specs.

  _Use to quantify how much a relocation or alternate roof loses or gains._

  ```bash
  pvgis-pp-cli sites diff --baseline 45.0,9.0 --target 41.9,12.5 --pnom 5 --system-loss 14 --agent
  ```

### Cross-database insight
- **`production compare`** — Run the same site under PVGIS-SARAH3, PVGIS-NSRDB, and PVGIS-ERA5; report yield deltas.

  _Use when assessing confidence intervals: how much do different databases disagree at this location?_

  ```bash
  pvgis-pp-cli production compare --lat 45 --lon 9 --pnom 1 --system-loss 14 --tilt 30 --agent
  ```
- **`weather similar`** — Rank cached TMYs by similarity to a target site's 12-month temperature and GHI signature.

  _Use to find analog climate sites for transfer-learning yield assumptions._

  ```bash
  pvgis-pp-cli weather similar --to 45.0,9.0 --within sites.csv --top 5 --agent
  ```

## Command Reference

**horizon** — DEM-derived terrain horizon profile — the angular height of distant terrain in 49 azimuth steps.

- `pvgis-pp-cli horizon` — Terrain horizon profile (printhorizon) for a site. Returns the horizon height in degrees at 49 azimuth points from...

**production** — PV system production estimates — monthly summaries and full hourly time series for fixed mountings.

- `pvgis-pp-cli production hourly` — Hourly PV production time series (seriescalc) for a fixed-mount system. Returns one row per hour over the requested...
- `pvgis-pp-cli production monthly` — Monthly and annual PV production (PVcalc) for a fixed-mount system. Returns monthly energy E_m, daily energy E_d,...

**radiation** — Solar radiation queries — monthly irradiance on horizontal, tilted, or optimally-tilted surfaces, with optional terrain horizon.

- `pvgis-pp-cli radiation` — Monthly solar radiation (MRcalc) for a site. Returns 12 monthly values on the horizontal plane, the requested tilt,...

**weather** — Typical Meteorological Year (TMY) weather data — 8760-hour synthetic year representative of long-term climate.

- `pvgis-pp-cli weather` — Typical Meteorological Year for a site. Returns 8760 hourly rows with air temperature T2m, relative humidity, global...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pvgis-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Compare three rooftops under the same system spec

```bash
pvgis-pp-cli sites rank --input rooftops.csv --tilt 30 --pnom 5 --system-loss 14 --json --select rank,site,e_y
```

rooftops.csv must have lat,lon,label columns. Output ranks them by annual production.

### Find the best tilt for a south-facing array

```bash
pvgis-pp-cli production optimal-tilt --lat 45 --lon 9 --azimuth 0 --json --select best_tilt,best_e_y
```

Returns just the winning tilt and its annual production.

### Dump TMY climate as CSV for PVSyst

```bash
pvgis-pp-cli weather tmy --lat 45 --lon 9 --csv > milano-tmy.csv
```

PVSyst and SAM consume hourly CSVs of T, GHI, DNI, DHI, wind.

### Check terrain shading at a hilly site

```bash
pvgis-pp-cli horizon --lat 46.2 --lon 11.3 --json --select 'outputs.horizon_profile'
```

Returns the 49-point DEM horizon in degrees; ingest into pvlib or visualize.

### Compare databases for a North-American site

```bash
pvgis-pp-cli production compare --lat 40.7 --lon -74 --pnom 1 --system-loss 14 --tilt 30 --json --select results.database,results.e_y
```

SARAH3 may not cover this longitude; NSRDB will. See how much they disagree where they overlap.

## Auth Setup

No authentication required.

Run `pvgis-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  pvgis-pp-cli horizon --lat 42 --lon 42 --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
pvgis-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pvgis-pp-cli feedback --stdin < notes.txt
pvgis-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.pvgis-pp-cli/feedback.jsonl`. They are never POSTed unless `PVGIS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PVGIS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
pvgis-pp-cli profile save briefing --json
pvgis-pp-cli --profile briefing horizon --lat 42 --lon 42
pvgis-pp-cli profile list --json
pvgis-pp-cli profile show briefing
pvgis-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `pvgis-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add pvgis-pp-mcp -- pvgis-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pvgis-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   pvgis-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `pvgis-pp-cli <command> --help`.
