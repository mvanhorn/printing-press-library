# Catasto CLI

**The free, single-binary Italian cadastre CLI — converts between cadastral references and GPS, online or offline, with agent-native JSON.**

Bridges Italian cadastral references (provincia / comune / foglio / particella) and WGS84 coordinates using the Agenzia delle Entrate public ajax endpoint plus the community-maintained ondata Parquet centroids. Produces JSON shapes that map cleanly onto downstream GIS pipelines. No credentials, no Python, no DuckDB — a single Go binary.

Printed by [@robertobissanti](https://github.com/robertobissanti) (Roberto Bissanti).

## Install

The recommended path installs both the `catasto-pp-cli` binary and the `pp-catasto` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install catasto
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install catasto --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install catasto --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install catasto --agent claude-code
npx -y @mvanhorn/printing-press install catasto --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/catasto-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-catasto --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-catasto --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-catasto skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-catasto. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/catasto-current).
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
    "catasto": {
      "command": "catasto-pp-mcp"
    }
  }
}
```

</details>

## Use with Agentic Systems

The CLI is designed to be agent-native: every command supports `--json`, `--select`, `--agent`, `--dry-run`; exit codes are typed; the MCP server (`catasto-pp-mcp`) auto-mirrors every Cobra command as an agent tool with `readOnlyHint` set on the read commands.

Three integration paths, pick whichever fits your stack:

### 1. Claude Desktop / Claude Code (drop-in `.mcpb` bundle)

The fastest path. The bundle compiles the MCP server and packages the manifest in one ZIP.

1. Locate `build/catasto-pp-mcp-darwin-arm64.mcpb` in this repo (or download from the published release).
2. Double-click the file — Claude Desktop opens an install dialog.
3. After install, every command appears as a tool in Claude (`gps`, `cadastral`, `comune`, `validate`, `doctor`, etc.). Read-only commands carry `readOnlyHint: true` so Claude doesn't ask for permission per call.

For Claude Code specifically, the install command above (`npx -y @mvanhorn/printing-press install catasto`) registers both the CLI and the agent skill (`pp-catasto`) — preferred over the raw MCP bundle when you want the SKILL.md prose loaded too.

### 2. Generic MCP host (stdio config)

Any host that speaks MCP over stdio (Cursor, Windsurf, Zed, custom integrations) — point it at the `catasto-pp-mcp` binary. Build the binary once with `go build -o catasto-pp-mcp ./cmd/catasto-pp-mcp`, then add to your host's MCP config:

```json
{
  "mcpServers": {
    "catasto": {
      "command": "/absolute/path/to/catasto-pp-mcp"
    }
  }
}
```

### 3. Codex CLI

Codex supports MCP servers via its config file. Add to `~/.codex/config.toml`:

```toml
[mcp_servers.catasto]
command = "/absolute/path/to/catasto-pp-mcp"
```

Then Codex sessions can call `gps`, `cadastral`, etc. as tools. Check the [current Codex docs](https://github.com/openai/codex) for the latest MCP config syntax — it has been evolving through 2026.

### 4. OpenCode

OpenCode reads MCP config from `~/.config/opencode/config.json`:

```json
{
  "mcp": {
    "catasto": {
      "type": "local",
      "command": ["/absolute/path/to/catasto-pp-mcp"]
    }
  }
}
```

See [opencode.ai docs](https://opencode.ai/) for the current syntax.

### 5. aider (no MCP — shell-out pattern)

aider doesn't speak MCP natively. Two options:

- **Add to aider's `--read` set:** put `SKILL.md` (in this dir) into your aider read-only context. aider will then know about the CLI and call it via shell.
- **One-off invocations:** ask aider to "run `catasto-pp-cli <subcommand> --agent` and use the JSON output." `--agent` enables JSON, compact mode, and disables prompts in one flag, so the output is friction-free for aider's chat → tool flow.

### 6. Any agent — direct shell-out (no MCP, no skill)

For minimal-integration setups, agents can shell out and parse JSON:

| Command | Agent-friendly invocation | Returns |
|---|---|---|
| GPS → cadastral | `catasto-pp-cli gps <lon> <lat> --agent` | Province, comune, foglio, particella |
| Batch GPS | `catasto-pp-cli gps --stdin --agent < points.csv` | One JSON object per row |
| Cadastral → GPS | `catasto-pp-cli cadastral --comune <belfiore-or-name> [--provincia <sigla>] [--cap <code>] --foglio <n> --particella <n> --agent` | WGS84 lat/lon + comune metadata |
| Comune resolver | `catasto-pp-cli comune --belfiore <code> --json` / `--name <nome> --provincia <sigla> --json` / `--cap <code> --json` | Comune metadata (no network) |
| Validate input | `catasto-pp-cli validate --comune <c> --foglio <f> --particella <p> --json` | `{valid: true}` or error report |
| Health check | `catasto-pp-cli doctor --json` | Reachability status |
| Self-describe | `catasto-pp-cli agent-context` | Full command tree + flags as JSON |

Standard exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error. Errors go to stderr; structured output goes to stdout.

Pre-load any agent with `catasto-pp-cli agent-context` to give it the complete command surface — flags, examples, exit codes — in one shot. Useful for agents that need discovery before they can use the CLI productively.

## Standalone use (no agent)

This CLI works as an ordinary command-line tool for human use. The most common workflows:

```bash
# 1. Verify upstream services are reachable.
catasto-pp-cli doctor

