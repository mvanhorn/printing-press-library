# Overpass CLI

**Find things worth photographing by name instead of by OpenStreetMap tag, with automatic failover across Overpass mirrors.**

Overpass is a query language wearing a REST costume, and its public mirrors fall over independently under load. This maps photographic vocabulary — water towers, lighthouses, brutalist buildings, piers, observatories — onto the right tag combinations, builds valid Overpass QL, retries across mirrors, caches results locally, and exports GeoJSON.

Learn more at [Overpass](https://wiki.openstreetmap.org/wiki/Overpass_API).

Created by [@justinwfu](https://github.com/justinwfu) (justinwfu).

## Install

The recommended path installs both the `overpass-pp-cli` binary and the `pp-overpass` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install overpass
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install overpass --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install overpass --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install overpass --agent claude-code
npx -y @mvanhorn/printing-press-library install overpass --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/overpass-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install overpass --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-overpass --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-overpass --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install overpass --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/overpass-current).
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
    "overpass": {
      "command": "overpass-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No credential at all. Both Overpass and Nominatim are keyless; they only require a descriptive User-Agent, which this CLI sends. Nominatim permits about one request per second and Overpass allocates a small number of concurrent slots per mirror, so the CLI paces itself and fails over rather than hammering one host.

## Quick Start

```bash
# See what subjects can be found before searching for one.
overpass-pp-cli types

# The core query: subjects by name near a place.
overpass-pp-cli near --at "Los Angeles" --type water_tower --radius 25km

# Check which Overpass instances are healthy if a query is slow.
overpass-pp-cli mirrors

# Put the results on a map.
overpass-pp-cli geojson --at "Los Angeles" --type viewpoint --radius 10km --out spots.geojson

```

## Known Gaps

- **Bare `overpass-pp-cli sync` is a no-op.** Overpass is a query interpreter, not a collection API: every endpoint needs a caller-supplied query, so there is no default set of resources to pull down. `sync` says so on stderr and exits 0 with `total_records: 0`. To actually populate the local store, name the resource and its required params:

  ```bash
  overpass-pp-cli sync --resources geocoding \
    --resource-param geocoding:q="San Pedro, CA" \
    --resource-param geocoding:format=json
  ```

  Both params are required. Without `format=json` Nominatim answers 200 with a non-JSON body and the sync records `non_json_200_body` with zero rows — it does not apply the spec's `format` default.

- **`overpass-pp-cli export geocoding` does not work.** `/search` lives on Nominatim, but the export path map is contract-checked against `tools-manifest.json`, which records the path against the Overpass base URL — so the request goes to `https://overpass-api.de/search` and comes back 404 (exit 3). Use `sync --resources geocoding ...` as above and then `search --data-source local --json`, or call `geocoding --q ... --json` directly.

- **`near`, `route`, and `geojson` do not write their results to the local store.** They read live and fail over across mirrors on every call. So `search --data-source local` and `export` see only rows put there by an explicit `sync --resources ...` — not the subjects a previous `near` returned.

- **The response cache does not cover `near`, `route`, or `geojson`.** Those commands POST to Overpass through their own failover runner rather than the generated HTTP client, so nothing lands in the cache directory and `--no-cache` has nothing to bypass on this path. An identical repeated query re-hits a rate-limiting upstream at full cost. Only the `geocoding` and `overpass` endpoint commands use the cached client.

- **A truncated answer is reported inside the document, never ahead of it.** When Overpass hits its own server-side timeout or memory ceiling it returns usable elements *plus* a remark, and the result undercounts reality. `near --json` and `route --json` carry `"partial": true` and `"partial_remark": "<Overpass's own text>"`; `geojson` carries the same two keys as FeatureCollection foreign members, so a saved `.geojson` still says so when it is reopened. A human-readable line goes to **stderr** in every mode, and to stdout only when stdout is already prose. Nothing is printed ahead of a JSON or GeoJSON document — that would make it unparseable for exactly the responses most worth reading.

- **`near` and `route` regularly take more than 10 seconds.** Each one geocodes through Nominatim and then runs an Overpass query that may fail over across mirrors. Raise `--timeout` rather than assuming a hang, and run `overpass-pp-cli mirrors` when it feels slower than usual.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Subjects without the query language
- **`near`** — Finds things worth photographing near a place — water towers, lighthouses, brutalist buildings, piers, viewpoints — by name rather than by OpenStreetMap tag.

  _Reach for this whenever someone wants to know what is worth shooting around a place, without needing to know OpenStreetMap tagging._

  ```bash
  overpass-pp-cli near --at "Los Angeles" --type water_tower --radius 25km
  ```
- **`types`** — Lists every subject type the CLI knows how to find, with the OpenStreetMap tags behind each one.

  _Reach for this to see what can be searched before running near, or to check what tags a subject type actually resolves to._

  ```bash
  overpass-pp-cli types --group architecture
  ```
- **`route`** — Finds subjects inside a corridor between two places, so a drive can be planned around what is worth stopping for.

  _Reach for this when planning a drive or day trip and the question is what to stop for on the way._

  ```bash
  overpass-pp-cli route --from "Los Angeles" --to "Salton Sea" --type water_tower --corridor 15km
  ```
- **`geojson`** — Writes results as GeoJSON for opening in a map, a Leaflet page, or any GIS tool.

  _Reach for this when the result needs to end up on a map rather than in a terminal._

  ```bash
  overpass-pp-cli geojson --at "Los Angeles" --type viewpoint --radius 10km
  ```

### Surviving a flaky public API
- **`mirrors`** — Shows which Overpass mirrors are currently answering and how many rate-limit slots each has free.

  _Reach for this when queries are failing or slow, before assuming the query itself is wrong._

  ```bash
  overpass-pp-cli mirrors
  ```

## Recipes

### What is worth shooting near here

```bash
overpass-pp-cli near --at "Los Angeles" --type water_tower --radius 25km
```

Maps the subject name to OSM tags, builds the query, and retries across mirrors.

### See the tags behind a subject

```bash
overpass-pp-cli types --group industrial
```

Shows exactly which OpenStreetMap tags each subject type resolves to, so the search is inspectable.

### Plan a drive around subjects

```bash
overpass-pp-cli route --from "Los Angeles" --to "Palm Springs" --type water_tower --corridor 12km
```

Builds a corridor between the two points and finds subjects inside it.

### Narrow a verbose result payload

```bash
overpass-pp-cli near --at "Los Angeles" --type viewpoint --radius 20km --agent --select subjects.name,subjects.latitude,subjects.longitude
```

Overpass elements carry every OSM tag; --select with dotted paths returns only the fields needed.

### Export for a map

```bash
overpass-pp-cli geojson --at "Los Angeles" --type viewpoint --radius 10km --out spots.geojson
```

Converts Overpass elements, whose coordinates live in different fields for nodes and ways, into standard GeoJSON.

## Usage

Run `overpass-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `OVERPASS_CONFIG_DIR`, `OVERPASS_DATA_DIR`, `OVERPASS_STATE_DIR`, or `OVERPASS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `OVERPASS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export OVERPASS_HOME=/srv/overpass
overpass-pp-cli doctor
```

Under `OVERPASS_HOME=/srv/overpass`, the four dirs resolve to `/srv/overpass/config`, `/srv/overpass/data`, `/srv/overpass/state`, and `/srv/overpass/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "overpass": {
      "command": "overpass-pp-mcp",
      "env": {
        "OVERPASS_HOME": "/srv/overpass"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `OVERPASS_DATA_DIR` overrides an explicit `--home` for that kind. Use `OVERPASS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `OVERPASS_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `overpass-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### geocoding

Place names to coordinates

- **`overpass-pp-cli geocoding`** - Resolve a place name to coordinates via Nominatim, optionally restricted by country. `near` and `route` geocode internally; reach for this when the coordinates themselves are the answer.

### overpass

Raw access to a single Overpass mirror

- **`overpass-pp-cli overpass query`** - Run a raw Overpass QL program against OpenStreetMap and return the matching elements. The escape hatch for queries the curated taxonomy does not cover; prefer `near` for ordinary use, since it builds correct QL, fails over across mirrors, and caches results.
- **`overpass-pp-cli overpass status`** - Show one Overpass mirror's rate-limit slots and running queries. Use `mirrors` instead when the question is which host to send a query to.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`overpass-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`overpass-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`overpass-pp-cli learnings list`** - Inspect taught rows
- **`overpass-pp-cli learnings forget <query>`** - Undo a teach
- **`overpass-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`overpass-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`overpass-pp-cli teach-pattern`** - Install a query/resource template up front
- **`overpass-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `OVERPASS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `overpass-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
overpass-pp-cli geocoding --q "San Pedro, CA"

# JSON for scripting and agents
overpass-pp-cli geocoding --q "San Pedro, CA" --json

# Filter to specific fields
overpass-pp-cli geocoding --q "San Pedro, CA" --json --select id,name,status

# Dry run — show the request without sending
overpass-pp-cli geocoding --q "San Pedro, CA" --dry-run

# Agent mode — JSON + compact + no prompts in one flag
overpass-pp-cli geocoding --q "San Pedro, CA" --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - `search --data-source local` and `export` read the local SQLite store, which an explicit `sync --resources ...` populates (see Known Gaps: bare `sync` is a no-op for this API)
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
overpass-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `overpass-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/overpass-pp-cli/config.toml`; `--home`, `OVERPASS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **runtime error: Dispatcher_Client::request_read_and_idx::timeout** — That mirror is overloaded, not your query. The CLI already retries across mirrors; run `overpass-pp-cli mirrors` to see which are healthy, or raise --timeout.
- **No results for a subject you know exists** — OpenStreetMap coverage is uneven and volunteer-contributed. Run `overpass-pp-cli types` to see the exact tags being queried, and widen --radius.
- **Geocoding a place name returns the wrong country** — Nominatim orders by relevance worldwide. Add a region to the query ("San Pedro, CA") or pass --country us.
- **Queries are slow** — Overpass allocates a couple of concurrent slots per mirror and large radii are expensive. Narrow --radius, or let the local cache serve a repeat query.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**overpass-turbo**](https://github.com/tyrasd/overpass-turbo) — JavaScript
- [**overpy**](https://github.com/DinoTools/python-overpy) — Python
- [**osmnx**](https://github.com/gboeing/osmnx) — Python
- [**overpass-api-python-wrapper**](https://github.com/mvexel/overpass-api-python-wrapper) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
