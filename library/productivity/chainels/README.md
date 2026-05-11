# Chainels CLI

**Every Chainels endpoint as a typed command, plus the cross-community search, turnover variance, and stale-issue digests the web UI cannot give you.**

Chainels' web UI scopes everything to one community at a time. This CLI syncs every community you can see into a local SQLite store and adds the joins that matter: cross-community FTS, issue-assignee load, turnover laggards/variance, agreement renewals, member-load audit, alarm diffs. Live API calls flow through OAuth2 client_credentials so it works headless in CI and agent contexts.

## Install

The recommended path installs both the `chainels-pp-cli` binary and the `pp-chainels` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install chainels
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install chainels --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/chainels-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-chainels --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-chainels --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-chainels skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-chainels. The skill defines how its required CLI can be installed.
```

## Authentication

Chainels uses OAuth 2.0 with three grant types: client_credentials for userless app access (recommended for this CLI; set CHAINELS_CLIENT_ID + CHAINELS_CLIENT_SECRET), authorization_code for user delegation, and a group_token grant that issues a token scoped to a group. Tokens are cached in the local config so the access_token endpoint isn't hit on every call.

## Quick Start

```bash
# Confirm OAuth credentials, demo/prod reachability, and local store state.
chainels-pp-cli doctor


# Pull every community, account, message, issue, agreement, and turnover report into the local SQLite store.
chainels-pp-cli sync --full


# Grep across every synced community at once.
chainels-pp-cli search "lift maintenance" --json


# Monday triage list of stale issues across the portfolio.
chainels-pp-cli issues stale --older-than 14d --json


# Who hasn't filed for the current period.
chainels-pp-cli turnover pending --period 2026-04 --json

