# Kit CLI

Manage Kit's v4 creator marketing API from the terminal: account data, subscribers, broadcasts, sequences, tags, forms, custom fields, snippets, purchases, and webhooks.

Learn more at [Kit](https://developers.kit.com).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `kit-pp-cli` binary and the `pp-kit` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install kit
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install kit --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install kit --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install kit --agent claude-code
npx -y @mvanhorn/printing-press-library install kit --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/kit/cmd/kit-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/kit-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-kit --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-kit --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install kit --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/kit-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `KIT_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/kit/cmd/kit-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "kit": {
      "command": "kit-pp-mcp",
      "env": {
        "KIT_API_KEY": "<your-key>"
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

Create a v4 API key from Kit's Developer settings, then pass it with the `KIT_API_KEY` environment variable. Kit sends this credential as the `X-Kit-Api-Key` request header.

```bash
export KIT_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/kit-pp-cli/config.toml`.

### 3. Verify Setup

```bash
kit-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
kit-pp-cli account list
```

## Cookbook

Check the authenticated account:

```bash
kit-pp-cli account list --json
```

Get a compact creator operating snapshot in one read-only call:

```bash
kit-pp-cli workflow creator-snapshot --agent
```

Audit audience health before segmentation or cleanup:

```bash
kit-pp-cli workflow audience-health --agent
```

Inventory reusable content surfaces for campaign planning:

```bash
kit-pp-cli workflow content-inventory --agent
```

Look up one subscriber with profile fields, tags, and engagement stats:

```bash
kit-pp-cli workflow subscriber-lookup --email <subscriber-email> --agent
```

List subscribers and keep only the fields an agent usually needs:

```bash
kit-pp-cli subscribers list --agent --select id,email_address,state,created_at
```

Create and verify a tag:

```bash
tag_name="pp-verify-$(date +%s)"
kit-pp-cli tags create --name "$tag_name" --json
kit-pp-cli tags list --agent --select id,name
```

Review broadcast performance:

```bash
kit-pp-cli broadcasts list-stats --agent --select id,subject,total_recipients,open_rate,click_rate
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Creator operations
- **`workflow creator-snapshot`** — One-call read-only operating snapshot for Kit account, growth, audience, content, webhooks, and broadcast stats.

  _Use this first when an agent needs current creator-account context without manually fanning out across endpoint mirrors._

  ```bash
  kit-pp-cli workflow creator-snapshot --agent
  ```

### Audience intelligence
- **`workflow audience-health`** — Read-only subscriber status counts, recent growth stats, and largest tags by subscriber count.

  _Use this before list cleaning, segmentation, or campaign planning to avoid multiple fragile subscriber and tag calls._

  ```bash
  kit-pp-cli workflow audience-health --agent
  ```
- **`workflow subscriber-lookup`** — Read-only subscriber dossier by email or id with profile, custom fields, tags, attribution, and email stats.

  _Use this for support, segmentation checks, and personalization debugging before raw subscriber endpoint calls._

### Content planning
- **`workflow content-inventory`** — Read-only inventory of sequences, sequence emails, snippets, forms, templates, and recent broadcast stats.

  _Use this for content audits and planning instead of separately listing each Kit content surface._

  ```bash
  kit-pp-cli workflow content-inventory --agent
  ```

### Trends and ranking
- **`growth-trends`** — Correlates `/v4/account/growth_stats` with recent `/v4/broadcasts/stats` to report subscriber growth alongside broadcast send/open/click rates over a date range. Optional `--cache-stats` warms the local store via the typed `UpsertBroadcastsStats` write path.

  ```bash
  kit-pp-cli growth-trends --agent
  kit-pp-cli growth-trends --starting 2026-01-01 --ending 2026-03-31 --cache-stats --json
  ```

- **`tag-performance`** — Reads tags from the local store, queries the live API for each tag's current subscriber count, and sorts by share-of-total. Optional `--subscriber-query` calls `store.SearchSubscribers` (domain-typed FTS5 wrapper) to narrow the matched-subscriber count for a specific segment.

  ```bash
  kit-pp-cli sync tags
  kit-pp-cli tag-performance --agent
  kit-pp-cli tag-performance --subscriber-query "vip OR trial" --json
  ```

### MCP intent tools

Four first-class MCP intent tools wrap the workflow commands with typed input schemas and read-only annotations. Each tool delegates in-process to the matching Cobra command so orchestration logic stays in one place. Endpoint mirror tools (75 typed endpoint tools) remain fully exposed alongside the intent tools.

- `intent_workflow_creator_snapshot`
- `intent_workflow_audience_health`
- `intent_workflow_content_inventory`
- `intent_workflow_subscriber_lookup`

The MCP server (`kit-pp-mcp`) ships both stdio and streamable HTTP transports. Defaults to stdio; set `PP_MCP_TRANSPORT=http` or pass `--transport http` with `--addr :7777` to bind a remote-capable server.

## Usage

Run `kit-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Manage account

- **`kit-pp-cli account list`** - Get current account
- **`kit-pp-cli account list-colors`** - List colors
- **`kit-pp-cli account list-creatorprofile`** - Get Creator Profile
- **`kit-pp-cli account list-emailstats`** - Get email stats
- **`kit-pp-cli account list-growthstats`** - Get growth stats for a specific time period. Defaults to last 90 days.<br/><br/>NOTE: We return your stats in your sending time zone. This endpoint does not return timestamps in UTC.
- **`kit-pp-cli account update`** - Update colors

### broadcasts

Manage broadcasts

- **`kit-pp-cli broadcasts create`** - Draft or schedule to send a broadcast to all or a subset of your subscribers.<br/><br/>To save a draft, set `send_at` to `null`.<br/><br/>To publish to the web, set `public` to `true`.<br/><br/>To schedule the broadcast for sending, provide a `send_at` timestamp. Scheduled broadcasts should contain a subject and your content, at a minimum.<br/><br/>We currently support targeting your subscribers based on segment or tag ids.<aside class='notice'>Starting point templates are not currently supported.</aside>
- **`kit-pp-cli broadcasts delete`** - Delete a broadcast
- **`kit-pp-cli broadcasts get`** - Get a broadcast
- **`kit-pp-cli broadcasts list`** - List broadcasts
- **`kit-pp-cli broadcasts list-stats`** - Get stats for a list of broadcasts
- **`kit-pp-cli broadcasts update`** - Update an existing broadcast. Continue to draft or schedule to send a broadcast to all or a subset of your subscribers.<br/><br/>To save a draft, set `public` to false.<br/><br/>To schedule the broadcast for sending, set `public` to true and provide `send_at`. Scheduled broadcasts should contain a subject and your content, at a minimum.<br/><br/>We currently support targeting your subscribers based on segment or tag ids.

### bulk

Manage bulk

- **`kit-pp-cli bulk create`** - See "[Bulk & async processing](#bulk-amp-async-processing)" for more information.
- **`kit-pp-cli bulk create-customfields`** - Bulk update subscriber custom field values
- **`kit-pp-cli bulk create-forms`** - Adding subscribers to double opt-in forms will trigger sending an Incentive Email. Subscribers already added to the specified form will not receive the Incentive Email again. For more information about double opt-in see "[Double opt-in](#double-opt-in)". <br/><br/>The subscribers being added to the form must already exist. Subscribers can be created in bulk using the "[Bulk create subscriber](#bulk-create-subscribers)" endpoint.<br/><br/>See "[Bulk & async processing](#bulk-amp-async-processing)" for more information.
- **`kit-pp-cli bulk create-subscribers`** - See "[Bulk & async processing](#bulk-amp-async-processing)" for more information.
- **`kit-pp-cli bulk create-tags`** - See "[Bulk & async processing](#bulk-amp-async-processing)" for more information.
- **`kit-pp-cli bulk create-tags-2`** - The subscribers being tagged must already exist. Subscribers can be created in bulk using the "[Bulk create subscriber](#bulk-create-subscribers)" endpoint.<br/><br/>See "[Bulk & async processing](#bulk-amp-async-processing)" for more information.
- **`kit-pp-cli bulk delete`** - See "[Bulk & async processing](#bulk-amp-async-processing)" for more information.

### custom-fields

Manage custom fields

- **`kit-pp-cli custom-fields create`** - Create a custom field for your account. The label field must be unique to your account. Whitespace will be removed from the beginning and the end of your label.<br/><br/>Additionally, a key field and a name field will be generated for you. The key is an ASCII-only, lowercased, underscored representation of your label. This key must be unique to your account. Keys are used in personalization tags in sequences and broadcasts. Names are unique identifiers for use in the HTML of custom forms. They are made up of a combination of ID and the key of the custom field prefixed with "ck_field".
- **`kit-pp-cli custom-fields delete`** - This will remove all data in this field from your subscribers.
- **`kit-pp-cli custom-fields list`** - A custom field allows you to collect subscriber information beyond the standard fields of first name and email address. An example would be a custom field called last name so you can get the full names of your subscribers.<br/><br/>You create a custom field, and then you're able to use that in your forms or emails.
- **`kit-pp-cli custom-fields update`** - Updates a custom field label (see [Create a custom field](/api-reference/custom-fields/create-a-custom-field) for more information on labels). Note that the key will change but the name remains the same when the label is updated.<br/><br/><strong>Warning: </strong>An update to a custom field will break all of the liquid personalization tags in emails that reference it - e.g. if you update a `Zip_Code` custom field to `Post_Code`, all liquid tags referencing `{{ subscriber.Zip_Code }}` would no longer work and need to be replaced with `{{ subscriber.Post_Code }}`.

### email-templates

Manage email templates

- **`kit-pp-cli email-templates`** - List email templates

### forms

Manage forms

- **`kit-pp-cli forms`** - List forms

### posts

Manage posts

- **`kit-pp-cli posts get`** - Get a post
- **`kit-pp-cli posts list`** - List posts

### purchases

Manage purchases

- **`kit-pp-cli purchases create`** - Create a purchase
- **`kit-pp-cli purchases get`** - Get a purchase
- **`kit-pp-cli purchases list`** - List purchases

### segments

Manage segments

- **`kit-pp-cli segments`** - List segments

### sequences

Manage sequences

- **`kit-pp-cli sequences create`** - Creates an empty sequence — the container that holds sequence emails. After creating the shell, use [Create a sequence email](/api-reference/sequence-emails/create-a-sequence-email) to populate it.

Only `name` is required. Every other field has a sensible default: Kit fills in the account's default sending address, a daily send schedule, and the account time zone — and any of these can be tuned later via [Update a sequence](/api-reference/sequences/update-a-sequence).

Two behavioural toggles worth flagging up front. `repeat` controls whether a subscriber can re-enter the sequence: by default a subscriber receives the emails once and is marked complete, but with `repeat: true`, re-adding the same subscriber via a Visual Automation, Rule, Bulk Action, or Import resets their position to the start. Filters and exclusions still apply across restarts. `hold` (evergreen) keeps subscribers active in the sequence after they've received every published email — useful when you plan to add more emails later. Without `hold`, subscribers transition to Completed and won't pick up future additions.

`exclude_subscriber_sources` lets you exclude subscribers acquired via specific tags, sequences, forms, or segments — they'll skip this sequence entirely.

For end-user context, see the help articles on [creating and sending a sequence](https://help.kit.com/en/articles/2502629-creating-and-sending-a-sequence-in-kit), [restarting a sequence](https://help.kit.com/en/articles/5022528-restart-a-sequence), and [holding subscribers in evergreen sequences](https://help.kit.com/en/articles/5192801-how-to-hold-subscribers-in-evergreen-sequences).
- **`kit-pp-cli sequences delete`** - Soft-deletes a sequence. The sequence is removed from active delivery immediately, with cleanup of associated state happening in the background.

**Warning:** deleting a sequence with active subscribers stops deliveries to those subscribers — they will not receive remaining emails, and any Visual Automations referencing the sequence will need to be updated. Confirm the sequence is not in active use before deleting.

If you want to pause rather than delete, use [Update a sequence](/api-reference/sequences/update-a-sequence) with `active: false` instead.
- **`kit-pp-cli sequences get`** - Fetches a single sequence by `id`. Use this when you need the current schedule, the `active` / `repeat` / `hold` flags, the configured `email_address` and `email_template_id`, or `exclude_subscriber_sources` for a known sequence — for example, to confirm settings before adding subscribers or to render an editor.

For the individual emails inside the sequence, use [List sequence emails](/api-reference/sequence-emails/list-sequence-emails). For the sequence model and field semantics, see [Create a sequence](/api-reference/sequences/create-a-sequence).
- **`kit-pp-cli sequences list`** - Returns every sequence on the account. A sequence is a self-contained set of automated emails — subscribers join, then receive each email in order, governed by per-email `delay_value` / `delay_unit` and the sequence's overall `send_days`, `send_hour`, and `time_zone` schedule.

Each entry carries the schedule defaults plus three behavioural toggles: `active` (whether the sequence is delivering), `repeat` (whether subscribers can re-enter), and `hold` (whether subscribers stay active after receiving every published email — an evergreen pattern). See [Create a sequence](/api-reference/sequences/create-a-sequence) for the full sequence model.

Once you have a sequence's `id`, [List sequence emails](/api-reference/sequence-emails/list-sequence-emails) returns the individual emails inside it.

For end-user context on how creators build sequences, see the help articles on [creating and sending a sequence](https://help.kit.com/en/articles/2502629-creating-and-sending-a-sequence-in-kit) and [evergreen content](https://help.kit.com/en/articles/2502575-what-is-evergreen-content).
- **`kit-pp-cli sequences update`** - Updates any sequence settings — `name`, `email_address`, schedule (`send_days`, `send_hour`, `time_zone`), `email_template_id`, `exclude_subscriber_sources`, or the `active` / `repeat` / `hold` flags. Only fields included in the request body change; everything else is preserved.

Some changes have user-visible side effects on subscribers already in the sequence:

**Note:** flipping `active` from `false` to `true` resumes delivery for queued subscribers. Flipping it back to `false` pauses the sequence — subscribers stay in their current position but no new emails are sent until it's reactivated.

**Note:** changing the schedule (`send_days`, `send_hour`, or `time_zone`) only affects future sends. It does not retroactively reschedule emails already queued for delivery.

**Warning:** turning off `repeat` while subscribers are mid-sequence does not stop them from finishing — but they won't be re-eligible to start over after completing.

See [Create a sequence](/api-reference/sequences/create-a-sequence) for the full sequence model and what each field controls.

### snippets

Manage snippets

- **`kit-pp-cli snippets create`** - Snippets are reusable pieces of email content you can drop into a broadcast or sequence email using Liquid: `{{ snippet.key }}`. Update the snippet once and every email that references it picks up the new content on next send.

There are two `snippet_type`s. **`inline`** snippets store plain-text content (with Liquid variable support like `{{ subscriber.first_name }}`) in the `content` field. **`block`** snippets store rich-text HTML — text, lists, images, buttons — in `document_attributes.value_html`. A snippet's type is fixed at creation: it cannot be changed via [Update a snippet](/api-reference/snippets/update-a-snippet).

The response includes a `key` field. That's the identifier you use in Liquid — for example, a snippet returned with `"key": "welcome-message"` is referenced inside a broadcast as `{{ snippet.welcome-message }}`. Keys are derived from the snippet name on creation.

**Note:** the API rejects circular references — a snippet cannot reference itself, directly or transitively — with a `422` validation error.

For end-user context on how creators build and edit snippets in the Kit UI, see the help articles on [content snippets](https://help.kit.com/en/articles/3812712-creating-and-using-content-snippets-in-your-kit-emails) and [code snippets for custom templates](https://help.kit.com/en/articles/2810398-code-snippets-for-custom-email-templates).
- **`kit-pp-cli snippets get`** - Fetches a single snippet by `id`. Unlike [List snippets](/api-reference/snippets/list-snippets), this endpoint **always returns the full `content` and `document`** — no `include_content` flag needed.

Use this when you have an `id` (e.g. stored from a prior create call) and need the current `key` and body — for example, to preview the resolved HTML or confirm a snippet still exists before referencing it as `{{ snippet.key }}` in a broadcast or sequence email. See [Create a snippet](/api-reference/snippets/create-a-snippet) for the snippet model.
- **`kit-pp-cli snippets list`** - Returns every snippet on the account. Each snippet's `key` is the identifier used in Liquid — `{{ snippet.key }}` — when creating a broadcast or sequence email. See [Create a snippet](/api-reference/snippets/create-a-snippet) for how snippets work end-to-end.

**Tip:** the heavier `content` and `document` fields are omitted by default to keep responses fast. Pass `include_content=true` when you need the body — for example, to render a preview or audit Liquid usage.

Filter the result with `snippet_type` (`inline` or `block`) and `archived` (defaults to `false`, set `true` to list only archived snippets).
- **`kit-pp-cli snippets update`** - Rename a snippet, replace its body, or archive/restore it. Updates apply on the next send of any email that references the snippet via `{{ snippet.key }}` — there's no per-email versioning, so a content change ripples to every broadcast or sequence email using that key.

The request body must match the existing `snippet_type`. For an **`inline`** snippet, send `content` (and optionally `name`, `archived`). For a **`block`** snippet, send `document_attributes.value_html` (and optionally `name`, `archived`). Pass `archived: true` to archive, `false` to restore.

**Warning:** `snippet_type` is immutable. Sending a different value, or sending the body shape for the wrong type, returns a `422` with `snippet_type cannot be changed`.

See [Create a snippet](/api-reference/snippets/create-a-snippet) for the full snippet model and how `key` ties into Liquid.

### subscribers

Manage subscribers

- **`kit-pp-cli subscribers create`** - Behaves as an upsert. If a subscriber with the provided email address does not exist, it creates one with the specified first name and state. If a subscriber with the provided email address already exists, it updates the first name.<br/><br/>If you include a custom field key that does not exist on your account, the request returns an error. Use [List custom fields](/api-reference/custom-fields/list-custom-fields) to retrieve existing keys, or [Create a custom field](/api-reference/custom-fields/create-a-custom-field) to add new fields before setting them for subscribers.<br/><br/><strong>NOTE:</strong> Updating the subscriber state with this endpoint is not supported at this time.<br/><strong>NOTE:</strong> We support creating/updating a maximum of 140 custom fields at a time.
- **`kit-pp-cli subscribers create-filter`** - Filter subscribers based on engagement
- **`kit-pp-cli subscribers get`** - Get a subscriber
- **`kit-pp-cli subscribers list`** - List subscribers
- **`kit-pp-cli subscribers update`** - If you include a custom field key that does not exist on your account, the request returns an error. Use [List custom fields](/api-reference/custom-fields/list-custom-fields) to retrieve existing keys, or [Create a custom field](/api-reference/custom-fields/create-a-custom-field) to add new fields before setting them for subscribers.<br/><br/><strong>NOTE:</strong> We support creating/updating a maximum of 140 custom fields at a time.

### tags

Manage tags

- **`kit-pp-cli tags create`** - Create a tag
- **`kit-pp-cli tags list`** - List tags
- **`kit-pp-cli tags update`** - Update tag name

### webhooks

Manage webhooks

- **`kit-pp-cli webhooks create`** - Available event types:<br/>- `subscriber.subscriber_activate`<br/>- `subscriber.subscriber_unsubscribe`<br/>- `subscriber.subscriber_bounce`<br/>- `subscriber.subscriber_complain`<br/>- `subscriber.form_subscribe`, required parameter `form_id` [Integer]<br/>- `subscriber.course_subscribe`, required parameter `sequence_id` [Integer]<br/>- `subscriber.course_complete`, required parameter `sequence_id` [Integer]<br/>- `subscriber.link_click`, required parameter `initiator_value` [String] as a link URL<br/>- `subscriber.product_purchase`, required parameter `product_id` [Integer]<br/>- `subscriber.tag_add`, required parameter `tag_id` [Integer]<br/>- `subscriber.tag_remove`, required parameter `tag_id` [Integer]<br/>- `purchase.purchase_create`<br/>- `custom_field.field_created`<br/>- `custom_field.field_deleted`<br/>- `custom_field.field_value_updated`, required parameter `custom_field_id` [Integer]
- **`kit-pp-cli webhooks delete`** - Delete a webhook
- **`kit-pp-cli webhooks list`** - Webhooks are automations that will receive subscriber data when a subscriber event is triggered, such as when a subscriber completes a sequence.<br/><br/>When a webhook is triggered, a `POST` request will be made to your URL with a JSON payload.

### workflow

Kit-specific compound workflows for agents. These call real Kit v4 endpoints and return compact JSON so an agent can work from higher-level operating context instead of juggling many raw endpoint tools.

- **`kit-pp-cli workflow creator-snapshot`** - One-call account, growth, audience, content, webhook, and broadcast operating snapshot.
- **`kit-pp-cli workflow audience-health`** - Subscriber status counts, growth stats, and largest tags by subscriber count.
- **`kit-pp-cli workflow content-inventory`** - Sequences, sequence emails, snippets, forms, email templates, and recent broadcast stats.
- **`kit-pp-cli workflow subscriber-lookup --email <email>`** - Subscriber profile, custom fields, tags, attribution, and email stats for support or personalization checks.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
kit-pp-cli account list

# JSON for scripting and agents
kit-pp-cli account list --json

# Filter to specific fields
kit-pp-cli account list --json --select id,name,status

# Dry run — show the request without sending
kit-pp-cli account list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
kit-pp-cli account list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Workflow-first** - Kit-specific `workflow` commands combine common endpoint fan-outs into one read-only result
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
kit-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/kit-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `KIT_API_KEY` | per_call | No | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `kit-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $KIT_API_KEY`
- Confirm the key was created as a v4 API key in Kit's Developer settings
- Kit API-key traffic is limited to 120 requests over a rolling 60 seconds; lower `--rate-limit` if you see 429 responses
- Some bulk and purchase creation endpoints require OAuth according to Kit's docs; this CLI intentionally stays on v4 API-key auth
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
