---
name: pp-sybill
description: "The first CLI for Sybill: every conversation, deal, and account in your terminal Trigger phrases: `what deals have gone dark`, `build my weekly call digest`, `show pending crm autofill suggestions`, `find pricing objections in my calls`, `roll up this account's activity`, `use sybill`, `run sybill`."
author: "riccardovandra"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - sybill-pp-cli
---

# Sybill — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `sybill-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install sybill --cli-only
   ```
2. Verify: `sybill-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/sybill/cmd/sybill-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Sybill records sales calls and generates AI summaries, deal briefs, and crmAutofill suggestions. This CLI pulls all of it into a local SQLite store you can query offline, then adds the joins the entity-by-entity API can't do: deals gone dark, a weekly call digest grouped by deal, pending crmAutofill diffs, and full-text transcript search.

## When to Use This CLI

Use this CLI when an agent or script needs Sybill call intelligence without the web UI: pulling call summaries and transcripts, answering pipeline-coverage questions across deals and conversations, reviewing crmAutofill suggestions before a CRM push, or grepping transcripts for objection and competitor patterns. It is the right tool any time the question spans more than one entity or needs offline, pipeable output.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-entity pipeline intelligence
- **`deals dark`** — List open deals with no call activity in the last N days, so nothing stalls silently.

  _Reach for this when an agent needs the re-engagement list: which open opportunities have gone quiet._

  ```bash
  sybill-pp-cli deals dark --days 14 --agent
  ```
- **`digest`** — Pull every call in a window, grouped by deal, with next steps per deal (summaries appear when conversation detail is synced).

  _Use this to generate a Monday pipeline review without clicking through every call._

  ```bash
  sybill-pp-cli digest --since 7d --agent
  ```
- **`account rollup`** — One offline view per account: call count, open deals by stage, contacts, and last activity.

  _Use this to prep a renewal or expansion conversation with full account context._

  ```bash
  sybill-pp-cli account rollup acc_456 --agent
  ```
- **`activity`** — Per-rep breakdown: calls made, deals touched, and deals gone dark over a window.

  _Use this for manager-side coaching prep and pipeline-coverage checks._

  ```bash
  sybill-pp-cli activity --by owner --since 7d --agent
  ```

### Sybill-specific signal
- **`crm-autofill`** — Show the AI-suggested CRM field updates Sybill generated, as a reviewable field-by-field diff.

  _Reach for this before pushing CRM updates, to review what Sybill wants to change._

  ```bash
  sybill-pp-cli crm-autofill --deal deal_123 --agent
  ```
- **`patterns`** — Count and locate transcript mentions of a term, grouped by deal and stage.

  _Reach for this to find where competitor mentions, pricing objections, or legal flags cluster._

  ```bash
  sybill-pp-cli patterns --term pricing --agent
  ```

## Command Reference

**accounts** — Manage accounts

- `sybill-pp-cli accounts get` — Get the detailed view for a single account.
- `sybill-pp-cli accounts list` — List accounts accessible to the API key's organization.

**conversations** — Manage conversations

- `sybill-pp-cli conversations delete` — Delete Conversation Ingest
- `sybill-pp-cli conversations get` — Get the detailed view for a single conversation.
- `sybill-pp-cli conversations ingest` — Ingest Conversation
- `sybill-pp-cli conversations list` — List conversations accessible to the API key's organization.

**deals** — Manage deals

- `sybill-pp-cli deals get` — Get the detailed view for a single deal.
- `sybill-pp-cli deals list` — List deals accessible to the API key's organization.

**documents** — Manage documents

- `sybill-pp-cli documents delete` — Delete Document Ingest
- `sybill-pp-cli documents get` — Get Document
- `sybill-pp-cli documents ingest` — Ingest Document
- `sybill-pp-cli documents list` — List Documents
- `sybill-pp-cli documents update` — Update Document Ingest

**health** — Manage health

- `sybill-pp-cli health` — Health Check

**messages** — Manage messages

- `sybill-pp-cli messages delete` — Delete Message Ingest
- `sybill-pp-cli messages get` — Get Message
- `sybill-pp-cli messages ingest` — Ingest Message
- `sybill-pp-cli messages list` — List Messages

**object-types** — Manage object types

- `sybill-pp-cli object-types create` — Create Object Type
- `sybill-pp-cli object-types delete` — Delete Object Type
- `sybill-pp-cli object-types get` — Get Object Type
- `sybill-pp-cli object-types list` — List Object Types
- `sybill-pp-cli object-types update` — Update Object Type

**rows** — Manage rows

- `sybill-pp-cli rows delete` — Delete Row Ingest
- `sybill-pp-cli rows get` — Get Row
- `sybill-pp-cli rows ingest` — Ingest Row
- `sybill-pp-cli rows list` — List Rows
- `sybill-pp-cli rows update` — Update Row Ingest Route

**sources** — Manage sources

- `sybill-pp-cli sources create` — Create Source
- `sybill-pp-cli sources delete` — Delete Source
- `sybill-pp-cli sources get` — Get Source
- `sybill-pp-cli sources list` — List Sources
- `sybill-pp-cli sources update` — Update Source


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
sybill-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Monday pipeline review

```bash
sybill-pp-cli digest --since 7d --agent
```

Every external call from the week, grouped by deal with summary and next steps, as JSON an agent can format.

### Find stalled deals

```bash
sybill-pp-cli deals dark --days 21 --include-uncovered
```

Open deals with no call in 21 days, including open deals that never had a call at all.

### Review pending CRM changes

```bash
sybill-pp-cli crm-autofill --agent --select dealName,field,suggested,current
```

The crmAutofill suggestions Sybill generated, narrowed to just the diff columns.

### Scan transcripts for objections

```bash
sybill-pp-cli patterns --term "pricing|discount|competitor"
```

Count and locate where pricing and competitor talk clusters across cached calls, grouped by deal and stage.

### Narrow a verbose call payload

```bash
sybill-pp-cli conversations get conv_789 --agent --select title,summary.keyTakeaways,summary.nextSteps
```

Conversation detail can be tens of KB; dotted --select pulls only the fields the agent needs.

## Auth Setup

Create an API key in the Sybill dashboard under Settings > Integrations > API Keys (it starts with sk_live_), then set SYBILL_API_KEY. Run doctor to confirm the key is valid and the API is reachable. Read commands need a key with the read scope; ingest commands need the ingest scope (both are set on the key in the dashboard).

Run `sybill-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  sybill-pp-cli accounts list --agent --select id,name,status
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
sybill-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
sybill-pp-cli feedback --stdin < notes.txt
sybill-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/sybill-pp-cli/feedback.jsonl`. They are never POSTed unless `SYBILL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SYBILL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
sybill-pp-cli profile save briefing --json
sybill-pp-cli --profile briefing accounts list
sybill-pp-cli profile list --json
sybill-pp-cli profile show briefing
sybill-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `sybill-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add sybill-pp-mcp -- sybill-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which sybill-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   sybill-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `sybill-pp-cli <command> --help`.
