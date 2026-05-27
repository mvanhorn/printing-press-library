---
name: pp-zaptec
description: "The only command-line tool for Zaptec Go chargers — pause and resume charging, read live state in plain English,... Trigger phrases: `pause my zaptec charger`, `is my car charging`, `how much did I charge this month`, `zaptec charger status`, `resume charging`, `use zaptec`, `run zaptec`."
author: "Pimmetjeoss"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - zaptec-pp-cli
---

# Zaptec — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `zaptec-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install zaptec --cli-only
   ```
2. Verify: `zaptec-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/zaptec/cmd/zaptec-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Control and monitor your Zaptec charger from the terminal or a cron job, without opening the portal app or running Home Assistant. Decodes the API's numeric observation and command IDs for you, syncs your charging history into a local SQLite store, and answers questions the portal can't — like monthly kWh and cost rollups (cost), instant fleet status (live), and offline-charger detection (chargers stale).

## When to Use This CLI

Use this CLI when an agent needs to inspect or control a Zaptec EV charger non-interactively: checking whether a car is charging, pausing or resuming charging in response to electricity prices, reading decoded charger telemetry, or answering energy and cost questions from charging history. It is the right tool whenever the task involves a Zaptec Go/Go2/Pro charger and a terminal or script rather than the phone app.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local history that compounds
- **`cost`** — See total kWh and cost for your charging over any period, rolled up by month or by charger.

  _Reach for this when the user asks how much they charged or what it cost over a period — it answers from local history instead of N paginated API calls._

  ```bash
  zaptec-pp-cli cost --by month --from 2026-01-01 --agent
  ```
- **`chargers stale`** — List chargers that are offline, disconnected, or haven't reported within a threshold.

  _Reach for this to catch dead or stuck chargers before a resident or driver complains._

  ```bash
  zaptec-pp-cli chargers stale --minutes 30 --agent
  ```
- **`sessions anomalies`** — Flag charging sessions with near-zero energy, abnormally long duration, or zero cost.

  _Reach for this to audit charging-session quality and catch stuck or mis-metered sessions._

  ```bash
  zaptec-pp-cli sessions anomalies --since 2026-04-01 --agent
  ```

### Decoded fleet visibility
- **`live`** — One table showing every charger's mode (charging/connected/finished/offline) with decoded power, current, and phase.

  _Reach for this for an instant fleet status check instead of clicking through chargers one by one in the portal._

  ```bash
  zaptec-pp-cli live --agent
  ```
- **`current headroom`** — Show how many amps of the installation's breaker limit are uncommitted versus actively drawn.

  _Reach for this before raising or lowering an installation's available current so the change is informed by real draw._

  ```bash
  zaptec-pp-cli current headroom 11111111-2222-3333-4444-555555555555 --agent
  ```
- **`firmware drift`** — Group an installation's chargers by firmware version and flag the ones behind the fleet majority.

  _Reach for this before scheduling firmware upgrades to see which chargers actually need one._

  ```bash
  zaptec-pp-cli firmware drift 11111111-2222-3333-4444-555555555555 --agent
  ```

## Command Reference

**chargehistory** — Manage chargehistory

- `zaptec-pp-cli chargehistory get` — Retrieves all completed charge sessions accessible by the current user, matching the provided filters. Default page...
- `zaptec-pp-cli chargehistory installationreport-get` — **Deprecated**: Use the POST version instead. Retrieves a usage report based on the provided filters (requires...
- `zaptec-pp-cli chargehistory installationreport-post` — Retrieves a usage report based on the provided filters (requires installation owner permissions).

**charger-firmware** — Manage charger firmware

- `zaptec-pp-cli charger-firmware <installationId>` — Retrieves firmware details for all chargers in the specified installation (requires owner or service permissions).

**chargers** — Manage chargers

- `zaptec-pp-cli chargers get` — Retrieves all chargers accessible by the current user, matching the provided filters. By default, returns the first...
- `zaptec-pp-cli chargers id-get` — Retrieves the specified charger (requires owner or service permissions).

**constants** — Manage constants

- `zaptec-pp-cli constants` — Retrieves a set of predefined constants that rarely change but may be updated occasionally. These constants include...

**installation** — Manage installation

- `zaptec-pp-cli installation get` — Retrieves all installations accessible by the current user, matching the provided filters. By default, returns the...
- `zaptec-pp-cli installation id-get` — Retrieves details for the specified installation. The level of detail available depends on the current user's...

**session** — Manage session

- `zaptec-pp-cli session <id>` — Retrieves details for the specified charging session.

**user-groups** — Manage user groups



### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
zaptec-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Pause charging when prices spike

```bash
zaptec-pp-cli pause CHARGER_ID
```

Send a StopCharging command from a price-watching cron job; add --dry-run first to preview.

### Monthly cost report

```bash
zaptec-pp-cli cost --by month --from 2026-01-01 --agent
```

Roll up local session history into per-month kWh and cost totals as structured JSON.

### Trim a verbose charger state

```bash
zaptec-pp-cli state CHARGER_ID --agent --select name,value
```

`state` returns one decoded observation per row (`state_id`, `name`, `value`, `unit`); `--select name,value` keeps just the human-readable pairs so an agent doesn't carry the full payload. For the raw, deeply nested charger object use `chargers get CHARGER_ID --agent --select Id,Name,OperationMode`.

### Catch offline chargers

```bash
zaptec-pp-cli chargers stale --minutes 30 --agent
```

List chargers that haven't reported recently before a resident complains.

### Plan a firmware rollout

```bash
zaptec-pp-cli firmware drift INSTALLATION_ID --agent
```

See which chargers are behind the fleet's majority firmware version before upgrading.

## Auth Setup

Zaptec uses OAuth2 password grant. Run `zaptec-pp-cli auth login` with your Zaptec portal username and password (or set ZAPTEC_USERNAME and ZAPTEC_PASSWORD); the CLI exchanges them for a bearer token at https://api.zaptec.com/oauth/token, caches it locally, and refreshes it when it expires.

Run `zaptec-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  zaptec-pp-cli chargehistory get --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
zaptec-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
zaptec-pp-cli feedback --stdin < notes.txt
zaptec-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.zaptec-pp-cli/feedback.jsonl`. They are never POSTed unless `ZAPTEC_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ZAPTEC_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
zaptec-pp-cli profile save briefing --json
zaptec-pp-cli --profile briefing chargehistory get
zaptec-pp-cli profile list --json
zaptec-pp-cli profile show briefing
zaptec-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `zaptec-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add zaptec-pp-mcp -- zaptec-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which zaptec-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   zaptec-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `zaptec-pp-cli <command> --help`.
