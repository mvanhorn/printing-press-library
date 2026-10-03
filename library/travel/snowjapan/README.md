# SnowJapan CLI

**Compare Japan ski areas with terrain facts, dated reports and explicit historical-season evidence.**

Search and inspect public SnowJapan resort facts, compare a small shortlist, and read dated regional snow observations. Saved facts support tradeoff comparisons, municipality portfolios, historical span intersections and transparent evidence gaps.

Learn more at [SnowJapan](https://www.snowjapan.com).

Created by [@zjsng](https://github.com/zjsng) (zjsng).

## Install

The recommended path installs both the `snowjapan-pp-cli` binary and the `pp-snowjapan` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install snowjapan
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install snowjapan --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install snowjapan --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install snowjapan --agent claude-code
npx -y @mvanhorn/printing-press-library install snowjapan --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/snowjapan/cmd/snowjapan-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/snowjapan-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install snowjapan --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-snowjapan --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-snowjapan --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install snowjapan --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/snowjapan-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/snowjapan/cmd/snowjapan-pp-cli@latest
go install github.com/mvanhorn/printing-press-library/library/travel/snowjapan/cmd/snowjapan-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "snowjapan": {
      "command": "snowjapan-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Public read-only HTML and provider-published charts; no account, API key or running browser is needed.

## Quick Start

```bash
# Check the local command setup.
snowjapan-pp-cli doctor --dry-run

# Find exact source identities for a ski shortlist.
snowjapan-pp-cli resorts search --query Hakuba --limit 5 --agent

# Inspect terrain, installed lifts and source update status.
snowjapan-pp-cli resorts get nagano-prefecture/hakuba-village/able-hakuba-goryu --agent

# Inspect recorded endpoints from a completed winter.
snowjapan-pp-cli seasons list --season 2025-2026 --agent --select results.name,results.first_recorded_day,results.last_recorded_day,results.span_days

```

## Cookbook

Local planners require an explicit directory capture and, for season queries, an explicit capture of that exact winter:

```bash
snowjapan-pp-cli sync --resources resorts
snowjapan-pp-cli sync --resources seasons --resource-param seasons:season=2025-2026
snowjapan-pp-cli plan frontier --prefecture Nagano --maximize vertical,courses,longest --limit 5 --agent
snowjapan-pp-cli plan coverage --season 2025-2026 --resorts able-hakuba-goryu,hakuba-happo-one --agent
```

A winter that has not been captured returns `season_not_captured`; the CLI does not infer missing source evidence from an empty local cache. Every full season sync replaces that winter's captured population, keeping other winters separate.

For detailed saved-fact changes, capture the same exact resort twice on separate observations, then compare its latest two compatible projections:

```bash
snowjapan-pp-cli sync --resources resorts --resorts nagano-prefecture/hakuba-village/able-hakuba-goryu
snowjapan-pp-cli plan changes --resorts able-hakuba-goryu --agent
```

With no compatible pair, output has `missing_baseline`. Repeating the explicit detailed capture supplies a second observation; unchanged facts produce an empty change list. The comparison selects the most recently saved compatible pair across catalog and detail projections, so a newer catalog pair is not hidden by older detail observations.

Offline `resorts get` and `reports get` require an exact detail capture. A list-only projection returns `detail_not_captured`, including a capture command, and cannot serve as automatic network fallback. To save a report’s numeric observations (up to four exact dated IDs):

```bash
snowjapan-pp-cli sync --resources reports --reports hakuba-now-1st-october-2026
snowjapan-pp-cli reports get hakuba-now-1st-october-2026 --data-source local --agent
```

Ordinary report sync saves list metadata. A later list sync can replace the mirror row; repeat the exact detail capture before offline inspection. Freshness hints use the actual saved observation times; a fresh partial capture does not refresh unrelated older records.

Close any active database writer before reading local facts. Existing WAL/SHM/journal files make local reads fail with a retry instruction. Reads and captures resolve symlink targets, pin their SQL connection and verify database identity; hard-linked databases and URI-sensitive literal filenames are rejected to avoid ambiguous journals or the wrong file. Sync writes use the canonical database path, including for supported symlink aliases. External replacement of the database file during a write is unsupported; detected retargeting or identity changes fail the capture.

## Source scope and evidence

Source fact rows preserve canonical URLs and observation times. Computed planners expose dataset source URLs and observed-time ranges in `.meta`; town summaries do not have individual resort permalinks. Resort statistics describe installed facilities, without live lift-operation claims. Detail `information_status` and `planned_window` are source labels; unconfirmed upcoming dates stay unconfirmed. A source update timestamp is not proof that every field was recently verified.

Historical winter rows record first and last dates and their inclusive calendar span. They do not establish uninterrupted daily operation. Separate access-base records, or conflicting source municipality rows, can share one resort URL; these remain separate rows and joins report ambiguity. Rows whose published dates fall outside the selected winter stay visible with `dates_outside_requested_winter`; planners count them as inconsistent evidence and never confirm a window from them. Town portfolios keep missing, ambiguous and inconsistent endpoint counts separate and do not sum shared terrain.

Dated report numbers describe the reporter's base/town. New snow means since the previous report, which may be more than 24 hours. Missing figures stay null; published zero stays zero. Report prose is not reproduced. October 2026 observations are preseason measurements.

Nationwide discovery supports resort name, municipality, prefecture and numeric facts. Popular-region membership is unavailable in the replayable source, so it is not a search filter. No forecasts, reservations, lift-ticket sales or safety assessments are provided. Requests, response bodies and scanned records are bounded; result limits are 1–200. HTTP and schema failures return errors, and network fallback is explicitly labeled as dated local data.

## Recipes

### Bound the shortlist

```bash
snowjapan-pp-cli resorts search --prefecture Nagano --limit 10 --agent --select results.name,results.vertical_m,results.source_url
```

Request a small factual projection.

### Inspect a dated observation

```bash
snowjapan-pp-cli reports get hakuba-now-1st-october-2026 --agent
```

Snowfall is measured at base/town since the prior report.

### Audit historical gaps

```bash
snowjapan-pp-cli plan coverage --season 2025-2026 --prefecture Nagano --limit 20 --agent
```

After syncing the directory and requested winter, missing and ambiguous endpoint records stay distinct; neither establishes closure.

### Compare past date boundaries

```bash
snowjapan-pp-cli plan windows --season 2025-2026 --from 2026-03-28 --to 2026-04-05 --resorts able-hakuba-goryu --agent
```

An overlap is a calendar span, with continuous operation unknown.

## Unique Features

These local computations use explicitly saved SnowJapan facts.

### Resort decisions
- **`plan frontier`** — Show nondominated resort choices for explicitly selected statistics.

  _When a traveler wants to expose numeric tradeoffs across a shortlist._

  ```bash
  snowjapan-pp-cli plan frontier --prefecture Nagano --maximize vertical,courses,longest --limit 10 --agent
  ```
- **`plan towns`** — Compare source-defined towns by known resort options and statistical ranges.

  _When deciding which municipalities warrant further resort inspection._

  ```bash
  snowjapan-pp-cli plan towns --prefecture Nagano --season 2025-2026 --limit 10 --agent
  ```

### Historical evidence
- **`plan windows`** — Intersect a past trip window with recorded first/last season spans.

  _When exact historical date boundaries matter._

  ```bash
  snowjapan-pp-cli plan windows --season 2025-2026 --from 2026-03-28 --to 2026-04-05 --resorts able-hakuba-goryu --agent
  ```
- **`plan coverage`** — Find listed resorts with missing or ambiguous historical endpoint evidence.

  _When plans need explicit unknowns for a changed shortlist._

  ```bash
  snowjapan-pp-cli plan coverage --season 2025-2026 --prefecture Nagano --limit 20 --agent
  ```

### Saved observations
- **`plan changes`** — See factual field changes between the latest two locally captured observations.

  _After an explicit capture or sync, to inspect source edits without continuous monitoring._

  ```bash
  snowjapan-pp-cli plan changes --resorts able-hakuba-goryu --agent
  ```

## Usage

Run `snowjapan-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.json` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SNOWJAPAN_CONFIG_DIR`, `SNOWJAPAN_DATA_DIR`, `SNOWJAPAN_STATE_DIR`, or `SNOWJAPAN_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SNOWJAPAN_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SNOWJAPAN_HOME=/srv/snowjapan
snowjapan-pp-cli doctor
```

Under `SNOWJAPAN_HOME=/srv/snowjapan`, the four dirs resolve to `/srv/snowjapan/config`, `/srv/snowjapan/data`, `/srv/snowjapan/state`, and `/srv/snowjapan/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "snowjapan": {
      "command": "snowjapan-pp-mcp",
      "env": {
        "SNOWJAPAN_HOME": "/srv/snowjapan"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SNOWJAPAN_DATA_DIR` overrides an explicit `--home` for that kind. Use `SNOWJAPAN_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SNOWJAPAN_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `snowjapan-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### reports

Dated regional snow observations at reporter base/town level and canonical report links.

- **`snowjapan-pp-cli reports get`** - Read one dated report's base-level snow figures; new snow means since the previous report.
- **`snowjapan-pp-cli reports list`** - List latest regional report metadata from the public homepage without report narratives.

### resorts

Factual active-area directory and individual resort records; installed lifts are not current lift operations.

- **`snowjapan-pp-cli resorts get`** - Inspect one exact canonical resort path with Japanese name, ability, lift and update-status facts.
- **`snowjapan-pp-cli resorts list`** - List source-owned national factual chart records through the bounded chart adapter.
- **`snowjapan-pp-cli resorts search`** - Filter factual names, towns, prefectures, vertical and installed-lift counts.
- **`snowjapan-pp-cli resorts compare <first> <second> [third] [fourth]`** - Compare two to four exact resort IDs or unique slugs.
- **`snowjapan-pp-cli sync`** - Explicitly save factual resources and compatible observation history.
- **`snowjapan-pp-cli search <term>`** - Search saved facts locally by resource type.

### seasons

Confirmed first and last dates of completed winters, not continuous operation or upcoming forecasts.

- **`snowjapan-pp-cli seasons list`** - List recorded historical season chart rows through the bounded chart adapter.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`snowjapan-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`snowjapan-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`snowjapan-pp-cli learnings list`** - Inspect taught rows
- **`snowjapan-pp-cli learnings forget <query>`** - Undo a teach
- **`snowjapan-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`snowjapan-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`snowjapan-pp-cli teach-pattern`** - Install a query/resource template up front
- **`snowjapan-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SNOWJAPAN_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `snowjapan-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
snowjapan-pp-cli reports list

# JSON for scripting and agents
snowjapan-pp-cli reports list --json
# Filter to specific fields
snowjapan-pp-cli reports list --json --select results.id,results.region,results.report_date

# Dry run — show the request without sending
snowjapan-pp-cli reports list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
snowjapan-pp-cli reports list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - use `--agent` for unattended JSON output; exact IDs are positional on get and compare commands
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - explicit sync captures source facts; search and planners read those saved facts locally
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
snowjapan-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `snowjapan-pp-cli doctor` to see the resolved config, data, state, and cache directories. Use `--home`, `SNOWJAPAN_HOME`, and the documented per-kind environment variables to relocate state.

Factual source commands use a fixed public source adapter and a maximum 2 requests per second; `--rate-limit` can lower that rate. They do not use generated static header configuration.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Local planning output is empty** — Run snowjapan-pp-cli sync --resources resorts, then sync seasons with --resource-param seasons:season=2025-2026.
- **Source fields or chart columns changed** — Use the canonical source_url to inspect the provider page and update the parser; no empty success is returned on schema drift.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.snowjapan.com/ski-areas-in-japan/quick-search
- Capture coverage: 6 API entries from 6 total network entries
- Reachability: standard_http (100% confidence)
- Protocols: ssr_embedded_data (100% confidence)
- Auth signals: none
- Generation hints: Direct HTTP runtime; custom factual chart projection required; no browser transport or credentials.

Warnings from discovery:
- custom_html_projection: Source HTML/JS-shaped data requires a bounded non-executing parser; ordinary generated HTML metadata is insufficient.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
