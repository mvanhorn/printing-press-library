# ePropertyPlus CLI

**Every land bank's public inventory, one command — enumerate, filter, export, and image any ePropertyPlus instance by slug.**

ePropertyPlus runs dozens of US land banks behind one-city-at-a-time web UIs. This CLI talks to their public JSON API parameterized by instance slug, hydrates the full inventory into a local SQLite dataset, splits structures from vacant lots, and exports CSV/GeoJSON for GIS, distressed-asset intelligence, and the Land Designer. No auth.

Printed by [@startos00](https://github.com/startos00) (startos00).

## Install

The recommended path installs both the `epropertyplus-pp-cli` binary and the `pp-epropertyplus` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install epropertyplus
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install epropertyplus --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install epropertyplus --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install epropertyplus --agent claude-code
npx -y @mvanhorn/printing-press-library install epropertyplus --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/epropertyplus-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-epropertyplus --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-epropertyplus --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-epropertyplus skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-epropertyplus. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/epropertyplus-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "epropertyplus": {
      "command": "epropertyplus-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the default instance (kclb) is reachable
epropertyplus-pp-cli doctor

# enumerate + hydrate the whole inventory into the local store
epropertyplus-pp-cli sync --instance kclb

# list vacant lots as CSV for analysis
epropertyplus-pp-cli list --kind lot --csv

# export land parcels as GeoJSON for GIS / Land Designer
epropertyplus-pp-cli export --format geojson --kind lot

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Multi-tenant reach
- **`instances`** — Point the same commands at any ePropertyPlus land bank by slug; manage a list of known instances.

  _Reach for this to compare or batch across many land banks instead of one city's web portal._

  ```bash
  epropertyplus-pp-cli instances list --agent
  ```
- **`compare`** — Compare synced inventories across two or more land-bank instances.

  _Reach for this to benchmark inventory size, structure ratio, or pricing across cities._

  ```bash
  epropertyplus-pp-cli compare --instances kclb,<slug> --agent
  ```

### Local state that compounds
- **`list`** — Filter an instance's inventory to structures (buildings) or vacant lots.

  _Use it to separate photographed buildings from vacant land for condition vs Land-Designer pipelines._

  ```bash
  epropertyplus-pp-cli list --kind structure --agent
  ```
- **`sync`** — Enumerate the index and hydrate every property's full detail into a local SQLite dataset.

  _Reach for this before any offline analysis, search, or export._

  ```bash
  epropertyplus-pp-cli sync --instance kclb
  ```

### Export for downstream
- **`export`** — Export inventory as GIS-ready GeoJSON features using lat/lng and parcel geometry.

  _Use it to drop a land bank's parcels straight into GIS or the Land Designer._

  ```bash
  epropertyplus-pp-cli export --format geojson --kind lot
  ```
- **`land-export`** — Export vacant-land parcels with geometry, zoning, and potential use — the fields the Land Designer needs.

  _Reach for this to feed distressed-land parcels into adaptive-reuse visualization._

  ```bash
  epropertyplus-pp-cli land-export --instance kclb --json
  ```
- **`enrich`** — Emit parcelNumber plus join instructions to county/Socrata condition and market-value data.

  _Use it to attach city-graded condition and assessment to land-bank inventory._

  ```bash
  epropertyplus-pp-cli enrich --parcels --instance kclb --json
  ```

## Recipes


### Structures with detail, slim JSON

```bash
epropertyplus-pp-cli list --kind structure --json --select id,propertyAddress1,parcelNumber,propertyClass,structureType
```

Narrow the deep property record to the join + classification fields agents actually need.

### Land Designer parcel feed

```bash
epropertyplus-pp-cli land-export --instance kclb --json
```

Vacant parcels with geometry, zoning, and potential use for adaptive-reuse visualization.

### Cross-city structure ratio

```bash
epropertyplus-pp-cli compare --instances kclb,<slug> --agent
```

Benchmark how many real buildings vs vacant lots each land bank actually lists.

## Usage

Run `epropertyplus-pp-cli --help` for the full command reference and flag list.

## Commands

### property

Manage property

- **`epropertyplus-pp-cli property get`** - Returns the full property record (parcel number, address, geometry, zoning, potential use, structure type, occupancy, asking price, image thumbnail, and custom condition fields) under the returnVal envelope.
- **`epropertyplus-pp-cli property get-custom-field-configs`** - Returns the instance's custom-field configuration so the cryptic s_custom_*/n_custom_* keys on a Property can be mapped to human-readable names (e.g. condition flags). Note: the upstream wraps the JSON in a 'var customFieldConfigs = JSON.parse(...)' assignment.
- **`epropertyplus-pp-cli property get-image`** - Returns the binary image. The imageId and filename come from a Property's publicThumbImgUrl (path shape /property/viewImage/{imageId}/{filename}).
- **`epropertyplus-pp-cli property list-properties`** - Returns the full set of published public properties as lightweight index rows (id, latitude, longitude, status, class code). This is the enumeration entry point; hydrate each id with getPublishedProperty for full detail.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
epropertyplus-pp-cli property get --property-id 42

# JSON for scripting and agents
epropertyplus-pp-cli property get --property-id 42 --json

# Filter to specific fields
epropertyplus-pp-cli property get --property-id 42 --json --select id,name,status

# Dry run — show the request without sending
epropertyplus-pp-cli property get --property-id 42 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
epropertyplus-pp-cli property get --property-id 42 --agent
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
epropertyplus-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/epropertyplus-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Empty results for an instance** — Verify the slug: open https://public-<slug>.epropertyplus.com/landmgmtpub/ in a browser; set --instance <slug> or EPROPERTYPLUS_BASE_URL.
- **custom-fields output looks like JS, not JSON** — The upstream wraps it in 'var customFieldConfigs = JSON.parse(...)'; the CLI unwraps it — re-run with --json.
