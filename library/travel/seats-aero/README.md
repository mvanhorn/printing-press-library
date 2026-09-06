# Seats.aero CLI

**Every Seats.aero Partner API endpoint, plus a local award-availability store that tells you what is new, what is still live, and where your miles reach nonstop.**

seats-aero-pp-cli wraps all seven Partner API endpoints (cached search, bulk availability, trips, routes, destinations, refresh, live) with agent-native output, then syncs routes and availability into typed SQLite tables. That local store powers new-since, calendar, direct-scan, reach and a quota-guarded recheck, none of which a thin wrapper or the web app can do.

Learn more at [Seats.aero](https://seats.aero).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).
Contributors: [@vinnyp](https://github.com/vinnyp) (Vinny Pasceri).

## Install

The recommended path installs both the `seats-aero-pp-cli` binary and the `pp-seats-aero` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install seats-aero
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install seats-aero --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install seats-aero --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install seats-aero --agent claude-code
npx -y @mvanhorn/printing-press-library install seats-aero --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/seats-aero/cmd/seats-aero-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/seats-aero-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install seats-aero --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-seats-aero --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-seats-aero --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install seats-aero --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/seats-aero-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SEATS_AERO_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/seats-aero/cmd/seats-aero-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "seats-aero": {
      "command": "seats-aero-pp-mcp",
      "env": {
        "SEATS_AERO_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Seats.aero Pro subscribers create a Partner API key under Settings, then export it as SEATS_AERO_API_KEY. Keys are Pro-tier (1,000 calls per day, resets at midnight UTC; the X-RateLimit-Remaining header tracks it) or commercial-agreement keys. Pro keys cannot call live search and commercial keys cannot call refresh; doctor reports the remaining daily calls and which tier your key appears to be.

## Quick Start

```bash
# Confirm the key is picked up and see the remaining daily quota
seats-aero-pp-cli doctor --dry-run

# Cheapest business-class awards JFK to LHR across every program in one call
seats-aero-pp-cli awards --origin-airport JFK --destination-airport LHR --cabins business --take 10 --agent

# Pull the tracked route atlas into the local store
seats-aero-pp-cli sync --resources routes --since 7d

# Bulk calendar for one program, ready to pipe into jq
seats-aero-pp-cli availability --source aeroplan --cabin business --take 20 --agent

# Where 90k miles get you nonstop from JFK, confirmed against dated seats
seats-aero-pp-cli reach --origin JFK --cabin business --max-mileage 90000 --top 10 --agent

```

## Upgrading from 2026.8.1

`seats-aero-partner-search` was renamed to `awards`; the old command remains a hidden, deprecated alias for one release. Its `--cabin` flag is now `--cabins` and accepts comma-separated values. Existing stores are migrated by the first read-write command, such as `sync`; `doctor` is read-only and reports `migration_pending` without changing the store.

The credential environment variable is now `SEATS_AERO_API_KEY`. `SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION` and the TOML key `aero_partner_partner_authorization` are still honoured; `doctor` reports the selected `credentials_location`.

The store is migrated in place on the first read-write command (`sync` or `doctor`), while the legacy `seats_aero_partner_search` table is left untouched. `sync --concurrency` now defaults to `1` (was `4`), and `--timeout` now defaults to `1m` (was `30s`). The MCP tool is now `awards_cached-search`.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`new-since`** — See which award seats appeared on a route since you last looked, from your synced local data.

  _Reach for this when the user asks what changed or what is new on a route, instead of re-running a full search and diffing by eye._

  ```bash
  seats-aero-pp-cli new-since --origin JFK --destination NRT --cabin business --since 24h --agent
  ```
- **`direct-scan`** — Find direct-flight award seats under a mileage ceiling across every synced program at once.

  _Pick this over the awards search when the user wants nonstop-only results across programs from already-synced data with zero API calls._

  ```bash
  seats-aero-pp-cli direct-scan --origin JFK --destination NRT --cabin business --max-mileage 90000 --sources united,virginatlantic,aeroplan --agent
  ```
- **`calendar`** — Turn one route's synced availability into a date-by-cabin matrix you can scan at a glance.

  _Use this when the user asks which dates have business or first availability on a route; it answers from the local store in one shot._

  ```bash
  seats-aero-pp-cli calendar --origin JFK --destination NRT --source united --start 2026-10-01 --end 2026-12-31 --agent
  ```

### Quota-aware plumbing
- **`recheck`** — Re-verify aging award rows are still live right before booking. It performs one live quota probe per run (1 daily call), and --apply is refused when quota is unknown unless --ignore-quota is passed.

  _Print-only by default: lists the aging rows it would refresh, their age, and the remaining daily quota (one probe call). Add --apply to spend refresh credits; --apply is refused when the quota is unknown unless --ignore-quota is passed._

  ```bash
  seats-aero-pp-cli recheck --origin JFK --destination NRT --cabin business --max-mileage 90000 --agent
  ```
- **`reach`** — Discover where your miles can take you nonstop from one airport, ranked by cost and cross-checked against real dated seats.

  _Reach for this when the user has miles but no fixed destination. It has no local mode, and results is an object {origin, cabin, max_mileage, source, destinations[]} rather than a bare array. --confirm-live makes up to 10 extra live /search calls (opt-in; never under the test harness)._

  ```bash
  seats-aero-pp-cli reach --origin JFK --cabin business --max-mileage 90000 --top 10 --agent
  ```

## Recipes

### Cheapest business awards across all programs, narrowed for an agent

```bash
seats-aero-pp-cli awards --origin-airport SFO --destination-airport NRT --cabins business --order-by lowest_mileage --take 10 --agent --select data.Date,data.Route.Source,data.JMileageCost,data.JDirect
```

One cached search across every program, with --select trimming the wide availability rows to the four fields that matter.

### What appeared since yesterday

```bash
seats-aero-pp-cli new-since --origin JFK --destination NRT --cabin business --since 24h --agent
```

Diffs the local availability table on first_seen_at, so only genuinely new rows come back.

### Verify before you book, without spending credits

```bash
seats-aero-pp-cli recheck --origin JFK --destination NRT --cabin business --max-mileage 90000 --agent
```

Print-only by default: lists the aging rows it would refresh, their age, and the remaining daily quota (one probe call). Add --apply to spend refresh credits.

### Direct-only under 90k miles across three programs

```bash
seats-aero-pp-cli direct-scan --origin JFK --destination NRT --cabin business --max-mileage 90000 --sources united,virginatlantic,aeroplan --agent
```

A cross-program join over synced data that the one-program-per-call bulk endpoint cannot answer.

### Where can 90k miles take me nonstop

```bash
seats-aero-pp-cli reach --origin JFK --cabin business --max-mileage 90000 --top 10 --agent
```

Fans out via destinations then confirms each candidate against dated seats. reach has no local mode, and results is an object {origin, cabin, max_mileage, source, destinations[]} rather than a bare array. --confirm-live makes up to 10 extra live /search calls (opt-in; never under the test harness).

## Usage

Run `seats-aero-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SEATS_AERO_CONFIG_DIR`, `SEATS_AERO_DATA_DIR`, `SEATS_AERO_STATE_DIR`, or `SEATS_AERO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SEATS_AERO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SEATS_AERO_HOME=/srv/seats-aero
seats-aero-pp-cli doctor
```

Under `SEATS_AERO_HOME=/srv/seats-aero`, the four dirs resolve to `/srv/seats-aero/config`, `/srv/seats-aero/data`, `/srv/seats-aero/state`, and `/srv/seats-aero/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "seats-aero": {
      "command": "seats-aero-pp-mcp",
      "env": {
        "SEATS_AERO_HOME": "/srv/seats-aero"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SEATS_AERO_DATA_DIR` overrides an explicit `--home` for that kind. Use `SEATS_AERO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SEATS_AERO_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `seats-aero-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

Outputs from `doctor`, `agent-context`, `learnings`, and `feedback` can contain local paths or free text and should not be delivered with `--deliver webhook:<url>` to third-party webhooks.

## Commands

### availability

Manage availability

- **`seats-aero-pp-cli availability`** - Retrieve a large amount of availability objects from one specific mileage program.

### awards

Manage awards

- **`seats-aero-pp-cli awards`** - Search Seats.aero cached award availability between one or more origin and destination airports, across one or more mileage programs. This is the flagship cached-search endpoint behind seats.aero's own web app.

### destinations

Manage destinations

- **`seats-aero-pp-cli destinations`** - Returns the airports reachable from (or to) a single airport, along with the cheapest raw nonstop mileage price per cabin, aggregated across every source. Only direct/nonstop itineraries are considered; connecting prices are excluded. Supply exactly one of origin_airport or destination_airport. A cabin with no nonstop availability is returned as null (never 0).

### live

Manage live

- **`seats-aero-pp-cli live`** - Commercial-agreement API keys only; Pro keys receive 403 -- do not retry. 5-15 s latency.

### refresh

Manage refresh

- **`seats-aero-pp-cli refresh`** - Use this endpoint to refresh old cached data. Pro API keys only; commercial-agreement keys cannot use /refresh (use /live). Credit-metered -- the response quota block shows remaining refreshes.

### routes

Manage routes

- **`seats-aero-pp-cli routes`** - Get all origin-destination routes tracked for a mileage program.

### trips

Manage trips

- **`seats-aero-pp-cli trips <id>`** - Retrieve flight-level information from an Availability object by its revalidation/trip ID from search or availability results.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`seats-aero-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`seats-aero-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`seats-aero-pp-cli learnings list`** - Inspect taught rows
- **`seats-aero-pp-cli learnings forget <query>`** - Undo a teach
- **`seats-aero-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`seats-aero-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`seats-aero-pp-cli teach-pattern`** - Install a query/resource template up front
- **`seats-aero-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SEATS_AERO_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `seats-aero-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
seats-aero-pp-cli destinations

# JSON for scripting and agents
seats-aero-pp-cli destinations --json
# Filter to specific fields
seats-aero-pp-cli destinations --json --select airport,business,economy

# Dry run — show the request without sending
seats-aero-pp-cli destinations --dry-run

# Agent mode — JSON + compact + no prompts in one flag
seats-aero-pp-cli destinations --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `SEATS_AERO_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

`sync` and `sync --resources availability` cannot populate availability because the endpoint requires `source`. Use `seats-aero-pp-cli sync --resources availability --resource-param availability:source=<program> --since 7d` once per program. The `routes` resource needs no parameter.

Covered command paths:
- `seats-aero-pp-cli availability`
- `seats-aero-pp-cli availability get`
- `seats-aero-pp-cli availability list`
- `seats-aero-pp-cli availability search`
- `seats-aero-pp-cli awards`
- `seats-aero-pp-cli awards get`
- `seats-aero-pp-cli awards list`
- `seats-aero-pp-cli awards search`
- `seats-aero-pp-cli destinations`
- `seats-aero-pp-cli destinations get`
- `seats-aero-pp-cli destinations list`
- `seats-aero-pp-cli destinations search`
- `seats-aero-pp-cli routes`
- `seats-aero-pp-cli routes get`
- `seats-aero-pp-cli routes list`
- `seats-aero-pp-cli routes search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
seats-aero-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `seats-aero-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/seats-aero-partner-pp-cli/config.toml`; `--home`, `SEATS_AERO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SEATS_AERO_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `seats-aero-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `seats-aero-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SEATS_AERO_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 403 on live search** — Live search needs a commercial-agreement key; Pro keys are rejected by design. Use awards (cached search) instead.
- **HTTP 429 or requests rejected until tomorrow** — The Pro daily quota of 1,000 calls is exhausted; run seats-aero-pp-cli doctor --json to see x-ratelimit-remaining and reset seconds, and prefer local commands (calendar, direct-scan, new-since) until it resets.
- **new-since or direct-scan return nothing** — sync and sync --resources availability cannot populate availability because the endpoint requires source. Run seats-aero-pp-cli sync --resources availability --resource-param availability:source=<program> --since 7d once per program; routes needs no parameter. Then re-run; local commands never call the API.
- **refresh says quota remaining 0** — Refresh credits are separate from daily calls and reset on the schedule in the response quota block. recheck is print-only by default but performs one live quota probe per run (1 daily call); --apply is refused when quota is unknown unless --ignore-quota is passed.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**seats.aero-mcp-server**](https://github.com/gavgrego/seats.aero-mcp-server) — TypeScript (25 stars)
- [**seats-aero-go**](https://github.com/denverquane/seats-aero-go) — Go (1 stars)
- [**award-travel-finder-plugin**](https://github.com/AwardTravelFinder/award-travel-finder-plugin) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
