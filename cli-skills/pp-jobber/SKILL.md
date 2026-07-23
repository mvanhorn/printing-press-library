---
name: pp-jobber
description: "Read-only Jobber CLI for offline analysis — every GraphQL surface synced to SQLite, every relationship queryable. Trigger phrases: `ar aging in jobber`, `jobber invoice payments mismatch`, `search jobber data`, `snapshot diff jobber`, `query jobber sql`, `use jobber-pp-cli`, `run jobber-pp-cli`."
author: "melanson633"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - jobber-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/sales-and-crm/jobber/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Jobber — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `jobber-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install jobber --cli-only
   ```
2. Verify: `jobber-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/jobber/cmd/jobber-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use jobber-pp-cli when an agent or user needs offline analysis of a Jobber tenant: AR aging, invoice payment tracing, week-over-week snapshot diffs, full-text search across synced data, or ad-hoc SQL over the local store. It is read-only by design — pair it with QBO or another accounting CLI for full reconciliation. It does not write to Jobber.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Accounting & AR analysis
- **`ar aging`** — Aged AR by client with 0-30/31-60/61-90/90+ buckets and per-bucket totals — answers the question every advisor and bookkeeper asks first, offline and instantly.

  _Reach for this whenever the user asks about overdue invoices, collections risk, or DSO. It's the offline-first, agent-shaped equivalent of opening the Jobber AR report and re-pivoting in Excel._

  ```bash
  jobber-pp-cli ar aging --as-of 2026-05-15 --json --select client,bucket_0_30,bucket_31_60,bucket_61_90,bucket_over_90
  ```
- **`invoices trace`** — Per-invoice ledger: total billed, sum of payment records, balance, allocated payout reference, status drift. `--mismatched` filters to invoices where payments don't equal total.

  _Use when the user asks 'why is this invoice still open' or wants to find misposted payments. One row per invoice covers everything the Jobber UI buries three clicks deep._

  ```bash
  jobber-pp-cli invoices trace --mismatched --json
  ```

### Diligence & diff
- **`snapshot diff`** — Diff two labeled SQLite snapshots: new clients, status transitions, paid invoices, open-AR deltas per client. `--save <label>` tags the current DB for a later diff.

  _Reach for this on a weekly cadence to write client memos, identify deltas, or build an audit log of changes between two points in time._

  ```bash
  jobber-pp-cli snapshot diff 2026-05-15 2026-05-22 --json
  ```

## Command Reference

**clients** — Clients (Jobber `clients` Relay connection)

- `jobber-pp-cli clients get` — Get a client by EncodedId
- `jobber-pp-cli clients list` — List clients with optional filters

**invoices** — Invoices (Jobber `invoices` Relay connection)

- `jobber-pp-cli invoices` — List invoices with optional filters

**jobber_jobs** — Jobs (Jobber `jobs` Relay connection). Resource key is `jobber_jobs` to avoid press v4.9.0 reserved-cobra collision; post-rewrite renames Cobra Use back to `jobs` and removes the unused built-in jobs ledger.

- `jobber-pp-cli jobber_jobs get` — Get a job by EncodedId
- `jobber-pp-cli jobber_jobs list` — List jobs with optional filters

**payment-records** — Payment records (Jobber `paymentRecords` Relay connection, PaymentRecordInterface)

- `jobber-pp-cli payment-records` — List payment records with optional filters (entryDate is exclusive both ends - pad +/-1 day)

**properties** — Properties (Jobber `properties` Relay connection)

- `jobber-pp-cli properties` — List properties with optional filters

**quotes** — Quotes (Jobber `quotes` Relay connection)

- `jobber-pp-cli quotes` — List quotes with optional filters

**visits** — Visits (Jobber `visits` Relay connection)

- `jobber-pp-cli visits` — List visits with optional filters


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
jobber-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Build a weekly AR delta memo

```bash
jobber-pp-cli snapshot save 2026-05-22
jobber-pp-cli snapshot diff 2026-05-15 2026-05-22 --json
```

Saves a labeled DB snapshot, then diffs it against last week to surface new clients, status transitions, newly paid invoices, and per-client open-AR deltas — exactly the changes that belong in a weekly client memo.

### Find invoices where payments don't match the total

```bash
jobber-pp-cli invoices trace --mismatched --json
```

Per-invoice ledger comparing `total`, `payments_total`, `deposit_amount`, and the sum of underlying payment records. Flags any drift > $0.005 — the misposted-payment finder.

### Ad-hoc SQL against the local store

```bash
jobber-pp-cli sql "SELECT invoice_number, total, payments_total FROM invoices WHERE invoice_status='awaiting_payment' ORDER BY total DESC LIMIT 25" --json
```

Read-only ad-hoc SQL (write tokens rejected at the CLI layer + sqlite `mode=ro`) for anything the curated commands don't already answer.

### Full-text search across synced data

```bash
jobber-pp-cli search "kitchen renovation" --json --limit 20
```

FTS5 search across every synced resource — clients, jobs, invoices, properties, quotes, visits, payment records — without hitting the live API.

## Auth Setup

Jobber uses OAuth2 authorization code flow with mandatory refresh-token rotation. `jobber-pp-cli` reads `JOBBER_CLIENT_ID`, `JOBBER_CLIENT_SECRET`, `JOBBER_CALLBACK_URL`, `JOBBER_ACCESS_TOKEN`, `JOBBER_REFRESH_TOKEN`, and `JOBBER_GRAPHQL_VERSION` from the environment. Every refresh persists the newest refresh token back to the Windows user environment (required by Jobber's rotation policy). Run `jobber-pp-cli doctor` to verify the connection.

Run `jobber-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  jobber-pp-cli clients list --agent --select id,name,status
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
jobber-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
jobber-pp-cli feedback --stdin < notes.txt
jobber-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.jobber-pp-cli/feedback.jsonl`. They are never POSTed unless `JOBBER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `JOBBER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
jobber-pp-cli profile save briefing --json
jobber-pp-cli --profile briefing clients list
jobber-pp-cli profile list --json
jobber-pp-cli profile show briefing
jobber-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `jobber-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add jobber-pp-mcp -- jobber-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which jobber-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   jobber-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `jobber-pp-cli <command> --help`.
