# 三井のリパーク (Repark) CLI

**Find Repark parking with live source vacancy, declared vehicle limits, and verified price estimates.**

Search named places or explicit coordinates, inspect source day and night rules, and compare chosen lots. Vacancy and declared vehicle fit remain separate; quotes come from the provider calculator.

Learn more at [三井のリパーク (Repark)](https://www.repark.jp).

Created by [@zjsng](https://github.com/zjsng) (Jet Sng).

This is a verified local build. Public installer and release links below become usable after separate publication. Build and run the local checkout with:

```bash
go build -o repark-pp-cli ./cmd/repark-pp-cli
./repark-pp-cli parking search 東京駅 --limit 3 --agent
```

## Install

The recommended path installs both the `repark-pp-cli` binary and the `pp-repark` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install repark
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install repark --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install repark --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install repark --agent claude-code
npx -y @mvanhorn/printing-press-library install repark --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/repark/cmd/repark-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/repark-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install repark --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-repark --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-repark --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install repark --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/repark-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/repark/cmd/repark-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "repark": {
      "command": "repark-pp-mcp"
    }
  }
}
```

</details>

## Parking command contract

| Command | Inputs | Source requests |
|---|---|---|
| `parking search` | One named place, station, area or address | Up to 3 including provider resolution |
| `parking nearby` | Explicit `--lat` and `--lon`, or `--park` | 1 or 2 |
| `parking detail` | Canonical REP ID or detail URL | 1 |
| `parking compare` | 2..5 distinct lots | Up to 5 |
| `parking quote` | Lot, actual `--bay`, exact `--start` and `--end` | 3 |
| `parking capabilities` | None | 0 |

Manual `sync` snapshots require a supplied bounded `--param range=...` or `REPARK_SYNC_RANGE` environment value (flags take precedence; no location is preset) and do not represent complete provider coverage. Parking commands read the public source on every invocation. They require no credentials or runtime browser. `--radius` is 50..2000 metres, `--limit` is 1..50 results, and `--max-scan-records` bounds marker matching independently of output size. Each response is limited to 2 MiB, each request to 20 seconds, and each command to 60 seconds or a shorter `--timeout`. There are no automatic retries. Source requests honor `--rate-limit`; the default paces at 2 requests per second.

Output uses `{meta, results}`. Detail and quote return one object; discovery returns `results.lots`; comparison returns `results.lots` plus `fetch_failures`. `--select` applies to the result object, so use `lots.id,lots.name,lots.occupancy` for a discovery projection. Empty discovery lists are `[]`; unresolved named searches report `needs_refinement` and observed source candidates. `--dry-run` performs no source request.

Occupancy categories are `available`, `crowded`, `full`, or `unknown`. Exact remaining spaces and measurement times are unknown. Available/crowded can reflect only compact or size-restricted bays. Published height/length/width in metres and weight in tonnes are checked separately; `declared_fit.guaranteed` stays false, and remaining-bay fit stays unknown. Source limits can differ by bay; unspecified variation is null. Bay-scoped or multi-amount maximum lines retain full source wording with unparsed numeric amounts.

Rate day-type labels remain in Japanese. Time bands preserve overnight boundaries; maximum rules distinguish time windows, elapsed time after entry, calendar-day wording, and unresolved conditions. `maximum_application` records explicit repeating or one-time wording, otherwise `unspecified`. The original conditions accompany normalization. Pricing is never totaled locally. Amounts preserve the displayed JPY value; `tax_included` is null unless the source explicitly states tax inclusion or exclusion. No extra tax is added.

Quotes submit only the provider's public simulation form. Times without offsets are JST; RFC3339 offsets convert to JST, with minute precision. Quotes use current source rates, exclude partner discounts, and do not guarantee final billed charges, vacancy or bay fit. The usual stay limit is 48 hours; end dates must fit the source's 365-day calendar window. Confirm the actual marked bay. Past-date simulation, if accepted by the source, is not a historical price lookup.

`observed_at` is the CLI fetch time in JST. Source `import_date` and `updated_at` are retained raw; their time zone and relation to occupancy measurement are unverified. Distance is calculated straight-line distance, not a travel route. Source coverage is unverified, and scan/output truncation is explicit.

Exit codes: 0 success (including empty or unresolved source results), 2 invalid input, 3 source 404, 5 source/parser failure, 6 cancelled/expired request, 7 HTTP 429. Comparison exposes failed lots and excludes them from successful results; all-failed comparison exits 5.

## Authentication

Public discovery and calculator simulation use no account, token or browser at runtime.

## Quick Start

```bash
# Check local setup without a network request.
repark-pp-cli doctor --dry-run

# Resolve a named place through the source.
repark-pp-cli parking search 東京駅 --limit 3 --agent

