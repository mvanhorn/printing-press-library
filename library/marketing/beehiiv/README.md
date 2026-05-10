# Beehiiv CLI



## Install

The recommended path installs both the `beehiiv-pp-cli` binary and the `pp-beehiiv` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install beehiiv
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install beehiiv --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/beehiiv-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-beehiiv --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-beehiiv --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-beehiiv skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-beehiiv. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
beehiiv-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export BEEHIIV_BEARER_AUTH="your-token-here"
```

### 3. Verify Setup

```bash
beehiiv-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
beehiiv-pp-cli advertisement-opportunities mock-value
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Publication intelligence
- **`insights growth-summary`** — Summarize publication, subscriber, post, referral, and custom-field health in one read-only response.

  _Use this first when an agent needs a compact account-level picture before choosing a narrower Beehiiv endpoint._

  ```bash
  beehiiv-pp-cli insights growth-summary pub_00000000-0000-0000-0000-000000000000 --agent
  ```
- **`insights post-performance`** — List recent posts with status, audience, publish timing, and any available expanded stats.

  _Use this when an agent needs to inspect content output before drilling into one post._

  ```bash
  beehiiv-pp-cli insights post-performance pub_00000000-0000-0000-0000-000000000000 --limit 25 --agent
  ```

### Audience intelligence
- **`insights subscriber-sources`** — Group subscribers by UTM, channel, and referring-site fields to see where audience growth is coming from.

  _Use this when the question is about acquisition channels rather than individual subscribers._

  ```bash
  beehiiv-pp-cli insights subscriber-sources pub_00000000-0000-0000-0000-000000000000 --limit 100 --agent
  ```
- **`insights field-coverage`** — Inspect custom-field definitions alongside subscriber sample size for enrichment planning.

  _Use this before importing, enriching, or auditing subscriber metadata._

  ```bash
  beehiiv-pp-cli insights field-coverage pub_00000000-0000-0000-0000-000000000000 --agent
  ```
- **`insights subscriber-lookup`** — Find one subscriber by email or subscription ID and return a compact subscriber record.

  _Use this when the task is about one subscriber and broad list calls would waste context._

  ```bash
  beehiiv-pp-cli insights subscriber-lookup pub_00000000-0000-0000-0000-000000000000 --email reader@example.com --agent
  ```

### Growth loops
- **`insights referral-health`** — Summarize referral-program configuration and subscriber referral-code coverage.

  _Use this when an agent needs to check whether referral growth is configured and visible in subscriber data._

  ```bash
  beehiiv-pp-cli insights referral-health pub_00000000-0000-0000-0000-000000000000 --agent
  ```

## Usage

Run `beehiiv-pp-cli --help` for the full command reference and flag list.

## Commands

### advertisement-opportunities

Manage advertisement opportunities

- **`beehiiv-pp-cli advertisement-opportunities index`** - Get advertisement opportunities <Badge intent="info" minimal outlined>OAuth Scope: posts:read</Badge>

### authors

Manage authors

- **`beehiiv-pp-cli authors index`** - Retrieve a list of authors available for the publication.
- **`beehiiv-pp-cli authors show`** - Retrieve a single author from a publication.

### automations

Manage automations

- **`beehiiv-pp-cli automations index`** - List automations <Badge intent="info" minimal outlined>OAuth Scope: automations:read</Badge>
- **`beehiiv-pp-cli automations show`** - Get automation <Badge intent="info" minimal outlined>OAuth Scope: automations:read</Badge>

### bulk-subscription-updates

Manage bulk subscription updates

- **`beehiiv-pp-cli bulk-subscription-updates index`** - List subscription updates <Badge intent="info" minimal outlined>OAuth Scope: subscriptions:read</Badge>
- **`beehiiv-pp-cli bulk-subscription-updates show`** - Get subscription update <Badge intent="info" minimal outlined>OAuth Scope: subscriptions:read</Badge>

### bulk-subscriptions

Manage bulk subscriptions

