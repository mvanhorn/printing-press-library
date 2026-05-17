---
name: pp-servicetitan-salestech
description: "Every ServiceTitan Sales/Estimates feature, plus a local mirror that answers the cross-cutting questions the ST web... Trigger phrases: `stale quotes`, `close rate by business unit`, `rep leaderboard`, `days to sell`, `dismissed reasons`, `estimate audit`, `pipeline snapshot`, `rep follow-ups`, `today's call list`, `reopen estimate`, `log follow-up on estimate`, `import quote from csv`, `use servicetitan-salestech`, `run servicetitan-salestech`."
author: "Pierce"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - servicetitan-salestech-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/sales-and-crm/servicetitan-salestech/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# ServiceTitan Sales & Estimates — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `servicetitan-salestech-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install servicetitan-salestech --cli-only
   ```
2. Verify: `servicetitan-salestech-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/cmd/servicetitan-salestech-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Wraps all 13 Sales/Estimates endpoints (plus a convenience `estimates reopen`) with agent-native JSON, --select dotted paths, --csv, and typed exit codes. Then adds 14 hand-built audits and workflow commands (stale, leaderboard, close-rate, days-to-sell, dismissed-reasons, pipeline-snapshot, recent-changes, audit, find, health, sku-frequency, rep follow-ups, local follow-up logging, CSV→estimate import) that compose locally over a SQLite mirror — every question a pipeline review meeting asks, plus the daily follow-up call list and a way to ingest sheet-based quotes, in one command. The matching MCP collapses 14 endpoint tools to 2 intent tools so agents pay near-zero per-turn context tax.

## When to Use This CLI

Reach for this CLI when an agent or user needs to answer a cross-cutting estimates question that the ST web UI buries — close rates, stale quotes, days-to-sell distributions, dismissed-reason patterns, point-in-time pipeline state, or any forensic lookup of a single estimate's full history. The 13 endpoint mirrors cover write-side work too, but the headline value is the 14 local-mirror audits. If you only need to read or mutate one specific estimate by id, the absorbed `estimates get/update/sell/unsell/dismiss/reopen` commands are the right reach; if you need to know something about the whole pipeline, the `reports`, `audit`, `find`, and `health` families are.

### Anti-triggers — do NOT use this CLI for

- **Customer phone / contact lookup** — Sales/Estimates spec deliberately omits contact data. Use the sibling `servicetitan-crm-pp-cli customers get <id>` instead.
- **Job dispatch / scheduling / technician routing** — see `servicetitan-dispatch-pp-cli`.
- **Invoice and payment data** — see ServiceTitan Accounting (separate module).
- **Pricebook SKU lookup / vendor management** — see `servicetitan-pricebook-pp-cli`.
- **Customer membership / recurring service data** — see `servicetitan-memberships-pp-cli`.
- **Anything outside the Sales/Estimates surface** — estimates and their line items + status changes is the entire domain of this CLI.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Pipeline review
- **`estimates stale`** — List Open estimates older than N days, ranked by age × total $ so the biggest-dollar stuck quotes surface first.

  _Use to find quotes the sales team has let go cold. Cheaper than walking the ST web UI page-by-page._

  ```bash
  servicetitan-salestech-pp-cli estimates stale --older-than 3d --json
  ```
- **`reports rep-leaderboard`** — Per-employee close rate, average days-to-sell, and total sold $ for the chosen window.

  _Use when an agent or owner needs to compare sales reps on close performance without pivoting in Excel._

  ```bash
  servicetitan-salestech-pp-cli reports rep-leaderboard --since 2026-01-01 --json
  ```
- **`reports close-rate`** — sold/(sold+dismissed) pivoted on businessUnit, rep, or month with a configurable date window.

  _Use to answer 'what's our close rate by business unit this quarter?' in one MCP turn instead of a 400-tool ST MCP query._

  ```bash
  servicetitan-salestech-pp-cli reports close-rate --group-by businessUnit --since 90d --json
  ```
- **`reports days-to-sell`** — p50/p90 percentiles of (Sold timestamp − createdOn) per rep or business unit.

  _Use to spot reps whose long-tail close time is dragging pipeline velocity._

  ```bash
  servicetitan-salestech-pp-cli reports days-to-sell --percentiles --since 90d --json
  ```
