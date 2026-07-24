# ArcGIS REST CLI

**Point it at any ArcGIS FeatureServer or MapServer URL and pull, discover, or spatially bind features, with correct pagination, CSV/GeoJSON output, and a local store no other ArcGIS tool keeps.**

Every incumbent dumps one layer to GeoJSON and stops. This CLI discovers whole service directories, introspects layer schemas, does point-in-polygon signal binding, keeps an offline SQLite mirror you can SQL and diff, and outputs CSV straight into a data loader. Works anonymously against public endpoints; optional ArcGIS token for secured layers.

Created by [@togorashi45](https://github.com/togorashi45) (togorashi45).

## Install

The recommended path installs both the `arcgis-pp-cli` binary and the `pp-arcgis` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install arcgis
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install arcgis --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install arcgis --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install arcgis --agent claude-code
npx -y @mvanhorn/printing-press-library install arcgis --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/cmd/arcgis-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/arcgis-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install arcgis --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-arcgis --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-arcgis --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install arcgis --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/arcgis-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/cmd/arcgis-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "arcgis": {
      "command": "arcgis-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the CLI is wired before hitting a live server
arcgis-pp-cli doctor --dry-run


# see a layer's fields, geometry type, and maxRecordCount
arcgis-pp-cli fields https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0


# how many rows before you pull
arcgis-pp-cli count https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0 --where "FLD_ZONE='AE'"


# pull matching features, paginated, to CSV
arcgis-pp-cli query https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0 --where "FLD_ZONE='AE'" --format csv --out zones.csv

```

## Unique Features

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

## Usage

Run `arcgis-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ARCGIS_CONFIG_DIR`, `ARCGIS_DATA_DIR`, `ARCGIS_STATE_DIR`, or `ARCGIS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ARCGIS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ARCGIS_HOME=/srv/arcgis
arcgis-pp-cli doctor
```

Under `ARCGIS_HOME=/srv/arcgis`, the four dirs resolve to `/srv/arcgis/config`, `/srv/arcgis/data`, `/srv/arcgis/state`, and `/srv/arcgis/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `ARCGIS_DATA_DIR` overrides an explicit `--home` for that kind. Use `ARCGIS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ARCGIS_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `arcgis-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### catalog

Browse services on the configured ArcGIS server (demo endpoint; most commands take an explicit URL instead)

- **`arcgis-pp-cli catalog`** - List services published at the configured base server

### features

Local mirror of features synced from an ArcGIS layer (populated by 'sync')

- **`arcgis-pp-cli features`** - List features from the local store (use 'sync <layer-url>' first)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
arcgis-pp-cli features

# JSON for scripting and agents
arcgis-pp-cli features --json

# Filter to specific fields
arcgis-pp-cli features --json --select id,name,status

# Dry run — show the request without sending
arcgis-pp-cli features --dry-run

# Agent mode — JSON + compact + no prompts in one flag
arcgis-pp-cli features --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
arcgis-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `arcgis-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/arcgis-pp-cli/config.toml`; `--home`, `ARCGIS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ARCGIS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `arcgis-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Only ~1000-2000 rows came back** — the layer hit maxRecordCount; query paginates automatically — if the server lacks resultOffset support, add --tile
- **exceededTransferLimit true but no pagination** — add --tile to subdivide the envelope, or --pager oid to force OBJECTID chunking
- **403 or token required** — the layer is secured; pass --token or set ARCGIS_TOKEN

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**pyesridump**](https://github.com/openaddresses/pyesridump) — Python (400 stars)
- [**esri-dump**](https://github.com/openaddresses/esri-dump) — JavaScript (150 stars)
- [**gis_scraper**](https://github.com/MatzFan/gis_scraper) — Ruby (30 stars)
- [**ezesri**](https://github.com/stiles/ezesri) — Python (20 stars)
- [**esridumpgdf**](https://github.com/wchatx/esridumpgdf) — Python (15 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