# 2. Resolve any Italian point to its cadastral parcel.
catasto-pp-cli gps 12.4924 41.8902

# 3. Look up a parcel's coordinates from its cadastral reference.
catasto-pp-cli cadastral --comune Roma --provincia RM --foglio 508 --particella B

# 4. Resolve comune metadata (no network).
catasto-pp-cli comune --cap 90121 --json | jq '.[].nome'

# 5. Batch reverse-geocode a CSV of coordinates.
cat coords.csv | catasto-pp-cli gps --stdin --json | jq -r '"\(.lon),\(.lat),\(.result.COD_COMUNE),\(.result.FOGLIO),\(.result.NUM_PART)"' > enriched.csv
```

Output defaults to a human-readable table in a terminal; pipe it (or pass `--json`) to get JSON for downstream tools.

## Authentication

No credentials required. Both upstream data sources (Agenzia delle Entrate ajax + ondata Parquet on GitHub) are publicly accessible without registration.

## Quick Start

```bash
# Verify the upstream AdE service is reachable before relying on output.
catasto-pp-cli doctor


# Forward lookup: GPS coordinates → cadastral reference (Roma Colosseum example).
catasto-pp-cli gps 12.4924 41.8902 --json


# Reverse lookup: cadastral reference → centroid coordinates.
catasto-pp-cli cadastral --comune H501 --foglio 508 --particella B --json


# Parse-only syntax check; no API call.
catasto-pp-cli validate --comune H501 --foglio 508 --particella B --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agent-native composability
- **`gps`** — Resolve a WGS84 lon/lat point to its Italian cadastral parcel (province, comune, foglio, particella). Supports single-point and streaming batch mode.

  _Reach for this when an agent has a coordinate (or list of coordinates) and needs to attach Italian cadastral references to it._

  ```bash
  catasto-pp-cli gps 12.4924 41.8902 --json
  ```

### Local state that compounds
- **`cadastral`** — Reverse lookup: given a comune codice belfiore + foglio + particella, return the WGS84 centroid coordinates. Powered by the ondata/dati_catastali Parquet dataset, cached locally on first use.

  _The headline reverse-direction lookup; pair it with `gps` for full round-trip between paper cadastral titles and digital maps._

  ```bash
  catasto-pp-cli cadastral --comune H501 --foglio 508 --particella B --json
  ```

### Field-work ergonomics
- **`validate`** — Parse-only validator for cadastral references. Explains shape rules without hitting any API.

  _Form-style flows and batch imports can short-circuit invalid input before burning an API call; agents use it as a guardrail._

  ```bash
  catasto-pp-cli validate --comune H501 --foglio 508 --particella B --json
  ```

## Usage

Run `catasto-pp-cli --help` for the full command reference and flag list.

## Commands

### lookup

Forward lookup: GPS coordinates to cadastral reference via the Agenzia delle Entrate public ajax endpoint.

- **`catasto-pp-cli lookup`** - Resolve a WGS84 longitude/latitude point to the cadastral parcel it falls inside. Returns province code, comune codice belfiore, comune name, sezione, foglio, and particella identifier.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
catasto-pp-cli lookup --op example-value --lon 42 --lat 42

# JSON for scripting and agents
catasto-pp-cli lookup --op example-value --lon 42 --lat 42 --json

# Filter to specific fields
catasto-pp-cli lookup --op example-value --lon 42 --lat 42 --json --select id,name,status