- **`reports dismissed-reasons`** — Top-N exact-match group-by on dismissal reason strings from the status-change feed.

  _Use to see what's killing deals (price, timing, scope) so the next sales script iteration addresses it._

  ```bash
  servicetitan-salestech-pp-cli reports dismissed-reasons --since 90d --top 20 --json
  ```
- **`reports pipeline`** — Total $ Open / Sold / Dismissed reconstructed for an arbitrary past date by replaying status_changes.

  _Use to answer 'where was pipeline last Monday?' without a manual snapshot job._

  ```bash
  servicetitan-salestech-pp-cli reports pipeline --as-of 2026-05-10 --json
  ```
- **`reports sku-frequency`** — Top SKUs by appearance on sold (or dismissed) estimates in a time window.

  _Use to inform pricebook decisions — which SKUs are actually carrying sold dollars._

  ```bash
  servicetitan-salestech-pp-cli reports sku-frequency --on sold --since 90d --top 50 --json
  ```
- **`reports follow-ups`** — Per-rep open estimates from the last N hours with customerId, jobId/jobNumber, and ST web deeplinks — the daily call list for follow-up outreach.

  _Use to generate today's call list per rep. Pipe customerId into `servicetitan-crm-pp-cli customers get <id>` to enrich with phone numbers._

  ```bash
  servicetitan-salestech-pp-cli reports follow-ups --rep all --since 48h --json
  ```
- **`estimates import`** — Read a defined CSV schema and create estimates with line items in ServiceTitan, with --dry-run preview and --batch-size flow control.

  _Use to convert an Excel/Google Sheets quote into a real ServiceTitan estimate without copy-pasting field by field. Export the sheet to CSV first; XLSX and Google Sheets are documented v2 paths._

  ```bash
  servicetitan-salestech-pp-cli estimates import --csv quotes.csv --dry-run
  ```

### Forensic lookup
- **`audit estimate`** — Single estimate forensic view — header + every line item + full status timeline in one shaped output.

  _Use when a CSR needs to explain to a customer 'what happened with quote 78421' without four ST web tabs._

  ```bash
  servicetitan-salestech-pp-cli audit estimate 78421 --json
  ```
- **`audit recent-changes`** — Every estimate whose status changed in a time window with from→to + actor + UTC timestamp.

  _Use first thing in the morning to triage overnight sold/dismissed/unsold activity._

  ```bash
  servicetitan-salestech-pp-cli audit recent-changes --since 24h --json
  ```
- **`find`** — Ranked full-text search across estimate name, summary, jobNumber, and nested SKU fields with structured filters.

  _Use when the customer's phrase is the only handle you have on a quote._

  ```bash
  servicetitan-salestech-pp-cli find "well pump" --status Open --min-total 5000 --json
  ```
- **`audit follow-up`** — Log a follow-up note + optional reminder date against an estimate into the local SQLite store, then list follow-ups due by date.

  _Use to keep follow-up state next to the estimate it belongs to, so 'who needs a callback this week' is one local SQL query away._

  ```bash
  servicetitan-salestech-pp-cli audit follow-up add 78421 --note "customer wants to talk Monday" --remind 2026-05-20
  ```

### Local mirror
- **`health`** — Cross-source reconciliation — API counts vs local SQLite counts vs last sync cursor age per table.

  _Use as a pre-flight check before any audit so you know whether the local mirror is fresh enough to trust._

  ```bash
  servicetitan-salestech-pp-cli health --json
  ```

## Command Reference

**estimates** — Manage estimates

- `servicetitan-salestech-pp-cli estimates create` — Estimates_create
- `servicetitan-salestech-pp-cli estimates export-async-legacy` — Provides export feed for estimates (legacy endpoint)
- `servicetitan-salestech-pp-cli estimates get` — Estimates_get
- `servicetitan-salestech-pp-cli estimates get-items` — Estimates_get items
- `servicetitan-salestech-pp-cli estimates get-list` — Estimates_get list
- `servicetitan-salestech-pp-cli estimates update` — Estimates_update

**sales-estimates-export** — Manage sales estimates export

- `servicetitan-salestech-pp-cli sales-estimates-export <tenant>` — Provides export feed for estimates

