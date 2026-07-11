# FPI India CLI

**The full history of Foreign Portfolio Investor flows into India, synced locally and queryable offline — something neither NSDL nor CDSL offers today.**

NSDL and CDSL publish authoritative FPI investment data, but only as ASP.NET report pages with a client-side Excel export button — no API, no bulk historical download, no scriptable access. fpi-india syncs the full net-investment series back to 1992-93, plus AUC, sector rotation, trade data, and the FPI registry, into a local SQLite database you can query, trend, and pipe.

## Install

The recommended path installs both the `fpi-india-pp-cli` binary and the `pp-fpi-india` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install fpi-india
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install fpi-india --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install fpi-india --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install fpi-india --agent claude-code
npx -y @mvanhorn/printing-press-library install fpi-india --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/fpi-india/cmd/fpi-india-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fpi-india-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install fpi-india --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-fpi-india --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-fpi-india --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install fpi-india --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fpi-india-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/fpi-india/cmd/fpi-india-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "fpi-india": {
      "command": "fpi-india-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the CLI can reach NSDL before syncing
fpi-india-pp-cli doctor --dry-run

# pull the full historical FY series (1992-93-to-present) into local SQLite
fpi-india-pp-cli sync --resources net_investment --full

# see the synced historical equity flow series
fpi-india-pp-cli net-investment fy --json

# compare this year's flow against last year, computed locally
fpi-india-pp-cli net-investment yoy --asset equity --year 2024 --json

# see the current buying/selling streak, a computation no source page offers
fpi-india-pp-cli net-investment streaks --asset equity --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`net-investment yoy`** — See this year's FPI net flow next to the same period last year, delta computed automatically.

  _Reach for this when a request compares FPI flow across two adjacent years instead of listing raw rows._

  ```bash
  fpi-india-pp-cli net-investment yoy --asset equity --year 2024 --json
  ```
- **`sector rotation`** — Rank which sectors saw the biggest FPI inflow/outflow swing between the two most recent synced periods.

  _Use this for 'what sector are FPIs rotating into/out of' instead of a single-period sector snapshot._

  ```bash
  fpi-india-pp-cli sector rotation --top 5 --period latest --json
  ```
- **`limits utilization`** — See what percentage of the regulatory FPI investment cap a sector or debt category has used.

  _Use this to flag sectors approaching a regulatory FPI cap, which can trigger a trading halt._

  ```bash
  fpi-india-pp-cli limits utilization --sector Banking --json
  ```
- **`net-investment streaks`** — See how many consecutive fortnights/years FPIs have been net buyers or sellers, and the most recent flip.

  _Use this for 'how many periods in a row' questions instead of scanning raw historical rows._

  ```bash
  fpi-india-pp-cli net-investment streaks --asset equity --json
  ```
- **`auc trend`** — Track how a country's or category's share of assets under custody has changed across synced snapshots.

  _Use this once at least two snapshots are synced to see custody-share direction, not a single-point value._

  ```bash
  fpi-india-pp-cli auc trend --by country --country Mauritius --json
  ```
- **`net-investment extremes`** — Find the largest single-period FPI inflows or outflows across the full 1992-93-to-present series.

  _Use this for 'biggest ever' / superlative questions about historical FPI flow._

  ```bash
  fpi-india-pp-cli net-investment extremes --asset equity --top 10 --json
  ```
- **`net-investment trend`** — See the rolling growth-rate and direction of FPI net flow over a window of recent periods.

  _Use this for a derived growth-rate/direction view, not raw historical rows (use 'net-investment fy'/'cy'/'quarterly'/'latest' for those)._

  ```bash
  fpi-india-pp-cli net-investment trend --asset equity --period fy --window 12 --json
  ```

### Reachability mitigation
- **`cdsl diff`** — Compare NSDL's and CDSL's figures for the same date and flag any mismatch.

  _Use this to cross-check the two Indian depositories' published figures for the same date._

  ```bash
  fpi-india-pp-cli cdsl diff --date 09072026 --json
  ```

## Recipes

### Full historical equity flow series

```bash
fpi-india-pp-cli net-investment fy --json --select period,total,cumulative_total
```

Narrow a 30+ year series down to just the fields needed for a chart or note.

### This fortnight vs last year, same fortnight

```bash
fpi-india-pp-cli net-investment yoy --asset equity --year 2024
```

The Monday-morning commentary workflow: one command instead of loading two pages and subtracting by hand.

### What sector is money rotating into

```bash
fpi-india-pp-cli sector rotation --top 5 --period latest
```

Ranks period-over-period sector deltas instead of a single period's absolute figures.

### Is any sector near its FPI investment cap

```bash
fpi-india-pp-cli limits utilization --sector Banking
```

Joins synced limits against synced investment totals to compute percent-of-cap used.

### Biggest FPI outflow ever recorded

```bash
fpi-india-pp-cli net-investment extremes --asset equity --top 10
```

Deterministic ranking over the full synced history for journalist-style superlative questions.

## Usage

Run `fpi-india-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `FPI_INDIA_CONFIG_DIR`, `FPI_INDIA_DATA_DIR`, `FPI_INDIA_STATE_DIR`, or `FPI_INDIA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `FPI_INDIA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export FPI_INDIA_HOME=/srv/fpi-india
fpi-india-pp-cli doctor
```

Under `FPI_INDIA_HOME=/srv/fpi-india`, the four dirs resolve to `/srv/fpi-india/config`, `/srv/fpi-india/data`, `/srv/fpi-india/state`, and `/srv/fpi-india/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "fpi-india": {
      "command": "fpi-india-pp-mcp",
      "env": {
        "FPI_INDIA_HOME": "/srv/fpi-india"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `FPI_INDIA_DATA_DIR` overrides an explicit `--home` for that kind. Use `FPI_INDIA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `FPI_INDIA_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `fpi-india-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### auc

FPI assets under custody (AUC), by country and by category

- **`fpi-india-pp-cli auc category`** - FPI AUC category-wise data
- **`fpi-india-pp-cli auc country`** - FPI AUC country-wise data (top 10 countries)

### cdsl_reports

CDSL daily FPI snapshot publication index and dated XLS snapshot download

- **`fpi-india-pp-cli cdsl-reports listing`** - Index page listing recent dated FPI snapshot XLS files
- **`fpi-india-pp-cli cdsl-reports snapshot`** - Dated daily FPI snapshot file (date format DDMMYYYY, e.g. 09072026)

### limits

Per-company (ISIN) foreign investment limits: NRI limit %, FPI limit %, sectoral cap %, and current monitored FPI holding count

- **`fpi-india-pp-cli limits`** - Per-ISIN foreign investment limit monitoring data

### net_investment

Historical FPI net-investment flows by asset class (equity, debt, hybrid, mutual funds, AIF), financial year and calendar year

- **`fpi-india-pp-cli net-investment cy`** - Calendar-year FPI net-investment series, with monthly breakdown when a specific year is requested
- **`fpi-india-pp-cli net-investment fy`** - Full financial-year FPI net-investment series (1992-93 to present)
- **`fpi-india-pp-cli net-investment latest`** - Latest fortnight FPI net-investment snapshot
- **`fpi-india-pp-cli net-investment quarterly`** - Quarterly FPI net-investment details (calendar year)

### registry

Registered FPI list, registration category counts, and DDP pendency

- **`fpi-india-pp-cli registry categories`** - FPI category-wise registration counts
- **`fpi-india-pp-cli registry list`** - List of registered FPIs/FIIs
- **`fpi-india-pp-cli registry pendency`** - DDP-wise pendency of FPI applications

### sector

Fortnightly sector-wise FPI investment data

- **`fpi-india-pp-cli sector`** - Fortnightly sector-wise FPI investment activity

### trades

Trade-wise FPI equity and debt data

- **`fpi-india-pp-cli trades debt`** - Trade-wise debt data of FPI
- **`fpi-india-pp-cli trades equity`** - Trade-wise equity data of FPI


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`fpi-india-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`fpi-india-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`fpi-india-pp-cli learnings list`** - Inspect taught rows
- **`fpi-india-pp-cli learnings forget <query>`** - Undo a teach
- **`fpi-india-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`fpi-india-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`fpi-india-pp-cli teach-pattern`** - Install a query/resource template up front
- **`fpi-india-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `FPI_INDIA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `fpi-india-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
fpi-india-pp-cli limits --action GetFilmReport_Detail --u-name example-resource

# JSON for scripting and agents
fpi-india-pp-cli limits --action GetFilmReport_Detail --u-name example-resource --json

# Filter to specific fields
fpi-india-pp-cli limits --action GetFilmReport_Detail --u-name example-resource --json --select id,name,status

# Dry run — show the request without sending
fpi-india-pp-cli limits --action GetFilmReport_Detail --u-name example-resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
fpi-india-pp-cli limits --action GetFilmReport_Detail --u-name example-resource --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
fpi-india-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `fpi-india-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `FPI_INDIA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **sync returns 0 rows for a report** — NSDL occasionally changes a report's table id after a site update; run `fpi-india-pp-cli doctor --json` to confirm the page is reachable, then check for a generator update.
- **`cdsl-reports snapshot` returns stale or missing dates** — that command downloads CDSL's legacy rolling-window daily .xls file; older dates will not be available from it even after sync. For real granular daily data (parsed, not a binary download), use `net-investment latest`/`monthly`/`archive` (NSDL) or `cdsl-reports daily`/`monthly` (CDSL) instead — `archive` reaches arbitrary historical dates, the others only the current period. See Known Gaps below.

## Known Gaps

Disclosed, deliberate scope reductions — genuine engineering-surface limits, not corner-cutting:

- **`registry list`** (raw FPI/FII name list) — `RegisteredFIISAFPI.aspx` is a search form requiring an ASP.NET WebForms VIEWSTATE/EVENTVALIDATION POST replay, a different engineering surface from every other GET-based report this CLI uses. Not synced; the command remains present but returns live-fetch results only. `registry categories` and `registry pendency` are unaffected (plain GET). (`net-investment archive` shows this same class of VIEWSTATE POST form is scriptable when the target page needs it — `registry list`'s form has more fields and validation to reverse-engineer, which is why it's still a disclosed gap rather than done.)
- **`cdsl-reports snapshot`** — CDSL's dated snapshot files (`/downloads/Publications/Latest/latest_{date}.xls`) are legacy binary XLS (OLE2 Compound Document format), not HTML or JSON, so this command downloads the raw file rather than parsing it. Real parsed daily/monthly CDSL data is available via `cdsl-reports daily`/`monthly` instead.
- **`trades equity`/`trades debt`** column naming — the source table has 3+ levels of header nesting (year groups × month sub-columns); the generic composite-name heuristic produces functional but imperfectly-labeled columns for this specific report.
