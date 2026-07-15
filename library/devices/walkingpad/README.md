# WalkingPad CLI

**The standalone CLI for WalkingPad treadmills — drive the belt from your laptop over Bluetooth LE, and keep a permanent local history of every walk the belt itself forgets on power-cut.**

Every other way to script a WalkingPad lives inside a phone app or a Home Assistant integration. This is a general-purpose CLI: it speaks the reverse-engineered WalkingPad BLE protocol (the ph4r05 lineage), holds a single connection open to actually *keep the belt running* (a one-shot write can't sustain it), and records each walk to a local history store so streaks, trends, and calorie estimates survive the belt's memory loss.

It is **device-native**: commands map to BLE device capabilities, not HTTP endpoints. By default it runs replay-backed and never opens a connection, so reading, history, and analytics commands work anywhere and verification never actuates real hardware. Controlling a physical belt requires a binary built with `-tags ble_live` plus the `--live` flag — see [Live device control](#live-device-control-ble).

Created by [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `walkingpad-pp-cli` binary and the `pp-walkingpad` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install walkingpad
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install walkingpad --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install walkingpad --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install walkingpad --agent claude-code
npx -y @mvanhorn/printing-press-library install walkingpad --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/walkingpad/cmd/walkingpad-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/walkingpad-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install walkingpad --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-walkingpad --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-walkingpad --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install walkingpad --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Live device control (BLE)

By default this CLI is replay-backed and never opens a connection. To control a real device:

- **Build with the BLE backend:** `go build -tags ble_live ./...` (CGO/CoreBluetooth on macOS; pure-Go D-Bus on Linux; WinRT on Windows). The default build links no BLE stack, so `scan` and any `--live` operation are no-ops without this tag.
- **Pass `--live`** to actuate, with optional `--address` (skip auto-discovery) and `--timeout`. Physical-effect and configuration-risk commands also require `--confirm-physical-effect` (or `--dry-run` to preview the write first).
- **`walkingpad-pp-cli doctor`** reports whether the live backend is compiled in, the device's service UUIDs, the protocol's operating quirks and proven workflows, and — with `--live` — whether the device is reachable. **`walkingpad-pp-cli scan --live`** lists nearby devices by service UUID.
- Your terminal needs OS Bluetooth permission, and **most BLE devices accept only one client at a time** — close the official WalkingPad app first or the laptop connection will fail.

**Safety classes.** Each command that touches the belt carries a safety class. `physical-effect` commands (`run`, `stop`, `start`, `wake`, `set-speed`) move the belt; `configuration-risk` commands (`set-mode`, every `prefs` write) change persistent device settings. Both require `--confirm-physical-effect` to actuate, and both refuse to run under verification. Inspect the full callable/withheld surface with `walkingpad-pp-cli capabilities --json`.

**Why `run`, not `start`.** A one-shot `start` write does **not** keep the belt going — the firmware needs a sustained connection with the right handshake, mode switch, and pacing. `run` holds that connection for the whole walk (start → hold at speed → record → stop). Use `run` to actually walk; `start`/`wake`/`set-speed` are low-level single writes for advanced use.

## MCP server

`walkingpad-pp-mcp` is a stdio MCP server that mirrors this CLI's read surface as agent tools. It execs `walkingpad-pp-cli` (it has no BLE dependency of its own, so it builds and runs anywhere), walks the Cobra command tree, and exposes each command according to its annotations: read commands (`status`, `capabilities`, `doctor`, `today`, `sessions`, `trends`, `streak`, `calories`, `last-record`, `profile show`, …) are surfaced with `readOnlyHint`. Commands that move the belt or hold a long-lived BLE connection (`run`, `stop`, `monitor`, `record`, `prefs`, `set-speed`, `scan`, …) are hidden via the `mcp:hidden` annotation — a held-connection belt cannot be driven reliably by a one-shot tool call, so the human CLI keeps those and the agent path gets reads plus history/analytics.

Register the server with an MCP host (no JSON config required for hosts that support `claude mcp add`):

```bash
claude mcp add walkingpad-pp-mcp -- walkingpad-pp-mcp
```

<details>
<summary>Manual JSON config (advanced)</summary>

Add to your MCP host's config (e.g. Claude Desktop's `~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "walkingpad": {
      "command": "walkingpad-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Check BLE readiness, build state, service UUIDs, and operating quirks
walkingpad-pp-cli doctor

# Inspect the callable and withheld BLE capabilities with safety metadata
walkingpad-pp-cli capabilities --json

# Today's recorded walking totals (distance, steps, active minutes)
walkingpad-pp-cli today

# Your current consecutive-day walking streak
walkingpad-pp-cli streak

# Per-day distance/steps/minutes over the last two weeks
walkingpad-pp-cli trends --days 14

# Drive a real belt for 30 minutes at 2 km/h and record the walk
# (needs a -tags ble_live build)
walkingpad-pp-cli run --speed 2.0 --duration 30m --live --confirm-physical-effect
```

## Unique Features

These capabilities aren't available in any other tool for this device.

### Local history & analytics

The belt loses its run memory on power-cut. This CLI persists every recorded walk to a local store so the long view survives.

- **`today`** — Today's recorded totals: distance, steps, active minutes, session count.

  ```bash
  walkingpad-pp-cli today --json
  ```
- **`streak`** — Your current consecutive-day walking streak.

  ```bash
  walkingpad-pp-cli streak --json
  ```
- **`trends`** — Per-day distance, steps, and active minutes over a window (including zero-activity days).

  ```bash
  walkingpad-pp-cli trends --days 30 --json
  ```
- **`calories`** — Estimate calories burned from your body weight (the number the belt refuses to report). Set your weight first with `profile set --weight <kg>`.

  ```bash
  walkingpad-pp-cli profile set --weight 80
  walkingpad-pp-cli calories --days 7 --json
  ```
- **`export`** — Write `daily.json` (last 60 days) and `yesterday.json` so an iPhone Shortcut can log Walking workouts into Apple Health.

  ```bash
  walkingpad-pp-cli export --json
  ```

### Reliable live control

- **`run`** — Hold one BLE connection for the whole walk: switch to manual, start the belt at `--speed`, stream live status, record the walk, and stop with the firmware's ceremony when `--duration` elapses or on Ctrl-C. The only reliable way to actually walk.

  ```bash
  walkingpad-pp-cli run --speed 2.0 --duration 30m --live --confirm-physical-effect
  ```
- **`stop`** — Idle a running belt with the firmware's stop ceremony (speed-0 → settle → standby). A one-shot speed-0 write leaves a walked belt running under weight; the standby switch is what idles it.

  ```bash
  walkingpad-pp-cli stop --live --confirm-physical-effect
  ```
- **`monitor`** — Stream live belt telemetry (speed, distance, steps, time) until interrupted.

  ```bash
  walkingpad-pp-cli monitor --live --duration 30s --json
  ```

### Device discovery

- **`scan`** — Find any compatible WalkingPad by its BLE *service* UUID rather than its name, so renamed or family-variant belts are still discovered.

  ```bash
  walkingpad-pp-cli scan --live
  ```

## Usage

Run `walkingpad-pp-cli --help` for the full command reference and flag list, and `walkingpad-pp-cli <command> --help` for any command's flags and examples.

## Commands

### Live device control

- **`walkingpad-pp-cli run`** — Start a guided walk: run the belt over a held connection and record it
- **`walkingpad-pp-cli stop`** — Stop the belt (speed-0, settle, then switch to standby)
- **`walkingpad-pp-cli start`** — Low-level single start write (does not sustain the belt; prefer `run`)
- **`walkingpad-pp-cli wake`** — Wake the belt from standby
- **`walkingpad-pp-cli set-speed <kmh>`** — Single set-speed write
- **`walkingpad-pp-cli set-mode <mode>`** — Switch belt mode (configuration-risk)
- **`walkingpad-pp-cli prefs`** — Configure belt preferences: `max-speed`, `start-speed`, `child-lock`, `auto-start`, `sensitivity`, `units` (each configuration-risk)

### Live telemetry

- **`walkingpad-pp-cli status`** — Read device status (replay-backed by default; live with `--live`)
- **`walkingpad-pp-cli monitor`** — Stream live belt telemetry until stopped
- **`walkingpad-pp-cli last-record`** — Read the belt's last stored run (time, distance, steps)
- **`walkingpad-pp-cli record`** — Record a live walk into the local history store

### Local history & analytics

- **`walkingpad-pp-cli today`** — Show today's recorded walking totals
- **`walkingpad-pp-cli sessions`** — List recorded sessions for a date (default today)
- **`walkingpad-pp-cli trends`** — Show per-day walking totals over a window
- **`walkingpad-pp-cli streak`** — Show the current consecutive-day walking streak
- **`walkingpad-pp-cli calories`** — Estimate calories burned (the belt never reports them)
- **`walkingpad-pp-cli export`** — Export daily totals as JSON (e.g. for an Apple Health Shortcut)

### Profile

- **`walkingpad-pp-cli profile set`** — Set your body weight (kg) for calorie estimates
- **`walkingpad-pp-cli profile show`** — Show your saved body profile

### Diagnostics & discovery

- **`walkingpad-pp-cli doctor`** — Check BLE readiness, build, and (with `--live`) device reachability
- **`walkingpad-pp-cli capabilities`** — Show generated BLE capability and safety metadata
- **`walkingpad-pp-cli scan`** — Discover nearby devices by their BLE service (requires `--live`)
- **`walkingpad-pp-cli session`** — Manage the replay-backed local BLE session runtime (`status`, `start`, `stop`)

## Output Formats

```bash
# Human-readable text (default in terminal)
walkingpad-pp-cli today

# JSON for scripting and agents
walkingpad-pp-cli today --json

# Agent-friendly JSON (equivalent to --json)
walkingpad-pp-cli today --agent

# Dry run — preview a device write without dispatching it
walkingpad-pp-cli run --speed 2.0 --dry-run --json
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** — never prompts; every input is a flag or positional argument
- **Pipeable** — `--json` / `--agent` output to stdout, errors to stderr
- **Previewable** — `--dry-run` shows the device write without sending it
- **Safe by default** — replay-backed unless `--live` is passed against a `-tags ble_live` build; physical-effect and configuration-risk commands require `--confirm-physical-effect`
- **Read-first MCP surface** — the MCP server exposes reads and history/analytics; held-connection control stays on the human CLI

Exit codes: `0` success, `1` error.

## Health Check

```bash
walkingpad-pp-cli doctor
```

Reports whether live BLE is compiled in, the active verify/dogfood state, the device's service UUIDs, the protocol's operating quirks and proven workflows, and — with `--live` — whether the device is reachable. Safe to run anywhere; never actuates the belt.

## Configuration

- **Live BLE flags (persistent):** `--live`, `--address <addr>`, `--timeout <dur>` (default `20s`), `--dry-run`, `--json` / `--agent`.
- **Local history store** and **profile** are kept under the OS user data directory for `walkingpad-pp-cli` (created on first write). `profile set --weight <kg>` is required before `calories` can estimate.
- **`WALKINGPAD_TRACE=1`** traces every BLE write and decoded status frame to stderr during a live `run`, for diagnosing whether mode and belt-state match expectations.

## Troubleshooting

- **`scan` / `--live` do nothing** — You're on the default replay build. Rebuild with `go build -tags ble_live ./...` to link the BLE backend.
- **Live connect fails / device not found** — Most belts accept only one BLE client at a time; close the official WalkingPad phone app, then retry. Confirm OS Bluetooth permission for your terminal, and that the pad's display is on.
- **The belt auto-stops a few seconds after starting** — In manual mode the firmware auto-stops within ~2s if it detects no actual walking (zero steps), even with someone standing still. This is a safety behavior, not an error; sustained running requires real steps.
- **`stop` didn't idle the belt** — A bare speed-0 write leaves a walked belt in manual mode (still running under weight). Use `walkingpad-pp-cli stop` (or let `run` exit), which does speed-0 → settle → standby.
- **`calories` says no weight set** — Run `walkingpad-pp-cli profile set --weight <kg>` first.

## Sources & Inspiration

This CLI was built by studying the reverse-engineered WalkingPad community work, principally:

- [**ph4r05/ph4-walkingpad**](https://github.com/ph4r05/ph4-walkingpad) — the WalkingPad BLE protocol reference (start/stop ceremony, command spacing, status frames)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