**status** — Manage status

- `servicetitan-salestech-pp-cli status <id> <tenant>` — Get estimate status change details along with UTC timestamp.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
servicetitan-salestech-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Monday pipeline review

```bash
servicetitan-salestech-pp-cli reports rep-leaderboard --since 2026-01-01 --json | jq '.[] | select(.close_rate < 0.4)'
```

Pull every rep with a sub-40% close rate this year — flag for coaching.

### Triage overnight activity

```bash
servicetitan-salestech-pp-cli audit recent-changes --since 24h --json --select id,from_status,to_status,changed_by_id,estimate_total
```

See every estimate whose status moved in the last 24h with only the fields the dispatcher cares about; --select keeps the response under a few KB even when 200+ estimates moved.

### Find a quote by customer phrase

```bash
servicetitan-salestech-pp-cli find "submersible pump" --status Open --min-total 5000 --json
```

FTS5 across name, summary, jobNumber, and SKU fields filtered to Open quotes over $5k.

### Forensic on one estimate

```bash
servicetitan-salestech-pp-cli audit estimate 78421 --json
```

Header + every line item + full status timeline in one shaped JSON object — perfect for an agent to explain 'what happened' to a customer.

### Historical pipeline check

```bash
servicetitan-salestech-pp-cli reports pipeline --as-of 2026-04-30 --json
```

Reconstruct end-of-month pipeline totals by replaying status_changes; impossible via any single ST API call.

### Build today's rep call list (with phone enrichment)

```bash
servicetitan-salestech-pp-cli reports follow-ups --rep all --since 48h --json | jq -r '.[] | "\(.rep_name)\t\(.estimate_id)\t\(.customer_id)"' | while IFS=$'\t' read rep est cust; do phone=$(servicetitan-crm-pp-cli customers get "$cust" --json | jq -r .phoneSettings.phoneNumber); echo "$rep\t$est\t$cust\t$phone"; done
```

Pipe customerId from this CLI into the CRM sibling CLI to resolve phone numbers — the Sales/Estimates module spec deliberately doesn't include customer contact data.

### Reopen a dismissed estimate

```bash
servicetitan-salestech-pp-cli estimates reopen 78421 --dry-run
```

Wraps Estimates_Update with status=Open to reverse a dismiss. Drop --dry-run when you're ready to commit.

### Log a follow-up reminder

```bash
servicetitan-salestech-pp-cli audit follow-up add 78421 --note "customer asked us to follow up Monday" --remind 2026-05-20
```

Writes to the local SQLite store only; later `audit follow-ups --due-by 2026-05-21` surfaces it.

### Import an Excel quote (via CSV export)

```bash
servicetitan-salestech-pp-cli estimates import --csv quotes.csv --dry-run
```

Export your Excel/Sheets quote to CSV first, then dry-run to preview the Estimates_Create + line-item calls. Drop --dry-run to commit.

## Auth Setup

Composed auth — both ST_APP_KEY (apiKey header) and an OAuth2 client_credentials bearer are required on every call. Set ST_APP_KEY, ST_CLIENT_ID, ST_CLIENT_SECRET, and ST_TENANT_ID, then run `auth login` to mint the bearer. `doctor` verifies all four are present and reachable. Whitespace is stripped defensively from every env var (a known JKA gotcha that produced opaque invalid_client 400s).

Run `servicetitan-salestech-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  servicetitan-salestech-pp-cli estimates get mock-value mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
servicetitan-salestech-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
servicetitan-salestech-pp-cli feedback --stdin < notes.txt
servicetitan-salestech-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.servicetitan-salestech-pp-cli/feedback.jsonl`. They are never POSTed unless `SERVICETITAN_SALESTECH_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SERVICETITAN_SALESTECH_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
servicetitan-salestech-pp-cli profile save briefing --json
servicetitan-salestech-pp-cli --profile briefing estimates get mock-value mock-value
servicetitan-salestech-pp-cli profile list --json
servicetitan-salestech-pp-cli profile show briefing
servicetitan-salestech-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `servicetitan-salestech-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add servicetitan-salestech-pp-mcp -- servicetitan-salestech-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which servicetitan-salestech-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   servicetitan-salestech-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `servicetitan-salestech-pp-cli <command> --help`.
