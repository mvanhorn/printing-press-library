---
name: pp-homeassistant
description: "A unified interface for your smart home with offline search and streaming states. Trigger phrases: `control home assistant`, `turn on light`, `check sensor state`, `home assistant services`."
author: "adrin425"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - homeassistant-pp-cli
---

# Home Assistant — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `homeassistant-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install homeassistant --cli-only
   ```
2. Verify: `homeassistant-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Call services, read sensors, and search your entire Home Assistant entity list instantly. Built with local SQLite caching so you never have to guess an entity ID again.

## When to Use This CLI

Use this CLI to orchestrate smart home automation flows, retrieve sensor data, and test service calls directly from the terminal. Perfect for bash scripts and agent-driven automations.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`homeassistant find`** — Find exact entities instantly using full-text search across friendly names and attributes.

  _Agents can locate entity IDs without knowing the exact domain prefix._

  ```bash
  homeassistant find "living room light"
  ```

### Agent-native plumbing
- **`homeassistant monitor`** — Stream state changes live to the terminal.

  _Allows waiting for an event to happen rather than polling._

  ```bash
  homeassistant monitor light.living_room
  ```
- **`homeassistant services payload`** — Print the exact JSON payload expected by any Home Assistant service.

  _Self-documenting commands prevent malformed requests._

  ```bash
  homeassistant services payload light.turn_on
  ```
- **`homeassistant template`** — Render a Home Assistant Jinja2 template server-side.

  _Agents can compute derived values using HA's native template language._

  ```bash
  homeassistant template "{{ states.light | count }} lights"
  ```
- **`homeassistant events`** — List event types or fire custom events on the HA event bus.

  _Enables automation triggers and custom event-driven workflows._

  ```bash
  homeassistant events list
  ```

### Temporal intelligence
- **`homeassistant history`** — Query state change history for entities over a time period.

  _Agents can analyze trends and detect anomalies without polling._

  ```bash
  homeassistant history --entity sensor.living_room_temperature
  ```
- **`homeassistant logbook`** — View the activity logbook with human-readable event descriptions.

  _Provides context about what happened and why, not just state values._

  ```bash
  homeassistant logbook --entity alarm_control_panel.area_001
  ```

### Operations
- **`homeassistant check-config`** — Validate the Home Assistant configuration.yaml remotely.

  _Catch config errors before restarting HA._

  ```bash
  homeassistant check-config
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**default** — Operations on default

- `homeassistant-pp-cli default <id>` — POST /{id}

**hassio** — Operations on info

- `homeassistant-pp-cli hassio create_info` — POST /api/hassio/addons/{id}/info
- `homeassistant-pp-cli hassio create_install` — POST /api/hassio/addons/{id}/install
- `homeassistant-pp-cli hassio create_restart` — POST /api/hassio/addons/{id}/restart
- `homeassistant-pp-cli hassio create_start` — POST /api/hassio/addons/{id}/start
- `homeassistant-pp-cli hassio create_stop` — POST /api/hassio/addons/{id}/stop
- `homeassistant-pp-cli hassio create_uninstall` — POST /api/hassio/addons/{id}/uninstall

**models** — Operations on models

- `homeassistant-pp-cli models` — POST /models

**ping** — Operations on ping

- `homeassistant-pp-cli ping` — DELETE /ping

**repository** — Operations on install

- `homeassistant-pp-cli repository create_install` — POST /repository/install
- `homeassistant-pp-cli repository create_uninstall` — POST /repository/uninstall
- `homeassistant-pp-cli repository create_update` — POST /repository/update

**states** — Operations on states

- `homeassistant-pp-cli states <id>` — GET /api/states/{id}

**websocket** — Operations on websocket

- `homeassistant-pp-cli websocket` — POST /api/websocket


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
homeassistant-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Search for an entity

```bash
homeassistant find thermostat
```

Quickly find the ID of your thermostat.

### Check current temperature

```bash
homeassistant states get sensor.living_room_temperature --agent --select state
```

Retrieve just the value of a temperature sensor.

### Tail all light changes

```bash
homeassistant monitor light
```

Watch lights turn on and off in real-time.

## Auth Setup

Requires a Long-Lived Access Token generated from your Home Assistant user profile.

Run `homeassistant-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  homeassistant-pp-cli default mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
homeassistant-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
homeassistant-pp-cli feedback --stdin < notes.txt
homeassistant-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.homeassistant-pp-cli/feedback.jsonl`. They are never POSTed unless `HOMEASSISTANT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HOMEASSISTANT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
homeassistant-pp-cli profile save briefing --json
homeassistant-pp-cli --profile briefing default mock-value
homeassistant-pp-cli profile list --json
homeassistant-pp-cli profile show briefing
homeassistant-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `homeassistant-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add homeassistant-pp-mcp -- homeassistant-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which homeassistant-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   homeassistant-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `homeassistant-pp-cli <command> --help`.
