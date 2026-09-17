---
name: pp-conduyt-crm
description: "Every Conduyt endpoint as a command, plus send-safety, import forensics, report deltas and dialer coverage no other Conduyt surface has. Trigger phrases: `is this list safe to text`, `did the morning import land`, `who is on pace this month`, `compare speed to lead week over week`, `what is the dialer handing out next`, `use conduyt-crm`, `run conduyt-crm`."
author: "Paul Taramona"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - conduyt-crm-pp-cli
    install:
      - kind: go
        bins: [conduyt-crm-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/cmd/conduyt-crm-pp-cli
---

# Conduyt CRM — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `conduyt-crm-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install conduyt-crm --cli-only
   ```
2. Verify: `conduyt-crm-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/cmd/conduyt-crm-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

The terminal and MCP surface of an AI-native CRM. Mirror the whole API with agent-native output, keep a local SQLite copy for offline search and SQL, and reach for the compound commands (send-check, imports blame, reports scorecard, dialer coverage) that answer the questions the app spreads across several screens.

## When to Use This CLI

Use this CLI when an agent or a script must read or change Conduyt CRM data from a terminal: contacts, deals, pipelines, automations, imports, messaging, dialer state and reports. It is the right choice for compound questions (is this list safe to text, did the import land, who is on pace, what will the dialer hand out next) and for bounded JSON with --select.

## Anti-triggers

Do not use this CLI for:
- Do not use it to place or drive live calls for a rep; the Smart Dialing queue hand-out (next, skip, worked, release) is human-session only in the app.
- Do not use it to send bulk SMS without running send-check first; consent and line-type rules apply.
- Do not use it for billing or subscription changes; those live in the app's Account settings.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Send safety
- **`send-check`** — A go/no-go verdict for a send list before any SMS or drip: missing phones, unverified line types, invalid numbers and DNC hits in one table.

  _Run it before texting a list so an agent never repeats a Kloudi-style blast to landlines or opted-out numbers._

  ```bash
  conduyt-crm-pp-cli send-check --list ea9fb7aa-7443-44c9-b206-32dbdd6b3c96 --limit 2000 --agent
  ```
- **`imports blame`** — For one import job: who got the SMS, who was skipped and why, grouped by reason, including queued and exhausted side effects.

  _Use it after an import finishes to explain delivery outcomes per row instead of stitching four calls._

  ```bash
  conduyt-crm-pp-cli imports blame 0f911935-c98a-450a-8e24-93bf21016bfd --json
  ```
- **`imports watch`** — Blocks until an import job reaches a terminal state, then prints the created/updated/skipped/error recap with typed exit codes.

  _Reach for it when a pipeline must wait on an import and fail loudly when it does not land. Verification is scoped to the import job, not tenant-wide delivery activity._

  ```bash
  conduyt-crm-pp-cli imports watch 0f911935-c98a-450a-8e24-93bf21016bfd --verify --interval 10s
  ```
- **`contacts verify-line-type`** — Counts the unverified numbers in scope from the local store and shows the tenant-paid Twilio exposure before the paid verification loop runs.

  _Use it before spending on line-type verification, then drive the loop from the same command._

  ```bash
  conduyt-crm-pp-cli contacts verify-line-type --estimate --smart-list ea9fb7aa-7443-44c9-b206-32dbdd6b3c96
  ```

### Reports as commands
- **`reports scorecard`** — Target versus actual per user and metric with attainment percent and pace to the end of the period.

  _Pick it when asked who is on pace, off pace, or how far from target a rep is._

  ```bash
  conduyt-crm-pp-cli reports scorecard --period month --agent
  ```
- **`reports compare`** — Any core report called for a window and the equal prior window, with prior, current, delta and delta percent on every numeric value.

  _Use it to explain why a number moved week over week without pulling both reports by hand._

  ```bash
  conduyt-crm-pp-cli reports compare speed-to-lead --range 7d --json
  ```
- **`automations hours-audit`** — Every Send SMS, Send Email and Assign step across synced automations, listed with or without an operating-hours window. With `--published-only`, `summary.unpublished` counts automations with no published audited actions.

  _Use it to confirm quiet-hours coverage after a publish or when compliance asks which steps can fire after hours._

  ```bash
  conduyt-crm-pp-cli automations hours-audit --published-only --json
  ```

### Dialer operations
- **`dialer coverage`** — Priorities in dial order with queue depth per priority and the count of available agents; empty priorities are listed under empty_priorities with a warning (exit 0).

  _Run it before the floor starts to see what Smart Dialing will hand out next and where a priority has run dry._

  ```bash
  conduyt-crm-pp-cli dialer coverage --limit 25 --agent
  ```

## Command Reference

**account** — Manage account

- `conduyt-crm-pp-cli account` — List account readiness

**account-exports** — Manage account exports

- `conduyt-crm-pp-cli account-exports` — List account-exports

**accounts** — Manage accounts

- `conduyt-crm-pp-cli accounts get` — List accounts
- `conduyt-crm-pp-cli accounts post` — Create / invoke accounts

**activities** — Activity feed and logging

- `conduyt-crm-pp-cli activities create-activity` — Log an activity
- `conduyt-crm-pp-cli activities get-id` — Get activities id
- `conduyt-crm-pp-cli activities list` — List activities

**admin** — Super-admin account management and impersonation

- `conduyt-crm-pp-cli admin clean-test-accounts` — Delete test accounts
- `conduyt-crm-pp-cli admin clean-test-data` — Clean test data from the system
- `conduyt-crm-pp-cli admin impersonate` — Impersonate a user (super-admin)
- `conduyt-crm-pp-cli admin list-accounts` — List all accounts (super-admin)
- `conduyt-crm-pp-cli admin stop-impersonate` — Stop impersonating
- `conduyt-crm-pp-cli admin toggle-comp` — Toggle comp (free) status for an account

**ai** — AI-powered features (chat, email compose, contact enrichment)

- `conduyt-crm-pp-cli ai chat` — AI chat assistant
- `conduyt-crm-pp-cli ai compose-email` — AI-assisted email composition
- `conduyt-crm-pp-cli ai deal-insights` — AI-generated deal insights and recommendations
- `conduyt-crm-pp-cli ai enrich-contact` — AI-powered contact data enrichment
- `conduyt-crm-pp-cli ai get-actions-runs-id` — Get ai actions runs id
- `conduyt-crm-pp-cli ai get-agenda` — List ai agenda
- `conduyt-crm-pp-cli ai get-daily-brief` — List ai daily-brief
- `conduyt-crm-pp-cli ai get-feed` — List ai feed
- `conduyt-crm-pp-cli ai get-insights` — List ai insights
- `conduyt-crm-pp-cli ai get-next-actions` — List ai next-actions
- `conduyt-crm-pp-cli ai get-usage` — List ai usage
- `conduyt-crm-pp-cli ai improve-email` — AI-assisted email improvement
- `conduyt-crm-pp-cli ai post-actions` — Create / invoke ai actions
- `conduyt-crm-pp-cli ai post-actions-execute` — Create / invoke ai actions execute
- `conduyt-crm-pp-cli ai post-conversation-intelligence` — Create / invoke ai conversation-intelligence
- `conduyt-crm-pp-cli ai post-insights` — Create / invoke ai insights
- `conduyt-crm-pp-cli ai post-tasks` — Create / invoke ai tasks
- `conduyt-crm-pp-cli ai summarize-contact` — AI-generated contact summary

**api-keys** — API key management

- `conduyt-crm-pp-cli api-keys check` — Returns tier, scopes, rate limit, IP allowlist, and usage for the API key used on the request.
- `conduyt-crm-pp-cli api-keys create` — Returns the full key exactly once. Store it securely.
- `conduyt-crm-pp-cli api-keys get-id` — Get api-keys id
- `conduyt-crm-pp-cli api-keys list` — Returns API key metadata. Never returns the full key.
- `conduyt-crm-pp-cli api-keys revoke` — Revoke an API key
- `conduyt-crm-pp-cli api-keys update` — Update key name, tier, scopes, IP allowlist, rate limit, and optional expiry. API-key auth cannot manage API keys.

**api_search** — Manage api search

- `conduyt-crm-pp-cli api-search` — Global search across contacts, companies, and deals

**appointments** — Appointment scheduling

- `conduyt-crm-pp-cli appointments create` — Create an appointment
- `conduyt-crm-pp-cli appointments delete` — Delete an appointment
- `conduyt-crm-pp-cli appointments get` — Get an appointment by ID
- `conduyt-crm-pp-cli appointments list` — List all appointments
- `conduyt-crm-pp-cli appointments update` — Update an appointment

**automation-executions** — Automation execution logs and step details

- `conduyt-crm-pp-cli automation-executions get` — Get execution details by ID
- `conduyt-crm-pp-cli automation-executions list` — List automation execution logs

**automation-folders** — Manage automation folders

- `conduyt-crm-pp-cli automation-folders delete-id` — Delete automation-folders id
- `conduyt-crm-pp-cli automation-folders get` — List automation-folders
- `conduyt-crm-pp-cli automation-folders patch-id` — Update automation-folders id
- `conduyt-crm-pp-cli automation-folders post` — Create / invoke automation-folders

**automations** — Workflow automations (native + n8n), publishing, analytics

- `conduyt-crm-pp-cli automations create` — Create an automation
- `conduyt-crm-pp-cli automations create-from-template` — Create automation from a template
- `conduyt-crm-pp-cli automations delete` — Delete an automation
- `conduyt-crm-pp-cli automations get` — Get an automation by ID
- `conduyt-crm-pp-cli automations get-failures` — List automations failures
- `conduyt-crm-pp-cli automations get-failures-drilldown` — List automations failures drilldown
- `conduyt-crm-pp-cli automations get-resolve` — List automations resolve
- `conduyt-crm-pp-cli automations get-schema` — List automations schema
- `conduyt-crm-pp-cli automations get-step-logs` — List automations step-logs
- `conduyt-crm-pp-cli automations list` — List automations
- `conduyt-crm-pp-cli automations list-actions` — List available automation actions
- `conduyt-crm-pp-cli automations list-condition-fields` — List available condition fields for triggers
- `conduyt-crm-pp-cli automations list-events` — List available trigger events
- `conduyt-crm-pp-cli automations list-templates` — List automation templates
- `conduyt-crm-pp-cli automations post-copilot` — Create / invoke automations copilot
- `conduyt-crm-pp-cli automations post-import` — Create / invoke automations import
- `conduyt-crm-pp-cli automations post-resolve` — Create / invoke automations resolve
- `conduyt-crm-pp-cli automations post-validate` — Create / invoke automations validate
- `conduyt-crm-pp-cli automations test-webhook` — Send a test payload to an automation's webhook URL
- `conduyt-crm-pp-cli automations update` — Update an automation

**availability** — Manage availability

- `conduyt-crm-pp-cli availability get` — Get current user's availability rules
- `conduyt-crm-pp-cli availability set` — Set availability rules

**batch-operations** — Manage batch operations

- `conduyt-crm-pp-cli batch-operations get` — List batch-operations
- `conduyt-crm-pp-cli batch-operations get-id` — Get batch-operations id
- `conduyt-crm-pp-cli batch-operations post` — Create / invoke batch-operations

**billing** — Stripe billing, checkout, and subscription status

- `conduyt-crm-pp-cli billing create-checkout-session` — Create a Stripe checkout session
- `conduyt-crm-pp-cli billing create-portal` — Create a Stripe billing portal session
- `conduyt-crm-pp-cli billing delete-addons` — Delete billing addons
- `conduyt-crm-pp-cli billing get-addons` — List billing addons
- `conduyt-crm-pp-cli billing get-gate` — List billing gate
- `conduyt-crm-pp-cli billing get-status` — Get subscription status
- `conduyt-crm-pp-cli billing patch-addons` — Update billing addons
- `conduyt-crm-pp-cli billing post-addons` — Create / invoke billing addons
- `conduyt-crm-pp-cli billing webhook-stripe` — Stripe billing webhook

**booking-pages** — Public booking pages (Calendly-style)

- `conduyt-crm-pp-cli booking-pages create` — Create a booking page
- `conduyt-crm-pp-cli booking-pages delete` — Delete a booking page
- `conduyt-crm-pp-cli booking-pages get` — Get a booking page by ID
- `conduyt-crm-pp-cli booking-pages list` — List booking pages
- `conduyt-crm-pp-cli booking-pages update` — Update a booking page

**booking-routing-forms** — Manage booking routing forms

- `conduyt-crm-pp-cli booking-routing-forms delete-id` — Delete booking-routing-forms id
- `conduyt-crm-pp-cli booking-routing-forms get` — List booking-routing-forms
- `conduyt-crm-pp-cli booking-routing-forms get-id` — Get booking-routing-forms id
- `conduyt-crm-pp-cli booking-routing-forms patch-id` — Update booking-routing-forms id
- `conduyt-crm-pp-cli booking-routing-forms post` — Create / invoke booking-routing-forms

**bulk** — Manage bulk

- `conduyt-crm-pp-cli bulk delete-contacts` — Bulk delete contacts
- `conduyt-crm-pp-cli bulk edit-contacts` — Bulk edit contact fields
- `conduyt-crm-pp-cli bulk edit-deals` — Bulk edit deal fields
- `conduyt-crm-pp-cli bulk get-status` — Get bulk operation status
- `conduyt-crm-pp-cli bulk post-contacts-dnc` — Create / invoke bulk contacts dnc
- `conduyt-crm-pp-cli bulk post-contacts-untag` — Create / invoke bulk contacts untag
- `conduyt-crm-pp-cli bulk tag-contacts` — Bulk add/remove tags on contacts
- `conduyt-crm-pp-cli bulk update-contacts` — Bulk update contacts with field values
- `conduyt-crm-pp-cli bulk update-deals` — Bulk update deals

**bulk-operations** — Bulk update, delete, and tag contacts and deals

- `conduyt-crm-pp-cli bulk-operations` — List bulk-operations unified

**calendar** — Internal calendar and appointment management

- `conduyt-crm-pp-cli calendar connect-google` — Initiate Google Calendar OAuth
- `conduyt-crm-pp-cli calendar connect-microsoft` — Initiate Microsoft Calendar OAuth
- `conduyt-crm-pp-cli calendar create-event` — Create an event on a connected calendar
- `conduyt-crm-pp-cli calendar delete-connection` — Disconnect a calendar
- `conduyt-crm-pp-cli calendar delete-event` — Delete a synced calendar event
- `conduyt-crm-pp-cli calendar get-connection` — Get a calendar connection by ID
- `conduyt-crm-pp-cli calendar get-event` — Get a synced calendar event
- `conduyt-crm-pp-cli calendar get-health` — List calendar health
- `conduyt-crm-pp-cli calendar google-callback` — Google Calendar OAuth callback
- `conduyt-crm-pp-cli calendar list-connections` — List calendar connections
- `conduyt-crm-pp-cli calendar list-events` — List synced calendar events
- `conduyt-crm-pp-cli calendar microsoft-callback` — Microsoft Calendar OAuth callback
- `conduyt-crm-pp-cli calendar sync` — Trigger manual calendar sync
- `conduyt-crm-pp-cli calendar update-event` — Update a synced calendar event
- `conduyt-crm-pp-cli calendar webhook-google` — Google Calendar push notification webhook
- `conduyt-crm-pp-cli calendar webhook-microsoft` — Microsoft Calendar webhook

**calendars** — Internal calendar and appointment management

- `conduyt-crm-pp-cli calendars create` — Create a calendar
- `conduyt-crm-pp-cli calendars get` — Get a calendar by ID
- `conduyt-crm-pp-cli calendars list` — List internal calendars
- `conduyt-crm-pp-cli calendars update` — Update a calendar

**call-flows** — Manage call flows

- `conduyt-crm-pp-cli call-flows delete-functions-id` — Delete call-flows functions id
- `conduyt-crm-pp-cli call-flows delete-id` — Delete call-flows id
- `conduyt-crm-pp-cli call-flows get` — List call-flows
- `conduyt-crm-pp-cli call-flows get-functions` — List call-flows functions
- `conduyt-crm-pp-cli call-flows get-functions-id` — Get call-flows functions id
- `conduyt-crm-pp-cli call-flows get-id` — Get call-flows id
- `conduyt-crm-pp-cli call-flows get-numbers` — List call-flows numbers
- `conduyt-crm-pp-cli call-flows get-roster` — List call-flows roster
- `conduyt-crm-pp-cli call-flows patch-functions-id` — Update call-flows functions id
- `conduyt-crm-pp-cli call-flows patch-id` — Update call-flows id
- `conduyt-crm-pp-cli call-flows post` — Create / invoke call-flows
- `conduyt-crm-pp-cli call-flows post-audio` — Create / invoke call-flows audio
- `conduyt-crm-pp-cli call-flows post-functions` — Create / invoke call-flows functions
- `conduyt-crm-pp-cli call-flows post-functions-id-test` — Create / invoke call-flows functions id test
- `conduyt-crm-pp-cli call-flows post-tombstones-release` — Create / invoke call-flows tombstones release

**calls** — Call log management

- `conduyt-crm-pp-cli calls create-record` — Create a call record
- `conduyt-crm-pp-cli calls get` — Get a call by ID
- `conduyt-crm-pp-cli calls list` — List call records
- `conduyt-crm-pp-cli calls post-voice-drop` — Create / invoke calls voice-drop
- `conduyt-crm-pp-cli calls update` — Update a call record (e.g., add notes)

**chat** — Internal team chat channels and messages

- `conduyt-crm-pp-cli chat add-member` — Add a member to a channel
- `conduyt-crm-pp-cli chat add-reaction` — Add a reaction to a message
- `conduyt-crm-pp-cli chat create-channel` — Create a chat channel
- `conduyt-crm-pp-cli chat delete-message` — Delete a chat message
- `conduyt-crm-pp-cli chat edit-message` — Edit a chat message
- `conduyt-crm-pp-cli chat get-channels-id-upload` — Get chat channels id upload
- `conduyt-crm-pp-cli chat get-message` — Get a chat message by ID
- `conduyt-crm-pp-cli chat get-typing-status` — Get who is currently typing
- `conduyt-crm-pp-cli chat list-channels` — List chat channels
- `conduyt-crm-pp-cli chat list-messages` — List messages in a channel
- `conduyt-crm-pp-cli chat remove-member` — Remove a member from a channel
- `conduyt-crm-pp-cli chat remove-reaction` — Remove a reaction from a message
- `conduyt-crm-pp-cli chat send-message` — Send a message in a channel
- `conduyt-crm-pp-cli chat send-typing-indicator` — Send a typing indicator
- `conduyt-crm-pp-cli chat upload-file` — Upload a file to a channel

**companies** — Company (organization) management

- `conduyt-crm-pp-cli companies create-company` — Create a company
- `conduyt-crm-pp-cli companies delete-company` — Soft-delete a company
- `conduyt-crm-pp-cli companies get-company` — Get a company by ID
- `conduyt-crm-pp-cli companies get-duplicates` — List companies duplicates
- `conduyt-crm-pp-cli companies get-export` — List companies export
- `conduyt-crm-pp-cli companies get-reporting` — List companies reporting
- `conduyt-crm-pp-cli companies list` — List companies
- `conduyt-crm-pp-cli companies post-merge` — Create / invoke companies merge
- `conduyt-crm-pp-cli companies update-company` — Update a company

**conduyt-auth** — Manage conduyt auth

- `conduyt-crm-pp-cli conduyt-auth accept-invite` — Accept a team invitation
- `conduyt-crm-pp-cli conduyt-auth change-password` — Change password (authenticated)
- `conduyt-crm-pp-cli conduyt-auth delete-sessions` — Delete auth sessions
- `conduyt-crm-pp-cli conduyt-auth forgot-password` — Request a password reset email
- `conduyt-crm-pp-cli conduyt-auth get-login-history` — List auth login-history
- `conduyt-crm-pp-cli conduyt-auth get-me` — Get current authenticated user
- `conduyt-crm-pp-cli conduyt-auth get-mfa-setup` — List auth mfa setup
- `conduyt-crm-pp-cli conduyt-auth get-saml-account-slug-metadata` — Get auth saml accountSlug metadata
- `conduyt-crm-pp-cli conduyt-auth get-sandbox-key` — List auth sandbox-key
- `conduyt-crm-pp-cli conduyt-auth get-security-center` — List auth security-center
- `conduyt-crm-pp-cli conduyt-auth get-sessions` — List auth sessions
- `conduyt-crm-pp-cli conduyt-auth login` — Authenticates user credentials and returns a session cookie. Rate limited to 5 requests per 15 minutes per IP.
- `conduyt-crm-pp-cli conduyt-auth logout` — Log out (destroy session)
- `conduyt-crm-pp-cli conduyt-auth patch-security-center` — Update auth security-center
- `conduyt-crm-pp-cli conduyt-auth post-mfa-challenge` — Create / invoke auth mfa challenge
- `conduyt-crm-pp-cli conduyt-auth post-mfa-disable` — Create / invoke auth mfa disable
- `conduyt-crm-pp-cli conduyt-auth post-mfa-setup` — Create / invoke auth mfa setup
- `conduyt-crm-pp-cli conduyt-auth post-mfa-verify` — Create / invoke auth mfa verify
- `conduyt-crm-pp-cli conduyt-auth post-sandbox-key` — Create / invoke auth sandbox-key
- `conduyt-crm-pp-cli conduyt-auth register` — Creates a new user and account. Rate limited to 3 requests per hour per IP.
- `conduyt-crm-pp-cli conduyt-auth reset-password` — Reset password with token
- `conduyt-crm-pp-cli conduyt-auth switch-account` — Switch to a different account

**confirm** — Manage confirm

- `conduyt-crm-pp-cli confirm post` — Create / invoke confirm
- `conduyt-crm-pp-cli confirm put` — Update confirm

**contact** — Contact management, tagging, scoring, import/export, merge, duplicates

- `conduyt-crm-pp-cli contact` — Creates or updates a contact by email or phone match. Designed for inbound webhook integrations.

**contacts** — Contact management, tagging, scoring, import/export, merge, duplicates

- `conduyt-crm-pp-cli contacts create` — Creates a new contact. Rate limited to 30 requests per minute.
- `conduyt-crm-pp-cli contacts delete` — Soft-delete a contact
- `conduyt-crm-pp-cli contacts export` — Export contacts as CSV
- `conduyt-crm-pp-cli contacts find-duplicate` — Find duplicate contacts
- `conduyt-crm-pp-cli contacts get` — Get a contact by ID
- `conduyt-crm-pp-cli contacts get-dnc-status` — List contacts dnc-status
- `conduyt-crm-pp-cli contacts get-geo` — List contacts geo
- `conduyt-crm-pp-cli contacts get-import-template` — Download CSV import template
- `conduyt-crm-pp-cli contacts get-sources` — List contacts sources
- `conduyt-crm-pp-cli contacts import` — Import contacts from CSV
- `conduyt-crm-pp-cli contacts list` — Returns a paginated list of contacts.
- `conduyt-crm-pp-cli contacts merge` — Merge two contacts
- `conduyt-crm-pp-cli contacts post-ai-import` — Create / invoke contacts ai-import
- `conduyt-crm-pp-cli contacts post-dry-run` — Create / invoke contacts dry-run
- `conduyt-crm-pp-cli contacts post-verify-line-type` — Create / invoke contacts verify-line-type
- `conduyt-crm-pp-cli contacts update` — Update a contact

**conversations** — Threaded conversation view per contact

- `conduyt-crm-pp-cli conversations get` — Get conversation thread for a contact
- `conduyt-crm-pp-cli conversations get-export` — List conversations export
- `conduyt-crm-pp-cli conversations get-metrics` — List conversations metrics
- `conduyt-crm-pp-cli conversations list` — List conversation threads
- `conduyt-crm-pp-cli conversations patch-contact-id` — Update conversations contactId

**custom-fields** — Custom field definitions for contacts and deals

- `conduyt-crm-pp-cli custom-fields create` — Create a custom field definition
- `conduyt-crm-pp-cli custom-fields delete` — Delete a custom field definition
- `conduyt-crm-pp-cli custom-fields get-id` — Get custom-fields id
- `conduyt-crm-pp-cli custom-fields list` — List custom field definitions
- `conduyt-crm-pp-cli custom-fields update` — Update a custom field definition

**dashboard** — Dashboard summary metrics

- `conduyt-crm-pp-cli dashboard` — Get dashboard summary metrics

**dashboards** — Dashboard summary metrics

- `conduyt-crm-pp-cli dashboards delete-id` — Delete dashboards id
- `conduyt-crm-pp-cli dashboards get` — List dashboards
- `conduyt-crm-pp-cli dashboards get-id` — Get dashboards id
- `conduyt-crm-pp-cli dashboards patch-id` — Update dashboards id
- `conduyt-crm-pp-cli dashboards post` — Create / invoke dashboards

**data-model** — Manage data model

- `conduyt-crm-pp-cli data-model` — List data-model quality

**deals** — Deal/opportunity management within pipelines

- `conduyt-crm-pp-cli deals create` — Creates a new deal in a pipeline stage. Rate limited to 30 requests per minute.
- `conduyt-crm-pp-cli deals delete` — Soft-delete a deal
- `conduyt-crm-pp-cli deals delete-views-id` — Delete deals views id
- `conduyt-crm-pp-cli deals get` — Get a deal by ID
- `conduyt-crm-pp-cli deals get-export` — List deals export
- `conduyt-crm-pp-cli deals get-forecast` — List deals forecast
- `conduyt-crm-pp-cli deals get-inspection` — List deals inspection
- `conduyt-crm-pp-cli deals get-views` — List deals views
- `conduyt-crm-pp-cli deals get-views-id` — Get deals views id
- `conduyt-crm-pp-cli deals list` — Returns deals with Kanban-optimized sort order (stage, sortOrder, then requested sort).
- `conduyt-crm-pp-cli deals patch-views-id` — Update deals views id
- `conduyt-crm-pp-cli deals post-probabilities` — Create / invoke deals probabilities
- `conduyt-crm-pp-cli deals post-views` — Create / invoke deals views
- `conduyt-crm-pp-cli deals update` — Update a deal

**dialer** — Click-to-call dialer via Twilio

- `conduyt-crm-pp-cli dialer delete-call-id-participants-participant-sid` — Delete dialer call id participants participantSid
- `conduyt-crm-pp-cli dialer delete-call-id-supervise` — Delete dialer call id supervise
- `conduyt-crm-pp-cli dialer get-agents-status` — List dialer agents-status
- `conduyt-crm-pp-cli dialer get-call-id-media` — Get dialer call id media
- `conduyt-crm-pp-cli dialer get-call-id-participants` — Get dialer call id participants
- `conduyt-crm-pp-cli dialer get-call-id-post-call` — Get dialer call id post-call
- `conduyt-crm-pp-cli dialer get-calls` — List dialer calls
- `conduyt-crm-pp-cli dialer get-calls-hourly` — List dialer calls hourly
- `conduyt-crm-pp-cli dialer get-capabilities` — List dialer capabilities
- `conduyt-crm-pp-cli dialer get-dispositions` — List dialer dispositions
- `conduyt-crm-pp-cli dialer get-health` — List dialer health
- `conduyt-crm-pp-cli dialer get-history` — Get recent call history
- `conduyt-crm-pp-cli dialer get-inbound-lookup` — List dialer inbound-lookup
- `conduyt-crm-pp-cli dialer get-intelligence` — List dialer intelligence
- `conduyt-crm-pp-cli dialer get-leaderboard` — List dialer leaderboard
- `conduyt-crm-pp-cli dialer get-live-monitor` — List dialer live-monitor
- `conduyt-crm-pp-cli dialer get-local-presence-resolve` — List dialer local-presence resolve
- `conduyt-crm-pp-cli dialer get-queue` — List dialer queue
- `conduyt-crm-pp-cli dialer get-queue-open` — List dialer queue open
- `conduyt-crm-pp-cli dialer get-recordings` — List dialer recordings
- `conduyt-crm-pp-cli dialer get-settings` — List dialer settings
- `conduyt-crm-pp-cli dialer get-setup` — List dialer setup
- `conduyt-crm-pp-cli dialer get-stats` — List dialer stats
- `conduyt-crm-pp-cli dialer get-status-log` — List dialer status-log
- `conduyt-crm-pp-cli dialer get-status-log-team` — List dialer status-log team
- `conduyt-crm-pp-cli dialer get-token` — Get a Twilio browser token for click-to-call
- `conduyt-crm-pp-cli dialer initiate-call` — Initiate an outbound call
- `conduyt-crm-pp-cli dialer patch-call-id` — Update dialer call id
- `conduyt-crm-pp-cli dialer patch-call-id-participants-participant-sid` — Update dialer call id participants participantSid
- `conduyt-crm-pp-cli dialer patch-settings` — Update dialer settings
- `conduyt-crm-pp-cli dialer post-call-eligibility` — Create / invoke dialer call eligibility
- `conduyt-crm-pp-cli dialer post-call-id-ai-disposition` — Create / invoke dialer call id ai-disposition
- `conduyt-crm-pp-cli dialer post-call-id-facetime-status` — Create / invoke dialer call id facetime-status
- `conduyt-crm-pp-cli dialer post-call-id-hangup` — Create / invoke dialer call id hangup
- `conduyt-crm-pp-cli dialer post-call-id-hold` — Create / invoke dialer call id hold
- `conduyt-crm-pp-cli dialer post-call-id-mute` — Create / invoke dialer call id mute
- `conduyt-crm-pp-cli dialer post-call-id-participants` — Create / invoke dialer call id participants
- `conduyt-crm-pp-cli dialer post-call-id-record` — Create / invoke dialer call id record
- `conduyt-crm-pp-cli dialer post-call-id-supervise` — Create / invoke dialer call id supervise
- `conduyt-crm-pp-cli dialer post-call-id-transfer` — Create / invoke dialer call id transfer
- `conduyt-crm-pp-cli dialer post-call-id-vm-drop` — Create / invoke dialer call id vm-drop
- `conduyt-crm-pp-cli dialer post-heartbeat` — Create / invoke dialer heartbeat
- `conduyt-crm-pp-cli dialer post-local-presence-sync` — Create / invoke dialer local-presence sync
- `conduyt-crm-pp-cli dialer post-mirror-token` — Create / invoke dialer mirror-token
- `conduyt-crm-pp-cli dialer post-queue-next` — Create / invoke dialer queue next
- `conduyt-crm-pp-cli dialer post-queue-release` — Create / invoke dialer queue release
- `conduyt-crm-pp-cli dialer post-queue-skip` — Create / invoke dialer queue skip
- `conduyt-crm-pp-cli dialer post-queue-worked` — Create / invoke dialer queue worked
- `conduyt-crm-pp-cli dialer post-setup-wire-inbound` — Create / invoke dialer setup wire-inbound
- `conduyt-crm-pp-cli dialer post-status-log` — Create / invoke dialer status-log
- `conduyt-crm-pp-cli dialer post-status-log-rollup` — Create / invoke dialer status-log rollup

**dnc** — Manage dnc

- `conduyt-crm-pp-cli dnc delete-id` — Delete dnc id
- `conduyt-crm-pp-cli dnc get` — List dnc
- `conduyt-crm-pp-cli dnc post` — Create / invoke dnc
- `conduyt-crm-pp-cli dnc post-import` — Create / invoke dnc import

**document-templates** — Proposal and contract templates with merge fields

- `conduyt-crm-pp-cli document-templates create` — Create a document template
- `conduyt-crm-pp-cli document-templates delete` — Delete a document template
- `conduyt-crm-pp-cli document-templates get` — Get a document template by ID
- `conduyt-crm-pp-cli document-templates list` — List document templates
- `conduyt-crm-pp-cli document-templates update` — Update a document template

**drip-campaigns** — SMS drip campaign engine

- `conduyt-crm-pp-cli drip-campaigns create` — Create a drip campaign
- `conduyt-crm-pp-cli drip-campaigns delete` — Delete a drip campaign
- `conduyt-crm-pp-cli drip-campaigns get-id` — Get drip-campaigns id
- `conduyt-crm-pp-cli drip-campaigns list` — List SMS drip campaigns
- `conduyt-crm-pp-cli drip-campaigns seed` — Seed default drip campaigns
- `conduyt-crm-pp-cli drip-campaigns update` — Update a drip campaign

**drip-enrollments** — Manage drip enrollments

- `conduyt-crm-pp-cli drip-enrollments list` — List drip enrollments
- `conduyt-crm-pp-cli drip-enrollments post-batch-pause` — Create / invoke drip-enrollments batch pause
- `conduyt-crm-pp-cli drip-enrollments post-batch-resume` — Create / invoke drip-enrollments batch resume
- `conduyt-crm-pp-cli drip-enrollments post-batch-stop` — Create / invoke drip-enrollments batch stop

**drip-tracks** — Manage drip tracks

- `conduyt-crm-pp-cli drip-tracks delete-id` — Delete drip-tracks id
- `conduyt-crm-pp-cli drip-tracks get` — List drip-tracks
- `conduyt-crm-pp-cli drip-tracks get-id` — Get drip-tracks id
- `conduyt-crm-pp-cli drip-tracks patch-id` — Update drip-tracks id
- `conduyt-crm-pp-cli drip-tracks post` — Create / invoke drip-tracks
- `conduyt-crm-pp-cli drip-tracks post-import-steps` — Create / invoke drip-tracks import-steps

**email** — Send individual and bulk emails

- `conduyt-crm-pp-cli email send` — Send an email to a contact
- `conduyt-crm-pp-cli email send-bulk` — Send bulk emails

**email-domains** — Custom email domain verification (Resend)

- `conduyt-crm-pp-cli email-domains add` — Add a custom email domain
- `conduyt-crm-pp-cli email-domains delete-reply-capture` — Delete email-domains reply-capture
- `conduyt-crm-pp-cli email-domains get` — Get email domain configuration
- `conduyt-crm-pp-cli email-domains get-reply-capture` — List email-domains reply-capture
- `conduyt-crm-pp-cli email-domains get-status` — List email-domains status
- `conduyt-crm-pp-cli email-domains post-reply-capture` — Create / invoke email-domains reply-capture
- `conduyt-crm-pp-cli email-domains post-reply-capture-verify` — Create / invoke email-domains reply-capture verify
- `conduyt-crm-pp-cli email-domains remove` — Remove email domain
- `conduyt-crm-pp-cli email-domains update` — Update email domain settings
- `conduyt-crm-pp-cli email-domains verify` — Verify DNS configuration for email domain

**emails** — Send individual and bulk emails

- `conduyt-crm-pp-cli emails create-sequence` — Create an email sequence
- `conduyt-crm-pp-cli emails create-template` — Create an email template
- `conduyt-crm-pp-cli emails delete-template` — Delete an email template
- `conduyt-crm-pp-cli emails enroll-in-sequence` — Enroll contacts in a sequence
- `conduyt-crm-pp-cli emails get-health` — List emails health
- `conduyt-crm-pp-cli emails get-sequence` — Get an email sequence by ID
- `conduyt-crm-pp-cli emails get-sequence-stats` — Get sequence performance stats
- `conduyt-crm-pp-cli emails get-template` — Get an email template by ID
- `conduyt-crm-pp-cli emails get-templates-id-usage` — Get emails templates id usage
- `conduyt-crm-pp-cli emails list` — List email messages
- `conduyt-crm-pp-cli emails list-sequence-enrollments` — List enrollments for a sequence
- `conduyt-crm-pp-cli emails list-sequences` — List email sequences
- `conduyt-crm-pp-cli emails list-templates` — List email templates
- `conduyt-crm-pp-cli emails post-sequences-id-preflight` — Create / invoke emails sequences id preflight
- `conduyt-crm-pp-cli emails test-send-template` — Send a test email from a template
- `conduyt-crm-pp-cli emails unenroll-from-sequence` — Unenroll contacts from a sequence
- `conduyt-crm-pp-cli emails update-sequence` — Update an email sequence
- `conduyt-crm-pp-cli emails update-template` — Update an email template

**errors** — Manage errors

- `conduyt-crm-pp-cli errors` — Create / invoke errors log

**files** — File uploads and attachments

- `conduyt-crm-pp-cli files create-record` — Create a file attachment record
- `conduyt-crm-pp-cli files delete` — Delete a file attachment
- `conduyt-crm-pp-cli files list` — List file attachments
- `conduyt-crm-pp-cli files upload` — Upload a file

**forms** — Lead capture forms and submissions

- `conduyt-crm-pp-cli forms create` — Create a form
- `conduyt-crm-pp-cli forms delete` — Archive a form
- `conduyt-crm-pp-cli forms get` — Get a form by ID
- `conduyt-crm-pp-cli forms list` — List forms
- `conduyt-crm-pp-cli forms update` — Update a form

**generated-documents** — Manage generated documents

- `conduyt-crm-pp-cli generated-documents delete-id` — Delete generated-documents id
- `conduyt-crm-pp-cli generated-documents get` — List generated-documents
- `conduyt-crm-pp-cli generated-documents get-id` — Get generated-documents id
- `conduyt-crm-pp-cli generated-documents patch-id` — Update generated-documents id
- `conduyt-crm-pp-cli generated-documents post` — Create / invoke generated-documents

**groups** — Manage groups

- `conduyt-crm-pp-cli groups delete-id` — Delete groups id
- `conduyt-crm-pp-cli groups get` — List groups
- `conduyt-crm-pp-cli groups patch-id` — Update groups id
- `conduyt-crm-pp-cli groups post` — Create / invoke groups

**health** — Manage health

- `conduyt-crm-pp-cli health` — List health

**imports** — CSV import jobs with mapping and deduplication

- `conduyt-crm-pp-cli imports create` — Create an import job
- `conduyt-crm-pp-cli imports get` — Get import job status
- `conduyt-crm-pp-cli imports get-users` — List imports users
- `conduyt-crm-pp-cli imports list` — List import jobs
- `conduyt-crm-pp-cli imports post-preflight` — Create / invoke imports preflight
- `conduyt-crm-pp-cli imports post-upload-url` — Create / invoke imports upload-url
- `conduyt-crm-pp-cli imports upload-file` — Upload a CSV file for import

**integrations** — Third-party integrations (Zapier, etc.)

- `conduyt-crm-pp-cli integrations connect` — Connect an integration
- `conduyt-crm-pp-cli integrations create-zapier-subscription` — Create a Zapier webhook subscription
- `conduyt-crm-pp-cli integrations delete-zapier-subscription` — Delete a Zapier subscription
- `conduyt-crm-pp-cli integrations disconnect` — Disconnect an integration
- `conduyt-crm-pp-cli integrations get-health` — List integrations health
- `conduyt-crm-pp-cli integrations get-slack-channels` — List integrations slack channels
- `conduyt-crm-pp-cli integrations get-zapier-sample-data` — Get sample data for a Zapier event
- `conduyt-crm-pp-cli integrations list` — List active integrations
- `conduyt-crm-pp-cli integrations list-zapier-subscriptions` — List Zapier webhook subscriptions
- `conduyt-crm-pp-cli integrations post-dpp-ingest` — Create / invoke integrations dpp ingest
- `conduyt-crm-pp-cli integrations post-slack-settings` — Create / invoke integrations slack settings
- `conduyt-crm-pp-cli integrations post-slack-test` — Create / invoke integrations slack test

**invoices** — Invoice creation, sending, payments, PDF generation

- `conduyt-crm-pp-cli invoices create` — Create an invoice
- `conduyt-crm-pp-cli invoices delete` — Delete an invoice
- `conduyt-crm-pp-cli invoices get` — Get an invoice by ID
- `conduyt-crm-pp-cli invoices get-next-number` — Get the next auto-incremented invoice number
- `conduyt-crm-pp-cli invoices list` — List invoices
- `conduyt-crm-pp-cli invoices update` — Update an invoice

**knowledge** — Manage knowledge

- `conduyt-crm-pp-cli knowledge delete-sources-id` — Delete knowledge sources id
- `conduyt-crm-pp-cli knowledge get-sources` — List knowledge sources
- `conduyt-crm-pp-cli knowledge post-sources` — Create / invoke knowledge sources
- `conduyt-crm-pp-cli knowledge post-sources-id-reindex` — Create / invoke knowledge sources id reindex

**lead-pool** — Manage lead pool

- `conduyt-crm-pp-cli lead-pool` — List lead-pool

**lead-routing** — Lead assignment routing rules and assignment history

- `conduyt-crm-pp-cli lead-routing create-rule` — Create a lead routing rule
- `conduyt-crm-pp-cli lead-routing delete-rule` — Delete a lead routing rule
- `conduyt-crm-pp-cli lead-routing get-rule` — Get a lead routing rule
- `conduyt-crm-pp-cli lead-routing list-lead-assignment-log` — List lead assignment history
- `conduyt-crm-pp-cli lead-routing list-rules` — List lead routing rules
- `conduyt-crm-pp-cli lead-routing reorder-rules` — Reorder lead routing rules
- `conduyt-crm-pp-cli lead-routing test` — Simulate lead routing
- `conduyt-crm-pp-cli lead-routing update-rule` — Update a lead routing rule

**mailbox** — Manage mailbox

- `conduyt-crm-pp-cli mailbox delete-connections-id` — Delete mailbox connections id
- `conduyt-crm-pp-cli mailbox get-connections` — List mailbox connections
- `conduyt-crm-pp-cli mailbox post-connect-microsoft` — Create / invoke mailbox connect microsoft

**messages** — SMS and email message history

- `conduyt-crm-pp-cli messages create` — Create a message record
- `conduyt-crm-pp-cli messages get-sms` — Get an SMS message by ID
- `conduyt-crm-pp-cli messages list` — List messages
- `conduyt-crm-pp-cli messages send-sms` — Send an SMS message

**notes** — Notes attached to contacts or deals

- `conduyt-crm-pp-cli notes create` — Body is capped at 50 KB. Returns 413 if exceeded.
- `conduyt-crm-pp-cli notes delete` — Delete a note
- `conduyt-crm-pp-cli notes get` — Get a note by ID
- `conduyt-crm-pp-cli notes list` — List notes
- `conduyt-crm-pp-cli notes update` — Update a note

**notifications** — In-app notifications

- `conduyt-crm-pp-cli notifications apply-profile` — Apply notification profile
- `conduyt-crm-pp-cli notifications create` — Create a notification
- `conduyt-crm-pp-cli notifications get-digest-settings` — Get notification digest settings
- `conduyt-crm-pp-cli notifications list` — List notifications
- `conduyt-crm-pp-cli notifications list-profiles` — List notification profiles
- `conduyt-crm-pp-cli notifications mark-all-read` — Mark all notifications as read
- `conduyt-crm-pp-cli notifications mark-read` — Mark a notification as read
- `conduyt-crm-pp-cli notifications replace-profiles` — Replace notification profiles
- `conduyt-crm-pp-cli notifications update-digest-settings` — Update notification digest settings

**phone-numbers** — Manage phone numbers

- `conduyt-crm-pp-cli phone-numbers` — List phone-numbers

**pipelines** — Sales pipeline and stage management

- `conduyt-crm-pp-cli pipelines create` — Requires owner or admin role. Subject to plan limits.
- `conduyt-crm-pp-cli pipelines delete` — Delete a pipeline
- `conduyt-crm-pp-cli pipelines get` — Get a pipeline by ID
- `conduyt-crm-pp-cli pipelines list` — List pipelines with stages
- `conduyt-crm-pp-cli pipelines update` — Update a pipeline

**playbook-enrollments** — Manage playbook enrollments

- `conduyt-crm-pp-cli playbook-enrollments get` — List playbook-enrollments
- `conduyt-crm-pp-cli playbook-enrollments get-id` — Get playbook-enrollments id
- `conduyt-crm-pp-cli playbook-enrollments post` — Create / invoke playbook-enrollments
- `conduyt-crm-pp-cli playbook-enrollments post-id` — Create / invoke playbook-enrollments id

**playbooks** — Manage playbooks

- `conduyt-crm-pp-cli playbooks delete-id` — Delete playbooks id
- `conduyt-crm-pp-cli playbooks get` — List playbooks
- `conduyt-crm-pp-cli playbooks get-id` — Get playbooks id
- `conduyt-crm-pp-cli playbooks patch-id` — Update playbooks id
- `conduyt-crm-pp-cli playbooks post` — Create / invoke playbooks

**products** — Product catalog for invoices

- `conduyt-crm-pp-cli products create` — Create a product
- `conduyt-crm-pp-cli products delete` — Delete a product
- `conduyt-crm-pp-cli products get` — Get a product by ID
- `conduyt-crm-pp-cli products list` — List products
- `conduyt-crm-pp-cli products update` — Update a product

**public** — Unauthenticated public endpoints (booking, form submit)

- `conduyt-crm-pp-cli public book-appointment` — Book an appointment via public page
- `conduyt-crm-pp-cli public get-booking-page` — Get a public booking page by slug
- `conduyt-crm-pp-cli public get-booking-routing-slug` — Get public booking routing slug
- `conduyt-crm-pp-cli public get-booking-slots` — Get available time slots for a booking page
- `conduyt-crm-pp-cli public get-booking-slug-embed` — Get public booking slug embed
- `conduyt-crm-pp-cli public get-booking-slug-embed-badge` — Get public booking slug embed badge
- `conduyt-crm-pp-cli public get-booking-slug-embed-popup` — Get public booking slug embed popup
- `conduyt-crm-pp-cli public get-booking-slug-frame-policy` — Get public booking slug frame-policy
- `conduyt-crm-pp-cli public get-booking-slug-ical` — Get public booking slug ical
- `conduyt-crm-pp-cli public get-branding-account-id-icon` — Get public branding accountId icon
- `conduyt-crm-pp-cli public get-screen-share-code` — Get public screen-share code
- `conduyt-crm-pp-cli public post-booking-routing-slug` — Create / invoke public booking routing slug
- `conduyt-crm-pp-cli public post-booking-slug-waitlist` — Create / invoke public booking slug waitlist
- `conduyt-crm-pp-cli public post-contact` — Create / invoke public contact
- `conduyt-crm-pp-cli public post-screen-share-code` — Create / invoke public screen-share code
- `conduyt-crm-pp-cli public post-subscribe` — Create / invoke public subscribe

**push** — Manage push

- `conduyt-crm-pp-cli push get-public-key` — Get VAPID public key for web push
- `conduyt-crm-pp-cli push subscribe` — Subscribe to web push notifications
- `conduyt-crm-pp-cli push unsubscribe` — Unsubscribe from web push

**quick-connects** — Manage quick connects

- `conduyt-crm-pp-cli quick-connects delete-id` — Delete quick-connects id
- `conduyt-crm-pp-cli quick-connects get` — List quick-connects
- `conduyt-crm-pp-cli quick-connects patch-id` — Update quick-connects id
- `conduyt-crm-pp-cli quick-connects post` — Create / invoke quick-connects

**quick-notes** — Manage quick notes

- `conduyt-crm-pp-cli quick-notes delete-id` — Delete quick-notes id
- `conduyt-crm-pp-cli quick-notes get` — List quick-notes
- `conduyt-crm-pp-cli quick-notes patch-id` — Update quick-notes id
- `conduyt-crm-pp-cli quick-notes post` — Create / invoke quick-notes

**remote-assist** — Manage remote assist

- `conduyt-crm-pp-cli remote-assist post-end` — Create / invoke remote-assist end
- `conduyt-crm-pp-cli remote-assist post-start` — Create / invoke remote-assist start

**rep-shifts** — Manage rep shifts

- `conduyt-crm-pp-cli rep-shifts delete-id` — Delete rep-shifts id
- `conduyt-crm-pp-cli rep-shifts get` — List rep-shifts
- `conduyt-crm-pp-cli rep-shifts patch-id` — Update rep-shifts id
- `conduyt-crm-pp-cli rep-shifts post` — Create / invoke rep-shifts

**reporting** — Manage reporting

- `conduyt-crm-pp-cli reporting get-email` — List reporting email
- `conduyt-crm-pp-cli reporting get-pipeline` — List reporting pipeline
- `conduyt-crm-pp-cli reporting get-sms` — List reporting sms

**reports** — Pipeline, revenue, activity, team, and custom reports

- `conduyt-crm-pp-cli reports create-custom` — Create a custom report
- `conduyt-crm-pp-cli reports delete-custom` — Delete a custom report
- `conduyt-crm-pp-cli reports delete-subscriptions-sid` — Delete reports subscriptions sid
- `conduyt-crm-pp-cli reports get-activity` — Activity report
- `conduyt-crm-pp-cli reports get-agent-performance` — List reports agent-performance
- `conduyt-crm-pp-cli reports get-appointments` — List reports appointments
- `conduyt-crm-pp-cli reports get-attribution` — List reports attribution
- `conduyt-crm-pp-cli reports get-call-performance` — List reports call-performance
- `conduyt-crm-pp-cli reports get-calls` — List reports calls
- `conduyt-crm-pp-cli reports get-custom` — Get a custom report by ID
- `conduyt-crm-pp-cli reports get-custom-fields` — List reports custom fields
- `conduyt-crm-pp-cli reports get-custom-id-subscriptions` — Get reports custom id subscriptions
- `conduyt-crm-pp-cli reports get-deliveries-did-files-index` — Get reports deliveries did files index
- `conduyt-crm-pp-cli reports get-dialer-agent-hourly` — List reports dialer agent-hourly
- `conduyt-crm-pp-cli reports get-dialer-awards` — List reports dialer awards
- `conduyt-crm-pp-cli reports get-dialer-dispositions` — List reports dialer dispositions
- `conduyt-crm-pp-cli reports get-dialer-list-performance` — List reports dialer list-performance
- `conduyt-crm-pp-cli reports get-email-providers` — List reports email-providers
- `conduyt-crm-pp-cli reports get-email-templates` — List reports email-templates
- `conduyt-crm-pp-cli reports get-funnel` — List reports funnel

  Omitting `--pipeline-id` selects the tenant's only pipeline. Multiple pipelines require an explicit ID; no pipelines returns not found, and missing pipeline scope returns an auth hint.
- `conduyt-crm-pp-cli reports get-pipeline` — Pipeline performance report
- `conduyt-crm-pp-cli reports get-revenue` — Revenue report
- `conduyt-crm-pp-cli reports get-sms-delivery` — List reports sms-delivery
- `conduyt-crm-pp-cli reports get-sms-drip-conversion` — List reports sms-drip-conversion
- `conduyt-crm-pp-cli reports get-sms-templates` — List reports sms-templates
- `conduyt-crm-pp-cli reports get-speed-to-lead` — List reports speed-to-lead
- `conduyt-crm-pp-cli reports get-subscriptions-sid` — Get reports subscriptions sid
- `conduyt-crm-pp-cli reports get-subscriptions-sid-deliveries` — Get reports subscriptions sid deliveries
- `conduyt-crm-pp-cli reports get-targets` — List reports targets
- `conduyt-crm-pp-cli reports get-team` — Team performance report
- `conduyt-crm-pp-cli reports list-custom` — List saved custom reports
- `conduyt-crm-pp-cli reports patch-subscriptions-sid` — Update reports subscriptions sid
- `conduyt-crm-pp-cli reports post-custom-id-export` — Create / invoke reports custom id export
- `conduyt-crm-pp-cli reports post-custom-id-subscriptions` — Create / invoke reports custom id subscriptions
- `conduyt-crm-pp-cli reports post-custom-preview` — Create / invoke reports custom preview
- `conduyt-crm-pp-cli reports post-subscriptions-sid-run` — Create / invoke reports subscriptions sid run
- `conduyt-crm-pp-cli reports put-targets` — Update reports targets
- `conduyt-crm-pp-cli reports run-custom` — Execute a custom report and return results
- `conduyt-crm-pp-cli reports update-custom` — Update a custom report

**ring-groups** — Manage ring groups

- `conduyt-crm-pp-cli ring-groups delete-id` — Delete ring-groups id
- `conduyt-crm-pp-cli ring-groups get` — List ring-groups
- `conduyt-crm-pp-cli ring-groups get-id` — Get ring-groups id
- `conduyt-crm-pp-cli ring-groups patch-id` — Update ring-groups id
- `conduyt-crm-pp-cli ring-groups post` — Create / invoke ring-groups
- `conduyt-crm-pp-cli ring-groups post-release-number` — Create / invoke ring-groups release-number

**saved-filters** — Manage saved filters

- `conduyt-crm-pp-cli saved-filters delete-id` — Delete saved-filters id
- `conduyt-crm-pp-cli saved-filters get` — List saved-filters
- `conduyt-crm-pp-cli saved-filters patch-id` — Update saved-filters id
- `conduyt-crm-pp-cli saved-filters post` — Create / invoke saved-filters

**schema** — Manage schema

- `conduyt-crm-pp-cli schema get-api-catalog` — List schema api-catalog
- `conduyt-crm-pp-cli schema get-public` — List schema public

**scim** — Manage scim

- `conduyt-crm-pp-cli scim delete-v2-users-id` — Delete scim v2 Users id
- `conduyt-crm-pp-cli scim get-v2-users` — List scim v2 Users
- `conduyt-crm-pp-cli scim get-v2-users-id` — Get scim v2 Users id
- `conduyt-crm-pp-cli scim patch-v2-users-id` — Update scim v2 Users id
- `conduyt-crm-pp-cli scim post-v2-users` — Create / invoke scim v2 Users
- `conduyt-crm-pp-cli scim put-v2-users-id` — Update scim v2 Users id

**scoring-rules** — Lead scoring rule management

- `conduyt-crm-pp-cli scoring-rules create` — Create a scoring rule
- `conduyt-crm-pp-cli scoring-rules delete` — Delete a scoring rule
- `conduyt-crm-pp-cli scoring-rules list` — List lead scoring rules
- `conduyt-crm-pp-cli scoring-rules recalculate-scores` — Recalculate all contact scores
- `conduyt-crm-pp-cli scoring-rules simulate` — Simulate scoring rules for a contact
- `conduyt-crm-pp-cli scoring-rules update` — Update a scoring rule

**screen-share** — Manage screen share

- `conduyt-crm-pp-cli screen-share delete-session-code` — Delete screen-share session code
- `conduyt-crm-pp-cli screen-share get-seats` — List screen-share seats
- `conduyt-crm-pp-cli screen-share post-seats` — Create / invoke screen-share seats
- `conduyt-crm-pp-cli screen-share post-session` — Create / invoke screen-share session

**settings** — Account settings, branding, SMS/Twilio configuration

- `conduyt-crm-pp-cli settings delete-ai-byo` — Delete settings ai byo
- `conduyt-crm-pp-cli settings delete-fromk` — Delete settings fromk
- `conduyt-crm-pp-cli settings delete-fromk-webhook` — Delete settings fromk webhook
- `conduyt-crm-pp-cli settings delete-project-blue` — Delete settings project-blue
- `conduyt-crm-pp-cli settings delete-project-blue-voice` — Delete settings project-blue voice
- `conduyt-crm-pp-cli settings delete-warmy` — Delete settings warmy
- `conduyt-crm-pp-cli settings delete-whatsapp` — Delete settings whatsapp
- `conduyt-crm-pp-cli settings get` — Get account settings
- `conduyt-crm-pp-cli settings get-ai` — List settings ai
- `conduyt-crm-pp-cli settings get-branding` — Get white-label branding settings
- `conduyt-crm-pp-cli settings get-delivery-guard` — List settings delivery-guard
- `conduyt-crm-pp-cli settings get-dormancy` — List settings dormancy
- `conduyt-crm-pp-cli settings get-fromk` — List settings fromk
- `conduyt-crm-pp-cli settings get-fromk-lines` — List settings fromk lines
- `conduyt-crm-pp-cli settings get-lead-visibility` — List settings lead-visibility
- `conduyt-crm-pp-cli settings get-operations` — List settings operations
- `conduyt-crm-pp-cli settings get-project-blue` — List settings project-blue
- `conduyt-crm-pp-cli settings get-project-blue-lines` — List settings project-blue lines
- `conduyt-crm-pp-cli settings get-project-blue-voice` — List settings project-blue voice
- `conduyt-crm-pp-cli settings get-scheduled-export` — List settings scheduled-export
- `conduyt-crm-pp-cli settings get-sms` — Get SMS provider settings
- `conduyt-crm-pp-cli settings get-twilio` — Get Twilio configuration
- `conduyt-crm-pp-cli settings get-warmy` — List settings warmy
- `conduyt-crm-pp-cli settings get-warmy-engine` — List settings warmy-engine
- `conduyt-crm-pp-cli settings get-whatsapp` — List settings whatsapp
- `conduyt-crm-pp-cli settings get-whatsapp-senders` — List settings whatsapp senders
- `conduyt-crm-pp-cli settings patch-ai` — Update settings ai
- `conduyt-crm-pp-cli settings patch-delivery-guard` — Update settings delivery-guard
- `conduyt-crm-pp-cli settings patch-dormancy` — Update settings dormancy
- `conduyt-crm-pp-cli settings patch-lead-visibility` — Update settings lead-visibility
- `conduyt-crm-pp-cli settings patch-scheduled-export` — Update settings scheduled-export
- `conduyt-crm-pp-cli settings patch-warmy-engine` — Update settings warmy-engine
- `conduyt-crm-pp-cli settings post-operations` — Create / invoke settings operations
- `conduyt-crm-pp-cli settings put-ai-byo` — Update settings ai byo
- `conduyt-crm-pp-cli settings put-fromk` — Update settings fromk
- `conduyt-crm-pp-cli settings put-fromk-webhook` — Update settings fromk webhook
- `conduyt-crm-pp-cli settings put-project-blue` — Update settings project-blue
- `conduyt-crm-pp-cli settings put-project-blue-lines-line-id` — Update settings project-blue lines lineId
- `conduyt-crm-pp-cli settings put-project-blue-voice` — Update settings project-blue voice
- `conduyt-crm-pp-cli settings put-warmy` — Update settings warmy
- `conduyt-crm-pp-cli settings put-whatsapp` — Update settings whatsapp
- `conduyt-crm-pp-cli settings test-integration` — Test an integration connection
- `conduyt-crm-pp-cli settings test-sms` — Send a test SMS
- `conduyt-crm-pp-cli settings test-twilio` — Test Twilio configuration
- `conduyt-crm-pp-cli settings update` — Update account settings
- `conduyt-crm-pp-cli settings update-branding` — Update white-label branding
- `conduyt-crm-pp-cli settings update-sms` — Update SMS provider settings
- `conduyt-crm-pp-cli settings update-twilio` — Update Twilio configuration

**smart-lists** — Static contact lists

- `conduyt-crm-pp-cli smart-lists create` — Create a smart list
- `conduyt-crm-pp-cli smart-lists delete-id` — Delete smart-lists id
- `conduyt-crm-pp-cli smart-lists get-id` — Get smart-lists id
- `conduyt-crm-pp-cli smart-lists list` — List smart lists (static contact lists)
- `conduyt-crm-pp-cli smart-lists update` — Update a smart list

**smart-views** — Manage smart views

- `conduyt-crm-pp-cli smart-views delete-id` — Delete smart-views id
- `conduyt-crm-pp-cli smart-views get-dial-order` — List smart-views dial-order
- `conduyt-crm-pp-cli smart-views list` — List available smart view definitions
- `conduyt-crm-pp-cli smart-views patch-dial-order` — Update smart-views dial-order
- `conduyt-crm-pp-cli smart-views patch-id` — Update smart-views id
- `conduyt-crm-pp-cli smart-views patch-reorder` — Update smart-views reorder
- `conduyt-crm-pp-cli smart-views post` — Create / invoke smart-views

**sms** — Manage sms

- `conduyt-crm-pp-cli sms get-send-logs` — List sms send-logs
- `conduyt-crm-pp-cli sms get-transports` — List sms transports
- `conduyt-crm-pp-cli sms get-whatsapp-templates` — List sms whatsapp templates
- `conduyt-crm-pp-cli sms post-send-logs` — Create / invoke sms send-logs

**sms-providers** — Manage sms providers

- `conduyt-crm-pp-cli sms-providers delete-id` — Delete sms-providers id
- `conduyt-crm-pp-cli sms-providers get` — List sms-providers
- `conduyt-crm-pp-cli sms-providers get-health` — List sms-providers health
- `conduyt-crm-pp-cli sms-providers get-id` — Get sms-providers id
- `conduyt-crm-pp-cli sms-providers patch-id` — Update sms-providers id
- `conduyt-crm-pp-cli sms-providers post` — Create / invoke sms-providers

**sso** — Manage sso

- `conduyt-crm-pp-cli sso delete-scim-tokens-id` — Delete sso scim-tokens id
- `conduyt-crm-pp-cli sso get-connection` — List sso connection
- `conduyt-crm-pp-cli sso get-scim-tokens` — List sso scim-tokens
- `conduyt-crm-pp-cli sso post-scim-tokens` — Create / invoke sso scim-tokens
- `conduyt-crm-pp-cli sso put-connection` — Update sso connection

**status-metadata** — Manage status metadata

- `conduyt-crm-pp-cli status-metadata delete-id` — Delete status-metadata id
- `conduyt-crm-pp-cli status-metadata get` — List status-metadata
- `conduyt-crm-pp-cli status-metadata get-buckets` — List status-metadata buckets
- `conduyt-crm-pp-cli status-metadata get-by-stage-stage-id` — Get status-metadata by-stage stageId
- `conduyt-crm-pp-cli status-metadata get-decisions` — List status-metadata decisions
- `conduyt-crm-pp-cli status-metadata get-id` — Get status-metadata id
- `conduyt-crm-pp-cli status-metadata patch-id` — Update status-metadata id
- `conduyt-crm-pp-cli status-metadata post` — Create / invoke status-metadata
- `conduyt-crm-pp-cli status-metadata post-dry-run` — Create / invoke status-metadata dry-run
- `conduyt-crm-pp-cli status-metadata post-import` — Create / invoke status-metadata import

**tags** — Tag management and merging

- `conduyt-crm-pp-cli tags create` — Create a tag
- `conduyt-crm-pp-cli tags delete` — Delete a tag
- `conduyt-crm-pp-cli tags get-id` — Get tags id
- `conduyt-crm-pp-cli tags list` — List tags
- `conduyt-crm-pp-cli tags merge` — Merge two tags
- `conduyt-crm-pp-cli tags update` — Update a tag

**tasks** — Task management with assignment and due dates

- `conduyt-crm-pp-cli tasks create` — Create a task
- `conduyt-crm-pp-cli tasks delete` — Delete a task
- `conduyt-crm-pp-cli tasks get` — Get a task by ID
- `conduyt-crm-pp-cli tasks get-assignment-rules` — List tasks assignment-rules
- `conduyt-crm-pp-cli tasks get-open-deal-ids` — List tasks open-deal-ids
- `conduyt-crm-pp-cli tasks get-queues` — List tasks queues
- `conduyt-crm-pp-cli tasks get-recurring` — List tasks recurring
- `conduyt-crm-pp-cli tasks get-sla-rules` — List tasks sla-rules
- `conduyt-crm-pp-cli tasks get-workload` — List tasks workload
- `conduyt-crm-pp-cli tasks list` — List tasks
- `conduyt-crm-pp-cli tasks put-assignment-rules` — Update tasks assignment-rules
- `conduyt-crm-pp-cli tasks put-queues` — Update tasks queues
- `conduyt-crm-pp-cli tasks put-recurring` — Update tasks recurring
- `conduyt-crm-pp-cli tasks put-sla-rules` — Update tasks sla-rules
- `conduyt-crm-pp-cli tasks update` — Update a task

**team** — Manage team

- `conduyt-crm-pp-cli team` — List team members

**up-for-grabs** — Manage up for grabs

- `conduyt-crm-pp-cli up-for-grabs` — List up-for-grabs

**users** — Team member management and invitations

- `conduyt-crm-pp-cli users get` — Get a team member by ID
- `conduyt-crm-pp-cli users get-me` — List users me
- `conduyt-crm-pp-cli users invite` — Invite a team member
- `conduyt-crm-pp-cli users list` — List team members
- `conduyt-crm-pp-cli users patch-me` — Update users me
- `conduyt-crm-pp-cli users post` — Create / invoke users
- `conduyt-crm-pp-cli users remove` — Remove a team member
- `conduyt-crm-pp-cli users update` — Update a team member

**warmy-engine** — Manage warmy engine

- `conduyt-crm-pp-cli warmy-engine get-health` — List warmy-engine health
- `conduyt-crm-pp-cli warmy-engine post-sync` — Create / invoke warmy-engine sync
- `conduyt-crm-pp-cli warmy-engine post-templates` — Create / invoke warmy-engine templates

**webhook-logs** — Manage webhook logs

- `conduyt-crm-pp-cli webhook-logs` — Request and response bodies are redacted unless the caller has full contact visibility.

**webhooks** — Outbound webhook management and logs

- `conduyt-crm-pp-cli webhooks create` — URL is validated for SSRF protection. HMAC signing secret is auto-generated.
- `conduyt-crm-pp-cli webhooks create-legacy` — Create an outbound webhook endpoint (legacy alias)
- `conduyt-crm-pp-cli webhooks delete` — Archive a webhook
- `conduyt-crm-pp-cli webhooks get` — Get a webhook by ID
- `conduyt-crm-pp-cli webhooks inbound-contact` — Inbound webhook for contact data
- `conduyt-crm-pp-cli webhooks inbound-deal` — Inbound webhook for deal data
- `conduyt-crm-pp-cli webhooks list` — List outbound webhooks
- `conduyt-crm-pp-cli webhooks list-deliveries` — List deliveries for a webhook
- `conduyt-crm-pp-cli webhooks list-endpoints` — List configured webhook endpoints (legacy alias)
- `conduyt-crm-pp-cli webhooks list-replay-deliveries` — List webhook deliveries for replay review
- `conduyt-crm-pp-cli webhooks messages` — Inbound webhook for message events
- `conduyt-crm-pp-cli webhooks replay` — Re-enqueues up to 100 matching deliveries. Replaying succeeded deliveries requires explicit confirmation.
- `conduyt-crm-pp-cli webhooks sms-inbound` — Twilio inbound SMS webhook
- `conduyt-crm-pp-cli webhooks sms-status` — Twilio SMS status callback
- `conduyt-crm-pp-cli webhooks stripe-invoice` — Stripe invoice webhook
- `conduyt-crm-pp-cli webhooks test` — Send a test payload to a webhook
- `conduyt-crm-pp-cli webhooks update` — Update a webhook
- `conduyt-crm-pp-cli webhooks voice-inbound` — Twilio inbound voice webhook
- `conduyt-crm-pp-cli webhooks voice-recording` — Twilio recording callback
- `conduyt-crm-pp-cli webhooks voice-status` — Twilio voice status callback
- `conduyt-crm-pp-cli webhooks voice-voicemail` — Twilio voicemail callback

**workflows** — Simple trigger-action workflows

- `conduyt-crm-pp-cli workflows create` — Create a workflow
- `conduyt-crm-pp-cli workflows delete` — Delete a workflow
- `conduyt-crm-pp-cli workflows get` — Get a workflow by ID
- `conduyt-crm-pp-cli workflows list` — List workflows
- `conduyt-crm-pp-cli workflows update` — Update a workflow


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `CONDUYT_CRM_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `conduyt-crm-pp-cli activities`
- `conduyt-crm-pp-cli activities list`
- `conduyt-crm-pp-cli admin`
- `conduyt-crm-pp-cli api-keys`
- `conduyt-crm-pp-cli api-keys list`
- `conduyt-crm-pp-cli appointments`
- `conduyt-crm-pp-cli appointments list`
- `conduyt-crm-pp-cli automation-executions`
- `conduyt-crm-pp-cli automation-executions list`
- `conduyt-crm-pp-cli automations`
- `conduyt-crm-pp-cli automations list`
- `conduyt-crm-pp-cli booking-pages`
- `conduyt-crm-pp-cli booking-pages list`
- `conduyt-crm-pp-cli calendar`
- `conduyt-crm-pp-cli calendars`
- `conduyt-crm-pp-cli calendars list`
- `conduyt-crm-pp-cli calls`
- `conduyt-crm-pp-cli calls list`
- `conduyt-crm-pp-cli chat`
- `conduyt-crm-pp-cli companies`
  Nested groups (run `--help` on each): `companies tags`, `companies timeline`, `companies intelligence`.
- `conduyt-crm-pp-cli companies list`
- `conduyt-crm-pp-cli contacts`
- `conduyt-crm-pp-cli contacts list`
- `conduyt-crm-pp-cli conversations`
- `conduyt-crm-pp-cli conversations list`
- `conduyt-crm-pp-cli custom-fields`
- `conduyt-crm-pp-cli custom-fields list`
- `conduyt-crm-pp-cli deals`
- `conduyt-crm-pp-cli deals list`
- `conduyt-crm-pp-cli document-templates`
- `conduyt-crm-pp-cli document-templates list`
- `conduyt-crm-pp-cli drip-campaigns`
- `conduyt-crm-pp-cli drip-campaigns list`
- `conduyt-crm-pp-cli drip-enrollments`
- `conduyt-crm-pp-cli drip-enrollments list`
- `conduyt-crm-pp-cli emails`
- `conduyt-crm-pp-cli emails list`
- `conduyt-crm-pp-cli files`
- `conduyt-crm-pp-cli files list`
- `conduyt-crm-pp-cli forms`
- `conduyt-crm-pp-cli forms list`
- `conduyt-crm-pp-cli imports`
- `conduyt-crm-pp-cli imports list`
- `conduyt-crm-pp-cli integrations`
- `conduyt-crm-pp-cli integrations list`
- `conduyt-crm-pp-cli invoices`
- `conduyt-crm-pp-cli invoices list`
- `conduyt-crm-pp-cli lead-routing`
- `conduyt-crm-pp-cli messages`
- `conduyt-crm-pp-cli messages list`
- `conduyt-crm-pp-cli notes`
- `conduyt-crm-pp-cli notes list`
- `conduyt-crm-pp-cli notifications`
- `conduyt-crm-pp-cli notifications list`
- `conduyt-crm-pp-cli pipelines`
  Nested group (run `--help`): `pipelines stages`.
- `conduyt-crm-pp-cli pipelines list`
- `conduyt-crm-pp-cli products`
- `conduyt-crm-pp-cli products list`
- `conduyt-crm-pp-cli reports`
- `conduyt-crm-pp-cli scoring-rules`
- `conduyt-crm-pp-cli scoring-rules list`
- `conduyt-crm-pp-cli smart-lists`
- `conduyt-crm-pp-cli smart-lists list`
- `conduyt-crm-pp-cli smart-views`
- `conduyt-crm-pp-cli smart-views list`
- `conduyt-crm-pp-cli tags`
- `conduyt-crm-pp-cli tags list`
- `conduyt-crm-pp-cli tasks`
- `conduyt-crm-pp-cli tasks list`
- `conduyt-crm-pp-cli users`
- `conduyt-crm-pp-cli users list`
- `conduyt-crm-pp-cli webhook-logs`
- `conduyt-crm-pp-cli webhooks`
- `conduyt-crm-pp-cli webhooks list`
- `conduyt-crm-pp-cli workflows`
- `conduyt-crm-pp-cli workflows list`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
conduyt-crm-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Is this list safe to text?

```bash
conduyt-crm-pp-cli send-check --list 8f2c1a7e --agent
```

one table per contact with phone, line type, validity and DNC state, with a summary of blocked contacts (the verdict is in the data, exit 0)

### Wait for the morning import, then explain it

```bash
conduyt-crm-pp-cli imports watch 3b9e4c2d --verify && conduyt-crm-pp-cli imports blame 3b9e4c2d --json
```

blocks until the job lands, then groups delivery outcomes by reason

### Why did speed to lead move?

```bash
conduyt-crm-pp-cli reports compare speed-to-lead --range 7d --agent --select metric,prior,current,delta_pct
```

the equal prior window is computed for you and every numeric value carries its delta

### Who is on pace this month?

```bash
conduyt-crm-pp-cli reports scorecard --range 30d --agent --select user,metric,target,actual,attainment_pct
```

targets joined to agent performance, narrowed to the columns an agent needs

### What will the dialer hand out next?

```bash
conduyt-crm-pp-cli dialer coverage --agent
```

priorities in dial order with queue depth and available agents

## Auth Setup

Run `conduyt-crm-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
echo "$TOKEN" | conduyt-crm-pp-cli auth set-token
```

Or set `CONDUYT_API_KEY` as an environment variable.

Run `conduyt-crm-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  conduyt-crm-pp-cli account-exports --agent --select data
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
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

- Use `--home <dir>` for one invocation, or set `CONDUYT_CRM_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `CONDUYT_CRM_CONFIG_DIR`, `CONDUYT_CRM_DATA_DIR`, `CONDUYT_CRM_STATE_DIR`, `CONDUYT_CRM_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `CONDUYT_CRM_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `conduyt-crm-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "conduyt-crm": {
        "command": "conduyt-crm-pp-mcp",
        "env": {
          "CONDUYT_CRM_HOME": "/srv/conduyt-crm"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `CONDUYT_CRM_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `CONDUYT_CRM_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
conduyt-crm-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "conduyt-crm-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `conduyt-crm-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `conduyt-crm-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `conduyt-crm-pp-cli sync --resources activities,admin,api-keys,appointments,automation-executions,automations,automations-actions,automations-condition-fields,automations-templates,booking-pages,calendar,calendars,calls,chat,companies,contacts,contacts-duplicates,conversations,custom-fields,deals,document-templates,drip-campaigns,drip-enrollments,emails,emails-sequences,emails-templates,files,forms,imports,integrations,integrations-zapier-subscribe,invoices,lead-routing,messages,notes,notifications,notifications-profiles,pipelines,products,reports,reports-custom,scoring-rules,smart-lists,smart-views,tags,tasks,users,webhook-logs,webhooks,webhooks-replay,workflows` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
conduyt-crm-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
conduyt-crm-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
conduyt-crm-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
conduyt-crm-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`conduyt-crm-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `CONDUYT_CRM_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
conduyt-crm-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
conduyt-crm-pp-cli feedback --stdin < notes.txt
conduyt-crm-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `CONDUYT_CRM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CONDUYT_CRM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
conduyt-crm-pp-cli profile save briefing --json
conduyt-crm-pp-cli --profile briefing account-exports
conduyt-crm-pp-cli profile list --json
conduyt-crm-pp-cli profile show briefing
conduyt-crm-pp-cli profile delete briefing --yes
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
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `conduyt-crm-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/cmd/conduyt-crm-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add conduyt-crm-pp-mcp -- conduyt-crm-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which conduyt-crm-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   conduyt-crm-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `conduyt-crm-pp-cli <command> --help`.
