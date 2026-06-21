# PDOK Location CLI

**One Go binary for every Dutch location workflow — geocode, reverse-geocode, batch CSVs, full GeoJSON geometries, and RD↔WGS84 conversion built in.**

PDOK Location wraps both Dutch government location services in one CLI because they are complementary, not redundant. The Solr-based **Locatieserver** is the established text geocoder — free-text matches, suggest/lookup chains, type-filtered reverse geocoding, ranked Solr scores. The OGC **Kadaster Location API** ships what Locatieserver cannot — full GeoJSON geometries, bbox filtering, and multi-CRS output (WGS84, Dutch RD, Web Mercator, ETRS89). A common Dutch geo workflow uses both: geocode text on Locatieserver, then pull full geometry on the Location API. Bundling them under one binary, one config, and one local cache turns that workflow into a single tool. `doctor` probes each service individually so when one upstream is down you see which commands still work.

### Why combine these two APIs in one CLI?

These two PDOK services cover overlapping but non-redundant ground in the Dutch geo domain.

| Service | Style | Returns | Strengths |
|---------|-------|---------|-----------|
| **Locatieserver** (`/bzk/locatieserver/search/v3_1`) | Solr-based text geocoding | Records with centroid coords (WKT) | Free-text search, suggest/lookup chain, reverse geocoding by lat/lon or RD, ranked Solr scores |
| **Kadaster Location API** (`/kadaster/location-api/v1`) | OGC API Features | Full GeoJSON features | Multi-collection (adres, perceel, gebouw, …), bbox filter, 4-CRS support, native GeoJSON output |

Each is incomplete without the other:
- Locatieserver lacks bbox and full geometries.
- Location API lacks Solr search ranking and reverse geocoding.

Combining them lets a single sync warm a shared local cache, a single config covers both, and `doctor` distinguishes which upstream is up. Future PDOK siblings (BAG, BRK direct, BGT, roads, etc.) will ship as separate `pdok-<name>-pp-cli` binaries to keep this CLI focused on "find a Dutch location → get full geometry".

