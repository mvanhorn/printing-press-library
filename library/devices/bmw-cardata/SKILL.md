---
name: pp-bmw-cardata
description: "Your BMW's telemetry, on the command line — device-code login, live telematic snapshots, charging history Trigger phrases: `check my BMW state of charge`, `BMW charging history`, `list my BMW vehicles`, `BMW telematic data`, `my BMW's range`, `use bmw-cardata`, `run bmw-cardata`."
author: "jvm"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - bmw-cardata-pp-cli
    install:
      - kind: go
        bins: [bmw-cardata-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/cmd/bmw-cardata-pp-cli
---

# BMW CarData — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `bmw-cardata-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install bmw-cardata --cli-only
   ```
2. Verify: `bmw-cardata-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/cmd/bmw-cardata-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

BMW CarData is the official, free-for-personal-use API for your own vehicle data. This CLI handles the OAuth device-code onboarding, fetches live telematic snapshots, charging history and tyre diagnosis, then keeps it all in SQLite so you can chart SoC trends (soc-trends), reconcile charging cost against your tariff (charging-cost), and see every vehicle at once (fleet status).

## When to Use This CLI

Use this CLI when you want programmatic, scriptable, or agent-driven access to your own BMW/MINI/Rolls-Royce/Supra vehicle telemetry over the official CarData API — fetching live snapshots, syncing charging history, or analysing trends across time and vehicles. It is ideal for Home Assistant users, data-sovereignty tinkerers, and anyone who wants their car's data in SQLite rather than trapped in an app.

## Anti-triggers

Do not use this CLI for:
- Do not use for remote vehicle commands (lock/unlock, honk, climate) — CarData is read-only telemetry; use the My BMW app or ConnectedDrive remote services.
- Do not use outside supported EU markets or without an active ConnectedDrive contract + SIM; the API will return 403.
- Do not use as a real-time high-frequency feed without checking `quota` first; the ~50 calls/day cap resets at midnight UTC.
- Do not use the B2B / partner CarData API path from this CLI; this targets the free personal customer API only.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local telemetry that compounds
- **`soc-trends`** — See your battery state-of-charge and derived range as a time series you can chart.

  _Pick this when an agent needs SoC/range history or trend direction instead of a single live snapshot._

  ```bash
  bmw-cardata-pp-cli soc-trends WBAJB3105JUV12345 --window 30d --agent
  ```
- **`battery-health`** — Track battery capacity degradation by comparing observed max energy against the nameplate capacity.

  _Pick this when an agent must report long-term battery degradation rather than current state of charge._

  ```bash
  bmw-cardata-pp-cli battery-health WBAJB3105JUV12345 --agent
  ```
- **`vehicles diff`** — Show exactly what changed between two telematic snapshots (windows, charging, locks, location).

  _Pick this when an agent needs discrete state changes (did charging start? did a window open?) not a numeric level._

  ```bash
  bmw-cardata-pp-cli vehicles diff WBAJB3105JUV12345 --since 24h
  ```
- **`trips`** — Reconstruct trips (start, end, elapsed, distance) from your vehicle's location breadcrumbs.

  _Pick this when an agent needs driven trips and distances rather than raw GPS points._

  ```bash
  bmw-cardata-pp-cli trips WBAJB3105JUV12345 --since 7d
  ```

### Cost & fleet insight
- **`charging-cost`** — Reconcile your charging sessions against your electricity tariff and see DC charge efficiency.

  _Pick this when an agent must turn raw charging sessions into money spent and charging efficiency._

  ```bash
  bmw-cardata-pp-cli charging-cost WBAJB3105JUV12345 --tariff 0.32 --agent
  ```
- **`fleet status`** — One table of current SoC, range, charging state and last location across every vehicle on your account.

  _Pick this when an agent needs a single multi-vehicle overview instead of calling each VIN separately._

  ```bash
  bmw-cardata-pp-cli fleet status --agent
  ```

### Self-service plumbing
- **`quota`** — See how many of your ~50 daily API calls you have spent and when the budget resets (midnight UTC).

  _Pick this before any sync or polling loop so an agent does not burn the daily quota and start failing._

  ```bash
  bmw-cardata-pp-cli quota
  ```
- **`descriptors search`** — Search the VSS telematic descriptor catalogue (e.g. batteryManagement, charging, navigation) to build containers.

  _Pick this when an agent needs to discover the exact VSS descriptor paths to put in a container._

  ```bash
  bmw-cardata-pp-cli descriptors search "batteryManagement"
  ```

## Command Reference

**customers** — Manage customers

- `bmw-cardata-pp-cli customers create-container` — Please note that some keys are bound to dedicated endpoints.
- `bmw-cardata-pp-cli customers delete-container` — Delete container by ID
- `bmw-cardata-pp-cli customers get-basic-data` — Request basic vehicle data for the specified vehicle.
- `bmw-cardata-pp-cli customers get-charging-history` — Request the vehicle's Charging History sessions.
- `bmw-cardata-pp-cli customers get-container-details` — Please note that some keys are bound to dedicated endpoints.
- `bmw-cardata-pp-cli customers get-image` — Request a vehicle image for the specified vehicle.
- `bmw-cardata-pp-cli customers get-location-based-charging-settings` — Request Vehicle's location based charging settings
- `bmw-cardata-pp-cli customers get-mappings` — Request mapped vehicles.
- `bmw-cardata-pp-cli customers get-smart-maintenance-tyre-diagnosis` — Request a vehicle Smart Maintenance Tyre Diagnosis for the specified vehicle.
- `bmw-cardata-pp-cli customers get-telematic-data` — Please note that some keys are bound to dedicated endpoints.
- `bmw-cardata-pp-cli customers list-containers` — List the containers


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
bmw-cardata-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Narrow a deeply nested telematic payload for an agent

```bash
bmw-cardata-pp-cli customers get-telematic-data WBAJB3105JUV12345 --container-id 11111111-1111-1111-1111-111111111111 --agent --select telematicData.vehicle.drivetrain.batteryManagement,telematicData.vehicle.drivetrain.electricEngine.charging
```

telematicData returns every descriptor as a nested {value,unit,timestamp} map; --select trims it to just the battery and charging keys so an agent does not parse tens of KB.

### Reconcile last month's charging against your tariff

```bash
bmw-cardata-pp-cli charging-cost WBAJB3105JUV12345 --tariff 0.32 --agent
```

Joins synced charging sessions with a per-kWh price locally to report money spent and DC efficiency.

### One-glance multi-car status

```bash
bmw-cardata-pp-cli fleet status --agent
```

Cross-joins every mapped VIN with its latest telematic snapshot for SoC, range, charging state and location.

### Watch today's API budget before syncing

```bash
bmw-cardata-pp-cli quota
```

Reads the local call counter against the ~50/day cap and reports the midnight-UTC reset time.

### Build a container from searched descriptors

```bash
bmw-cardata-pp-cli customers create-container --technical-descriptors $(bmw-cardata-pp-cli descriptors search batteryManagement --json | jq -r ".[].path") --dry-run
```

Search the VSS catalogue, then create a container of matching descriptors with a dry-run preview.

## Auth Setup

BMW CarData uses an OAuth2 Device Code Flow with PKCE. First, generate a client_id in the BMW CarData portal (My BMW > BMW CarData > Create CarData Client; tick 'Request access to CarData API', and 'CarData Stream' if you want streaming) and save it — the portal hides it on reload. Then run `bmw-cardata-pp-cli auth login`, open the printed verification URL, log in to BMW, and approve. The CLI stores the access and refresh tokens locally and refreshes them automatically. You can also export BMW_CARDATA_CLIENT_ID and run auth login non-interactively. Requires an EU vehicle with an active SIM, a ConnectedDrive contract, and the vehicle mapped to your account as the primary user.

Run `bmw-cardata-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  bmw-cardata-pp-cli customers create-container --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
bmw-cardata-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
bmw-cardata-pp-cli feedback --stdin < notes.txt
bmw-cardata-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/bmw-cardata-pp-cli/feedback.jsonl`. They are never POSTed unless `BMW_CARDATA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BMW_CARDATA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
bmw-cardata-pp-cli profile save briefing --json
bmw-cardata-pp-cli --profile briefing customers create-container
bmw-cardata-pp-cli profile list --json
bmw-cardata-pp-cli profile show briefing
bmw-cardata-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `bmw-cardata-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/cmd/bmw-cardata-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add bmw-cardata-pp-mcp -- bmw-cardata-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which bmw-cardata-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   bmw-cardata-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `bmw-cardata-pp-cli <command> --help`.