- **`beehiiv-pp-cli bulk-subscriptions create`** - Bulk create subscription <Badge intent="info" minimal outlined>OAuth Scope: subscriptions:write</Badge>

### condition-sets

Manage condition sets

- **`beehiiv-pp-cli condition-sets index`** - Retrieve all active condition sets for a publication. Condition sets define reusable audience segments for targeting content to specific subscribers. Use the `purpose` parameter to filter by a specific use case.
- **`beehiiv-pp-cli condition-sets show`** - Retrieve a single active dynamic content condition set for a publication. Use `expand[]=stats` to calculate and return the active subscriber count synchronously.

### custom-fields

Manage custom fields

- **`beehiiv-pp-cli custom-fields create`** - Create custom field <Badge intent="info" minimal outlined>OAuth Scope: custom_fields:write</Badge>
- **`beehiiv-pp-cli custom-fields delete`** - Delete custom field <Badge intent="info" minimal outlined>OAuth Scope: custom_fields:write</Badge>
- **`beehiiv-pp-cli custom-fields index`** - List custom fields <Badge intent="info" minimal outlined>OAuth Scope: custom_fields:read</Badge>
- **`beehiiv-pp-cli custom-fields patch`** - Update custom field <Badge intent="info" minimal outlined>OAuth Scope: custom_fields:write</Badge>
- **`beehiiv-pp-cli custom-fields put`** - Update custom field <Badge intent="info" minimal outlined>OAuth Scope: custom_fields:write</Badge>
- **`beehiiv-pp-cli custom-fields show`** - Get custom field <Badge intent="info" minimal outlined>OAuth Scope: custom_fields:read</Badge>

### data-privacy

Manage data privacy

- **`beehiiv-pp-cli data-privacy data-deletion-create`** - <Warning>This is a gated feature that requires enablement. Contact support to enable Data Deletion API access for your organization.</Warning>

Creates a data deletion request for a subscriber within your organization. The subscriber's data will be redacted from all publications in the organization after a 14-day safety delay. This action cannot be undone once processing begins.
- **`beehiiv-pp-cli data-privacy data-deletion-index`** - <Warning>This is a gated feature that requires enablement. Contact support to enable Data Deletion API access for your organization.</Warning>

List all data deletion requests for your organization.
- **`beehiiv-pp-cli data-privacy data-deletion-show`** - <Warning>This is a gated feature that requires enablement. Contact support to enable Data Deletion API access for your organization.</Warning>

Retrieve the details and current status of a specific data deletion request.

### email-blasts

Manage email blasts

- **`beehiiv-pp-cli email-blasts index`** - List email blasts <Badge intent="info" minimal outlined>OAuth Scope: posts:read</Badge>
- **`beehiiv-pp-cli email-blasts show`** - Get email blast <Badge intent="info" minimal outlined>OAuth Scope: posts:read</Badge>

### engagements

Manage engagements

- **`beehiiv-pp-cli engagements index`** - Retrieve email engagement metrics for a specific publication over a defined date range and granularity.<br><br> By default, the endpoint returns metrics for the past day, aggregated daily. The max number of days allowed is 31. All dates and times are in UTC.

### newsletter-lists

Manage newsletter lists

- **`beehiiv-pp-cli newsletter-lists index`** - <Note title="Currently in beta" icon="b">
  Newsletter Lists is currently in beta, the API is subject to change.
</Note>
List all newsletter lists for a publication.
- **`beehiiv-pp-cli newsletter-lists show`** - <Note title="Currently in beta" icon="b">
  Newsletter Lists is currently in beta, the API is subject to change.
</Note>
Retrieve a single newsletter list belonging to a specific publication.

### polls

Manage polls

- **`beehiiv-pp-cli polls index`** - Retrieve all polls belonging to a specific publication. Poll choices are always included. Use `expand[]=stats` to include aggregate vote counts per choice.
- **`beehiiv-pp-cli polls show`** - Retrieve detailed information about a specific poll belonging to a publication. Use `expand[]=stats` for aggregate vote counts, or `expand[]=poll_responses` for individual subscriber responses.

