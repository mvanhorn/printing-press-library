---
name: pp-dreo
description: "The only standalone CLI for Dreo smart-home devices — bulk control, live sensor streams, and a local history every... Trigger phrases: `turn off my fans`, `what's the temperature in my house`, `check air quality`, `bedtime mode`, `watch dreo devices`, `use dreo`, `run dreo`."
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - dreo-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/devices/dreo/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Dreo — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `dreo-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install dreo --cli-only
   ```
2. Verify: `dreo-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/dreo/cmd/dreo-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI whenever an agent or script needs to read or control Dreo devices without Home Assistant in the loop. It is ideal for one-shot bulk commands (turn everything off at bedtime), whole-house sensor reads, live state streaming for automation debugging, and any cron-driven Dreo automation. The agent-callable MCP surface mirrors every Cobra command so AI agents can drive your home without a custom integration.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Multi-device control
- **`bulk`** — Power, mode, or speed across every device matching a type/room filter in one command.

  _Replaces the #1 user pain — tapping each device in the Dreo app at bedtime — with one cron-callable line._

  ```bash
  dreo-pp-cli bulk --action off --type tower-fan --dry-run
  ```
- **`scene save`** — Capture the current state across selected devices as a named scene and replay it later as parallel WebSocket frames.

  _Sam's nightly bedtime routine becomes one command; survives app updates._

  ```bash
  dreo-pp-cli scene save bedtime --all && dreo-pp-cli scene apply bedtime --dry-run
  ```

### Cross-device intelligence
- **`sensors`** — Aggregated temperature, humidity, and PM2.5 across every sensor-bearing device in one ranked table.

  _Answers the agent question 'what's the air quality across my house?' in one tool call._

  ```bash
  dreo-pp-cli sensors --json
  ```
- **`sensors record`** — Persist WebSocket state events to a local sensor_readings table and query temperature/humidity/PM2.5 over arbitrary time windows.

  _Answers 'when did the bedroom fan last go to sleep mode' and similar historical questions agents and users actually ask._

  ```bash
  dreo-pp-cli sensors query --metric temperature --since 1h --json
  ```
- **`alerts`** — Report devices with low filter life, empty water tank, offline heartbeat, or sensor readings past a threshold.

  _Surfaces actionable problems (filter, water, dead devices) without manually inspecting each device._

  ```bash
  dreo-pp-cli alerts --pm25-above 50 --json
  ```
- **`rooms`** — Group devices by room with on-count, average temperature, and average humidity per room.

  _Answers 'what's happening in my bedroom right now' in one query._

  ```bash
  dreo-pp-cli rooms --json
  ```
- **`devices search`** — Full-text search over cached device name, room, model, and serial.

  _Fast device lookup for scripts and agents without round-tripping the cloud._

  ```bash
  dreo-pp-cli devices search Fan
  ```

### Realtime observability
- **`watch`** — Tail-f for Dreo device state — every WebSocket update as a JSON line on stdout.

  _Enables automation debugging without running Home Assistant, and feeds agent stream-processing pipelines._

  ```bash
  dreo-pp-cli watch --all --json
  ```

## Command Reference

**devices** — Discover and inspect Dreo devices on your account

- `dreo-pp-cli devices list` — List every Dreo device on your account
- `dreo-pp-cli devices state` — Read the current state snapshot for one device

**firmware** — Read firmware metadata and check for updates

- `dreo-pp-cli firmware` — Check whether a firmware update is available for a device

**settings** — Read and write persistent per-device settings

- `dreo-pp-cli settings get` — Get persistent settings for a device
- `dreo-pp-cli settings update` — Update persistent settings for a device


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
dreo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Hand-written Extensions

These commands are declared by the spec author and require separate hand-written wiring; the generator does not emit Cobra registration for them. They are listed here for discoverability and are intentionally outside `## Command Reference` so the verify-skill unknown-command check does not treat them as generator-owned paths.

