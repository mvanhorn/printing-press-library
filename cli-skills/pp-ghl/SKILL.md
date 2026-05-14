---
name: pp-ghl
description: "Every GoHighLevel surface that matters for a marketing/CRM operator — with offline SQL, kill-switch tag awareness,... Trigger phrases: `look up a GHL contact`, `find this lead in highlevel`, `is this contact opted out`, `what's our GHL pipeline this week`, `show me unread GHL conversations`, `use ghl`, `run ghl-pp-cli`."
author: "Alex Puckhaber"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ghl-pp-cli
---

# GoHighLevel — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ghl-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install ghl --cli-only
   ```
2. Verify: `ghl-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Wraps the GoHighLevel v2 API (contacts, conversations, calendars, opportunities, workflows, tags, custom fields) with a Go binary that mirrors every endpoint, caches every entity in local SQLite, and adds the surfaces the official MCP lacks: workflow membership, tag analytics, cross-entity activity windows, send preflight, and kill-switch roster. Default output is token-efficient one-liners; --full and --select expose the rest.

## When to Use This CLI

Reach for `pp-ghl` whenever you need GHL data in an agent loop without burning tokens on verbose JSON, when you need to compose a kill-switch-aware nurture step, or when you want answers that span contacts + conversations + opportunities + appointments in one query. Skip it if you only need to open the GHL UI for a one-off click.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Kill-switch awareness
- **`killswitch list`** — List every contact in the location tagged with `ai off` or `human handover` so you can audit who Riley should not be texting.

  _Reach for this before bulk-sending or auditing AI nurture — the two tags it surfaces gate every Riley message; missing one is a real-world safety violation._

  ```bash
  ghl-pp-cli killswitch list --tag ai-off --json
  ```
- **`killswitch check`** — Return a typed exit code for a single contact: 0 clear, 2 ai-off, 3 human-handover. Designed to gate a send from a workflow.

  _Reach for this in any send-preflight script or workflow gate — typed exit codes let bash and agent loops skip with no parsing._

  ```bash
  ghl-pp-cli killswitch check +1-555-0100 || echo 'do not send'
  ```
- **`sms preflight`** — Validate a planned SMS: contact exists, has E.164 phone, no `ai off` tag, optionally within business hours — returns typed exit code without sending.

  _Reach for this in every agent send-loop — it returns immediately with a typed reason and never sends, so it's safe to call before deciding._

  ```bash
  ghl-pp-cli sms preflight +1-555-0100 --body 'see you tomorrow'
  ```

### Cross-entity local queries
- **`activity`** — Recent activity across contacts, messages, opportunities, and appointments in one timeline. `--since 24h` or `--since 7d`.

  _Reach for this when answering 'what happened in the last day' — the official MCP has no equivalent and the UI requires four tabs._

  ```bash
  ghl-pp-cli activity --since 24h --json
  ```
- **`tags stats`** — List every tag in the location with contact-count, last-used date, and a kill-switch flag.

  _Reach for this before tag cleanup, segmentation reviews, or auditing how many contacts a tag actually covers._

  ```bash
  ghl-pp-cli tags stats --json
  ```
- **`kpi today`** — One-line cross-entity aggregate: new contacts, SMS sent, appointments booked, opportunities moved, kill-switch trips — JSON-friendly for dashboard cron.

  _Reach for this from a business-dashboard cron or a daily standup script — one row, no parsing._

  ```bash
  ghl-pp-cli kpi today --json
  ```
- **`contacts recency`** — For each contact (optionally filtered by tag), show last inbound and last outbound message timestamps; sort by oldest-last-touch.

  _Reach for this for coach roster reviews and 'who haven't I talked to in two weeks' triage._

  ```bash
  ghl-pp-cli contacts recency --tag client --over 14d --json
  ```
- **`inbox triage`** — Conversations with unread inbound messages and no outbound reply in the window, kill-switch-aware (skips `ai off`).

  _Reach for this for daily standup or any 'who do I need to reply to' question._

  ```bash
  ghl-pp-cli inbox triage --since 4h --json
  ```
- **`opportunities stale`** — List opportunities whose stage has not moved in N days, grouped by pipeline and stage.

  _Reach for this in end-of-week pipeline reviews when you need to know which deals stopped moving._

  ```bash
  ghl-pp-cli opportunities stale --days 14 --json
  ```
