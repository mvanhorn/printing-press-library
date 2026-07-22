---
name: pp-walkingpad
description: "The standalone CLI for WalkingPad treadmills — drive the belt over Bluetooth LE and keep a permanent local history of every walk the belt forgets on power-cut. Trigger phrases: `start my walkingpad`, `how far have I walked today`, `what's my walking streak`, `walking trends this week`, `estimate my walking calories`, `log my walk to Apple Health`, `run the treadmill at 2 km/h`, `use walkingpad`, `run walkingpad`."
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - walkingpad-pp-cli
---

# WalkingPad — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `walkingpad-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install walkingpad --cli-only
   ```
2. Verify: `walkingpad-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/walkingpad/cmd/walkingpad-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI whenever an agent or script needs to inspect a WalkingPad's recorded walk history, compute streaks/trends/calories, or drive a physical belt from a laptop instead of the phone app. It is ideal for "how far did I walk today / this week", consecutive-day streak tracking, calorie estimates the belt won't report, exporting walks to Apple Health, and — on a live build — starting, holding, and stopping the belt over a reliable held BLE connection. The MCP surface mirrors the read and history/analytics commands so an agent can answer walking questions without the phone app.

## Unique Capabilities

These capabilities aren't available in any other tool for this device.

### Local history & analytics

The belt loses its run memory on power-cut. This CLI persists every recorded walk locally so the long view survives.

- **`today`** — Today's recorded totals: distance, steps, active minutes, session count.

  ```bash
  walkingpad-pp-cli today --json
  ```
- **`streak`** — Current consecutive-day walking streak.

  ```bash
  walkingpad-pp-cli streak --json
  ```
- **`trends`** — Per-day distance, steps, and active minutes over a window (including zero-activity days).

  ```bash
  walkingpad-pp-cli trends --days 30 --json
  ```
- **`calories`** — Estimate calories burned from your body weight (the belt never reports them). Set weight first with `profile set --weight <kg>`.

  ```bash
  walkingpad-pp-cli profile set --weight 80
  walkingpad-pp-cli calories --days 7 --json
  ```
- **`export`** — Write `daily.json` (last 60 days) and `yesterday.json` for an iPhone Shortcut to log Walking workouts into Apple Health.

  ```bash
  walkingpad-pp-cli export --json
  ```

### Reliable live control

- **`run`** — Hold one BLE connection for the whole walk: switch to manual, start at `--speed`, stream live status, record the walk, and stop with the firmware's ceremony when `--duration` elapses or on Ctrl-C. The only reliable way to actually walk; a one-shot `start` does not sustain the belt.

  ```bash
  walkingpad-pp-cli run --speed 2.0 --duration 30m --live --confirm-physical-effect
  ```
- **`stop`** — Idle a running belt with the firmware's stop ceremony (speed-0 → settle → standby).

  ```bash
  walkingpad-pp-cli stop --live --confirm-physical-effect
  ```

### Device discovery

- **`scan`** — Find any compatible WalkingPad by its BLE service UUID rather than its name, so renamed or family-variant belts are still discovered.

  ```bash
  walkingpad-pp-cli scan --live
  ```

## Command Reference

**Live device control**

- `walkingpad-pp-cli run` — Start a guided walk: run the belt over a held connection and record it
- `walkingpad-pp-cli stop` — Stop the belt (speed-0, settle, then switch to standby)
- `walkingpad-pp-cli prefs` — Configure belt preferences (`max-speed`, `start-speed`, `child-lock`, `auto-start`, `sensitivity`, `units`)

> Low-level single-write commands also exist — `start`, `wake`, `set-speed <kmh>`, and `set-mode <mode>` — for advanced use. They issue one BLE write and do not sustain the belt; prefer `run`/`stop` for actual walks. See `walkingpad-pp-cli --help`.

**Live telemetry**

- `walkingpad-pp-cli status` — Read device status (replay-backed by default; live with `--live`)
- `walkingpad-pp-cli monitor` — Stream live belt telemetry until stopped
- `walkingpad-pp-cli last-record` — Read the belt's last stored run (time, distance, steps)
- `walkingpad-pp-cli record` — Record a live walk into the local history store

