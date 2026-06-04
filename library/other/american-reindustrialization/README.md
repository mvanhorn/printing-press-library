# American Reindustrialization CLI

**Browse, slice, and analyze the curated American Reindustrialization directory — with diffs, geo clusters, sector heatmaps, and offline SQL no website view shows.**

A read-only CLI for the company directory and jobs board at americanreindustrialization.com. Sync once, then run cross-entity queries (jobs at robotics companies in TX, sector × state heatmaps, funding × sector crosstabs, week-over-week diffs) entirely offline, with agent-native JSON and a local SQLite surface no view on the website exposes.

Created by [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `american-reindustrialization-pp-cli` binary and the `pp-american-reindustrialization` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install american-reindustrialization
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install american-reindustrialization --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install american-reindustrialization --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install american-reindustrialization --agent claude-code
npx -y @mvanhorn/printing-press-library install american-reindustrialization --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/american-reindustrialization/cmd/american-reindustrialization-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/american-reindustrialization-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-american-reindustrialization --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-american-reindustrialization --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install american-reindustrialization --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/american-reindustrialization-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "american-reindustrialization": {
      "command": "american-reindustrialization-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Pull every company, job, category, and tag into local SQLite; all subsequent commands run offline.
american-reindustrialization-pp-cli sync

# Cached list of California companies with full agent-native output.
american-reindustrialization-pp-cli companies list --state CA --json

# Composed multi-axis filter no website view exposes.
american-reindustrialization-pp-cli openings find --work-mode remote --experience senior --salary-min 150000

# Sector × state crosstab weighted by job openings — pure local SQL.
american-reindustrialization-pp-cli analytics sector-heatmap --weight jobs

# Diff against the prior sync snapshot — companies and jobs that appeared or changed.
american-reindustrialization-pp-cli whats-new --since 2026-05-12

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`whats-new`** — Show companies and jobs added or updated since a date you provide, so a weekly sweep takes one command instead of an eyeball-the-list session.

  _Reach for this when an analyst or agent needs week-over-week deltas across the directory; the site offers no such view._

  ```bash
  american-reindustrialization-pp-cli whats-new --since 2026-05-12 --json
  ```

### Cross-entity local queries
- **`openings find`** — Filter the jobs board by work_mode, experience_level, salary floor, state, company size, sector, and posted-since in a single query — combinations the website's UI cannot express.

  _Use this when a user wants the shortlist instead of 25 paginated pages; cross-resource filters are this CLI's reason to exist._

  ```bash
  american-reindustrialization-pp-cli openings find --work-mode remote --experience senior --salary-min 150000 --state TX --json
  ```
- **`companies top-hiring`** — Rank companies by jobs_count descending, with optional filters by sector, state, or funding_stage — the site has no ranking view.

  _Reach for this when a job seeker or recruiter wants 'who has the most openings in sector X right now'._

  ```bash
  american-reindustrialization-pp-cli companies top-hiring --sector robotics --limit 10 --json
  ```
- **`companies profile`** — Single-shot rich profile for one company: full fields plus that company's open jobs plus similar companies (same primary_sector and employee_range bucket).

  _Use this when a single named company needs full context plus peers — research, due diligence, competitor scans._

  ```bash
  american-reindustrialization-pp-cli companies profile harmony-ai --json
  ```

### Ecosystem analytics
- **`analytics sector-heatmap`** — Crosstab of primary_sector × HQ state with company counts (optionally weighted by jobs_count or filtered by funding_stage), revealing the geographic shape of each sector.

  _Pull this when answering 'where is sector X clustering' or 'which states are bidding loudest on hiring in sector Y'._

  ```bash
  american-reindustrialization-pp-cli analytics sector-heatmap --funding-stage seed --weight jobs --json
  ```
- **`analytics funding-by-sector`** — Crosstab of funding_stage × primary_sector with company counts and median employee_range, exposing where capital is concentrating.

  _Use this when an investor needs the capital map across the reindustrialization directory._

  ```bash
  american-reindustrialization-pp-cli analytics funding-by-sector --json
  ```
- **`analytics geo-clusters`** — Grid-bucket companies by lat/lon (default 50km cells) and emit cluster centroid, member count, member companies, and the dominant sector per cluster.

  _Pull this for 'which metro is the densest cluster in sector X' style questions; the site has no map view._

  ```bash
  american-reindustrialization-pp-cli analytics geo-clusters --state TX --radius-km 50 --json
  ```
- **`openings salary-stats`** — p25 / p50 / p75 of midpoint salary across filtered jobs, with null-salary count reported separately so missing data is honest.

  _Use this when a job seeker or comp analyst wants band ranges, not individual postings._

  ```bash
  american-reindustrialization-pp-cli openings salary-stats --sector robotics --experience senior --json
  ```
- **`companies cohorts`** — Bucket companies by founded_year (default 5-year buckets) with company counts and the top-3 sectors per cohort.

  _Reach for this when writing about 'companies founded since 2020 in the reindustrialization wave' or tracking ecosystem age over time._

  ```bash
  american-reindustrialization-pp-cli companies cohorts --bucket 5 --json
  ```

## Usage

Run `american-reindustrialization-pp-cli --help` for the full command reference and flag list.

## Commands

### categories

Top-level sectors (hierarchical via parent_id)

- **`american-reindustrialization-pp-cli categories counts`** - Map of category_id -> company count
- **`american-reindustrialization-pp-cli categories get`** - Get one category by slug
- **`american-reindustrialization-pp-cli categories list`** - List every category (bare array)
- **`american-reindustrialization-pp-cli categories search`** - Search categories by query string

### companies

US-based companies driving reindustrialization (manufacturing, robotics, advanced materials, supply chains)

- **`american-reindustrialization-pp-cli companies get`** - Get one company by slug (returns the full object directly, not wrapped)
- **`american-reindustrialization-pp-cli companies list`** - List companies with pagination and optional server-side filters
- **`american-reindustrialization-pp-cli companies search`** - Full-text search across company names and descriptions; returns a bare array

### news

News feed (currently empty upstream; reserved for future API population)

- **`american-reindustrialization-pp-cli news`** - List news items (returns a bare array; empty at capture)

### openings

Open job listings aggregated from companies in the directory

- **`american-reindustrialization-pp-cli openings categories`** - Autocomplete list of categories that have at least one opening
- **`american-reindustrialization-pp-cli openings companies`** - Autocomplete list of {id, name} for companies that currently have open openings
- **`american-reindustrialization-pp-cli openings get`** - Get one job opening by slug
- **`american-reindustrialization-pp-cli openings list`** - List job openings with pagination and optional server-side filters (work_mode and experience_level are honored; state is silently ignored upstream — use `openings find` for cross-resource filters)
- **`american-reindustrialization-pp-cli openings tags`** - Autocomplete list of tags that appear on at least one opening
- **`american-reindustrialization-pp-cli openings titles`** - Autocomplete list of job titles matching the query

### tags

Typed tags (tag_type = tech, sector, focus area, etc.)

- **`american-reindustrialization-pp-cli tags counts`** - Map of tag_id -> company count
- **`american-reindustrialization-pp-cli tags get`** - Get one tag by slug
- **`american-reindustrialization-pp-cli tags list`** - List every tag (bare array)
- **`american-reindustrialization-pp-cli tags search`** - Search tags by query string

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
american-reindustrialization-pp-cli categories list

# JSON for scripting and agents
american-reindustrialization-pp-cli categories list --json

# Filter to specific fields
american-reindustrialization-pp-cli categories list --json --select id,name,status

# Dry run — show the request without sending
american-reindustrialization-pp-cli categories list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
american-reindustrialization-pp-cli categories list --agent
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
american-reindustrialization-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/american-reindustrialization-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **search/sql/analytics commands return empty results** — Run `american-reindustrialization-pp-cli sync` first; every offline command reads the local store, which is empty until sync populates it.
- **`--state CA` works on companies but not on jobs** — The upstream /api/jobs silently ignores state. Use `openings find --state CA` instead — the local join filters on the joined company's state.
- **whats-new reports zero changes after sync** — The first sync establishes a baseline snapshot; run sync again later, then `whats-new --since <prior-sync-date>` will show deltas.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
