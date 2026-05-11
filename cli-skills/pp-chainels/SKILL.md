---
name: pp-chainels
description: "Every Chainels endpoint as a typed command, plus the cross-community search, turnover variance, and stale-issue... Trigger phrases: `search chainels for`, `show stale chainels issues`, `who hasn't filed turnover`, `chainels renewals coming up`, `diff chainels alarm config`, `chainels member audit`, `use chainels`, `run chainels`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - chainels-pp-cli
---

# Chainels — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `chainels-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install chainels --cli-only
   ```
2. Verify: `chainels-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/chainels/cmd/chainels-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Chainels' web UI scopes everything to one community at a time. This CLI syncs every community you can see into a local SQLite store and adds the joins that matter: cross-community FTS, issue-assignee load, turnover laggards/variance, agreement renewals, member-load audit, alarm diffs. Live API calls flow through OAuth2 client_credentials so it works headless in CI and agent contexts.

## When to Use This CLI

Reach for chainels-pp-cli when you manage more than one Chainels community and need answers the web UI can't produce in one query — cross-community search, who-hasn't-submitted lists, renewal calendars, and member-load audits. It is also the right tool for Yardi/Entrata integrators who need a delta-sync primitive and machine-readable joins between agreements, accounts, and roles.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`search`** — Search across every community's messages, issues, agreements, and timeline posts from one terminal.

  _Reach for this when you need to find one tenant note, one issue, or one agreement across many properties without round-trips to the API._

  ```bash
  chainels-pp-cli search "lift maintenance" --json --select community_id,resource_type,title
  ```
- **`issues load`** — Group open issues by assignee with age buckets so you can spot who's drowning before sprint planning.

  _Pick this when a property manager asks who has the oldest issues open or which assignee needs help before Monday triage._

  ```bash
  chainels-pp-cli issues load --community 42 --json
  ```
- **`issues stale`** — List issues with no state transition for N days across every community.

  _Use this when a community manager needs a Monday escalation list across a portfolio._

  ```bash
  chainels-pp-cli issues stale --older-than 14d --json
  ```
- **`turnover variance`** — Compute per-tenant variance vs trailing-N-month median for retail turnover reports.

  _Pick this when a landlord wants to spot a tenant whose sales dropped relative to that tenant's own normal._

  ```bash
  chainels-pp-cli turnover variance --months 12 --json
  ```
- **`turnover pending`** — List tenants who haven't submitted a turnover report for a given period.

  _Use this on the 6th of every month to chase the missing submissions across a portfolio._

  ```bash
  chainels-pp-cli turnover pending --period 2026-04 --json
  ```
- **`agreements renewals`** — List lease/agreement records whose end-of-term falls inside an N-day window.

  _Pick this when a renewal calendar is needed by Friday and the manager has more than one community to sweep._

  ```bash
  chainels-pp-cli agreements renewals --within 90d --json
  ```
- **`members audit`** — Per-account role count across entities + flag duplicates and orphans.

  _Use this when an integrator (Yardi/Entrata) is reconciling who has what role and where, and the answer must be machine-readable._

  ```bash
  chainels-pp-cli members audit --community 42 --json
  ```

### Agent-native plumbing
- **`alarms diff`** — Diff alarm-configuration recipient lists between two communities or two snapshots.

  _Reach for this when checking drift between sister buildings or before/after a config change._

  ```bash
  chainels-pp-cli alarms diff 42 43 --json
  ```
- **`changed`** — Union of rows where synced_at >= --since, grouped by resource, across the local store.

  _Pick this for integrators (Yardi/Entrata) who run weekly deltas and need a single "what's new" report across resources._

  ```bash
  chainels-pp-cli changed --since 2026-05-01 --json
  ```

## Command Reference

**accounts** — Endpoints related to accounts

- `chainels-pp-cli accounts edit` — Edit an account. It is a PATCH, so you only include the fields you wish to change
- `chainels-pp-cli accounts get` — Get the account information of the authenticated user.
- `chainels-pp-cli accounts get-my` — Get the account information of the authenticated user.

**agreements** — Endpoints related to agreements.

- `chainels-pp-cli agreements delete` — Delete an existing agreement
- `chainels-pp-cli agreements delete-item` — Remove an agreement item from an existing agreement.
- `chainels-pp-cli agreements get` — Get an agreement
- `chainels-pp-cli agreements get-item` — Get a specific item in an agreement.
- `chainels-pp-cli agreements update` — Update an existing agreement in a community
- `chainels-pp-cli agreements update-item` — Update an existing item within an agreement.

**alams** — Manage alams


**alarms** — Endpoints related to the managing, sending and replying to, alarms. This is only relevant if the community in question has activated the Alarm module

- `chainels-pp-cli alarms add-recipients` — Add companies to alarm recipients list of the authenticated account. The users of the added companies will be...
- `chainels-pp-cli alarms create` — Create and send an alarm to the alarm contact list. This will be created on behalf of the current authenticated...
- `chainels-pp-cli alarms get` — Get a list of alarms you have created, or have been sent to you.
- `chainels-pp-cli alarms get-alarmid` — Get a specific alarm chat
- `chainels-pp-cli alarms get-recipients` — Returns the list of both default recipients specified by the community, and extra recipients the current...
- `chainels-pp-cli alarms remove-recipients` — Remove companies from the alarm recipients list of the authenticated account.