**Local history & analytics**

- `walkingpad-pp-cli today` — Show today's recorded walking totals
- `walkingpad-pp-cli sessions` — List recorded sessions for a date (default today)
- `walkingpad-pp-cli trends` — Show per-day walking totals over a window
- `walkingpad-pp-cli streak` — Show the current consecutive-day walking streak
- `walkingpad-pp-cli calories` — Estimate calories burned (the belt never reports them)
- `walkingpad-pp-cli export` — Export daily totals as JSON (e.g. for an Apple Health Shortcut)

**Profile**

- `walkingpad-pp-cli profile set` — Set your body weight (kg) for calorie estimates
- `walkingpad-pp-cli profile show` — Show your saved body profile

**Diagnostics & discovery**

- `walkingpad-pp-cli doctor` — Check BLE readiness, build, and (with `--live`) device reachability
- `walkingpad-pp-cli capabilities` — Show generated BLE capability and safety metadata
- `walkingpad-pp-cli scan` — Discover nearby devices by their BLE service (requires `--live`)
- `walkingpad-pp-cli session` — Manage the replay-backed local BLE session runtime

## Live Device Control (BLE)

By default the CLI is replay-backed and never opens a connection — safe to run anywhere. To actuate a real belt:

- Build with the BLE backend: `go build -tags ble_live ./...` (CoreBluetooth/CGO on macOS, D-Bus on Linux, WinRT on Windows).
- Pass `--live` to contact the device, with optional `--address` and `--timeout`.
- **Safety classes:** `physical-effect` commands (`run`, `stop`, `start`, `wake`, `set-speed`) move the belt; `configuration-risk` commands (`set-mode`, every `prefs` write) change persistent settings. Both require `--confirm-physical-effect` (or `--dry-run` to preview first).
- **One client at a time:** most belts accept a single BLE connection — close the official WalkingPad app before connecting.
- **`run`, not `start`:** a one-shot `start` write does not keep the belt going; `run` holds the connection for the whole walk. Use `run` to actually walk.

Inspect the full callable/withheld surface and safety metadata with `walkingpad-pp-cli capabilities --json`, and check live readiness with `walkingpad-pp-cli doctor`.

## Recipes

### How far have I walked today

```bash
walkingpad-pp-cli today --json
```

Today's distance, steps, active minutes, and session count from the local history store.

### Walking streak and weekly trend

```bash
walkingpad-pp-cli streak --json
walkingpad-pp-cli trends --days 7 --json
```

Consecutive-day streak plus per-day totals for the last week.

### Calorie estimate (set weight once)

```bash
walkingpad-pp-cli profile set --weight 80
walkingpad-pp-cli calories --days 7 --json
```

MET-method estimate from recorded sessions and body weight — the number the belt refuses to report.

### Drive a real belt and record the walk (live build)

```bash
walkingpad-pp-cli run --speed 2.0 --duration 30m --live --confirm-physical-effect
```

Holds the BLE connection, runs the belt at 2 km/h for 30 minutes, records the walk, then stops it.

### Preview a device write without sending it

```bash
walkingpad-pp-cli run --speed 2.0 --dry-run --json
```

`--dry-run` shows the planned write and safety class without contacting the belt.

## Agent Mode

Add `--agent` (equivalent to `--json`) to any command for machine-readable JSON on stdout, errors on stderr.

- **Pipeable** — JSON on stdout, errors on stderr
- **Previewable** — `--dry-run` shows a device write without sending it
- **Non-interactive** — never prompts; every input is a flag or positional argument
- **Safe by default** — replay-backed unless `--live` is passed against a `-tags ble_live` build; physical-effect and configuration-risk commands require `--confirm-physical-effect`

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `walkingpad-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

`walkingpad-pp-mcp` is a stdio MCP server that mirrors the read and history/analytics commands as agent tools (held-connection control commands are hidden). Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add walkingpad-pp-mcp -- walkingpad-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which walkingpad-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   walkingpad-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `walkingpad-pp-cli <command> --help`.
5. For any command that contacts a real belt (`--live`), confirm the user wants physical actuation before running it, and pass `--confirm-physical-effect` only with that confirmation.