- **`opportunities funnel`** — For one pipeline, count and total monetary value per stage in stage order.

  _Reach for this for weekly pipeline reviews and quarterly forecasting baselines._

  ```bash
  ghl-pp-cli opportunities funnel pipeline_abc --json
  ```

### Workflow coverage
- **`workflows members`** — List the contacts currently enrolled in a workflow with a terse default.

  _Reach for this when auditing nurture enrollment or debugging why Riley isn't seeing a contact._

  ```bash
  ghl-pp-cli workflows members wf_abc123 --json
  ```

## Command Reference

**calendars** — Manage calendars

- `ghl-pp-cli calendars add-to-schedule` — Associates a calendar with the given schedule by adding the calendarId to a schedule
- `ghl-pp-cli calendars create` — Create calendar in a location.
- `ghl-pp-cli calendars create-appointment` — Create appointment
- `ghl-pp-cli calendars create-appointment-note` — Create Note
- `ghl-pp-cli calendars create-block-slot` — Create block slot
- `ghl-pp-cli calendars create-group` — Create Calendar Group
- `ghl-pp-cli calendars create-resource` — Create calendar resource by resource type
- `ghl-pp-cli calendars create-schedule` — Create new schedule with specified rules, timezone, location, user and calendar associations.
- `ghl-pp-cli calendars delete` — Delete calendar by ID
- `ghl-pp-cli calendars delete-appointment-note` — Delete Note
- `ghl-pp-cli calendars delete-event` — Delete event by ID
- `ghl-pp-cli calendars delete-group` — Delete Group
- `ghl-pp-cli calendars delete-resource` — Delete calendar resource by ID
- `ghl-pp-cli calendars delete-schedule` — Permanently remove a schedule and all its associated rules. This action cannot be undone.
- `ghl-pp-cli calendars disable-group` — Disable Group
- `ghl-pp-cli calendars edit-appointment` — Update appointment
- `ghl-pp-cli calendars edit-block-slot` — Update block slot by ID
- `ghl-pp-cli calendars edit-group` — Update Group by group ID
- `ghl-pp-cli calendars fetch-resources` — List calendar resources by resource type and location ID
- `ghl-pp-cli calendars get` — Get all calendars in a location.
- `ghl-pp-cli calendars get-all-schedules` — Retrieve user availability schedules based on various filters including location, calendar, and user. Supports...
- `ghl-pp-cli calendars get-appointment` — Get appointment by ID
- `ghl-pp-cli calendars get-appointment-notes` — Get Appointment Notes
- `ghl-pp-cli calendars get-blocked-slots` — Get Blocked Slots
- `ghl-pp-cli calendars get-calendarid` — Get calendar by ID
- `ghl-pp-cli calendars get-events` — Get Calendar Events
- `ghl-pp-cli calendars get-groups` — Get all calendar groups in a location.
- `ghl-pp-cli calendars get-resource` — Get calendar resource by ID
- `ghl-pp-cli calendars get-schedule-by-id` — Retrieve a specific schedule by its unique identifier. Returns detailed information including rules, timezone, and...
- `ghl-pp-cli calendars remove-from-schedule` — Removes the association between a team calendar and the given schedule by removing the calendarId from the schedule
- `ghl-pp-cli calendars update` — Update calendar by ID.
- `ghl-pp-cli calendars update-appointment-note` — Update Note
- `ghl-pp-cli calendars update-resource` — Update calendar resource by ID
- `ghl-pp-cli calendars update-schedule` — Modify an existing schedule by updating its rules, timezone, and name All fields are optional - only provided fields...
- `ghl-pp-cli calendars validate-groups-slug` — Validate if group slug is available or not.

**contacts** — Documentation for Contacts API