**bans** — Manage bans

- `chainels-pp-cli bans get` — Get a StoreBan resource by id
- `chainels-pp-cli bans get-communities` — Get the communities that have a specific ban service enabled

**booking** — Endpoints related to retrieving and creating bookings and bookable objects

- `chainels-pp-cli booking delete` — Delete a booking
- `chainels-pp-cli booking delete-reply` — Delete a booking reply
- `chainels-pp-cli booking get` — Get a booking by its id.
- `chainels-pp-cli booking get-bookable` — Get a bookable object by its id.
- `chainels-pp-cli booking get-reply` — Get an individual reply resource.
- `chainels-pp-cli booking get-slots` — Get the slots of a bookable between the given time range. If both `from` and `to` are left empty, the endpoint will...
- `chainels-pp-cli booking update-approval-status` — Change the approval status of a booking.

**communities** — Manage communities


**companies** — Manage companies

- `chainels-pp-cli companies edit-entity` — Update an existing company. It is a PATCH, so you only include the fields you wish to change
- `chainels-pp-cli companies get-company` — Get a company. This endpoint returns the complete company object. A community profile is also a company, when...

**discounts** — Endpoints related to retrieving and creating discounts

- `chainels-pp-cli discounts edit` — Edit an existing discount submission. You only need to provide the fields you wish to update.
- `chainels-pp-cli discounts get` — Get a discount by its id.

**drive** — Manage drive

- `chainels-pp-cli drive <resource_id>` — Returns a file or directory.

**entities** — Manage entities


**invite-templates** — Endpoints related to managing invite templates

- `chainels-pp-cli invite-templates delete` — Permanently delete an invite template.
- `chainels-pp-cli invite-templates get` — Retrieve a single invite template by ID.
- `chainels-pp-cli invite-templates update` — Update an existing invite template. Supports partial update — only provided fields are changed.

**invoices** — Endpoints related to invoices.