### post-templates

Manage post templates

- **`beehiiv-pp-cli post-templates index`** - Retrieve a list of post templates available for the publication.

### posts

Manage posts

- **`beehiiv-pp-cli posts aggregate-stats`** - Get aggregate stats <Badge intent="info" minimal outlined>OAuth Scope: posts:read</Badge>
- **`beehiiv-pp-cli posts create`** - <Note title="Currently in beta" icon="b">
  This feature is currently in beta, the API is subject to change, and available only to Enterprise users.<br/><br/>To inquire about Enterprise pricing,
  please visit our <a href="https://www.beehiiv.com/enterprise">Enterprise page</a>.
</Note>
Create a post for a specific publication. For a detailed walkthrough including setup, testing workflows, and working with custom HTML and templates, see the <a href="https://www.beehiiv.com/support/article/36759164012439-using-the-send-api-and-create-post-endpoint">Using the Send API and Create Post Endpoint</a> guide.

## Content methods

There are three ways to provide content for a post. You must provide either `blocks` or `body_content`, but not both.

### 1. Blocks

Use the `blocks` field to build your post with structured content blocks such as paragraphs, images, headings, buttons, tables, and more. Each block has a `type` and its own set of properties. This method gives you fine-grained control over individual content elements and supports features like visual settings, visibility settings, and dynamic content targeting.

### 2. Raw HTML (`body_content`)

Use the `body_content` field to provide a single string of raw HTML. The HTML is wrapped in an `htmlSnippet` block internally. This is useful when you have pre-built HTML content or are migrating from another platform.

### 3. HTML blocks within blocks

Use `type: html` blocks inside the `blocks` array to embed raw HTML snippets alongside other structured blocks. This lets you mix structured content (paragraphs, images, etc.) with custom HTML where needed.

## CSS and styling guardrails

beehiiv processes all HTML content through a sanitization pipeline. When using `body_content` or `html` blocks, be aware of the following:

- **`<style>` tags are removed.** All `<style>` block elements are stripped during sanitization. Do not rely on embedded stylesheets.
- **`<link>` tags are removed.** External stylesheet references are not allowed.
- **Inline styles are preserved.** Styles applied directly to elements via the `style` attribute (e.g., `<div style="color: red;">`) are kept intact.
- **CSS classes have no effect.** While class attributes are not stripped, no corresponding stylesheets are loaded to apply them.
- **beehiiv's email template wraps your content.** Your HTML is rendered inside beehiiv's email table structure, which applies its own layout and spacing. This may affect the appearance of your content.
- **Use inline styles for all visual styling.** Since `<style>` and `<link>` tags are removed, inline styles on individual elements are the only reliable way to control appearance.
- **`beehiiv-pp-cli posts delete`** - Delete or Archive a post. Any post that has been confirmed will have it's status changed to `archived`. Posts in the `draft` status will be permanently deleted.
- **`beehiiv-pp-cli posts index`** - List posts <Badge intent="info" minimal outlined>OAuth Scope: posts:read</Badge>
- **`beehiiv-pp-cli posts show`** - Get post <Badge intent="info" minimal outlined>OAuth Scope: posts:read</Badge>
- **`beehiiv-pp-cli posts update`** - <Note title="Currently in beta" icon="b">
  This feature is currently in beta, the API is subject to change, and available only to Enterprise users.<br/><br/>To inquire about Enterprise pricing,
  please visit our <a href="https://www.beehiiv.com/enterprise">Enterprise page</a>.
</Note>
Update an existing post for a specific publication. Only the fields provided in the request body will be updated — all other fields remain unchanged. For a detailed walkthrough of content methods and working with custom HTML, see the <a href="https://www.beehiiv.com/support/article/36759164012439-using-the-send-api-and-create-post-endpoint">Using the Send API and Create Post Endpoint</a> guide.

