---
name: pp-arcgis
description: "Point it at any ArcGIS FeatureServer or MapServer URL and pull, discover, or spatially bind features, with correct pagination, CSV/GeoJSON output, and a local store no other ArcGIS tool keeps. Trigger phrases: `pull an arcgis layer`, `dump a featureserver`, `discover an arcgis service`, `arcgis fields for this layer`, `point in polygon parcel`, `use arcgis`, `run arcgis`."
author: "togorashi45"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - arcgis-pp-cli
    install:
      - kind: go
        bins: [arcgis-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/cmd/arcgis-pp-cli
---

# ArcGIS REST — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `arcgis-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install arcgis --cli-only
   ```
2. Verify: `arcgis-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every incumbent dumps one layer to GeoJSON and stops. This CLI discovers whole service directories, introspects layer schemas, does point-in-polygon signal binding, keeps an offline SQLite mirror you can SQL and diff, and outputs CSV straight into a data loader. Works anonymously against public endpoints; optional ArcGIS token for secured layers.

## When to Use This CLI

Use this CLI to pull, discover, or spatially query any ArcGIS REST FeatureServer/MapServer — county parcels, code-enforcement, permits, flood zones. It is the right tool when you need correct pagination, CSV/GeoJSON for a data loader, an offline mirror, or point-in-polygon binding of a coordinate to its containing feature.

## Anti-triggers

Do not use this CLI for:
- Do not use it to render maps or tiles — it extracts and queries data, not imagery.
- Do not use it for non-Esri OGC/WFS endpoints; it speaks the ArcGIS REST query protocol specifically.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Spatial signal binding

- **`locate`** — Bind a coordinate (or a CSV of coordinates) to the containing feature via one intersects query.

  _Reach for this to attach a lat/lng distress signal to the exact parcel polygon that contains it._

  ```bash
  arcgis-pp-cli locate https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0 --point -71.46568,42.42269 --fields FLD_ZONE --json
  ```

### Schema intelligence

- **`audit`** — Walk every layer in a service and report which high-value fields (owner, mailing address, homestead, last-sale) each layer exposes.

  _Reach for this to see which distress-signal fields a county actually exposes before building a load._

  ```bash
  arcgis-pp-cli audit https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer --agent
  ```

### Local state that compounds

- **`diff`** — Re-query a layer and diff a tracked field against the last local sync to surface changes.

  _Reach for this to catch ownership/value changes between pulls without a deed scraper._

  ```bash
  arcgis-pp-cli diff https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0 --track FLD_ZONE --where "OBJECTID<=30" --agent
  ```
- **`sql`** — Run SQLite/FTS queries over a synced layer's attributes with no further API calls.

  _Reach for this to slice a synced layer locally and compose with jq. Requires a prior sync._

  ```bash
  arcgis-pp-cli sql "SELECT json_extract(attributes,'$.FLD_ZONE') AS zone, count(*) c FROM features GROUP BY zone ORDER BY c DESC"
  ```

### Robust extraction

- **`query`** — Auto-subdivide the query envelope into quadrants when a layer exceeds the transfer limit and lacks pagination.

  _Reach for this on large layers that return exceededTransferLimit but do not support resultOffset paging._

  ```bash
  arcgis-pp-cli query https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0 --where "FLD_ZONE='AE'" --limit 5 --format geojson
  ```
- **`stats`** — Get counts/sums grouped by a field via outStatistics without downloading rows.

  _Reach for this for per-category aggregates (parcels per land-use, acreage by owner) without a full pull._

  ```bash
  arcgis-pp-cli stats https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0 --group-by FLD_ZONE --out count --agent
  ```

## Command Reference

**catalog** — Browse services on the configured ArcGIS server (demo endpoint; most commands take an explicit URL instead)

- `arcgis-pp-cli catalog` — List services published at the configured base server

**features** — Local mirror of features synced from an ArcGIS layer (populated by 'sync')

- `arcgis-pp-cli features` — List features from the local store (use 'sync <layer-url>' first)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
arcgis-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Pull matching features to CSV

```bash
arcgis-pp-cli query https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0 --where "FLD_ZONE='AE'" --fields FLD_ZONE,SFHA_TF --format csv --out zones.csv
```

Paginated pull, only the fields you need.

### Bind a coordinate to its containing polygon

```bash
arcgis-pp-cli locate https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0 --point -71.46568,42.42269 --agent --select features
```

One intersects query returns the containing feature; --select narrows the response.

### Audit a service's fields before a load

```bash
arcgis-pp-cli audit https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer --agent
```

Reports which high-value fields each layer exposes.

### Aggregate without a full pull

```bash
arcgis-pp-cli stats https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0 --group-by FLD_ZONE --out count --agent --select features
```

Server-side group-by counts.

### Detect changes since last sync

```bash
arcgis-pp-cli diff https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0 --track FLD_ZONE --agent
```

Diffs a tracked field against the local sync baseline (run sync first).

## Auth Setup

No authentication required.

Run `arcgis-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  arcgis-pp-cli features --agent --select id,name,status
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `ARCGIS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `ARCGIS_CONFIG_DIR`, `ARCGIS_DATA_DIR`, `ARCGIS_STATE_DIR`, `ARCGIS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `ARCGIS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `arcgis-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "arcgis": {
        "command": "arcgis-pp-mcp",
        "env": {
          "ARCGIS_HOME": "/srv/arcgis"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `ARCGIS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `ARCGIS_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
arcgis-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
arcgis-pp-cli feedback --stdin < notes.txt
arcgis-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `ARCGIS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ARCGIS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
arcgis-pp-cli profile save briefing --json
arcgis-pp-cli --profile briefing features
arcgis-pp-cli profile list --json
arcgis-pp-cli profile show briefing
arcgis-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `arcgis-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/cmd/arcgis-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add arcgis-pp-mcp -- arcgis-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which arcgis-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   arcgis-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `arcgis-pp-cli <command> --help`.