```

## Unique Features

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

## Usage

Run `chainels-pp-cli --help` for the full command reference and flag list.

## Commands

### accounts

Endpoints related to accounts

- **`chainels-pp-cli accounts edit`** - Edit an account. It is a PATCH, so you only include the fields you wish to change
- **`chainels-pp-cli accounts get`** - Get the account information of the authenticated user.
- **`chainels-pp-cli accounts get-my`** - Get the account information of the authenticated user.

### agreements

Endpoints related to agreements.

- **`chainels-pp-cli agreements delete`** - Delete an existing agreement
- **`chainels-pp-cli agreements delete-item`** - Remove an agreement item from an existing agreement.
- **`chainels-pp-cli agreements get`** - Get an agreement
- **`chainels-pp-cli agreements get-item`** - Get a specific item in an agreement.
- **`chainels-pp-cli agreements update`** - Update an existing agreement in a community
- **`chainels-pp-cli agreements update-item`** - Update an existing item within an agreement.

### alams

Manage alams


### alarms

Endpoints related to the managing, sending and replying to, alarms. This is only relevant if the community in question has activated the Alarm module

- **`chainels-pp-cli alarms add-recipients`** - Add companies to alarm recipients list of the authenticated account. The users of the added companies will be notified of this action.
- **`chainels-pp-cli alarms create`** - Create and send an alarm to the alarm contact list. This will be created on behalf of the current authenticated account with its current active entity.
Once the alarm is sent, a chat channel will be created for this alarm, where each recipient can reply to.
- **`chainels-pp-cli alarms get`** - Get a list of alarms you have created, or have been sent to you.
- **`chainels-pp-cli alarms get-alarmid`** - Get a specific alarm chat
- **`chainels-pp-cli alarms get-recipients`** - Returns the list of both default recipients specified by the community, and extra recipients the current authenticated account has specified.
- **`chainels-pp-cli alarms remove-recipients`** - Remove companies from the alarm recipients list of the authenticated account.

### bans

Manage bans

- **`chainels-pp-cli bans get`** - Get a StoreBan resource by id
- **`chainels-pp-cli bans get-communities`** - Get the communities that have a specific ban service enabled

### booking

Endpoints related to retrieving and creating bookings and bookable objects

- **`chainels-pp-cli booking delete`** - Delete a booking
- **`chainels-pp-cli booking delete-reply`** - Delete a booking reply
- **`chainels-pp-cli booking get`** - Get a booking by its id.
- **`chainels-pp-cli booking get-bookable`** - Get a bookable object by its id.
- **`chainels-pp-cli booking get-reply`** - Get an individual reply resource.
- **`chainels-pp-cli booking get-slots`** - Get the slots of a bookable between the given time range. If both `from` and `to` are left empty, the endpoint will return the upcoming `count` slots from the current time.
- **`chainels-pp-cli booking update-approval-status`** - Change the approval status of a booking.

### communities

Manage communities


### companies

Manage companies

- **`chainels-pp-cli companies edit-entity`** - Update an existing company. It is a PATCH, so you only include the fields you wish to change
- **`chainels-pp-cli companies get-company`** - Get a company. This endpoint returns the complete company object. A community profile is also a company, when `entity_type == "community"`

### discounts

Endpoints related to retrieving and creating discounts

- **`chainels-pp-cli discounts edit`** - Edit an existing discount submission. You only need to provide the fields you wish to update.
- **`chainels-pp-cli discounts get`** - Get a discount by its id.

### drive

Manage drive

- **`chainels-pp-cli drive get-resource`** - Returns a file or directory.

### entities

Manage entities


### invite-templates

Endpoints related to managing invite templates

- **`chainels-pp-cli invite-templates delete`** - Permanently delete an invite template.
- **`chainels-pp-cli invite-templates get`** - Retrieve a single invite template by ID.
- **`chainels-pp-cli invite-templates update`** - Update an existing invite template. Supports partial update — only provided fields are changed.

### invoices

Endpoints related to invoices.

- **`chainels-pp-cli invoices delete`** - Permanently delete an invoice. This action cannot be undone. Only managers can delete invoices.
- **`chainels-pp-cli invoices get`** - Get a single invoice by its ID
- **`chainels-pp-cli invoices update`** - Update an existing invoice. This is a PATCH operation - only provided fields are updated. Immutable fields (id, createdAt, updatedAt, community, entity) cannot be changed.

### issues

Manage issues

- **`chainels-pp-cli issues delete`** - Delete an issue by its id.
- **`chainels-pp-cli issues get`** - Get an issue by its id.

### messages

Endpoints related to retrieving and creating messages

- **`chainels-pp-cli messages delete`** - Delete a message
- **`chainels-pp-cli messages edit`** - Edit an existing message. The ID of the edited message changes when you change its publishing status.
- **`chainels-pp-cli messages get`** - Get a message by its id. Messages have multiple types, and depending on the type you might receive one of more extra properties.

### metrics

Manage metrics


### payments

Endpoints related to payments.

- **`chainels-pp-cli payments delete`** - Permanently delete a payment. This action cannot be undone. Only managers can delete payments.
- **`chainels-pp-cli payments get`** - Get a single payment
- **`chainels-pp-cli payments update`** - Update an existing payment. This is a PATCH operation - only provided fields are updated. Immutable fields (id, createdAt, updatedAt, community, entity) cannot be changed.

### replies

Endpoints related to replies and comments, and making nested replies. These apply to all features where you can reply (messages, issues, form submissions, etc)

- **`chainels-pp-cli replies delete-reply`** - Remove a reply
- **`chainels-pp-cli replies get-reply`** - Get an individual reply resource.

### reporting

Manage reporting

- **`chainels-pp-cli reporting delete-periodic-report`** - Delete a submitted periodic report. Only reports with status `filled_in` can be deleted.
Requires management-level permissions for the reporting service.
- **`chainels-pp-cli reporting edit-periodic-report`** - Edit an existing periodic report. Only reports with status `filled_in` can be edited.
The member must have edit permissions for the report based on the scheme's deadline settings.
- **`chainels-pp-cli reporting get-all-periodic-reports-of-period`** - Get all the non-open periodic reports for a given period and scheme
- **`chainels-pp-cli reporting get-open-periodic-reports-of-period`** - Get all the open periodic reports for a given period and scheme
- **`chainels-pp-cli reporting get-periodic-report`** - Get a periodic report by its id. Supports both:
- Numeric ID for saved reports (e.g., "12345")
- Alternative ID for open/unsaved reports in format `{entity_id}_{scheme_id}_{period_key}` (e.g., "789_456_2024-01")
- **`chainels-pp-cli reporting get-periodic-scheme`** - Get a periodic reporting scheme by its id.
- **`chainels-pp-cli reporting get-periodic-scheme-periods`** - Get the periods of a periodic reporting scheme
- **`chainels-pp-cli reporting get-periodic-statistics`** - Returns statistics counters for a given scheme and period, including total members, submitted reports count, and submission rate.
- **`chainels-pp-cli reporting get-reminders-of-period`** - Returns the reminders for a given scheme and period with their exact scheduled date and time.
- **`chainels-pp-cli reporting remove-member-from-period`** - Remove a member from a period's target list. If the member has a submitted report
for that period, the report is also deleted.
- **`chainels-pp-cli reporting save-periodic-report`** - Submit a periodic report for a specific scheme and period
- **`chainels-pp-cli reporting send-reminder-notification`** - Sends a reminder notification to a specific member for a given scheme and period.

### requests

Manage requests

- **`chainels-pp-cli requests create-submission`** - Create a new submission for a request form
- **`chainels-pp-cli requests delete-submission-reply`** - Delete a submission reply
- **`chainels-pp-cli requests edit-submission`** - Edit an existing submission. All answers must be included again, partial updates are not supported.
- **`chainels-pp-cli requests get-form`** - Get a request form by its id.
- **`chainels-pp-cli requests get-submission`** - Get a request submission by its id.
- **`chainels-pp-cli requests get-submission-replies`** - Get the replies of the specified submission. We return by default 10 per page (max 30 per page)
- **`chainels-pp-cli requests get-submission-reply`** - Get an individual reply resource.
- **`chainels-pp-cli requests save-submission-reply`** - Reply to a submission.
- **`chainels-pp-cli requests update-submission-status`** - Apply a transition to change the status of a submission.

### service-accounts

Endpoints for managing service accounts (non-human API accounts).

- **`chainels-pp-cli service-accounts delete`** - Permanently delete a service account and all its OAuth clients
- **`chainels-pp-cli service-accounts delete-client`** - Permanently delete an OAuth client belonging to a service account.
- **`chainels-pp-cli service-accounts get`** - Get a single service account by id
- **`chainels-pp-cli service-accounts get-client`** - Returns a single service account OAuth client by ID.
The client secret is never returned in get responses.
- **`chainels-pp-cli service-accounts get-client-scopes`** - Returns the scopes available for selection when creating a service account OAuth client.
- **`chainels-pp-cli service-accounts rotate-client-secret`** - Generate a new secret for a service account OAuth client. Returns the client with the new plain-text secret. The previous secret is immediately invalidated.
- **`chainels-pp-cli service-accounts update`** - Update properties of a service account
- **`chainels-pp-cli service-accounts update-client`** - Update a service account OAuth client's name and/or scopes.

### spaces

Endpoints related to spaces.

- **`chainels-pp-cli spaces get`** - Get a space
- **`chainels-pp-cli spaces update`** - Update an existing space in a community

### storeban

Manage storeban

- **`chainels-pp-cli storeban get-communities`** - Get the communities that use the collective store ban module on Chainels

### turnover

Manage turnover

- **`chainels-pp-cli turnover get-all-reports-of-period`** - Get all the non-open turnover reports for a given period and scheme
- **`chainels-pp-cli turnover get-open-reports-of-period`** - Get all the open turnover reports for a given period and scheme
- **`chainels-pp-cli turnover get-report`** - Get a turnover report by its id. Supports both:
- Numeric ID for saved reports (e.g., "12345")
- Alternative ID for open/unsaved reports in format `{entity_id}_{scheme_id}_{period_key}` (e.g., "789_456_2024-01")
- **`chainels-pp-cli turnover get-scheme`** - Get a turnover scheme by its id.
- **`chainels-pp-cli turnover get-scheme-periods`** - Get the periods of a turnover scheme
- **`chainels-pp-cli turnover save-report`** - Submit a turnover report for a specific scheme and period


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
chainels-pp-cli accounts get mock-value

# JSON for scripting and agents
chainels-pp-cli accounts get mock-value --json

# Filter to specific fields
chainels-pp-cli accounts get mock-value --json --select id,name,status

# Dry run — show the request without sending
chainels-pp-cli accounts get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
chainels-pp-cli accounts get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-chainels -g
```

