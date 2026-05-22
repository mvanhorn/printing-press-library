---
name: pp-american-reindustrialization
description: "Browse, slice, and analyze the curated American Reindustrialization directory — with diffs, geo clusters, sector... Trigger phrases: `look up american reindustrialization companies`, `find reindustrialization jobs`, `browse american manufacturing directory`, `sector heatmap of reindustrialization companies`, `use american-reindustrialization`, `run american-reindustrialization`."
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - american-reindustrialization-pp-cli
---

# American Reindustrialization — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `american-reindustrialization-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install american-reindustrialization --cli-only
   ```
2. Verify: `american-reindustrialization-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/american-reindustrialization/cmd/american-reindustrialization-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI when an agent or analyst needs to slice, join, or analyze the americanreindustrialization.com company directory and jobs board beyond what the website's filter UI exposes. The local SQLite store enables composed cross-resource filters (jobs at companies with property X), aggregate analytics (sector × state heatmaps, funding × sector crosstabs, salary distributions), and diff over time (whats-new since a prior sync). Pair `--select` with `--json` whenever an agent only needs a subset of the ~30 fields per company.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**categories** — Top-level sectors (hierarchical via parent_id)

- `american-reindustrialization-pp-cli categories counts` — Map of category_id -> company count
- `american-reindustrialization-pp-cli categories get` — Get one category by slug
- `american-reindustrialization-pp-cli categories list` — List every category (bare array)
- `american-reindustrialization-pp-cli categories search` — Search categories by query string

**companies** — US-based companies driving reindustrialization (manufacturing, robotics, advanced materials, supply chains)

- `american-reindustrialization-pp-cli companies get` — Get one company by slug (returns the full object directly, not wrapped)
- `american-reindustrialization-pp-cli companies list` — List companies with pagination and optional server-side filters
- `american-reindustrialization-pp-cli companies search` — Full-text search across company names and descriptions; returns a bare array

**news** — News feed (currently empty upstream; reserved for future API population)

- `american-reindustrialization-pp-cli news` — List news items (returns a bare array; empty at capture)

**openings** — Open job listings aggregated from companies in the directory

- `american-reindustrialization-pp-cli openings categories` — Autocomplete list of categories that have at least one opening
- `american-reindustrialization-pp-cli openings companies` — Autocomplete list of {id, name} for companies that currently have open openings
- `american-reindustrialization-pp-cli openings get` — Get one job opening by slug
- `american-reindustrialization-pp-cli openings list` — List job openings with pagination and optional server-side filters (work_mode and experience_level are honored;...
- `american-reindustrialization-pp-cli openings tags` — Autocomplete list of tags that appear on at least one opening
- `american-reindustrialization-pp-cli openings titles` — Autocomplete list of job titles matching the query

**tags** — Typed tags (tag_type = tech, sector, focus area, etc.)

- `american-reindustrialization-pp-cli tags counts` — Map of tag_id -> company count
- `american-reindustrialization-pp-cli tags get` — Get one tag by slug
- `american-reindustrialization-pp-cli tags list` — List every tag (bare array)
- `american-reindustrialization-pp-cli tags search` — Search tags by query string


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
american-reindustrialization-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Where are senior remote robotics roles paying well?

```bash
american-reindustrialization-pp-cli openings find --work-mode remote --experience senior --sector robotics --salary-min 150000 --json --select title,company.name,location_display,salary_min,salary_max,apply_url
```

Composed filter + field pruning — agent gets a tight shortlist payload instead of 25 paginated full-body responses.

### Weekly investor sweep

```bash
american-reindustrialization-pp-cli whats-new --since 2026-05-12 --json --select kind,slug,name,primary_sector,funding_stage,hq_state
```

Diff-since-last-sync limited to the deal-flow fields; one command replaces a Monday morning eyeball-the-list session.

### Sector geography map

```bash
american-reindustrialization-pp-cli analytics geo-clusters --radius-km 75 --csv
```

Grid-bucket every company by lat/lon at 75km cells, emit centroid + members + dominant sector as CSV for downstream mapping.

### Capital map across the directory

```bash
american-reindustrialization-pp-cli analytics funding-by-sector --json
```

Crosstab of funding_stage × primary_sector — answers 'where is capital concentrating' in one local query.

## Auth Setup

No authentication required.

Run `american-reindustrialization-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  american-reindustrialization-pp-cli categories list --agent --select id,name,status
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

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
american-reindustrialization-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
american-reindustrialization-pp-cli feedback --stdin < notes.txt
american-reindustrialization-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.american-reindustrialization-pp-cli/feedback.jsonl`. They are never POSTed unless `AMERICAN_REINDUSTRIALIZATION_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AMERICAN_REINDUSTRIALIZATION_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
american-reindustrialization-pp-cli profile save briefing --json
american-reindustrialization-pp-cli --profile briefing categories list
american-reindustrialization-pp-cli profile list --json
american-reindustrialization-pp-cli profile show briefing
american-reindustrialization-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `american-reindustrialization-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add american-reindustrialization-pp-mcp -- american-reindustrialization-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which american-reindustrialization-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   american-reindustrialization-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `american-reindustrialization-pp-cli <command> --help`.
