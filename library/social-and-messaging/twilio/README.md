# Twilio CLI

**Every Twilio Core feature, plus offline message and call history, FTS, and SQL-grade analytics no other Twilio tool ships.**

Twilio's official CLI is a thin Node wrapper with 1–3 second cold start and no local cache. This CLI syncs your Messages, Calls, Recordings, UsageRecords, and IncomingPhoneNumbers into a local SQLite store you can grep, SQL, and aggregate offline. It ships every v2010 endpoint as a typed Cobra command, plus 12 novel features the Twilio ecosystem does not have: delivery-failure breakdowns, subaccount spend matrices, call-trace stitches, TCPA opt-out checks, idle-number reclamation, webhook-orphan audits, and more.

Learn more at [Twilio](https://support.twilio.com).

## Install

The recommended path installs both the `twilio-pp-cli` binary and the `pp-twilio` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install twilio
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install twilio --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/cmd/twilio-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/twilio-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-twilio --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-twilio --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-twilio skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-twilio. The skill defines how its required CLI can be installed.
```

## Authentication

Twilio uses HTTP Basic auth. You can authenticate with your Account SID (AC...) plus your Auth Token, OR with a scoped API Key SID (SK...) plus its Secret. The Account SID is always required because it is part of the URL path. Run `twilio-pp-cli doctor` to verify your credentials, detect whether the key is master or scoped, and warn on parent/subaccount mismatches.

## Quick Start

```bash
# Set credentials (HTTP Basic auth):
#   Account SID + Auth Token (master) — preferred when you control the account
export TWILIO_ACCOUNT_SID=ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
export TWILIO_AUTH_TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

#   Or scoped API Key (revocable without rotating the master token)
# export TWILIO_API_KEY_SID=SKxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
# export TWILIO_API_KEY_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# Verify auth wiring and connectivity. Reports auth_mode (master vs scoped) and
# warns on AC/SK prefix mismatches before any real work.
twilio-pp-cli doctor --json

# Local-only freshness check: which resources have data and how old is it?
twilio-pp-cli sync-status --json

# Sync the high-gravity tables before running analytics:
twilio-pp-cli sync --resources messages,calls,incoming-phone-numbers

# First analytic against the local store — failures by error code and destination country
twilio-pp-cli delivery-failures --since 7d --json

# Find phone numbers you are paying for but have not used in 30 days
twilio-pp-cli idle-numbers --since 30d --json

# Group IncomingPhoneNumbers by webhook URL to flag orphans (--probe to HEAD-check)
twilio-pp-cli webhook-audit --json
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`delivery-failures`** — See why messages failed last week, grouped by error code and destination country, with the dollar cost of those failures totaled.

  _When debugging delivery, agents need the failure distribution, not 4000 individual rows. This command answers 'why is delivery degraded' in one query._

  ```bash
  twilio-pp-cli delivery-failures --since 7d --json
  ```
- **`subaccount-spend`** — One CSV with every subaccount's last-month spend pivoted across SMS, MMS, voice, and recording categories — the agency-billing report nobody else ships.

  _For agency operators, this is the entire monthly billing report in one command. Agents can run it with --period thisMonth for a live forecast._

  ```bash
  twilio-pp-cli subaccount-spend --period last-month --csv > march-billing.csv
  ```
- **`message-status-funnel`** — Distribution of message terminal statuses (delivered, failed, undelivered, sent) with delivery-rate percentages and median time-to-delivery.

  _Operations dashboards need the funnel, not the raw rows. One query is the whole 'is delivery healthy right now' answer._

  ```bash
  twilio-pp-cli message-status-funnel --since 24h --json
  ```
- **`call-disposition`** — Cross-tab of call Status (completed, busy, no-answer, failed, canceled) by AnsweredBy (human, machine_start, fax), with dollar cost per bucket.

  _Outbound campaign tuning depends on knowing the human-pickup rate vs voicemail rate. Agents can answer this without paging through thousands of call records._

  ```bash
  twilio-pp-cli call-disposition --since 7d --json
  ```

### Cross-entity stitches
- **`call-trace`** — Given a CallSid, return everything that happened on that call: metadata, recordings, transcriptions, conference participation — in one structured output.

  _On-call engineers paged about a dropped call need the full picture in seconds, not three API round trips while the customer waits._

  ```bash
  twilio-pp-cli call-trace CAxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx --json
  ```
- **`idle-numbers`** — Phone numbers you are paying for but have not used to send or receive in N days — with the dollar amount you are wasting per month.

  _At $1/number/month, agencies running 100+ subaccount numbers can save hundreds annually. Agents can run it as a monthly cleanup task._

  ```bash
  twilio-pp-cli idle-numbers --since 30d --json
  ```
- **`webhook-audit`** — Group your IncomingPhoneNumbers by their Voice/SMS webhook URL to find single-use URLs that may be orphans pointing at deleted endpoints. Add --probe for a live HEAD check.

  _Orphan webhooks silently fail incoming calls and SMS. Quarterly audit is cheap insurance against silent dropped traffic._

  ```bash
  twilio-pp-cli webhook-audit --probe --json
  ```
- **`conversation`** — All messages and calls involving one phone number, merged into a single timestamped timeline with in/out arrows, body, and duration.

  _When investigating a customer complaint or fraud claim, the full back-and-forth is one query. Agents can answer 'what did we say to this number' without four searches._

  ```bash
  twilio-pp-cli conversation +14155551234 --json
  ```

### Compliance & audit
- **`opt-out-violations`** — Find numbers that texted STOP/UNSUBSCRIBE/END/QUIT and any messages your account sent to them afterwards — the TCPA exposure check the Twilio API has no resource for.

  _TCPA fines are $500–$1,500 per violation. Compliance teams audit for this quarterly; agents can spot-check it any time._

  ```bash
  twilio-pp-cli opt-out-violations --since 90d --json --select to,opt_out_at,subsequent_send_at,message_sid
  ```
- **`error-code-explain`** — Top error codes from your recent messages and calls, with curated one-line explanations and fixes for the most common Twilio errors.

  _Every Twilio user Googles error codes. Bundling the explanation with the count removes a constant context-switch._

  ```bash
  twilio-pp-cli error-code-explain --since 7d --json
  ```

### Agent-native plumbing
- **`sync-status`** — Last sync timestamp and row count per resource in the local store — so you know how fresh every other analytic command is.

  _Without freshness UX every analytic command is suspect. One read tells the agent whether to call sync first._

  ```bash
  twilio-pp-cli sync-status --json
  ```
- **`tail-messages`** — Stream new messages with a status filter as they happen — twilio-pp-cli's --follow that twilio-cli does not ship.

  _Incident triage and on-call response need a live feed of failures, not a polling shell loop. One command, no daemon._

  ```bash
  twilio-pp-cli tail-messages --status failed --follow
  ```

## Usage

Run `twilio-pp-cli --help` for the full command reference and flag list.

## Commands

### 2010-04-01

Manage 2010 04 01

- **`twilio-pp-cli 2010-04-01 create-account`** - Create a new Twilio Subaccount from the account making the request
- **`twilio-pp-cli 2010-04-01 fetch-account`** - Fetch the account specified by the provided Account Sid
- **`twilio-pp-cli 2010-04-01 list-account`** - Retrieves a collection of Accounts belonging to the account used to make the request
- **`twilio-pp-cli 2010-04-01 update-account`** - Modify the properties of a given Account

### addresses

Manage addresses

- **`twilio-pp-cli addresses delete-address`** - Delete address
- **`twilio-pp-cli addresses fetch-address`** - Fetch address
- **`twilio-pp-cli addresses update-address`** - Update address

### addresses-json

Manage addresses json

- **`twilio-pp-cli addresses-json create-address`** - Create address
- **`twilio-pp-cli addresses-json list-address`** - List address

### applications

Manage applications

- **`twilio-pp-cli applications delete`** - Delete the application by the specified application sid
- **`twilio-pp-cli applications fetch`** - Fetch the application specified by the provided sid
- **`twilio-pp-cli applications update`** - Updates the application's properties

### applications-json

Manage applications json

- **`twilio-pp-cli applications-json create-application`** - Create a new application within your account
- **`twilio-pp-cli applications-json list-application`** - Retrieve a list of applications representing an application within the requesting account

### authorized-connect-apps

Manage authorized connect apps

- **`twilio-pp-cli authorized-connect-apps fetch`** - Fetch an instance of an authorized-connect-app

### authorized-connect-apps-json

Manage authorized connect apps json

- **`twilio-pp-cli authorized-connect-apps-json list-authorized-connect-app`** - Retrieve a list of authorized-connect-apps belonging to the account used to make the request

### available-phone-numbers

Manage available phone numbers

- **`twilio-pp-cli available-phone-numbers fetch-country`** - Fetch country

### available-phone-numbers-json

Manage available phone numbers json

- **`twilio-pp-cli available-phone-numbers-json list-available-phone-number-country`** - List available phone number country

### balance-json

Manage balance json

- **`twilio-pp-cli balance-json fetch-balance`** - Fetch the balance for an Account based on Account Sid. Balance changes may not be reflected immediately. Child accounts do not contain balance information

### calls

Manage calls

- **`twilio-pp-cli calls delete`** - Delete a Call record from your account. Once the record is deleted, it will no longer appear in the API and Account Portal logs.
- **`twilio-pp-cli calls fetch`** - Fetch the call specified by the provided Call SID
- **`twilio-pp-cli calls update`** - Initiates a call redirect or terminates a call

### calls-json

Manage calls json

- **`twilio-pp-cli calls-json create-call`** - Create a new outgoing call to phones, SIP-enabled endpoints or Twilio Client connections
- **`twilio-pp-cli calls-json list-call`** - Retrieves a collection of calls made to and from your account

### conferences

Manage conferences

- **`twilio-pp-cli conferences fetch`** - Fetch an instance of a conference
- **`twilio-pp-cli conferences update`** - Update

### conferences-json

Manage conferences json

- **`twilio-pp-cli conferences-json list-conference`** - Retrieve a list of conferences belonging to the account used to make the request

### connect-apps

Manage connect apps

- **`twilio-pp-cli connect-apps delete`** - Delete an instance of a connect-app
- **`twilio-pp-cli connect-apps fetch`** - Fetch an instance of a connect-app
- **`twilio-pp-cli connect-apps update`** - Update a connect-app with the specified parameters

### connect-apps-json

Manage connect apps json

- **`twilio-pp-cli connect-apps-json list-connect-app`** - Retrieve a list of connect-apps belonging to the account used to make the request

### incoming-phone-numbers

Manage incoming phone numbers

- **`twilio-pp-cli incoming-phone-numbers create-local`** - Create local
- **`twilio-pp-cli incoming-phone-numbers create-mobile`** - Create mobile
- **`twilio-pp-cli incoming-phone-numbers create-toll-free`** - Create toll free
- **`twilio-pp-cli incoming-phone-numbers delete`** - Delete a phone-numbers belonging to the account used to make the request.
- **`twilio-pp-cli incoming-phone-numbers fetch`** - Fetch an incoming-phone-number belonging to the account used to make the request.
- **`twilio-pp-cli incoming-phone-numbers list-local`** - List local
- **`twilio-pp-cli incoming-phone-numbers list-mobile`** - List mobile
- **`twilio-pp-cli incoming-phone-numbers list-toll-free`** - List toll free
- **`twilio-pp-cli incoming-phone-numbers update`** - Update an incoming-phone-number instance.

### incoming-phone-numbers-json

Manage incoming phone numbers json

- **`twilio-pp-cli incoming-phone-numbers-json create-incoming-phone-number`** - Purchase a phone-number for the account.
- **`twilio-pp-cli incoming-phone-numbers-json list-incoming-phone-number`** - Retrieve a list of incoming-phone-numbers belonging to the account used to make the request.

### keys

Manage keys

- **`twilio-pp-cli keys delete`** - Delete
- **`twilio-pp-cli keys fetch`** - Fetch
- **`twilio-pp-cli keys update`** - Update

### keys-json

Manage keys json

- **`twilio-pp-cli keys-json create-new-key`** - Create new key
- **`twilio-pp-cli keys-json list-key`** - List key

### messages

Manage messages

- **`twilio-pp-cli messages delete`** - Deletes a Message resource from your account
- **`twilio-pp-cli messages fetch`** - Fetch a specific Message
- **`twilio-pp-cli messages update`** - Update a Message resource (used to redact Message `body` text and to cancel not-yet-sent messages)

### messages-json

Manage messages json

- **`twilio-pp-cli messages-json create-message`** - Send a message
- **`twilio-pp-cli messages-json list-message`** - Retrieve a list of Message resources associated with a Twilio Account

### notifications

Manage notifications

- **`twilio-pp-cli notifications fetch`** - Fetch a notification belonging to the account used to make the request

### notifications-json

Manage notifications json

- **`twilio-pp-cli notifications-json list-notification`** - Retrieve a list of notifications belonging to the account used to make the request

### outgoing-caller-ids

Manage outgoing caller ids

- **`twilio-pp-cli outgoing-caller-ids delete`** - Delete the caller-id specified from the account
- **`twilio-pp-cli outgoing-caller-ids fetch`** - Fetch an outgoing-caller-id belonging to the account used to make the request
- **`twilio-pp-cli outgoing-caller-ids update`** - Updates the caller-id

### outgoing-caller-ids-json

Manage outgoing caller ids json

- **`twilio-pp-cli outgoing-caller-ids-json create-validation-request`** - Create validation request
- **`twilio-pp-cli outgoing-caller-ids-json list-outgoing-caller-id`** - Retrieve a list of outgoing-caller-ids belonging to the account used to make the request

### queues

Manage queues

- **`twilio-pp-cli queues delete`** - Remove an empty queue
- **`twilio-pp-cli queues fetch`** - Fetch an instance of a queue identified by the QueueSid
- **`twilio-pp-cli queues update`** - Update the queue with the new parameters

### queues-json

Manage queues json

- **`twilio-pp-cli queues-json create-queue`** - Create a queue
- **`twilio-pp-cli queues-json list-queue`** - Retrieve a list of queues belonging to the account used to make the request

### recordings

Manage recordings

- **`twilio-pp-cli recordings delete`** - Delete a recording from your account
- **`twilio-pp-cli recordings fetch`** - Fetch an instance of a recording

### recordings-json

Manage recordings json

- **`twilio-pp-cli recordings-json list-recording`** - Retrieve a list of recordings belonging to the account used to make the request

### signing-keys

Manage signing keys

- **`twilio-pp-cli signing-keys delete`** - Delete
- **`twilio-pp-cli signing-keys fetch`** - Fetch
- **`twilio-pp-cli signing-keys update`** - Update

### signing-keys-json

Manage signing keys json

- **`twilio-pp-cli signing-keys-json create-new-signing-key`** - Create a new Signing Key for the account making the request.
- **`twilio-pp-cli signing-keys-json list-signing-key`** - List signing key

### sip

Manage sip

- **`twilio-pp-cli sip create-auth-calls-credential-list-mapping`** - Create a new credential list mapping resource
- **`twilio-pp-cli sip create-auth-calls-ip-access-control-list-mapping`** - Create a new IP Access Control List mapping
- **`twilio-pp-cli sip create-auth-registrations-credential-list-mapping`** - Create a new credential list mapping resource
- **`twilio-pp-cli sip create-credential`** - Create a new credential resource.
- **`twilio-pp-cli sip create-credential-list`** - Create a Credential List
- **`twilio-pp-cli sip create-credential-list-mapping`** - Create a CredentialListMapping resource for an account.
- **`twilio-pp-cli sip create-domain`** - Create a new Domain
- **`twilio-pp-cli sip create-ip-access-control-list`** - Create a new IpAccessControlList resource
- **`twilio-pp-cli sip create-ip-access-control-list-mapping`** - Create a new IpAccessControlListMapping resource.
- **`twilio-pp-cli sip create-ip-address`** - Create a new IpAddress resource.
- **`twilio-pp-cli sip delete-auth-calls-credential-list-mapping`** - Delete a credential list mapping from the requested domain
- **`twilio-pp-cli sip delete-auth-calls-ip-access-control-list-mapping`** - Delete an IP Access Control List mapping from the requested domain
- **`twilio-pp-cli sip delete-auth-registrations-credential-list-mapping`** - Delete a credential list mapping from the requested domain
- **`twilio-pp-cli sip delete-credential`** - Delete a credential resource.
- **`twilio-pp-cli sip delete-credential-list`** - Delete a Credential List
- **`twilio-pp-cli sip delete-credential-list-mapping`** - Delete a CredentialListMapping resource from an account.
- **`twilio-pp-cli sip delete-domain`** - Delete an instance of a Domain
- **`twilio-pp-cli sip delete-ip-access-control-list`** - Delete an IpAccessControlList from the requested account
- **`twilio-pp-cli sip delete-ip-access-control-list-mapping`** - Delete an IpAccessControlListMapping resource.
- **`twilio-pp-cli sip delete-ip-address`** - Delete an IpAddress resource.
- **`twilio-pp-cli sip fetch-auth-calls-credential-list-mapping`** - Fetch a specific instance of a credential list mapping
- **`twilio-pp-cli sip fetch-auth-calls-ip-access-control-list-mapping`** - Fetch a specific instance of an IP Access Control List mapping
- **`twilio-pp-cli sip fetch-auth-registrations-credential-list-mapping`** - Fetch a specific instance of a credential list mapping
- **`twilio-pp-cli sip fetch-credential`** - Fetch a single credential.
- **`twilio-pp-cli sip fetch-credential-list`** - Get a Credential List
- **`twilio-pp-cli sip fetch-credential-list-mapping`** - Fetch a single CredentialListMapping resource from an account.
- **`twilio-pp-cli sip fetch-domain`** - Fetch an instance of a Domain
- **`twilio-pp-cli sip fetch-ip-access-control-list`** - Fetch a specific instance of an IpAccessControlList
- **`twilio-pp-cli sip fetch-ip-access-control-list-mapping`** - Fetch an IpAccessControlListMapping resource.
- **`twilio-pp-cli sip fetch-ip-address`** - Read one IpAddress resource.
- **`twilio-pp-cli sip list-auth-calls-credential-list-mapping`** - Retrieve a list of credential list mappings belonging to the domain used in the request
- **`twilio-pp-cli sip list-auth-calls-ip-access-control-list-mapping`** - Retrieve a list of IP Access Control List mappings belonging to the domain used in the request
- **`twilio-pp-cli sip list-auth-registrations-credential-list-mapping`** - Retrieve a list of credential list mappings belonging to the domain used in the request
- **`twilio-pp-cli sip list-credential`** - Retrieve a list of credentials.
- **`twilio-pp-cli sip list-credential-list`** - Get All Credential Lists
- **`twilio-pp-cli sip list-credential-list-mapping`** - Read multiple CredentialListMapping resources from an account.
- **`twilio-pp-cli sip list-domain`** - Retrieve a list of domains belonging to the account used to make the request
- **`twilio-pp-cli sip list-ip-access-control-list`** - Retrieve a list of IpAccessControlLists that belong to the account used to make the request
- **`twilio-pp-cli sip list-ip-access-control-list-mapping`** - Retrieve a list of IpAccessControlListMapping resources.
- **`twilio-pp-cli sip list-ip-address`** - Read multiple IpAddress resources.
- **`twilio-pp-cli sip update-credential`** - Update a credential resource.
- **`twilio-pp-cli sip update-credential-list`** - Update a Credential List
- **`twilio-pp-cli sip update-domain`** - Update the attributes of a domain
- **`twilio-pp-cli sip update-ip-access-control-list`** - Rename an IpAccessControlList
- **`twilio-pp-cli sip update-ip-address`** - Update an IpAddress resource.

### sms

Manage sms

- **`twilio-pp-cli sms fetch-short-code`** - Fetch an instance of a short code
- **`twilio-pp-cli sms list-short-code`** - Retrieve a list of short-codes belonging to the account used to make the request
- **`twilio-pp-cli sms update-short-code`** - Update a short code with the following parameters

### tokens-json

Manage tokens json

- **`twilio-pp-cli tokens-json create-token`** - Create a new token for ICE servers

### transcriptions

Manage transcriptions

- **`twilio-pp-cli transcriptions delete`** - Delete a transcription from the account used to make the request
- **`twilio-pp-cli transcriptions fetch`** - Fetch an instance of a Transcription

### transcriptions-json

Manage transcriptions json

- **`twilio-pp-cli transcriptions-json list-transcription`** - Retrieve a list of transcriptions belonging to the account used to make the request

### usage

Manage usage

- **`twilio-pp-cli usage create-trigger`** - Create a new UsageTrigger
- **`twilio-pp-cli usage delete-trigger`** - Delete trigger
- **`twilio-pp-cli usage fetch-trigger`** - Fetch and instance of a usage-trigger
- **`twilio-pp-cli usage list-record`** - Retrieve a list of usage-records belonging to the account used to make the request
- **`twilio-pp-cli usage list-record-all-time`** - List record all time
- **`twilio-pp-cli usage list-record-daily`** - List record daily
- **`twilio-pp-cli usage list-record-last-month`** - List record last month
- **`twilio-pp-cli usage list-record-monthly`** - List record monthly
- **`twilio-pp-cli usage list-record-this-month`** - List record this month
- **`twilio-pp-cli usage list-record-today`** - List record today
- **`twilio-pp-cli usage list-record-yearly`** - List record yearly
- **`twilio-pp-cli usage list-record-yesterday`** - List record yesterday
- **`twilio-pp-cli usage list-trigger`** - Retrieve a list of usage-triggers belonging to the account used to make the request
- **`twilio-pp-cli usage update-trigger`** - Update an instance of a usage trigger


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
twilio-pp-cli 2010-04-01 create-account

# JSON for scripting and agents
twilio-pp-cli 2010-04-01 create-account --json

# Filter to specific fields
twilio-pp-cli 2010-04-01 create-account --json --select id,name,status

# Dry run — show the request without sending
twilio-pp-cli 2010-04-01 create-account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
twilio-pp-cli 2010-04-01 create-account --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-twilio -g
```

Then invoke `/pp-twilio <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/cmd/twilio-pp-mcp@latest
```

Then register it:

```bash
claude mcp add twilio twilio-pp-mcp -e TWILIO_ACCOUNT_SID=<account-sid> -e TWILIO_AUTH_TOKEN=<auth-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/twilio-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TWILIO_ACCOUNT_SID` and `TWILIO_AUTH_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/cmd/twilio-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "twilio": {
      "command": "twilio-pp-mcp",
      "env": {
        "TWILIO_ACCOUNT_SID": "<account-sid>",
        "TWILIO_AUTH_TOKEN": "<auth-token>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
twilio-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/twilio-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TWILIO_ACCOUNT_SID` | per_call | Yes | Twilio Account SID (starts with AC). Required because the URL path includes it. |
| `TWILIO_AUTH_TOKEN` | per_call | No | Master Auth Token. Set this OR both TWILIO_API_KEY_SID and TWILIO_API_KEY_SECRET. |
| `TWILIO_API_KEY_SID` | per_call | No | Scoped API Key SID (starts with SK). Pair with TWILIO_API_KEY_SECRET. |
| `TWILIO_API_KEY_SECRET` | per_call | No | Scoped API Key Secret. Pair with TWILIO_API_KEY_SID. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `twilio-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TWILIO_ACCOUNT_SID && echo $TWILIO_AUTH_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Authenticate (code 20003)** — Run `twilio-pp-cli doctor`. Common causes: scoped API Key bound to a different account than the one in TWILIO_ACCOUNT_SID, or a rotated Auth Token in the secondary slot.
- **Some commands return empty for a fresh install** — Run `twilio-pp-cli sync --full` once before the analytic commands. Then `twilio-pp-cli sync-status --json` confirms freshness per resource.
- **429 Too Many Requests during sync** — Twilio's default rate limit is ~100 req/s. Reduce parallelism with `--concurrency 4`, or open a support ticket to raise the cap.
- **Subaccount commands fail with 401** — Scoped API Keys can only operate on the account that minted them. Create a master Auth Token credential or call against the subaccount's own SK key. `twilio-pp-cli doctor` flags this.
- **Searching Body returns nothing for known content** — FTS reads the local store. Re-run `twilio-pp-cli sync --resources messages` to refresh. The `\\b` boundary regex in queries needs a doubled backslash on shells that interpret the single one.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**twilio-python**](https://github.com/twilio/twilio-python) — Python (2057 stars)
- [**twilio-node**](https://github.com/twilio/twilio-node) — TypeScript (1535 stars)
- [**twilio-go**](https://github.com/twilio/twilio-go) — Go (370 stars)
- [**twilio-cli**](https://github.com/twilio/twilio-cli) — JavaScript (189 stars)
- [**twilio-labs/mcp**](https://github.com/twilio-labs/mcp) — TypeScript (104 stars)
- [**steampipe-plugin-twilio**](https://github.com/turbot/steampipe-plugin-twilio) — Go (4 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