To update post content, provide either `blocks` or `body_content` (not both). If neither is provided, the existing content is preserved. The same content methods and CSS guardrails described in the create endpoint apply here.

### publications

Manage publications

- **`beehiiv-pp-cli publications index`** - List publications <Badge intent="info" minimal outlined>OAuth Scope: publications:read</Badge>
- **`beehiiv-pp-cli publications show`** - Get publication <Badge intent="info" minimal outlined>OAuth Scope: publications:read</Badge>

### referral-program

Manage referral program

- **`beehiiv-pp-cli referral-program show`** - Get referral program <Badge intent="info" minimal outlined>OAuth Scope: referral_program:read</Badge>

### segments

Manage segments

- **`beehiiv-pp-cli segments create`** - Create a new segment.<br><br> **Manual segments** — Use `subscriptions` or `emails` input to create a segment from an explicit list of subscription IDs or email addresses. The segment is processed synchronously and returns with `status: completed`. Net new email addresses will be ignored; create subscriptions using the `Create Subscription` endpoint.<br><br> **Dynamic segments** — Use `custom_fields` input to create a segment that filters subscribers by custom field values. The segment is processed asynchronously and returns with `status: pending`. Results will be available in the `List Segment Subscribers` endpoint after processing is complete.
- **`beehiiv-pp-cli segments delete`** - Delete a segment. Deleting the segment does not effect the subscriptions in the segment.
- **`beehiiv-pp-cli segments index`** - List segments <Badge intent="info" minimal outlined>OAuth Scope: segments:read</Badge>
- **`beehiiv-pp-cli segments show`** - Get segment <Badge intent="info" minimal outlined>OAuth Scope: segments:read</Badge>

### subscriptions

Manage subscriptions

- **`beehiiv-pp-cli subscriptions bulk-updates-patch`** - Update subscriptions <Badge intent="info" minimal outlined>OAuth Scope: subscriptions:write</Badge>
- **`beehiiv-pp-cli subscriptions bulk-updates-patch-status`** - Update subscriptions' status <Badge intent="info" minimal outlined>OAuth Scope: subscriptions:write</Badge>
- **`beehiiv-pp-cli subscriptions bulk-updates-put`** - Update subscriptions <Badge intent="info" minimal outlined>OAuth Scope: subscriptions:write</Badge>
- **`beehiiv-pp-cli subscriptions bulk-updates-put-status`** - Update subscriptions' status <Badge intent="info" minimal outlined>OAuth Scope: subscriptions:write</Badge>
- **`beehiiv-pp-cli subscriptions create`** - Create subscription <Badge intent="info" minimal outlined>OAuth Scope: subscriptions:write</Badge>
- **`beehiiv-pp-cli subscriptions delete`** - <Warning>This cannot be undone. All data associated with the subscription will also be deleted. We recommend unsubscribing when possible instead of deleting. If a premium subscription is deleted they will no longer be billed.</Warning> Deletes a subscription.
- **`beehiiv-pp-cli subscriptions get-by-email`** - <Info>Please note that this endpoint requires the email to be URL encoded. Please reference your language's documentation for the correct method of encoding.</Info> Retrieve a single subscription belonging to a specific email address in a specific publication.
- **`beehiiv-pp-cli subscriptions get-by-id`** - <Info>In previous versions of the API, another endpoint existed to retrieve a subscription by the subscriber ID. This endpoint is now deprecated and will be removed in a future version of the API. Please use this endpoint instead. The subscription ID can be found by exporting a list of subscriptions either via the `Settings > Publications > Export Data` or by exporting a CSV in a segment.</Info> Retrieve a single subscription belonging to a specific publication.
- **`beehiiv-pp-cli subscriptions get-by-subscriber-id`** - Get subscription by subscriber ID <Badge intent="info" minimal outlined>OAuth Scope: subscriptions:read</Badge>
- **`beehiiv-pp-cli subscriptions index`** - Retrieve all subscriptions belonging to a specific publication.

