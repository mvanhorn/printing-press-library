# Conduyt Crm CLI

The Conduyt CRM API provides programmatic access to your CRM data including
contacts, companies, deals, pipelines, tasks, notes, automations, invoices,
email sequences, and more.

## Authentication

All authenticated endpoints require a Bearer token (API key) in the
`Authorization` header:

```
Authorization: Bearer cdy_<your-api-key>
```

API keys are generated in **Settings > API Keys** within the Conduyt
dashboard. Keys use the `cdy_` prefix and are hashed with bcrypt on
creation — the full key is shown exactly once at creation time.

Session cookie authentication (`conduyt_session`) is also supported for
browser-based clients but is not recommended for integrations.

## Rate Limiting

Most write endpoints enforce per-IP rate limits. When exceeded, the API
returns `429 Too Many Requests` with a `Retry-After` header indicating
seconds until the next allowed request.

| Endpoint Group      | Limit               |
|---------------------|---------------------|
| Login               | 5 / 15 min per IP   |
| Registration        | 3 / hour per IP     |
| Contact/Deal create | 30 / min per IP     |
| General             | 100 / 15 min per IP |

## Pagination

List endpoints support cursor-based pagination via query parameters:

- `page` — Page number (default: 1)
- `per_page` — Items per page (default: 50, max: 200)

Responses include a `meta` object: `{ page, per_page, total }`.

## Response Format

All successful responses wrap data in a `data` envelope:
```json
{ "data": { ... } }
```

Error responses use:
```json
{ "error": "Human-readable error message" }
```

## Multi-Tenancy

Conduyt is multi-tenant. All data is scoped to the authenticated user's
current account. API keys are bound to a specific account at creation time.

## Webhook Events

Outbound webhooks fire on events including:
`contact.created`, `contact.updated`, `contact.deleted`,
`deal.created`, `deal.updated`, `deal.won`, `deal.lost`,
`task.created`, `task.completed`, `note.created`,
`appointment.created`, `appointment.updated`,
`form.submitted`, `invoice.paid`

Payloads are signed with HMAC-SHA256 using the webhook's secret. Retries
follow exponential backoff: 1m, 5m, 15m, 1h, 6h, 24h, 72h (7 attempts).

