---
name: pp-freshservice
description: "Every Freshservice operation in one Go binary — with offline search, SLA intelligence, and agent-native JSON that... Trigger phrases: `list open tickets in freshservice`, `create a ticket in freshservice`, `check SLA breach risk`, `who has the most open tickets`, `find related freshservice tickets`, `approve a change in freshservice`, `search freshservice knowledge base`, `use freshservice`, `freshservice ticket status`."
author: "Mark van de Ven"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - freshservice-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/freshservice/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Freshservice — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `freshservice-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install freshservice --cli-only
   ```
2. Verify: `freshservice-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This is the first general-purpose terminal CLI for Freshservice — the jira-cli equivalent for ITSM. It covers tickets, changes, assets, users, and knowledge base with full CRUD, local SQLite sync, FTS across all entity types, and novel analytics commands that would require three dashboards and a spreadsheet to replicate manually.

## When to Use This CLI

Use freshservice-pp-cli when an AI agent needs to query, create, or update ITSM records in Freshservice — ticket triage, change approval workflows, asset inventory checks, or SLA compliance reporting. It is especially powerful for cross-entity correlation tasks (finding all open tickets linked to an asset, checking change window collisions, or surfacing knowledge gaps) that would require multiple API calls and manual data joining if done via raw HTTP.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### SLA Management
- **`breach-risk`** — Shows every open ticket projected to breach SLA within the next N hours, sorted by minutes remaining — act before the clock runs out, not after.

  _Use this when an SRE or IT admin needs to know which tickets will breach SLA before the next check-in — prevents reactive firefighting._

  ```bash
  freshservice-pp-cli breach-risk --hours 4 --group Infrastructure --agent
  ```
- **`dept-sla`** — Aggregates SLA compliance percentage, breach count, and mean time to resolve by requester department for a rolling period — exec-ready ranking without exporting to Excel.

  _Use this when an AI agent is generating an executive SLA compliance report or identifying departments that need service level attention._

  ```bash
  freshservice-pp-cli dept-sla --period 30d --sort breach-rate --agent
  ```

### Daily Workflow
- **`my-queue`** — Combines all tickets assigned to you with SLA countdown plus any change records awaiting your approval — the first command an agent runs each morning.

  _Use this to get an AI agent's complete pending workload in one structured call before deciding which task to action next._

  ```bash
  freshservice-pp-cli my-queue user_at_example.com --agent
  ```
- **`search`** — Runs a single ranked full-text search across tickets, assets, change records, and KB articles simultaneously — find everything related to an incident keyword in one shot.

  _Use this when an AI agent needs to gather context about a symptom across all ITSM entities before proposing a resolution._

  ```bash
  freshservice-pp-cli search "database crash" --in tickets,assets,changes --agent
  ```

### Team Operations
- **`workload`** — Table of agents with open ticket count, average ticket age, P1/P2 count, and normalized load score — see who is drowning and who has capacity in five seconds.

  _Use this when an AI agent needs to decide which human agent to assign a new ticket to based on current capacity._

  ```bash
  freshservice-pp-cli workload --group "Network Support" --agent
  ```
- **`oncall-gap`** — Identifies time windows where high-severity tickets arrived but no agent in the group acknowledged within SLA — surfaces staffing gaps in on-call rotations.

  _Use this to identify on-call schedule gaps before the next incident strikes the same window._

  ```bash
  freshservice-pp-cli oncall-gap --group Infrastructure --period 4w --severity P1,P2 --agent
  ```

### Change Management
- **`change-collisions`** — Flags change records whose planned maintenance windows overlap, optionally filtered by CI — prevents two teams from scheduling conflicting downtime on the same system.

  _Use this before approving a change to verify no other group has a conflicting maintenance window on the same infrastructure._

  ```bash
  freshservice-pp-cli change-collisions --window 48h --ci prod-db-01 --agent
  ```

### Problem Management
- **`recurrence`** — Uses FTS similarity on ticket subjects and descriptions to surface repeated symptom patterns grouped by asset, requester, or keyword — shows which problems keep coming back.

  _Use this to identify root-cause candidates when an AI agent is investigating a chronic incident pattern._

  ```bash
  freshservice-pp-cli recurrence --asset FS-1042 --days 90 --agent
  ```

### Knowledge Management
- **`kb-gaps`** — Matches recent ticket subjects against the KB article corpus using FTS and ranks topic clusters with no matching article by ticket volume — tells you exactly what to document first.

  _Use this when an AI agent is drafting a knowledge base improvement plan and needs to prioritize which gaps to fill._

  ```bash
  freshservice-pp-cli kb-gaps --group "Desktop Support" --days 30 --min-tickets 3 --agent
  ```

### Asset Management
- **`orphan-assets`** — Finds assets with no associated open ticket, no active contract, and no assigned user activity in the last N days — surfaces hardware you are paying maintenance on that nobody uses.

  _Use this during IT asset audits to identify candidates for decommission or reallocation without manual cross-referencing._

  ```bash
  freshservice-pp-cli orphan-assets --type laptop --days 60 --agent
  ```

## Command Reference

**agent-fields** — Manage agent fields

- `freshservice-pp-cli agent-fields` — List agent form fields

**agents** — Manage agents

- `freshservice-pp-cli agents create` — Create an agent
- `freshservice-pp-cli agents delete` — Delete agent
- `freshservice-pp-cli agents get` — Get agent by ID
- `freshservice-pp-cli agents list` — List agents
- `freshservice-pp-cli agents update` — Update agent

**assets** — Manage assets

- `freshservice-pp-cli assets create` — Create an asset
- `freshservice-pp-cli assets delete` — Delete an asset
- `freshservice-pp-cli assets get` — Get asset by display ID
- `freshservice-pp-cli assets list` — List or search assets
- `freshservice-pp-cli assets update` — Update an asset

**canned-responses** — Manage canned responses

- `freshservice-pp-cli canned-responses get` — Get canned response
- `freshservice-pp-cli canned-responses list` — List canned responses

**change-form-fields** — Manage change form fields

- `freshservice-pp-cli change-form-fields` — List change form fields

**changes** — Manage changes

- `freshservice-pp-cli changes create` — Create a change
- `freshservice-pp-cli changes delete` — Delete a change
- `freshservice-pp-cli changes filter` — Filter changes by query
- `freshservice-pp-cli changes get` — Get change by ID
- `freshservice-pp-cli changes list` — List changes
- `freshservice-pp-cli changes update` — Update a change

**contracts** — Manage contracts

- `freshservice-pp-cli contracts` — List contracts

**departments** — Manage departments

- `freshservice-pp-cli departments` — List departments

**groups** — Manage groups

- `freshservice-pp-cli groups create` — Create agent group
- `freshservice-pp-cli groups get` — Get agent group
- `freshservice-pp-cli groups list` — List agent groups
- `freshservice-pp-cli groups update` — Update agent group

**locations** — Manage locations

- `freshservice-pp-cli locations` — List locations

**products** — Manage products

- `freshservice-pp-cli products create` — Create product
- `freshservice-pp-cli products get` — Get product
- `freshservice-pp-cli products list` — List products
- `freshservice-pp-cli products update` — Update product

**requester-fields** — Manage requester fields

- `freshservice-pp-cli requester-fields` — List requester form fields

**requesters** — Manage requesters

- `freshservice-pp-cli requesters create` — Create a requester
- `freshservice-pp-cli requesters deactivate` — Deactivate requester
- `freshservice-pp-cli requesters get` — Get requester by ID
- `freshservice-pp-cli requesters list` — List requesters
- `freshservice-pp-cli requesters update` — Update requester

**service-catalog** — Manage service catalog

- `freshservice-pp-cli service-catalog list-items` — List service catalog items
- `freshservice-pp-cli service-catalog place-service-request` — Place a service catalog request

**solutions** — Manage solutions

- `freshservice-pp-cli solutions get-category` — Get knowledge base category
- `freshservice-pp-cli solutions list-categories` — List knowledge base categories

**ticket-form-fields** — Manage ticket form fields

- `freshservice-pp-cli ticket-form-fields` — List ticket form fields

**tickets** — Manage tickets

- `freshservice-pp-cli tickets create` — Create a ticket
- `freshservice-pp-cli tickets delete` — Delete a ticket
- `freshservice-pp-cli tickets filter` — Filter tickets by query
- `freshservice-pp-cli tickets get` — Get ticket by ID
- `freshservice-pp-cli tickets list` — List tickets
- `freshservice-pp-cli tickets update` — Update a ticket

**vendors** — Manage vendors

- `freshservice-pp-cli vendors` — List vendors

**workspaces** — Manage workspaces

- `freshservice-pp-cli workspaces` — List workspaces


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
freshservice-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Triage the morning ticket queue

```bash
freshservice-pp-cli tickets filter --query "status:2 AND priority:3" --json --select tickets.id,tickets.subject,tickets.due_by,tickets.responder_id
```

pull high-priority open tickets via Freshservice's filter syntax with only the fields needed for triage routing

### Check SLA risk before standup

```bash
freshservice-pp-cli breach-risk --hours 4 --agent
```

surface every ticket that will breach its SLA in the next 4 hours so the team can act proactively

### Find recurring issues on a server

```bash
freshservice-pp-cli recurrence --asset FS-2301 --days 90 --agent
```

fingerprint repeated incident patterns linked to a specific asset over 90 days

### Check for change window conflicts

```bash
freshservice-pp-cli change-collisions --window 48h --agent
```

detect overlapping maintenance windows before approving a new change

### Get full context before creating a ticket

```bash
freshservice-pp-cli search "printer offline" --in tickets,assets,kb --agent --select results.type,results.id,results.subject,results.status
```

cross-entity search to find existing tickets, related assets, and KB articles before opening a duplicate

## Auth Setup

Authenticate with a Personal Access Token (PAT) from your Freshservice account. Set FRESHSERVICE_APIKEY=<your-pat> and FRESHSERVICE_DOMAIN=<yourcompany.freshservice.com>, then run `freshservice-pp-cli doctor` to verify connectivity.

Run `freshservice-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  freshservice-pp-cli agent-fields --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
freshservice-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
freshservice-pp-cli feedback --stdin < notes.txt
freshservice-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.freshservice-pp-cli/feedback.jsonl`. They are never POSTed unless `FRESHSERVICE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FRESHSERVICE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
freshservice-pp-cli profile save briefing --json
freshservice-pp-cli --profile briefing agent-fields
freshservice-pp-cli profile list --json
freshservice-pp-cli profile show briefing
freshservice-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `freshservice-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add freshservice-pp-mcp -- freshservice-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which freshservice-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   freshservice-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `freshservice-pp-cli <command> --help`.
