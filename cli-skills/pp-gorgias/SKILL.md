---
name: pp-gorgias
description: "Every Gorgias support workflow, agent-native, in one binary. Trigger phrases: `list gorgias tickets`, `search gorgias`, `find a ticket about <topic>`, `show gorgias customers`, `what tickets came in yesterday`, `tag this gorgias ticket`, `reply to gorgias ticket`."
author: "Chris Young"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - gorgias-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/sales-and-crm/gorgias/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Gorgias — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `gorgias-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install gorgias --cli-only
   ```
2. Verify: `gorgias-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/cmd/gorgias-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

A token-efficient CLI for the Gorgias customer support API with a sibling MCP server. Covers 108 endpoints across tickets, customers, messages, macros, tags, teams, integrations, events, rules, satisfaction surveys, voice calls, custom fields, and views. Built for AI agents first: JSON output, structured doctor checks, local SQLite mirror, and code-orchestration MCP — 15 tools (~9K tokens measured for `tools/list`) cover the full 108-endpoint surface via the `gorgias_search` + `gorgias_execute` gateway.

## When to Use This CLI

Reach for gorgias-pp-cli whenever a support workflow needs to read or write Gorgias data: pulling recent tickets for digest, searching past conversations for context, automating bulk tag/macro applications, monitoring oncall queues, or building agent-driven escalation flows. The sibling MCP server lets LLMs drive the same surface without shelling out.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`gorgias-pp-cli doctor --json`** — Probes /account with the configured credentials and reports `credentials: valid` only when an authenticated call succeeds.

  _Saves the first-five-minutes credential-debug cycle when wiring up an agent._

  ```bash
  gorgias-pp-cli doctor --json
  ```
- **`gorgias-pp-cli sync --resources tickets --since 7d && gorgias-pp-cli search 'refund' --agent`** — Syncs API data to a local SQLite DB so subsequent searches, analytics, and joins run without hitting the API.

  _Makes repeated agent-driven lookups (e.g. searching for similar past tickets) practical at scale. Ticket `--since` uses documented `order_by=updated_datetime:desc` plus local filtering; do not add undocumented filters like `updated_datetime__gte` unless Gorgias documents them and a live smoke confirms them._

  ```bash
  gorgias-pp-cli sync --resources tickets --since 30d --json
  ```

## Command Reference

**account** — Account-level settings and tenant metadata

- `gorgias-pp-cli account get` — Retrieve the current Gorgias account's metadata: subdomain, plan, billing state, and account-level flags. Use this...
- `gorgias-pp-cli account settings-create` — Create a new account-level settings record for the current Gorgias tenant. Use when bootstrapping a fresh tenant or...
- `gorgias-pp-cli account settings-list` — List the global settings on the current Gorgias account (business hours, language, default channels, notification...
- `gorgias-pp-cli account settings-update` — Update an account settings record by `id`. Use this to flip a tenant-wide flag, change business hours, or adjust a...

**custom-fields** — Define and manage custom fields on tickets and customers

- `gorgias-pp-cli custom-fields create` — Define a new custom field on tickets or customers (the only supported `object_type` values). Required body:...
- `gorgias-pp-cli custom-fields get` — Fetch a single custom field definition by `id`, returning its data type, label, target object, and option list. Use...
- `gorgias-pp-cli custom-fields list` — List custom field definitions for a single `object_type` (`Ticket` or `Customer` — REQUIRED query param)....
- `gorgias-pp-cli custom-fields update` — Update one custom field definition by `id` — relabel it, change its options, or toggle visibility. Note: this...
- `gorgias-pp-cli custom-fields update-all` — Bulk-update multiple custom field definitions in one call (no path id). Useful when reordering picklist options or...

**customers** — Read and write Gorgias customer records (CRM core)

- `gorgias-pp-cli customers create` — Create a new customer record. Pass `name`, `email`, optional `channels` (email/phone/social handles), and `data` for...
- `gorgias-pp-cli customers custom-fields-list` — List every custom field value attached to a single customer (`id`). Use to read CRM-style attributes (lifetime...
- `gorgias-pp-cli customers custom-fields-set` — Set a single custom field value on a customer: first `{id}` is the customer, second `{id}` is the custom field. Use...
- `gorgias-pp-cli customers custom-fields-set-all` — Bulk-set custom field values on a single customer (`id`) — pass an array of field/value pairs. Preferred over the...
- `gorgias-pp-cli customers custom-fields-unset` — Clear a custom field value on a customer: first `{id}` is the customer ID, second `{id}` is the custom field ID. Use...
- `gorgias-pp-cli customers data-update` — Set a customer's `data` blob (`id` in path). Body: `data` (required) plus optional `version` for last-write-wins...
- `gorgias-pp-cli customers delete` — Delete one customer by `id`. Hard-deletes the record and may cascade to associated tickets/messages depending on...
- `gorgias-pp-cli customers delete-all` — Bulk-delete customers. Required body: `ids` (array of customer IDs to delete). Does NOT accept query-style filters...
- `gorgias-pp-cli customers get` — Fetch a single customer by `id`, including their channels (email, phone, social handles), `data` blob, and...
- `gorgias-pp-cli customers list` — List customers with pagination and optional filter params (`email`, `external_id`, `name`, `language`,...
- `gorgias-pp-cli customers merge` — Merge one customer into another. Required query params: `source_id` (the duplicate, will be merged in and deleted)...
- `gorgias-pp-cli customers update` — Update a customer (`id`) — change name, add/remove channels, edit external IDs, or overwrite top-level fields. Use...

**events** — Audit log of who-changed-what across tickets, customers, settings

- `gorgias-pp-cli events get` — Retrieve a single audit event by `id` — captures who/what/when on ticket, customer, or settings mutations. Use to...
- `gorgias-pp-cli events list` — List audit events. Documented filters: `object_type` (e.g. Ticket/Customer/User), `object_id`, `user_ids` (actor),...

**gorgias-jobs** — Schedule and track async Gorgias jobs (bulk exports, macro applies)

- `gorgias-pp-cli gorgias-jobs create` — Kick off an asynchronous Gorgias job. Required body: `type` (enum: applyMacro, deleteTicket, exportTicket,...
- `gorgias-pp-cli gorgias-jobs delete` — Delete a job record by `id`. Useful for cleaning up completed or failed entries from listings; does not cancel an...
- `gorgias-pp-cli gorgias-jobs get` — Fetch a single async job (`id`) with its status, progress, params, and result/error fields. The polling endpoint...
- `gorgias-pp-cli gorgias-jobs list` — List async jobs with filters by type, status, and datetime. Use to find a recent export job by an agent or to...
- `gorgias-pp-cli gorgias-jobs update` — Update an async job (`id`) — typically to cancel it or adjust metadata. Reach for this only when you need to abort...

**integrations** — Install and configure third-party integrations (Shopify, SMS, social)

- `gorgias-pp-cli integrations create` — Install a new third-party integration on the Gorgias account (Shopify, Instagram, SMS provider, etc.). Pass `type`...
- `gorgias-pp-cli integrations delete` — Uninstall an integration by `id`. Destructive — disconnects the channel and may stop syncing orders/messages from...
- `gorgias-pp-cli integrations get` — Fetch a single integration (`id`) including its type, status, last-sync time, and provider-specific config. Use to...
- `gorgias-pp-cli integrations list` — List all installed integrations on the account — Shopify, Magento, Facebook, voice, etc. Use to discover what...
- `gorgias-pp-cli integrations update` — Update an integration's config (`id`) — refresh credentials, toggle sync features, or rename. Reach for this when...

**macros** — Reusable canned-reply templates with variables and actions

- `gorgias-pp-cli macros archive` — Archive one or more macros (soft delete) — pass macro IDs in the body. Use this rather than `macros_delete` to...
- `gorgias-pp-cli macros create` — Create a new macro: a reusable reply/action template. Required body: `name`. Optional: `intent`, `language`,...
- `gorgias-pp-cli macros delete` — Delete a macro by `id`. Hard-deletes it from the macro library. Prefer `macros_archive` for soft removal so...
- `gorgias-pp-cli macros get` — Fetch a single macro by `id`, returning its body, actions, and variable definitions. Use before applying a macro so...
- `gorgias-pp-cli macros list` — List all macros, with optional filters (archived, name). The agent's discovery endpoint for available canned replies...
- `gorgias-pp-cli macros unarchive` — Unarchive one or more macros, restoring them to the active library. Pass macro IDs in the body. The companion to...
- `gorgias-pp-cli macros update` — Update a macro (`id`) — edit its body, variables, tags-to-add, or action list. Use when an agent is refining a...

**messages** — Read messages across tickets (account-wide listing)

- `gorgias-pp-cli messages` — List messages account-wide, paginated. Supported filters are `ticket_id` only (plus `cursor`, `limit`, `order_by`);...

**phone** — Voice calls, call events, and recorded audio

- `gorgias-pp-cli phone call-events-get` — Fetch a single voice-call event by `id` — events capture call lifecycle (ringing, answered, hung-up, transferred)....
- `gorgias-pp-cli phone call-events-list` — List voice-call lifecycle events. Documented filter is `call_id` only (plus `cursor`, `limit`). Use to inspect the...
- `gorgias-pp-cli phone call-recordings-delete` — Delete a stored voice-call recording by `id`. Use to honor a customer privacy/erasure request or to scrub a test...
- `gorgias-pp-cli phone call-recordings-get` — Fetch metadata for a single call recording (`id`) — duration, URL, related call/ticket. Pair with...
- `gorgias-pp-cli phone call-recordings-list` — List voice-call recordings. Documented filter is `call_id` only (plus `cursor`, `limit`). Use to find the...
- `gorgias-pp-cli phone calls-get` — Fetch a single voice call (`id`) with direction, status, duration, participants, and the linked ticket. Use when an...
- `gorgias-pp-cli phone calls-list` — List voice calls, paginated. Documented filter is `ticket_id` only (plus `cursor`, `limit`, `order_by`). Use to...

**pickups** — Delete pickup logistics records (single destructive endpoint)

- `gorgias-pp-cli pickups <id>` — Delete a pickup record by `id`. Counterpart to `pickups_create_pickups`; use to cancel or remove a stale logistics...

**reporting** — Run a Gorgias analytics query (POST /reporting/stats)

- `gorgias-pp-cli reporting` — Run a Gorgias analytics query: POST a JSON body with `metric`, `dimensions`, `filters`, and a `period`. The single...

**rules** — Automation rules: route, tag, auto-reply, escalate on incoming tickets

- `gorgias-pp-cli rules create` — Create a new automation rule. Required body: `name` and `code` (the rule logic written as JavaScript). Optional:...
- `gorgias-pp-cli rules delete` — Delete an automation rule by `id`. Stops the rule from firing on future tickets but does not undo past actions. Use...
- `gorgias-pp-cli rules get` — Fetch a single automation rule (`id`) with its full conditions/actions tree and enabled state. Use to inspect why a...
- `gorgias-pp-cli rules list` — List all automation rules with their order, enabled state, and summary. The agent's map of what automations are...
- `gorgias-pp-cli rules set-priorities` — Set the execution priorities of automation rules. Required body: `priorities` — an array of objects mapping rule...
- `gorgias-pp-cli rules update` — Update an automation rule (`id`) — edit conditions, actions, or enabled flag. Use to tune an existing workflow...

**satisfaction-surveys** — CSAT survey definitions and customer ratings/comments

- `gorgias-pp-cli satisfaction-surveys create` — Create a satisfaction-survey instance attached to one ticket and customer. Required body: `customer_id`,...
- `gorgias-pp-cli satisfaction-surveys get` — Fetch a single satisfaction-survey instance by `id` — the linked ticket/customer, score, customer comment, and...
- `gorgias-pp-cli satisfaction-surveys list` — List satisfaction-survey instances (each one tied to a single ticket). Filter with `ticket_id` to fetch the survey...
- `gorgias-pp-cli satisfaction-surveys update` — Update a satisfaction-survey instance (`id`) — typically to record/correct the customer's `score` (1–5),...

**tags** — Ticket tags — the labels that drive routing rules and reporting

- `gorgias-pp-cli tags create` — Create a new tag in the account's tag library. Body: `name` (required, max 256 chars, case-sensitive), `description`...
- `gorgias-pp-cli tags delete` — Delete a tag by `id`. Removes it from the library and unassociates it from every ticket/customer that carries it....
- `gorgias-pp-cli tags delete-all` — Bulk-delete tags. Required body: `ids` (array of tag IDs, min 1). Tags currently referenced by macros or rules...
- `gorgias-pp-cli tags get` — Fetch a single tag (`id`) with its name, decoration, and metadata. Use to verify a tag exists before applying it, or...
- `gorgias-pp-cli tags list` — List all tags in the account, optionally filtered by name. The agent's lookup endpoint for finding the right...
- `gorgias-pp-cli tags merge` — Merge other tags INTO this tag — path `{id}` is the destination (surviving) tag, and the body field...
- `gorgias-pp-cli tags update` — Update a tag (`id`) — rename it or change its color/decoration. Affects every record currently carrying the tag,...

**teams** — Agent teams: how tickets are grouped and routed for assignment

- `gorgias-pp-cli teams create` — Create a new team (group of agents) in the account. Pass `name` and optionally members. Use when organizing routing...
- `gorgias-pp-cli teams delete` — Delete a team by `id`. Removes it from routing rules and views; members remain but lose the team grouping. Use when...
- `gorgias-pp-cli teams get` — Fetch a single team (`id`) with its members and metadata. Use when an agent needs to know who's on a team before...
- `gorgias-pp-cli teams list` — List all teams in the account. The agent's lookup for valid team IDs/names when assigning a ticket, routing via a...
- `gorgias-pp-cli teams update` — Update a team (`id`) — rename it or change its membership. Use to reorganize agents or correct a misconfigured team.

**ticket-search** — Search across tickets, customers, messages, etc.

- `gorgias-pp-cli ticket-search` — Full-text search across Gorgias tickets, customers, and messages. POST a JSON body with `query`, `resource_type`,...

**tickets** — Read and write Gorgias tickets, messages, and tag assignments

- `gorgias-pp-cli tickets create` — Create a new ticket. Body specifies `channel`, `via`, `subject`, an initial `messages` array, the customer, and...
- `gorgias-pp-cli tickets custom-fields-list` — List every custom field value on ticket (`id`). Use to read structured metadata an agent or integration attached...
- `gorgias-pp-cli tickets custom-fields-set` — Set a single custom field value on a ticket: first `{id}` is the ticket, second `{id}` is the custom field. Use to...
- `gorgias-pp-cli tickets custom-fields-set-all` — Bulk-set custom field values on ticket (`id`) — pass an array of field/value pairs. Preferred when an agent needs...
- `gorgias-pp-cli tickets custom-fields-unset` — Clear a custom field value on a ticket: first `{id}` is the ticket, second `{id}` is the custom field. Unsets (does...
- `gorgias-pp-cli tickets delete` — Delete a ticket by `id`. Hard-deletes the conversation and its messages — reserve for GDPR erasure, spam, or...
- `gorgias-pp-cli tickets get` — Fetch a single ticket by `id` with status, channel, assignee, customer, tags, and summary fields. Use after...
- `gorgias-pp-cli tickets list` — List tickets with filters (status, assignee, customer, channel, datetime, tag). The agent's primary endpoint for...
- `gorgias-pp-cli tickets messages-create` — Post a new message on ticket (`id`) — used to reply to the customer or write an internal note. The body...
- `gorgias-pp-cli tickets messages-delete` — Delete a message from a ticket: first `{id}` is the ticket, second `{id}` is the message. Use sparingly —...
- `gorgias-pp-cli tickets messages-get` — Fetch a single message: first `{id}` is the ticket, second `{id}` is the message. Use to load full body and...
- `gorgias-pp-cli tickets messages-list` — List all messages on ticket (`id`) in chronological order — both customer-sent and agent-sent, public and internal...
- `gorgias-pp-cli tickets messages-update` — Update a message: first `{id}` is the ticket, second `{id}` is the message. Typically used to edit an internal...
- `gorgias-pp-cli tickets tags-add` — Add one or more tags to ticket (`id`). The body shape (tag IDs vs names; whether unknown names auto-create) is not...
- `gorgias-pp-cli tickets tags-list` — List the tags currently attached to ticket (`id`). Use to read the ticket's categorization before deciding what...
- `gorgias-pp-cli tickets tags-remove` — Remove tags from ticket (`id`). Pass the tag IDs/names to detach. Use when re-categorizing a ticket or undoing an...
- `gorgias-pp-cli tickets tags-replace` — Replace ticket (`id`)'s entire tag set with the supplied list. Use for full re-tagging; for additive/subtractive...
- `gorgias-pp-cli tickets update` — Update a ticket (`id`) — change status (`open`/`closed`/`resolved`), assignee, priority, subject, or `via`. The...

**users** — Agents and admin users on the Gorgias account

- `gorgias-pp-cli users create` — Create a new user (Gorgias agent/operator). Pass name, email, role, and optionally team memberships. Use when...
- `gorgias-pp-cli users delete` — Delete a user (`id`) — deactivates the agent account and removes them from routing. Their historical ticket...
- `gorgias-pp-cli users get` — Fetch a single user (`id`) — agent name, email, role, teams, status. Use to look up who an assignee is or to...
- `gorgias-pp-cli users list` — List users (Gorgias agents/operators) on the account, with filters for role, status, and team. The agent's lookup...
- `gorgias-pp-cli users update` — Update a user (`id`) — change role, name, team membership, or active state. Use for admin operations like...

**views** — Saved Gorgias inbox views (named filters used by agents)

- `gorgias-pp-cli views create` — Create a saved view — a filtered ticket list (e.g. 'My open tickets', 'Urgent + unassigned') defined by...
- `gorgias-pp-cli views delete` — Delete a saved view by `id`. Removes it from the sidebar for everyone who saw it. Use when retiring stale filters.
- `gorgias-pp-cli views get` — Fetch a single saved view (`id`) with its filter definition and metadata. Use to introspect what conditions a view...
- `gorgias-pp-cli views items-list` — Return the ticket items currently matching saved view (`id`). Required: `id` (path). Optional: `cursor`, `direction`...
- `gorgias-pp-cli views items-update` — Update the materialized items of a view (`id`) — used to reorder, bulk-mutate, or refresh the cached set depending...
- `gorgias-pp-cli views list` — List all saved views on the account, including ownership and visibility. The agent's catalogue of pre-built ticket...
- `gorgias-pp-cli views update` — Update a saved view (`id`) — change its filter criteria, name, or sharing. Use to evolve a view's definition as...

**widgets** — Configure on-site chat/contact widget instances

- `gorgias-pp-cli widgets create` — Create a new agent-facing sidebar widget shown inside the Gorgias helpdesk (on ticket, customer, or user views —...
- `gorgias-pp-cli widgets delete` — Delete a sidebar widget config by `id`. After deletion the widget stops rendering in the helpdesk UI on the...
- `gorgias-pp-cli widgets get` — Fetch a single sidebar widget (`id`) with its `context` (ticket/customer/user), `template` (data source), order, and...
- `gorgias-pp-cli widgets list` — List all agent-facing sidebar widgets on the account, optionally filtered by `integration_id` or `app_id`. Use to...
- `gorgias-pp-cli widgets update` — Update a sidebar widget (`id`) — typically to change its `template` (data source), `order` (display position),...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
gorgias-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Sync recent tickets to a local SQLite mirror

```bash
gorgias-pp-cli sync --resources tickets --since 7d
```

Hydrates the local DB so subsequent searches and analytics run offline against a recent snapshot instead of hitting the API on every call.

For tickets, `--since` is implemented as documented newest-first ordering plus a local cutoff. A live test on May 23, 2026 showed `updated_datetime__gte` returns HTTP 400 `Unknown field`, so agents should not try to "repair" ticket sync with that parameter.

### Full-text search across synced tickets

```bash
gorgias-pp-cli search 'refund cancellation' --limit 10 --agent
```

Searches the local SQLite mirror for tickets matching the query — much faster than the live API for repeated lookups.

### Stream live ticket changes

```bash
gorgias-pp-cli tail --interval 30s --agent
```

Polls the API on the given interval and emits one JSON line per new or updated ticket — useful for triage agents that should react to changes.

### Find tickets with no assignee

```bash
gorgias-pp-cli orphans --agent
```

Lists items missing key fields like assignee or project — the standard 'unowned work' query for support leads.

### Find stale tickets

```bash
gorgias-pp-cli stale --days 7 --agent
```

Lists items with no updates in the last 7 days. Pairs well with `orphans` for an oncall review.

### Run a compound workflow

```bash
gorgias-pp-cli workflow --agent
```

Lists available compound workflows that combine multiple API operations into one bounded payload.

## Auth Setup

Gorgias uses HTTP Basic auth — your account email as username and an API key from Settings → Account → REST API as password. Set `GORGIAS_USERNAME`, `GORGIAS_API_KEY`, and `GORGIAS_BASE_URL` (https://<tenant>.gorgias.com/api). How those env vars get into the process is up to you — a shell profile, a secrets manager, a CI secret store; the CLI doesn't care.

Run `gorgias-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  gorgias-pp-cli account get --agent --select id,name,status
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

