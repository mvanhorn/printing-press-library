---
name: pp-pdok-location
description: "One Go binary for every Dutch location workflow — geocode, reverse-geocode, batch CSVs, full GeoJSON geometries,... Trigger phrases: `geocode a Dutch address`, `find the gemeente for this lat/lon`, `convert RD coordinates to WGS84`, `look up a Dutch postcode`, `batch geocode a CSV of Dutch addresses`, `find the nearest address to this point in the Netherlands`, `use pdok-location`, `run pdok-location`."
author: "markvandeven"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pdok-location-pp-cli
---

# PDOK Location — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `pdok-location-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install pdok-location --cli-only
   ```
2. Verify: `pdok-location-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/pdok-location/cmd/pdok-location-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

PDOK Location is the right CLI when an agent needs to work with Dutch addresses, parcels, roads, or administrative boundaries — including batch geocoding CSVs, looking up the gemeente containing a point, converting between RD and WGS84 coordinates, or pulling full GeoJSON features inside a bounding box. It is Netherlands-only (do not use for non-Dutch addresses) and free (no auth, no key).

### Why this CLI bundles two PDOK services

This binary wraps two upstreams — **Locatieserver** (`/bzk/locatieserver/search/v3_1`) and **Kadaster Location API** (`/kadaster/location-api/v1`) — because they are complementary, not redundant.

- **Locatieserver** (Solr-based) is the primary text-geocoding surface: free-text matches, suggest→lookup chains, reverse geocoding by lat/lon or RD, ranked Solr scores. Returns records with centroid coordinates only.
- **Kadaster Location API** (OGC API Features) is the secondary geometry surface: full GeoJSON, bbox filters, multi-CRS output (WGS84, RD/EPSG:28992, Web Mercator, ETRS89), 14 collections.

A common Dutch geo workflow uses both: geocode text on Locatieserver, then pull full geometry on the Location API. Each is incomplete without the other (Locatieserver lacks bbox and full geometries; Location API lacks Solr ranking and reverse geocoding), so bundling them under one binary, one config, and one local cache turns that workflow into a single tool. Run `pdok-location-pp-cli doctor --json` to see which of the two upstreams is reachable right now — the `sources.locatieserver` and `sources.kadaster_location_api` keys each report status + which commands are affected if the source is down.

## When Not to Use This CLI

- **Do not use for non-Dutch addresses.** PDOK only covers the Netherlands; passing a US, UK, German, Belgian, or other non-Dutch address returns either an empty result or a low-score false positive. Pick a different geocoder for those.
- **Do not use for mutating remote state.** This printed CLI exposes read-only commands only — no creating, updating, deleting, publishing, commenting, ordering, booking, or other state-changing operations.

## Unique Capabilities

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

## Command Reference

**collections** — Manage collections

- `pdok-location-pp-cli collections get` — A list of all collections (geospatial data resources) in this dataset.
- `pdok-location-pp-cli collections get-adres` — adres collection (geospatial data resource) in this dataset.
- `pdok-location-pp-cli collections get-functioneel-gebied` — functioneel_gebied collection (geospatial data resource) in this dataset.
- `pdok-location-pp-cli collections get-gebouw` — gebouw collection (geospatial data resource) in this dataset.
- `pdok-location-pp-cli collections get-gemeentegebied` — gemeentegebied collection (geospatial data resource) in this dataset.
- `pdok-location-pp-cli collections get-geografisch-gebied` — geografisch_gebied collection (geospatial data resource) in this dataset.
- `pdok-location-pp-cli collections get-inrichtingselement` — inrichtingselement collection (geospatial data resource) in this dataset.
- `pdok-location-pp-cli collections get-perceel` — perceel collection (geospatial data resource) in this dataset.
- `pdok-location-pp-cli collections get-plaats` — plaats collection (geospatial data resource) in this dataset.
- `pdok-location-pp-cli collections get-provinciegebied` — provinciegebied collection (geospatial data resource) in this dataset.
- `pdok-location-pp-cli collections get-spoorbaandeel` — spoorbaandeel collection (geospatial data resource) in this dataset.
- `pdok-location-pp-cli collections get-waterdeel` — waterdeel collection (geospatial data resource) in this dataset.
- `pdok-location-pp-cli collections get-wegdeel` — wegdeel collection (geospatial data resource) in this dataset.
- `pdok-location-pp-cli collections get-woonplaats` — woonplaats collection (geospatial data resource) in this dataset.

