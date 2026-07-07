---
name: pp-expensify
description: "File expenses and submit reports to Expensify in one line. Every command an agent should need, with a local cache so searches stay offline. Trigger phrases: `file an expense`, `submit my expense report`, `expense that`, `what did I expense this month`, `use expensify`, `run expensify`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - expensify-pp-cli
    install:
      - kind: go
        bins: [expensify-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/expensify/cmd/expensify-pp-cli
---

# Expensify — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `expensify-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install expensify --cli-only
   ```
2. Verify: `expensify-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/expensify/cmd/expensify-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

expensify-pp-cli turns the Expensify web app into a terminal. Log in once, and every filing/reviewing/submitting task that used to require clicking through forms becomes a single command. A local SQLite store gives you offline search, rollups, dupe detection, and missing-receipt alerts that no other Expensify tool has.

## When to Use This CLI

Pick this CLI when you want to file, review, or submit Expensify expenses and reports without leaving the terminal. It excels at one-liner expense filing from natural language, end-of-month report drafting from a date range, offline search across years of expenses, and orchestrating accounting-system exports. Agents can drive it through the standard --json output and typed exit codes, or through the built-in MCP bridge mode.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Filing at thought speed
- **`expense quick`** — File an expense with one line: amount, merchant, and category parsed from a short prompt — no forms, no web UI.

  _When a user tells an agent 'I just expensed dinner at Maya for $42.50,' the agent should file it in one call — not walk through six fields._

  ```bash
  expensify-pp-cli expense quick "Dinner at Maya $42.50" --agent
  ```
- **`report draft`** — Create a report and auto-attach every un-reported expense from a date range in a single command.

  _End-of-month submission turns from 45 clicks into one command._

  ```bash
  expensify-pp-cli report draft --since 2026-04-01 --title "April expenses" --policy YOUR_POLICY_ID
  ```
- **`expense from-line`** — Paste a raw bank or card CSV row and the CLI extracts date/merchant/amount/currency and files the expense.

  _Reconciling AmEx? Paste a row, file an expense. No copy-paste-copy-paste._

  ```bash
  expensify-pp-cli expense from-line "2026-04-18 DOORDASH*JOES $14.25" --category Meals
  ```

### Local state that compounds
- **`damage`** — Single-glance summary: total expensed, pending, approved, paid for the current month (or a custom range).

  _Agents asked 'how much did I expense this month' get one answer in one call._

  ```bash
  expensify-pp-cli damage --month current --json
  ```
- **`expense search`** — FTS5 search over all your expenses by merchant, comment, category, or tag. Regex-friendly.

  _Agents asked 'did I expense that Starbucks last month' get an answer in one local query._

  ```bash
  expensify-pp-cli expense search "coffee" --since 2026-01-01 --json
  ```
- **`expense missing-receipts`** — Lists expenses without attached receipts so you can catch them before submitting a report.

  _Submit-report-and-get-bounced feels bad; surface missing receipts upfront._

  ```bash
  expensify-pp-cli expense missing-receipts --json
  ```
- **`expense rollup`** — Pivot-table expenses by category, tag, or merchant for any time range.

  _Build your own spending dashboard without burning API budget._

  ```bash
  expensify-pp-cli expense rollup --month 2026-04 --by category
  ```
- **`expense dupes`** — Finds expenses that look like duplicates by (merchant, amount, date±window).

  _Accidental double-file is a top AP pain point; surface it before submission._

  ```bash
  expensify-pp-cli expense dupes --window 3d --json
  ```

### Agent-native plumbing
- **`expense bulk`** — File a whole list of expenses in a single Expense_Create request.

  _Reach for this when filing many expenses at once instead of looping create — it is one atomic request._

  ```bash
  expensify-pp-cli expense bulk --input rows.jsonl --dry-run
  ```
- **`report submit`** — Submit a report and optionally poll until it leaves SUBMITTED.

  _Use --wait when a downstream step depends on the report actually being approved/rejected, not just submitted._

  ```bash
  expensify-pp-cli report submit --report-id 1587860702457827 --wait --timeout 1h
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**admin** — Integration Server: policy, employee, and rules admin

- `expensify-pp-cli admin cards-list` — List domain cards (Domain Cards Getter)
- `expensify-pp-cli admin cards-owners` — List card owners (Card Owner Data)
- `expensify-pp-cli admin employee-add` — Add an employee to a policy (Advanced Employee Updater)
- `expensify-pp-cli admin employee-remove` — Remove an employee from a policy
- `expensify-pp-cli admin employee-update` — Update an employee (Advanced Employee Updater)
- `expensify-pp-cli admin policy-get` — Get a policy's full config (Policy Getter)
- `expensify-pp-cli admin policy-list` — List all policies you admin (Policy List Getter)
- `expensify-pp-cli admin policy-new` — Create a new policy (Policy Creator)
- `expensify-pp-cli admin policy-set-categories` — Update categories for a policy from YAML
- `expensify-pp-cli admin policy-set-fields` — Update report fields for a policy
- `expensify-pp-cli admin policy-set-tags` — Update tags for a policy from YAML
- `expensify-pp-cli admin report-set-status` — Force a report status transition (Report Status Updater)
- `expensify-pp-cli admin rules-new` — Create an expense rule (Expense Rules Creator)
- `expensify-pp-cli admin rules-update` — Update an expense rule
- `expensify-pp-cli admin tag-approvers-set` — Set tag approvers (Tag Approvers Updater)

**category** — Workspace categories (for expense classification)

- `expensify-pp-cli category` — List categories for a workspace

**expense** — Create, list, and manage personal expenses

- `expensify-pp-cli expense attach` — Attach or replace a receipt on an expense
- `expensify-pp-cli expense create` — Create a new expense
- `expensify-pp-cli expense delete` — Delete an expense
- `expensify-pp-cli expense edit` — Edit an existing expense
- `expensify-pp-cli expense get` — Get expense detail by transaction ID
- `expensify-pp-cli expense list` — List your expenses with filters

**export_resource** — Integration Server: export reports to accounting systems (admin)

- `expensify-pp-cli export-resource download` — Download a previously generated export file
- `expensify-pp-cli export-resource run` — Export reports via Report Exporter (Integration Server)

**me** — Current user profile

- `expensify-pp-cli me` — Get current user profile

**recon** — Integration Server: corporate card reconciliation (admin)

- `expensify-pp-cli recon` — Export reconciliation data for a domain

**report** — Create, manage, and submit expense reports

- `expensify-pp-cli report add` — Add expenses to a report
- `expensify-pp-cli report approve` — Approve a report (manager action)
- `expensify-pp-cli report comment` — Add a comment to a report thread
- `expensify-pp-cli report create` — Create a new report
- `expensify-pp-cli report delete` — Delete a draft report
- `expensify-pp-cli report get` — Get report detail
- `expensify-pp-cli report list` — List your reports
- `expensify-pp-cli report pay` — Mark a report as reimbursed
- `expensify-pp-cli report reopen` — Reopen a submitted report back to draft
- `expensify-pp-cli report submit` — Submit a report for approval

**tag** — Workspace tags (multi-level, for expense classification)

- `expensify-pp-cli tag` — List tags for a workspace

**workspace** — View workspaces (policies) you have access to

- `expensify-pp-cli workspace get` — Get workspace detail
- `expensify-pp-cli workspace list` — List workspaces accessible to your account


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
expensify-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### File a quick expense

```bash
expensify-pp-cli expense quick "Lunch at Joe's $18.50"
```

One line, one call, one expense. Category auto-suggested from your history.

### Draft this month's report

```bash
expensify-pp-cli report draft --since 2026-04-01 --until 2026-04-30 --title "April" --policy 1234567
```

Creates a report and attaches every un-reported April expense in a single command.

### Submit for approval and wait

```bash
expensify-pp-cli report submit --report-id 1587860702457827 --wait --timeout 1h
```

Blocks until approval arrives — drop this in CI after a closing script.

### Find expenses with no receipt

```bash
expensify-pp-cli expense missing-receipts --json
```

Catches receipt gaps before they bounce your report.

### Search your offline expense cache

```bash
expensify-pp-cli expense search "coffee" --json
```

FTS5 search over every synced expense — merchant, comment, category, tag — with no network call.

## Auth Setup

Two ways to authenticate: (1) `expensify auth login` opens a browser, you log in, the CLI captures your session token — works immediately for all filing/submitting commands; (2) `expensify auth set-keys` stores your Integration Server partner credentials (get them at https://www.expensify.com/tools/integrations/) — required only for export/admin commands. Most users only need option 1.

Run `expensify-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  expensify-pp-cli category --policy-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `EXPENSIFY_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `EXPENSIFY_CONFIG_DIR`, `EXPENSIFY_DATA_DIR`, `EXPENSIFY_STATE_DIR`, `EXPENSIFY_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `EXPENSIFY_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `expensify-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "expensify": {
        "command": "expensify-pp-mcp",
        "env": {
          "EXPENSIFY_HOME": "/srv/expensify"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `EXPENSIFY_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `EXPENSIFY_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
expensify-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
expensify-pp-cli feedback --stdin < notes.txt
expensify-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `EXPENSIFY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `EXPENSIFY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
expensify-pp-cli profile save briefing --json
expensify-pp-cli --profile briefing category --policy-id 550e8400-e29b-41d4-a716-446655440000
expensify-pp-cli profile list --json
expensify-pp-cli profile show briefing
expensify-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `expensify-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/expensify/cmd/expensify-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add expensify-pp-mcp -- expensify-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which expensify-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   expensify-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `expensify-pp-cli <command> --help`.
