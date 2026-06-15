---
name: pp-sendfox
description: "Printing Press CLI for SendFox: contacts, lists, campaign reads, audience hygiene, CSV reconciliation, launch plans, signup-form handoffs, webhook packets, and capability checks."
author: "cathrynlavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - sendfox-pp-cli
    install:
      - kind: go
        bins: [sendfox-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/sendfox/cmd/sendfox-pp-cli
---

# SendFox — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `sendfox-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install sendfox --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `sendfox-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/sendfox/cmd/sendfox-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Operate SendFox contacts, lists, campaign reads, audience hygiene, CSV reconciliation, launch checklists, signup-form handoffs, webhook setup packets, and public API capability checks from the terminal.
## Command Reference

**capabilities** — Documented public API support matrix

- `sendfox-pp-cli capabilities` — Show which SendFox resources support list/get/create/update/delete and where dashboard handoffs are required

**campaigns** — Campaign reads

- `sendfox-pp-cli campaigns get` — Get campaign by ID
- `sendfox-pp-cli campaigns list` — List campaigns

**contacts** — Contacts and subscription state

- `sendfox-pp-cli contacts create` — Create a contact
- `sendfox-pp-cli contacts get` — Get a contact by ID
- `sendfox-pp-cli contacts list` — List contacts or find a contact by email
- `sendfox-pp-cli contacts onboard` — Create a contact and attach list memberships in one automation-aware flow
- `sendfox-pp-cli contacts import-csv` — Bulk-create contacts from CSV behind a dry-run/--yes safety gate
- `sendfox-pp-cli contacts audit-csv` — Validate subscriber CSVs for invalid and duplicate emails before any mutation
- `sendfox-pp-cli contacts reconcile-csv` — Compare a CSV against live contacts and emit create/skip actions

**forms** — Generate SendFox integration assets

- `sendfox-pp-cli forms generate` — Generate a self-contained HTML signup form and server-proxy handoff

**lists** — Contact lists

- `sendfox-pp-cli lists create` — Create a contact list
- `sendfox-pp-cli lists get` — Get a contact list by ID
- `sendfox-pp-cli lists list-lists` — List contact lists

**workflow** — Compound SendFox workflows for agents

- `sendfox-pp-cli workflow account-snapshot` — Summarize account, list, contact, and campaign state
- `sendfox-pp-cli workflow audience-map` — Map contacts to lists and surface segmentation gaps
- `sendfox-pp-cli workflow campaign-digest` — Summarize campaign count, status mix, and recency
- `sendfox-pp-cli workflow hygiene-report` — Find duplicate emails, invalid emails, status mix, and list-membership gaps
- `sendfox-pp-cli workflow launch-plan` — Generate a safe SendFox list-launch checklist and exact next CLI/dashboard steps

**webhooks** — Generate SendFox webhook/dashboard handoffs

- `sendfox-pp-cli webhooks handoff` — Generate dashboard setup and handler-contract packets for SendFox webhook receivers

**me** — Manage me

- `sendfox-pp-cli me` — Get authenticated user

**unsubscribe** — Manage unsubscribe

- `sendfox-pp-cli unsubscribe` — Unsubscribe a contact by email


## Unique Capabilities

Prefer these compound commands before raw endpoint mirrors:

- `sendfox-pp-cli workflow account-snapshot --agent` — one read-only operating packet across account, lists, contacts, and campaigns.
- `sendfox-pp-cli workflow audience-map --agent` — list membership map plus contacts without list membership.
- `sendfox-pp-cli workflow campaign-digest --agent` — campaign status counts and recent campaign rows.
- `sendfox-pp-cli capabilities --agent` — support matrix that stops agents from inventing unsupported campaign/webhook writes.
- `sendfox-pp-cli workflow hygiene-report --agent` — live hygiene report for duplicate/invalid emails, contact status mix, and contacts without lists.
- `sendfox-pp-cli workflow launch-plan --list-id <id> --csv subscribers.csv --agent` — safe launch checklist with exact CLI commands and dashboard-only campaign creation called out explicitly.
- `sendfox-pp-cli contacts audit-csv --file subscribers.csv --lists <ids> --agent` — validate duplicate/invalid rows before mutation.
- `sendfox-pp-cli contacts reconcile-csv --file subscribers.csv --lists <ids> --dry-run --agent` — compare CSV rows to live contacts and emit create/skip actions.
- `sendfox-pp-cli contacts onboard --email <email> --lists <ids> --dry-run --agent` — preview a contact-create request that may trigger list automations; remove `--dry-run` for live execution.
- `sendfox-pp-cli contacts import-csv --file subscribers.csv --lists <ids> --dry-run --agent` — preview a guarded bulk import; live runs require `--yes` or `--agent`.
- `sendfox-pp-cli forms generate --list-id <id> --output form.html --agent` — create a signup-form handoff and keep the bearer token server-side.
- `sendfox-pp-cli webhooks handoff --endpoint <url> --agent` — dashboard setup packet and receiver contract without pretending there is public webhook CRUD.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
sendfox-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `sendfox-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
sendfox-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `SENDFOX_API_TOKEN` as an environment variable (`SENDFOX_BEARER_AUTH` remains supported as a compatibility alias).

Run `sendfox-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  sendfox-pp-cli campaigns list --agent --select id,name,status
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
sendfox-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
sendfox-pp-cli feedback --stdin < notes.txt
sendfox-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/sendfox-pp-cli/feedback.jsonl`. They are never POSTed unless `SENDFOX_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SENDFOX_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
sendfox-pp-cli profile save briefing --json
sendfox-pp-cli --profile briefing campaigns list
sendfox-pp-cli profile list --json
sendfox-pp-cli profile show briefing
sendfox-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `sendfox-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/sendfox/cmd/sendfox-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add sendfox-pp-mcp -- sendfox-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which sendfox-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   sendfox-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `sendfox-pp-cli <command> --help`.
