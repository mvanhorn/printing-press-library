---
name: pp-tenderned
description: "Every Dutch public tender, with the sub-threshold long tail that EU TED never sees, in a local-first CLI you can pipe. Trigger phrases: `find dutch tenders`, `nederlandse aanbestedingen`, `search tenderned`, `dutch procurement`, `sub-threshold tenders nederland`, `use tenderned`, `run tenderned`."
author: "markvandeven"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tenderned-pp-cli
---

# TenderNed — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tenderned-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install tenderned --cli-only
   ```
2. Verify: `tenderned-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/tenderned/cmd/tenderned-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Pick this CLI when the user is working in the Dutch procurement market and needs sub-threshold coverage, document-corpus search, per-buyer analytics, or anything that requires holding more than one publication in working memory at once. For above-threshold EU-wide queries (TED publication numbers, cross-country buyer dossiers), pair it with the eu-tenders CLI.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local aggregations that beat the API
- **`buyer dossier`** — Full profile for one contracting authority: spending cadence, top CPVs, top procedures, active vs awarded vs cancelled counts over a window.

  _Reach for this when a user wants the full procurement picture of one buyer in one call instead of paginating through their notice list._

  ```bash
  tenderned-pp-cli buyer dossier Rotterdam --since 2025-01-01 --agent
  ```
- **`leads`** — Filter notices to national-only AND below an estimated-value cap AND inside CPV stripes — the long-tail tenders that EU TED never sees.

  _Reach for this when serving a Dutch SME or municipality-focused supplier whose target contracts are below EU thresholds._

  ```bash
  tenderned-pp-cli leads --national --max-value 200000 --cpv 45000000-7,71000000-8 --since 2026-04-01 --agent
  ```
- **`deadline`** — Lists notices closing within N days, grouped by day, with countdown plus buyer plus procedure filters.

  _Reach for this when triaging which open tenders need a response this week. Mirrors eu-tenders deadline._

  ```bash
  tenderned-pp-cli deadline --within 14 --cpv 45000000-7 --agent
  ```
- **`buyers top`** — Top contracting authorities by tender count for a CPV plus NUTS plus date slice.

  _Reach for this when the user asks 'which buyers are most active in this slice?'_

  ```bash
  tenderned-pp-cli buyers top --cpv 72000000-5 --nuts NL33 --since 2025-01-01 --agent
  ```
- **`concentration`** — Herfindahl-Hirschman index of award concentration for a CPV plus NUTS slice; tells you whether a market is dominated by one buyer or fragmented.

  _Reach for this when assessing whether to bid in a market dominated by a few incumbents. Mirrors eu-tenders concentration._

  ```bash
  tenderned-pp-cli concentration --cpv 72000000-5 --nuts NL --since 2024-01-01 --agent
  ```
- **`deadline-heat`** — Ranked calendar of expiring notices weighted by urgency times estimated value; daily prioritized view of what needs attention.

  _Reach for this when sequencing bid effort across many open deadlines. Mirrors eu-tenders deadline-heat._

  ```bash
  tenderned-pp-cli deadline-heat --days 14 --cpv 45000000-7 --agent
  ```

### Workflow primitives
- **`watch`** — Persist a filter set; watch run returns only NEW notices since the cursor's last advance.

  _Reach for this when a user wants a recurring filter without an external alerting system._

  ```bash
  tenderned-pp-cli watch run civils-watch --agent
  ```

### Time-series intelligence
- **`cpv-drift`** — Year-by-year top-CPV table for one buyer or NUTS region; shows whether a buyer's purchasing mix is shifting.

  _Reach for this when answering 'is this buyer's spending mix changing?' for a market-trend analysis. Mirrors eu-tenders cpv-drift._

  ```bash
  tenderned-pp-cli cpv-drift --buyer Amsterdam --years 3 --agent
  ```
- **`velocity`** — Weekly publication-count trend over the local snapshot; spot heating or cooling markets per CPV or buyer.

  _Reach for this when sizing market momentum for one CPV or buyer. Mirrors eu-tenders velocity._

  ```bash
  tenderned-pp-cli velocity --cpv 72000000-5 --weeks 12 --agent
  ```