Learn more at [Conduyt Crm](https://conduyt.app).

Created by [@ptaramona](https://github.com/ptaramona) (Conduyt).

## Install

The recommended path installs both the `conduyt-crm-pp-cli` binary and the `pp-conduyt-crm` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install conduyt-crm
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install conduyt-crm --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install conduyt-crm --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install conduyt-crm --agent claude-code
npx -y @mvanhorn/printing-press-library install conduyt-crm --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/cmd/conduyt-crm-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/conduyt-crm-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install conduyt-crm --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-conduyt-crm --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-conduyt-crm --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install conduyt-crm --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/conduyt-crm-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CONDUYT_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "conduyt-crm": {
      "command": "conduyt-crm-pp-mcp",
      "env": {
        "CONDUYT_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
conduyt-crm-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export CONDUYT_BEARER_AUTH="your-token-here"
```

### 3. Verify Setup

```bash
conduyt-crm-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
conduyt-crm-pp-cli activities list
```

## Usage

Run `conduyt-crm-pp-cli --help` for the full command reference and flag list.

## Commands

### activities

Activity feed and logging

- **`conduyt-crm-pp-cli activities create-activity`** - Log an activity
- **`conduyt-crm-pp-cli activities list`** - List activities

### admin

Super-admin account management and impersonation

- **`conduyt-crm-pp-cli admin clean-test-accounts`** - Delete test accounts
- **`conduyt-crm-pp-cli admin clean-test-data`** - Clean test data from the system
- **`conduyt-crm-pp-cli admin impersonate`** - Impersonate a user (super-admin)
- **`conduyt-crm-pp-cli admin list-accounts`** - List all accounts (super-admin)
- **`conduyt-crm-pp-cli admin stop-impersonate`** - Stop impersonating
- **`conduyt-crm-pp-cli admin toggle-comp`** - Toggle comp (free) status for an account

### ai

AI-powered features (chat, email compose, contact enrichment)

- **`conduyt-crm-pp-cli ai chat`** - AI chat assistant
- **`conduyt-crm-pp-cli ai compose-email`** - AI-assisted email composition
- **`conduyt-crm-pp-cli ai deal-insights`** - AI-generated deal insights and recommendations
- **`conduyt-crm-pp-cli ai enrich-contact`** - AI-powered contact data enrichment
- **`conduyt-crm-pp-cli ai improve-email`** - AI-assisted email improvement
- **`conduyt-crm-pp-cli ai summarize-contact`** - AI-generated contact summary

### api-keys

API key management

- **`conduyt-crm-pp-cli api-keys create`** - Returns the full key exactly once. Store it securely.
- **`conduyt-crm-pp-cli api-keys list`** - Returns API key metadata. Never returns the full key.
- **`conduyt-crm-pp-cli api-keys revoke`** - Revoke an API key

### appointments

Appointment scheduling

- **`conduyt-crm-pp-cli appointments create`** - Create an appointment
- **`conduyt-crm-pp-cli appointments delete`** - Delete an appointment
- **`conduyt-crm-pp-cli appointments get`** - Get an appointment by ID
- **`conduyt-crm-pp-cli appointments list`** - List all appointments
- **`conduyt-crm-pp-cli appointments update`** - Update an appointment

### automation-executions

Automation execution logs and step details

- **`conduyt-crm-pp-cli automation-executions get`** - Get execution details by ID
- **`conduyt-crm-pp-cli automation-executions list`** - List automation execution logs

### automations

Workflow automations (native + n8n), publishing, analytics

- **`conduyt-crm-pp-cli automations create`** - Create an automation
- **`conduyt-crm-pp-cli automations create-from-template`** - Create automation from a template
- **`conduyt-crm-pp-cli automations delete`** - Delete an automation
- **`conduyt-crm-pp-cli automations get`** - Get an automation by ID
- **`conduyt-crm-pp-cli automations list`** - List automations
- **`conduyt-crm-pp-cli automations list-actions`** - List available automation actions
- **`conduyt-crm-pp-cli automations list-condition-fields`** - List available condition fields for triggers
- **`conduyt-crm-pp-cli automations list-events`** - List available trigger events
- **`conduyt-crm-pp-cli automations list-templates`** - List automation templates
- **`conduyt-crm-pp-cli automations test-webhook`** - Send a test payload to an automation's webhook URL
- **`conduyt-crm-pp-cli automations update`** - Update an automation

### availability

Manage availability

- **`conduyt-crm-pp-cli availability get`** - Get current user's availability rules
- **`conduyt-crm-pp-cli availability set`** - Set availability rules

### billing

Stripe billing, checkout, and subscription status

- **`conduyt-crm-pp-cli billing create-checkout-session`** - Create a Stripe checkout session
- **`conduyt-crm-pp-cli billing create-portal`** - Create a Stripe billing portal session
- **`conduyt-crm-pp-cli billing get-status`** - Get subscription status
- **`conduyt-crm-pp-cli billing webhook-stripe`** - Stripe billing webhook

### booking-pages

Public booking pages (Calendly-style)

- **`conduyt-crm-pp-cli booking-pages create`** - Create a booking page
- **`conduyt-crm-pp-cli booking-pages delete`** - Delete a booking page
- **`conduyt-crm-pp-cli booking-pages get`** - Get a booking page by ID
- **`conduyt-crm-pp-cli booking-pages list`** - List booking pages
- **`conduyt-crm-pp-cli booking-pages update`** - Update a booking page

### bulk

Manage bulk

- **`conduyt-crm-pp-cli bulk delete-contacts`** - Bulk delete contacts
- **`conduyt-crm-pp-cli bulk edit-contacts`** - Bulk edit contact fields
- **`conduyt-crm-pp-cli bulk edit-deals`** - Bulk edit deal fields
- **`conduyt-crm-pp-cli bulk get-status`** - Get bulk operation status
- **`conduyt-crm-pp-cli bulk tag-contacts`** - Bulk add/remove tags on contacts
- **`conduyt-crm-pp-cli bulk update-contacts`** - Bulk update contacts with field values
- **`conduyt-crm-pp-cli bulk update-deals`** - Bulk update deals

### calendar

Internal calendar and appointment management

- **`conduyt-crm-pp-cli calendar connect-google`** - Initiate Google Calendar OAuth
- **`conduyt-crm-pp-cli calendar connect-microsoft`** - Initiate Microsoft Calendar OAuth
- **`conduyt-crm-pp-cli calendar create-event`** - Create an event on a connected calendar
- **`conduyt-crm-pp-cli calendar delete-connection`** - Disconnect a calendar
- **`conduyt-crm-pp-cli calendar delete-event`** - Delete a synced calendar event
- **`conduyt-crm-pp-cli calendar get-connection`** - Get a calendar connection by ID
- **`conduyt-crm-pp-cli calendar get-event`** - Get a synced calendar event
- **`conduyt-crm-pp-cli calendar google-callback`** - Google Calendar OAuth callback
- **`conduyt-crm-pp-cli calendar list-connections`** - List calendar connections
- **`conduyt-crm-pp-cli calendar list-events`** - List synced calendar events
- **`conduyt-crm-pp-cli calendar microsoft-callback`** - Microsoft Calendar OAuth callback
- **`conduyt-crm-pp-cli calendar sync`** - Trigger manual calendar sync
- **`conduyt-crm-pp-cli calendar update-event`** - Update a synced calendar event
- **`conduyt-crm-pp-cli calendar webhook-google`** - Google Calendar push notification webhook
- **`conduyt-crm-pp-cli calendar webhook-microsoft`** - Microsoft Calendar webhook

### calendars

Internal calendar and appointment management

- **`conduyt-crm-pp-cli calendars create`** - Create a calendar
- **`conduyt-crm-pp-cli calendars get`** - Get a calendar by ID
- **`conduyt-crm-pp-cli calendars list`** - List internal calendars
- **`conduyt-crm-pp-cli calendars update`** - Update a calendar

### calls

Call log management

- **`conduyt-crm-pp-cli calls create-record`** - Create a call record
- **`conduyt-crm-pp-cli calls get`** - Get a call by ID
- **`conduyt-crm-pp-cli calls list`** - List call records
- **`conduyt-crm-pp-cli calls update`** - Update a call record (e.g., add notes)

### chat

Internal team chat channels and messages

- **`conduyt-crm-pp-cli chat add-member`** - Add a member to a channel
- **`conduyt-crm-pp-cli chat add-reaction`** - Add a reaction to a message
- **`conduyt-crm-pp-cli chat create-channel`** - Create a chat channel
- **`conduyt-crm-pp-cli chat delete-message`** - Delete a chat message
- **`conduyt-crm-pp-cli chat edit-message`** - Edit a chat message
- **`conduyt-crm-pp-cli chat get-message`** - Get a chat message by ID
- **`conduyt-crm-pp-cli chat get-typing-status`** - Get who is currently typing
- **`conduyt-crm-pp-cli chat list-channels`** - List chat channels
- **`conduyt-crm-pp-cli chat list-messages`** - List messages in a channel
- **`conduyt-crm-pp-cli chat remove-member`** - Remove a member from a channel
- **`conduyt-crm-pp-cli chat remove-reaction`** - Remove a reaction from a message
- **`conduyt-crm-pp-cli chat send-message`** - Send a message in a channel
- **`conduyt-crm-pp-cli chat send-typing-indicator`** - Send a typing indicator
- **`conduyt-crm-pp-cli chat upload-file`** - Upload a file to a channel

### companies

Company (organization) management

- **`conduyt-crm-pp-cli companies create-company`** - Create a company
- **`conduyt-crm-pp-cli companies delete-company`** - Soft-delete a company
- **`conduyt-crm-pp-cli companies get-company`** - Get a company by ID
- **`conduyt-crm-pp-cli companies list`** - List companies
- **`conduyt-crm-pp-cli companies update-company`** - Update a company

### conduyt-auth

Manage conduyt auth

- **`conduyt-crm-pp-cli conduyt-auth accept-invite`** - Accept a team invitation
- **`conduyt-crm-pp-cli conduyt-auth change-password`** - Change password (authenticated)
- **`conduyt-crm-pp-cli conduyt-auth forgot-password`** - Request a password reset email
- **`conduyt-crm-pp-cli conduyt-auth get-me`** - Get current authenticated user
- **`conduyt-crm-pp-cli conduyt-auth login`** - Authenticates user credentials and returns a session cookie. Rate limited to 5 requests per 15 minutes per IP.
- **`conduyt-crm-pp-cli conduyt-auth logout`** - Log out (destroy session)
- **`conduyt-crm-pp-cli conduyt-auth register`** - Creates a new user and account. Rate limited to 3 requests per hour per IP.
- **`conduyt-crm-pp-cli conduyt-auth reset-password`** - Reset password with token
- **`conduyt-crm-pp-cli conduyt-auth switch-account`** - Switch to a different account

### conduyt-search

Manage conduyt search

- **`conduyt-crm-pp-cli conduyt-search global`** - Global search across contacts, companies, and deals

### contact

Contact management, tagging, scoring, import/export, merge, duplicates

- **`conduyt-crm-pp-cli contact upsert`** - Creates or updates a contact by email or phone match. Designed for inbound webhook integrations.

### contacts

Contact management, tagging, scoring, import/export, merge, duplicates

- **`conduyt-crm-pp-cli contacts create`** - Creates a new contact. Rate limited to 30 requests per minute.
- **`conduyt-crm-pp-cli contacts delete`** - Soft-delete a contact
- **`conduyt-crm-pp-cli contacts export`** - Export contacts as CSV
- **`conduyt-crm-pp-cli contacts find-duplicate`** - Find duplicate contacts
- **`conduyt-crm-pp-cli contacts get`** - Get a contact by ID
- **`conduyt-crm-pp-cli contacts get-import-template`** - Download CSV import template
- **`conduyt-crm-pp-cli contacts import`** - Import contacts from CSV
- **`conduyt-crm-pp-cli contacts list`** - Returns a paginated list of contacts. Supports search, filtering by tag,
source, company, assigned user, date ranges, lead score, and smart views.

Smart views: `needs_follow_up`, `hot_leads`, `new_this_week`,
`recently_won`, `untagged`, `missing_email`, `missing_phone`.
- **`conduyt-crm-pp-cli contacts list-smart-views`** - List available smart view definitions
- **`conduyt-crm-pp-cli contacts merge`** - Merge two contacts
- **`conduyt-crm-pp-cli contacts update`** - Update a contact

### conversations

Threaded conversation view per contact

- **`conduyt-crm-pp-cli conversations get`** - Get conversation thread for a contact
- **`conduyt-crm-pp-cli conversations list`** - List conversation threads

### custom-fields

Custom field definitions for contacts and deals

- **`conduyt-crm-pp-cli custom-fields create`** - Create a custom field definition
- **`conduyt-crm-pp-cli custom-fields delete`** - Delete a custom field definition
- **`conduyt-crm-pp-cli custom-fields list`** - List custom field definitions
- **`conduyt-crm-pp-cli custom-fields update`** - Update a custom field definition

### dashboard

Dashboard summary metrics

- **`conduyt-crm-pp-cli dashboard get`** - Get dashboard summary metrics

### deals

Deal/opportunity management within pipelines

- **`conduyt-crm-pp-cli deals create`** - Creates a new deal in a pipeline stage. Rate limited to 30 requests per minute.
- **`conduyt-crm-pp-cli deals delete`** - Soft-delete a deal
- **`conduyt-crm-pp-cli deals get`** - Get a deal by ID
- **`conduyt-crm-pp-cli deals list`** - Returns deals with Kanban-optimized sort order (stage, sortOrder, then requested sort).
- **`conduyt-crm-pp-cli deals update`** - Update a deal

### dialer

Click-to-call dialer via Twilio

- **`conduyt-crm-pp-cli dialer get-history`** - Get recent call history
- **`conduyt-crm-pp-cli dialer get-token`** - Get a Twilio browser token for click-to-call
- **`conduyt-crm-pp-cli dialer initiate-call`** - Initiate an outbound call

### document-templates

Proposal and contract templates with merge fields

- **`conduyt-crm-pp-cli document-templates create`** - Create a document template
- **`conduyt-crm-pp-cli document-templates delete`** - Delete a document template
- **`conduyt-crm-pp-cli document-templates get`** - Get a document template by ID
- **`conduyt-crm-pp-cli document-templates list`** - List document templates
- **`conduyt-crm-pp-cli document-templates update`** - Update a document template

### drip-campaigns

SMS drip campaign engine

- **`conduyt-crm-pp-cli drip-campaigns create`** - Create a drip campaign
- **`conduyt-crm-pp-cli drip-campaigns delete`** - Delete a drip campaign
- **`conduyt-crm-pp-cli drip-campaigns list`** - List SMS drip campaigns
- **`conduyt-crm-pp-cli drip-campaigns seed`** - Seed default drip campaigns
- **`conduyt-crm-pp-cli drip-campaigns update`** - Update a drip campaign

### drip-enrollments

Manage drip enrollments

- **`conduyt-crm-pp-cli drip-enrollments list`** - List drip enrollments

### email

Send individual and bulk emails

- **`conduyt-crm-pp-cli email send`** - Send an email to a contact
- **`conduyt-crm-pp-cli email send-bulk`** - Send bulk emails

### email-domains

Custom email domain verification (Resend)

- **`conduyt-crm-pp-cli email-domains add`** - Add a custom email domain
- **`conduyt-crm-pp-cli email-domains get`** - Get email domain configuration
- **`conduyt-crm-pp-cli email-domains remove`** - Remove email domain
- **`conduyt-crm-pp-cli email-domains update`** - Update email domain settings
- **`conduyt-crm-pp-cli email-domains verify`** - Verify DNS configuration for email domain

### emails

Send individual and bulk emails

- **`conduyt-crm-pp-cli emails create-sequence`** - Create an email sequence
- **`conduyt-crm-pp-cli emails create-template`** - Create an email template
- **`conduyt-crm-pp-cli emails delete-template`** - Delete an email template
- **`conduyt-crm-pp-cli emails enroll-in-sequence`** - Enroll contacts in a sequence
- **`conduyt-crm-pp-cli emails get-sequence`** - Get an email sequence by ID
- **`conduyt-crm-pp-cli emails get-sequence-stats`** - Get sequence performance stats
- **`conduyt-crm-pp-cli emails get-template`** - Get an email template by ID
- **`conduyt-crm-pp-cli emails list`** - List email messages
- **`conduyt-crm-pp-cli emails list-sequence-enrollments`** - List enrollments for a sequence
- **`conduyt-crm-pp-cli emails list-sequences`** - List email sequences
- **`conduyt-crm-pp-cli emails list-templates`** - List email templates
- **`conduyt-crm-pp-cli emails test-send-template`** - Send a test email from a template
- **`conduyt-crm-pp-cli emails unenroll-from-sequence`** - Unenroll contacts from a sequence
- **`conduyt-crm-pp-cli emails update-sequence`** - Update an email sequence
- **`conduyt-crm-pp-cli emails update-template`** - Update an email template

### files

File uploads and attachments

- **`conduyt-crm-pp-cli files create-record`** - Create a file attachment record
- **`conduyt-crm-pp-cli files delete`** - Delete a file attachment
- **`conduyt-crm-pp-cli files list`** - List file attachments
- **`conduyt-crm-pp-cli files upload`** - Upload a file

### forms

Lead capture forms and submissions

- **`conduyt-crm-pp-cli forms create`** - Create a form
- **`conduyt-crm-pp-cli forms delete`** - Delete a form
- **`conduyt-crm-pp-cli forms get`** - Get a form by ID
- **`conduyt-crm-pp-cli forms list`** - List forms
- **`conduyt-crm-pp-cli forms update`** - Update a form

### imports

CSV import jobs with mapping and deduplication

- **`conduyt-crm-pp-cli imports create`** - Create an import job
- **`conduyt-crm-pp-cli imports get`** - Get import job status
- **`conduyt-crm-pp-cli imports list`** - List import jobs
- **`conduyt-crm-pp-cli imports upload-file`** - Upload a CSV file for import

### integrations

Third-party integrations (Zapier, etc.)

- **`conduyt-crm-pp-cli integrations connect`** - Connect an integration
- **`conduyt-crm-pp-cli integrations create-zapier-subscription`** - Create a Zapier webhook subscription
- **`conduyt-crm-pp-cli integrations delete-zapier-subscription`** - Delete a Zapier subscription
- **`conduyt-crm-pp-cli integrations disconnect`** - Disconnect an integration
- **`conduyt-crm-pp-cli integrations get-zapier-sample-data`** - Get sample data for a Zapier event
- **`conduyt-crm-pp-cli integrations list`** - List active integrations
- **`conduyt-crm-pp-cli integrations list-zapier-subscriptions`** - List Zapier webhook subscriptions

### invoices

Invoice creation, sending, payments, PDF generation

- **`conduyt-crm-pp-cli invoices create`** - Create an invoice
- **`conduyt-crm-pp-cli invoices delete`** - Delete an invoice
- **`conduyt-crm-pp-cli invoices get`** - Get an invoice by ID
- **`conduyt-crm-pp-cli invoices get-next-number`** - Get the next auto-incremented invoice number
- **`conduyt-crm-pp-cli invoices list`** - List invoices
- **`conduyt-crm-pp-cli invoices update`** - Update an invoice

### messages

SMS and email message history

- **`conduyt-crm-pp-cli messages create`** - Create a message record
- **`conduyt-crm-pp-cli messages get-sms`** - Get an SMS message by ID
- **`conduyt-crm-pp-cli messages list`** - List messages
- **`conduyt-crm-pp-cli messages send-sms`** - Send an SMS message

### notes

Notes attached to contacts or deals

- **`conduyt-crm-pp-cli notes create`** - Body is capped at 50 KB. Returns 413 if exceeded.
- **`conduyt-crm-pp-cli notes delete`** - Delete a note
- **`conduyt-crm-pp-cli notes get`** - Get a note by ID
- **`conduyt-crm-pp-cli notes list`** - List notes
- **`conduyt-crm-pp-cli notes update`** - Update a note

### notifications

In-app notifications

- **`conduyt-crm-pp-cli notifications create`** - Create a notification
- **`conduyt-crm-pp-cli notifications list`** - List notifications
- **`conduyt-crm-pp-cli notifications mark-all-read`** - Mark all notifications as read
- **`conduyt-crm-pp-cli notifications mark-read`** - Mark a notification as read

### pipelines

Sales pipeline and stage management

- **`conduyt-crm-pp-cli pipelines create`** - Requires owner or admin role. Subject to plan limits.
- **`conduyt-crm-pp-cli pipelines delete`** - Delete a pipeline
- **`conduyt-crm-pp-cli pipelines get`** - Get a pipeline by ID
- **`conduyt-crm-pp-cli pipelines list`** - List pipelines with stages
- **`conduyt-crm-pp-cli pipelines update`** - Update a pipeline

### products

Product catalog for invoices

- **`conduyt-crm-pp-cli products create`** - Create a product
- **`conduyt-crm-pp-cli products delete`** - Delete a product
- **`conduyt-crm-pp-cli products get`** - Get a product by ID
- **`conduyt-crm-pp-cli products list`** - List products
- **`conduyt-crm-pp-cli products update`** - Update a product

### public

Unauthenticated public endpoints (booking, form submit)

- **`conduyt-crm-pp-cli public book-appointment`** - Book an appointment via public page
- **`conduyt-crm-pp-cli public get-booking-page`** - Get a public booking page by slug
- **`conduyt-crm-pp-cli public get-booking-slots`** - Get available time slots for a booking page

### push

Manage push

- **`conduyt-crm-pp-cli push get-public-key`** - Get VAPID public key for web push
- **`conduyt-crm-pp-cli push subscribe`** - Subscribe to web push notifications
- **`conduyt-crm-pp-cli push unsubscribe`** - Unsubscribe from web push

### reports

Pipeline, revenue, activity, team, and custom reports

- **`conduyt-crm-pp-cli reports create-custom`** - Create a custom report
- **`conduyt-crm-pp-cli reports delete-custom`** - Delete a custom report
- **`conduyt-crm-pp-cli reports get-activity`** - Activity report
- **`conduyt-crm-pp-cli reports get-custom`** - Get a custom report by ID
- **`conduyt-crm-pp-cli reports get-pipeline`** - Pipeline performance report
- **`conduyt-crm-pp-cli reports get-revenue`** - Revenue report
- **`conduyt-crm-pp-cli reports get-team`** - Team performance report
- **`conduyt-crm-pp-cli reports list-custom`** - List saved custom reports
- **`conduyt-crm-pp-cli reports run-custom`** - Execute a custom report and return results
- **`conduyt-crm-pp-cli reports update-custom`** - Update a custom report

### scoring-rules

Lead scoring rule management

- **`conduyt-crm-pp-cli scoring-rules create`** - Create a scoring rule
- **`conduyt-crm-pp-cli scoring-rules delete`** - Delete a scoring rule
- **`conduyt-crm-pp-cli scoring-rules list`** - List lead scoring rules
- **`conduyt-crm-pp-cli scoring-rules recalculate-scores`** - Recalculate all contact scores
- **`conduyt-crm-pp-cli scoring-rules update`** - Update a scoring rule

### settings

Account settings, branding, SMS/Twilio configuration

- **`conduyt-crm-pp-cli settings get`** - Get account settings
- **`conduyt-crm-pp-cli settings get-branding`** - Get white-label branding settings
- **`conduyt-crm-pp-cli settings get-sms`** - Get SMS provider settings
- **`conduyt-crm-pp-cli settings get-twilio`** - Get Twilio configuration
- **`conduyt-crm-pp-cli settings test-integration`** - Test an integration connection
- **`conduyt-crm-pp-cli settings test-sms`** - Send a test SMS
- **`conduyt-crm-pp-cli settings test-twilio`** - Test Twilio configuration
- **`conduyt-crm-pp-cli settings update`** - Update account settings
- **`conduyt-crm-pp-cli settings update-branding`** - Update white-label branding
- **`conduyt-crm-pp-cli settings update-sms`** - Update SMS provider settings
- **`conduyt-crm-pp-cli settings update-twilio`** - Update Twilio configuration

### smart-lists

Static contact lists

- **`conduyt-crm-pp-cli smart-lists create`** - Create a smart list
- **`conduyt-crm-pp-cli smart-lists list`** - List smart lists (static contact lists)
- **`conduyt-crm-pp-cli smart-lists update`** - Update a smart list

### tags

Tag management and merging

- **`conduyt-crm-pp-cli tags create`** - Create a tag
- **`conduyt-crm-pp-cli tags delete`** - Delete a tag
- **`conduyt-crm-pp-cli tags list`** - List tags
- **`conduyt-crm-pp-cli tags merge`** - Merge two tags
- **`conduyt-crm-pp-cli tags update`** - Update a tag

### tasks

Task management with assignment and due dates

- **`conduyt-crm-pp-cli tasks create`** - Create a task
- **`conduyt-crm-pp-cli tasks delete`** - Delete a task
- **`conduyt-crm-pp-cli tasks get`** - Get a task by ID
- **`conduyt-crm-pp-cli tasks list`** - List tasks
- **`conduyt-crm-pp-cli tasks update`** - Update a task

### users

Team member management and invitations

- **`conduyt-crm-pp-cli users get`** - Get a team member by ID
- **`conduyt-crm-pp-cli users invite`** - Invite a team member
- **`conduyt-crm-pp-cli users list`** - List team members
- **`conduyt-crm-pp-cli users remove`** - Remove a team member
- **`conduyt-crm-pp-cli users update`** - Update a team member

### webhook-logs

Manage webhook logs

- **`conduyt-crm-pp-cli webhook-logs list`** - List webhook delivery logs

### webhooks

Outbound webhook management and logs

- **`conduyt-crm-pp-cli webhooks create`** - URL is validated for SSRF protection. HMAC signing secret is auto-generated.
- **`conduyt-crm-pp-cli webhooks delete`** - Delete a webhook
- **`conduyt-crm-pp-cli webhooks get`** - Get a webhook by ID
- **`conduyt-crm-pp-cli webhooks inbound-contact`** - Inbound webhook for contact data
- **`conduyt-crm-pp-cli webhooks inbound-deal`** - Inbound webhook for deal data
- **`conduyt-crm-pp-cli webhooks list`** - List outbound webhooks
- **`conduyt-crm-pp-cli webhooks list-endpoints`** - List configured webhook endpoints
- **`conduyt-crm-pp-cli webhooks messages`** - Inbound webhook for message events
- **`conduyt-crm-pp-cli webhooks receive`** - Receive an inbound webhook payload
- **`conduyt-crm-pp-cli webhooks sms-inbound`** - Twilio inbound SMS webhook
- **`conduyt-crm-pp-cli webhooks sms-status`** - Twilio SMS status callback
- **`conduyt-crm-pp-cli webhooks stripe-invoice`** - Stripe invoice webhook
- **`conduyt-crm-pp-cli webhooks test`** - Send a test payload to a webhook
- **`conduyt-crm-pp-cli webhooks update`** - Update a webhook
- **`conduyt-crm-pp-cli webhooks voice-inbound`** - Twilio inbound voice webhook
- **`conduyt-crm-pp-cli webhooks voice-recording`** - Twilio recording callback
- **`conduyt-crm-pp-cli webhooks voice-status`** - Twilio voice status callback
- **`conduyt-crm-pp-cli webhooks voice-voicemail`** - Twilio voicemail callback

### workflows

Simple trigger-action workflows

- **`conduyt-crm-pp-cli workflows create`** - Create a workflow
- **`conduyt-crm-pp-cli workflows delete`** - Delete a workflow
- **`conduyt-crm-pp-cli workflows get`** - Get a workflow by ID
- **`conduyt-crm-pp-cli workflows list`** - List workflows
- **`conduyt-crm-pp-cli workflows update`** - Update a workflow

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
conduyt-crm-pp-cli activities list

# JSON for scripting and agents
conduyt-crm-pp-cli activities list --json

# Filter to specific fields
conduyt-crm-pp-cli activities list --json --select id,name,status

# Dry run — show the request without sending
conduyt-crm-pp-cli activities list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
conduyt-crm-pp-cli activities list --agent
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

## Health Check

```bash
conduyt-crm-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/conduyt-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CONDUYT_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `conduyt-crm-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CONDUYT_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
