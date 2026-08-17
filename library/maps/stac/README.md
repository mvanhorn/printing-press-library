# STAC CLI

**The first Go single-binary STAC CLI: every search filter the Python tools have, plus an offline cache, scene ranking, coverage, and asset resolution no other STAC tool offers.**

stac-pp-cli searches any STAC API (default: AWS Earth Search) for satellite imagery with full bbox/datetime/intersects/cloud-cover filtering, then goes beyond every existing tool: a local SQLite cache powers offline search, `coverage`, `timeline`, `gaps`, and `watch`; `scenes best` ranks for the clearest image with a client-side fallback; and `assets` resolves provider-aware band download URLs. Agent-native by default with --json, --select, and a typed MCP surface.

Learn more at [STAC](http://stacspec.org).

## Install

The recommended path installs both the `stac-pp-cli` binary and the `pp-stac` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install stac
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install stac --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install stac --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install stac --agent claude-code
npx -y @mvanhorn/printing-press-library install stac --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/maps/stac/cmd/stac-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/stac-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install stac --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-stac --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-stac --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install stac --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/stac-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/maps/stac/cmd/stac-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "stac": {
      "command": "stac-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirm the CLI is healthy and the STAC endpoint is reachable (no auth needed).
stac-pp-cli doctor --dry-run

# List the collections the active STAC endpoint serves.
stac-pp-cli collections get

# Find the single clearest Sentinel-2 scene over San Francisco in June 2024.
stac-pp-cli scenes best --collection sentinel-2-l2a --bbox -122.5,37.7,-122.3,37.9 --datetime 2024-06-01/2024-06-30

# Check whether Landsat covers the AOI and over what time span.
stac-pp-cli coverage --collection landsat-c2-l2 --bbox -122.5,37.7,-122.3,37.9 --datetime 2023-01-01/2024-12-31

```

## Unique Features

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

## Usage

Run `stac-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `STAC_CONFIG_DIR`, `STAC_DATA_DIR`, `STAC_STATE_DIR`, or `STAC_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `STAC_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export STAC_HOME=/srv/stac
stac-pp-cli doctor
```

Under `STAC_HOME=/srv/stac`, the four dirs resolve to `/srv/stac/config`, `/srv/stac/data`, `/srv/stac/state`, and `/srv/stac/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `STAC_DATA_DIR` overrides an explicit `--home` for that kind. Use `STAC_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `STAC_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `stac-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### aggregate

Manage aggregate

- **`stac-pp-cli aggregate`** - Executes one or more aggregations (e.g. cloud_cover_frequency, datetime_frequency) over the items matching the given filters. Returns frequency-distribution buckets or scalar values.

### aggregations

Manage aggregations

- **`stac-pp-cli aggregations get`** - Lists the aggregations available across the catalog (e.g. total_count, datetime_frequency, cloud_cover_frequency).

### collections

All endpoints related to STAC API - Collections

- **`stac-pp-cli collections describe`** - A single Feature Collection for the given if `collectionId`.
Request this endpoint to get a full list of metadata for the Feature Collection.
- **`stac-pp-cli collections get`** - A body of Feature Collections that belong or are used together with additional links.
Request may not return the full set of metadata per Feature Collection.

### conformance

Manage conformance

- **`stac-pp-cli conformance`** - A list of all conformance classes specified in a standard that the
server conforms to.

### items

Manage items

- **`stac-pp-cli items get-search`** - Retrieve Items matching filters. Intended as a shorthand API for simple
queries.

This method is required to implement.

If this endpoint is implemented on a server, it is required to add a
link referring to this endpoint with `rel` set to `search` to the
`links` array in `GET /`. As `GET` is the default method, the `method`
may not be set explicitly in the link.
- **`stac-pp-cli items post-search`** - Retrieve items matching filters. Intended as the standard, full-featured
query API.

This method is optional to implement, but recommended.

If this endpoint is implemented on a server, it is required to add a
link referring to this endpoint with `rel` set to `search` and `method`
set to `POST` to the `links` array in `GET /`.

### queryables

Manage queryables

- **`stac-pp-cli queryables get`** - Returns the set of queryable properties (JSON Schema) usable in query/filter across the whole catalog.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
stac-pp-cli aggregate

# JSON for scripting and agents
stac-pp-cli aggregate --json

# Filter to specific fields
stac-pp-cli aggregate --json --select id,name,status

# Dry run — show the request without sending
stac-pp-cli aggregate --dry-run

# Agent mode — JSON + compact + no prompts in one flag
stac-pp-cli aggregate --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
stac-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `stac-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/stac-pp-cli/config.toml`; `--home`, `STAC_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Search returns items ignoring my --filter CQL2 expression** — Earth Search does not implement CQL2; use --max-cloud / --query which compile to the supported query extension. Run 'stac-pp-cli conformance' to see what the active provider supports.
- **sortby returns a 400 from the server** — Some providers reject server-side sort; 'scenes best' falls back to client-side cloud-cover ranking automatically.
- **A local command returns [] with a 'no local mirror' hint** — Run 'stac-pp-cli sync --resources items' first to populate the offline store.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**eodag**](https://github.com/CS-SI/eodag) — Python (600 stars)
- [**pystac-client**](https://github.com/stac-utils/pystac-client) — Python (380 stars)
- [**stactools**](https://github.com/stac-utils/stactools) — Python (280 stars)
- [**rustac**](https://github.com/stac-utils/rustac) — Rust (240 stars)
- [**go-stac**](https://github.com/planetlabs/go-stac) — Go (180 stars)
- [**stac-mcp**](https://github.com/Wayfinder-Foundry/stac-mcp) — Python (12 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