Created by [@markvandeven](https://github.com/markvandeven) (markvandeven).

## Install

The recommended path installs both the `pdok-location-pp-cli` binary and the `pp-pdok-location` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install pdok-location
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install pdok-location --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install pdok-location --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install pdok-location --agent claude-code
npx -y @mvanhorn/printing-press-library install pdok-location --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/pdok-location/cmd/pdok-location-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pdok-location-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install pdok-location --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pdok-location --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pdok-location --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install pdok-location --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pdok-location-current).
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
    "pdok-location": {
      "command": "pdok-location-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Both PDOK Location services are free, open, and require no authentication or API key. The CLI works out of the box — no env vars to set, no doctor failures on a fresh install.

## Quick Start

```bash
# Verify the API is reachable with a real Dutch address — should return one match with lat/lon and RD coords.
pdok-location-pp-cli free --q 'Hertog Aalbrechtweg 5 1823DL Alkmaar' --rows 1 --json

# Demonstrate the suggest→lookup chain returning full GeoJSON geometry for the canonical match.
pdok-location-pp-cli resolve 'Damrak Amsterdam' --geojson

# Seed the local gazetteer (342 gemeenten, 12 provincies) so offline lookups work immediately.
pdok-location-pp-cli sync

# Show that gemeente lookups are now served from the local cache.
pdok-location-pp-cli gemeente get amsterdam --json

# Cross-source reverse geocode — nearest address, parcel, hectometer marker, and gemeente in one call.
pdok-location-pp-cli nearest --lat 52.3731 --lon 4.8922 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Batch and bulk workflows
- **`batch geocode`** — Geocode an entire CSV of Dutch addresses in one command — outputs lat/lon, RD X/Y, match score, and an error column for failed rows.

  _When an agent needs to enrich a list of Dutch addresses, this is the one-shot command — no per-row scripting, with cached re-runs and a clear error column._

  ```bash
  pdok-location-pp-cli batch geocode incidents.csv --address-col street --out incidents-geocoded.csv
  ```
- **`features search`** — Search across multiple Location API OGC collections (adres, perceel, gebouw, ...) with an optional bounding-box filter, flattened to JSON or CSV.

  _When an agent needs a flat CSV of address+parcel matches inside a study area, this is the lightest path from a query to a multi-collection result set._

  ```bash
  pdok-location-pp-cli features search --query damrak --collections adres,perceel --bbox 4.85,52.36,4.92,52.40 --csv
  ```

### Geocoding workflows
- **`resolve`** — Type a partial address, get the canonical match with full GeoJSON geometry — the suggest→lookup chain collapsed into a single command.

  _When an agent needs to turn imprecise user text into a canonical address with geometry, this is the cheapest path._

  ```bash
  pdok-location-pp-cli resolve 'Damrak Amsterdam' --geojson
  ```
- **`nearest`** — From any lat/lon or RD coordinate, return the nearest address, nearest parcel, nearest hectometer marker, and the gemeente/provincie containing the point — all in one call.

  _Reverse-geocoding usually needs four separate API calls; this one returns the full picture in one shot, ideal for field-data enrichment pipelines._

  ```bash
  pdok-location-pp-cli nearest --lat 52.3731 --lon 4.8922 --json
  ```
- **`top`** — Return the single best match for a query if (and only if) the Solr score clears a threshold — exits non-zero when no match clears the bar.

  _When an agent needs a yes/no on whether an address is well-known, this is the predicate._

  ```bash
  pdok-location-pp-cli top 'Hertog Aalbrechtweg 5 1823DL Alkmaar' --min-score 5.0 --require-type adres --json
  ```

### Offline conversions
- **`convert rd-to-ll`** — Convert Dutch RD (EPSG:28992) coordinates to WGS84 lat/lon and back, with pure-math precision — no API call needed.

  _When an agent must reconcile Dutch RD coords with global WGS84, this is the local, deterministic path — no rate limit, no auth._

  ```bash
  pdok-location-pp-cli convert rd-to-ll 121200 488000 --json
  ```
- **`convert wkt-to-geojson`** — Take any WKT geometry (POINT, POLYGON, MULTIPOLYGON, MULTILINESTRING) and emit GeoJSON — or convert GeoJSON back to WKT.

  _When an agent needs to feed a PDOK response into mapping or analysis tooling expecting GeoJSON, this saves a parser._

  ```bash
  pdok-location-pp-cli convert wkt-to-geojson 'POINT(4.76 52.64)'
  ```

### Local state that compounds
- **`gemeente get`** — After a one-time sync, look up any of the 342 Dutch gemeenten or 12 provincies fully offline — name, centroid, codes, parent provincie.

  _For any agent workflow that asks 'which gemeente?' or 'list all gemeenten in this provincie', the answer is local and instant._

  ```bash
  pdok-location-pp-cli gemeente get amsterdam --json
  ```
- **`search`** — Full-text search across every address, gemeente, and lookup you've previously fetched — finds matches locally before falling back to the API.

  _When an agent repeats lookups on the same dataset, this short-circuits the API entirely once the cache is warm._

  ```bash
  pdok-location-pp-cli search 'amsterdam centraal' --json
  ```
- **`gemeente of-point`** — Given any lat/lon or RD coordinate, return which gemeente and provincie contain it — using the offline gazetteer with an API fallback.

  _For 'which municipality is this incident in' questions an agent asks repeatedly, this is the cached, offline-first answer._

  ```bash
  pdok-location-pp-cli gemeente of-point --lat 52.3731 --lon 4.8922 --json
  ```

### Service-specific identity
- **`perceel lookup`** — Look up a Dutch cadastral parcel by its full kadastrale aanduiding (e.g. 'AMR03 N 1234') and get back the GeoJSON parcel feature.

  _For cadastral investigations, this is the only command-line path from aanduiding text to a GeoJSON parcel._

  ```bash
  pdok-location-pp-cli perceel lookup --aanduiding 'ASD02 A 4332' --json
  ```

## Usage

Run `pdok-location-pp-cli --help` for the full command reference and flag list.

## Commands

### collections

Manage collections

- **`pdok-location-pp-cli collections get`** - A list of all collections (geospatial data resources) in this dataset.
- **`pdok-location-pp-cli collections get-adres`** - adres collection (geospatial data resource) in this dataset.
- **`pdok-location-pp-cli collections get-functioneel-gebied`** - functioneel_gebied collection (geospatial data resource) in this dataset.
- **`pdok-location-pp-cli collections get-gebouw`** - gebouw collection (geospatial data resource) in this dataset.
- **`pdok-location-pp-cli collections get-gemeentegebied`** - gemeentegebied collection (geospatial data resource) in this dataset.
- **`pdok-location-pp-cli collections get-geografisch-gebied`** - geografisch_gebied collection (geospatial data resource) in this dataset.
- **`pdok-location-pp-cli collections get-inrichtingselement`** - inrichtingselement collection (geospatial data resource) in this dataset.
- **`pdok-location-pp-cli collections get-perceel`** - perceel collection (geospatial data resource) in this dataset.
- **`pdok-location-pp-cli collections get-plaats`** - plaats collection (geospatial data resource) in this dataset.
- **`pdok-location-pp-cli collections get-provinciegebied`** - provinciegebied collection (geospatial data resource) in this dataset.
- **`pdok-location-pp-cli collections get-spoorbaandeel`** - spoorbaandeel collection (geospatial data resource) in this dataset.
- **`pdok-location-pp-cli collections get-waterdeel`** - waterdeel collection (geospatial data resource) in this dataset.
- **`pdok-location-pp-cli collections get-wegdeel`** - wegdeel collection (geospatial data resource) in this dataset.
- **`pdok-location-pp-cli collections get-woonplaats`** - woonplaats collection (geospatial data resource) in this dataset.

### conformance

Manage conformance

- **`pdok-location-pp-cli conformance`** - A list of all conformance classes specified in a standard that the server conforms to.

### free

Manage free

- **`pdok-location-pp-cli free`** - De Free API biedt de mogelijkheid om vrij te zoeken (klassiek geocoderen), waar zonder
tussenkomst van suggesties de API direct resultaten teruggeeft op basis van de zoekopdracht.

### lookup

Manage lookup

- **`pdok-location-pp-cli lookup`** - Zodra er op basis van suggesties van de Suggest API een keuze is gemaakt, wordt de
Lookup API aangeroepen, welke o.a. een (versimpelde) geometrie van de zoekopdracht
teruggeeft.

### pdok-location-api

Manage pdok location api

- **`pdok-location-pp-cli pdok-location-api`** - This document

### pdok-location-search

Manage pdok location search

- **`pdok-location-pp-cli pdok-location-search`** - This endpoint allows one to implement autocomplete functionality for location search. The `q` parameter accepts a partial location name and will return all matching locations up to the specified `limit`. The list of search results are offered as features (in GeoJSON, JSON-FG) but contain only minimal information; like a feature ID, highlighted text and a bounding box. When you want to get the full feature you must follow the included link (`href`) in the search result. This allows one to retrieve all properties of the feature and the full geometry from the corresponding OGC API.

### reverse

Manage reverse

- **`pdok-location-pp-cli reverse`** - De Reverse API biedt de mogelijkheid om een locatie (punt geometrie) op te geven
om vervolgens verschillende gegevens in een range rondom deze locatie te ontvangen.

### suggest

Manage suggest

- **`pdok-location-pp-cli suggest`** - De Suggest API biedt de mogelijkheid om een (gedeelte van een) zoekopdracht op
te voeren, waarnaar er suggesties teruggegeven worden.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pdok-location-pp-cli collections get

# JSON for scripting and agents
pdok-location-pp-cli collections get --json

# Filter to specific fields
pdok-location-pp-cli collections get --json --select id,name,status

# Dry run — show the request without sending
pdok-location-pp-cli collections get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pdok-location-pp-cli collections get --agent
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
pdok-location-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/pdok-location-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Geocoding returns numFound > 0 but score below your threshold.** — Lower --min-score (default Solr scores are usually 5-15) or relax --type / --bron filters.
- **centroide_ll comes back blank.** — Add 'centroide_ll' to --fl, or use --all-fields to request the full default field list.
- **reverse returns numFound=0 with --distance set.** — Remove --distance or widen it; the default 50m radius is sometimes too tight for less dense areas.
- **Bulk batch geocode hits sustained 5xx errors.** — Lower --concurrency (default 5) and add --retries 3; the limiter will back off on 429.
- **Local FTS search returns no results.** — Run 'pdok-location-pp-cli sync' first, then geocode or lookup at least a few addresses to populate the cache.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**foarsitter/locatieserver**](https://github.com/foarsitter/locatieserver) — Python
- [**Amsterdam/pdok-api-client**](https://github.com/Amsterdam/pdok-api-client) — Python
- [**uRosConf/nlgeocoder**](https://github.com/uRosConf/nlgeocoder) — R
- [**oldgeogap/angular-locatieserver-geocoder**](https://github.com/oldgeogap/angular-locatieserver-geocoder) — TypeScript
- [**PDOK/locatieserver (docs+wiki)**](https://github.com/PDOK/locatieserver) — Documentation

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