- `chainels-pp-cli invoices delete` — Permanently delete an invoice. This action cannot be undone. Only managers can delete invoices.
- `chainels-pp-cli invoices get` — Get a single invoice by its ID
- `chainels-pp-cli invoices update` — Update an existing invoice. This is a PATCH operation - only provided fields are updated. Immutable fields (id,...

**issues** — Manage issues

- `chainels-pp-cli issues delete` — Delete an issue by its id.
- `chainels-pp-cli issues get` — Get an issue by its id.

**messages** — Endpoints related to retrieving and creating messages

- `chainels-pp-cli messages delete` — Delete a message
- `chainels-pp-cli messages edit` — Edit an existing message. The ID of the edited message changes when you change its publishing status.
- `chainels-pp-cli messages get` — Get a message by its id. Messages have multiple types, and depending on the type you might receive one of more extra...

**metrics** — Manage metrics


**payments** — Endpoints related to payments.

- `chainels-pp-cli payments delete` — Permanently delete a payment. This action cannot be undone. Only managers can delete payments.
- `chainels-pp-cli payments get` — Get a single payment
- `chainels-pp-cli payments update` — Update an existing payment. This is a PATCH operation - only provided fields are updated. Immutable fields (id,...

**replies** — Endpoints related to replies and comments, and making nested replies. These apply to all features where you can reply (messages, issues, form submissions, etc)

- `chainels-pp-cli replies delete-reply` — Remove a reply
- `chainels-pp-cli replies get-reply` — Get an individual reply resource.

**reporting** — Manage reporting

- `chainels-pp-cli reporting delete-periodic-report` — Delete a submitted periodic report. Only reports with status `filled_in` can be deleted. Requires management-level...
- `chainels-pp-cli reporting edit-periodic-report` — Edit an existing periodic report. Only reports with status `filled_in` can be edited. The member must have edit...
- `chainels-pp-cli reporting get-all-periodic-reports-of-period` — Get all the non-open periodic reports for a given period and scheme
- `chainels-pp-cli reporting get-open-periodic-reports-of-period` — Get all the open periodic reports for a given period and scheme
- `chainels-pp-cli reporting get-periodic-report` — Get a periodic report by its id. Supports both: - Numeric ID for saved reports (e.g., '12345') - Alternative ID for...
- `chainels-pp-cli reporting get-periodic-scheme` — Get a periodic reporting scheme by its id.
- `chainels-pp-cli reporting get-periodic-scheme-periods` — Get the periods of a periodic reporting scheme
- `chainels-pp-cli reporting get-periodic-statistics` — Returns statistics counters for a given scheme and period, including total members, submitted reports count, and...
- `chainels-pp-cli reporting get-reminders-of-period` — Returns the reminders for a given scheme and period with their exact scheduled date and time.
- `chainels-pp-cli reporting remove-member-from-period` — Remove a member from a period's target list. If the member has a submitted report for that period, the report is...
- `chainels-pp-cli reporting save-periodic-report` — Submit a periodic report for a specific scheme and period
- `chainels-pp-cli reporting send-reminder-notification` — Sends a reminder notification to a specific member for a given scheme and period.

**requests** — Manage requests

- `chainels-pp-cli requests create-submission` — Create a new submission for a request form
- `chainels-pp-cli requests delete-submission-reply` — Delete a submission reply
- `chainels-pp-cli requests edit-submission` — Edit an existing submission. All answers must be included again, partial updates are not supported.
- `chainels-pp-cli requests get-form` — Get a request form by its id.
- `chainels-pp-cli requests get-submission` — Get a request submission by its id.
- `chainels-pp-cli requests get-submission-replies` — Get the replies of the specified submission. We return by default 10 per page (max 30 per page)
- `chainels-pp-cli requests get-submission-reply` — Get an individual reply resource.
- `chainels-pp-cli requests save-submission-reply` — Reply to a submission.
- `chainels-pp-cli requests update-submission-status` — Apply a transition to change the status of a submission.

**service-accounts** — Endpoints for managing service accounts (non-human API accounts).

- `chainels-pp-cli service-accounts delete` — Permanently delete a service account and all its OAuth clients
- `chainels-pp-cli service-accounts delete-client` — Permanently delete an OAuth client belonging to a service account.
- `chainels-pp-cli service-accounts get` — Get a single service account by id
- `chainels-pp-cli service-accounts get-client` — Returns a single service account OAuth client by ID. The client secret is never returned in get responses.
- `chainels-pp-cli service-accounts get-client-scopes` — Returns the scopes available for selection when creating a service account OAuth client.
- `chainels-pp-cli service-accounts rotate-client-secret` — Generate a new secret for a service account OAuth client. Returns the client with the new plain-text secret. The...
- `chainels-pp-cli service-accounts update` — Update properties of a service account
- `chainels-pp-cli service-accounts update-client` — Update a service account OAuth client's name and/or scopes.

**spaces** — Endpoints related to spaces.

- `chainels-pp-cli spaces get` — Get a space
- `chainels-pp-cli spaces update` — Update an existing space in a community

**storeban** — Manage storeban

- `chainels-pp-cli storeban` — Get the communities that use the collective store ban module on Chainels

**turnover** — Manage turnover

- `chainels-pp-cli turnover get-all-reports-of-period` — Get all the non-open turnover reports for a given period and scheme
- `chainels-pp-cli turnover get-open-reports-of-period` — Get all the open turnover reports for a given period and scheme
- `chainels-pp-cli turnover get-report` — Get a turnover report by its id. Supports both: - Numeric ID for saved reports (e.g., '12345') - Alternative ID for...
- `chainels-pp-cli turnover get-scheme` — Get a turnover scheme by its id.
- `chainels-pp-cli turnover get-scheme-periods` — Get the periods of a turnover scheme
- `chainels-pp-cli turnover save-report` — Submit a turnover report for a specific scheme and period


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
chainels-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Monday triage across the portfolio

```bash
chainels-pp-cli issues stale --older-than 14d --json --select community_id,id,title,assignee,updated_at
```

Run after sync; produces the list a community manager needs before standup.

### Turnover follow-up on the 6th

```bash
chainels-pp-cli turnover pending --period 2026-04 --json --select community_id,tenant,location
```

Set-difference between expected submitters and actual reports for the closed period.

### Lease-renewal calendar with deep field select

```bash
chainels-pp-cli agreements renewals --within 90d --agent --select agreements.id,agreements.end_at,agreements.entity.name,agreements.community_id
```

Dotted --select narrows the nested agreement payload to renewal-relevant fields so an agent can pipe straight into a calendar.

### Drift check between two sister buildings

```bash
chainels-pp-cli alarms diff 42 43 --json
```

Diff alarm-configuration recipient lists between communities 42 and 43; output is a set-difference suitable for piping into jq.

### Weekly delta for a Yardi integrator

```bash
chainels-pp-cli changed --since 2026-05-01 --json
```

Single command unifies every resource's updated_at into one delta report for a downstream reconcile script.

## Auth Setup

Chainels uses OAuth 2.0 with three grant types: client_credentials for userless app access (recommended for this CLI; set CHAINELS_CLIENT_ID + CHAINELS_CLIENT_SECRET), authorization_code for user delegation, and a group_token grant that issues a token scoped to a group. Tokens are cached in the local config so the access_token endpoint isn't hit on every call.

Run `chainels-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  chainels-pp-cli accounts get mock-value --agent --select id,name,status
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
chainels-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
chainels-pp-cli feedback --stdin < notes.txt
chainels-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.chainels-pp-cli/feedback.jsonl`. They are never POSTed unless `CHAINELS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CHAINELS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
chainels-pp-cli profile save briefing --json
chainels-pp-cli --profile briefing accounts get mock-value
chainels-pp-cli profile list --json
chainels-pp-cli profile show briefing
chainels-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `chainels-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add chainels-pp-mcp -- chainels-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which chainels-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   chainels-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `chainels-pp-cli <command> --help`.
