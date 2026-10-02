# Haneda Airport CLI

**Query first-party Haneda flight boards with precise dates, codeshares and terminal handoffs.**

Read domestic and international arrivals and departures, inspect source disruptions and published schedules, and compare your saved observations. Explicit source timestamps and unknown fields keep terminal planning grounded in what the airport actually reports.

Learn more at [Haneda Airport](https://www.tokyo-haneda.com).

## Local build

This is an unpublished local build. Build the CLI and optional MCP server from this directory:

```bash
go build -o build/stage/bin/haneda-airport-pp-cli ./cmd/haneda-airport-pp-cli
go build -o build/stage/bin/haneda-airport-pp-mcp ./cmd/haneda-airport-pp-mcp
```

Add `build/stage/bin` to your PATH to run the examples below. The library installation paths in the next section apply after publication; this run does not claim a published package or release.

## Install

The recommended path installs both the `haneda-airport-pp-cli` binary and the `pp-haneda-airport` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install haneda-airport
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install haneda-airport --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install haneda-airport --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install haneda-airport --agent claude-code
npx -y @mvanhorn/printing-press-library install haneda-airport --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/cmd/haneda-airport-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/haneda-airport-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install haneda-airport --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-haneda-airport --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-haneda-airport --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install haneda-airport --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/haneda-airport-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/cmd/haneda-airport-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "haneda-airport": {
      "command": "haneda-airport-pp-mcp"
    }
  }
}
```

</details>

## Authentication

The supported source paths are anonymous. No API key, login, reservation or browser runtime is required.

## Quick Start

```bash
# Check the generated runtime safely.
haneda-airport-pp-cli doctor --dry-run

# Read a small current JST departure board.
haneda-airport-pp-cli flights search --kind international --direction departure --limit 5 --agent

# Resolve Japanese airport names and provider codes.
haneda-airport-pp-cli catalog airports --kind domestic --query 札幌 --agent

# Resolve a flight and its terminal facilities.
haneda-airport-pp-cli plan NH849 --kind international --agent