- `dreo-pp-cli set <device>` — Set state on a device (power, speed, mode, oscillation, timer) over WebSocket
- `dreo-pp-cli bulk` — Fan-out a control command across every device matching a filter (--type, --room)
- `dreo-pp-cli watch` — Tail-f for Dreo device state — every WebSocket update as a JSON line on stdout
- `dreo-pp-cli sensors` — Whole-house temperature, humidity, and PM2.5 across every sensor-bearing device
- `dreo-pp-cli sensors record` — Record WebSocket state events to a local sensor_readings table for timeseries history
- `dreo-pp-cli sensors query` — Query the local sensor_readings table over arbitrary time windows
- `dreo-pp-cli alerts` — Report devices with low filter life, empty water tank, offline heartbeat, or sensor thresholds
- `dreo-pp-cli scene save <name>` — Snapshot the current state across selected devices as a named scene
- `dreo-pp-cli scene apply <name>` — Replay a saved scene as parallel WebSocket control frames
- `dreo-pp-cli rooms` — Per-room aggregates (device count, on-count, avg temperature, avg humidity)
- `dreo-pp-cli auth login` — Exchange Dreo email and password for an access token; caches token for subsequent calls
- `dreo-pp-cli auth logout` — Clear cached Dreo access token from local config
- `dreo-pp-cli auth status` — Show current Dreo auth status (cached token age, region, last login)

## Recipes


### Bedtime bulk-off

```bash
dreo-pp-cli bulk --action off --type tower-fan && dreo-pp-cli bulk --action sleep --type air-purifier
```

One line replaces ten taps in the Dreo app; cron-friendly.

### Whole-house sensor read (agent-friendly with --select)

```bash
dreo-pp-cli sensors --json --select 'devices.name,devices.room,devices.temperature_c,devices.humidity,devices.pm25'
```

Returns only the sensor columns an agent needs, dropping the verbose device metadata.

### Capture a bedtime scene

```bash
dreo-pp-cli scene save bedtime --all && dreo-pp-cli scene apply bedtime --dry-run
```

Snapshot all device state under a named scene, then replay it any night.

### Filter and water-tank alert sweep

```bash
dreo-pp-cli alerts --json
```

Reports purifiers with filter life < 10% and humidifiers with empty water tanks across your whole account.

### Sensor history query

```bash
dreo-pp-cli sensors query --device Fan --metric temperature --since 24h --json
```

Run the recorder in the background, then query the local timeseries — no other Dreo tool keeps history.

## Auth Setup

Dreo has no public developer API. Authentication uses your Dreo account email and password against the same OAuth endpoint the Dreo iOS app calls. The password is MD5-hashed before the wire; the bearer token returned is cached locally and reused.

**Env vars (only — no `--password` flag):**

```bash
export DREO_USERNAME='your-dreo-account-email'
export DREO_PASSWORD='your-dreo-password'
```

Then run any command. The CLI exchanges credentials for an access token on first use, caches both the credentials and the bearer token to `~/.config/dreo-pp-cli/config.toml` (mode `0600`), and reuses them. When the bearer expires (Dreo issues no refresh token), the CLI mints a new one transparently using the cached credentials — cron jobs and unattended runs don't need env vars re-exported. Optional: pin `DREO_REGION=us` (or `eu`) to skip region discovery on first login.

**Three ways to supply credentials, in order of preference:**

1. **Env vars (recommended):** `DREO_USERNAME` / `DREO_PASSWORD` exported in your shell or set by your secret-loading wrapper.
2. **`--password-stdin` (scriptable, no leak):** pipe the password from a secret store. Docker-style:
   ```bash
   op read 'op://Personal/Dreo/password' | dreo-pp-cli auth login --username me@example.com --password-stdin
   ```
3. **`--password <value>` (insecure, prints a stderr warning):** mysql/curl-style. Supported for ergonomics but prints a warning every time because the plaintext value lands in `ps`, `/proc/<pid>/cmdline`, audit logs, and shell history.

**Security contract:**
- Flag-supplied passwords (`--password <value>`) are accepted but warned against on stderr.
- Credentials and the bearer token are **persisted to `~/.config/dreo-pp-cli/config.toml` at mode `0600`**. This matches AWS CLI's `~/.aws/credentials` pattern and is necessary because Dreo's OAuth flow has no refresh token — without persistence, expired bearer tokens would require manual re-login. Treat the config file as sensitive: don't commit it, don't share it, don't sync it to cloud-storage providers without encryption.
- `dreo-pp-cli auth logout` wipes both the cached token and the persisted credentials.
- `dreo-pp-cli auth status` shows current authentication state without revealing the token or password.

Run `dreo-pp-cli doctor` to verify end-to-end.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  dreo-pp-cli devices list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
dreo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
dreo-pp-cli feedback --stdin < notes.txt
dreo-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.dreo-pp-cli/feedback.jsonl`. They are never POSTed unless `DREO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DREO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
dreo-pp-cli profile save briefing --json
dreo-pp-cli --profile briefing devices list
dreo-pp-cli profile list --json
dreo-pp-cli profile show briefing
dreo-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `dreo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add dreo-pp-mcp -- dreo-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which dreo-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   dreo-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `dreo-pp-cli <command> --help`.