# Inspect source rates and vehicle limits.
repark-pp-cli parking detail REP0022209 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Parking planning
- **`parking compare`** — Compare source rates, maximum rules, opening hours and vehicle limits across chosen lot IDs.

  _Compare source rates, maximum rules, opening hours and vehicle limits across chosen lot IDs._

  ```bash
  repark-pp-cli parking compare REP0022209 REP0029431 --agent
  ```
- **`parking nearby`** — Assess supplied dimensions independently from remaining bay suitability.

  _Assess supplied dimensions independently from remaining bay suitability._

  ```bash
  repark-pp-cli parking nearby --lat 34.663534 --lon 135.516310 --height 2.1 --agent
  ```
- **`parking nearby`** — Filter source vacancy categories within an explicit search radius.

  _Filter source vacancy categories within an explicit search radius._

  ```bash
  repark-pp-cli parking nearby --park REP0022209 --available-only --agent
  ```
- **`parking detail`** — Read day type, rate windows and maximum rules alongside exact source conditions.

  _Read day type, rate windows and maximum rules alongside exact source conditions._

  ```bash
  repark-pp-cli parking detail REP0022209 --agent
  ```
- **`parking capabilities`** — Inspect source support, measurement unknowns and bounded request policy.

  _Inspect source support, measurement unknowns and bounded request policy._

  ```bash
  repark-pp-cli parking capabilities --agent
  ```

## Recipes

### Source facts

```bash
repark-pp-cli parking detail REP0022209 --agent --select id,name,occupancy,rates,maximum_charges
```

Narrow detailed output to source identity and tariff facts.

### Vehicle fit

```bash
repark-pp-cli parking nearby --park REP0022209 --height 2.1 --limit 3 --agent
```

See declared limit conflicts separately from vacancy.

### Source quote

```bash
repark-pp-cli parking quote REP0022209 --bay 1 --start 2026-10-03T08:00 --end 2026-10-03T12:00 --agent
```

Get a provider estimate for a future exact JST interval.

## Cookbook

```bash
# Compare named lots and retain uncertainty.
repark-pp-cli parking compare REP0022209 REP0029431 --height 2.1 --agent

# Narrow a supplied anchor's live shortlist.
repark-pp-cli parking nearby --lat 35.6812996 --lon 139.7670658 --limit 3 --agent --select lots.id,lots.name,lots.occupancy,lots.vehicle_limits

# Ask the source for an exact overnight estimate; no local total arithmetic.
repark-pp-cli parking quote REP0022209 --bay 1 --start 2026-10-03T20:00 --end 2026-10-04T08:00 --agent
```

## Usage

Run `repark-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `REPARK_CONFIG_DIR`, `REPARK_DATA_DIR`, `REPARK_STATE_DIR`, or `REPARK_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `REPARK_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export REPARK_HOME=/srv/repark
repark-pp-cli doctor
```

Under `REPARK_HOME=/srv/repark`, the four dirs resolve to `/srv/repark/config`, `/srv/repark/data`, `/srv/repark/state`, and `/srv/repark/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "repark": {
      "command": "repark-pp-mcp",
      "env": {
        "REPARK_HOME": "/srv/repark"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `REPARK_DATA_DIR` overrides an explicit `--home` for that kind. Use `REPARK_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `REPARK_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `repark-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### site

Source interfaces for maintainers; prefer parking commands for planning facts

- **`repark-pp-cli site calculator-form`** - Inspect whether the source calculator form is present
- **`repark-pp-cli site detail-page`** - Fetch the source lot detail document
- **`repark-pp-cli site markers`** - Fetch source markers for an explicit bounded map window


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`repark-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`repark-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`repark-pp-cli learnings list`** - Inspect taught rows
- **`repark-pp-cli learnings forget <query>`** - Undo a teach
- **`repark-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`repark-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`repark-pp-cli teach-pattern`** - Install a query/resource template up front
- **`repark-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `REPARK_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `repark-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
repark-pp-cli site calculator-form --park example-value

# JSON for scripting and agents
repark-pp-cli site calculator-form --park example-value --json
# Filter to specific fields
repark-pp-cli site calculator-form --park example-value --json --select title

# Dry run — show the request without sending
repark-pp-cli site calculator-form --park example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
repark-pp-cli site calculator-form --park example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
repark-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `repark-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/repark-pp-cli/config.toml`; `--home`, `REPARK_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **A location query is ambiguous or yields no anchor.** — Use explicit --lat and --lon with parking nearby, or refine the Japanese place/address.
- **The calculator form is absent or rejects a bay.** — Read the source detail and calculator URL; choose an actual marked bay. No local total is substituted.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.repark.jp/parking_user/time/result/detail/
- Capture coverage: 0 API entries from 1 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: ssr_embedded_data (85% confidence)

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