```

## Unique Features

Additional capabilities combine the source's flight facts with local observations.

### Flight planning and observations
- **`snapshot save`** — Preserve timestamped scoped boards locally.

  _Preserve timestamped scoped boards locally._

  ```bash
  haneda-airport-pp-cli snapshot save --file /tmp/haneda-board.json --kind international --direction departure
  ```
- **`snapshot search`** — Filter a saved board without network access.

  _Filter a saved board without network access._

  ```bash
  haneda-airport-pp-cli snapshot search --file /tmp/haneda-board.json --flight NH849 --limit 5
  ```
- **`snapshot diff`** — See changed status, time and terminal facilities.

  _See changed status, time and terminal facilities._

  ```bash
  haneda-airport-pp-cli snapshot diff --before /tmp/haneda-before.json --after /tmp/haneda-after.json
  ```
- **`plan`** — Find the source terminal, gates and check-in handoffs.

  _Find the source terminal, gates and check-in handoffs._

  ```bash
  haneda-airport-pp-cli plan NH849 --kind international --agent
  ```
- **`flights rollover`** — Inspect adjacent service days and cross-midnight changes.

  _Inspect adjacent service days and cross-midnight changes._

  ```bash
  haneda-airport-pp-cli flights rollover --kind international --direction departure --limit 5
  ```

## Cookbook

Use the following recipes for a small board, arrivals, disruptions and published schedules.

## Recipes

### Small flight board

```bash
haneda-airport-pp-cli flights search --kind international --limit 5 --agent --select flights.id,flights.source_primary_flight,flights.scheduled_at,flights.status,sources.reported_at,observed_at
```

Keep useful flight facts and source time in compact agent output.

### Domestic arrivals

```bash
haneda-airport-pp-cli flights search --kind domestic --direction arrival --destination CTS --limit 5 --agent
```

Airport and source city aliases are resolved without changing codeshare identity.

### Disruptions

```bash
haneda-airport-pp-cli flights disruptions --kind international --direction both --limit 10 --agent
```

Separate airport summary counts from the returned delayed/canceled/diverted flight groups.

### Monthly schedule

```bash
haneda-airport-pp-cli schedule search --kind international --flight NH849 --limit 5 --agent
```

Inspect published periods and operating weekdays with explicit feed coverage.

### Codeshare detail

```bash
haneda-airport-pp-cli flights detail UA8003 --kind international --direction departure --limit 5 --agent
```

Resolve a marketing number through the source-primary-only endpoint and bounded board fallback; retain the original listed group.

### Padded terminal lookup

```bash
haneda-airport-pp-cli plan NH0849 --kind international --direction departure --agent
```

Find the same primary group with a zero-padded number and use source terminal/facility handoffs.

### Midnight and adjacent days

```bash
haneda-airport-pp-cli flights rollover --kind international --direction both --limit 5 --agent
```

Inspect explicit service and revised dates. Empty output means this observation contained no matching rollover evidence.

### Preserve an observation

```bash
haneda-airport-pp-cli snapshot save --kind international --direction both --file ./haneda-before.json --agent
```

Write a complete local observation; choose a new filename if it already exists. No provider state is changed.

### Search a saved group

```bash
haneda-airport-pp-cli snapshot search --file ./haneda-before.json --flight UA8003 --limit 5 --agent
```

After saving the preceding observation, resolve its marketing group offline with its original time and age.

### Compare observations

```bash
haneda-airport-pp-cli snapshot diff --before ./haneda-before.json --after ./haneda-after.json --limit 5 --agent
```

First save a second board to haneda-after.json with the same date/kind/direction. No changes is a valid result; disappearance is not inferred cancellation.

### Resolve an airline

```bash
haneda-airport-pp-cli catalog airlines --kind international --query ANA --limit 5 --agent
```

Retain provider airline identifiers, IATA flight prefixes, Japanese names and canonical airline URLs.

## Usage

Run `haneda-airport-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `HANEDA_AIRPORT_CONFIG_DIR`, `HANEDA_AIRPORT_DATA_DIR`, `HANEDA_AIRPORT_STATE_DIR`, or `HANEDA_AIRPORT_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `HANEDA_AIRPORT_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export HANEDA_AIRPORT_HOME=/srv/haneda-airport
haneda-airport-pp-cli doctor
```

Under `HANEDA_AIRPORT_HOME=/srv/haneda-airport`, the four dirs resolve to `/srv/haneda-airport/config`, `/srv/haneda-airport/data`, `/srv/haneda-airport/state`, and `/srv/haneda-airport/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "haneda-airport": {
      "command": "haneda-airport-pp-mcp",
      "env": {
        "HANEDA_AIRPORT_HOME": "/srv/haneda-airport"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `HANEDA_AIRPORT_DATA_DIR` overrides an explicit `--home` for that kind. Use `HANEDA_AIRPORT_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `HANEDA_AIRPORT_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `haneda-airport-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

| Command | Purpose |
| --- | --- |
| `flights search` | Live domestic/international arrivals and departures; local flight, airline, airport/city, status and terminal filters |
| `flights detail [flight-or-id]` | Resolve a listed number or board ID to its source group, full facilities and canonical detail link |
| `flights disruptions` | Source-declared delayed, canceled and diverted services, plus a separately scoped airport summary |
| `flights rollover` | Adjacent service days and explicit revised dates crossing midnight |
| `plan [flight-or-id]` | Flight facilities, terminal floor, inter-terminal transfer and connection-guide handoffs |
| `catalog airports` | English/Japanese names, airport codes and distinct source city values |
| `catalog airlines` | English/Japanese names, source airline codes, flight prefixes and airline URLs |
| `schedule search` | Published feed windows, service periods and weekdays; accepts full listed flight numbers |
| `snapshot save` | Fetch a complete unfiltered board and atomically save an observation |
| `snapshot search` | Filter one saved observation without network requests |
| `snapshot diff` | Compare two complete observations with identical source/date/kind/direction coverage |

Use `--kind domestic|international|all` and `--direction departure|arrival|both`. Defaults are international departures; detail and plan search both directions. `--destination` identifies the other airport: destination for departures, origin for arrivals. `--airline` matches any listed airline, including marketing codeshares; `--flight` accepts full numbers such as NH849 or UA8003. Detail and plan also resolve zero padding such as NH0849. `--status` accepts a source category or exact source status text, a comma-separated list, or `all` / `unknown`. Empty CSV tokens are errors. `--terminal T1|T2|T3` filters known terminals.

All result pages default to 20 groups, with `--limit 1..200` and `--offset 0..20000`. `next_offset` refers to the locally filtered result. Re-running a live page fetches a new observation, so totals may change between pages. Use a saved snapshot for stable offline paging.

## Flight evidence and dates

`--date YYYY-MM-DD` is a JST service-day query and defaults to today for live boards. The source supports domestic dates from today and international dates from yesterday, through three calendar months. An `all` query uses the domestic start. The provider may include flights from adjacent service days; their `service_date` and stable `hnd:kind:direction:YYYYMMDD:primary` IDs retain those days. Use `--service-day-only` to narrow to the requested day, or `flights rollover` to inspect rollover evidence.

Each `listed_flights` array retains source order. Its first number is `source_primary_flight`; the source does not establish that this is the operating carrier, so `operating_flight` stays null. Marketing codeshares match the same group without replacing its identity. Detail tries the source's primary-only exact endpoint, then performs a bounded board lookup when a codeshare or padded number needs resolution. `coverage.query_mode` and `coverage.flight_lookup` disclose the lookup scope.

`scheduled_at` and `revised_at` are distinct RFC3339 timestamps in JST. Revised times use the provider's explicit changed date, including earlier calendar days; `time_change_minutes` can be negative. The provider does not label a changed time as estimated versus actual, so `actual_at` stays null. Missing values remain null or empty arrays. `unknown_fields` identifies unavailable operator/actual time, airport, schedule/revision, terminal, boarding-gate and check-in-counter facts, plus unknown status/category or terminal mapping; a blank category with visible status text is still a known status. Gates, check-in counters, security checkpoints and arrival exits have separate fields. An arrival exit is not an aircraft boarding gate. Full `facilities` entries carry available provider map/detail links when requested with `--include-facilities` or via detail/plan.

`observed_at` is the local fetch time. Board `sources.reported_at` is the provider response time, not proof of when an individual flight last changed; board `source_updated_at` stays null. Monthly schedule `sources.updated_at` preserves its published `last_upd`. The disruption summary has its own `gettingAt` and coverage, so its counts need not equal a dated or filtered board. Airline information can arrive late; use the canonical flight/airline links for current operational guidance.

Monthly feeds expose raw `period_start` / `period_end`, each service's operating period, and weekdays. An optional exact `--date` must lie inside the feed's published window and applies period/weekday rules. Haneda clock times use JST. Remote international airport times remain unassigned to a timezone/day when the feed does not establish them. Schedule lookup uses a listed flight number; board IDs belong to `flights detail`.

## Saved observations

`snapshot save` records every group in the selected date/kind/direction scope with source totals and observation time. It refuses truncated scans. The default destination is a timestamped JSON file under the CLI cache's `haneda-snapshots` directory; `--file` chooses an explicit path, and replacing an existing regular file requires `--overwrite`. Files are capped at 8 MiB. Search uses `--file` or the latest cached observation. Explicit kind/direction requests must fit its coverage; explicit `--date` must match its source request date and selects only that service day. Uncovered requests are errors, while a covered query may return zero matching groups. Default search still preserves adjacent service-day rows. Snapshot data is explicitly local; it is never an implicit live fallback. Ages over five minutes set `stale_snapshot` by default and retain the original timestamps. `snapshot search --max-age 30m` chooses another positive age threshold.

Diff uses explicit `--before` / `--after`, or the newest compatible cached pair with matching origin/date/kind/direction. A newest unpaired scope does not hide an older compatible pair. Automatic selection examines at most 64 MiB. If a valid compatible pair has already been found, a scan stop caused by the budget or an unreadable/malformed older file preserves that comparison with `cache_selection_complete:false` and `cache_selection_notes`; a newer pair may remain unexamined. Choose explicit paths for exact pair selection. Without a known pair, a stopped scan is an error. Full provider facility entries and map handoffs participate in material changes. Missing baselines return empty results with `baseline_sufficient:false`. Explicitly selected malformed snapshots, invalid timestamps, missing source scopes, count mismatches and incomplete observations are errors; the bounded automatic selection exception above applies only to older files after a fully validated pair is already known. A disappeared group is `no_longer_reported`, never inferred canceled. Only the source's status establishes a reported cancellation.

## Request bounds

Domain commands have a 30-second hard deadline, no automatic retries, at most 20 requests, a 4 MiB response cap and a 16 MiB aggregate cap. The adaptive source limiter defaults to two requests per second. Board/catalog metadata is reused only within one command; live boards are freshly fetched. `budget` reports the actual request and response-byte counts. Typical single-kind boards use three requests; all kinds/both directions use eight. Detail fallback can add up to four board requests. Source failures and rate limits are errors rather than empty successes.

`--max-scan-records` defaults to 5000 and permits 1..20000. A reached cap is explicit in `scan_cap_hit`; filtered totals then cover only examined records. A failed component aborts an `all`/`both` scope. Airport status, schedules and terminal facilities do not establish seat inventory, fares, reservations or guaranteed connections.


### Raw source diagnostics

The hidden `source` group retains read-only provider shapes for diagnosis. Use the domain commands above for normalized planning facts; these raw commands are excluded from MCP tools.

- **`haneda-airport-pp-cli source airlines-domestic`** - Read the raw first-party airlines-domestic JSON source.
- **`haneda-airport-pp-cli source airlines-international`** - Read the raw first-party airlines-international JSON source.
- **`haneda-airport-pp-cli source airports-domestic`** - Read the raw first-party airports-domestic JSON source.
- **`haneda-airport-pp-cli source airports-international`** - Read the raw first-party airports-international JSON source.
- **`haneda-airport-pp-cli source board`** - Read the raw first-party board JSON source.
- **`haneda-airport-pp-cli source disruption-summary`** - Read the raw first-party disruption-summary JSON source.
- **`haneda-airport-pp-cli source schedule-domestic-arrivals`** - Read the raw first-party schedule-domestic-arrivals JSON source.
- **`haneda-airport-pp-cli source schedule-domestic-departures`** - Read the raw first-party schedule-domestic-departures JSON source.
- **`haneda-airport-pp-cli source schedule-international-arrivals`** - Read the raw first-party schedule-international-arrivals JSON source.
- **`haneda-airport-pp-cli source schedule-international-departures`** - Read the raw first-party schedule-international-departures JSON source.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`haneda-airport-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`haneda-airport-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`haneda-airport-pp-cli learnings list`** - Inspect taught rows
- **`haneda-airport-pp-cli learnings forget <query>`** - Undo a teach
- **`haneda-airport-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`haneda-airport-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`haneda-airport-pp-cli teach-pattern`** - Install a query/resource template up front
- **`haneda-airport-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `HANEDA_AIRPORT_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `haneda-airport-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
haneda-airport-pp-cli source airlines-domestic

# JSON for scripting and agents
haneda-airport-pp-cli source airlines-domestic --json
# Filter to specific fields
haneda-airport-pp-cli source airlines-domestic --json --select en,ja,ko

# Dry run — show the request without sending
haneda-airport-pp-cli source airlines-domestic --dry-run

# Agent mode — JSON + compact + no prompts in one flag
haneda-airport-pp-cli source airlines-domestic --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline observations** - use `snapshot search` and `snapshot diff` for saved board files; live domain commands reject `--data-source local`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
haneda-airport-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `haneda-airport-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/haneda-airport-pp-cli/config.toml`; `--home`, `HANEDA_AIRPORT_HOME`, and per-kind env vars can relocate it.

Raw source diagnostics use the generated header configuration. Domain commands use the anonymous first-party client and accept the configured base URL/timeout/rate limit, without transmitting configured credentials or custom headers.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run `flights search` in the same date/kind/direction scope to see reported flight groups

### API-specific
- **Date outside provider search coverage** — Use today in JST for domestic boards; international boards permit yesterday through three calendar months.
- **No gate, counter, operating carrier or actual time** — Treat the explicit unknown field as unavailable and follow the canonical airport/airline link.
- **Saved board looks old** — Fetch a new observation with haneda-airport-pp-cli flights search or snapshot save.

## Discovery Signals

Public assets were observed in native Chrome; request contracts were then confirmed by anonymous HTTP replay. The enriched capture contains actual replay exchanges rather than a browser-exported HAR.
- Target observed: https://www.tokyo-haneda.com/en/app/api/v2/flight/search
- Replay coverage: 14 public API exchanges across 10 endpoint shapes
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence)
- Candidate command ideas: create_search — Derived from observed POST /en/app/api/v2/flight/search traffic.; list_city_list_search.json — Derived from observed GET /site_resource/flight/data/dms/city_list_search.json traffic.; list_company_list_search.json — Derived from observed GET /site_resource/flight/data/dms/company_list_search.json traffic.; list_flight_status.json — Derived from observed GET /en/app_resource/flight/flightStatus/flight_status.json traffic.; list_hdacfasc.json — Derived from observed GET /app_resource/flight/data/dms/hdacfasc.json traffic.; list_hdacfdsc.json — Derived from observed GET /app_resource/flight/data/dms/hdacfdsc.json traffic.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
