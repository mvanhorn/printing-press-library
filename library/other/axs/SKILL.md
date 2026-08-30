---
name: pp-axs
description: "SRAM AXS CLI for bikes, components, registrations, ride activities, and synced component-summary analytics. Trigger phrases: `list my SRAM components`, `register a SRAM component`, `show my AXS garage`, `my recent AXS rides`, `use axs`, `run axs`."
author: "Greg Stellato"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - axs-pp-cli
    install:
      - kind: go
        bins: [axs-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/axs/cmd/axs-pp-cli
---

# SRAM AXS — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `axs-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install axs --cli-only
   ```
2. Verify: `axs-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/axs/cmd/axs-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

SRAM AXS CLI for bikes, components, registrations, ride activities, and synced component-summary analytics.

## When to Use This CLI

Reach for this CLI when a rider or agent needs scriptable access to a SRAM AXS account: listing bikes and components, registering parts by serial, checking synced firmware/battery status, browsing the model catalog, reviewing ride activities, or querying synced data offline.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to flash or install firmware; it can report web-visible firmware status only.
- Do not use it to configure shifting behavior or pair components over BLE — that is a phone-app/Bluetooth task, not an HTTP API task.
- Do not use it for non-SRAM drivetrains.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Synced component-summary views
- **`firmware-check`** — Show the latest synced firmware version per AXS device type.

  _Pick this when an agent needs the current synced firmware signal without clicking through every component._

  ```bash
  axs-pp-cli firmware-check --json
  ```
- **`battery`** — Latest synced battery status per AXS device type, including voltage when available.

  _Pick this for a quick pre-ride battery signal across synced AXS device types._

  ```bash
  axs-pp-cli battery --json
  ```

### Local state that compounds
- **`wear`** — Rank components by synced shift counts, distance, and actuations from component summaries.

  _Use to find the most-used drivetrain component for maintenance planning._

  ```bash
  axs-pp-cli wear --json --select device_type,device_label,shift_count,fd_shift_count,rd_shift_count
  ```
- **`shifts`** — Show per-ride front/rear shift counts, chainrings, and cogs from synced component summaries.

  _Use for ride-by-ride drivetrain usage analysis without dumping thousands of raw gear samples._

  ```bash
  axs-pp-cli shifts --totals --json
  ```
- **`since`** — Show new ride activities and notifications since your last sync.

  _Pick this to catch up on what changed without re-reading everything._

  ```bash
  axs-pp-cli since 7d
  ```

### AXS web data joined for agents
- **`garage`** — A tree of every bike with its components and serials in one shot.

  _Use as the one-command overview of a rider's whole AXS setup._

  ```bash
  axs-pp-cli garage
  ```

## Command Reference

**account** — Your SRAM account profile and settings

- `axs-pp-cli account accessgroups` — Get your public access groups
- `axs-pp-cli account export` — Request a data export of your account
- `axs-pp-cli account flags` — Get your account feature flags
- `axs-pp-cli account profile` — Get your account profile

**activities** — Your ride activities (telemetry host)

- `axs-pp-cli activities list` — List your ride activities
- `axs-pp-cli activities types` — List activity types

**bikes** — Your registered bikes

- `axs-pp-cli bikes get` — Get a bike by ID
- `axs-pp-cli bikes list` — List your registered bikes

**components** — Your AXS components (derailleurs, shifters, batteries, dropper posts)

- `axs-pp-cli components get` — Get a component by ID
- `axs-pp-cli components list` — List your AXS components

**devicetypes** — Catalog of AXS device types (public, no login required)

- `axs-pp-cli devicetypes` — List all AXS device types

**linkedids** — Linked third-party accounts (Strava, etc.)

- `axs-pp-cli linkedids list` — List your linked third-party accounts
- `axs-pp-cli linkedids unlink` — Unlink a third-party account

**models** — Catalog of component models and their firmware versions

- `axs-pp-cli models` — List component models and latest firmware versions

**notifications** — Your notifications inbox

- `axs-pp-cli notifications` — List your notifications

**products** — Product detail lookups

- `axs-pp-cli products <id>` — Get product detail by product ID

**registrations** — Register AXS components by serial number

- `axs-pp-cli registrations` — Register a component to your account by serial number

**stats** — Aggregate account stats (telemetry host)

- `axs-pp-cli stats` — Get your aggregate riding stats

**summaries** — Activity and component usage summaries (telemetry host)

- `axs-pp-cli summaries activities` — List per-activity summaries
- `axs-pp-cli summaries components` — List component usage summaries (wear, shift counts, distance)

**units** — Measurement / advanced units catalog

- `axs-pp-cli units` — List advanced measurement units


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
axs-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Pre-ride charge check

```bash
axs-pp-cli battery --json
```

Lists latest battery_status per synced AXS device type.

### Find stale firmware

```bash
axs-pp-cli firmware-check --json
```

Lists latest fw_version per synced AXS device type.

### Rank drivetrain use

```bash
axs-pp-cli wear --json
```

Ranks component-summary records by shift-count wear proxy.

### Total ride shifts

```bash
axs-pp-cli shifts --totals --json
```

Totals front and rear shifts across synced ride summaries.

### Register a part safely

```bash
axs-pp-cli registrations create --serial ABC123 --dry-run
```

Shows the registration request without sending it.

## Auth Setup

AXS uses your SRAM account (Auth0). Run `axs-pp-cli auth login` to sign in with your email and password (handled directly against SRAM's token endpoint — no browser), or paste an access token with `axs-pp-cli auth set-token` / set SRAM_AXS_TOKEN. The public device-type catalog works with no login.

Run `axs-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  axs-pp-cli activities list --agent --select id,name,status
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
axs-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
axs-pp-cli feedback --stdin < notes.txt
axs-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/axs-pp-cli/feedback.jsonl`. They are never POSTed unless `AXS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AXS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
axs-pp-cli profile save briefing --json
axs-pp-cli --profile briefing activities list
axs-pp-cli profile list --json
axs-pp-cli profile show briefing
axs-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `axs-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/axs/cmd/axs-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add axs-pp-mcp -- axs-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which axs-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   axs-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `axs-pp-cli <command> --help`.
