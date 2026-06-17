---
name: pp-nhc
description: "Credible, real-time National Hurricane Center data for AI agents: active storms, parsed advisories Trigger phrases: `is there a hurricane`, `active tropical storms`, `tropical weather outlook`, `hurricane advisory`, `what's in the tropics`, `use nhc`, `run nhc`."
author: "Abe Diaz (@abe238)"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - nhc-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/nhc/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# National Hurricane Center — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `nhc-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install nhc --cli-only
   ```
2. Verify: `nhc-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI gives agents the most credible real-time hurricane information straight from the National Hurricane Center and the National Weather Service: active storms (CurrentStorms.json), parsed Public Advisories and Forecast Discussions, the Tropical Weather Outlook with formation odds, and live tropical watches and warnings. It is built text-first and links out to the official GIS for anyone who wants polygons. Deep thanks to the forecasters, hurricane hunters, and support staff at NHC who sacrifice so much for the safety of so many. This is an unofficial tool; in an emergency, follow the official watches, warnings, and evacuation orders from NHC, the NWS, and your local authorities.

## When to Use This CLI

Use nhc-pp-cli when an AI agent or operator needs trustworthy, current tropical-cyclone information: whether a storm is active, its forecast and intensity, the meteorologist's discussion, what may develop, and which official watches and warnings are in effect. It is ideal for situational-awareness assistants, briefing generators, and disaster-response tooling that must cite an authoritative source.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to make or replace official forecasts or evacuation decisions; defer to NHC, the NWS, and local authorities.
- Do not use it for non-tropical or general weather; it covers tropical cyclones only.
- Do not expect rendered GIS polygons; it links out to the ArcGIS MapServer instead of parsing it.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### One-call situational awareness
- **`brief`** — One command returns active storms, the tropical outlook, and live NWS tropical alerts as a single payload, with an optional human briefing.

  _Reach for this first when an agent asks 'what is the tropical situation right now' — one call instead of four._

  ```bash
  nhc-pp-cli brief --basin atl --markdown
  ```

### Parsed NHC products
- **`storm`** — Full detail for one storm: vitals plus every advisory-product URL and GIS link-out, with not-applicable fields returned as explicit null.

  _Use when you have a storm id and need its products and graphics without parsing the raw feed._

  ```bash
  nhc-pp-cli storm al092024
  ```
- **`advisory`** — Fetches and parses the Public Advisory, Forecast Discussion, or Forecast/Marine Advisory into structured fields plus the clean body text, with no HTML scraping by the agent.

  _Use when you need the meteorologist's reasoning or exact wind/pressure/movement fields, not a web page._

  ```bash
  nhc-pp-cli advisory al092024 --type tcd
  ```
- **`outlook`** — Parses the Tropical Weather Outlook for any basin into development areas and 48-hour / 7-day formation chances, with graphic link-outs.

  _Use in the quiet season or to forecast-ahead; it is useful when 'storms list' is empty._

  ```bash
  nhc-pp-cli outlook --basin atl
  ```

### Link-outs done right
- **`graphics`** — Returns the cone, track, surge, and wind link-outs for a storm; --download saves the files locally and --open views them.

  _Use to hand a dashboard or person the official NHC graphics without re-deriving file paths._

  ```bash
  nhc-pp-cli graphics al092024 --kind cone,surge
  ```
- **`gis`** — Maps a storm to its ArcGIS REST layer URLs (forecast cone, wind field, watch/warning) for mapping clients. Link-out only; never ingested.

  _Use when a caller wants the spatial layers; this CLI references them instead of parsing them._

  ```bash
  nhc-pp-cli gis al092024
  ```

### Gratitude and safety
- **`credits`** — Thanks the people of the National Hurricane Center and states plainly that this is an unofficial tool and NHC/NWS is authoritative.

  _Read or surface this so users know the source of truth and who to thank._

  ```bash
  nhc-pp-cli credits
  ```

## Command Reference

**alerts** — Active NWS tropical watches and warnings (api.weather.gov)

- `nhc-pp-cli alerts` — Active tropical alerts. Pass a comma-separated --event list (Hurricane/Tropical Storm/Storm Surge x Warning/Watch).

**storms** — Active tropical cyclones from NHC CurrentStorms.json (all basins)

- `nhc-pp-cli storms` — List every active tropical cyclone across all basins.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
nhc-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Active storms as compact JSON for an agent

```bash
nhc-pp-cli storms list --json --select id,name,classification,intensity,pressure
```

Returns just the high-gravity fields so an agent does not burn context on the full feed.

### Read the forecast discussion

```bash
nhc-pp-cli advisory al092024 --type tcd
```

Fetches and parses the meteorologist's narrative for a storm id.

### One-call situational briefing

```bash
nhc-pp-cli brief --basin atl --markdown
```

Bundles active storms, the outlook, and tropical alerts into a single human-readable briefing.

### Official cone and surge graphics for a storm

```bash
nhc-pp-cli graphics al092024 --kind cone,surge
```

Hands back the official NHC link-outs without re-deriving file paths.

## Auth Setup

No authentication required.

Run `nhc-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  nhc-pp-cli storms --agent --select id,name,status
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
nhc-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
nhc-pp-cli feedback --stdin < notes.txt
nhc-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/nhc-pp-cli/feedback.jsonl`. They are never POSTed unless `NHC_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NHC_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
nhc-pp-cli profile save briefing --json
nhc-pp-cli --profile briefing storms
nhc-pp-cli profile list --json
nhc-pp-cli profile show briefing
nhc-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `nhc-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add nhc-pp-mcp -- nhc-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which nhc-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   nhc-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `nhc-pp-cli <command> --help`.
