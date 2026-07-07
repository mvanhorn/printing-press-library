---
name: pp-qbo
description: "Interact with QuickBooks Online directly."
author: "Martin Kessler"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - qbo-pp-cli
    install:
      - kind: go
        bins: [qbo-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/qbo/cmd/qbo-pp-cli
---

# Qbo — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `qbo-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install qbo --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `qbo-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/qbo/cmd/qbo-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

QuickBooks Online CLI — sync full ledger, query SQLite locally, run duplicate expenses audit, and manage accounting from the terminal.

## Command Reference

**accounts** — Manage accounts

- `qbo-pp-cli accounts create` — Create or update an account
- `qbo-pp-cli accounts get` — Get an account by ID

**bills** — Manage vendor bills

- `qbo-pp-cli bills create` — Create or update a bill
- `qbo-pp-cli bills get` — Get a bill by ID

**cdc** — Change Data Capture (CDC) endpoints for incremental syncs

- `qbo-pp-cli cdc` — Query changed entities since a timestamp

**changed** — Show ledger entities modified since a duration or timestamp

- `qbo-pp-cli changed --since 24h` — Retrieve records updated since cutoff date (from local cache)

**customers** — Manage customers

- `qbo-pp-cli customers create` — Create or update a customer
- `qbo-pp-cli customers get` — Get a customer by ID

**duplicates** — Find duplicate purchases or bills to prevent double-billing

- `qbo-pp-cli duplicates --days 3` — Scan local database for matching transactions

**invoices** — Manage invoices

- `qbo-pp-cli invoices create` — Create or update an invoice
- `qbo-pp-cli invoices get` — Get an invoice by ID

**journal_entries** — Manage journal entries

- `qbo-pp-cli journal-entries create` — Create or update a journal entry
- `qbo-pp-cli journal-entries get` — Get a journal entry by ID

**net-worth** — Calculate net worth from asset and liability accounts

- `qbo-pp-cli net-worth` — Output net worth ledger report (from local cache)

**payments** — Manage customer payments

- `qbo-pp-cli payments create` — Create or update a payment
- `qbo-pp-cli payments get` — Get a payment by ID

**purchases** — Manage purchases (Expenses)

- `qbo-pp-cli purchases create` — Create or update a purchase
- `qbo-pp-cli purchases get` — Get a purchase by ID

**query** — Run raw SQL queries against QBO (limitations apply; sync first to query SQLite locally)

- `qbo-pp-cli query` — Run raw QuickBooks SQL query

**reconcile** — Match bank transaction lines for reconciliation

- `qbo-pp-cli reconcile --amount 150.00 --date 2026-06-01` — Find match candidates in local database

**sync** — Synchronize QBO ledger tables to local SQLite cache

- `qbo-pp-cli sync` — Initial bulk paginated sync or CDC incremental update

**vendors** — Manage vendors

- `qbo-pp-cli vendors create` — Create or update a vendor
- `qbo-pp-cli vendors get` — Get a vendor by ID


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
qbo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

The CLI uses the OAuth2 Authorization Code flow. To authorize the CLI:

1. **Interactive Login**:
   Run the login command, providing your Client ID and Client Secret:
   ```bash
   qbo-pp-cli auth login --client-id <id> --client-secret <secret>
   ```
2. **Production Routing**:
   If authenticating against a production company, set the environment environment variable before logging in:
   ```bash
   export QBO_ENVIRONMENT=production
   ```
3. **Custom Callback Redirects**:
   If your app uses a hosted production redirect URI (e.g. Cloudflare page), pass the `--redirect-uri` flag:
   ```bash
   qbo-pp-cli auth login \
     --client-id <id> \
     --client-secret <secret> \
     --redirect-uri https://1cdc515d.qbo-legal-docs-xc5.pages.dev/callback
   ```

Run `qbo-pp-cli doctor` to verify setup and active token status.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  qbo-pp-cli accounts get mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
qbo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
qbo-pp-cli feedback --stdin < notes.txt
qbo-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/qbo-pp-cli/feedback.jsonl`. They are never POSTed unless `QBO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `QBO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
qbo-pp-cli profile save briefing --json
qbo-pp-cli --profile briefing accounts get mock-value
qbo-pp-cli profile list --json
qbo-pp-cli profile show briefing
qbo-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `qbo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/qbo/cmd/qbo-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add qbo-pp-mcp -- qbo-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which qbo-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   qbo-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `qbo-pp-cli <command> --help`.
