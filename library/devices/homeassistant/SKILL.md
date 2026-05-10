---
name: pp-homeassistant
description: "Printing Press CLI for Homeassistant. Discovered API spec for homeassistant (crowd-sniff)"
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

# Homeassistant — Printing Press CLI

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

Discovered API spec for homeassistant (crowd-sniff)

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**calendars** — Calendar entity operations

- `homeassistant-pp-cli calendars get-calendar-events` — Returns calendar events between start and end times
- `homeassistant-pp-cli calendars list-calendars` — Returns the list of calendar entities

**camera** — Camera proxy

- `homeassistant-pp-cli camera <entity_id>` — Returns the image data from the specified camera entity

**components** — Loaded integration components

- `homeassistant-pp-cli components` — Returns a list of currently loaded components

**config** — Home Assistant instance configuration

- `homeassistant-pp-cli config check-config` — Trigger a check of configuration.yaml and return validation result
- `homeassistant-pp-cli config get-config` — Returns the current configuration including version, location, timezone, and loaded components

**default** — Operations on default

- `homeassistant-pp-cli default <id>` — POST /{id}

**error_log** — Error log retrieval

- `homeassistant-pp-cli error_log` — Retrieve all errors logged during the current session as plaintext

**events** — Event bus operations

- `homeassistant-pp-cli events fire-event` — Fires an event with event_type and optional event_data
- `homeassistant-pp-cli events list-events` — Returns an array of event objects with event name and listener count

**hassio** — Operations on Hassio addon management

- `homeassistant-pp-cli hassio create-info` — POST /api/hassio/addons/{id}/info
- `homeassistant-pp-cli hassio create-install` — POST /api/hassio/addons/{id}/install
- `homeassistant-pp-cli hassio create-restart` — POST /api/hassio/addons/{id}/restart
- `homeassistant-pp-cli hassio create-start` — POST /api/hassio/addons/{id}/start
- `homeassistant-pp-cli hassio create-stop` — POST /api/hassio/addons/{id}/stop
- `homeassistant-pp-cli hassio create-uninstall` — POST /api/hassio/addons/{id}/uninstall

**history** — State change history

- `homeassistant-pp-cli history <timestamp>` — Returns state changes in the past, filtered by entity and time range

**intent** — Intent handling

- `homeassistant-pp-cli intent` — Handle an intent with name and data

**logbook** — Activity logbook

- `homeassistant-pp-cli logbook <timestamp>` — Returns an array of logbook entries with optional entity and time filters

**models** — Operations on models

- `homeassistant-pp-cli models` — POST /models

**ping** — Operations on ping

- `homeassistant-pp-cli ping` — DELETE /ping

**repository** — Operations on install

- `homeassistant-pp-cli repository create-install` — POST /repository/install
- `homeassistant-pp-cli repository create-uninstall` — POST /repository/uninstall
- `homeassistant-pp-cli repository create-update` — POST /repository/update

**services** — Service domain operations

- `homeassistant-pp-cli services call-service` — Calls a service within a specific domain with optional service_data
- `homeassistant-pp-cli services list-services` — Returns an array of service objects grouped by domain

**states** — Operations on states

- `homeassistant-pp-cli states delete-state` — Deletes an entity with the specified entity_id
- `homeassistant-pp-cli states get-states` — Returns a state object for specified entity_id
- `homeassistant-pp-cli states list-states` — Returns an array of state objects with entity_id, state, last_changed and attributes
- `homeassistant-pp-cli states update-state` — Updates or creates a state representation

**template** — Template rendering

- `homeassistant-pp-cli template` — Render a Home Assistant Jinja2 template and return the result as plaintext

**websocket** — Operations on websocket

- `homeassistant-pp-cli websocket` — POST /api/websocket


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
homeassistant-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Store your access token:

```bash
homeassistant-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `HOMEASSISTANT_TOKEN` as an environment variable.

Run `homeassistant-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

   ```bash
   homeassistant-pp-cli calendars get-calendar-events mock-value --start example-value --end example-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

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
homeassistant-pp-cli --profile briefing calendars get-calendar-events mock-value --start example-value --end example-value
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