### Cross-system bridges
- **`ted-link`** — Extracts the canonical TED publication number from the eForms XML, prints it plus the TED URL.

  _Reach for this when the user wants to pivot from a TenderNed notice to the EU-wide TED record (e.g., for use with the sibling eu-tenders CLI)._

  ```bash
  tenderned-pp-cli ted-link 425283 --agent
  ```
- **`thread reconcile`** — Walks PIN to CN to CAN to Modification chains for one buyer; flags orphan PINs, orphan CNs, and unresolved modifications.

  _Reach for this when a compliance officer is auditing whether announced tenders were actually awarded or cancelled._

  ```bash
  tenderned-pp-cli thread reconcile --buyer Eindhoven --since 2024-01-01 --agent
  ```

### Document corpus tools
- **`docs grep`** — Streams documents from matching notices, runs a regex against PDF and Word text, prints pub_id, doc_id, line hits.

  _Reach for this when the user wants to find every notice mentioning a clause, vendor, or technical requirement across many bestek documents._

  ```bash
  tenderned-pp-cli docs grep "clausule.*aansprakelijkheid" --cpv 45000000-7 --limit 50 --agent
  ```

## Command Reference

**buyers** — Browse contracting authorities (aanbestedende diensten) — Dutch public buyers

- `tenderned-pp-cli buyers get` — Fetch one contracting authority by ID
- `tenderned-pp-cli buyers list` — List Dutch contracting authorities (paginated)

**docs** — List and download tender documents (bestek, PvE, evaluation criteria, Q&A)

- `tenderned-pp-cli docs download` — Download all documents for one publication as a zip archive
- `tenderned-pp-cli docs get` — Download a single document's binary content (PDF/Word/etc.)
- `tenderned-pp-cli docs list` — List attached documents for one publication

**notices** — Search, list and fetch tender notices (aankondigingen) from TenderNed — mirrors 'eu-tenders notices' for the Dutch market

- `tenderned-pp-cli notices get` — Fetch full structured metadata for one publication
- `tenderned-pp-cli notices list` — Search and list tender publications with rich filters (CPV, dates, buyer, procedure, scope)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tenderned-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily new-opportunity sweep

```bash
tenderned-pp-cli notices list --agent --select content.publicatieId,content.aanbestedingNaam,content.opdrachtgeverNaam,content.sluitingsDatum
```

Narrow the fat publication payload to just the fields a triage agent needs.

### Cross-reference a notice to EU TED

```bash
tenderned-pp-cli ted-link 425283 --agent
```

Extract the canonical TED publication number so you can pivot to the eu-tenders CLI for EU-wide analysis.

### Find every bestek mentioning a clause

```bash
tenderned-pp-cli docs grep aansprakelijkheidsclausule --limit 25 --agent
```

Bulk regex across attached tender documents; unique to a local CLI.

### Watch a saved query for new hits

```bash
tenderned-pp-cli watch add civils '{"cpv":["45000000-7"],"nationaal":true,"max_value":200000}' && tenderned-pp-cli watch run civils --agent
```

Save a filter, then return only NEW publications since the last run. The cursor lives in SQLite.

### Buyer purchasing-mix drift

```bash
tenderned-pp-cli cpv-drift --buyer Amsterdam --years 3 --agent
```

Year-bucket CPV-2-digit distribution for one buyer — answers 'is their spending mix shifting?'

## Auth Setup

No authentication required.

Run `tenderned-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tenderned-pp-cli buyers list --agent --select id,name,status
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
tenderned-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tenderned-pp-cli feedback --stdin < notes.txt
tenderned-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.tenderned-pp-cli/feedback.jsonl`. They are never POSTed unless `TENDERNED_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TENDERNED_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
tenderned-pp-cli profile save briefing --json
tenderned-pp-cli --profile briefing buyers list
tenderned-pp-cli profile list --json
tenderned-pp-cli profile show briefing
tenderned-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `tenderned-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add tenderned-pp-mcp -- tenderned-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tenderned-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tenderned-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tenderned-pp-cli <command> --help`.
