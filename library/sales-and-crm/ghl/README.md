# GoHighLevel CLI

**Every GoHighLevel surface that matters for a marketing/CRM operator — with offline SQL, kill-switch tag awareness, and a local cache that makes Riley-grade automation safe.**

Wraps the GoHighLevel v2 API (contacts, conversations, calendars, opportunities, workflows, tags, custom fields) with a Go binary that mirrors every endpoint, caches every entity in local SQLite, and adds the surfaces the official MCP lacks: workflow membership, tag analytics, cross-entity activity windows, send preflight, and kill-switch roster. Default output is token-efficient one-liners; --full and --select expose the rest.

Learn more at [GoHighLevel](https://highlevel.stoplight.io/docs/integrations/).

## Install

The recommended path installs both the `ghl-pp-cli` binary and the `pp-ghl` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install ghl
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install ghl --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ghl-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ghl --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ghl --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-ghl skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-ghl. The skill defines how its required CLI can be installed.
```

## Authentication

Authenticates with a Private Integration Token (PIT) issued from a HighLevel sub-account location (Settings → Private Integrations). Store it once with `ghl-pp-cli auth set-token`, then every command sends `Authorization: Bearer <pit>` plus the required `Version: 2021-07-28` header on your behalf. The token is location-scoped — one CLI install per sub-account.

## Quick Start

```bash
# Paste your PIT once — stored under XDG config, not in any spec or repo.
ghl-pp-cli auth set-token


# Verify the PIT reaches the API and locate which location it owns.
ghl-pp-cli doctor


# Populate the local store so all transcendence commands (killswitch, activity, tags stats) have data.
ghl-pp-cli sync --full


# Audit every contact tagged `ai off` or `human handover` before any nurture run.
ghl-pp-cli killswitch list --json


# One-line daily metric ticker for a business-dashboard cron.
ghl-pp-cli kpi today --json

```

## Unique Features

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

## Usage

Run `ghl-pp-cli --help` for the full command reference and flag list.

## Commands

### calendars

Manage calendars

- **`ghl-pp-cli calendars add-to-schedule`** - Associates a calendar with the given schedule by adding the calendarId to a schedule
- **`ghl-pp-cli calendars create`** - Create calendar in a location.
- **`ghl-pp-cli calendars create-appointment`** - Create appointment
- **`ghl-pp-cli calendars create-appointment-note`** - Create Note
- **`ghl-pp-cli calendars create-block-slot`** - Create block slot
- **`ghl-pp-cli calendars create-group`** - Create Calendar Group
- **`ghl-pp-cli calendars create-resource`** - Create calendar resource by resource type
- **`ghl-pp-cli calendars create-schedule`** - Create new schedule with specified rules, timezone, location, user and calendar associations.
- **`ghl-pp-cli calendars delete`** - Delete calendar by ID
- **`ghl-pp-cli calendars delete-appointment-note`** - Delete Note
- **`ghl-pp-cli calendars delete-event`** - Delete event by ID
- **`ghl-pp-cli calendars delete-group`** - Delete Group
- **`ghl-pp-cli calendars delete-resource`** - Delete calendar resource by ID
- **`ghl-pp-cli calendars delete-schedule`** - Permanently remove a schedule and all its associated rules. This action cannot be undone.
- **`ghl-pp-cli calendars disable-group`** - Disable Group
- **`ghl-pp-cli calendars edit-appointment`** - Update appointment
- **`ghl-pp-cli calendars edit-block-slot`** - Update block slot by ID
- **`ghl-pp-cli calendars edit-group`** - Update Group by group ID
- **`ghl-pp-cli calendars fetch-resources`** - List calendar resources by resource type and location ID
- **`ghl-pp-cli calendars get`** - Get all calendars in a location.
- **`ghl-pp-cli calendars get-all-schedules`** - Retrieve user availability schedules based on various filters including location, calendar, and user. Supports pagination.
- **`ghl-pp-cli calendars get-appointment`** - Get appointment by ID
- **`ghl-pp-cli calendars get-appointment-notes`** - Get Appointment Notes
- **`ghl-pp-cli calendars get-blocked-slots`** - Get Blocked Slots
- **`ghl-pp-cli calendars get-calendarid`** - Get calendar by ID
- **`ghl-pp-cli calendars get-events`** - Get Calendar Events
- **`ghl-pp-cli calendars get-groups`** - Get all calendar groups in a location.
- **`ghl-pp-cli calendars get-resource`** - Get calendar resource by ID
- **`ghl-pp-cli calendars get-schedule-by-id`** - Retrieve a specific schedule by its unique identifier. Returns detailed information including rules, timezone, and associated calendars/users.
- **`ghl-pp-cli calendars remove-from-schedule`** - Removes the association between a team calendar and the given schedule by removing the calendarId from the schedule
- **`ghl-pp-cli calendars update`** - Update calendar by ID.
- **`ghl-pp-cli calendars update-appointment-note`** - Update Note
- **`ghl-pp-cli calendars update-resource`** - Update calendar resource by ID
- **`ghl-pp-cli calendars update-schedule`** - Modify an existing schedule by updating its rules, timezone, and name All fields are optional - only provided fields will be updated.
- **`ghl-pp-cli calendars validate-groups-slug`** - Validate if group slug is available or not.

### contacts

Documentation for Contacts API

- **`ghl-pp-cli contacts add-remove-from-business`** - Add/Remove Contacts From Business . Passing a `null` businessId will remove the businessId from the contacts
- **`ghl-pp-cli contacts create`** - Please find the list of acceptable values for the `country` field  <a href="https://highlevel.stoplight.io/docs/integrations/ZG9jOjI4MzUzNDIy-country-list" target="_blank">here</a>
- **`ghl-pp-cli contacts create-association`** - Allows you to update tags to multiple contacts at once, you can add or remove tags from the contacts
- **`ghl-pp-cli contacts delete`** - Delete Contact
- **`ghl-pp-cli contacts get`** - Get Contacts

 **Note:** This API endpoint is deprecated. Please use the [Search Contacts](https://marketplace.gohighlevel.com/docs/ghl/contacts/search-contacts-advanced) endpoint instead.
- **`ghl-pp-cli contacts get-by-business-id`** - Get Contacts By BusinessId
- **`ghl-pp-cli contacts get-contactid`** - Get Contact
- **`ghl-pp-cli contacts get-duplicate`** - Get Duplicate Contact.<br/><br/>If `Allow Duplicate Contact` is disabled under Settings, the global unique identifier will be used for searching the contact. If the setting is enabled, first priority for search is `email` and the second priority will be `phone`.
- **`ghl-pp-cli contacts search-advanced`** - Search contacts based on combinations of advanced filters. Documentation Link - https://doc.clickup.com/8631005/d/h/87cpx-158396/6e629989abe7fad
- **`ghl-pp-cli contacts update`** - Please find the list of acceptable values for the `country` field  <a href="https://highlevel.stoplight.io/docs/integrations/ZG9jOjI4MzUzNDIy-country-list" target="_blank">here</a>
- **`ghl-pp-cli contacts upsert`** - Please find the list of acceptable values for the `country` field  <a href="https://highlevel.stoplight.io/docs/integrations/ZG9jOjI4MzUzNDIy-country-list" target="_blank">here</a><br/><br/>The Upsert API will adhere to the configuration defined under the “Allow Duplicate Contact” setting at the Location level. If the setting is configured to check both Email and Phone, the API will attempt to identify an existing contact based on the priority sequence specified in the setting, and will create or update the contact accordingly.<br/><br/>If two separate contacts already exist—one with the same email and another with the same phone—and an upsert request includes both the email and phone, the API will update the contact that matches the first field in the configured sequence, and ignore the second field to prevent duplication.

### conversations

Manage conversations

- **`ghl-pp-cli conversations add-an-inbound-message`** - Post the necessary fields for the API to add a new inbound message. <br />
- **`ghl-pp-cli conversations add-an-outbound-message`** - Post the necessary fields for the API to add a new outbound call.
- **`ghl-pp-cli conversations add-message-attachments`** - Set attachments on an existing message (replaces existing). Maximum 5 URLs. Supported for TYPE_CUSTOM_CALL (34) and TYPE_CALL (1) with subType EXTERNAL_CALL.
- **`ghl-pp-cli conversations cancel-scheduled-email-message`** - Post the messageId for the API to delete a scheduled email message. <br />
- **`ghl-pp-cli conversations cancel-scheduled-message`** - Post the messageId for the API to delete a scheduled message. <br />
- **`ghl-pp-cli conversations complete-file-upload`** - Validates the uploaded file in GCS and returns the public URL. Call this endpoint after successfully uploading the file to the signed URL.
- **`ghl-pp-cli conversations create`** - Creates a new conversation with the data provided
- **`ghl-pp-cli conversations create-custom-subtype`** - Create a new custom subtype for a location. Requires agency or account admin role.
- **`ghl-pp-cli conversations delete`** - Delete the conversation details based on the conversation ID
- **`ghl-pp-cli conversations download-message-transcription`** - Download the recording transcription for a message by passing the message id
- **`ghl-pp-cli conversations export-messages-by-location`** - Export messages for a specific location with cursor-based pagination support. Response includes messageType (string), source, and subType fields. The channel parameter is optional - if not provided, all non-email message types will be returned including activity messages (opportunity updates, appointments, etc.).
- **`ghl-pp-cli conversations get`** - Get the conversation details based on the conversation ID
- **`ghl-pp-cli conversations get-all-custom-subtypes`** - Get all custom subtypes for a location
- **`ghl-pp-cli conversations get-contact-unsubscription-status`** - Get all subscription statuses for a contact (all emails or specific email)
- **`ghl-pp-cli conversations get-email-by-id`** - Get email by Id
- **`ghl-pp-cli conversations get-message`** - Get message by message id.
- **`ghl-pp-cli conversations get-message-recording`** - Get the recording for a message by passing the message id
- **`ghl-pp-cli conversations get-message-transcription`** - Get the recording transcription for a message by passing the message id
- **`ghl-pp-cli conversations initiate-file-upload`** - Generates a signed URL for direct file upload to Google Cloud Storage. Returns a signed URL valid for 15 minutes. Upload file via PUT request, then call /complete to finalize.
- **`ghl-pp-cli conversations live-chat-agent-typing`** - Agent/AI-Bot will call this when they are typing a message in live chat message
- **`ghl-pp-cli conversations search`** - Returns a list of all conversations matching the search criteria along with the sort and filter options selected.
- **`ghl-pp-cli conversations send-a-new-message`** - Post the necessary fields for the API to send a new message.
- **`ghl-pp-cli conversations send-review-reply`** - Post a reply to a customer review on Google My Business
- **`ghl-pp-cli conversations update`** - Update the conversation details based on the conversation ID
- **`ghl-pp-cli conversations update-custom-subtype`** - Update or archive a custom subtype. Requires agency or account admin role.
- **`ghl-pp-cli conversations update-message-status`** - Post the necessary fields for the API to update message status.
- **`ghl-pp-cli conversations upload-file-attachments`** - Post the necessary fields for the API to upload files. The files need to be a buffer with the key "fileAttachment". <br /><br /> The allowed file types are: <br/> <ul><li>JPG</li><li>JPEG</li><li>PNG</li><li>MP4</li><li>MPEG</li><li>ZIP</li><li>RAR</li><li>PDF</li><li>DOC</li><li>DOCX</li><li>TXT</li><li>MP3</li><li>WAV</li></ul> <br /><br /> The API will return an object with the URLs
- **`ghl-pp-cli conversations user-subscription-change`** - Process subscription change initiated by a user (admin/agent). Supports individual custom subscription changes and resub all functionality. Legal forms are automatically created for user-initiated resubscribe actions on custom subscriptions.

### custom-fields

Manage custom fields

- **`ghl-pp-cli custom-fields create`** - <div>
                  <p> Create Custom Field </p> 
                  <div>
                    <span style= "display: inline-block;
                                width: 25px; height: 25px;
                                background-color: yellow;
                                color: black;
                                font-weight: bold;
                                font-size: 24px;
                                text-align: center;
                                line-height: 22px;
                                border: 2px solid black;
                                border-radius: 10%;
                                margin-right: 10px;">
                                !
                      </span>
                      <span>
                        <strong>
                        Only supports Custom Objects and Company (Business) today. Will be extended to other Standard Objects in the future.
                        </strong>
                      </span>
                  </div>
                </div>
- **`ghl-pp-cli custom-fields create-folder`** - <div>
    <p> Create Custom Field Folder </p> 
    <div>
      <span style= "display: inline-block;
                  width: 25px; height: 25px;
                  background-color: yellow;
                  color: black;
                  font-weight: bold;
                  font-size: 24px;
                  text-align: center;
                  line-height: 22px;
                  border: 2px solid black;
                  border-radius: 10%;
                  margin-right: 10px;">
                  !
        </span>
        <span>
          <strong>
          Only supports Custom Objects and Company (Business) today. Will be extended to other Standard Objects in the future.
          </strong>
        </span>
    </div>
  </div>
- **`ghl-pp-cli custom-fields delete`** - <div>
    <p> Delete Custom Field By Id </p> 
    <div>
      <span style= "display: inline-block;
                  width: 25px; height: 25px;
                  background-color: yellow;
                  color: black;
                  font-weight: bold;
                  font-size: 24px;
                  text-align: center;
                  line-height: 22px;
                  border: 2px solid black;
                  border-radius: 10%;
                  margin-right: 10px;">
                  !
        </span>
        <span>
          <strong>
          Only supports Custom Objects and Company (Business) today. Will be extended to other Standard Objects in the future.
          </strong>
        </span>
    </div>
  </div>
- **`ghl-pp-cli custom-fields delete-folder`** - <div>
    <p> Create Custom Field Folder </p> 
    <div>
      <span style= "display: inline-block;
                  width: 25px; height: 25px;
                  background-color: yellow;
                  color: black;
                  font-weight: bold;
                  font-size: 24px;
                  text-align: center;
                  line-height: 22px;
                  border: 2px solid black;
                  border-radius: 10%;
                  margin-right: 10px;">
                  !
        </span>
        <span>
          <strong>
          Only supports Custom Objects and Company (Business) today. Will be extended to other Standard Objects in the future.
          </strong>
        </span>
    </div>
  </div>
- **`ghl-pp-cli custom-fields get-by-id`** - <div>
                  <p> Get Custom Field / Folder By Id.</p> 
                  <div>
                    <span style= "display: inline-block;
                                width: 25px; height: 25px;
                                background-color: yellow;
                                color: black;
                                font-weight: bold;
                                font-size: 24px;
                                text-align: center;
                                line-height: 22px;
                                border: 2px solid black;
                                border-radius: 10%;
                                margin-right: 10px;">
                                !
                      </span>
                      <span>
                        <strong>
                        Only supports Custom Objects and Company (Business) today. Will be extended to other Standard Objects in the future.
                        </strong>
                      </span>
                  </div>
                </div>
- **`ghl-pp-cli custom-fields get-by-object-key`** - <div>
                  <p> Get Custom Fields By Object Key</p> 
                  <div>
                    <span style= "display: inline-block;
                                width: 25px; height: 25px;
                                background-color: yellow;
                                color: black;
                                font-weight: bold;
                                font-size: 24px;
                                text-align: center;
                                line-height: 22px;
                                border: 2px solid black;
                                border-radius: 10%;
                                margin-right: 10px;">
                                !
                      </span>
                      <span>
                        <strong>
                        Only supports Custom Objects and Company (Business) today. Will be extended to other Standard Objects in the future.
                        </strong>
                      </span>
                  </div>
                </div>
- **`ghl-pp-cli custom-fields update`** - <div>
    <p> Update Custom Field By Id </p> 
    <div>
      <span style= "display: inline-block;
                  width: 25px; height: 25px;
                  background-color: yellow;
                  color: black;
                  font-weight: bold;
                  font-size: 24px;
                  text-align: center;
                  line-height: 22px;
                  border: 2px solid black;
                  border-radius: 10%;
                  margin-right: 10px;">
                  !
        </span>
        <span>
          <strong>
          Only supports Custom Objects and Company (Business) today. Will be extended to other Standard Objects in the future.
          </strong>
        </span>
    </div>
  </div>
- **`ghl-pp-cli custom-fields update-folder`** - <div>
    <p> Create Custom Field Folder </p> 
    <div>
      <span style= "display: inline-block;
                  width: 25px; height: 25px;
                  background-color: yellow;
                  color: black;
                  font-weight: bold;
                  font-size: 24px;
                  text-align: center;
                  line-height: 22px;
                  border: 2px solid black;
                  border-radius: 10%;
                  margin-right: 10px;">
                  !
        </span>
        <span>
          <strong>
          Only supports Custom Objects and Company (Business) today. Will be extended to other Standard Objects in the future.
          </strong>
        </span>
    </div>
  </div>

### locations

Manage locations

- **`ghl-pp-cli locations create`** - <div>
                  <p>Create a new Sub-Account (Formerly Location) based on the data provided</p> 
                  <div>
                    <span style= "display: inline-block;
                                width: 25px; height: 25px;
                                background-color: yellow;
                                color: black;
                                font-weight: bold;
                                font-size: 24px;
                                text-align: center;
                                line-height: 22px;
                                border: 2px solid black;
                                border-radius: 10%;
                                margin-right: 10px;">
                                !
                      </span>
                      <span>
                        <strong>
                          This feature is only available on Agency Pro ($497) plan.
                        </strong>
                      </span>
                  </div>
                </div>
- **`ghl-pp-cli locations delete`** - Delete a Sub-Account (Formerly Location) from the Agency
- **`ghl-pp-cli locations get`** - Get details of a Sub-Account (Formerly Location) by passing the sub-account id
- **`ghl-pp-cli locations put`** - Update a Sub-Account (Formerly Location) based on the data provided
- **`ghl-pp-cli locations search`** - Search Sub-Account (Formerly Location)

### opportunities

Manage opportunities

- **`ghl-pp-cli opportunities create-opportunity`** - Create Opportunity
- **`ghl-pp-cli opportunities delete-opportunity`** - Delete Opportunity
- **`ghl-pp-cli opportunities get-lost-reason`** - Get lost reason
- **`ghl-pp-cli opportunities get-opportunity`** - Get Opportunity
- **`ghl-pp-cli opportunities get-pipelines`** - Get Pipelines
- **`ghl-pp-cli opportunities search-advanced`** - Search Opportunities based on combinations of advanced filters. Documentation Link - https://doc.clickup.com/8631005/d/h/87cpx-424216/7bf11bc9b94f80f
- **`ghl-pp-cli opportunities search-opportunity`** - Search Opportunity
- **`ghl-pp-cli opportunities update-opportunity`** - Update Opportunity
- **`ghl-pp-cli opportunities upsert-opportunity`** - Upsert Opportunity

### users

Manage users

- **`ghl-pp-cli users create`** - Create User
- **`ghl-pp-cli users delete`** - Delete User
- **`ghl-pp-cli users filter-by-email`** - Filter users by company ID, deleted status, and email array
- **`ghl-pp-cli users get`** - Get User
- **`ghl-pp-cli users get-by-location`** - Deprecated. Use `GET /users/search` instead. Pass `locationId` as a query parameter to filter results by location, along with the required `companyId` and other search filters as needed.
- **`ghl-pp-cli users search`** - Search Users
- **`ghl-pp-cli users update`** - Update User

### workflows

Documentation for Contacts API

- **`ghl-pp-cli workflows get`** - Get Workflow


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ghl-pp-cli calendars get --location-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
ghl-pp-cli calendars get --location-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
ghl-pp-cli calendars get --location-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
ghl-pp-cli calendars get --location-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ghl-pp-cli calendars get --location-id 550e8400-e29b-41d4-a716-446655440000 --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-ghl -g
```

Then invoke `/pp-ghl <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add ghl ghl-pp-mcp -e GOHIGHLEVEL_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ghl-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GOHIGHLEVEL_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ghl": {
      "command": "ghl-pp-mcp",
      "env": {
        "GOHIGHLEVEL_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
ghl-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/gohighlevel-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GOHIGHLEVEL_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ghl-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GOHIGHLEVEL_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`Invalid Private Integration token`** — Regenerate the PIT in HighLevel Settings → Private Integrations and re-run `ghl-pp-cli auth set-token`. Sub-account PITs are scoped to one location; verify you are in the right sub-account.
- **Empty results from `killswitch list` or `tags stats`** — Run `ghl-pp-cli sync --full` — these commands read the local store. Without a sync the join has no rows.
- **`429 Too Many Requests`** — GHL caps roughly 10 req/sec per location. The CLI surfaces a typed `*cliutil.RateLimitError`; wait the suggested seconds or rerun with `--rate-limit-rps 5`.
- **Required header `Version` missing** — Should never appear — the client injects `Version: 2021-07-28` automatically. If you see it, file a bug; do not pass `--header` manually.

---

## Known Gaps

These are documented limitations from the v1 generator run. None of them block the live-API read path, which is verified end-to-end.

- **`sync` only populates `locations` + `templates`.** The press's sync resource detector did not enumerate every primary entity for the GHL spec because most list endpoints require a `locationId` query param. Workaround until the generator-side fix lands: call list endpoints directly with `--location-id <loc>` (`contacts get`, `calendars get`, `opportunities search-opportunity`, `workflows`, etc.) or pipe a search via `contacts search-advanced --stdin`. Transcendence commands that read only from the local store (`killswitch list`, `tags stats`, `kpi today`, `activity`, `contacts recency`, `inbox triage`, `opportunities stale`, `opportunities funnel`, `workflows members`) will return empty results until you manually populate the store or the sync gap is fixed. The kill-switch READ path that matters most for live agent loops — `killswitch check <id> --live-fallback` — works fine today.
- **`opportunities search-advanced` demands a spurious flag.** The generator emitted `--additional-details-calendar-events` as required even though the API doesn't need it. Use `opportunities search-opportunity` instead — same surface, no false-positive flag.
- **`contacts tags` has no read subcommand.** GHL's `GET /contacts/{contactId}/tags` exists but wasn't emitted as a Cobra subcommand. Read tags via the parent `contacts get-contactid <id>` JSON output — the `tags` array is in the default response (and `--compact` preserves it).
- **Flag-name normalization is uneven across commands.** Most commands use `--location-id` (kebab), some use `--locationId`, some require positional. Use `<cmd> --help` to confirm shape per command.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Official HighLevel MCP**](https://marketplace.gohighlevel.com/docs/other/mcp/) — JavaScript
- [**mastanley13/GoHighLevel-MCP**](https://github.com/mastanley13/GoHighLevel-MCP) — JavaScript
- [**basicmachines-co/open-ghl-mcp**](https://github.com/basicmachines-co/open-ghl-mcp) — Python
- [**BusyBee3333/Go-High-Level-MCP-2026-Complete**](https://github.com/BusyBee3333/Go-High-Level-MCP-2026-Complete) — TypeScript
- [**@gohighlevel/api-client**](https://www.npmjs.com/package/@gohighlevel/api-client) — TypeScript
- [**M2KDevelopments/gohighlevel**](https://github.com/M2KDevelopments/gohighlevel) — JavaScript
- [**GoHighLevel/highlevel-api-docs**](https://github.com/GoHighLevel/highlevel-api-docs) — JSON

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
