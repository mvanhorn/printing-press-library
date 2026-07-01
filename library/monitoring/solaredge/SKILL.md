---
name: pp-solaredge
description: "Every SolarEdge Monitoring API endpoint, plus local history that answers questions the API itself can't. Trigger phrases: `check my solar production`, `is my SolarEdge system underperforming`, `check my SolarEdge equipment for faults`, `list my SolarEdge sites`, `SolarEdge site overview`, `use solaredge`, `run solaredge`."
author: "garrickpg"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - solaredge-pp-cli
    install:
      - kind: go
        bins: [solaredge-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/monitoring/solaredge/cmd/solaredge-pp-cli
---

# SolarEdge — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `solaredge-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install solaredge --cli-only
   ```
2. Verify: `solaredge-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/solaredge/cmd/solaredge-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Existing SolarEdge wrappers (Python, Node, Go, Rust) are thin 1:1 API clients that return exactly what one endpoint gives you. This CLI computes things the API never does on its own: 'site underperformance' flags days statistically below the site's own trailing average, 'site changes' diffs the current period against the prior one, 'site health' combines four calls into a single go/no-go view, and 'budget status' tracks how much of the 300-request/day quota this CLI has used, since the API exposes no header or endpoint for it.

## When to Use This CLI

Use this CLI when you own or manage one or more SolarEdge sites and want offline-queryable production/consumption history, equipment health checks, or fleet-wide rollups without re-deriving them by hand from the raw Monitoring API on every check.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for real-time control of the inverter or battery (the SolarEdge Monitoring API is read-only; use the SetApp mobile app or installer tools for configuration changes)
- Do not use this CLI for live home-automation event triggers (e.g. turning on a load when production spikes) — it polls on demand, it does not push events; use Home Assistant or solaredge2mqtt for that
- Do not use this CLI to bypass the 300-request/day account limit — it tracks the budget, it cannot raise it

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing

- **`site health`** — See one combined go/no-go status for a site instead of cross-referencing several separate calls.

  _Pick this over 'site overview' or 'site current-power-flow' when you need one answer to 'is this site OK right now'._

  ```bash
  solaredge-pp-cli site health 1223050 --json
  ```
- **`site underperformance`** — Flag days where production fell below this site's own historical average for that time of year.

  _Pick this when the question is 'is this normal for this site' rather than 'what changed recently' (use 'site changes' for the latter)._

  ```bash
  solaredge-pp-cli site underperformance 1223050 --since 30d --agent
  ```
- **`site changes`** — Get a short digest of the energy delta vs the prior period of equal length, plus a current equipment-count snapshot.

  _Pick this for a recent-delta digest; pick 'site underperformance' instead for statistical baseline comparisons._

  ```bash
  solaredge-pp-cli site changes 1223050 --since 7d --json
  ```
- **`equipment faults`** — See only the inverters, batteries, or system elements in a non-nominal state.

  _Use this for a filtered fault list; use 'equipment inventory' for the full unfiltered equipment list._

  ```bash
  solaredge-pp-cli equipment faults 1223050 --json
  ```

### Reachability mitigation

- **`budget status`** — See how much of today's 300-request quota this CLI has used, per site.

  _Check this before running several site commands in a row so a 429 doesn't interrupt you partway through._

  ```bash
  solaredge-pp-cli budget status --json
  ```

## Command Reference

**accounts** — Installer account and sub-account management

- `solaredge-pp-cli accounts` — List the account and its sub-accounts (requires an account-level API key)

**api-version** — Monitoring API version information

- `solaredge-pp-cli api-version current` — Most up-to-date API version number
- `solaredge-pp-cli api-version supported` — List of supported API version numbers

**equipment** — Query SolarEdge equipment (inverters, optimizers, sensors)

- `solaredge-pp-cli equipment inverter-data` — Technical telemetry for one inverter (voltage, current, power) over a period (1 week max window)
- `solaredge-pp-cli equipment list` — List inverters/SMIs on this site, with name, model, manufacturer, serial number
- `solaredge-pp-cli equipment sensors` — List of sensors installed on this site and the gateway they connect to

**site** — Query a single SolarEdge site

- `solaredge-pp-cli site current-power-flow` — Live power flow between PV, storage, load, and grid
- `solaredge-pp-cli site data-period` — Production data start/end dates for this site
- `solaredge-pp-cli site details` — Site details: name, location, status, peak power
- `solaredge-pp-cli site energy` — Site energy measurements (1 year max for DAY, 1 month max for QUARTER_OF_AN_HOUR/HOUR)
- `solaredge-pp-cli site energy-details` — Detailed energy by meter: production, consumption, self-consumption, feed-in, purchased
- `solaredge-pp-cli site installer-image` — Installer logo image as uploaded to the monitoring portal
- `solaredge-pp-cli site inventory` — Full equipment inventory: inverters, batteries, meters, gateways, sensors
- `solaredge-pp-cli site overview` — Current power and lifetime/yearly/monthly/daily energy and revenue
- `solaredge-pp-cli site power` — 15-minute resolution power measurements (1 month max window)
- `solaredge-pp-cli site power-details` — Detailed power by meter: production, consumption, self-consumption, feed-in, purchased (1 month max window)
- `solaredge-pp-cli site sensor-data` — Telemetry from all sensors on this site (1 week max window)
- `solaredge-pp-cli site site-image` — Site image as uploaded to the monitoring portal
- `solaredge-pp-cli site storage-data` — Battery state of energy, power, and lifetime energy (1 week max window)
- `solaredge-pp-cli site time-frame-energy` — Total energy produced over a given period (1 year max)

**sites** — List and bulk-query SolarEdge sites

- `solaredge-pp-cli sites data-period` — Bulk: production data start/end dates for multiple sites
- `solaredge-pp-cli sites energy` — Bulk: site energy measurements for multiple sites
- `solaredge-pp-cli sites list` — List all sites visible to this API key, with search/sort/pagination
- `solaredge-pp-cli sites overview` — Bulk: current power and lifetime/yearly/monthly/daily energy for multiple sites
- `solaredge-pp-cli sites power` — Bulk: 15-minute resolution power measurements for multiple sites (max 1 month window)
- `solaredge-pp-cli sites time-frame-energy` — Bulk: total energy produced over a period for multiple sites


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
solaredge-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning health check

```bash
solaredge-pp-cli site health 1223050
```

One combined go/no-go status instead of cross-referencing overview, power flow, and equipment calls by hand.

### Is this normal for this time of year

```bash
solaredge-pp-cli site underperformance 1223050 --since 30d --agent
```

Flags days statistically below this site's own trailing average, the question the raw API can never answer in one call.

### Pull only the meter breakdown an agent needs

```bash
solaredge-pp-cli site energy-details 1223050 --start-time '2026-06-01 00:00:00' --end-time '2026-06-30 23:59:59' --agent --select meters.type,meters.values.value
```

energyDetails returns a verbose nested meters array; --select narrows it to just the meter type and values an agent needs.

### Check the daily rate-limit budget before a big sync

```bash
solaredge-pp-cli budget status --json
```

Shows requests used/remaining against the 300/day cap before kicking off a fleet-wide sync that could trigger a 429.

## Auth Setup

SolarEdge issues two kinds of API keys: an Account-level key (Account Admin > Company Details > API Access) that can see every site on the account, or a Site-level key (Site Admin > Site Details > API Access) scoped to one site. Either works with `SOLAREDGE_API_KEY`; account-level keys additionally unlock `accounts list` and bulk multi-site commands.

Run `solaredge-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  solaredge-pp-cli accounts --agent --select id,name,status
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `SOLAREDGE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SOLAREDGE_CONFIG_DIR`, `SOLAREDGE_DATA_DIR`, `SOLAREDGE_STATE_DIR`, `SOLAREDGE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SOLAREDGE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `solaredge-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "solaredge": {
        "command": "solaredge-pp-mcp",
        "env": {
          "SOLAREDGE_HOME": "/srv/solaredge"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SOLAREDGE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SOLAREDGE_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
solaredge-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
solaredge-pp-cli feedback --stdin < notes.txt
solaredge-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SOLAREDGE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SOLAREDGE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
solaredge-pp-cli profile save briefing --json
solaredge-pp-cli --profile briefing accounts
solaredge-pp-cli profile list --json
solaredge-pp-cli profile show briefing
solaredge-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `solaredge-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/monitoring/solaredge/cmd/solaredge-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add solaredge-pp-mcp -- solaredge-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which solaredge-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   solaredge-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `solaredge-pp-cli <command> --help`.