### Error envelope

Under `--json` (or `--agent`), failures emit a single JSON document on stderr:

```json
{"error": {"message": "<human-readable cause>", "exit_code": <int>}}
```

Exit codes match the table in [Exit Codes](#exit-codes) below. Commands that
naturally carry status in their own JSON body (e.g. `doctor --json`) embed
the failure inside the report instead, and stderr is empty — `jq` always sees
a single JSON document per stream.

### Configuration discovery

In addition to `GORGIAS_USERNAME`, `GORGIAS_API_KEY`, and `GORGIAS_BASE_URL`, the
CLI honors `GORGIAS_CONFIG` (path to a TOML config file, default
`$XDG_CONFIG_HOME/gorgias-pp-cli/config.toml`) and the standard XDG variables
`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`. On Windows, the CLI falls
back to `os.UserConfigDir()` / `os.UserCacheDir()` (`%APPDATA%`,
`%LOCALAPPDATA%`) when the XDG vars are unset.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
gorgias-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
gorgias-pp-cli feedback --stdin < notes.txt
gorgias-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.gorgias-pp-cli/feedback.jsonl`. They are never POSTed unless `GORGIAS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GORGIAS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration

```
gorgias-pp-cli profile save briefing --json
gorgias-pp-cli --profile briefing account get
gorgias-pp-cli profile list --json
gorgias-pp-cli profile show briefing
gorgias-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `gorgias-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add gorgias-pp-mcp -- gorgias-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which gorgias-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   gorgias-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `gorgias-pp-cli <command> --help`.
