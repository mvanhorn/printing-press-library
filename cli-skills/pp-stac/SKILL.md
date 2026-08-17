---
name: pp-stac
description: "The first Go single-binary STAC CLI: every search filter the Python tools have, plus an offline cache, scene ranking, coverage, and asset resolution no other STAC tool offers. Trigger phrases: `find a satellite image of`, `least cloudy scene over`, `what sentinel-2 imagery is available for`, `check coverage for this area`, `get the band URLs for this scene`, `use stac`, `run stac`."
author: "ghltshubh"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - stac-pp-cli
    install:
      - kind: go
        bins: [stac-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/maps/stac/cmd/stac-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/maps/stac/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# STAC — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `stac-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install stac --cli-only
   ```
2. Verify: `stac-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/maps/stac/cmd/stac-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

stac-pp-cli searches any STAC API (default: AWS Earth Search) for satellite imagery with full bbox/datetime/intersects/cloud-cover filtering, then goes beyond every existing tool: a local SQLite cache powers offline search, `coverage`, `timeline`, `gaps`, and `watch`; `scenes best` ranks for the clearest image with a client-side fallback; and `assets` resolves provider-aware band download URLs. Agent-native by default with --json, --select, and a typed MCP surface.

## When to Use This CLI

Use stac-pp-cli when an agent or analyst needs to discover satellite imagery (Sentinel-2, Landsat, Sentinel-1, NAIP, Copernicus DEM) from a STAC API, pick the clearest scene over an area, check temporal coverage, or resolve band download URLs. It is ideal for scripting EO discovery and for agent workflows that need bounded, structured output instead of verbose GeoJSON.

## Anti-triggers

Do not use this CLI for:
- Downloading or reprojecting the actual raster pixels (use rasterio, gdal, or stackstac/odc-stac after resolving URLs with 'assets')
- Authoring or validating static STAC catalogs (use stactools or go-stac)
- Tiling or rendering imagery for a web map (use titiler)

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Scene selection
- **`scenes best`** — Pick the single clearest scene over an area and date range.

  _Reach for this when an agent needs the one best image for an AOI, not a list to sift through._

  ```bash
  stac-pp-cli scenes best --collection sentinel-2-l2a --bbox -122.5,37.7,-122.3,37.9 --datetime 2024-06-01/2024-06-30 --agent
  ```
- **`clouds`** — Show the cloud-cover distribution over an area and date range.

  _Use before downloading to judge how cloudy a window is without pulling any imagery._

  ```bash
  stac-pp-cli clouds --collection sentinel-2-l2a --bbox -122.5,37.7,-122.3,37.9 --datetime 2024-06-01/2024-08-31 --agent
  ```

### Coverage and time
- **`coverage`** — Answer whether data exists over an area and when, with a temporal summary.

  _Reach for this to confirm an AOI has usable data before planning an analysis._

  ```bash
  stac-pp-cli coverage --collection landsat-c2-l2 --bbox -122.5,37.7,-122.3,37.9 --datetime 2023-01-01/2024-12-31 --agent
  ```
- **`timeline`** — Build a per-date time series of scenes for change detection.

  _Use to assemble a clean observation series for NDVI or change detection._

  ```bash
  stac-pp-cli timeline --collection sentinel-2-l2a --bbox -122.5,37.7,-122.3,37.9 --datetime 2024-01-01/2024-06-30 --max-cloud 30 --agent
  ```
- **`gaps`** — Find missing acquisition windows over an area versus expected revisit.

  _Reach for this to spot coverage holes before relying on a time series._

  ```bash
  stac-pp-cli gaps --collection sentinel-2-l2a --bbox -122.5,37.7,-122.3,37.9 --datetime 2024-01-01/2024-06-30 --revisit-days 5 --agent
  ```
- **`watch`** — Report scenes published over an area since the last check.

  _Use to monitor an AOI for fresh imagery across runs._

  ```bash
  stac-pp-cli watch --collection sentinel-2-l2a --bbox -122.5,37.7,-122.3,37.9 --agent
  ```

### Assets and analysis
- **`assets`** — Resolve an item's band and asset download URLs, provider-aware.

  _Reach for this to get ready-to-download band URLs without parsing the raw item JSON._

  ```bash
  stac-pp-cli assets S2A_10SEG_20240622_0_L2A --collection sentinel-2-l2a --bands red,green,blue,nir --agent
  ```
- **`compare`** — Compare two collections over the same area and date range.

  _Use to choose between sensors (e.g. Sentinel-2 vs Landsat) for an AOI._

  ```bash
  stac-pp-cli compare sentinel-2-l2a landsat-c2-l2 --bbox -122.5,37.7,-122.3,37.9 --datetime 2024-01-01/2024-06-30 --agent
  ```
- **`stack-snippet`** — Emit a ready-to-paste stackstac/odc-stac Python snippet for matched scenes.

  _Reach for this to jump from scene discovery straight into an xarray workflow._

  ```bash
  stac-pp-cli stack-snippet --collection sentinel-2-l2a --bbox -122.5,37.7,-122.3,37.9 --datetime 2024-06-01/2024-06-30 --bands red,green,blue,nir
  ```

## Command Reference

**aggregate** — Manage aggregate

- `stac-pp-cli aggregate` — Executes one or more aggregations (e.g.

**aggregations** — Manage aggregations

- `stac-pp-cli aggregations get` — Lists the aggregations available across the catalog (e.g. total_count, datetime_frequency, cloud_cover_frequency).

**collections** — All endpoints related to STAC API - Collections

- `stac-pp-cli collections describe` — A single Feature Collection for the given if `collectionId`.
- `stac-pp-cli collections get` — A body of Feature Collections that belong or are used together with additional links.

**conformance** — Manage conformance

- `stac-pp-cli conformance` — A list of all conformance classes specified in a standard that the server conforms to.

**items** — Manage items

- `stac-pp-cli items get-search` — Retrieve Items matching filters. Intended as a shorthand API for simple queries. This method is required to implement.
- `stac-pp-cli items post-search` — Retrieve items matching filters. Intended as the standard, full-featured query API.

**queryables** — Manage queryables

- `stac-pp-cli queryables get` — Returns the set of queryable properties (JSON Schema) usable in query/filter across the whole catalog.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
stac-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Clearest scene this season

```bash
stac-pp-cli scenes best --collection sentinel-2-l2a --bbox -122.5,37.7,-122.3,37.9 --datetime 2024-06-01/2024-09-30
```

Ranks scenes by cloud cover and returns the single best one with its band URLs.

### Trim a verbose search for an agent

```bash
stac-pp-cli items get-search --collections sentinel-2-l2a --bbox -122.5,37.7,-122.3,37.9 --datetime 2024-06-01/2024-06-30 --agent --select 'features.id,features.properties.eo:cloud_cover,features.properties.datetime'
```

Pairs --agent with --select dotted paths to return only id, cloud cover, and datetime from the deeply nested item response.

### Is there any data here?

```bash
stac-pp-cli coverage --collection sentinel-2-l2a --bbox 2.2,48.8,2.4,48.9 --datetime 2024-01-01/2024-12-31
```

Returns the matched count and temporal min/max/frequency so you know if an AOI is worth analyzing.

### Build a clean time series

```bash
stac-pp-cli timeline --collection sentinel-2-l2a --bbox -122.5,37.7,-122.3,37.9 --datetime 2024-01-01/2024-06-30 --max-cloud 30
```

One low-cloud observation per date, deduped by orbit and tile, ready for change detection.

### Get band download URLs

```bash
stac-pp-cli assets S2A_10SEG_20240622_0_L2A --collection sentinel-2-l2a --bands red,green,blue,nir
```

Resolves the cloud-optimized GeoTIFF URLs for the requested bands.

## Auth Setup

No authentication required.

Run `stac-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  stac-pp-cli aggregate --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `STAC_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `STAC_CONFIG_DIR`, `STAC_DATA_DIR`, `STAC_STATE_DIR`, `STAC_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `STAC_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `stac-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "stac": {
        "command": "stac-pp-mcp",
        "env": {
          "STAC_HOME": "/srv/stac"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `STAC_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `STAC_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
stac-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
stac-pp-cli feedback --stdin < notes.txt
stac-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `STAC_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `STAC_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
stac-pp-cli profile save briefing --json
stac-pp-cli --profile briefing aggregate
stac-pp-cli profile list --json
stac-pp-cli profile show briefing
stac-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `stac-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/maps/stac/cmd/stac-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add stac-pp-mcp -- stac-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which stac-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   stac-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `stac-pp-cli <command> --help`.
