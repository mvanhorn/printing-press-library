---
name: pp-zoho-desk
description: "The only scriptable, agent-native CLI for Zoho Desk: every ticket operation plus a local SQLite store, offline search, and SLA/triage analytics no help-desk tool ships. Trigger phrases: `triage my zoho desk tickets`, `which agents are overloaded`, `tickets about to breach SLA`, `what changed on my tickets overnight`, `look up this customer in zoho desk`, `use zoho-desk`, `run zoho-desk`."
author: ""
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - zoho-desk-pp-cli
    install:
      - kind: go
        bins: [zoho-desk-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/cmd/zoho-desk-pp-cli
---

# Zoho Desk — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `zoho-desk-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install zoho-desk --cli-only
   ```
2. Verify: `zoho-desk-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/cmd/zoho-desk-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Manage tickets, contacts, accounts, and agents from the command line with automatic OAuth token refresh, multi-data-center handling, and auto-pagination with 429 backoff. Then go beyond the API: sla-radar forecasts breaches before they happen, agent-load finds the real bottleneck, triage builds one ranked queue, and morning composes your whole shift brief.

## When to Use This CLI

Reach for this CLI when you need to script or automate Zoho Desk: bulk ticket operations, scheduled exports, cross-ticket analytics, SLA monitoring, or feeding ticket data to an agent. It is the right tool when the web UI is too slow or cannot express the query, and when you want offline, pipeable, JSON output.

## Anti-triggers

Do not use this CLI for:
- Do not use for end-user ticket submission portals; use the customer-facing Help Center.
- Do not use for Zoho CRM, Books, or other Zoho products; this is Desk-only.
- Do not use for real-time chat/IM; Desk threads are async email-style conversations.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local analytics no UI exposes
- **`sla-radar`** — See which open tickets will breach SLA in the next N hours, ranked by time-to-due, before they breach.

  _Reach for this to act before a breach instead of reporting one after._

  ```bash
  zoho-desk-pp-cli sla-radar --within 2h --agent
  ```
- **`agent-load`** — See which agents are actually overloaded right now, weighting open tickets by priority and age versus the team median.

  _Use before reassigning work to find the real bottleneck, not just raw counts._

  ```bash
  zoho-desk-pp-cli agent-load --weighted --agent
  ```
- **`triage`** — One ranked queue merging unassigned, overdue, and high-priority tickets with a combined score.

  _Start the day here: the single list of what needs attention first._

  ```bash
  zoho-desk-pp-cli triage --limit 20 --agent
  ```
- **`morning`** — A single brief composing breach forecast, agent load, and overnight changes.

  _The one command to run at the start of a shift._

  ```bash
  zoho-desk-pp-cli morning --agent
  ```
- **`rebalance`** — Proposes ticket moves from overloaded agents to idle ones, then optionally applies them in bulk.

  _Use to even out queues without hand-picking each reassignment._

  ```bash
  zoho-desk-pp-cli rebalance --plan --agent
  ```
- **`breach-history`** — Who breached SLA and the trend, grouped by agent or department.

  _Use for retrospective SLA accountability._

  ```bash
  zoho-desk-pp-cli breach-history --by agent --agent
  ```

### Local state that compounds
- **`since`** — Every ticket modified within a time window, with its current status, priority, and assignee, sorted most-recent first.

  _Use to catch up on what moved while you were away._

  ```bash
  zoho-desk-pp-cli since 12h --agent
  ```
- **`contact-360`** — Everything about one customer in one view: their contact record, account, and every ticket with status and priority.

  _Use before a call to load full customer context in one command._

  ```bash
  zoho-desk-pp-cli contact-360 PII_EMAIL_EXAMPLE --agent
  ```

## Command Reference

**accounts** — Manage accounts (companies)

- `zoho-desk-pp-cli accounts contacts` — List contacts for an account
- `zoho-desk-pp-cli accounts create` — Create an account
- `zoho-desk-pp-cli accounts get` — Get an account by ID
- `zoho-desk-pp-cli accounts list` — List accounts
- `zoho-desk-pp-cli accounts tickets` — List tickets for an account
- `zoho-desk-pp-cli accounts update` — Update an account

**agents** — Help-desk agents

- `zoho-desk-pp-cli agents count` — Get total agent count
- `zoho-desk-pp-cli agents get` — Get an agent by ID
- `zoho-desk-pp-cli agents list` — List agents

**articles** — Knowledge base articles

- `zoho-desk-pp-cli articles get` — Get a KB article by ID
- `zoho-desk-pp-cli articles list` — List KB articles
- `zoho-desk-pp-cli articles search` — Search KB articles

**comments** — Internal/public ticket comments

- `zoho-desk-pp-cli comments create` — Add a comment to a ticket
- `zoho-desk-pp-cli comments delete` — Delete a comment
- `zoho-desk-pp-cli comments list` — List comments on a ticket

**contacts** — Manage contacts (customers)

- `zoho-desk-pp-cli contacts create` — Create a contact
- `zoho-desk-pp-cli contacts get` — Get a contact by ID
- `zoho-desk-pp-cli contacts list` — List contacts
- `zoho-desk-pp-cli contacts tickets` — List tickets for a contact
- `zoho-desk-pp-cli contacts update` — Update a contact

**departments** — Departments

- `zoho-desk-pp-cli departments get` — Get a department by ID
- `zoho-desk-pp-cli departments list` — List departments

**organizations** — Portals/organizations you can access (orgId source)

- `zoho-desk-pp-cli organizations accessible` — List all organizations accessible to the user
- `zoho-desk-pp-cli organizations get` — Get one organization
- `zoho-desk-pp-cli organizations list` — List organizations the token can access

**products** — Products

- `zoho-desk-pp-cli products get` — Get a product by ID
- `zoho-desk-pp-cli products list` — List products

**slas** — Service-level agreements

- `zoho-desk-pp-cli slas` — List SLAs

**tags** — Tags

- `zoho-desk-pp-cli tags` — List tags

**tasks** — Tasks

- `zoho-desk-pp-cli tasks create` — Create a task
- `zoho-desk-pp-cli tasks get` — Get a task by ID
- `zoho-desk-pp-cli tasks list` — List tasks

**teams** — Agent teams

- `zoho-desk-pp-cli teams agents` — List agents in a team
- `zoho-desk-pp-cli teams get` — Get a team by ID
- `zoho-desk-pp-cli teams list` — List teams

**threads** — Ticket conversation threads

- `zoho-desk-pp-cli threads get` — Get a thread's full content
- `zoho-desk-pp-cli threads latest` — Get the latest thread on a ticket
- `zoho-desk-pp-cli threads list` — List threads on a ticket

**ticketfields** — Ticket field/layout metadata

- `zoho-desk-pp-cli ticketfields` — List ticket fields

**tickets** — Manage support tickets

- `zoho-desk-pp-cli tickets close` — Close one or more tickets
- `zoho-desk-pp-cli tickets create` — Create a ticket
- `zoho-desk-pp-cli tickets draft-reply` — Create a draft reply on a ticket
- `zoho-desk-pp-cli tickets get` — Get a ticket by ID
- `zoho-desk-pp-cli tickets history` — Get a ticket's history/audit trail
- `zoho-desk-pp-cli tickets list` — List tickets
- `zoho-desk-pp-cli tickets merge` — Merge other tickets into this one
- `zoho-desk-pp-cli tickets metrics` — Get response/resolution time metrics for a ticket
- `zoho-desk-pp-cli tickets send-reply` — Send a reply on a ticket
- `zoho-desk-pp-cli tickets update` — Update a ticket

**tickets_search** — Cross-module search

- `zoho-desk-pp-cli tickets-search` — Search tickets by criteria

**timeentries** — Ticket time entries

- `zoho-desk-pp-cli timeentries create` — Add a time entry to a ticket
- `zoho-desk-pp-cli timeentries list` — List time entries on a ticket


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
zoho-desk-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Morning shift brief

```bash
zoho-desk-pp-cli morning --agent
```

One command: breach forecast, agent load, and overnight changes.

### Rebalance an overloaded queue

```bash
zoho-desk-pp-cli rebalance --plan --agent
```

See proposed moves from overloaded agents to idle ones before applying.

### Customer context before a call

```bash
zoho-desk-pp-cli contact-360 PII_EMAIL_EXAMPLE --agent
```

All tickets and account for one customer.

### Narrow a large ticket list

```bash
zoho-desk-pp-cli tickets list --status Open --limit 50 --agent --select data.id,data.subject,data.status,data.assigneeId
```

Ticket payloads are large; --select with dotted paths returns only the fields you need.

### Find forgotten tickets

```bash
zoho-desk-pp-cli stale --days 5 --agent
```

Tickets with no update in 5 days.

## Auth Setup

Zoho Desk uses OAuth2 with a permanent refresh token and 1-hour access tokens. Register a self-client at api-console.zoho.com, grant Desk scopes, and exchange the code for a refresh token. Set ZOHO_DESK_CLIENT_ID, ZOHO_DESK_CLIENT_SECRET, and ZOHO_DESK_REFRESH_TOKEN; the CLI refreshes access tokens automatically. Every call also needs your orgId (run 'organizations list' to find it) and the right data center.

Run `zoho-desk-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  zoho-desk-pp-cli accounts list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `ZOHO_DESK_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `ZOHO_DESK_CONFIG_DIR`, `ZOHO_DESK_DATA_DIR`, `ZOHO_DESK_STATE_DIR`, `ZOHO_DESK_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `ZOHO_DESK_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `zoho-desk-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "zoho-desk": {
        "command": "zoho-desk-pp-mcp",
        "env": {
          "ZOHO_DESK_HOME": "/srv/zoho-desk"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `ZOHO_DESK_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `ZOHO_DESK_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
zoho-desk-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
zoho-desk-pp-cli feedback --stdin < notes.txt
zoho-desk-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `ZOHO_DESK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ZOHO_DESK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
zoho-desk-pp-cli profile save briefing --json
zoho-desk-pp-cli --profile briefing accounts list
zoho-desk-pp-cli profile list --json
zoho-desk-pp-cli profile show briefing
zoho-desk-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `zoho-desk-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/cmd/zoho-desk-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add zoho-desk-pp-mcp -- zoho-desk-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which zoho-desk-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   zoho-desk-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `zoho-desk-pp-cli <command> --help`.
