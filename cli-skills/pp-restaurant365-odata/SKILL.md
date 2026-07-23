---
name: pp-restaurant365-odata
description: "Printing Press CLI for Restaurant365 OData. Read-only schema discovery, safe sampling, backfill planning, export, sync, and deletion-tombstone checks for documented Restaurant365 OData reporting views."
author: "Dhilip Subramanian"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - restaurant365-odata-pp-cli
    install:
      - kind: go
        bins: [restaurant365-odata-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/restaurant365-odata/cmd/restaurant365-odata-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/commerce/restaurant365-odata/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Restaurant365 OData — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `restaurant365-odata-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install restaurant365-odata --cli-only
   ```
2. Verify: `restaurant365-odata-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/restaurant365-odata/cmd/restaurant365-odata-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**list-views** — List documented Restaurant365 OData views

- `restaurant365-odata-pp-cli list-views --agent` — Returns view names, resource names, field counts, date fields, rowVersion support, and suggested sync pattern. Fetches `$metadata` only.

**describe-view** — Describe one Restaurant365 OData view

- `restaurant365-odata-pp-cli describe-view Location --agent` — Returns field names and OData types from `$metadata`. Does not fetch rows.

**sample** — Safely sample one Restaurant365 OData view

- `restaurant365-odata-pp-cli sample --view Location --limit 5 --agent` — Returns row count and columns only. Values are redacted by default.
- `restaurant365-odata-pp-cli sample --view Location --limit 5 --include-values --agent` — Includes row values only when explicitly requested by the user.

**backfill-plan** — Build a request plan without fetching rows

- `restaurant365-odata-pp-cli backfill-plan --view SalesDetail --from 2026-05-01 --to 2026-06-15 --agent` — Splits date-backed views into bounded DateTimeOffset windows.
- `restaurant365-odata-pp-cli backfill-plan --view Transaction --watermark 0 --agent` — Builds a rowVersion incremental filter.

**deleted-records** — Inspect `EntityDeleted` tombstones

- `restaurant365-odata-pp-cli deleted-records --entity TransactionDetail --since-row-version 0 --limit 5 --agent` — Returns counts by entity and redacts values by default.

**export** — Export a bounded view slice

- `restaurant365-odata-pp-cli export --view SalesDetail --from 2026-05-01 --to 2026-05-01 --format jsonl --output sales-detail.jsonl` — Writes one date-window slice to JSONL.

**company** — Manage company

- `restaurant365-odata-pp-cli company` — Returns Company reporting rows from Restaurant365 OData.

**employee** — Manage employee

- `restaurant365-odata-pp-cli employee` — Returns Employee reporting rows from Restaurant365 OData.

**entity-deleted** — Manage entity deleted

- `restaurant365-odata-pp-cli entity-deleted` — Returns deletion tombstone rows from Restaurant365 OData.

**gl-account** — Manage gl account

- `restaurant365-odata-pp-cli gl-account` — Returns GlAccount reporting rows from Restaurant365 OData.

**glaccount** — Manage glaccount

- `restaurant365-odata-pp-cli glaccount` — Returns GLAccount reporting rows from Restaurant365 OData.

**item** — Manage item

- `restaurant365-odata-pp-cli item` — Returns Item reporting rows from Restaurant365 OData.

**job-title** — Manage job title

- `restaurant365-odata-pp-cli job-title` — Returns JobTitle reporting rows from Restaurant365 OData.

**labor-detail** — Manage labor detail

- `restaurant365-odata-pp-cli labor-detail` — Returns LaborDetail reporting rows from Restaurant365 OData.

**location** — Manage location

- `restaurant365-odata-pp-cli location` — Returns Location reporting rows from Restaurant365 OData.

**metadata** — Manage metadata

- `restaurant365-odata-pp-cli metadata` — Returns the service metadata document describing available Restaurant365 OData views and fields.

**payroll-summary** — Manage payroll summary

- `restaurant365-odata-pp-cli payroll-summary` — Returns PayrollSummary reporting rows from Restaurant365 OData.

**posemployee** — Manage posemployee

- `restaurant365-odata-pp-cli posemployee` — Returns POSEmployee reporting rows from Restaurant365 OData.

**sales-detail** — Manage sales detail

- `restaurant365-odata-pp-cli sales-detail` — Returns SalesDetail reporting rows from Restaurant365 OData.

**sales-employee** — Manage sales employee

- `restaurant365-odata-pp-cli sales-employee` — Returns SalesEmployee reporting rows from Restaurant365 OData.

**sales-payment** — Manage sales payment

- `restaurant365-odata-pp-cli sales-payment` — Returns SalesPayment reporting rows from Restaurant365 OData.

**transaction** — Manage transaction

- `restaurant365-odata-pp-cli transaction` — Returns Transaction reporting rows from Restaurant365 OData.

**transaction-detail** — Manage transaction detail

- `restaurant365-odata-pp-cli transaction-detail` — Returns TransactionDetail reporting rows from Restaurant365 OData.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
restaurant365-odata-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Restaurant365 OData uses HTTP Basic authentication. Set the tenant username, commonly in `domain\username` form, and the matching password:

```bash
export RESTAURANT365_ODATA_USERNAME="domain\\username"
export RESTAURANT365_ODATA_PASSWORD="<password>"
export RESTAURANT365_ODATA_BASE_URL="https://odata.restaurant365.net/api/v2/views"
```

Short aliases are also supported:

```bash
export R365_ODATA_USERNAME="domain\\username"
export R365_ODATA_PASSWORD="<password>"
export R365_ODATA_BASE_URL="https://odata.restaurant365.net/api/v2/views"
```

Or persist credentials in `~/.config/restaurant365-odata-pp-cli/config.toml`.

Run `restaurant365-odata-pp-cli doctor` to verify setup.

## R365 Refresh Guidance

- Use `list-views` or `describe-view` before fetching rows.
- Use `sample` without `--include-values` for private tenants unless row-level output is explicitly needed.
- Use `backfill-plan` before `export` or `sync` for sales, labor, and payroll date windows.
- Use rowVersion watermarks for `Transaction`, `TransactionDetail`, and `EntityDeleted`.
- Use `deleted-records` first in count mode when checking whether a pipeline needs delete handling.
- Prefer full refreshes for small dimensions such as `Location`, `Company`, `GLAccount`, `Item`, and `JobTitle`.
- Date-backed Restaurant365 filters should use DateTimeOffset boundaries such as `date ge 2026-05-01T00:00:00Z and date le 2026-05-31T23:59:59Z`, not bare dates.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  restaurant365-odata-pp-cli company --agent --select id,name,status
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
restaurant365-odata-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
restaurant365-odata-pp-cli feedback --stdin < notes.txt
restaurant365-odata-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/restaurant365-odata-pp-cli/feedback.jsonl`. They are never POSTed unless `RESTAURANT365_ODATA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `RESTAURANT365_ODATA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
restaurant365-odata-pp-cli profile save briefing --json
restaurant365-odata-pp-cli --profile briefing company
restaurant365-odata-pp-cli profile list --json
restaurant365-odata-pp-cli profile show briefing
restaurant365-odata-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `restaurant365-odata-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/restaurant365-odata/cmd/restaurant365-odata-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add restaurant365-odata-pp-mcp -- restaurant365-odata-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which restaurant365-odata-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. For private Restaurant365 tenants, choose schema or redacted commands first: `list-views`, `describe-view`, `sample`, `backfill-plan`, or `deleted-records`.
4. Execute with the `--agent` flag:
   ```bash
   restaurant365-odata-pp-cli <command> [subcommand] [args] --agent
   ```
5. If ambiguous, drill into subcommand help: `restaurant365-odata-pp-cli <command> --help`.
