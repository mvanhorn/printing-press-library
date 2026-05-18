---
name: pp-twilio
description: "Every Twilio Core feature, plus offline message and call history, FTS, and SQL-grade analytics no other Twilio tool... Trigger phrases: `send an SMS via Twilio`, `list Twilio messages`, `Twilio call recordings`, `Twilio usage cost`, `Twilio opt-out audit`, `use twilio`, `run twilio`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - twilio-pp-cli
    install:
      - kind: go
        bins: [twilio-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/cmd/twilio-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/social-and-messaging/twilio/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Twilio — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `twilio-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install twilio --cli-only
   ```
2. Verify: `twilio-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/cmd/twilio-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Twilio's official CLI is a thin Node wrapper with 1–3 second cold start and no local cache. This CLI syncs your Messages, Calls, Recordings, UsageRecords, and IncomingPhoneNumbers into a local SQLite store you can grep, SQL, and aggregate offline. It ships every v2010 endpoint as a typed Cobra command, plus 12 novel features the Twilio ecosystem does not have: delivery-failure breakdowns, subaccount spend matrices, call-trace stitches, TCPA opt-out checks, idle-number reclamation, webhook-orphan audits, and more.

## When to Use This CLI

Reach for this CLI when an agent or operator needs Twilio data they cannot get in one API call: cross-subaccount aggregation, delivery-failure analysis, TCPA opt-out audits, idle-number cleanup, or any 'how was last week' question. It is also the right tool for incident triage when a CallSid needs the full Call+Recording+Transcription stitch in one command. Skip it for one-off message sends if you already have twilio-cli configured — but for any analytic or investigative workflow it is faster, offline-capable, and agent-native.

## Unique Capabilities

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

## Command Reference

**2010-04-01** — Manage 2010 04 01

- `twilio-pp-cli 2010-04-01 create-account` — Create a new Twilio Subaccount from the account making the request
- `twilio-pp-cli 2010-04-01 fetch-account` — Fetch the account specified by the provided Account Sid
- `twilio-pp-cli 2010-04-01 list-account` — Retrieves a collection of Accounts belonging to the account used to make the request
- `twilio-pp-cli 2010-04-01 update-account` — Modify the properties of a given Account

**addresses** — Manage addresses

- `twilio-pp-cli addresses delete-address` — Delete address
- `twilio-pp-cli addresses fetch-address` — Fetch address
- `twilio-pp-cli addresses update-address` — Update address

**addresses-json** — Manage addresses json

- `twilio-pp-cli addresses-json create-address` — Create address
- `twilio-pp-cli addresses-json list-address` — List address

**applications** — Manage applications

- `twilio-pp-cli applications delete` — Delete the application by the specified application sid
- `twilio-pp-cli applications fetch` — Fetch the application specified by the provided sid
- `twilio-pp-cli applications update` — Updates the application's properties

**applications-json** — Manage applications json

- `twilio-pp-cli applications-json create-application` — Create a new application within your account
- `twilio-pp-cli applications-json list-application` — Retrieve a list of applications representing an application within the requesting account

**authorized-connect-apps** — Manage authorized connect apps

- `twilio-pp-cli authorized-connect-apps <AccountSid> <ConnectAppSid>` — Fetch an instance of an authorized-connect-app

**authorized-connect-apps-json** — Manage authorized connect apps json

- `twilio-pp-cli authorized-connect-apps-json <AccountSid>` — Retrieve a list of authorized-connect-apps belonging to the account used to make the request

**available-phone-numbers** — Manage available phone numbers

- `twilio-pp-cli available-phone-numbers <AccountSid> <CountryCode>` — Fetch country

**available-phone-numbers-json** — Manage available phone numbers json

- `twilio-pp-cli available-phone-numbers-json <AccountSid>` — List available phone number country

**balance-json** — Manage balance json

- `twilio-pp-cli balance-json <AccountSid>` — Fetch the balance for an Account based on Account Sid. Balance changes may not be reflected immediately. Child...

**calls** — Manage calls

- `twilio-pp-cli calls delete` — Delete a Call record from your account. Once the record is deleted, it will no longer appear in the API and Account...
- `twilio-pp-cli calls fetch` — Fetch the call specified by the provided Call SID
- `twilio-pp-cli calls update` — Initiates a call redirect or terminates a call

**calls-json** — Manage calls json

- `twilio-pp-cli calls-json create-call` — Create a new outgoing call to phones, SIP-enabled endpoints or Twilio Client connections
- `twilio-pp-cli calls-json list-call` — Retrieves a collection of calls made to and from your account

**conferences** — Manage conferences

- `twilio-pp-cli conferences fetch` — Fetch an instance of a conference
- `twilio-pp-cli conferences update` — Update

**conferences-json** — Manage conferences json

- `twilio-pp-cli conferences-json <AccountSid>` — Retrieve a list of conferences belonging to the account used to make the request

**connect-apps** — Manage connect apps

- `twilio-pp-cli connect-apps delete` — Delete an instance of a connect-app
- `twilio-pp-cli connect-apps fetch` — Fetch an instance of a connect-app
- `twilio-pp-cli connect-apps update` — Update a connect-app with the specified parameters

**connect-apps-json** — Manage connect apps json

- `twilio-pp-cli connect-apps-json <AccountSid>` — Retrieve a list of connect-apps belonging to the account used to make the request

**incoming-phone-numbers** — Manage incoming phone numbers

- `twilio-pp-cli incoming-phone-numbers create-local` — Create local
- `twilio-pp-cli incoming-phone-numbers create-mobile` — Create mobile
- `twilio-pp-cli incoming-phone-numbers create-toll-free` — Create toll free
- `twilio-pp-cli incoming-phone-numbers delete` — Delete a phone-numbers belonging to the account used to make the request.
- `twilio-pp-cli incoming-phone-numbers fetch` — Fetch an incoming-phone-number belonging to the account used to make the request.
- `twilio-pp-cli incoming-phone-numbers list-local` — List local
- `twilio-pp-cli incoming-phone-numbers list-mobile` — List mobile
- `twilio-pp-cli incoming-phone-numbers list-toll-free` — List toll free
- `twilio-pp-cli incoming-phone-numbers update` — Update an incoming-phone-number instance.

**incoming-phone-numbers-json** — Manage incoming phone numbers json

- `twilio-pp-cli incoming-phone-numbers-json create-incoming-phone-number` — Purchase a phone-number for the account.
- `twilio-pp-cli incoming-phone-numbers-json list-incoming-phone-number` — Retrieve a list of incoming-phone-numbers belonging to the account used to make the request.

**keys** — Manage keys

- `twilio-pp-cli keys delete` — Delete
- `twilio-pp-cli keys fetch` — Fetch
- `twilio-pp-cli keys update` — Update

**keys-json** — Manage keys json

- `twilio-pp-cli keys-json create-new-key` — Create new key
- `twilio-pp-cli keys-json list-key` — List key

**messages** — Manage messages

- `twilio-pp-cli messages delete` — Deletes a Message resource from your account
- `twilio-pp-cli messages fetch` — Fetch a specific Message
- `twilio-pp-cli messages update` — Update a Message resource (used to redact Message `body` text and to cancel not-yet-sent messages)

**messages-json** — Manage messages json

- `twilio-pp-cli messages-json create-message` — Send a message
- `twilio-pp-cli messages-json list-message` — Retrieve a list of Message resources associated with a Twilio Account

**notifications** — Manage notifications

- `twilio-pp-cli notifications <AccountSid> <Sid>` — Fetch a notification belonging to the account used to make the request

**notifications-json** — Manage notifications json

- `twilio-pp-cli notifications-json <AccountSid>` — Retrieve a list of notifications belonging to the account used to make the request

**outgoing-caller-ids** — Manage outgoing caller ids

- `twilio-pp-cli outgoing-caller-ids delete` — Delete the caller-id specified from the account
- `twilio-pp-cli outgoing-caller-ids fetch` — Fetch an outgoing-caller-id belonging to the account used to make the request
- `twilio-pp-cli outgoing-caller-ids update` — Updates the caller-id

**outgoing-caller-ids-json** — Manage outgoing caller ids json

- `twilio-pp-cli outgoing-caller-ids-json create-validation-request` — Create validation request
- `twilio-pp-cli outgoing-caller-ids-json list-outgoing-caller-id` — Retrieve a list of outgoing-caller-ids belonging to the account used to make the request

**queues** — Manage queues

- `twilio-pp-cli queues delete` — Remove an empty queue
- `twilio-pp-cli queues fetch` — Fetch an instance of a queue identified by the QueueSid
- `twilio-pp-cli queues update` — Update the queue with the new parameters

**queues-json** — Manage queues json

- `twilio-pp-cli queues-json create-queue` — Create a queue
- `twilio-pp-cli queues-json list-queue` — Retrieve a list of queues belonging to the account used to make the request

**recordings** — Manage recordings

- `twilio-pp-cli recordings delete` — Delete a recording from your account
- `twilio-pp-cli recordings fetch` — Fetch an instance of a recording

**recordings-json** — Manage recordings json

- `twilio-pp-cli recordings-json <AccountSid>` — Retrieve a list of recordings belonging to the account used to make the request

**signing-keys** — Manage signing keys

- `twilio-pp-cli signing-keys delete` — Delete
- `twilio-pp-cli signing-keys fetch` — Fetch
- `twilio-pp-cli signing-keys update` — Update

**signing-keys-json** — Manage signing keys json

- `twilio-pp-cli signing-keys-json create-new-signing-key` — Create a new Signing Key for the account making the request.
- `twilio-pp-cli signing-keys-json list-signing-key` — List signing key

**sip** — Manage sip

- `twilio-pp-cli sip create-auth-calls-credential-list-mapping` — Create a new credential list mapping resource
- `twilio-pp-cli sip create-auth-calls-ip-access-control-list-mapping` — Create a new IP Access Control List mapping
- `twilio-pp-cli sip create-auth-registrations-credential-list-mapping` — Create a new credential list mapping resource
- `twilio-pp-cli sip create-credential` — Create a new credential resource.
- `twilio-pp-cli sip create-credential-list` — Create a Credential List
- `twilio-pp-cli sip create-credential-list-mapping` — Create a CredentialListMapping resource for an account.
- `twilio-pp-cli sip create-domain` — Create a new Domain
- `twilio-pp-cli sip create-ip-access-control-list` — Create a new IpAccessControlList resource
- `twilio-pp-cli sip create-ip-access-control-list-mapping` — Create a new IpAccessControlListMapping resource.
- `twilio-pp-cli sip create-ip-address` — Create a new IpAddress resource.
- `twilio-pp-cli sip delete-auth-calls-credential-list-mapping` — Delete a credential list mapping from the requested domain
- `twilio-pp-cli sip delete-auth-calls-ip-access-control-list-mapping` — Delete an IP Access Control List mapping from the requested domain
- `twilio-pp-cli sip delete-auth-registrations-credential-list-mapping` — Delete a credential list mapping from the requested domain
- `twilio-pp-cli sip delete-credential` — Delete a credential resource.
- `twilio-pp-cli sip delete-credential-list` — Delete a Credential List
- `twilio-pp-cli sip delete-credential-list-mapping` — Delete a CredentialListMapping resource from an account.
- `twilio-pp-cli sip delete-domain` — Delete an instance of a Domain
- `twilio-pp-cli sip delete-ip-access-control-list` — Delete an IpAccessControlList from the requested account
- `twilio-pp-cli sip delete-ip-access-control-list-mapping` — Delete an IpAccessControlListMapping resource.
- `twilio-pp-cli sip delete-ip-address` — Delete an IpAddress resource.
- `twilio-pp-cli sip fetch-auth-calls-credential-list-mapping` — Fetch a specific instance of a credential list mapping
- `twilio-pp-cli sip fetch-auth-calls-ip-access-control-list-mapping` — Fetch a specific instance of an IP Access Control List mapping
- `twilio-pp-cli sip fetch-auth-registrations-credential-list-mapping` — Fetch a specific instance of a credential list mapping
- `twilio-pp-cli sip fetch-credential` — Fetch a single credential.
- `twilio-pp-cli sip fetch-credential-list` — Get a Credential List
- `twilio-pp-cli sip fetch-credential-list-mapping` — Fetch a single CredentialListMapping resource from an account.
- `twilio-pp-cli sip fetch-domain` — Fetch an instance of a Domain
- `twilio-pp-cli sip fetch-ip-access-control-list` — Fetch a specific instance of an IpAccessControlList
- `twilio-pp-cli sip fetch-ip-access-control-list-mapping` — Fetch an IpAccessControlListMapping resource.
- `twilio-pp-cli sip fetch-ip-address` — Read one IpAddress resource.
- `twilio-pp-cli sip list-auth-calls-credential-list-mapping` — Retrieve a list of credential list mappings belonging to the domain used in the request
- `twilio-pp-cli sip list-auth-calls-ip-access-control-list-mapping` — Retrieve a list of IP Access Control List mappings belonging to the domain used in the request
- `twilio-pp-cli sip list-auth-registrations-credential-list-mapping` — Retrieve a list of credential list mappings belonging to the domain used in the request
- `twilio-pp-cli sip list-credential` — Retrieve a list of credentials.
- `twilio-pp-cli sip list-credential-list` — Get All Credential Lists
- `twilio-pp-cli sip list-credential-list-mapping` — Read multiple CredentialListMapping resources from an account.
- `twilio-pp-cli sip list-domain` — Retrieve a list of domains belonging to the account used to make the request
- `twilio-pp-cli sip list-ip-access-control-list` — Retrieve a list of IpAccessControlLists that belong to the account used to make the request
- `twilio-pp-cli sip list-ip-access-control-list-mapping` — Retrieve a list of IpAccessControlListMapping resources.
- `twilio-pp-cli sip list-ip-address` — Read multiple IpAddress resources.
- `twilio-pp-cli sip update-credential` — Update a credential resource.
- `twilio-pp-cli sip update-credential-list` — Update a Credential List
- `twilio-pp-cli sip update-domain` — Update the attributes of a domain
- `twilio-pp-cli sip update-ip-access-control-list` — Rename an IpAccessControlList
- `twilio-pp-cli sip update-ip-address` — Update an IpAddress resource.

**sms** — Manage sms

- `twilio-pp-cli sms fetch-short-code` — Fetch an instance of a short code
- `twilio-pp-cli sms list-short-code` — Retrieve a list of short-codes belonging to the account used to make the request
- `twilio-pp-cli sms update-short-code` — Update a short code with the following parameters

**tokens-json** — Manage tokens json

- `twilio-pp-cli tokens-json <AccountSid>` — Create a new token for ICE servers

**transcriptions** — Manage transcriptions

- `twilio-pp-cli transcriptions delete` — Delete a transcription from the account used to make the request
- `twilio-pp-cli transcriptions fetch` — Fetch an instance of a Transcription

**transcriptions-json** — Manage transcriptions json

- `twilio-pp-cli transcriptions-json <AccountSid>` — Retrieve a list of transcriptions belonging to the account used to make the request

**usage** — Manage usage

- `twilio-pp-cli usage create-trigger` — Create a new UsageTrigger
- `twilio-pp-cli usage delete-trigger` — Delete trigger
- `twilio-pp-cli usage fetch-trigger` — Fetch and instance of a usage-trigger
- `twilio-pp-cli usage list-record` — Retrieve a list of usage-records belonging to the account used to make the request
- `twilio-pp-cli usage list-record-all-time` — List record all time
- `twilio-pp-cli usage list-record-daily` — List record daily
- `twilio-pp-cli usage list-record-last-month` — List record last month
- `twilio-pp-cli usage list-record-monthly` — List record monthly
- `twilio-pp-cli usage list-record-this-month` — List record this month
- `twilio-pp-cli usage list-record-today` — List record today
- `twilio-pp-cli usage list-record-yearly` — List record yearly
- `twilio-pp-cli usage list-record-yesterday` — List record yesterday
- `twilio-pp-cli usage list-trigger` — Retrieve a list of usage-triggers belonging to the account used to make the request
- `twilio-pp-cli usage update-trigger` — Update an instance of a usage trigger


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
twilio-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Weekly delivery-failure report

```bash
twilio-pp-cli delivery-failures --since 7d --json --select error_code,to_country,count,cost_total
```

Runs the failure aggregation against the local store, narrows the response to the four fields that matter so an agent can summarise without parsing 4000 raw rows.

### Last-month agency billing CSV

```bash
twilio-pp-cli subaccount-spend --period last-month --csv > march-billing.csv
```

Walks every subaccount, calls usage/records per subaccount, pivots categories to columns, writes one wide CSV the finance team can paste straight into a spreadsheet.

### On-call call-trace from a Slack page

```bash
twilio-pp-cli call-trace CAxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx --json --select call.status,call.duration,recordings,transcriptions.text
```

Single command for the full Call → Recording → Transcription chain. Use --select with dotted paths to keep the response small enough that an agent can summarise it in a Slack reply.

### Quarterly TCPA opt-out audit

```bash
twilio-pp-cli opt-out-violations --since 90d --csv > q1-opt-out-audit.csv
```

Local temporal join finds inbound STOP/UNSUBSCRIBE messages and any subsequent outbound messages to the same number — the report compliance teams pay legal teams to construct manually today.

### Find idle numbers wasting money

```bash
twilio-pp-cli idle-numbers --since 30d --json --select phone_number,friendly_name,monthly_cost,last_activity
```

LEFT JOIN over IncomingPhoneNumbers / Messages / Calls flagging numbers with no activity in 30 days. Output is one row per number with the dollar amount being wasted per month.

## Auth Setup

Twilio uses HTTP Basic auth. You can authenticate with your Account SID (AC...) plus your Auth Token, OR with a scoped API Key SID (SK...) plus its Secret. The Account SID is always required because it is part of the URL path. Run `twilio-pp-cli doctor` to verify your credentials, detect whether the key is master or scoped, and warn on parent/subaccount mismatches.

Run `twilio-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  twilio-pp-cli 2010-04-01 create-account --agent --select id,name,status
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
twilio-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
twilio-pp-cli feedback --stdin < notes.txt
twilio-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.twilio-pp-cli/feedback.jsonl`. They are never POSTed unless `TWILIO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TWILIO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
twilio-pp-cli profile save briefing --json
twilio-pp-cli --profile briefing 2010-04-01 create-account
twilio-pp-cli profile list --json
twilio-pp-cli profile show briefing
twilio-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `twilio-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/cmd/twilio-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add twilio-pp-mcp -- twilio-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which twilio-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   twilio-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `twilio-pp-cli <command> --help`.