<Info> **New**: This endpoint now supports cursor-based pagination for better performance and consistency. Use the `cursor` parameter instead of `page` for new integrations. </Info>
<Warning> **Deprecation Notice**: Offset-based pagination (using `page` parameter) is deprecated and limited to 100 pages maximum. Please migrate to cursor-based pagination. See our [Pagination Guide](/welcome/pagination) for details. </Warning>
- **`beehiiv-pp-cli subscriptions patch`** - Update subscription by ID <Badge intent="info" minimal outlined>OAuth Scope: subscriptions:write</Badge>
- **`beehiiv-pp-cli subscriptions put`** - Update subscription by ID <Badge intent="info" minimal outlined>OAuth Scope: subscriptions:write</Badge>
- **`beehiiv-pp-cli subscriptions update-by-email`** - Update subscription by email <Badge intent="info" minimal outlined>OAuth Scope: subscriptions:write</Badge>

### tiers

Manage tiers

- **`beehiiv-pp-cli tiers create`** - Create a tier <Badge intent="info" minimal outlined>OAuth Scope: tiers:write</Badge>
- **`beehiiv-pp-cli tiers index`** - List tiers <Badge intent="info" minimal outlined>OAuth Scope: tiers:read</Badge>
- **`beehiiv-pp-cli tiers patch`** - Update a tier <Badge intent="info" minimal outlined>OAuth Scope: tiers:write</Badge>
- **`beehiiv-pp-cli tiers put`** - Update a tier <Badge intent="info" minimal outlined>OAuth Scope: tiers:write</Badge>
- **`beehiiv-pp-cli tiers show`** - Get tier <Badge intent="info" minimal outlined>OAuth Scope: tiers:read</Badge>

### users

Manage users

- **`beehiiv-pp-cli users oauth-identify`** - Identify user <Badge intent="info" minimal outlined>OAuth Scope: identify:read</Badge>

### webhooks

Manage webhooks

- **`beehiiv-pp-cli webhooks create`** - Create a webhook <Badge intent="info" minimal outlined>OAuth Scope: webhooks:write</Badge>
- **`beehiiv-pp-cli webhooks delete`** - Delete a webhook <Badge intent="info" minimal outlined>OAuth Scope: webhooks:write</Badge>
- **`beehiiv-pp-cli webhooks index`** - List webhooks <Badge intent="info" minimal outlined>OAuth Scope: webhooks:read</Badge>
- **`beehiiv-pp-cli webhooks show`** - Get webhook <Badge intent="info" minimal outlined>OAuth Scope: webhooks:read</Badge>
- **`beehiiv-pp-cli webhooks update`** - Update webhook <Badge intent="info" minimal outlined>OAuth Scope: webhooks:write</Badge>

### workspaces

Manage workspaces

- **`beehiiv-pp-cli workspaces identify`** - Identify workspace <Badge intent="info" minimal outlined>OAuth Scope: identify:read</Badge>
- **`beehiiv-pp-cli workspaces publications-by-subscription-email`** - Retrieve all publications in the workspace that have a subscription for the specified email address. The workspace is determined by the provided API key.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
beehiiv-pp-cli advertisement-opportunities mock-value

# JSON for scripting and agents
beehiiv-pp-cli advertisement-opportunities mock-value --json

# Filter to specific fields
beehiiv-pp-cli advertisement-opportunities mock-value --json --select id,name,status

# Dry run — show the request without sending
beehiiv-pp-cli advertisement-opportunities mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
beehiiv-pp-cli advertisement-opportunities mock-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-beehiiv -g
```

Then invoke `/pp-beehiiv <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add beehiiv beehiiv-pp-mcp -e BEEHIIV_BEARER_AUTH=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/beehiiv-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BEEHIIV_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "beehiiv": {
      "command": "beehiiv-pp-mcp",
      "env": {
        "BEEHIIV_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
beehiiv-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/beehiiv-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BEEHIIV_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `beehiiv-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BEEHIIV_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
