---
name: pp-catasto
description: "The free, single-binary Italian cadastre CLI — converts between cadastral references and GPS, online or offline,... Trigger phrases: `convert cadastral to GPS`, `Italian cadastre lookup`, `particella to coordinates`, `comune foglio particella`, `reverse geocode Italian parcel`, `use catasto`, `run catasto`."
author: "Roberto Bissanti"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - catasto-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/catasto/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Catasto — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `catasto-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install catasto --cli-only
   ```
2. Verify: `catasto-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/catasto/cmd/catasto-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Bridges Italian cadastral references (provincia / comune / foglio / particella) and WGS84 coordinates using the Agenzia delle Entrate public ajax endpoint plus the community-maintained ondata Parquet centroids. Produces JSON shapes that map cleanly onto downstream GIS pipelines. No credentials, no Python, no DuckDB — a single Go binary.

## When to Use This CLI

Reach for this CLI whenever an agent needs to bridge Italian cadastral identifiers and geographic coordinates — real-estate due diligence, surveying preparation, GIS data enrichment, batch reverse-geocoding of address lists. It's the right tool when the work is offline-friendly, scriptable, or needs to be embedded in field laptops without Python/QGIS/DuckDB dependencies.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**lookup** — Forward lookup: GPS coordinates to cadastral reference via the Agenzia delle Entrate public ajax endpoint.

- `catasto-pp-cli lookup` — Resolve a WGS84 longitude/latitude point to the cadastral parcel it falls inside. Returns province code, comune...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
catasto-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Resolve a coordinate to a cadastral parcel

```bash
catasto-pp-cli gps 12.4924 41.8902 --json --select SIGLA_PROV,COD_COMUNE,FOGLIO,NUM_PART
```

Forward lookup for a single point with narrowed JSON output for agent consumption.

### Batch reverse-geocode coordinates from stdin

```bash
catasto-pp-cli gps --stdin --json
```

Stream lon,lat pairs (one per line, CSV-compatible) and emit one JSON object per row with the input echoed alongside the result. Points outside Italy are flagged inline rather than aborting the stream.

### Reverse-lookup a cadastral reference

<!-- PATCH: Make the Comune/Foglio/Particella -> GPS workflow explicit for agents. -->
```bash
catasto-pp-cli cadastral --comune M011 --foglio 2 --particella 2 --agent
```

Geolocalizza una particella catastale da Comune + Foglio + Particella, restituendo coordinate GPS WGS84 (`lon`, `lat`) del centroide. `--comune` accetta sia codice Belfiore sia nome umano; quando usi il nome, aggiungi `--provincia` se il comune e' ambiguo:

```bash
catasto-pp-cli cadastral --comune "Roma" --provincia RM --foglio 508 --particella B --agent
```

Resolves codice belfiore/name + foglio + particella to a WGS84 centroid via the ondata Parquet dataset. First call in a region downloads and caches the file.

### Validate references before importing a spreadsheet

```bash
catasto-pp-cli validate --comune H501 --foglio 508 --particella B --json
```

Free, deterministic syntax check. Exits 2 on invalid input with a JSON report of every problem.

## Auth Setup

No authentication required.

Run `catasto-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  catasto-pp-cli lookup --op example-value --lon 42 --lat 42 --agent --select id,name,status
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
catasto-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
catasto-pp-cli feedback --stdin < notes.txt
catasto-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.catasto-pp-cli/feedback.jsonl`. They are never POSTed unless `CATASTO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CATASTO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
catasto-pp-cli profile save briefing --json
catasto-pp-cli --profile briefing lookup --op example-value --lon 42 --lat 42
catasto-pp-cli profile list --json
catasto-pp-cli profile show briefing
catasto-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `catasto-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add catasto-pp-mcp -- catasto-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which catasto-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   catasto-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `catasto-pp-cli <command> --help`.
