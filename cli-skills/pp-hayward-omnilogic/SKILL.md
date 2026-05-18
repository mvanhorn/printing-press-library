---
name: pp-hayward-omnilogic
description: "Take control of every Hayward OmniLogic feature, plus a local store, schedule diffs, chemistry trends, and a morning... Trigger phrases: `is the pool ready`, `what's the pool temperature`, `show me the pool chemistry log`, `why isn't the pool pump running`, `preheat the pool for tonight`, `run the morning pool sweep`, `use hayward-omnilogic`, `run hayward-omnilogic`."
author: "Rob Zehner"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - hayward-omnilogic-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/devices/hayward-omnilogic/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Hayward OmniLogic — Printing Press CLI

> ## Write-command status
>
> The Hayward `.ashx` handler overloads the `IsOn` parameter on `SetUIEquipmentCmd`: it expects `dataType="int"` 0-100 for variable-speed pumps, `dataType="bool"` `True`/`False` for everything else. This was reverse-engineered via a side-by-side packet capture against the canonical Python wrapper (`djtimca/omnilogic-api` 0.6.1).
>
> **Verified working against the live API** (turn the pump on at 50%, observe `filterSpeed: 50` in telemetry, turn it off, observe `filterSpeed: 0`):
> - `pump set-speed <name> --bow Pool --speed N` — VSP control 0-100. Speed=0 stops the pump.
> - `equipment on <name> --bow Pool` — for VSPs, routes through `SetPumpSpeed(speed=100)`. For non-VSP equipment (relays, valves, single-speed accessory pumps) sends `IsOn=bool`.
> - `equipment off <name> --bow Pool` — same routing; speed=0 for VSPs, `IsOn=False` for non-VSPs.
>
> **Probably working but not yet verified end-to-end** (the operations use different XML shapes that didn't hit the IsOn overload trap, so they should work, but treat with care until exercised):
> - `heater enable`, `heater disable`, `heater set-temp` (different op: `SetHeaterEnable` / `SetUIHeaterCmd`, uses `Enabled` and `Temp` params)
> - `spillover set` (`SetUISpilloverCmd`, uses `Speed` param)
> - `superchlor on`, `superchlor off` (`SetUISuperCHLORCmd`, uses `IsOn=bool` which is the non-overloaded form)
> - `light show` (`SetStandAloneLightShow`, uses `Show` param)
> - `chlorinator set-params` (`SetCHLORParams`, uses ordered fixed-format builder)
> - `ready-by` (chains `SetHeaterEnable` + `SetUIHeaterCmd`; depends on the heater operations above)
>
> **Always working** (read-only commands, exhaustively tested against the live API):
> `sites list`, `config get`, `alarms list`, `telemetry get`, `chemistry get`, `status`, `sweep`, `sync`, `search`, `sql`, `command-log`, `runtime`, `schedule diff`, `chemistry log`, `chemistry drift`, `auth login/logout/status`, `doctor`, `capabilities get/set/clear`.
>
> Cloud propagation lag is 10-15 seconds — after a successful write, `telemetry get` may show the prior state until the cloud catches up. The CLI's command response (`{"status":"ok"}`) returns as soon as Hayward accepts the request.

## Pool sensor capabilities — configure once per site

Not every OmniLogic install has every sensor. Common variants:

- No pH or ORP probe (most basic salt-pool installs)
- No salt cell (chlorine-tab or liquid-chlorine pools)
- Water-temp sensor that only reads while the pump is running (a quirk of certain pump/sensor pairings)

When sensors are absent, Hayward's API returns `-1` or null for those fields. **Without telling the CLI which sensors actually exist, `status` will report `"caution"` for a perfectly healthy pool** simply because chemistry comes back as `unknown`. The `chemistry get`, `telemetry get`, and `status` commands include a `setup_hint` field in the JSON output when this is detected.

**Configure once per site:**

```bash
# Example: pool without pH/ORP probes, no salt cell, temp sensor needs pump flow
hayward-omnilogic-pp-cli capabilities set \
  --has-ph false --has-orp false --has-salt false --temp-needs-flow true

# Inspect:
hayward-omnilogic-pp-cli capabilities get --json
```

Stored in the local SQLite store per `MspSystemID`. After configuration: `status` correctly excludes the missing sensors from its verdict, `chemistry get` returns `not_equipped` instead of `unknown` for sensors that don't exist, and `water_temp = -1` while the pump is idle is reported as `"n/a (pump off)"` instead of "sensor offline".

**Agents seeing `setup_hint` in CLI output should surface the suggested `capabilities set` command to the user as a one-time setup step before relying on chemistry/temp verdicts.**

## Prerequisites: Install the CLI

This skill drives the `hayward-omnilogic-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install hayward-omnilogic --cli-only
   ```
2. Verify: `hayward-omnilogic-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Drives the same partner API the Hayward mobile app and Home Assistant integration use, but adds the historical chemistry log, the equipment diagnostic the cloud refuses to compute, the schedule-change detector for service techs, and the multi-site alarm sweep pool-service businesses ask for. Agent-native JSON throughout, with a local SQLite store that turns single-shot cloud reads into compound answers.

## When to Use This CLI

Pick this CLI when an agent needs to read or control a Hayward OmniLogic pool/spa system over the partner cloud API: pool owners managing one pool, integrators wiring OmniLogic into agentic workflows, and pool-service businesses managing many sites. The CLI replaces ad-hoc Python scripts on top of `omnilogic-api`, the Hayward mobile app for diagnostics and trends, and the per-site click-through pool-service operators do every morning.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Pool readiness at a glance
- **`status`** — One-shot "is the pool ready for guests?" view: chemistry in range, temp at setpoint, no active alarms, pump running, with a traffic-light verdict.

  _Reach for this when an agent or user wants a one-shot pool-state summary instead of correlating three commands._

  ```bash
  hayward-omnilogic-pp-cli status --json
  ```
- **`ready-by`** — Enables the heater and computes when to start it so the pool hits your target temperature by a specified arrival time, using the learned heat rate from telemetry history.

  _Use this instead of "set heater + setpoint and guess" when the user has a specific swim time._

  ```bash
  hayward-omnilogic-pp-cli ready-by 18:00 --temp 84
  ```

### Local state that compounds
- **`chemistry log`** — Weekly/monthly pH, ORP, salt, and temperature history from the local store, exportable as CSV or JSON for HOA / service / insurance records.

  _Use this when an agent needs trend data or a compliance log over a date range._

  ```bash
  hayward-omnilogic-pp-cli chemistry log --since 30d --csv
  ```
- **`chemistry drift`** — Detects pH/ORP/salt drift versus a rolling baseline before Hayward's static thresholds fire; --forecast projects when each metric will leave the safe range.

  _Reach for this when the user wants early warning, not just "alarm is active."_

  ```bash
  hayward-omnilogic-pp-cli chemistry drift --forecast --json
  ```
- **`runtime`** — Pump hours, heater hours, salt-cell hours derived from telemetry deltas — for maintenance planning, warranty, and end-of-season service.

  _Use this when a service-business agent needs maintenance projections or a warranty case-file._

  ```bash
  hayward-omnilogic-pp-cli runtime --since 90d --json
  ```
- **`command-log`** — Every Set* command issued via this CLI is logged with who/when/what/result. --replay <id> re-issues a prior command for quick undo or redo.

  _Use this when an agent or operator needs to know what was changed recently or roll back a misfire._

  ```bash
  hayward-omnilogic-pp-cli command-log --since 7d
  ```

### Diagnostics the cloud can't do
- **`why-not-running`** — Diagnose why a pump, heater, or light isn't running: correlates active alarms, current relay state, scheduled run windows, heater demand, and superchlor lockouts into one explanation.

  _Reach for this when an agent is asked to triage "X isn't running" instead of telling the user to open the app._

  ```bash
  hayward-omnilogic-pp-cli why-not-running 'Main Pump'
  ```
- **`schedule diff`** — Diffs today's MSP-config schedule tree against prior versioned snapshots; catches silent edits made by service techs or by other app users.

  _Use this when a user reports unexpected pool behavior and you need to know what changed._

  ```bash
  hayward-omnilogic-pp-cli schedule diff --since yesterday
  ```

### Multi-site operations
- **`sweep`** — Across every site in the account, surface active alarms, out-of-range chemistry, and offline controllers in a single report — built for pool-service businesses doing route planning.

  _Use this when an agent is asked to prioritize the day's truck rolls across many pools._

  ```bash
  hayward-omnilogic-pp-cli sweep --alarms --chemistry --json
  ```

## Command Reference

**alarms** — Active alarms across the OmniLogic system

- `hayward-omnilogic-pp-cli alarms` — List active alarms for a site or every site.

**chemistry** — Chemistry-only view of telemetry plus historical-store-backed readouts

- `hayward-omnilogic-pp-cli chemistry` — Current chemistry snapshot: pH, ORP, salt, water temp, with safe-range verdict.

**chlorinator** — Salt chlorinator configuration

- `hayward-omnilogic-pp-cli chlorinator` — Set chlorinator config (op mode, timed percent, ORP timeout, etc). Defaults to current MSP values for any flag you...

**config** — Equipment inventory (MSP config tree) per site

- `hayward-omnilogic-pp-cli config` — Fetch the equipment inventory for one site — pumps, heaters, chlorinator, lights, valves, relays, sensors.

**equipment** — Generic on/off + timed-run control for valves, relays, lights, and accessory pumps

- `hayward-omnilogic-pp-cli equipment off` — Turn an equipment item off.
- `hayward-omnilogic-pp-cli equipment on` — Turn an equipment item on. Use --for to run for a bounded duration; otherwise stays on until you turn it off.

**heater** — Heater control (enable/disable + setpoint)

- `hayward-omnilogic-pp-cli heater disable` — Turn a heater off.
- `hayward-omnilogic-pp-cli heater enable` — Turn a heater on. Heater stays on until set-temp is reached or you disable it.
- `hayward-omnilogic-pp-cli heater set_temp` — Set a heater's target setpoint in degrees Fahrenheit.

**light** — ColorLogic light shows

- `hayward-omnilogic-pp-cli light list_shows` — List every available ColorLogic show with its numeric ID and human-readable name.
- `hayward-omnilogic-pp-cli light show` — Activate a ColorLogic show. V2 lights also accept --speed and --brightness.

**pump** — Variable-speed pump control

- `hayward-omnilogic-pp-cli pump <pump_name>` — Set a pump's running speed in RPM or percent (range comes from the pump's MSP config).

**sites** — Hayward OmniLogic sites (one per backyard controller registered to your account)

- `hayward-omnilogic-pp-cli sites` — List every site registered under your Hayward account.

**spillover** — Spillover control (pool-to-spa overflow)

- `hayward-omnilogic-pp-cli spillover` — Set spillover speed and optional run duration.

**superchlor** — Manual superchlorination (one-shot)

- `hayward-omnilogic-pp-cli superchlor off` — Stop superchlorination.
- `hayward-omnilogic-pp-cli superchlor on` — Start superchlorination on the body-of-water's salt chlorinator. Runs until the configured SCTimeout expires or you...

**telemetry** — Live state of every equipment item at one site

- `hayward-omnilogic-pp-cli telemetry` — Snapshot live state: chemistry (pH/ORP/salt), water and air temperature, pump speeds, heater enable, light state,...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
hayward-omnilogic-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Is the pool ready for guests?

```bash
hayward-omnilogic-pp-cli status --json --select verdict,chemistry,water_temp,active_alarms
```

One call, structured answer. Use --select to keep the JSON tight when piping to an LLM.

### Weekly chemistry log for the service record

```bash
hayward-omnilogic-pp-cli chemistry log --since 7d --csv > pool-chem-$(date +%Y-%m-%d).csv
```

Cron this every Sunday and the binder writes itself.

### Why isn't the main pump running right now?

```bash
hayward-omnilogic-pp-cli why-not-running 'Main Pump' --json
```

Returns the explanation as structured fields: active_alarms, scheduled_window, current_state, heater_demand, superchlor_lockout.

### Morning sweep across every site

```bash
hayward-omnilogic-pp-cli sweep --alarms --chemistry --json | jq '.sites[] | select(.alarms != [] or .chemistry_out_of_range == true)'
```

Pool-service operators: this collapses 12 in-app site-checks into one piped command.

### Preheat for an evening swim with ETA from learned heat rate

```bash
hayward-omnilogic-pp-cli ready-by 19:30 --temp 86 --dry-run
```

Pass --dry-run first to see when the heater would start; drop the flag to actually enable it.

## Auth Setup

No authentication required.

Run `hayward-omnilogic-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  hayward-omnilogic-pp-cli alarms --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
hayward-omnilogic-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
hayward-omnilogic-pp-cli feedback --stdin < notes.txt
hayward-omnilogic-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.hayward-omnilogic-pp-cli/feedback.jsonl`. They are never POSTed unless `HAYWARD_OMNILOGIC_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HAYWARD_OMNILOGIC_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
hayward-omnilogic-pp-cli profile save briefing --json
hayward-omnilogic-pp-cli --profile briefing alarms
hayward-omnilogic-pp-cli profile list --json
hayward-omnilogic-pp-cli profile show briefing
hayward-omnilogic-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `hayward-omnilogic-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add hayward-omnilogic-pp-mcp -- hayward-omnilogic-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which hayward-omnilogic-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   hayward-omnilogic-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `hayward-omnilogic-pp-cli <command> --help`.