# Dry run — show the request without sending
catasto-pp-cli lookup --op example-value --lon 42 --lat 42 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
catasto-pp-cli lookup --op example-value --lon 42 --lat 42 --agent
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
catasto-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/catasto-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Lookup for a Trentino-Alto-Adige comune returns 'comune not in ondata index'** — TAA runs an autonomous cadastral system separate from AdE; no public dataset exists. Use the local TAA cadastral portals (catasto.provincia.tn.it / catasto.bz.it).
- **AdE ajax returns TIPOLOGIA: STRADA instead of a parcel shape** — The point fell on a street, not a parcel. Move the input coordinate slightly into the property area, or accept the street result if that is what you want.
- **AdE returns empty response** — Check coordinates are inside Italy (lon 6.6–18.5, lat 35.5–47.1). AdE returns nothing — not an error — for points outside cadastral coverage. Pass `--strict` to convert empty results into an exit-3 NotFound.
- **First `cadastral` call is slow** — On first use the relevant region's Parquet file (a few MB up to ~50MB) is downloaded and cached under the OS user-cache dir. Subsequent calls in the same region are instant.

---

## Cookbook

Worked recipes for common tasks.

### Round-trip: confirm a point matches its cadastral title

```bash
# Forward: take coordinates, get cadastral ref
catasto-pp-cli gps 12.4924 41.8902 --json --select COD_COMUNE,FOGLIO,NUM_PART
# {"COD_COMUNE":"H501","FOGLIO":"508","NUM_PART":"B"}

# Reverse: take the cadastral ref, get coordinates back
catasto-pp-cli cadastral --comune H501 --foglio 508 --particella B --json --select lat,lon
# {"lat":41.890252,"lon":12.492405}
```

### Resolve a Roman address from a CAP

```bash
catasto-pp-cli comune --cap 00184 --json --select nome,codice_belfiore,provincia_sigla
# [{"nome":"Roma","codice_belfiore":"H501","provincia_sigla":"RM"}]
```

### Batch reverse-geocode a CSV

```bash
# coords.csv: each row is "lon,lat"
cat coords.csv | catasto-pp-cli gps --stdin --json --select result.COD_COMUNE,result.FOGLIO,result.NUM_PART
```

### Disambiguate a shared name

Italy has 7 cases of comuni with the same name (e.g., Castro in BG and LE). Add `--provincia`:

```bash
catasto-pp-cli cadastral --comune Castro --provincia BG --foglio 1 --particella 1 --json
# resolves to C337 (Castro, Bergamo)
```

Or use the standalone resolver to see candidates first:

```bash
catasto-pp-cli comune --name Castro
# Error: multiple comuni match: name="Castro" → 2 candidates: Castro (BG, C337); Castro (LE, M261)
```

### Pre-flight validate before importing a spreadsheet

```bash
catasto-pp-cli validate --comune H501 --foglio 508 --particella B --json
# {"valid":true,"comune":"H501",...}
# exit 0

catasto-pp-cli validate --comune ROMA --foglio abc --particella ""
# {"valid":false,"errors":["foglio \"abc\" is not numeric","particella is required"]}
# exit 2
```

### Diagnostic errors when a parcel isn't found

The CLI distinguishes three failure modes so you can tell input typos from data gaps:

```bash
# Unknown comune
catasto-pp-cli cadastral --comune ZZZZ --foglio 1 --particella 1
# Error: comune ZZZZ has 0 rows in 12_Lazio.parquet (check the codice belfiore)

# Wrong foglio
catasto-pp-cli cadastral --comune H501 --foglio 9999 --particella 1
# Error: comune=H501 has N distinct foglios but none match foglio=9999; nearest existing foglio is ...

# Wrong particella (right comune+foglio)
catasto-pp-cli cadastral --comune G273 --foglio 35 --particella 1900
# Error: comune=G273 foglio=35 exists with 1530 parcels, but particella=1900 is not among them (nearest: ...)
```

The last form is especially useful: if the CLI says "foglio exists with N parcels," then the parcel you typed genuinely doesn't exist in the ondata snapshot — likely because the snapshot is older than the parcel split/renumbering, or because the parcel uses a sezione the ondata dataset doesn't expose.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**ondata/dati_catastali**](https://github.com/ondata/dati_catastali) — Python
- [**pigreco/workshop-estate-gis-2021**](https://github.com/pigreco/workshop-estate-gis-2021) — Python
- [**enricofer/catasto**](https://github.com/enricofer/catasto) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