- `ghl-pp-cli contacts add-remove-from-business` — Add/Remove Contacts From Business . Passing a `null` businessId will remove the businessId from the contacts
- `ghl-pp-cli contacts create` — Please find the list of acceptable values for the `country` field <a...
- `ghl-pp-cli contacts create-association` — Allows you to update tags to multiple contacts at once, you can add or remove tags from the contacts
- `ghl-pp-cli contacts delete` — Delete Contact
- `ghl-pp-cli contacts get` — Get Contacts **Note:** This API endpoint is deprecated. Please use the [Search...
- `ghl-pp-cli contacts get-by-business-id` — Get Contacts By BusinessId
- `ghl-pp-cli contacts get-contactid` — Get Contact
- `ghl-pp-cli contacts get-duplicate` — Get Duplicate Contact.<br/><br/>If `Allow Duplicate Contact` is disabled under Settings, the global unique...
- `ghl-pp-cli contacts search-advanced` — Search contacts based on combinations of advanced filters. Documentation Link -...
- `ghl-pp-cli contacts update` — Please find the list of acceptable values for the `country` field <a...
- `ghl-pp-cli contacts upsert` — Please find the list of acceptable values for the `country` field <a...

**conversations** — Manage conversations

- `ghl-pp-cli conversations add-an-inbound-message` — Post the necessary fields for the API to add a new inbound message. <br />
- `ghl-pp-cli conversations add-an-outbound-message` — Post the necessary fields for the API to add a new outbound call.
- `ghl-pp-cli conversations add-message-attachments` — Set attachments on an existing message (replaces existing). Maximum 5 URLs. Supported for TYPE_CUSTOM_CALL (34) and...
- `ghl-pp-cli conversations cancel-scheduled-email-message` — Post the messageId for the API to delete a scheduled email message. <br />
- `ghl-pp-cli conversations cancel-scheduled-message` — Post the messageId for the API to delete a scheduled message. <br />
- `ghl-pp-cli conversations complete-file-upload` — Validates the uploaded file in GCS and returns the public URL. Call this endpoint after successfully uploading the...
- `ghl-pp-cli conversations create` — Creates a new conversation with the data provided
- `ghl-pp-cli conversations create-custom-subtype` — Create a new custom subtype for a location. Requires agency or account admin role.
- `ghl-pp-cli conversations delete` — Delete the conversation details based on the conversation ID
- `ghl-pp-cli conversations download-message-transcription` — Download the recording transcription for a message by passing the message id
- `ghl-pp-cli conversations export-messages-by-location` — Export messages for a specific location with cursor-based pagination support. Response includes messageType...
- `ghl-pp-cli conversations get` — Get the conversation details based on the conversation ID
- `ghl-pp-cli conversations get-all-custom-subtypes` — Get all custom subtypes for a location
- `ghl-pp-cli conversations get-contact-unsubscription-status` — Get all subscription statuses for a contact (all emails or specific email)
- `ghl-pp-cli conversations get-email-by-id` — Get email by Id
- `ghl-pp-cli conversations get-message` — Get message by message id.
- `ghl-pp-cli conversations get-message-recording` — Get the recording for a message by passing the message id
- `ghl-pp-cli conversations get-message-transcription` — Get the recording transcription for a message by passing the message id
- `ghl-pp-cli conversations initiate-file-upload` — Generates a signed URL for direct file upload to Google Cloud Storage. Returns a signed URL valid for 15 minutes....
- `ghl-pp-cli conversations live-chat-agent-typing` — Agent/AI-Bot will call this when they are typing a message in live chat message
- `ghl-pp-cli conversations search` — Returns a list of all conversations matching the search criteria along with the sort and filter options selected.
- `ghl-pp-cli conversations send-a-new-message` — Post the necessary fields for the API to send a new message.
- `ghl-pp-cli conversations send-review-reply` — Post a reply to a customer review on Google My Business
- `ghl-pp-cli conversations update` — Update the conversation details based on the conversation ID
- `ghl-pp-cli conversations update-custom-subtype` — Update or archive a custom subtype. Requires agency or account admin role.
- `ghl-pp-cli conversations update-message-status` — Post the necessary fields for the API to update message status.
- `ghl-pp-cli conversations upload-file-attachments` — Post the necessary fields for the API to upload files. The files need to be a buffer with the key 'fileAttachment'....
- `ghl-pp-cli conversations user-subscription-change` — Process subscription change initiated by a user (admin/agent). Supports individual custom subscription changes and...

**custom-fields** — Manage custom fields

- `ghl-pp-cli custom-fields create` — <div> <p> Create Custom Field </p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...
- `ghl-pp-cli custom-fields create-folder` — <div> <p> Create Custom Field Folder </p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...
- `ghl-pp-cli custom-fields delete` — <div> <p> Delete Custom Field By Id </p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...
- `ghl-pp-cli custom-fields delete-folder` — <div> <p> Create Custom Field Folder </p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...
- `ghl-pp-cli custom-fields get-by-id` — <div> <p> Get Custom Field / Folder By Id.</p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...
- `ghl-pp-cli custom-fields get-by-object-key` — <div> <p> Get Custom Fields By Object Key</p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...
- `ghl-pp-cli custom-fields update` — <div> <p> Update Custom Field By Id </p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...
- `ghl-pp-cli custom-fields update-folder` — <div> <p> Create Custom Field Folder </p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...

**locations** — Manage locations

- `ghl-pp-cli locations create` — <div> <p>Create a new Sub-Account (Formerly Location) based on the data provided</p> <div> <span style= 'display:...
- `ghl-pp-cli locations delete` — Delete a Sub-Account (Formerly Location) from the Agency
- `ghl-pp-cli locations get` — Get details of a Sub-Account (Formerly Location) by passing the sub-account id
- `ghl-pp-cli locations put` — Update a Sub-Account (Formerly Location) based on the data provided
- `ghl-pp-cli locations search` — Search Sub-Account (Formerly Location)

**opportunities** — Manage opportunities

- `ghl-pp-cli opportunities create-opportunity` — Create Opportunity
- `ghl-pp-cli opportunities delete-opportunity` — Delete Opportunity
- `ghl-pp-cli opportunities get-lost-reason` — Get lost reason
- `ghl-pp-cli opportunities get-opportunity` — Get Opportunity
- `ghl-pp-cli opportunities get-pipelines` — Get Pipelines
- `ghl-pp-cli opportunities search-advanced` — Search Opportunities based on combinations of advanced filters. Documentation Link -...
- `ghl-pp-cli opportunities search-opportunity` — Search Opportunity
- `ghl-pp-cli opportunities update-opportunity` — Update Opportunity
- `ghl-pp-cli opportunities upsert-opportunity` — Upsert Opportunity

**users** — Manage users

- `ghl-pp-cli users create` — Create User
- `ghl-pp-cli users delete` — Delete User
- `ghl-pp-cli users filter-by-email` — Filter users by company ID, deleted status, and email array
- `ghl-pp-cli users get` — Get User
- `ghl-pp-cli users get-by-location` — Deprecated. Use `GET /users/search` instead. Pass `locationId` as a query parameter to filter results by location,...
- `ghl-pp-cli users search` — Search Users
- `ghl-pp-cli users update` — Update User

**workflows** — Documentation for Contacts API

- `ghl-pp-cli workflows` — Get Workflow


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ghl-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily Riley audit

```bash
ghl-pp-cli killswitch list --json --select id,name,phone,killswitch_tag
```

Lists every contact Riley must skip, with just the fields a workflow needs.

### Pre-send gate inside a workflow

```bash
ghl-pp-cli killswitch check $CONTACT_ID || exit 0
```

Returns exit code 2 or 3 if the contact is flagged; the workflow short-circuits without any SMS.

### Slim conversation list for an agent

```bash
ghl-pp-cli conversations list --json --select results.id,results.contactName,results.lastMessageDate,results.unreadCount
```

Default one-liner is already terse, but --select trims even further for context-window-sensitive callers.

### Friday client recency review

```bash
ghl-pp-cli contacts recency --tag client --over 14d --json
```

Lists every client tag-member whose last touchpoint was over two weeks ago, oldest first.

### Pipeline funnel ticker

```bash
ghl-pp-cli opportunities funnel <pipeline-id> --json
```

One row per stage with count and SUM(monetary_value); pipe to jq for charts.

## Auth Setup

Authenticates with a Private Integration Token (PIT) issued from a HighLevel sub-account location (Settings → Private Integrations). Store it once with `ghl-pp-cli auth set-token`, then every command sends `Authorization: Bearer <pit>` plus the required `Version: 2021-07-28` header on your behalf. The token is location-scoped — one CLI install per sub-account.

Run `ghl-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ghl-pp-cli calendars get --location-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
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
ghl-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ghl-pp-cli feedback --stdin < notes.txt
ghl-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.ghl-pp-cli/feedback.jsonl`. They are never POSTed unless `GHL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GHL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ghl-pp-cli profile save briefing --json
ghl-pp-cli --profile briefing calendars get --location-id 550e8400-e29b-41d4-a716-446655440000
ghl-pp-cli profile list --json
ghl-pp-cli profile show briefing
ghl-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ghl-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add ghl-pp-mcp -- ghl-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ghl-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ghl-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ghl-pp-cli <command> --help`.