Then invoke `/pp-chainels <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add chainels chainels-pp-mcp \
  -e CHAINELS_CLIENT_ID=<your-client-id> \
  -e CHAINELS_CLIENT_SECRET=<your-client-secret>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/chainels-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CHAINELS_CLIENT_ID` and `CHAINELS_CLIENT_SECRET` when Claude Desktop prompts you. (If you already have a pre-fetched bearer token, set `CHAINELS_OAUTH_CODE` instead.)

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "chainels": {
      "command": "chainels-pp-mcp",
      "env": {
        "CHAINELS_CLIENT_ID": "<your-client-id>",
        "CHAINELS_CLIENT_SECRET": "<your-client-secret>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
chainels-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/chainels-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CHAINELS_CLIENT_ID` | auth_flow_input | Conditional | OAuth2 client ID. Required when bootstrapping via `auth client-credentials` or `auth login`. Set this *or* `CHAINELS_OAUTH_CODE`. |
| `CHAINELS_CLIENT_SECRET` | auth_flow_input | Conditional | OAuth2 client secret. Paired with `CHAINELS_CLIENT_ID`. |
| `CHAINELS_OAUTH_CODE` | per_call | Conditional | A pre-fetched bearer access token. Set this if you already minted a token elsewhere and want to skip the auth flow. |
| `CHAINELS_BASE_URL` | per_call | No | Override the API base URL. Default `https://www.chainels.com/api/v2`; set to `https://demo.chainels.com/api/v2` to point at demo. |
| `CHAINELS_TOKEN_URL` | per_call | No | Override the OAuth2 token endpoint. Default `https://www.chainels.com/oauth/access_token`. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `chainels-pp-cli doctor` to check credentials.
- Verify your OAuth2 inputs: `echo $CHAINELS_CLIENT_ID` and (for client_credentials) that `$CHAINELS_CLIENT_SECRET` is also set; or, if you pre-minted a token, `echo $CHAINELS_OAUTH_CODE`.
- For headless use, run `chainels-pp-cli auth client-credentials` once to exchange the id/secret pair for an access_token cached in the local config.
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **invalid_client on token request** — Confirm CHAINELS_CLIENT_ID and CHAINELS_CLIENT_SECRET come from the same Chainels OAuth app, then run chainels-pp-cli doctor to re-fetch.
- **403 on /messages or /turnover endpoints** — The OAuth app may lack the required scope (write.messages, read.turnover). Check the app's scopes in the Chainels developer console.
- **search returns nothing** — Run chainels-pp-cli sync --full first; search reads from the local store and is empty until sync populates it.
- **demo vs prod confusion** — Set CHAINELS_BASE_URL=https://demo.chainels.com/api/v2 to point at demo; default is prod.

## Cookbook

Headless auth + sync + cross-community triage in three commands:

```bash
# 1. Userless token for CI / agent contexts (no browser).
chainels-pp-cli auth client-credentials \
  --client-id  "$CHAINELS_CLIENT_ID" \
  --client-secret "$CHAINELS_CLIENT_SECRET"

# 2. Pull every community / account / issue / agreement / turnover report.
chainels-pp-cli sync --full

# 3. Stale-issue digest across every synced community.
chainels-pp-cli issues stale --older-than 14d --json --select id,company,title,days_idle
```

Pipe `agreements renewals` into `jq` to feed a renewals calendar:

```bash
chainels-pp-cli agreements renewals --within 90d --json \
  | jq -r '.[] | [.end_date, .community_id, .entity_id, .name] | @tsv'
```

Drift-check alarm recipients between two communities:

```bash
chainels-pp-cli alarms diff "$COMMUNITY_A" "$COMMUNITY_B" --json \
  | jq '{only_in_a, only_in_b, common}'
```

Weekly delta for a Yardi/Entrata integrator (one query unions every resource):

```bash
chainels-pp-cli changed --since "$(date -v-7d +%Y-%m-%d)" --json
```

Turnover follow-up on the 6th:

```bash
chainels-pp-cli turnover pending --period "$(date -v-1m +%Y-%m)" --json
chainels-pp-cli turnover variance --months 12 --json
```

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