**conformance** — Manage conformance

- `pdok-location-pp-cli conformance` — A list of all conformance classes specified in a standard that the server conforms to.

**free** — Manage free

- `pdok-location-pp-cli free` — De Free API biedt de mogelijkheid om vrij te zoeken (klassiek geocoderen), waar zonder tussenkomst van suggesties de...

**lookup** — Manage lookup

- `pdok-location-pp-cli lookup` — Zodra er op basis van suggesties van de Suggest API een keuze is gemaakt, wordt de Lookup API aangeroepen, welke...

**pdok-location-api** — Manage pdok location api

- `pdok-location-pp-cli pdok-location-api` — This document

**pdok-location-search** — Manage pdok location search

- `pdok-location-pp-cli pdok-location-search` — This endpoint allows one to implement autocomplete functionality for location search. The `q` parameter accepts a...

**reverse** — Manage reverse

- `pdok-location-pp-cli reverse` — De Reverse API biedt de mogelijkheid om een locatie (punt geometrie) op te geven om vervolgens verschillende...

**suggest** — Manage suggest

- `pdok-location-pp-cli suggest` — De Suggest API biedt de mogelijkheid om een (gedeelte van een) zoekopdracht op te voeren, waarnaar er suggesties...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pdok-location-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Bulk geocode a CSV of customer addresses

```bash
pdok-location-pp-cli batch geocode customers.csv --address-col street --out customers-geocoded.csv
```

Geocode every row, write a new CSV with lat/lon, RD x/y, score, and an error column. Re-runs are free because successful matches are cached.

### Resolve a partial address to its canonical match with geometry

```bash
pdok-location-pp-cli resolve 'Damrak Amsterdam' --geojson
```

One command that does the suggest→lookup chain and returns GeoJSON ready to drop into a map.

### Trim a verbose reverse-geocode for agent context (--agent + --select)

```bash
pdok-location-pp-cli nearest --lat 52.3731 --lon 4.8922 --agent --select adres.weergavenaam,adres.id,perceel.weergavenaam,gemeente.gemeentenaam,gemeente.provincienaam
```

Pair --agent with --select dotted paths so the agent gets only the address line, parcel description, and admin context — not the full per-source response.

### List matching parcels inside a study bbox

```bash
pdok-location-pp-cli features search --query amsterdam --collections perceel --bbox 4.85,52.36,4.92,52.40 --csv
```

Pulls every parcel matching 'amsterdam' inside the bbox from the Location API and emits a flat CSV; swap --collections to adres, gebouw, or any other OGC collection.

### Convert Dutch RD coordinates to WGS84 with no API call

```bash
pdok-location-pp-cli convert rd-to-ll 121200 488000 --json
```

Pure-math transform, runs offline, deterministic — useful for normalizing legacy datasets stored in RD.

## Auth Setup

No authentication required.

Run `pdok-location-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  pdok-location-pp-cli collections get --agent --select id,name,status
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
pdok-location-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pdok-location-pp-cli feedback --stdin < notes.txt
pdok-location-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.pdok-location-pp-cli/feedback.jsonl`. They are never POSTed unless `PDOK_LOCATION_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PDOK_LOCATION_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
pdok-location-pp-cli profile save briefing --json
pdok-location-pp-cli --profile briefing collections get
pdok-location-pp-cli profile list --json
pdok-location-pp-cli profile show briefing
pdok-location-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `pdok-location-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add pdok-location-pp-mcp -- pdok-location-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pdok-location-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   pdok-location-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `pdok-location-pp-cli <command> --help`.
