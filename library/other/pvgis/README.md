# PVGIS CLI

**The free, single-binary PVGIS CLI — JRC solar-radiation and PV-yield estimates with offline cache, multi-site batch, and agent-native JSON.**

Estimates monthly and hourly PV production, downloads Typical Meteorological Year (TMY) weather, fetches DEM-derived horizon profiles, and finds optimal tilt for any point covered by PVGIS-SARAH3, PVGIS-NSRDB, or PVGIS-ERA5. Adds offline SQLite caching, multi-site fan-out (`sites rank`), and a local 2D tilt/azimuth heatmap (`production sweep`) that no PVGIS endpoint returns directly.

Created by [@robertobissanti](https://github.com/robertobissanti) (Roberto Bissanti).

## Install

The recommended path installs both the `pvgis-pp-cli` binary and the `pp-pvgis` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install pvgis
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install pvgis --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install pvgis --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install pvgis --agent claude-code
npx -y @mvanhorn/printing-press-library install pvgis --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/pvgis/cmd/pvgis-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pvgis-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pvgis --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-pvgis --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install pvgis --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pvgis-current).
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
    "pvgis": {
      "command": "pvgis-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirm the JRC API is reachable from this machine.
pvgis-pp-cli doctor

# Monthly irradiance for a Milano-area site, fixed tilt 30 south.
pvgis-pp-cli radiation monthly --lat 45.0 --lon 9.0 --tilt 30 --azimuth 0 --json

# Annual yield for a 5 kWp crystSi system at the same spot.
pvgis-pp-cli production monthly --lat 45.0 --lon 9.0 --pnom 5 --system-loss 14 --tilt 30 --azimuth 0 --json

# Sweep tilt 0-60 to find the production peak for a south-facing system.
pvgis-pp-cli production optimal-tilt --lat 45.0 --lon 9.0 --azimuth 0 --json

# Pull the full 8760-hour TMY; pipe through jq or save for downstream tools.
pvgis-pp-cli weather tmy --lat 45.0 --lon 9.0 --json --select outputs.tmy_hourly

```

## Unique Features

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

## Usage

Run `pvgis-pp-cli --help` for the full command reference and flag list.

## Commands

### horizon

DEM-derived terrain horizon profile — the angular height of distant terrain in 49 azimuth steps.

- **`pvgis-pp-cli horizon`** - Terrain horizon profile (printhorizon) for a site. Returns the horizon height in degrees at 49 azimuth points from -180 to 180 (step 7.5 degrees), derived from the SRTM DEM. Also returns the sun path at winter and summer solstices for solar-availability analysis.

### production

PV system production estimates — monthly summaries and full hourly time series for fixed mountings.

- **`pvgis-pp-cli production hourly`** - Hourly PV production time series (seriescalc) for a fixed-mount system. Returns one row per hour over the requested year range with power P (W), in-plane irradiance G(i), sun height, air temperature, wind speed. Typical response is 8760 rows per year.
- **`pvgis-pp-cli production monthly`** - Monthly and annual PV production (PVcalc) for a fixed-mount system. Returns monthly energy E_m, daily energy E_d, and in-plane irradiation H(i)_m, plus yearly totals E_y. Accounts for module technology, system losses, and optional terrain horizon.

### radiation

Solar radiation queries — monthly irradiance on horizontal, tilted, or optimally-tilted surfaces, with optional terrain horizon.

- **`pvgis-pp-cli radiation`** - Monthly solar radiation (MRcalc) for a site. Returns 12 monthly values on the horizontal plane, the requested tilt, and/or the locally optimal tilt, plus monthly average air temperature. Uses PVGIS-SARAH3 for Europe/Africa/Asia, PVGIS-NSRDB for the Americas, PVGIS-ERA5 elsewhere.

### weather

Typical Meteorological Year (TMY) weather data — 8760-hour synthetic year representative of long-term climate.

- **`pvgis-pp-cli weather`** - Typical Meteorological Year for a site. Returns 8760 hourly rows with air temperature T2m, relative humidity, global horizontal irradiance G(h), beam normal Gb(n), diffuse Gd(h), longwave IR(h), wind speed WS10m, wind direction WD10m, surface pressure SP. Constructed from the long-term record so it represents climate, not a specific year.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pvgis-pp-cli horizon --lat 42 --lon 42

# JSON for scripting and agents
pvgis-pp-cli horizon --lat 42 --lon 42 --json

# Filter to specific fields
pvgis-pp-cli horizon --lat 42 --lon 42 --json --select id,name,status

# Dry run — show the request without sending
pvgis-pp-cli horizon --lat 42 --lon 42 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pvgis-pp-cli horizon --lat 42 --lon 42 --agent
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
pvgis-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/pvgis-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **All commands hang or return network errors** — PVGIS occasionally has maintenance windows. Run `pvgis-pp-cli doctor` and retry. Use --base-url to pin to https://re.jrc.ec.europa.eu/api/v5_2 if v5_3 is degraded.
- **Response shows `H(i)_m: 0.0` for every month** — MRcalc requires --tilted=1 (or --selectrad=1) plus --tilt/--azimuth to compute the tilted plane. Without it only the horizontal plane is returned.
- **`weather tmy` response is huge** — TMY returns 8760 hourly rows. Use `--select outputs.tmy_hourly[0]` to inspect one row, or pipe into the local SQLite store via `pvgis weather sync --lat --lon`.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**pvlib-python**](https://github.com/pvlib/pvlib-python) — Python (1100 stars)
- [**pvgis-mcp (local)**](https://github.com/internal/pvgis-mcp) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
