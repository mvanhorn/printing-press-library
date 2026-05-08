---
name: pp-ted-search
description: "The entire EU public procurement corpus — €815B/year — searchable offline, with B2B lead generation for... Trigger phrases: `find EU procurement contracts`, `search TED tenders`, `who wins EU IT contracts in Germany`, `open tenders expiring soon`, `public procurement analysis`, `EU contract opportunities`, `use eu-ted`, `run eu-ted-pp-cli`."
author: "Mathias Michel"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ted-search-pp-cli
    install:
      - kind: go
        bins: [ted-search-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/ted-search/cmd/ted-search-pp-cli
---

# TED (Tenders Electronic Daily) — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ted-search-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install ted-search --cli-only
   ```
2. Verify: `ted-search-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/ted-search/cmd/ted-search-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

TED publishes every significant EU public contract (€676K notices/year) but the web interface makes analysis nearly impossible. This CLI syncs the corpus to SQLite, then layers market intelligence nobody else offers: win rates, concentration scores, dark-buyer detection, and opportunity scoring — free, composable, agent-native.

## When to Use This CLI

Reach for eu-ted-pp-cli when you need to analyze EU public procurement at scale: scanning for bid opportunities, researching buyer behavior, studying market concentration, or building procurement data pipelines. It is the right tool when you need composable, scriptable access to TED data without paying €500+/month for a SaaS portal. Not the right choice for non-EU procurement (use SAM.gov or national portals instead).

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Lead generation
- **`leads`** — Surface recent construction contract award winners as B2B outreach candidates — company name, project location, contract value, and construction type — so you can contact winners who need construction machinery.

  _Reach for this when prospecting construction companies for B2B outreach — it identifies firms that just won large projects and will need equipment, subcontractors, and services in the near term._

  ```bash
  ted-search-pp-cli leads --cpv 45 --country DEU --region NW --min-value 500000 --days 30 --agent --select leads.winner_name,leads.project_location,leads.contract_value,leads.contact_url
  ```

### Market intelligence
- **`win-rate`** — See what fraction of contract competitions in a market go to new winners vs. incumbents — your real odds before writing a proposal.

  _Reach for this when deciding whether a procurement market is worth entering — high repeat-winner rate means the contract is effectively pre-determined._

  ```bash
  ted-search-pp-cli win-rate --cpv 72000000 --country FRA --min-calls 5 --show-winners --agent
  ```
- **`concentration`** — Compute which companies capture what share of awarded contract value in a sector and country, with HHI score and year-over-year trend.

  _Reach for this when researching market structure, competitive dynamics, or potential regulatory concentration issues in EU procurement._

  ```bash
  ted-search-pp-cli concentration --cpv 72000000 --country DEU --top 5 --since 2022-01-01 --agent
  ```
- **`velocity`** — See whether a procurement market is heating up, cooling off, or spiking — weekly notice count trends over rolling windows vs. same period last year.

  _Use this to spot demand surges in your sector before competitors react — especially useful after policy announcements or budget cycles._

  ```bash
  ted-search-pp-cli velocity --country DEU --cpv 72000000 --window 90d --compare 1y --agent
  ```
- **`cpv-drift`** — See which procurement categories are growing or shrinking in a country's spending mix year-over-year — essential for platform builders and policy researchers.

  _Use this to identify which procurement markets are expanding and worth investing in — particularly useful after major policy shifts like digital transformation mandates._

  ```bash
  ted-search-pp-cli cpv-drift --country DEU --since 2020-01-01 --top 20 --metric value --agent
  ```

### Bid intelligence
- **`score`** — Get a ranked shortlist of open tenders scored by deadline urgency, contract value, keyword fit, and market openness — your morning briefing, prioritized.

  _Use this when you need to decide which tenders deserve proposal effort today — it surfaces fit + urgency + openness in a single ranked list._

  ```bash
  ted-search-pp-cli score --keywords "cloud migration" --country DEU --cpv 72 --min-value 500000 --max-days 30 --agent
  ```
- **`buyer`** — Build a full procurement dossier on any contracting authority: their spending cadence, CPV mix, typical contract values, and repeat winner patterns.

  _Reach for this when preparing a bid to a specific buyer — understand their preferences, typical timelines, and incumbent relationships before writing._

  ```bash
  ted-search-pp-cli buyer --name "Bundesagentur für Arbeit" --since 2020-01-01 --show-winners --agent
  ```
- **`deadline-heat`** — A ranked calendar of expiring tenders weighted by urgency × value / competition density — your daily prioritized view of what needs attention now.

  _Use this as a daily morning briefing command — it surfaces high-urgency, high-value, low-competition opportunities that a simple deadline sort misses._

  ```bash
  ted-search-pp-cli deadline-heat --country DEU --cpv 72 --days 14 --min-value 200000 --agent
  ```

### Compliance & integrity
- **`dark-buyers`** — Surface contracting authorities whose calls-for-tender rarely produce public awards, or whose awards show suspiciously low winner diversity — a compliance and integrity signal.

  _Reach for this when investigating procurement compliance, building risk scores for contracting authorities, or preparing investigative journalism._

  ```bash
  ted-search-pp-cli dark-buyers --country POL --cpv 45000000 --since 2022-01-01 --min-calls 3 --agent
  ```

## Command Reference

**notices** — Manage notices

- `ted-search-pp-cli notices` — Search for notices using expert search query. More information about the query format and field names can be found...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ted-search-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Construction machinery lead generation (klarx use case)

```bash
eu-ted-pp-cli leads --cpv 45 --country DEU --region NW --min-value 500000 --days 30 --agent --select leads.winner_name,leads.project_location,leads.contract_value,leads.construction_type,leads.contact_url
```

Find recent construction contract award winners in German North Rhine-Westphalia — companies that just won large projects and will need cranes, containers, and site equipment.

### Morning bid opportunity briefing

```bash
eu-ted-pp-cli deadline-heat --country DEU --cpv 72 --days 14 --min-value 200000 --agent --select notices.score,notices.title,notices.buyer_name,notices.deadline,notices.estimated_value
```

Run daily to surface the highest-urgency, highest-value open tenders in German IT.

### Is this market worth entering?

```bash
eu-ted-pp-cli win-rate --cpv 72000000 --country FRA --since 2022-01-01 --show-winners --agent --select market.repeat_rate,market.unique_winner_count,market.top_winners
```

Check repeat-winner rate before investing proposal effort in French IT services.

### Competitor intelligence

```bash
eu-ted-pp-cli concentration --cpv 72000000 --country DEU --top 5 --since 2022-01-01 --agent --select winners.name,winners.share,winners.total_value
```

See which firms dominate German IT procurement and their market share.

### Buyer deep-dive before a bid

```bash
eu-ted-pp-cli buyer --name "Bundesagentur für Arbeit" --since 2020-01-01 --show-winners --agent --select profile.cpv_mix,profile.avg_value,profile.repeat_winner_rate,profile.avg_call_to_award_days
```

Build a full dossier on a buyer before writing a proposal — understand their preferences and incumbent relationships.

### Sync + SQL analysis

```bash
eu-ted-pp-cli sync --country DEU --cpv 72000000 --since 2023-01-01 && eu-ted-pp-cli sql "SELECT buyer_name, COUNT(*) AS calls, AVG(estimated_value) AS avg_value FROM notices WHERE notice_type='cn-standard' GROUP BY buyer_name ORDER BY calls DESC LIMIT 20"
```

Pull data locally then run arbitrary SQL — works offline, composable with duckdb or pandas.

## Auth Setup

No authentication required.

Run `ted-search-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ted-search-pp-cli notices --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
ted-search-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ted-search-pp-cli feedback --stdin < notes.txt
ted-search-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.ted-search-pp-cli/feedback.jsonl`. They are never POSTed unless `TED_SEARCH_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TED_SEARCH_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ted-search-pp-cli profile save briefing --json
ted-search-pp-cli --profile briefing notices
ted-search-pp-cli profile list --json
ted-search-pp-cli profile show briefing
ted-search-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ted-search-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/ted-search/cmd/ted-search-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add ted-search-pp-mcp -- ted-search-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ted-search-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ted-search-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ted-search-pp-cli <command> --help`.
