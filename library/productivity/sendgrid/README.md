# Twilio SendGrid CLI

**Every Twilio SendGrid endpoint, plus offline suppression diffs, stats time-series rollups, and a template-variable linter no other tool has.**

Built from the official twilio/sendgrid-oai spec, sendgrid-pp-cli mirrors the full v3 surface (mail send, marketing, suppressions, stats, templates, subusers, IPs, webhooks) with offline SQLite, agent-native --json/--select/--csv output, and eight novel commands the existing SendGrid CLIs and MCP servers do not have. Suppression sync collapses the API's per-type format inconsistency into one table; templates lint catches silently-empty Handlebars variables before send; stats rollup turns flat API buckets into proper time-series.

Learn more at [Twilio SendGrid](https://support.sendgrid.com/hc/en-us).

Printed by [@natekettles](https://github.com/natekettles) (Nathan Kettles).

## Install

The recommended path installs both the `sendgrid-pp-cli` binary and the `pp-sendgrid` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install sendgrid
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install sendgrid --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install sendgrid --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install sendgrid --agent claude-code
npx -y @mvanhorn/printing-press-library install sendgrid --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/sendgrid/cmd/sendgrid-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sendgrid-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sendgrid --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sendgrid --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-sendgrid skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-sendgrid. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sendgrid-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SENDGRID_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sendgrid": {
      "command": "sendgrid-pp-mcp",
      "env": {
        "SENDGRID_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

sendgrid-pp-cli reads SENDGRID_API_KEY (Bearer token) from the environment. Use a restricted key for read-only auditing or a full-access key for sends. Subuser impersonation via the on-behalf-of header is supported on admin endpoints (mail/send excluded by SendGrid policy).

## Quick Start

```bash
# Verify auth and reachability
sendgrid-pp-cli doctor

# Mirror suppressions locally so diffs and joins are instant
sendgrid-pp-cli sync suppressions

# Audit drift between your CRM and SendGrid
sendgrid-pp-cli suppression diff bounces --against crm.csv --json

# Get a real time-series view without writing SQL
sendgrid-pp-cli stats rollup --by week --metric opens,clicks --window 30d --json

# Catch missing template variables before you send
sendgrid-pp-cli templates lint d-abc123 --against '{"first_name":"Sam"}' --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`suppression sync`** — Bidirectional sync between SendGrid suppressions and an external CSV/CRM source, with dry-run preview and unified schema across the four inconsistent suppression types.

  _When you need to reconcile SendGrid's suppression list with another system without writing custom glue code per suppression type._

  ```bash
  sendgrid-pp-cli suppression sync --from internal-suppressions.csv --apply --dry-run --json
  ```
- **`suppression diff`** — Three-way diff between the local SQLite mirror, live API, and an external CSV; shows adds, drops, and drift across the suppression universe.

  _When auditing CRM ↔ SendGrid drift before a cleanup pass or migration._

  ```bash
  sendgrid-pp-cli suppression diff bounces --against crm-export.csv --json
  ```
- **`stats rollup`** — Pulls flat windowed stats from the API, stores them in SQLite, and computes day/week/month rollups plus WoW/MoM deltas.

  _Use for trend reporting and weekly deliverability standups without paying for paid analytics._

  ```bash
  sendgrid-pp-cli stats rollup --by week --metric opens,clicks --window 90d --json
  ```
- **`templates diff`** — Side-by-side semantic diff of two template versions (HTML, plain text, subject, test data) with HTML-aware normalization.

  _Use during template review to see exactly what changed between draft versions._

  ```bash
  sendgrid-pp-cli templates diff d-abc123 v1 v2
  ```

### Pre-flight checks the API skips

- **`templates lint`** — Statically extracts {{handlebars}} from a template version and cross-checks them against a contact record or JSON payload, flagging missing or typo'd variables before send.

  _Use before any production send to catch silently-empty template variables that would otherwise ship as blanks._

  ```bash
  sendgrid-pp-cli templates lint d-abc123 --against '{"first_name":"Sam"}' --json
  ```

### Cross-endpoint joins

- **`bounce why`** — Joins suppressions, email activity, and stats locally to produce a narrative explaining why a specific address keeps bouncing.

  _When a customer reports they aren't getting emails and you need a fast root cause._

  ```bash
  sendgrid-pp-cli bounce why user@example.com --json
  ```
- **`subusers rollup`** — Fans out per-subuser stats pulls in parallel, caches them locally, and produces a single aggregated table for ESP operators managing tenant hierarchies.

  _Use for tenant-level deliverability reporting when you run subusers and need per-tenant numbers in one view._

  ```bash
  sendgrid-pp-cli subusers rollup --metric reputation,bounces --window 30d --json
  ```

### Rate-aware streaming

- **`activity tail`** — Streams Email Activity events with rate-limit-aware polling (respects the 6/min cap), local FTS, and filters by status/from/to.

  _Use during a deliverability incident to watch events as they happen without tripping the rate limit._

  ```bash
  sendgrid-pp-cli activity tail --filter status:bounce --json
  ```

## Usage

Run `sendgrid-pp-cli --help` for the full command reference and flag list.

## Commands

### access-settings

Manage access settings

- **`sendgrid-pp-cli access-settings add-ip-to-allow-list`** - **This endpoint allows you to add one or more allowed IP addresses.**

To allow one or more IP addresses, pass them to this endpoint in an array. Once an IP address is added to your allow list, it will be assigned an `id` that can be used to remove the address. You can retrieve the ID associated with an IP using the "Retrieve a list of currently allowed IPs" endpoint.
- **`sendgrid-pp-cli access-settings delete-allowed-ip`** - **This endpoint allows you to remove a specific IP address from your list of allowed addresses.**

When removing a specific IP address from your list, you must include the ID in your call.  You can retrieve the IDs associated with your allowed IP addresses using the "Retrieve a list of currently allowed IPs" endpoint.
- **`sendgrid-pp-cli access-settings delete-allowed-ips`** - **This endpoint allows you to remove one or more IP addresses from your list of allowed addresses.**

To remove one or more IP addresses, pass this endpoint an array containing the ID(s) associated with the IP(s) you intend to remove. You can retrieve the IDs associated with your allowed IP addresses using the "Retrieve a list of currently allowed IPs" endpoint.

It is possible to remove your own IP address, which will block access to your account. You will need to submit a [support ticket](https://sendgrid.com/docs/ui/account-and-settings/support/) if this happens. For this reason, it is important to double check that you are removing only the IPs you intend to remove when using this endpoint.
- **`sendgrid-pp-cli access-settings get-allowed-ip`** - **This endpoint allows you to retreive a specific IP address that has been allowed to access your account.**

You must include the ID for the specific IP address you want to retrieve in your call. You can retrieve the IDs associated with your allowed IP addresses using the "Retrieve a  list of currently allowed IPs" endpoint.
- **`sendgrid-pp-cli access-settings list-access-activity`** - **This endpoint allows you to retrieve a list of all of the IP addresses that recently attempted to access your account either through the User Interface or the API.**
- **`sendgrid-pp-cli access-settings list-allowed-ip`** - **This endpoint allows you to retrieve a list of IP addresses that are currently allowed to access your account.**

Each IP address returned to you will have `created_at` and `updated_at` dates. Each IP will also be associated with an `id` that can be used to remove the address from your allow list.

### alerts

Twilio SendGrid Alerts API.

- **`sendgrid-pp-cli alerts create`** - **This endpoint allows you to create a new alert.**
- **`sendgrid-pp-cli alerts delete`** - **This endpoint allows you to delete an alert.**
- **`sendgrid-pp-cli alerts get`** - **This endpoint allows you to retrieve a specific alert.**
- **`sendgrid-pp-cli alerts list`** - **This endpoint allows you to retrieve all of your alerts.**
- **`sendgrid-pp-cli alerts update`** - **This endpoint allows you to update an alert.**

### api-keys

Twilio SendGrid API Keys API.

- **`sendgrid-pp-cli api-keys create`** - **This endpoint allows you to create a new API Key for the user.**

To create your initial SendGrid API Key, you should [use the SendGrid App](https://app.sendgrid.com/settings/api_keys). Once you have created a first key with scopes to manage additional API keys, you can use this API for all other key management.
A JSON request body containing a `name` property is required when making requests to this endpoint. If the number of maximum keys, 100, is reached, a `403` status will be returned.
Though the `name` field is required, it does not need to be unique. A unique API key ID will be generated for each key you create and returned in the response body.
It is not necessary to pass a `scopes` field to the API when creating a key, but you should be aware that omitting the `scopes` field from your request will create a key with "Full Access" permissions by default.
See the [API Key Permissions List](https://docs.sendgrid.com/api-reference/how-to-use-the-sendgrid-v3-api/authorization) for all available scopes. An API key's scopes can be updated after creation using the "Update API keys" endpoint.
- **`sendgrid-pp-cli api-keys delete`** - **This endpoint allows you to revoke an existing API Key using an `api_key_id`**

Authentications using a revoked API Key will fail after after some small propogation delay. If the API Key ID does not exist, a `404` status will be returned.
- **`sendgrid-pp-cli api-keys get`** - **This endpoint allows you to retrieve a single API key using an `api_key_id`.**

The endpoint will return a key's name, ID, and scopes. If the API Key ID does not, exist a `404` status will be returned.

See the [API Key Permissions List](https://docs.sendgrid.com/api-reference/how-to-use-the-sendgrid-v3-api/authorization) for all available scopes. An API key's scopes can be updated after creation using the "Update API keys" endpoint.
- **`sendgrid-pp-cli api-keys list`** - **This endpoint allows you to retrieve all API Keys that belong to the authenticated user.**

A successful response from this API will include all available API keys' names and IDs.

For security reasons, there is not a way to retrieve the key itself after it's created. If you lose your API key, you must create a new one. Only the "Create API keys" endpoint will return a key to you and only at the time of creation.

An `api_key_id` can be used to update or delete the key, as well as retrieve the key's details, such as its scopes.
- **`sendgrid-pp-cli api-keys update`** - **This endpoint allows you to update the name and scopes of a given API key.**

You must pass this endpoint a JSON request body with a `name` field and a `scopes` array containing at least one scope. The `name` and `scopes` fields will be used to update the key associated with the `api_key_id` in the request URL.

If you need to update a key's scopes only, pass the `name` field with the key's existing name; the `name` will not be modified. If you need to update a key's name only, use the "Update API key name" endpoint.

See the [API Key Permissions List](https://docs.sendgrid.com/api-reference/how-to-use-the-sendgrid-v3-api/authorization) for all available scopes.
- **`sendgrid-pp-cli api-keys update-name`** - **This endpoint allows you to update the name of an existing API Key.**

You must pass this endpoint a JSON request body with a `name` property, which will be used to rename the key associated with the `api_key_id` passed in the URL.

### asm

Manage asm

- **`sendgrid-pp-cli asm add-suppression-to-group`** - **This endpoint allows you to add email addresses to an unsubscribe group.**

If you attempt to add suppressions to a group that has been deleted or does not exist, the suppressions will be added to the global suppressions list.
- **`sendgrid-pp-cli asm creat-group`** - **This endpoint allows you to create a new suppression group.**

To add an email address to the suppression group, [create a Suppression](https://docs.sendgrid.com/api-reference/suppressions-suppressions/add-suppressions-to-a-suppression-group).
- **`sendgrid-pp-cli asm create-global-suppression`** - **This endpoint allows you to add one or more email addresses to the global suppressions group.**
- **`sendgrid-pp-cli asm delete-global-suppression`** - **This endpoint allows you to remove an email address from the global suppressions group.**

Deleting a suppression group will remove the suppression, meaning email will once again be sent to the previously suppressed addresses. This should be avoided unless a recipient indicates they wish to receive email from you again. You can use our [bypass filters](https://sendgrid.com/docs/ui/sending-email/index-suppressions/#bypass-suppressions) to deliver messages to otherwise suppressed addresses when exceptions are required.
- **`sendgrid-pp-cli asm delete-group`** - **This endpoint allows you to delete a suppression group.**

If a recipient uses the "one-click unsubscribe" option on an email associated with a deleted group, that recipient will be added to the global suppression list.

Deleting a suppression group will remove the suppression, meaning email will once again be sent to the previously suppressed addresses. This should be avoided unless a recipient indicates they wish to receive email from you again. You can use our [bypass filters](https://sendgrid.com/docs/ui/sending-email/index-suppressions/#bypass-suppressions) to deliver messages to otherwise suppressed addresses when exceptions are required.
- **`sendgrid-pp-cli asm delete-suppression-from-group`** - **This endpoint allows you to remove a suppressed email address from the given suppression group.**

Removing an address will remove the suppression, meaning email will once again be sent to the previously suppressed addresses. This should be avoided unless a recipient indicates they wish to receive email from you again. You can use our [bypass filters](https://sendgrid.com/docs/ui/sending-email/index-suppressions/#bypass-suppressions) to deliver messages to otherwise suppressed addresses when exceptions are required.
- **`sendgrid-pp-cli asm get-global-suppression`** - **This endpoint allows you to retrieve a global suppression. You can also use this endpoint to confirm if an email address is already globally suppresed.**

If the email address you include in the URL path parameter `{email}` is already globally suppressed, the response will include that email address. If the address you enter for `{email}` is not globally suppressed, an empty JSON object `{}` will be returned.
- **`sendgrid-pp-cli asm get-group`** - **This endpoint allows you to retrieve a single suppression group.**
- **`sendgrid-pp-cli asm get-suppression`** - **This endpoint returns a list of all groups from which the given email address has been unsubscribed.**
- **`sendgrid-pp-cli asm list-group`** - **This endpoint allows you to retrieve a list of all suppression groups created by this user.**

This endpoint can also return information for multiple group IDs that you include in your request. To add a group ID to your request, simply append `?id=123456&id=123456`, with the appropriate group IDs.
- **`sendgrid-pp-cli asm list-suppression`** - **This endpoint allows you to retrieve a list of all suppressions.**
- **`sendgrid-pp-cli asm list-suppression-from-group`** - **This endpoint allows you to retrieve all suppressed email addresses belonging to the given group.**
- **`sendgrid-pp-cli asm search-suppression-from-group`** - **This endpoint allows you to search a suppression group for multiple suppressions.**

When given a list of email addresses and a group ID, this endpoint will only return the email addresses that have been unsubscribed from the given group.
- **`sendgrid-pp-cli asm update-group`** - **This endpoint allows you to update or change a suppression group.**

### browsers

Manage browsers

- **`sendgrid-pp-cli browsers`** - **This endpoint allows you to retrieve your email statistics segmented by browser type.**

**We only store up to 7 days of email activity in our database.** By default, 500 items will be returned per request via the Advanced Stats API endpoints.

Advanced Stats provide a more in-depth view of your email statistics and the actions taken by your recipients. You can segment these statistics by geographic location, device type, client type, browser, and mailbox provider. For more information about statistics, please see our [Statistics Overview](https://sendgrid.com/docs/ui/analytics-and-reporting/stats-overview/).

### campaigns

Manage campaigns

- **`sendgrid-pp-cli campaigns create`** - **This endpoint allows you to create a campaign.**

In order to send or schedule the campaign, you will be required to provide a subject, sender ID, content (we suggest both html and plain text), and at least one list or segment ID. This information is not required when you create a campaign.
- **`sendgrid-pp-cli campaigns delete`** - **This endpoint allows you to delete a specific campaign.**
- **`sendgrid-pp-cli campaigns get`** - **This endpoint allows you to retrieve a specific campaign.**
- **`sendgrid-pp-cli campaigns list`** - **This endpoint allows you to retrieve a paginated list of all of your campaigns.**

Returns campaigns in reverse order they were created (newest first).

Returns an empty array if no campaigns exist.

You can use the `limit` query parameter to set the page size. If your list contains more items than the page size permits, you can make multiple requests. Use the `offset` query parameter to control the position in the list from which to start retrieving additional items.
- **`sendgrid-pp-cli campaigns update`** - **This endpoint allows you to update a specific campaign.**

This is especially useful if you only set up the campaign using POST /campaigns, but didn't set many of the parameters.

### categories

Twilio SendGrid Category Stats API

- **`sendgrid-pp-cli categories list-category`** - **This endpoint allows you to retrieve a paginated list of all of your categories.**

You can use the `limit` query parameter to set the page size. If your list contains more items than the page size permits, you can make multiple requests. Use the `offset` query parameter to control the position in the list from which to start retrieving additional items.
- **`sendgrid-pp-cli categories list-category-stat`** - **This endpoint allows you to retrieve all of your email statistics for each of your categories.**

If you do not define any query parameters, this endpoint will return a sum for each category in groups of 10.
- **`sendgrid-pp-cli categories list-category-stat-sum`** - **This endpoint allows you to retrieve the total sum of each email statistic for every category over the given date range.**

If you do not define any query parameters, this endpoint will return a sum for each category in groups of 10.

### clients

Manage clients

- **`sendgrid-pp-cli clients`** - **This endpoint allows you to retrieve your email statistics segmented by client type.**

**We only store up to 7 days of email activity in our database.** By default, 500 items will be returned per request via the Advanced Stats API endpoints.

Advanced Stats provide a more in-depth view of your email statistics and the actions taken by your recipients. You can segment these statistics by geographic location, device type, client type, browser, and mailbox provider. For more information about statistics, please see our [Statistics Overview](https://sendgrid.com/docs/ui/analytics-and-reporting/stats-overview/).

### contactdb

Manage contactdb

- **`sendgrid-pp-cli contactdb add-recipient`** - **This endpoint allows you to add a Marketing Campaigns recipient.**

You can add custom field data as a parameter on this endpoint. We have provided an example using some of the default custom fields SendGrid provides.

The rate limit is three requests every 2 seconds. You can upload 1000  contacts per request. So the maximum upload rate is 1500 recipients per second.
- **`sendgrid-pp-cli contactdb add-recipient-to-contact-db-list`** - **This endpoint allows you to add a single recipient to a list.**
- **`sendgrid-pp-cli contactdb add-recipients-to-contact-db-list`** - **This endpoint allows you to add multiple recipients to a list.**

Adds existing recipients to a list, passing in the recipient IDs to add. Recipient IDs (base64-encoded email addresses) should be passed exactly as they are returned from recipient endpoints.
- **`sendgrid-pp-cli contactdb create-contact-db-list`** - **This endpoint allows you to create a list for your recipients.**
- **`sendgrid-pp-cli contactdb create-custom-field`** - **This endpoint allows you to create a custom field.**

**You can create up to 120 custom fields.**
- **`sendgrid-pp-cli contactdb create-segment`** - **This endpoint allows you to create a new segment.**

  Valid operators for create and update depend on the type of the field for which you are searching.

**Dates**
- "eq", "ne", "lt" (before), "gt" (after)
    - You may use MM/DD/YYYY for day granularity or an epoch for second granularity.
- "empty", "not_empty"
- "is within"
    - You may use an [ISO 8601 date format](https://en.wikipedia.org/wiki/ISO_8601) or the # of days.

**Text**
- "contains"
- "eq" (is/equals - matches the full field)
- "ne" (is not/not equals - matches any field where the entire field is not the condition value)
- "empty"
- "not_empty"

**Numbers**
- "eq" (is/equals)
- "lt" (is less than)
- "gt" (is greater than)
- "empty"
- "not_empty"

**Email Clicks and Opens**
- "eq" (opened)
- "ne" (not opened)

All field values must be a string.

Conditions using "eq" or "ne" for email clicks and opens should provide a "field" of either `clicks.campaign_identifier` or `opens.campaign_identifier`.
The condition value should be a string containing the id of a completed campaign.

The conditions list may contain multiple conditions, joined by an "and" or "or" in the "and_or" field.

The first condition in the conditions list must have an empty "and_or", and subsequent conditions must all specify an "and_or".
- **`sendgrid-pp-cli contactdb delete-contact-db-list`** - **This endpoint allows you to delete a specific recipient list with the given ID.**
- **`sendgrid-pp-cli contactdb delete-contact-db-lists`** - **This endpoint allows you to delete multiple recipient lists.**
- **`sendgrid-pp-cli contactdb delete-custom-field`** - **This endpoint allows you to delete a custom field by ID.**
- **`sendgrid-pp-cli contactdb delete-recipient`** - **This endpoint allows you to delete a single recipient with the given ID from your contact database.**

> Use this to permanently delete your recipients from all of your contact lists and all segments if required by applicable law.
- **`sendgrid-pp-cli contactdb delete-recipient-from-contact-db-list`** - **This endpoint allows you to delete a single recipient from a list.**
- **`sendgrid-pp-cli contactdb delete-recipients`** - **This endpoint allows you to deletes one or more recipients.**

The body of an API call to this endpoint must include an array of recipient IDs of the recipients you want to delete.
- **`sendgrid-pp-cli contactdb delete-segment`** - **This endpoint allows you to delete a segment from your recipients database.**

You also have the option to delete all the contacts from your Marketing Campaigns recipient database who were in this segment.
- **`sendgrid-pp-cli contactdb export-recipient`** - **Use this endpoint to export lists or segments of recipients**.

If you would just like to have a link to the exported list sent to your email set the `notifications.email` option to `true` in the `POST` payload.

If you would like to download the list, take the `id` that is returned and use the "Export Recipients Status" endpoint to get the `urls`. Once you have the list of URLs, make a `GET` request to each URL provided to download your CSV file(s).

You specify the segments and or/recipient lists you wish to export by providing the relevant IDs in, respectively, the `segment_ids` and `list_ids` fields in the request body.

The lists will be provided in either JSON or CSV files. To specify which of these you would required, set the request body `file_type` field to `json` or `csv`.

You can also specify a maximum file size (in MB). If the export file is larger than this, it will be split into multiple files.
- **`sendgrid-pp-cli contactdb get-billable`** - **This endpoint allows you to retrieve the number of Marketing Campaigns recipients that you will be billed for.**

You are billed for marketing campaigns based on the highest number of recipients you have had in your account at one time. This endpoint will allow you to know the current billable count value.
- **`sendgrid-pp-cli contactdb get-contact-db-list`** - **This endpoint allows you to retrieve a single recipient list.**
- **`sendgrid-pp-cli contactdb get-custom-field`** - **This endpoint allows you to retrieve a custom field by ID.**
- **`sendgrid-pp-cli contactdb get-export-recipient`** - **This endpoint can be used to check the status of a recipient export job**. 

To use this call, you will need the `id` from the "Export Recipients" call.

If you would like to download a list, take the `id` that is returned from the "Export Recipients" endpoint and make an API request here to get the `urls`. Once you have the list of URLs, make a `GET` request on each URL to download your CSV file(s).

Twilio SendGrid recommends exporting your recipients regularly as a backup to avoid issues or lost data.
- **`sendgrid-pp-cli contactdb get-recipient`** - **This endpoint allows you to retrieve a single recipient by ID from your contact database.**
- **`sendgrid-pp-cli contactdb get-recipient-list`** - **This endpoint allows you to retrieve the lists that a given recipient belongs to.**

Each recipient can be on many lists. This endpoint gives you all of the lists that any one recipient has been added to.
- **`sendgrid-pp-cli contactdb get-segment`** - **This endpoint allows you to retrieve a single segment with the given ID.**
- **`sendgrid-pp-cli contactdb list-contact-db-list`** - **This endpoint allows you to retrieve all of your recipient lists. If you don't have any lists, an empty array will be returned.**
- **`sendgrid-pp-cli contactdb list-custom-field`** - **This endpoint allows you to retrieve all custom fields.**
- **`sendgrid-pp-cli contactdb list-export-recipient`** - **Use this endpoint to retrieve details of all current exported jobs**.

It will return an array of objects, each of which records an export job in flight or recently completed. 

Each object's `export_type` field will tell you which kind of export it is and its `status` field will indicate what stage of processing it has reached. Exports which are `ready` will be accompanied by a `urls` field which lists the URLs of the export's downloadable files — there will be more than one if you specified a maximum file size in your initial export request.

Use this endpoint if you have exports in flight but do not know their IDs, which are required for the "Export Recipients Status" endpoint.
- **`sendgrid-pp-cli contactdb list-recipient`** - **This endpoint allows you to retrieve all of your Marketing Campaigns recipients.**

Batch deletion of a page makes it possible to receive an empty page of recipients before reaching the end of
the list of recipients. To avoid this issue; iterate over pages until a 404 is retrieved.
- **`sendgrid-pp-cli contactdb list-recipient-count`** - **This endpoint allows you to retrieve the total number of Marketing Campaigns recipients.**
- **`sendgrid-pp-cli contactdb list-recipient-for-segment`** - **This endpoint allows you to retrieve all of the recipients in a segment with the given ID.**
- **`sendgrid-pp-cli contactdb list-recipients-from-contact-db-list`** - **This endpoint allows you to retrieve all recipients on the list with the given ID.**
- **`sendgrid-pp-cli contactdb list-reserved-field`** - **This endpoint allows you to list all fields that are reserved and can't be used for custom field names.**
- **`sendgrid-pp-cli contactdb list-search-recipient`** - **This endpoint allows you to perform a search on all of your Marketing Campaigns recipients.**

field_name:

* is a variable that is substituted for your actual custom field name from your recipient.
* Text fields must be url-encoded. Date fields are searchable only by unix timestamp (e.g. 2/2/2015 becomes 1422835200)
* If field_name is a 'reserved' date field, such as created_at or updated_at, the system will internally convert
your epoch time to a date range encompassing the entire day. For example, an epoch time of 1422835600 converts to
Mon, 02 Feb 2015 00:06:40 GMT, but internally the system will search from Mon, 02 Feb 2015 00:00:00 GMT through
Mon, 02 Feb 2015 23:59:59 GMT.
- **`sendgrid-pp-cli contactdb list-segment`** - **This endpoint allows you to retrieve all of your segments.**
- **`sendgrid-pp-cli contactdb list-status`** - **This endpoint allows you to check the upload status of a Marketing Campaigns recipient.**
- **`sendgrid-pp-cli contactdb search-recipient`** - Search using segment conditions without actually creating a segment.
Body contains a JSON object with `conditions`, a list of conditions as described below, and an optional `list_id`, which is a valid list ID for a list to limit the search on.

Valid operators for create and update depend on the type of the field for which you are searching.

- Dates:
  - `"eq"`, `"ne"`, `"lt"` (before), `"gt"` (after)
    - You may use MM/DD/YYYY for day granularity or an epoch for second granularity.
  - `"empty"`, `"not_empty"`
  - `"is within"`
    - You may use an [ISO 8601](https://en.wikipedia.org/wiki/ISO_8601) date format or the # of days.
- Text: `"contains"`, `"eq"` (is - matches the full field), `"ne"` (is not - matches any field where the entire field is not the condition value), `"empty"`, `"not_empty"`
- Numbers: `"eq"`, `"lt"`, `"gt"`, `"empty"`, `"not_empty"`
- Email Clicks and Opens: `"eq"` (opened), `"ne"` (not opened)

Field values must all be a string.

Search conditions using `"eq"` or `"ne"` for email clicks and opens should provide a "field" of either `clicks.campaign_identifier` or `opens.campaign_identifier`.

The condition value should be a string containing the id of a completed campaign.

Search conditions list may contain multiple conditions, joined by an `"and"` or `"or"` in the `"and_or"` field.

The first condition in the conditions list must have an empty `"and_or"`, and subsequent conditions must all specify an `"and_or"`.
- **`sendgrid-pp-cli contactdb update-contact-db-list`** - **This endpoint allows you to update the name of one of your recipient lists.**
- **`sendgrid-pp-cli contactdb update-recipient`** - **This endpoint allows you to update one or more recipients.**

The body of an API call to this endpoint must include an array of one or more recipient objects.

It is of note that you can add custom field data as parameters on recipient objects. We have provided an example using some of the default custom fields SendGrid provides.
- **`sendgrid-pp-cli contactdb update-segment`** - **This endpoint allows you to update a segment.**

### designs

Twilio SendGrid Marketing Campaigns Designs API

- **`sendgrid-pp-cli designs create`** - **This endpoint allows you to create a new design**.

You can add a new design by passing data, including a string of HTML email content, to `/designs`. When creating designs from scratch, be aware of the styling constraints inherent to many email clients. For a list of best practices, see our guide to [Cross-Platform Email Design](https://sendgrid.com/docs/ui/sending-email/cross-platform-html-design/).

The Design Library can also convert your design’s HTML elements into drag and drop modules that are editable in the Designs Library user interface. For more, visit the [Design and Code Editor documentation](https://sendgrid.com/docs/ui/sending-email/editor/#drag--drop-markup).

Because the `/designs` endpoint makes it easy to add designs, you can create a design with your preferred tooling or migrate designs you already own without relying on the Design Library UI.
- **`sendgrid-pp-cli designs delete`** - **This endpoint allows you to delete a single design**.

Be sure to check the ID of the design you intend to delete before making this request; deleting a design is a permanent action.
- **`sendgrid-pp-cli designs duplicate`** - **This endpoint allows you to duplicate one of your existing designs**.

Modifying an existing design is often the easiest way to create something new.

You are not required to pass any data in the body of a request to this endpoint. If you choose to leave the `name` field blank, your duplicate will be assigned the name of the design it was copied from with the text "Duplicate: " prepended to it. This name change is only a convenience, as the duplicate will be assigned a unique ID that differentiates it from your other designs.

You can modify your duplicate’s name at the time of creation by passing an updated value to the `name` field when making the initial request.
More on retrieving design IDs can be found below.
- **`sendgrid-pp-cli designs duplicate-pre-built`** - **This endpoint allows you to duplicate one of the pre-built Twilio SendGrid designs**.

Like duplicating one of your existing designs, you are not required to pass any data in the body of a request to this endpoint. If you choose to leave the `name` field blank, your duplicate will be assigned the name of the design it was copied from with the text "Duplicate: " prepended to it. This name change is only a convenience, as the duplicate design will be assigned a unique ID that differentiates it from your other designs. You can retrieve the IDs for Twilio SendGrid pre-built designs using the "List SendGrid Pre-built Designs" endpoint.

You can modify your duplicate’s name at the time of creation by passing an updated value to the `name` field when making the initial request.
More on retrieving design IDs can be found above.
- **`sendgrid-pp-cli designs get`** - **This endpoint allows you to retrieve a single design**.

A GET request to `/designs/{id}` will retrieve details about a specific design in your Design Library.

This endpoint is valuable when retrieving information stored in a field that you wish to update using a PATCH request.
- **`sendgrid-pp-cli designs get-pre-built`** - **This endpoint allows you to retrieve a single pre-built design**.

A GET request to `/designs/pre-builts/{id}` will retrieve details about a specific pre-built design.

This endpoint is valuable when retrieving details about a pre-built design that you wish to duplicate and modify.
- **`sendgrid-pp-cli designs list`** - **This endpoint allows you to retrieve a list of designs already stored in your Design Library**.

A GET request to `/designs` will return a list of your existing designs. This endpoint will not return the pre-built Twilio SendGrid designs. Pre-built designs can be retrieved using the `/designs/pre-builts` endpoint, which is detailed below.

By default, you will receive 100 results per request; however, you can modify the number of results returned by passing an integer to the `page_size` query parameter.
- **`sendgrid-pp-cli designs list-pre-built`** - **This endpoint allows you to retrieve a list of pre-built designs provided by Twilio SendGrid**.

Unlike the `/designs` endpoint where *your* designs are stored, a GET request made to `designs/pre-builts` will retrieve a list of the pre-built Twilio SendGrid designs. This endpoint will not return the designs stored in your Design Library.

By default, you will receive 100 results per request; however, you can modify the number of results returned by passing an integer to the `page_size` query parameter.

This endpoint is useful for retrieving the IDs of Twilio SendGrid designs that you want to duplicate and modify.
- **`sendgrid-pp-cli designs update`** - **This endpoint allows you to edit a design**.

The Design API supports PATCH requests, which allow you to make partial updates to a single design. Passing data to a specific field will update only the data stored in that field; all other fields will be unaltered.

For example, updating a design's name requires that you make a PATCH request to this endpoint with data specified for the `name` field only.

```
{
    "name": "<Updated Name>"
}
```

### devices

Manage devices

- **`sendgrid-pp-cli devices`** - **This endpoint allows you to retrieve your email statistics segmented by the device type.**

**We only store up to 7 days of email activity in our database.** By default, 500 items will be returned per request via the Advanced Stats API endpoints.

## Available Device Types
| **Device** | **Description** | **Example** |
|---|---|---|
| Desktop | Email software on desktop computer. | I.E., Outlook, Sparrow, or Apple Mail. |
| Webmail |	A web-based email client. | I.E., Yahoo, Google, AOL, or Outlook.com. |
| Phone | A smart phone. | iPhone, Android, Blackberry, etc.
| Tablet | A tablet computer. | iPad, android based tablet, etc. |
| Other | An unrecognized device. |

Advanced Stats provide a more in-depth view of your email statistics and the actions taken by your recipients. You can segment these statistics by geographic location, device type, client type, browser, and mailbox provider. For more information about statistics, please see our [Statistics Overview](https://sendgrid.com/docs/ui/analytics-and-reporting/stats-overview/).

### engagementquality

Manage engagementquality

- **`sendgrid-pp-cli engagementquality list-engagement-quality-score`** - **This operation allows you to retrieve your SendGrid Engagement Quality (SEQ) scores for a specified date range**.
A successful request with this API operation will return either a `200` or `202` response.
### 202
This operation returns a `202` response when SendGrid does not yet have scores available for the specified date range. Scores are calculated asynchronously from requests to this endpoint. This means a score may be available for the specified date at a later time, but a score is not available at the time of your API request.
### 200
A 200 response will include all available scores beginning on the `from` and ending on the `to` dates specified. The `score` and `metrics` properties will be omitted from the response for any days in which the user is not eligible to receive a score.
The `score` property represents a user's overall engagement quality. The `metrics` property provides additional scores for the input categories that contribute to that overall score. All scores range from `1` to `5` with a higher number representing better engagement quality.
See [**SendGrid Engagement Quality Overview**](https://docs.sendgrid.com/api-reference/sendgrid-engagement-quality-api/overview) for more information
- **`sendgrid-pp-cli engagementquality list-subuser-engagement-quality-score`** - **This operation allows you to retrieve SendGrid Engagement Quality (SEQ) scores for your Subusers or customer accounts for a specific date.**
A successful request with this API operation will return either a `200` or `202` response.
### 202
This operation returns a `202` response when SendGrid does not yet have scores available for the specified date range. Scores are calculated asynchronously from requests to this endpoint. This means a score may be available for the specified date at a later time, but a score is not available at the time of your API request.
### 200
A `200` response will include scores for all Subusers or customer accounts belonging to the requesting parent or reseller account. The `score` and `metrics` properties will be omitted from the response if a Subuser or customer account is not eligible to receive a score for the specified date.
The `score` property represents a Subuser or customer account's overall engagement quality. The `metrics` property provides additional scores for the input categories that contribute to that overall score. All scores range from `1` to `5` with a higher number representing better engagement quality.
See [**SendGrid Engagement Quality Overview**](https://docs.sendgrid.com/api-reference/sendgrid-engagement-quality-api/overview) for more information

### geo

Manage geo

- **`sendgrid-pp-cli geo`** - **This endpoint allows you to retrieve your email statistics segmented by country and state/province.**

**We only store up to 7 days of email activity in our database.** By default, 500 items will be returned per request via the Advanced Stats API endpoints.

For Regional (EU) subusers, this service is not available due to PII restrictions.

Advanced Stats provide a more in-depth view of your email statistics and the actions taken by your recipients. You can segment these statistics by geographic location, device type, client type, browser, and mailbox provider. For more information about statistics, please see our [User Guide](https://wwww.twilio.com/docs/sendgrid/ui/analytics-and-reporting/stats-overview).

### ips

Manage ips

- **`sendgrid-pp-cli ips add`** - **This endpoint is for adding a(n) IP Address(es) to your account.**
- **`sendgrid-pp-cli ips add-to-pool`** - **This endpoint allows you to add an IP address to an IP pool.**

You can add the same IP address to multiple pools. It may take up to 60 seconds for your IP address to be added to a pool after your request is made.

Before you can add an IP to a pool, you need to activate it in your SendGrid account:

1. Log into your SendGrid account.  
1. Navigate to **Settings** and then select **IP Addresses**.  
1. Find the IP address you want to activate and then click **Edit**.  
1. Check **Allow my account to send mail using this IP address**.
1. Click **Save**.

You can retrieve all of your available IP addresses from the "Retrieve all IP addresses" endpoint.
- **`sendgrid-pp-cli ips create-pool`** - **This endpoint allows you to create an IP pool.**

Before you can create an IP pool, you need to activate the IP in your SendGrid account: 

1. Log into your SendGrid account.  
1. Navigate to **Settings** and then select **IP Addresses**.  
1. Find the IP address you want to activate and then click **Edit**.  
1. Check **Allow my account to send mail using this IP address**.
1. Click **Save**.
- **`sendgrid-pp-cli ips delete-from-pool`** - **This endpoint allows you to remove an IP address from an IP pool.**
- **`sendgrid-pp-cli ips delete-pool`** - **This endpoint allows you to delete an IP pool.**
- **`sendgrid-pp-cli ips get`** - **This endpoint allows you to see which IP pools a particular IP address has been added to.**

The same IP address can be added to multiple IP pools.

A single IP address or a range of IP addresses may be dedicated to an account in order to send email for multiple domains. The reputation of this IP is based on the aggregate performance of all the senders who use it.
- **`sendgrid-pp-cli ips get-pool`** - **This endpoint allows you to get all of the IP addresses that are in a specific IP pool.**
- **`sendgrid-pp-cli ips get-warm-up`** - **This endpoint allows you to retrieve the warmup status for a specific IP address.**

You can retrieve all of your warming IPs using the "Retrieve all IPs currently in warmup" endpoint.
- **`sendgrid-pp-cli ips list`** - **This endpoint allows you to retrieve a paginated list of all assigned and unassigned IPs.**

Response includes warm up status, pools, assigned subusers, and reverse DNS info. The start_date field corresponds to when warmup started for that IP.

A single IP address or a range of IP addresses may be dedicated to an account in order to send email for multiple domains. The reputation of this IP is determined by the aggregate performance of all email traffic sent from it.

You can use the `limit` query parameter to set the page size. If your list contains more items than the page size permits, you can make multiple requests. Use the `offset` query parameter to control the position in the list from which to start retrieving additional items.
- **`sendgrid-pp-cli ips list-assigned`** - **This endpoint allows you to retrieve only assigned IP addresses.**

A single IP address or a range of IP addresses may be dedicated to an account in order to send email for multiple domains. The reputation of this IP is based on the aggregate performance of all the senders who use it.
- **`sendgrid-pp-cli ips list-pool`** - **This endpoint allows you to get all of your IP pools.**
- **`sendgrid-pp-cli ips list-remaining-count`** - **This endpoint gets amount of IP Addresses that can still be created during a given period and the price of those IPs.**
- **`sendgrid-pp-cli ips list-warm-up`** - **This endpoint allows you to retrieve all of your IP addresses that are currently warming up.**
- **`sendgrid-pp-cli ips stop-warm-up`** - **This endpoint allows you to remove an IP address from warmup mode.**

Your request will return a 204 status code if the specified IP was successfully removed from warmup mode. To retrieve details of the IP’s warmup status *before* removing it from warmup mode, call the  "Retrieve the warmpup status for a specific IP address" endpoint.
- **`sendgrid-pp-cli ips update-pool`** - **This endpoint allows you to update the name of an IP pool.**
- **`sendgrid-pp-cli ips warm-up`** - **This endpoint allows you to put an IP address into warmup mode.**

### logs

Manage logs

- **`sendgrid-pp-cli logs get-message-by-id`** - Get all of the details about the specified message.
- **`sendgrid-pp-cli logs list-messages-by-filter`** - List recent messages within Email Logs, or search for messages using a filter.

### mail

Manage mail

- **`sendgrid-pp-cli mail create-batch`** - **This operation allows you to generate a new mail batch ID.**

Once a batch ID is created, you can associate it with a mail send by passing
it in the request body of the [Mail Send operation](https://docs.sendgrid.com/api-reference/mail-send/mail-send).
This makes it possible to group multiple requests to the Mail Send operation
by assigning them the same batch ID.

A batch ID that's associated with a mail send can be used to access and modify the associated send. For example, you can pause or cancel a send using its batch ID. See the [Scheduled Sends API](https://www.twilio.com/docs/sendgrid/api-reference/cancel-scheduled-sends) for more information about pausing and cancelling a mail send.
- **`sendgrid-pp-cli mail get-batch`** - **This operation allows you to validate a mail batch ID.**

If you provide a valid batch ID, this operation will return a `200` status code and the batch ID itself.
If you provide an invalid batch ID, you will receive a `400` level status code and an error message.
A batch ID does not need to be assigned to a send to be considered valid. A successful response means only that the batch ID has been created, but it does not indicate that the ID has been assigned to a send.
- **`sendgrid-pp-cli mail send`** - *The Mail Send operation allows you to send email over SendGrid's v3 Web API*

For an overview of this API, including its features and limitations, please see the [Mail Send API overview page](https://www.twilio.com/docs/sendgrid/api-reference/mail-send)

The overview page also includes links to SendGrid's Email API quickstarts and helper libraries to get you working with this endpoint even faster.

### mail-settings

Twilio SendGrid Mail Settings API

- **`sendgrid-pp-cli mail-settings list`** - **This endpoint allows you to retrieve a paginated list of all mail settings.**

Each setting will be returned with an `enabled` status set to `true` or `false` and a short description that explains what the setting does.

You can use the `limit` query parameter to set the page size. If your list contains more items than the page size permits, you can make multiple requests. Use the `offset` query parameter to control the position in the list from which to start retrieving additional items.
- **`sendgrid-pp-cli mail-settings list-address-whitelist`** - **This endpoint allows you to retrieve your current email address whitelist settings.**

The Address Whitelist setting allows you to specify email addresses or domains for which mail should never be suppressed.

For Regional Subusers - Utilizing this feature for Regional (EU) subusers will cause customer personal information to be stored outside of the EU.

For example, if you own the domain `example.com`, and one or more of your recipients use `email@example.com` addresses, placing `example.com` in the address whitelist setting instructs Twilio SendGrid to ignore all bounces, blocks, and unsubscribes logged for that domain. In other words, all bounces, blocks, and unsubscribes will still be sent to `example.com` as if they were sent under normal sending conditions.
- **`sendgrid-pp-cli mail-settings list-bounce-purge`** - **This endpoint allows you to retrieve your current Bounce Purge mail settings.**

The Bounce Purge mail setting allows you to configure the maximum age of contacts in your hard and soft bounce suppressions lists. All contacts older than their respective configured age are deleted.

A hard bounce occurs when an email message has been returned to the sender because the recipient's address is invalid. A hard bounce might occur because the domain name doesn't exist or because the recipient is unknown.

A soft bounce occurs when an email message reaches the recipient's mail server but is bounced back undelivered before it actually reaches the recipient. A soft bounce might occur because the recipient's inbox is full.

You can also manage this setting in the [Mail Settings section of the Twilio SendGrid App](https://app.sendgrid.com/settings/mail_settings). You can manage your bounces manually using the [Bounces API](https://docs.sendgrid.com/api-reference/bounces-api) or the [Bounces menu in the Twilio SendGrid App](https://app.sendgrid.com/suppressions/bounces).
- **`sendgrid-pp-cli mail-settings list-footer`** - **This endpoint allows you to retrieve your current Footer mail settings.**

The Footer setting will insert a custom footer at the bottom of your text and HTML email message bodies.

You can insert your HTML or plain text directly using the "Update footer mail settings" endpoint, or you can create the footer using the [Mail Settings menu in the Twilio SendGrid App](https://app.sendgrid.com/settings/mail_settings).
- **`sendgrid-pp-cli mail-settings list-forward-bounce`** - **This endpoint allows you to retrieve your current bounce forwarding mail settings.**

Enabling the Forward Bounce setting allows you to specify `email` addresses to which bounce reports will be forwarded. This endpoint returns the email address you have set to receive forwarded bounces and an `enabled` status indicating if the setting is active.
- **`sendgrid-pp-cli mail-settings list-forward-spam`** - **This endpoint allows you to retrieve your current Forward Spam mail settings.**

Enabling the Forward Spam setting allows you to specify `email` addresses to which spam reports will be forwarded. This endpoint returns any email address(es) you have set to receive forwarded spam and an `enabled` status indicating if the setting is active.
- **`sendgrid-pp-cli mail-settings list-template`** - **This endpoint allows you to retrieve your current legacy email template settings.**

This setting refers to our original email templates. We currently support more fully featured [Dynamic Transactional Templates](https://sendgrid.com/docs/ui/sending-email/how-to-send-an-email-with-dynamic-transactional-templates/).

The legacy email template setting wraps an HTML template around your email content. This can be useful for sending out marketing email and/or other HTML formatted messages. For instructions on using legacy templates, see how to ["Create and Edit Legacy Transactional Templates](https://sendgrid.com/docs/ui/sending-email/create-and-edit-legacy-transactional-templates/). For help migrating to our current template system, see ["Migrating from Legacy Templates"](https://sendgrid.com/docs/ui/sending-email/migrating-from-legacy-templates/).
- **`sendgrid-pp-cli mail-settings update-address-whitelist`** - **This endpoint allows you to update your current email address whitelist settings.**

For Regional Subusers - Utilizing this feature for Regional (EU) subusers will cause customer personal information to be stored outside of the EU.

You can select whether or not this setting should be enabled by assigning the `enabled` field a `true` or `false` value.

Passing only the `enabled` field to this endpoint will not alter your current `list` of whitelist entries. However, any modifications to your `list` of entries will overwrite the entire list. For this reason, you must included all existing entries you wish to retain in your `list` in addition to any new entries you intend to add. To remove one or more `list` entries, pass a `list` with only the entries you wish to retain.

You should not add generic domains such as `gmail.com` or `yahoo.com`  in your `list` because your emails will not honor recipients' unsubscribes. This may cause a legal violation of [CAN-SPAM](https://sendgrid.com/docs/glossary/can-spam/) and could damage your sending reputation.

The Address Whitelist setting allows you to specify email addresses or domains for which mail should never be suppressed.

For example, if you own the domain `example.com`, and one or more of your recipients use `email@example.com` addresses, placing `example.com` in the address whitelist setting instructs Twilio SendGrid to ignore all bounces, blocks, and unsubscribes logged for that domain. In other words, all bounces, blocks, and unsubscribes will still be sent to `example.com` as if they were sent under normal sending conditions.
- **`sendgrid-pp-cli mail-settings update-bounce-purge`** - **This endpoint allows you to update your current Bounce Purge mail settings.**

The Bounce Purge mail metting allows you to configure the maximum age of contacts in your hard and soft bounce suppressions lists. All contacts older than their respective configured age are deleted.

A hard bounce occurs when an email message has been returned to the sender because the recipient's address is invalid. A hard bounce might occur because the domain name doesn't exist or because the recipient is unknown.

A soft bounce occurs when an email message reaches the recipient's mail server but is bounced back undelivered before it actually reaches the recipient. A soft bounce might occur because the recipient's inbox is full.

You can also manage this setting in the [Mail Settings section of the Twilio SendGrid App](https://app.sendgrid.com/settings/mail_settings). You can manage your bounces manually using the [Bounces API](https://docs.sendgrid.com/api-reference/bounces-api) or the [Bounces menu in the Twilio SendGrid App](https://app.sendgrid.com/suppressions/bounces).
- **`sendgrid-pp-cli mail-settings update-footer`** - **This endpoint allows you to update your current Footer mail settings.**

The Footer setting will insert a custom footer at the bottom of your text and HTML email message bodies.

You can insert your HTML or plain text directly using this endpoint, or you can create the footer using the [Mail Settings menu in the Twilio SendGrid App](https://app.sendgrid.com/settings/mail_settings).
- **`sendgrid-pp-cli mail-settings update-forward-bounce`** - **This endpoint allows you to update your current bounce forwarding mail settings.**

Enabling the Forward Bounce setting allows you to specify an `email` address to which bounce reports will be forwarded.

You can also configure the Forward Spam mail settings in the [Mail Settings section of the Twilio SendGrid App](https://app.sendgrid.com/settings/mail_settings).
- **`sendgrid-pp-cli mail-settings update-forward-spam`** - **This endpoint allows you to update your current Forward Spam mail settings.**

Enabling the Forward Spam setting allows you to specify `email` addresses to which spam reports will be forwarded. You can set multiple addresses by passing this endpoint a comma separated list of emails in a single string.

```
{
  "email": "alerts@example.com",
  "enabled": true
}
```

The Forward Spam setting may also be used to receive emails sent to `abuse@` and `postmaster@` role addresses if you have authenticated your domain.

For example, if you authenticated `example.com` as your root domain and set a custom return path of `sub` for that domain, you could turn on Forward Spam, and any emails sent to `abuse@sub.example.com` or `postmaster@sub.example.com` would be forwarded to the email address you entered in the `email` field.

You can authenticate your domain using the "Authenticate a domain" endpoint or in the [Sender Authentication section of the Twilio SendGrid App](https://app.sendgrid.com/settings/sender_auth). You can also configure the Forward Spam mail settings in the [Mail Settings section of the Twilio SendGrid App](https://app.sendgrid.com/settings/mail_settings).
- **`sendgrid-pp-cli mail-settings update-template`** - **This endpoint allows you to update your current legacy email template settings.**

This setting refers to our original email templates. We currently support more fully featured [Dynamic Transactional Templates](https://sendgrid.com/docs/ui/sending-email/how-to-send-an-email-with-dynamic-transactional-templates/).

The legacy email template setting wraps an HTML template around your email content. This can be useful for sending out marketing email and/or other HTML formatted messages. For instructions on using legacy templates, see how to ["Create and Edit Legacy Transactional Templates](https://sendgrid.com/docs/ui/sending-email/create-and-edit-legacy-transactional-templates/). For help migrating to our current template system, see ["Migrating from Legacy Templates"](https://sendgrid.com/docs/ui/sending-email/migrating-from-legacy-templates/).

### mailbox-providers

Manage mailbox providers

- **`sendgrid-pp-cli mailbox-providers`** - **This endpoint allows you to retrieve your email statistics segmented by recipient mailbox provider.**

**We only store up to 7 days of email activity in our database.** By default, 500 items will be returned per request via the Advanced Stats API endpoints.

Advanced Stats provide a more in-depth view of your email statistics and the actions taken by your recipients. You can segment these statistics by geographic location, device type, client type, browser, and mailbox provider. For more information about statistics, please see our [Statistics Overview](https://sendgrid.com/docs/ui/analytics-and-reporting/stats-overview/).

### marketing

Manage marketing

- **`sendgrid-pp-cli marketing add-integration`** - This endpoint creates an Integration for email event forwarding. Each Integration has a maximum number of allowed Integration instances per user. For example, users can create up to 10 Segment Integrations.
- **`sendgrid-pp-cli marketing create-field-definition`** - **This endpoint creates a new custom field definition.**

Custom field definitions are created with the given `name` and `field_type`. Although field names are stored in a case-sensitive manner, all field names must be case-insensitively unique. This means you may create a field named `CamelCase` or `camelcase`, but not both. Additionally, a Custom Field name cannot collide with any Reserved Field names. You should save the returned `id` value in order to update or delete the field at a later date. You can have up to 500 custom fields.

The custom field name should be created using only alphanumeric characters (A-Z and 0-9) and underscores (\_). Custom fields can only begin with letters  A-Z or underscores (_). The field type can be date, text, or number fields. The field type is important for creating segments from your contact database.

**Note: Creating a custom field that begins with a number will cause issues with sending in Marketing Campaigns.**
- **`sendgrid-pp-cli marketing create-list`** - **This endpoint creates a new contacts list.**

Once you create a list, you can use the UI to [trigger an automation](https://sendgrid.com/docs/ui/sending-email/getting-started-with-automation/#create-an-automation) every time you add a new contact to the list.

A link to the newly created object is in `_metadata`.
- **`sendgrid-pp-cli marketing create-segment`** - Segment `name` has to be unique. A user can not create a new segment with an existing segment name.
- **`sendgrid-pp-cli marketing create-sender`** - **This endpoint allows you to create a new Sender.**

*You may create up to 100 unique Senders.*

Senders are required to be verified before use. If your domain has been authenticated, a new Sender will auto verify on creation. Otherwise an email will be sent to the `from.email`.
- **`sendgrid-pp-cli marketing create-single-send`** - **This endpoint allows you to create a new Single Send.**

Please note that if you are migrating from the previous version of Single Sends, you no longer need to pass a template ID with your request to this endpoint. Instead, you will pass all template data in the `email_config` object.

This endpoint will create a draft of the Single Send but will not send it or schedule it to be sent. Any `send_at` property value set with this endpoint will prepopulate the Single Send's send date. However, the Single Send will remain an unscheduled draft until it's updated with the [**Schedule Single Send**](https://docs.sendgrid.com/api-reference/single-sends/schedule-single-send) endpoint or SendGrid application UI.
- **`sendgrid-pp-cli marketing delete-contact`** - **This endpoint can be used to delete one or more contacts**.

The query parameter `ids` must set to a comma-separated list of contact IDs for bulk contact deletion.

The query parameter `delete_all_contacts` must be set to `"true"` to delete **all** contacts. 

You must set either `ids` or `delete_all_contacts`.

Deletion jobs are processed asynchronously.

Twilio SendGrid recommends exporting your contacts regularly as a backup to avoid issues or lost data.
- **`sendgrid-pp-cli marketing delete-contact-identifier`** - **This endpoint can be used to delete one identifier from a contact.**

Deletion jobs are processed asynchronously.

Note this is different from deleting a contact. If the contact has only one identifier, the asynchronous request will fail. All contacts are required to have at least one identifier.

The request body field `identifier_type` must have a valid value of "EMAIL", "PHONENUMBERID", "EXTERNALID", or "ANONYMOUSID".
- **`sendgrid-pp-cli marketing delete-contact-lists`** - **This endpoint allows you to remove contacts from a given list.**

The contacts will not be deleted. Only their list membership will be changed.
- **`sendgrid-pp-cli marketing delete-field-definition`** - **This endpoint deletes a defined Custom Field.**

You can delete only Custom Fields; Reserved Fields cannot be deleted.
- **`sendgrid-pp-cli marketing delete-integration`** - This endpoint deletes Integrations.
- **`sendgrid-pp-cli marketing delete-list`** - **This endpoint allows you to deletes a specific list.**

Optionally, you can also delete contacts associated to the list. The query parameter, `delete_contacts=true`, will delete the list and start an asynchronous job to delete associated contacts.
- **`sendgrid-pp-cli marketing delete-scheduled-single-send`** - **This endpoint allows you to cancel a scheduled Single Send using a Single Send ID.**

Making a DELETE request to this endpoint will cancel the scheduled sending of a Single Send. The request will not delete the Single Send itself. Deleting a Single Send can be done by passing a DELETE request to `/marketing/singlesends/{id}`.
- **`sendgrid-pp-cli marketing delete-segment`** - **This endpoint allows you to delete a segment by `segment_id`.**

Note that deleting a segment does not delete the contacts associated with the segment by default. Contacts associated with a deleted segment will remain in your list of all contacts and any other segments they belong to.
- **`sendgrid-pp-cli marketing delete-segment-segments`** - **This endpoint allows you to delete a segment by ID.**
- **`sendgrid-pp-cli marketing delete-sender`** - **This endpoint allows you to delete an existing Sender.**
- **`sendgrid-pp-cli marketing delete-single-send`** - **This endpoint allows you to delete one Single Send using a Single Send ID.**

To first retrieve all your Single Sends' IDs, you can make a GET request to the `/marketing/singlensends` endpoint.

Please note that a `DELETE` request is permanent, and your Single Send will not be recoverable after deletion.
- **`sendgrid-pp-cli marketing delete-single-sends`** - **This endpoint allows you to delete multiple Single Sends using an array of Single Sends IDs.**

To first retrieve all your Single Sends' IDs, you can make a GET request to the `/marketing/singlensends` endpoint.

Please note that a DELETE request is permanent, and your Single Sends will not be recoverable after deletion.
- **`sendgrid-pp-cli marketing duplicate-single-send`** - **This endpoint allows you to duplicate an existing Single Send using its Single Send ID.**

Duplicating a Single Send is useful when you want to create a Single Send but don't want to start from scratch. Once duplicated, you can update or edit the Single Send by making a PATCH request to the `/marketing/singlesends/{id}` endpoint.
 
If you leave the `name` field blank, your duplicate will be assigned the name of the Single Send it was copied from with the text “Copy of ” prepended to it. The `name` field length is limited to 100 characters, so the end of the new Single Send name, including “Copy of ”, will be trimmed if the name exceeds this limit.
- **`sendgrid-pp-cli marketing export-automation-stat`** - **This endpoint allows you to export Single Send stats as .CSV data**.

You can specify one Single Send or many: include as many Single Send IDs as you need, separating them with commas, as the value of the `ids` query string parameter.

The data is returned as plain text response but in .CSV format, so your application making the call can present the information in whatever way is most appropriate, or just save the data as a .csv file.
- **`sendgrid-pp-cli marketing export-contact`** - **Use this endpoint to export lists or segments of contacts**.

If you would just like to have a link to the exported list sent to your email set the `notifications.email` option to `true` in the `POST` payload.

If you would like to download the list, take the `id` that is returned and use the "Export Contacts Status" endpoint to get the `urls`. Once you have the list of URLs, make a `GET` request to each URL provided to download your CSV file(s).

You specify the segments and or/contact lists you wish to export by providing the relevant IDs in, respectively, the `segment_ids` and `list_ids` fields in the request body.

The lists will be provided in either JSON or CSV files. To specify which of these you would required, set the request body `file_type` field to `json` or `csv`.

You can also specify a maximum file size (in MB). If the export file is larger than this, it will be split into multiple files.
- **`sendgrid-pp-cli marketing export-single-send-stat`** - **This endpoint allows you to export Single Send stats as .CSV data**.

You can specify one Single Send or many: include as many Single Send IDs as you need, separating them with commas, as the value of the `ids` query string parameter.

The data is returned as plain text response but in .CSV format, so your application making the call can present the information in whatever way is most appropriate, or just save the data as a .csv file.
- **`sendgrid-pp-cli marketing find-integration-by-id`** - This endpoint returns the data for a specific Integration.
- **`sendgrid-pp-cli marketing get-automation-stat`** - **This endpoint allows you to retrieve stats for a single Automation using its ID.**

Multiple Automation IDs can be retrieved using the "Get All Automation Stats" endpoint. Once you have an ID, this endpoint will return detailed stats for the single automation specified.

You may constrain the stats returned using the `start_date` and `end_date` query string parameters. You can also use the `group_by` and `aggregated_by` query string parameters to further refine the stats returned.
- **`sendgrid-pp-cli marketing get-contact`** - **This endpoint returns the full details and all fields for the specified contact**.
The "Get Contacts by Identifier" endpoint can be used to get the ID of a contact.
- **`sendgrid-pp-cli marketing get-contact-by-identifiers`** - Use this endpoint to retrieve up to 100 contacts that match the requested identifier values for a single identifier type.

`identifier_type` must be a valid identifier type: `email`, `phone_number_id`, `external_id`, or `anonymous_id`.

Use this endpoint instead of the [Search Contacts endpoint](https://www.twilio.com/docs/sendgrid/api-reference/contacts/search-contacts) when you can provide exact identifiers and do not need to include other [Segmentation Query Language (SGQL)](https://www.twilio.com/docs/sendgrid/for-developers/sending-email/segmentation-query-language/) filters when searching.

This endpoint returns a `200` status code when any contacts match the identifiers you supplied. When searching multiple identifier values in a single request, it is possible that some will match a contact while others will not. When a partially successful search like this is made, the matching contacts are returned in an object and an error message is returned for the identifier values that are not found.

This endpoint returns a `404` status code when no contacts are found for the provided identifiers.

This endpoint returns a `400` status code if any searched addresses are invalid.

Twilio SendGrid recommends exporting your contacts regularly as a backup to avoid issues or lost data.
- **`sendgrid-pp-cli marketing get-export-contact`** - **This endpoint can be used to check the status of a contact export job**. 

To use this call, you will need the `id` from the "Export Contacts" call.

If you would like to download a list, take the `id` that is returned from the "Export Contacts" endpoint and make an API request here to get the `urls`. Once you have the list of URLs, make a `GET` request on each URL to download your CSV file(s).

Twilio SendGrid recommends exporting your contacts regularly as a backup to avoid issues or lost data.
- **`sendgrid-pp-cli marketing get-import-contact`** - **This endpoint can be used to check the status of a contact import job**. 

Use the `job_id` from the "Import Contacts," "Add or Update a Contact," or "Delete Contacts" endpoints as the `id` in the path parameter.

If there is an error with your `PUT` request, download the `errors_url` file and open it to view more details.

The job `status` field indicates whether the job is `pending`, `completed`, `errored`, or `failed`. 

Pending means not started. Completed means finished without any errors. Errored means finished with some errors. Failed means finished with all errors, or the job was entirely unprocessable: for example, if you attempt to import file format we do not support.

The `results` object will have fields depending on the job type.

Twilio SendGrid recommends exporting your contacts regularly as a backup to avoid issues or lost data.
- **`sendgrid-pp-cli marketing get-integrations-by-user`** - This endpoint returns all the Integrations for the user making this call.
- **`sendgrid-pp-cli marketing get-list`** - **This endpoint returns data about a specific list.**
Setting the optional parameter `contact_sample=true` returns the `contact_sample` in the response body. Up to 50 of the most recent contacts uploaded or attached to a list will be returned.
The full contact count is also returned.
- **`sendgrid-pp-cli marketing get-segment`** - **This endpoint allows you to retrieve a single segment by ID.**
- **`sendgrid-pp-cli marketing get-segment-segments`** - Get Marketing Campaigns Segment by ID
- **`sendgrid-pp-cli marketing get-sender`** - **This endpoint allows you to get the details for a specific Sender by `id`.**
- **`sendgrid-pp-cli marketing get-single-send`** - **This endpoint allows you to retrieve details about one Single Send using a Single Send ID.**

You can retrieve all of your Single Sends by making a GET request to the `/marketing/singlesends` endpoint.
- **`sendgrid-pp-cli marketing get-single-send-stat`** - **This endpoint allows you to retrieve stats for an individual Single Send using a Single Send ID.**

Multiple Single Send IDs can be retrieved using the "Get All Single Sends Stats" endpoint. Once you have an ID, this endpoint will return detailed stats for the Single Send specified.

You may constrain the stats returned using the `start_date` and `end_date` query string parameters. You can also use the `group_by` and `aggregated_by` query string parameters to further refine the stats returned.
- **`sendgrid-pp-cli marketing import-contact`** - **This endpoint allows a CSV upload containing up to one million contacts or 5GB of data, whichever is smaller. At least one identifier is required for a successful import.**

Imports take place asynchronously: the endpoint returns a URL (`upload_uri`) and HTTP headers (`upload_headers`) which can subsequently be used to `PUT` a file of contacts to be imported into our system.

Uploaded CSV files may also be [gzip-compressed](https://en.wikipedia.org/wiki/Gzip).

In either case, you must include the field `file_type` with the value `csv` in your request body.

The `field_mappings` parameter is a respective list of field definition IDs to map the uploaded CSV columns to. It allows you to use CSVs where one or more columns are skipped (`null`) or remapped to the contact field.

For example, if `field_mappings` is set to `[null, "w1", "_rf1"]`, this means skip column 0, map column 1 to the custom field with the ID `w1`, and map column 2 to the reserved field with the ID `_rf1`. See the "Get All Field Definitions" endpoint to fetch your custom and reserved field IDs to use with `field_mappings`.

Once you receive the response body you can then initiate a **second** API call where you use the supplied URL and HTTP header to upload your file. For example:

`curl --upload-file "file/path.csv" "URL_GIVEN" -H "HEADER_GIVEN"`

If you would like to monitor the status of your import job, use the `job_id` and the "Import Contacts Status" endpoint.

Twilio SendGrid recommends exporting your contacts regularly as a backup to avoid issues or lost data.
- **`sendgrid-pp-cli marketing list-automation-stat`** - **This endpoint allows you to retrieve stats for all your Automations.**

By default, all of your Automations will be returned, but you can specify a selection by passing in a comma-separated list of Automation IDs as the value of the query string parameter `automation_ids`.

Responses are paginated. You can limit the number of responses returned per batch using the `page_size` query string parameter. The default is 25, but you can specify a value between 1 and 50.

You can retrieve a specific page of responses with the `page_token` query string parameter.
- **`sendgrid-pp-cli marketing list-batched-contact`** - **This endpoint is used to retrieve a set of contacts identified by their IDs.**

This can be more efficient endpoint to get contacts than making a series of individual `GET` requests to the "Get a Contact by ID" endpoint.

You can supply up to 100 IDs. Pass them into the `ids` field in your request body as an array or one or more strings.

Twilio SendGrid recommends exporting your contacts regularly as a backup to avoid issues or lost data.
- **`sendgrid-pp-cli marketing list-category`** - **This endpoint allows you to retrieve all the categories associated with your Single Sends.**

This endpoint will return your latest 1,000 categories.
- **`sendgrid-pp-cli marketing list-click-tracking-stat`** - **This endpoint lets you retrieve click-tracking stats for a single Automation**.

The stats returned list the URLs embedded in your Automation and the number of clicks each one received.
- **`sendgrid-pp-cli marketing list-contact`** - **This endpoint will return up to 50 of the most recent contacts uploaded or attached to a list**. 

This list will then be sorted by email address.

The full contact count is also returned.

Please note that pagination of the contacts has been deprecated.

Twilio SendGrid recommends exporting your contacts regularly as a backup to avoid issues or lost data.
- **`sendgrid-pp-cli marketing list-contact-by-email`** - **This endpoint allows you to retrieve up to 100 contacts matching the searched `email` address(es), including any `alternate_emails`.** 

Email addresses are unique to a contact, meaning this endpoint can treat an email address as a primary key to search by. The contact object associated with the address, whether it is their `email` or one of their `alternate_emails` will be returned if matched.

Email addresses in the search request do not need to match the case in which they're stored, but the email addresses in the result will be all lower case. Empty strings are excluded from the search and will not be returned.

This endpoint should be used in place of the "Search Contacts" endpoint when you can provide exact email addresses and do not need to include other [Segmentation Query Language (SGQL)](https://sendgrid.com/docs/for-developers/sending-email/segmentation-query-language/) filters when searching.

If you need to access a large percentage of your contacts, we recommend exporting your contacts with the "Export Contacts" endpoint and filtering the client side results.

This endpoint returns a `200` status code when any contacts match the address(es) you supplied. When searching multiple addresses in a single request, it is possible that some addresses will match a contact while others will not. When a partially successful search like this is made, the matching contacts are returned in an object and an error message is returned for the email address(es) that are not found. 

This endpoint returns a `404` status code when no contacts are found for the provided email address(es).

A `400` status code is returned if any searched addresses are invalid.

Twilio SendGrid recommends exporting your contacts regularly as a backup to avoid issues or lost data.
- **`sendgrid-pp-cli marketing list-contact-count`** - **This endpoint returns the total number of contacts you have stored.**

Twilio SendGrid recommends exporting your contacts regularly as a backup to avoid issues or lost data.
- **`sendgrid-pp-cli marketing list-contact-count-lists`** - **This endpoint returns the number of contacts on a specific list.**
- **`sendgrid-pp-cli marketing list-export-contact`** - **Use this endpoint to retrieve details of all current exported jobs**.

It will return an array of objects, each of which records an export job in flight or recently completed. 

Each object's `export_type` field will tell you which kind of export it is and its `status` field will indicate what stage of processing it has reached. Exports which are `ready` will be accompanied by a `urls` field which lists the URLs of the export's downloadable files — there will be more than one if you specified a maximum file size in your initial export request.

Use this endpoint if you have exports in flight but do not know their IDs, which are required for the "Export Contacts Status" endpoint.
- **`sendgrid-pp-cli marketing list-field-definition`** - **This endpoint retrieves all defined Custom Fields and Reserved Fields.**
- **`sendgrid-pp-cli marketing list-list`** - **This endpoint returns an array of all of your contact lists.**
- **`sendgrid-pp-cli marketing list-segment`** - **This endpoint allows you to retrieve a list of segments.**

The query param `parent_list_ids` is treated as a filter.  Any match will be returned.  Zero matches will return a response code of 200 with an empty `results` array.

`parent_list_ids` | `no_parent_list_id` | `ids` | `result`
-----------------:|:--------------------:|:-------------:|:-------------:
empty | false | empty | all segments values
list_ids | false | empty | segments filtered by list_ids values
list_ids |true | empty | segments filtered by list_ids and segments with no parent list_ids empty
empty | true | empty | segments with no parent list_ids
anything | anything | ids | segments with matching segment ids |
- **`sendgrid-pp-cli marketing list-segment-segments`** - **This endpoint allows you to retrieve a list of segments.**

The query param `parent_list_ids` is treated as a filter.  Any match will be returned.  Zero matches will return a response code of 200 with an empty `results` array.

`parent_list_ids` | `no_parent_list_id` | `ids` | `result`
-----------------:|:--------------------:|:-------------:|:-------------:
empty | false | empty | all segments values
list_ids | false | empty | segments filtered by list_ids values
list_ids |true | empty | segments filtered by list_ids and segments with no parent list_ids empty
empty | true | empty | segments with no parent list_ids
anything | anything | ids | segments with matching segment ids |
- **`sendgrid-pp-cli marketing list-sender`** - **This endpoint allows you to get a list of all your Senders.**
- **`sendgrid-pp-cli marketing list-single-send`** - **This endpoint allows you to retrieve all your Single Sends.**

Returns all of your Single Sends with condensed details about each, including the Single Sends' IDs. For more details about an individual Single Send, pass the Single Send's ID to the `/marketing/singlesends/{id}` endpoint.
- **`sendgrid-pp-cli marketing list-single-send-stat`** - **This endpoint allows you to retrieve stats for all your Single Sends.**

By default, all of your Single Sends will be returned, but you can specify a selection by passing in a comma-separated list of Single Send IDs as the value of the query string parameter `singlesend_ids`.

Responses are paginated. You can limit the number of responses returned per batch using the `page_size` query string parameter. The default is 25, but you specify a value between 1 and 50.

You can retrieve a specific page of responses with the `page_token` query string parameter.
- **`sendgrid-pp-cli marketing list-single-send-tracking-stat`** - **This endpoint lets you retrieve click-tracking stats for one Single Send**.

The stats returned list the URLs embedded in the specified Single Send and the number of clicks each one received.
- **`sendgrid-pp-cli marketing refresh-segment`** - Manually refresh a segment by segment ID.
- **`sendgrid-pp-cli marketing reset-sender-verification`** - **This endpoint allows you to resend the verification request for a specific Sender.**
- **`sendgrid-pp-cli marketing schedule-single-send`** - **This endpoint allows you to send a Single Send immediately or schedule it to be sent at a later time.**

To send your message immediately, set the `send_at` property value to the string `now`. To schedule the Single Send for future delivery, set the `send_at` value to your desired send time in [ISO 8601 date time format](https://www.iso.org/iso-8601-date-and-time-format.html) (`yyyy-MM-ddTHH:mm:ssZ`).
- **`sendgrid-pp-cli marketing search-contact`** - **Use this endpoint to locate contacts**.

The request body's `query` field accepts valid [SGQL](https://sendgrid.com/docs/for-developers/sending-email/segmentation-query-language/) for searching for a contact.

Because contact emails are stored in lower case, using SGQL to search by email address requires the provided email address to be in lower case. The SGQL `lower()` function can be used for this.

Only the first 50 contacts that meet the search criteria will be returned.

If the query takes longer than 20 seconds, a `408 Request Timeout` status will be returned.

Formatting the `created_at` and `updated_at` values as Unix timestamps is deprecated. Instead, they are returned as ISO format as string.
- **`sendgrid-pp-cli marketing search-single-send`** - **This endpoint allows you to search for Single Sends based on specified criteria.**

You can search for Single Sends by passing a combination of values using the `name`, `status`, and `categories` request body fields.

For example, if you want to search for all Single Sends that are "drafts" or "scheduled" and also associated with the category "shoes," your request body may look like the example below.

```javascript
{
  "status": [
    "draft",
    "scheduled"
  ],
  "categories": [
    "shoes"
  ],
}
```
- **`sendgrid-pp-cli marketing send-test-email`** - **This endpoint allows you to send a test marketing email to a list of email addresses**.

Before sending a marketing message, you can test it using this endpoint. You may specify up to **10 contacts** in the `emails` request body field. You must also specify a `template_id` and include either a `from_address` or `sender_id`. You can manage your templates with the [Twilio SendGrid App](https://mc.sendgrid.com/dynamic-templates) or the [Transactional Templates API](https://docs.sendgrid.com/api-reference/transactional-templates).

> Please note that this endpoint works with Dynamic Transactional Templates only. Legacy Transactional Templates will not be delivered.

For more information about managing Dynamic Transactional Templates, see [How to Send Email with Dynamic Transactional Templates](https://sendgrid.com/docs/ui/sending-email/how-to-send-an-email-with-dynamic-transactional-templates/).

You can also test your Single Sends in the [Twilio SendGrid Marketing Campaigns UI](https://mc.sendgrid.com/single-sends).
- **`sendgrid-pp-cli marketing update-contact`** - **This endpoint allows the [upsert](https://en.wiktionary.org/wiki/upsert) (insert or update) of up to 30,000 contacts, or 6MB of data, whichever is lower**.
Because the creation and update of contacts is an asynchronous process, the response will not contain immediate feedback on the processing of your upserted contacts. Rather, it will contain an HTTP 202 response indicating the contacts are queued for processing or an HTTP 4XX error containing validation errors. Should you wish to get the resulting contact's ID or confirm that your contacts have been updated or added, you can use the [Get Contacts by Identifiers operation](https://www.twilio.com/docs/sendgrid/api-reference/contacts/get-contacts-by-identifiers).
Please note that custom fields need to have been already created if you wish to set their values for the contacts being upserted. To do this, please use the [Create Custom Field Definition endpoint](https://www.twilio.com/docs/sendgrid/api-reference/custom-fields/create-custom-field-definition).
You will see a `job_id` in the response to your request. This can be used to check the status of your upsert job. To do so, please use the [Import Contacts Status endpoint](https://www.twilio.com/docs/sendgrid/api-reference/contacts/import-contacts-status).
If the contact already exists in the system, any entries submitted via this endpoint will update the existing contact. In order to update a contact, all of its existing identifiers must be present. Any fields omitted from the request will remain as they were. A contact's ID cannot be used to update the contact.
The email field will be changed to all lower-case. If a contact is added with an email that exists but contains capital letters, the existing contact with the all lower-case email will be updated.
- **`sendgrid-pp-cli marketing update-field-definition`** - **This endpoint allows you to update a defined Custom Field.**

Only your Custom fields can be modified; Reserved Fields cannot be updated.
- **`sendgrid-pp-cli marketing update-integration`** - This endpoint updates an existing Integration.
- **`sendgrid-pp-cli marketing update-list`** - **This endpoint updates the name of a list.**
- **`sendgrid-pp-cli marketing update-segment`** - Segment `name` has to be unique. A user can not create a new segment with an existing segment name.
- **`sendgrid-pp-cli marketing update-sender`** - **This endpoint allows you to update an existing Sender.**

Updates to `from.email` require re-verification. If your domain has been authenticated, a new Sender will auto verify on creation. Otherwise, an email will be sent to the `from.email`.

Partial updates are allowed, but fields that are marked as "required" in the `POST` (create) endpoint must not be nil if that field is included in the `PATCH` request.
- **`sendgrid-pp-cli marketing update-single-send`** - **This endpoint allows you to update a Single Send using a Single Send ID.**

You only need to pass the properties you want to update. Any blank or missing properties will remain unaltered.

This endpoint will update a draft of the Single Send but will not send it or schedule it to be sent. Any `send_at` property value set with this endpoint will prepopulate the Single Send's send date. However, the Single Send will remain an unscheduled draft until it's updated with the [**Schedule Single Send**](https://docs.sendgrid.com/api-reference/single-sends/schedule-single-send) endpoint or SendGrid application UI.

### messages

Manage messages

- **`sendgrid-pp-cli messages download-csv`** - **This endpoint will return a presigned URL that can be used to download the CSV that was requested from the "Request a CSV" endpoint.**
- **`sendgrid-pp-cli messages get`** - Get all of the details about the specified message.

For Regional (EU) subusers, no data will be generated for this service.
- **`sendgrid-pp-cli messages list`** - Filter all messages to search your Email Activity. All queries must be [URL encoded](https://meyerweb.com/eric/tools/dencoder/), and use the following format:

`query={query_type}="{query_content}"`

 Once URL encoded, the previous query will look like this:

`query=type%3D%22query_content%22`

For example, to filter by a specific email, use the following query:

`query=to_email%3D%22example%40example.com%22`

Visit our [Query Reference section](https://docs.sendgrid.com/for-developers/sending-email/getting-started-email-activity-api#query-reference) to see a full list of basic query types and examples.
- **`sendgrid-pp-cli messages request-csv`** - This request will kick off a backend process to generate a CSV file. Once generated, the worker will then send an email for the user download the file. The link will expire in 3 days.

The CSV will contain the events from the last 30 days, limited to the last 1 million events maximum. This endpoint will be rate limited to 1 request every 12 hours (rate limit may change).

This endpoint is similar to the GET Single Message endpoint - the only difference is that /download is added to indicate that this is a CSV download requests but the same query is used to determine what the CSV should contain.

### partner-settings

Twilio SendGrid Partner Settings API

- **`sendgrid-pp-cli partner-settings`** - **This endpoint allows you to retrieve a paginated list of all partner settings that you can enable.**

You can use the `limit` query parameter to set the page size. If your list contains more items than the page size permits, you can make multiple requests.

Use the `offset` query parameter to control the position in the list from which to start retrieving additional items.'

### partners

Manage partners

- **`sendgrid-pp-cli partners add-account-ips`** - Adds IP(s) to the specified account.
- **`sendgrid-pp-cli partners authenticate-account`** - Authenticates and logs in a user to Twilio Sendgrid as a specific admin identity configured for SSO by partner. Any additional teammates or subusers will need to log in directly via app.sendgrid.com
- **`sendgrid-pp-cli partners create-account`** - Creates a new account, with specified offering, under the organization.
- **`sendgrid-pp-cli partners delete-account`** - Delete a specific account under your organization by account ID. Note that this is an **irreversible** action that does the following:

 - Revokes API Keys and SSO so that the account user cannot log in or access SendGrid data.
 - Removes all offerings and configured SendGrid resources such as dedicated IPs.
 - Cancels billing effective immediately.
- **`sendgrid-pp-cli partners get-account-state`** - Retrieve the state of the specified account.
- **`sendgrid-pp-cli partners list-account`** - Retrieves all accounts under the organization.
- **`sendgrid-pp-cli partners list-account-ips`** - Retrieves a paginated list of IPs associated with the specified account, ordered by most recently added IP.
- **`sendgrid-pp-cli partners list-account-offering`** - Retrieves offering information about the specified account.
- **`sendgrid-pp-cli partners list-offering`** - Retrieves offerings available under the organization.
- **`sendgrid-pp-cli partners remove-account-ips`** - Removes IP(s) from the specified account.
- **`sendgrid-pp-cli partners update-account-offering`** - Changes a package offering for the specified account. Please note that an account can have only one package offering. Also associates one or more add-on offerings such as Marketing Campaigns, Dedicated IP Addresses, and Expert Services to the specified account.
- **`sendgrid-pp-cli partners update-account-state`** - Update the state of the specified account.

### recipients

Twilio SendGrid Legacy Marketing Campaigns Contacts: Recipients API

- **`sendgrid-pp-cli recipients`** - **This operation allows you to delete your recipients' personal email data**

The Delete Recipients' Email Data operation accepts a list of 5,000 `email_addresses` or a total payload size of 256Kb per request, whichever comes first. Upon a successful request with this operation, SendGrid will run a search on the email addresses provided against the SendGrid system to identify matches. SendGrid will then delete all personal data associated with the matched users such as the recipients' names, email addresses, subject lines, categories, and IP addresses.

This endpoint is rate limited to 100 requests per minute. If more requests need to be made, consider using the batch option to include multiple email addresses in each request (up to 5,000 addresses per request).

All email addresses are filtered for uniqueness and tested for structural validity—any invalid addresses will be returned in an error response.

Please note that recipient data is deleted for the account making the request only—deletions do not cascade from a parent account to its Subusers' recipients. To delete a Subuser's recipients' data, you can use the `on-behalf-of` header.

### scopes

Twilio SendGrid Scopes API

- **`sendgrid-pp-cli scopes`** - **This endpoint returns a list of all scopes that this user has access to.**

API Keys are used to authenticate with [SendGrid's v3 API](https://docs.sendgrid.com/api-reference/how-to-use-the-sendgrid-v3-api/authorization).

API Keys may be assigned certain permissions, or scopes, that limit which API endpoints they are able to access.

This endpoint returns all the scopes assigned to the key you use to authenticate with it. To retrieve the scopes assigned to another key, you can pass an API key ID to the "Retrieve an existing API key" endpoint.

For a more detailed explanation of how you can use API Key permissions, please visit our [API Keys documentation](https://sendgrid.com/docs/ui/account-and-settings/api-keys/).

### send-ips

Manage send ips

- **`sendgrid-pp-cli send-ips add-ip`** - This operation adds a Twilio SendGrid IP address to your account. You can also assign up to 100 Subusers to the IP address at creation.
- **`sendgrid-pp-cli send-ips add-ips-to-ip-pool`** - This operation appends a batch of IPs to an IP Pool. This operation requires all IP assignments to succeed. If any IP assignments fail, this endpoint will return an error.
- **`sendgrid-pp-cli send-ips add-sub-users-to-ip`** - This operation appends a batch of Subusers to a specified IP address. This endpoint requires all Subuser assignments to succeed. If a Subuser assignment fails, this endpoint will return an error.
- **`sendgrid-pp-cli send-ips create-ip-pool`** - This operation will create a named IP Pool and associate specified IP addresses with the newly created Pool. This operation requires all IP assignments to succeed. If any IP assignments fail, this endpoint will return an error and the Pool will not be created.

Each IP Pool may have a maximum of 100 assigned IP addresses.
- **`sendgrid-pp-cli send-ips delete-ip-pool`** - This operation deletes an IP Pool and unassigns all IP addresses associated with the Pool. IP addresses associated with the deleted Pool will remain in your account.
- **`sendgrid-pp-cli send-ips delete-ips-from-ip-pool`** - This operation removes a batch of IPs from an IP Pool. All IPs associated with the Pool will be unassigned from the deleted Pool. However, this operation does not remove the IPs from your account.
- **`sendgrid-pp-cli send-ips delete-sub-users-from-ip`** - This operation removes a batch of Subusers from a specified IP address.
- **`sendgrid-pp-cli send-ips get-ip`** - This operation returns details for a specified IP address. Details include whether the IP is assigned to a parent account, set to warm up automatically, which Pools the IP is associated with, when the IP was added and modified, whether the IP is leased, and whether the IP is enabled. Note that this operation will not return Subuser information associated with the IP. To retrieve Subuser information, use the "Get a List of Subusers Assigned to an IP" endpoint.
- **`sendgrid-pp-cli send-ips get-ip-pool`** - This operation will return the details for a specified IP Pool, including the Pool's name, ID, a sample list of the IPs associated with the Pool, and the total number of IPs belonging to the Pool.

A maximum of 10 IPs will be returned per IP Pool by default. To retrieve additional IP addresses associated with a Pool, use the "Get IPs Assigned to an IP Pool" operation.
- **`sendgrid-pp-cli send-ips list-ip`** - This operation returns a list of all IP addresses associated with your account. A sample of IP details is returned with each IP, including which Pools the IP is associated with, whether the IP is set to warm up automatically, and when the IP was last updated.

### Limitations

The `is_parent_assigned` parameter and `pool` parameter cannot be used at the same time. By definition, an IP cannot be assigned to a Pool if it is not first enabled. You can use either the `before_key` or `after_key` in combination with the `limit` parameter to iterate through paginated results but not both.
- **`sendgrid-pp-cli send-ips list-ip-assigned-to-ip-pool`** - This operation returns the IP addresses that are assigned to the specified IP pool.
- **`sendgrid-pp-cli send-ips list-ip-pool`** - This operation returns a list of your IP Pools and a sample of each Pools' associated IP addresses.

A maximum of 10 IPs will be returned per IP Pool by default. To retrieve additional IP addresses associated with a Pool, use the "Get IPs Assigned to an IP Pool" operation. Each user may have a maximum of 100 IP Pools.
- **`sendgrid-pp-cli send-ips list-sub-user-assigned-to-ip`** - This operation returns a list of Subuser IDs that have been assigned the specified IP address. To retrieve more information about the returned Subusers, use the [Subusers API](https://docs.sendgrid.com/api-reference/subusers-api/list-all-subusers).

You can use the `after_key` and `limit` query parameters to iterate through paginated results. The maximum limit is 100, meaning you may retrieve up to 100 Subusers per request. If the `after_key` in the API response is not null, there are more Subusers assigned to the IP address than those returned in the request. You can repeat the request with the non-null `after_key` value and the same limit to retrieve the next group of Subusers.
- **`sendgrid-pp-cli send-ips update-ip`** - This operation updates an IP address's settings, including whether the IP is set to warm up automatically, if the IP is  assigned by a parent account, and whether the IP is enabled or disabled. The request body must include at least one of the `is_auto_warmup`, `is_parent_assigned`, or `is_enabled` fields.
- **`sendgrid-pp-cli send-ips update-ip-pool`** - This operation will rename an IP Pool. An IP Pool name cannot start with a dot/period (.) or space.

### senders

Twilio SendGrid Marketing Campaigns Senders API

- **`sendgrid-pp-cli senders create`** - **This endpoint allows you to create a new sender identity.**

You may create up to 100 unique sender identities.
- **`sendgrid-pp-cli senders delete`** - **This endpoint allows you to delete one of your sender identities.**
- **`sendgrid-pp-cli senders get`** - **This endpoint allows you to retrieve a specific sender identity.**
- **`sendgrid-pp-cli senders list`** - **This endpoint allows you to retrieve a list of all sender identities that have been created for your account.**
- **`sendgrid-pp-cli senders update`** - **This endpoint allows you to update a sender identity.**

Updates to `from.email` require re-verification.

Partial updates are allowed, but fields that are marked as "required" in the POST (create) endpoint must not be nil if that field is included in the PATCH request.

### sso

Manage sso

- **`sendgrid-pp-cli sso create-certificate`** - **This endpoint allows you to create an SSO certificate.**
- **`sendgrid-pp-cli sso create-integration`** - **This endpoint allows you to create an SSO integration.**
- **`sendgrid-pp-cli sso create-teammate`** - **This endpoint allows you to create an SSO Teammate.**

The email address provided for the Teammate will also function as the Teammate's username. Once created, the Teammate's email address cannot be changed.

### Scopes

When creating a Teammate, you will assign it permissions or scopes. These scopes determine which actions the Teammate can perform and which features they can access. Scopes are provided with one of three properties passed to this endpoint: `is_admin`, `scopes`, and `persona`.

You can make a Teammate an administrator by setting `is_admin` to `true`. Administrators will have all scopes assigned to them. Alternatively, you can assign a `persona` to the teammate, which will assign them a block of permissions commonly required for that type of user. See the "Persona scopes" section of [**Teammate Permissions**](https://docs.sendgrid.com/ui/account-and-settings/teammate-permissions#persona-scopes) for a list of permsissions granted by persona. Lastly, you can assign individual permissions with the `scopes` property. See [**Teammate Permissions**](https://docs.sendgrid.com/ui/account-and-settings/teammate-permissions) for a full list of scopes that can be assigned to a Teammate.

### Subuser access

SendGrid Teammates may be assigned access to one or more Subusers. Subusers function like SendGrid sub-accounts with their own resources. See [**Subusers**](https://docs.sendgrid.com/ui/account-and-settings/subusers) for more information.

When assigning Subuser access to a Teammate, you may set the `has_restricted_subuser_access` property to `true` to constrain the Teammate so that they can operate only on behalf of the Subusers to which they are assigned. You may further set the level of access the Teammate has to each Subuser with the `subuser_access` property.
- **`sendgrid-pp-cli sso delete-certificate`** - **This endpoint allows you to delete an SSO certificate.**

You can retrieve a certificate's ID from the response provided by the "Get All SSO Integrations" endpoint.
- **`sendgrid-pp-cli sso delete-integration`** - **This endpoint allows you to delete an IdP configuration by ID.**

You can retrieve the IDs for your configurations from the response provided by the "Get All SSO Integrations" endpoint.
- **`sendgrid-pp-cli sso get-certificate`** - **This endpoint allows you to retrieve an individual SSO certificate.**
- **`sendgrid-pp-cli sso get-integration`** - **This endpoint allows you to retrieve an SSO integration by ID.**

You can retrieve the IDs for your configurations from the response provided by the "Get All SSO Integrations" endpoint.
- **`sendgrid-pp-cli sso list-integration`** - **This endpoint allows you to retrieve all SSO integrations tied to your Twilio SendGrid account.**

The IDs returned by this endpoint can be used by the APIs additional endpoints to modify your SSO integrations.
- **`sendgrid-pp-cli sso list-integration-certificate`** - **This endpoint allows you to retrieve all your IdP configurations by configuration ID.**

The `integration_id` expected by this endpoint is the `id` returned in the response by the "Get All SSO Integrations" endpoint.
- **`sendgrid-pp-cli sso update-certificate`** - **This endpoint allows you to update an existing certificate by ID.**

You can retrieve a certificate's ID from the response provided by the "Get All SSO Integrations" endpoint.
- **`sendgrid-pp-cli sso update-integration`** - **This endpoint allows you to modify an exisiting SSO integration.**

You can retrieve the IDs for your configurations from the response provided by the "Get All SSO Integrations" endpoint.
- **`sendgrid-pp-cli sso update-teammate`** - **This endpoint allows you to modify an existing SSO Teammate.**

Only the parent user and Teammates with admin permissions can update another Teammate's permissions.

### Scopes

When updating a Teammate, you will assign it permissions or scopes. These scopes determine which actions the Teammate can perform and which features they can access. Scopes are provided with one of three properties passed to this endpoint: `is_admin`, `scopes`, and `persona`.

You can make a Teammate an administrator by setting `is_admin` to `true`. Administrators will have all scopes assigned to them. Alternatively, you can assign a `persona` to the teammate, which will assign them a block of permissions commonly required for that type of user. See the "Persona scopes" section of [**Teammate Permissions**](https://docs.sendgrid.com/ui/account-and-settings/teammate-permissions#persona-scopes) for a list of permsissions granted by persona. Lastly, you can assign individual permissions with the `scopes` property. See [**Teammate Permissions**](https://docs.sendgrid.com/ui/account-and-settings/teammate-permissions) for a full list of scopes that can be assigned to a Teammate.

### Subuser access

SendGrid Teammates may be assigned access to one or more Subusers. Subusers function like SendGrid sub-accounts with their own resources. See [**Subusers**](https://docs.sendgrid.com/ui/account-and-settings/subusers) for more information.

When assigning Subuser access to a Teammate, you may set the `has_restricted_subuser_access` property to `true` to constrain the Teammate so that they can operate only on behalf of the Subusers to which they are assigned. You may further set the level of access the Teammate has to each Subuser with the `subuser_access` property.

### stats

Twilio SendGrid Marketing Campaigns Stats API

- **`sendgrid-pp-cli stats`** - **This endpoint allows you to retrieve all of your global email statistics between a given date range.**

Parent accounts can see either aggregated stats for the parent account or aggregated stats for a subuser specified in the `on-behalf-of` header. Subuser accounts will see only their own stats.

### subusers

Twilio SendGrid Subusers API

- **`sendgrid-pp-cli subusers create`** - **This endpoint allows you to create a new subuser.**
- **`sendgrid-pp-cli subusers delete`** - **This endpoint allows you to delete a subuser.**

This is a permanent action. Once deleted, a subuser cannot be retrieved.
- **`sendgrid-pp-cli subusers list`** - **This endpoint allows you to retrieve a paginated list of all your subusers.**

You can use the `username` query parameter to filter the list for specific subusers.

You can use the `limit` query parameter to set the page size. If your list contains more items than the page size permits, you can make multiple requests. Use the `offset` query parameter to control the position in the list from which to start retrieving additional items.
- **`sendgrid-pp-cli subusers list-monthly-stat`** - **This endpoint allows you to retrieve the monthly email statistics for all subusers over the given date range.**

When using the `sort_by_metric` to sort your stats by a specific metric, you can not sort by the following metrics:
`bounce_drops`, `deferred`, `invalid_emails`, `processed`, `spam_report_drops`, `spam_reports`, or `unsubscribe_drops`.
- **`sendgrid-pp-cli subusers list-reputation`** - **This endpoint allows you to request the reputations for your subusers.**

Subuser sender reputations give a good idea how well a sender is doing with regards to how recipients and recipient servers react to the mail that is being received. When a bounce, spam report, or other negative action happens on a sent email, it will affect your sender rating.  For Regional (EU) subusers, subuser reputation scores will not calculate accurately due to restrictions in place to maintain data residency.
- **`sendgrid-pp-cli subusers list-stat`** - **This endpoint allows you to retrieve the email statistics for the given subusers.**

You may retrieve statistics for up to 10 different subusers by including an additional _subusers_ parameter for each additional subuser.
- **`sendgrid-pp-cli subusers list-stat-sum`** - **This endpoint allows you to retrieve the total sums of each email statistic metric for all subusers over the given date range.**
- **`sendgrid-pp-cli subusers update`** - **This endpoint allows you to enable or disable a subuser.**

### suppression

Twilio SendGrid Suppressions API

- **`sendgrid-pp-cli suppression delete-block`** - **This endpoint allows you to delete a specific email address from your blocks list.**
- **`sendgrid-pp-cli suppression delete-blocks`** - **This endpoint allows you to delete all email addresses on your blocks list.**

There are two options for deleting blocked emails: 

1. You can delete all blocked emails by setting `delete_all` to `true` in the request body. 
2. You can delete a selection of blocked emails by specifying the email addresses in the `emails` array of the request body.
- **`sendgrid-pp-cli suppression delete-bounce`** - **This endpoint allows you to remove an email address from your bounce list.**
- **`sendgrid-pp-cli suppression delete-bounces`** - **This endpoint allows you to delete all emails on your bounces list.**

There are two options for deleting bounced emails: 

1. You can delete all bounced emails by setting `delete_all` to `true` in the request body. 
2. You can delete a selection of bounced emails by specifying the email addresses in the `emails` array of the request body. 

**WARNING:** You can not have both `emails` and `delete_all` set.
- **`sendgrid-pp-cli suppression delete-invalid-email`** - **This endpoint allows you to remove a specific email address from the invalid email address list.**
- **`sendgrid-pp-cli suppression delete-invalid-emails`** - **This endpoint allows you to remove email addresses from your invalid email address list.**

There are two options for deleting invalid email addresses: 

1) You can delete all invalid email addresses by setting `delete_all` to true in the request body.
2) You can delete some invalid email addresses by specifying certain addresses in an array in the request body.
- **`sendgrid-pp-cli suppression delete-spam-report`** - **This endpoint allows you to delete a specific spam report by email address.**

Deleting a spam report will remove the suppression, meaning email will once again be sent to the previously suppressed address. This should be avoided unless a recipient indicates they wish to receive email from you again. You can use our [bypass filters](https://sendgrid.com/docs/ui/sending-email/index-suppressions/#bypass-suppressions) to deliver messages to otherwise suppressed addresses when exceptions are required.
- **`sendgrid-pp-cli suppression delete-spam-reports`** - **This endpoint allows you to delete your spam reports.**

Deleting a spam report will remove the suppression, meaning email will once again be sent to the previously suppressed address. This should be avoided unless a recipient indicates they wish to receive email from you again. You can use our [bypass filters](https://sendgrid.com/docs/ui/sending-email/index-suppressions/#bypass-suppressions) to deliver messages to otherwise suppressed addresses when exceptions are required.

There are two options for deleting spam reports: 

1. You can delete all spam reports by setting the `delete_all` field to `true` in the request body.
2. You can delete a list of select spam reports by specifying the email addresses in the `emails` array of the request body.
- **`sendgrid-pp-cli suppression get-block`** - **This endpoint allows you to retrieve a specific email address from your blocks list.**
- **`sendgrid-pp-cli suppression get-bounces`** - **This endpoint allows you to retrieve a specific bounce by email address.**
- **`sendgrid-pp-cli suppression get-bounces-classifications`** - This endpoint will return the number of bounces for the classification specified in descending order for each day. You can retrieve the bounce classification totals in CSV format by specifying `"text/csv"` in the Accept header.
- **`sendgrid-pp-cli suppression get-invalid-email`** - **This endpoint allows you to retrieve a specific invalid email addresses.**
- **`sendgrid-pp-cli suppression get-spam-report`** - **This endpoint allows you to retrieve a specific spam report by email address.**
- **`sendgrid-pp-cli suppression list-block`** - **This endpoint allows you to retrieve a paginated list of all email addresses that are currently on your blocks list.**

You can use the `limit` query parameter to set the page size. If your list contains more items than the page size permits, you can make multiple requests. Use the `offset` query parameter to control the position in the list from which to start retrieving additional items.
- **`sendgrid-pp-cli suppression list-bounces`** - **This endpoint allows you to retrieve a paginated list of all your bounces.**

You can use the `limit` query parameter to set the page size. If your list contains more items than the page size permits, you can make multiple requests. Use the `offset` query parameter to control the position in the list from which to start retrieving additional items.
- **`sendgrid-pp-cli suppression list-bounces-classifications`** - This endpoint will return the total number of bounces by classification in descending order for each day. You can retrieve the bounce classification totals in CSV format by specifying `"text/csv"` in the Accept header.
- **`sendgrid-pp-cli suppression list-global`** - **This endpoint allows you to retrieve a paginated list of all email address that are globally suppressed.**

You can use the `limit` query parameter to set the page size. If your list contains more items than the page size permits, you can make multiple requests. Use the `offset` query parameter to control the position in the list from which to start retrieving additional items.
- **`sendgrid-pp-cli suppression list-invalid-email`** - **This endpoint allows you to retrieve a paginated list of all invalid email addresses.**

You can use the `limit` query parameter to set the page size. If your list contains more items than the page size permits, you can make multiple requests. Use the `offset` query parameter to control the position in the list from which to start retrieving additional items.
- **`sendgrid-pp-cli suppression list-spam-report`** - **This endpoint allows you to retrieve a paginated list of all spam reports.**

You can use the `limit` query parameter to set the page size. If your list contains more items than the page size permits, you can make multiple requests. Use the `offset` query parameter to control the position in the list from which to start retrieving additional items.

### teammates

Twilio SendGrid Teammates API

- **`sendgrid-pp-cli teammates delete`** - **This endpoint allows you to delete a teammate.**

**Only the parent user or an admin teammate can delete another teammate.**
- **`sendgrid-pp-cli teammates delete-pending`** - **This endpoint allows you to delete a pending teammate invite.**
- **`sendgrid-pp-cli teammates get`** - **This endpoint allows you to retrieve a specific Teammate by username.**

You can retrieve the username's for each of your Teammates using the "Retrieve all Teammates" endpoint.
- **`sendgrid-pp-cli teammates invite`** - **This endpoint allows you to invite a Teammate to your account via email.**

You can set a Teammate's initial permissions using the `scopes` array in the request body. Teammate's will receive a minimum set of scopes from Twilio SendGrid that are necessary for the Teammate to function.

**Note:** A teammate invite will expire after 7 days, but you may resend the invitation at any time to reset the expiration date.
- **`sendgrid-pp-cli teammates list`** - **This endpoint allows you to retrieve a paginated list of all current Teammates.**

You can use the `limit` query parameter to set the page size. If your list contains more items than the page size permits, you can make multiple requests. Use the `offset` query parameter to control the position in the list from which to start retrieving additional items.
- **`sendgrid-pp-cli teammates list-pending`** - **This endpoint allows you to retrieve a list of all pending Teammate invitations.**

Each teammate invitation is valid for 7 days. Users may resend the invitation to refresh the expiration date.
- **`sendgrid-pp-cli teammates resend-invite`** - **This endpoint allows you to resend a Teammate invitation.**

Teammate invitations will expire after 7 days. Resending an invitation will reset the expiration date.
- **`sendgrid-pp-cli teammates update`** - **This endpoint allows you to update a teammate’s permissions.**

To turn a teammate into an admin, the request body should contain an `is_admin` set to `true`. Otherwise, set `is_admin` to `false` and pass in all the scopes that a teammate should have.

**Only the parent user or other admin teammates can update another teammate’s permissions.**

**Admin users can only update permissions.**

### templates

Twilio SendGrid Templates API

- **`sendgrid-pp-cli templates create`** - **This endpoint allows you to create a transactional template.**
- **`sendgrid-pp-cli templates delete`** - **This endpoint allows you to delete a transactional template.**
- **`sendgrid-pp-cli templates duplicate`** - **This endpoint allows you to duplicate a transactional template.**
- **`sendgrid-pp-cli templates get`** - **This endpoint allows you to retrieve a single transactional template.**
- **`sendgrid-pp-cli templates list`** - **This endpoint allows you to retrieve all transactional templates.**
- **`sendgrid-pp-cli templates update`** - **This endpoint allows you to edit the name of a transactional template.**

To edit the template itself, [create a new transactional template version](https://docs.sendgrid.com/api-reference/transactional-templates-versions/create-a-new-transactional-template-version).

### tracking-settings

Manage tracking settings

- **`sendgrid-pp-cli tracking-settings list`** - **This endpoint allows you to retrieve a list of all tracking settings on your account.**
- **`sendgrid-pp-cli tracking-settings list-click`** - **This endpoint allows you to retrieve your current click tracking setting.**

Click Tracking overrides all the links and URLs in your emails and points them to either SendGrid’s servers or the domain with which you branded your link. When a customer clicks a link, SendGrid tracks those [clicks](https://sendgrid.com/docs/glossary/clicks/).

Click tracking helps you understand how users are engaging with your communications. SendGrid can track up to 1000 links per email
- **`sendgrid-pp-cli tracking-settings list-google-analytics`** - **This endpoint allows you to retrieve your current setting for Google Analytics.**

Google Analytics helps you understand how users got to your site and what they're doing there. For more information about using Google Analytics, please refer to [Google’s URL Builder](https://support.google.com/analytics/answer/1033867?hl=en) and their article on ["Best Practices for Campaign Building"](https://support.google.com/analytics/answer/1037445).

We default the settings to Google’s recommendations. For more information, see [Google Analytics Demystified](https://sendgrid.com/docs/ui/analytics-and-reporting/google-analytics/).
- **`sendgrid-pp-cli tracking-settings list-open`** - **This endpoint allows you to retrieve your current settings for open tracking.**

Open Tracking adds an invisible image at the end of the email which can track email opens.

If the email recipient has images enabled on their email client, a request to SendGrid’s server for the invisible image is executed and an open event is logged.

These events are logged in the Statistics portal, Email Activity interface, and are reported by the Event Webhook.
- **`sendgrid-pp-cli tracking-settings list-subscription`** - **This endpoint allows you to retrieve your current settings for subscription tracking.**

Subscription tracking adds links to the bottom of your emails that allows your recipients to subscribe to, or unsubscribe from, your emails.
- **`sendgrid-pp-cli tracking-settings update-click`** - **This endpoint allows you to enable or disable your current click tracking setting.**

Click Tracking overrides all the links and URLs in your emails and points them to either SendGrid’s servers or the domain with which you branded your link. When a customer clicks a link, SendGrid tracks those [clicks](https://sendgrid.com/docs/glossary/clicks/).

Click tracking helps you understand how users are engaging with your communications. SendGrid can track up to 1000 links per email
- **`sendgrid-pp-cli tracking-settings update-google-analytics`** - **This endpoint allows you to update your current setting for Google Analytics.**

Google Analytics helps you understand how users got to your site and what they're doing there. For more information about using Google Analytics, please refer to [Google’s URL Builder](https://support.google.com/analytics/answer/1033867?hl=en) and their article on ["Best Practices for Campaign Building"](https://support.google.com/analytics/answer/1037445).

We default the settings to Google’s recommendations. For more information, see [Google Analytics Demystified](https://sendgrid.com/docs/ui/analytics-and-reporting/google-analytics/).
- **`sendgrid-pp-cli tracking-settings update-open`** - **This endpoint allows you to update your current settings for open tracking.**

Open Tracking adds an invisible image at the end of the email which can track email opens.

If the email recipient has images enabled on their email client, a request to SendGrid’s server for the invisible image is executed and an open event is logged.

These events are logged in the Statistics portal, Email Activity interface, and are reported by the Event Webhook.
- **`sendgrid-pp-cli tracking-settings update-subscription`** - **This endpoint allows you to update your current settings for subscription tracking.**

Subscription tracking adds links to the bottom of your emails that allows your recipients to subscribe to, or unsubscribe from, your emails.

### user

Manage user

- **`sendgrid-pp-cli user create-event-webhook`** - **This endpoint allows you to create a new Event Webhook.**

When creating a webhook, you will provide a URL where you want the webhook to send POST requests, and you will select which events you want to receive in those request. See the [**Event Webhook Reference**](https://docs.sendgrid.com/for-developers/tracking-events/event#delivery-events) for details about each event type.

### Webhook identifiers

When your webhook is succesfully created, you will receive a webhook `id` in the response returned by this endpoint. You can use that ID to [update the webhook's settings](https://docs.sendgrid.com/api-reference/webhooks/update-an-event-webhook), [delete the webhook](https://docs.sendgrid.com/api-reference/webhooks/delete-an-event-webhook), [enable or disable signature verification for the webhook](https://docs.sendgrid.com/api-reference/webhooks/toggle-signature-verification-for-an-event-webhook), and, if signature verification is enabled, [retrieve the webhook's public key](https://docs.sendgrid.com/api-reference/webhooks/get-signed-event-webhooks-public-key).

You may also assign an optional friendly name to each of your webhooks. The friendly name is for convenience only and should not be used to programmatically differentiate your webhooks because it does not need to be unique. Use the webhook ID to reliably differentiate among your webhooks.

### OAuth

You can optionally configure OAuth verification for your webhook at the time of creation by passing the appropriate values in the `oauth_client_id`, `oauth_client_secret`, and `oauth_token_url` properties. You can enable or disable OAuth for the webhook after creation with the [**Update an Event Webhook**](https://docs.sendgrid.com/api-reference/webhooks/update-an-event-webhook) operation.

You may share one OAuth configuration across all your webhooks or create unique credentials for each. See our [webhook security documentation](https://docs.sendgrid.com/for-developers/tracking-events/getting-started-event-webhook-security-features#oauth-20) for details about OAuth and the Event Webhook.

### Signature verification

Enabling signature verification for your webhook is a separate process and cannot be done at the time of creation with this endpoint. You can use the webhook ID to [enable or disable signature verification with the endpoint dedicated for that operation](https://docs.sendgrid.com/api-reference/webhooks/toggle-signature-verification-for-an-event-webhook).
- **`sendgrid-pp-cli user create-parse-setting`** - **This endpoint allows you to create a new inbound parse setting.**

Creating an Inbound Parse setting requires two pieces of information: a `url` and a `hostname`.

The `hostname` must correspond to a domain authenticated by Twilio SendGrid on your account. If you need to complete domain authentication, you can use the [Twilio SendGrid App](https://app.sendgrid.com/settings/sender_auth) or the **Authenticate a Domain** endpoint. See [**How to Set Up Domain Authentication**](https://sendgrid.com/docs/ui/account-and-settings/how-to-set-up-domain-authentication/) for instructions.

Any email received by the `hostname` will be parsed when you complete this setup. You must also add a Twilio SendGrid MX record to this domain's DNS records. See [**Setting up the Inbound Parse Webhook**](https://sendgrid.com/docs/for-developers/parsing-email/setting-up-the-inbound-parse-webhook/) for full instructions.

The `url` represents a location where the parsed message data will be delivered. Twilio SendGrid will make an HTTP POST request to this `url` with the message data. The `url` must be publicly reachable, and your application must return a `200` status code to signal that the message data has been received.
- **`sendgrid-pp-cli user create-scheduled-send`** - **This endpoint allows you to cancel or pause a scheduled send associated with a `batch_id`.**

Passing this endpoint a `batch_id` and status will cancel or pause the scheduled send.

Once a scheduled send is set to `pause` or `cancel` you must use the "Update a scheduled send" endpoint to change its status or the "Delete a cancellation or pause from a scheduled send" endpoint to remove the status. Passing a status change to a scheduled send that has already been paused or cancelled will result in a `400` level status code.

If the maximum number of cancellations/pauses are added to a send, a `400` level status code will be returned.
- **`sendgrid-pp-cli user create-security-policy`** - Create a new webhook security policy. Note: One of signature or oauth must be given to have a valid security policy.
- **`sendgrid-pp-cli user delete-event-webhook`** - **This endpoint allows you to delete a single Event Webhook by ID.**

Unlike the [**Get an Event Webhook**](https://docs.sendgrid.com/api-reference/webhooks/get-an-event-webhook) and [**Update an Event Webhook**](https://docs.sendgrid.com/api-reference/webhooks/update-an-event-webhook) endpoints, which will operate on your oldest webhook by `created_date` when you don't provide an ID, this endpoint will return an error if you do not pass it an ID. This behavior prevents customers from unintentionally deleting a webhook. You can retrieve your webhooks' IDs using the [**Get All Event Webhooks**](https://docs.sendgrid.com/api-reference/webhooks/get-all-event-webhooks) endpoint.

### Enable or disable the webhook

This endpoint will permanently delete the webhook specified. If you instead want to disable a webhook, you can set the `enabled` property to `false` with the [**Update an Event Webhook**](https://docs.sendgrid.com/api-reference/webhooks/update-an-event-webhook) endpoint.
- **`sendgrid-pp-cli user delete-parse-setting`** - **This endpoint allows you to delete a specific inbound parse setting by hostname.**

You can retrieve all your Inbound Parse settings and their associated host names with the "Retrieve all parse settings" endpoint.
- **`sendgrid-pp-cli user delete-scheduled-send`** - **This endpoint allows you to delete the cancellation/pause of a scheduled send.**

Scheduled sends cancelled less than 10 minutes before the scheduled time are not guaranteed to be cancelled.
- **`sendgrid-pp-cli user delete-security-policy`** - Permanently delete a webhook security policy by its ID.
- **`sendgrid-pp-cli user get-event-webhook`** - **This endpoint allows you to retrieve a single Event Webhook by ID.**

If you do not pass a webhook ID to this endpoint, it will return your oldest webhook by `created_date`. This means the default webhook returned by this endpoint when no ID is provided will be the first one you created. This functionality allows customers who do not have multiple webhooks to use this endpoint to retrieve their only webhook, even if they do not supply an ID. If you have multiple webhooks, you can retrieve their IDs using the [**Get All Event Webhooks**](https://docs.sendgrid.com/api-reference/webhooks/get-all-event-webhooks) endpoint.

### Event settings

Your webhook will be returned with all of its settings, which include the events that will be included in the POST request by the webhook and the URL where they will be sent. If an event type is marked as `true`, the event webhook will send information about that event type. See the [**Event Webhook Reference**](https://docs.sendgrid.com/for-developers/tracking-events/event#delivery-events) for details about each event type.

### Signature verification

The `public_key` property will be returned only for webhooks with signature verification enabled.

### OAuth

You may share one OAuth configuration across all your webhooks or create unique credentials for each. The OAuth properties will be returned only for webhooks with OAuth configured.
- **`sendgrid-pp-cli user get-parse-setting`** - **This endpoint allows you to retrieve a specific inbound parse setting by hostname.**

You can retrieve all your Inbound Parse settings and their associated host names with the "Retrieve all parse settings" endpoint.
- **`sendgrid-pp-cli user get-scheduled-send`** - **This endpoint allows you to retrieve the cancel/paused scheduled send information for a specific `batch_id`.**
- **`sendgrid-pp-cli user get-security-policy`** - Retrieve the details of a specific webhook security policy by its ID.
- **`sendgrid-pp-cli user get-signed-event-webhook`** - **This endpoint allows you to retrieve the public key for a single Event Webhook by ID.**

If you do not pass a webhook ID to this endpoint, it will return the public key for your oldest webhook by `created_date`. This means the default key returned by this endpoint when no ID is provided will be for the first webhook you created. This functionality allows customers who do not have multiple webhooks to use this endpoint to retrieve their only webhook's public key, even if they do not supply an ID. If you have multiple webhooks, you can retrieve their IDs using the [**Get All Event Webhooks**](https://docs.sendgrid.com/api-reference/webhooks/get-all-event-webhooks) endpoint.

Once you have enabled signature verification for a webhook, you will need the public key provided to verify the signatures on requests coming from Twilio SendGrid. You can use the webhook ID to [enable or disable signature verification with the endpoint dedicated for that operation](https://docs.sendgrid.com/api-reference/webhooks/toggle-signature-verification-for-an-event-webhook).

For more information about cryptographically signing the Event Webhook, see [**Getting Started with the Event Webhook Security Features**](https://sendgrid.com/docs/for-developers/tracking-events/getting-started-event-webhook-security-features).
- **`sendgrid-pp-cli user list-account`** - **This endpoint allows you to retrieve your user account details.**

Your user's account information includes the user's account type and reputation.
- **`sendgrid-pp-cli user list-all-security-policies`** - Returns a list of all webhook security policies configured for your account, including their IDs, names, and security configurations.
- **`sendgrid-pp-cli user list-credit`** - **This endpoint allows you to retrieve the current credit balance for your account.**

Each account has a credit balance, which is a base number of emails it can send before receiving per-email charges. For more information about credits and billing, see [Billing and Plan details information](https://sendgrid.com/docs/ui/account-and-settings/billing/).
- **`sendgrid-pp-cli user list-email`** - **This endpoint allows you to retrieve the email address currently on file for your account.**
- **`sendgrid-pp-cli user list-enforced-tls-setting`** - **This endpoint allows you to retrieve your current Enforced TLS settings.**

The Enforced TLS settings specify whether or not the recipient is required to support TLS or have a valid certificate.

If either `require_tls` or `require_valid_cert` is set to `true`, the recipient must support TLS 1.1 or higher or have a valid certificate. If these conditions are not met, Twilio SendGrid will drop the message and send a block event with “TLS required but not supported” as the description.
- **`sendgrid-pp-cli user list-event-webhook`** - **This endpoint allows you to retrieve all of your Event Webhooks.**

Each webhook will be returned as an object in the `webhooks` array with the webhook's configuration details and ID. You can use a webhook's ID to [update the webhook's settings](https://docs.sendgrid.com/api-reference/webhooks/update-an-event-webhook), [delete the webhook](https://docs.sendgrid.com/api-reference/webhooks/delete-an-event-webhook), [enable or disable signature verification for the webhook](https://docs.sendgrid.com/api-reference/webhooks/toggle-signature-verification-for-an-event-webhook), and, if signature verification is enabled, [retrieve the webhook's public key](https://docs.sendgrid.com/api-reference/webhooks/get-signed-event-webhooks-public-key) when signature verification is enabled.

### Event settings

Each webhook's settings determine which events will be included in the POST request by the webhook and the URL where the request will be sent. See the [**Event Webhook Reference**](https://docs.sendgrid.com/for-developers/tracking-events/event#delivery-events) for details about each event type.

### Signature verification

The `public_key` property will be returned only for webhooks with signature verification enabled.

### OAuth

You may share one OAuth configuration across all your webhooks or create unique credentials for each. The OAuth properties will be returned only for webhooks with OAuth configured.
- **`sendgrid-pp-cli user list-parse-setting`** - **This endpoint allows you to retrieve all of your current inbound parse settings.**
- **`sendgrid-pp-cli user list-parse-static`** - **This endpoint allows you to retrieve the statistics for your Parse Webhook usage.**

SendGrid's Inbound Parse Webhook allows you to parse the contents and attachments of incoming emails. The Parse API can then POST the parsed emails to a URL that you specify. The Inbound Parse Webhook cannot parse messages greater than 30MB in size, including all attachments.

There are a number of pre-made integrations for the SendGrid Parse Webhook which make processing events easy. You can find these integrations in the [Library Index](https://docs.sendgrid.com/for-developers/sending-email/libraries#webhook-libraries).
- **`sendgrid-pp-cli user list-profile`** - **This endpoint allows you to retrieve your current profile details.**
- **`sendgrid-pp-cli user list-scheduled-send`** - **This endpoint allows you to retrieve all cancelled and paused scheduled send information.**

This endpoint will return only the scheduled sends that are associated with a `batch_id`. If you have scheduled a send using the `/mail/send` endpoint and the `send_at` field but no `batch_id`, the send will be scheduled for delivery; however, it will not be returned by this endpoint. For this reason, you should assign a `batch_id` to any scheduled send you may need to pause or cancel in the future.
- **`sendgrid-pp-cli user list-username`** - **This endpoint allows you to retrieve your current account username.**
- **`sendgrid-pp-cli user test-event-webhook`** - **This endpoint allows you to test an Event Webhook.**

Retry logic for this endpoint differs from other endpoints, which use a rolling 24-hour retry.

This endpoint will make a POST request with a fake event notification to a URL you provide. This allows you to verify that you have properly configured the webhook before sending real data to your URL.

### Test OAuth configuration

To test your OAuth configuration, you must include the necessary OAuth properties: `oauth_client_id`, `oauth_client_secret`, and `oauth_token_url`.

If the webhook you are testing already has OAuth credentials saved, you will provide only the `oauth_client_id` and `oauth_token_url`—we will pull the secret for you. If you are testing a new set of OAuth credentials that have not been saved with SendGrid, you must provide all three property values.

You can retrieve a previously saved `oauth_client_id` and `oauth_token_url` from the [**Get an Event Webhook**](https://docs.sendgrid.com/api-reference/webhooks/get-an-event-webhook) endpoint; however, for security reasons, SendGrid will not provide your `oauth_client_secret`.
- **`sendgrid-pp-cli user update-email`** - **This endpoint allows you to update the email address currently on file for your account.**
- **`sendgrid-pp-cli user update-enforced-tls-setting`** - **This endpoint allows you to update your Enforced TLS settings.**

To require TLS from recipients, set `require_tls` to `true`. If either `require_tls` or `require_valid_cert` is set to `true`, the recipient must support TLS 1.1 or higher or have a valid certificate. If these conditions are not met, Twilio SendGrid will drop the message and send a block event with “TLS required but not supported” as the description.
- **`sendgrid-pp-cli user update-event-webhook`** - **This endpoint allows you to update a single Event Webhook by ID.**

If you do not pass a webhook ID to this endpoint, it will update and return your oldest webhook by `created_date`. This means the default webhook updated by this endpoint when no ID is provided will be the first one you created. This functionality allows customers who do not have multiple webhooks to use this endpoint to update their only webhook, even if they do not supply an ID. If you have multiple webhooks, you can retrieve their IDs using the [**Get All Event Webhooks**](https://docs.sendgrid.com/api-reference/webhooks/get-all-event-webhooks) endpoint.

### Enable or disable the webhook

You can set the `enabled` property to `true` to enable the webhook or `false` to disable it. Disabling a webhook will not delete it from your account, but it will prevent the webhook from sending events to your designated URL.

### URL

A webhook's URL is the endpoint where you want the webhook to send POST requests containing event data. No more than one webhook may be configured to send to the same URL. SendGrid will return an error if you attempt to set a URL for a webhook that is already in use by the user on another webhook.

### Event settings

If an event type is marked as `true`, the event webhook will send information about that event type. See the [**Event Webhook Reference**](https://docs.sendgrid.com/for-developers/tracking-events/event#delivery-events) for details about each event type.

### Webhook identifiers

You may assign an optional friendly name to each of your webhooks. The friendly name is for convenience only and should not be used to programmatically differentiate your webhooks because it does not need to be unique.

### OAuth

You can configure OAuth for your webhook by passing the required values to this endpoint in the `oauth_client_id`, `oauth_client_secret`, and `oauth_token_url` properties. To disable OAuth, pass an empty string to this endpoint for each of the OAuth properties. You may share one OAuth configuration across all your webhooks or create unique credentials for each. See our [webhook security documentation](https://docs.sendgrid.com/for-developers/tracking-events/getting-started-event-webhook-security-features#oauth-20) for more detailed information about OAuth and the Event Webhook.

### Signature verification

Enabling signature verification for your webhook is a separate process and cannot be done with this endpoint. You can use the webhook ID to [enable or disable signature verification with the endpoint dedicated for that operation](https://docs.sendgrid.com/api-reference/webhooks/toggle-signature-verification-for-an-event-webhook).
- **`sendgrid-pp-cli user update-parse-setting`** - **This endpoint allows you to update a specific inbound parse setting by hostname.**

You can retrieve all your Inbound Parse settings and their associated host names with the "Retrieve all parse settings" endpoint.
- **`sendgrid-pp-cli user update-password`** - **This endpoint allows you to update your password.**
- **`sendgrid-pp-cli user update-profile`** - **This endpoint allows you to update your current profile details.**

Any one or more of the parameters can be updated via the PATCH `/user/profile` endpoint. You must include at least one when you PATCH.
- **`sendgrid-pp-cli user update-scheduled-send`** - **This endpoint allows you to update the status of a scheduled send for the given `batch_id`.**

If you have already set a `cancel` or `pause` status on a scheduled send using the "Cancel or pause a scheduled send" endpoint, you can update it's status using this endpoint. Attempting to update a status once it has been set with the "Cancel or pause a scheduled send" endpoint will result in a `400` error.
- **`sendgrid-pp-cli user update-security-policy`** - Update an existing webhook security policy with new configuration values.
- **`sendgrid-pp-cli user update-signed-event-webhook`** - **This endpoint allows you to enable or disable signature verification for a single Event Webhook by ID.**

If you do not pass a webhook ID to this endpoint, it will enable signature verification for your oldest webhook by `created_date`. This means the default webhook operated on by this endpoint when no ID is provided will be the first one you created. This functionality allows customers who do not have multiple webhooks to enable or disable signature verifiction for their only webhook, even if they do not supply an ID. If you have multiple webhooks, you can retrieve their IDs using the [**Get All Event Webhooks**](https://docs.sendgrid.com/api-reference/webhooks/get-all-event-webhooks) endpoint.

This endpoint accepts a single boolean request property, `enabled`, that can be set `true` or `false` to enable or disable signature verification. This endpoint will return the public key required to verify Twilio SendGrid signatures if it is enabled or an empty string if signing is disabled. You can also retrieve your public key using the [**Get an Event Webhook's Public Key**](https://docs.sendgrid.com/api-reference/webhooks/get-signed-event-webhooks-public-key) endpoint.

For more information about cryptographically signing the Event Webhook, see [**Getting Started with the Event Webhook Security Features**](https://sendgrid.com/docs/for-developers/tracking-events/getting-started-event-webhook-security-features).
- **`sendgrid-pp-cli user update-username`** - **This endpoint allows you to update the username for your account.**

### validations

Manage validations

- **`sendgrid-pp-cli validations get-email-job-for-verification`** - **This endpoint returns a specific Bulk Email Validation Job. You can use this endpoint to check on the progress of a Job.**
- **`sendgrid-pp-cli validations get-email-jobs`** - **This endpoint returns a list of all of a user's Bulk Email Validation Jobs.**
- **`sendgrid-pp-cli validations list-email-job-for-verification`** - **This endpoint returns a presigned URL and request headers. Use this information to upload a list of email addresses for verification.**

Note that in a successful response the `content-type` header value matches the provided `file_type` parameter in the `PUT` request.

Once you have an `upload_uri` and the `upload_headers`, you're ready to upload your email address list for verification. For the expected format of the email address list and a sample upload request, see the [Bulk Email Address Validation Overview page](https://www.twilio.com/docs/sendgrid/ui/managing-contacts/email-address-validation/bulk-email-address-validation-overview).
- **`sendgrid-pp-cli validations validate-email`** - **This endpoint allows you to validate an email address.**

### verified-senders

Manage verified senders

- **`sendgrid-pp-cli verified-senders create`** - **This endpoint allows you to create a new Sender Identify**.

Upon successful submission of a `POST` request to this endpoint, an identity will be created, and a verification email will be sent to the address assigned to the `from_email` field. You must complete the verification process using the sent email to fully verify the sender.

If you need to resend the verification email, you can do so with the Resend Verified Sender Request, `/resend/{id}`, endpoint.

If you need to authenticate a domain rather than a Single Sender, see the [Domain Authentication API](https://docs.sendgrid.com/api-reference/domain-authentication/authenticate-a-domain).
- **`sendgrid-pp-cli verified-senders delete`** - **This endpoint allows you to delete a Sender Identity**.

Pass the `id` assigned to a Sender Identity to this endpoint to delete the Sender Identity from your account.

You can retrieve the IDs associated with Sender Identities using the "Get All Verified Senders" endpoint.
- **`sendgrid-pp-cli verified-senders list`** - **This endpoint allows you to retrieve all the Sender Identities associated with an account.**

This endpoint will return both verified and unverified senders.

You can limit the number of results returned using the `limit`, `lastSeenID`, and `id` query string parameters.

* `limit` allows you to specify an exact number of Sender Identities to return.
* `lastSeenID` will return senders with an ID number occuring after the passed in ID. In other words, the `lastSeenID` provides a starting point from which SendGrid will iterate to find Sender Identities associated with your account.
* `id` will return information about only the Sender Identity passed in the request.
- **`sendgrid-pp-cli verified-senders list-domain`** - **This endpoint returns a list of domains known to implement DMARC and categorizes them by failure type — hard failure or soft failure**.

Domains listed as hard failures will not deliver mail when used as a [Sender Identity](https://sendgrid.com/docs/for-developers/sending-email/sender-identity/) due to the domain's DMARC policy settings.

For example, using a `yahoo.com` email address as a Sender Identity will likely result in the rejection of your mail. For more information about DMARC, see [Everything about DMARC](https://sendgrid.com/docs/ui/sending-email/dmarc/).
- **`sendgrid-pp-cli verified-senders list-steps-completed`** - **This endpoint allows you to determine which of SendGrid’s verification processes have been completed for an account**.

This endpoint returns boolean values, `true` and `false`, for [Domain Authentication](https://sendgrid.com/docs/for-developers/sending-email/sender-identity/#domain-authentication), `domain_verified`, and [Single Sender Verification](https://sendgrid.com/docs/for-developers/sending-email/sender-identity/#single-sender-verification), `sender_verified`, for the account.

An account may have one, both, or neither verification steps completed. If you need to authenticate a domain rather than a Single Sender, see the "Authenticate a domain" endpoint.
- **`sendgrid-pp-cli verified-senders resend`** - **This endpoint allows you to resend a verification email to a specified Sender Identity**.

Passing the `id` assigned to a Sender Identity to this endpoint will resend a verification email to the `from_address` associated with the Sender Identity. This can be useful if someone loses their verification email or needs to have it resent for any other reason.

You can retrieve the IDs associated with Sender Identities by passing a "Get All Verified Senders" endpoint.
- **`sendgrid-pp-cli verified-senders update`** - **This endpoint allows you to update an existing Sender Identity**.

Pass the `id` assigned to a Sender Identity to this endpoint as a path parameter. Include any fields you wish to update in the request body in JSON format.

You can retrieve the IDs associated with Sender Identities by passing a `GET` request to the Get All Verified Senders endpoint, `/verified_senders`.

**Note:** Unlike a `PUT` request, `PATCH` allows you to update only the fields you wish to edit. Fields that are not passed as part of a request will remain unaltered.
- **`sendgrid-pp-cli verified-senders verify-sender-token`** - **This endpoint allows you to verify a sender requests.**

The token is generated by SendGrid and included in a verification email delivered to the address that's pending verification.

### whitelabel

Manage whitelabel

- **`sendgrid-pp-cli whitelabel add-ip-to-authenticated-domain`** - **This endpoint allows you to add an IP address to an authenticated domain.**
- **`sendgrid-pp-cli whitelabel associate-branded-link-with-subuser`** - **This endpoint allows you to associate a branded link with a subuser account.**

Link branding can be associated with subusers from the parent account. This functionality allows subusers to send mail using their parent's link branding. To associate link branding, the parent account must first create a branded link and validate it. The parent may then associate that branded link with a subuser via the API or the [Subuser Management page of the Twilio SendGrid App](https://app.sendgrid.com/settings/subusers).
- **`sendgrid-pp-cli whitelabel associate-subuser-with-domain`** - **This endpoint allows you to associate a specific authenticated domain with a subuser.**
Authenticated domains can be associated with (i.e. assigned to) subusers from a parent account. This functionality allows subusers to send mail using their parent's domain. To associate an authenticated domain with a subuser, the parent account must first authenticate and validate the domain. The parent may then associate the authenticated domain via the subuser management tools.

[You can associate more than one domain with a subuser using the `v3/whitelabel/domains/{domain_id}/subuser:add` endpoint](https://www.twilio.com/docs/sendgrid/api-reference/domain-authentication/associate-an-authenticated-domain-with-a-subuser-multiple).
- **`sendgrid-pp-cli whitelabel associate-subuser-with-domain-multiple`** - **This endpoint allows you to associate a specific authenticated domain with a subuser. It can be used to associate up to five authenticated domains.**

This functionality allows subusers to send mail using their parent's domain. Authenticated domains can be associated with (i.e. assigned to) subusers from a parent account. To associate an authenticated domain with a subuser, the parent account must first authenticate and validate the domain. The parent may then associate the authenticated domain via the subuser management tools.

A subuser can have up to five associated authenticated domains. To see the domains that have already been associated with this user, you can [use the API to list the domains currently associated with the subuser](https://www.twilio.com/docs/sendgrid/api-reference/domain-authentication/list-the-authenticated-domain-associated-with-a-subuser-multiple).

When selecting a domain to send email from, SendGrid checks for domains in the following order and chooses the first one that appears in the hierarchy: 
1. Domain assigned by the subuser that matches the email's `From` address domain. 
2. The subuser's default domain. 
3. Domain assigned by the parent user that matches the `From` address domain. 
4. Parent user's default domain. 
5. sendgrid.net
- **`sendgrid-pp-cli whitelabel authenticate-domain`** - **This endpoint allows you to authenticate a domain.**

If you are authenticating a domain for a subuser, you have two options:
1. Use the "username" parameter. This allows you to authenticate a domain on behalf of your subuser. This means the subuser is able to see and modify the authenticated domain.
2. Use the Association workflow (see Associate Domain section). This allows you to authenticate a domain created by the parent to a subuser. This means the subuser will default to the assigned domain, but will not be able to see or modify that authenticated domain. However, if the subuser authenticates their own domain it will overwrite the assigned domain.
- **`sendgrid-pp-cli whitelabel create-branded-link`** - **This endpoint allows you to create a new branded link.**

To create the link branding, supply the root domain and, optionally, the subdomain — these go into separate fields in your request body. The root domain should match your FROM email address. If you provide a  subdomain, it must be different from the subdomain you used for authenticating your domain.

You can submit this request as one of your subusers if you include their ID in the `on-behalf-of` header in the request.
- **`sendgrid-pp-cli whitelabel delete-authenticated-domain`** - **This endpoint allows you to delete an authenticated domain.**
- **`sendgrid-pp-cli whitelabel delete-branded-link`** - **This endpoint allows you to delete a branded link.**

Your request will receive a response with a 204 status code if the deletion was successful. The call does not return the link's details, so if you wish to record these make sure you call the  "Retrieve a branded link" endpoint *before* you request its deletion.

You can submit this request as one of your subusers if you include their ID in the `on-behalf-of` header in the request.
- **`sendgrid-pp-cli whitelabel delete-ip-from-authenticated-domain`** - **This endpoint allows you to remove an IP address from that domain's authentication.**
- **`sendgrid-pp-cli whitelabel delete-reverse-dns`** - **This endpoint allows you to delete a reverse DNS record.**

A call to this endpoint will respond with a 204 status code if the deletion was successful.

You can retrieve the IDs associated with all your reverse DNS records using the "Retrieve all reverse DNS records" endpoint.
- **`sendgrid-pp-cli whitelabel disassociate-authenticated-domain-from-user`** - **This endpoint allows you to disassociate a specific authenticated domain from a subuser.**

Authenticated domains can be associated with (i.e. assigned to) subusers from a parent account. This functionality allows subusers to send mail using their parent's domain. To associate an authenticated domain with a subuser, the parent account must first authenticate and validate the domain. The parent may then associate the authenticated domain via the subuser management tools.

Note that if you used the [`/v3/whitelabel/domains/{domain_id}/subuser:add` endpoint](https://www.twilio.com/docs/sendgrid/api-reference/domain-authentication/associate-an-authenticated-domain-with-a-subuser-multiple) to add multiple domains to the subuser, you should use the [`/v3/whitelabel/domains/{domain_id}/subuser` endpoint](https://www.twilio.com/docs/sendgrid/api-reference/domain-authentication/disassociate-an-authenticated-domain-from-a-subuser-multiple) to disassociate those domains.
- **`sendgrid-pp-cli whitelabel disassociate-branded-link-from-subuser`** - **This endpoint allows you to take a branded link away from a subuser.**

Link branding can be associated with subusers from the parent account. This functionality allows subusers to send mail using their parent's link branding. To associate link branding, the parent account must first create a branded link and validate it. The parent may then associate that branded link with a subuser via the API or the [Subuser Management page of the Twilio SendGrid App](https://app.sendgrid.com/settings/subusers).

Your request will receive a response with a 204 status code if the disassociation was successful.
- **`sendgrid-pp-cli whitelabel disassociate-subuser-from-domain`** - **This endpoint allows you to disassociate a specific authenticated domain from a subuser, for users with up to five associated domains.**

This functionality allows subusers to send mail using their parent's domain. Authenticated domains can be associated with (i.e. assigned to) subusers kknt, and a subuser can have up to five associated authenticated domains. 

You can dissociate an authenticated domain from any subuser that has one or more authenticated domains using this endpoint.
- **`sendgrid-pp-cli whitelabel email-dns-record`** - **This endpoint is used to share DNS records with a colleagues**

Use this endpoint to send SendGrid-generated DNS record information to a co-worker so they can enter it into your DNS provider to validate your domain and link branding. 

What type of records are sent will depend on whether you have chosen Automated Security or not. When using Automated Security, SendGrid provides you with three CNAME records. If you turn Automated Security off, you are instead given TXT and MX records.

If you pass a `link_id` to this endpoint, the generated email will supply the DNS records necessary to complete [Link Branding](https://sendgrid.com/docs/ui/account-and-settings/how-to-set-up-link-branding/) setup. If you pass a `domain_id` to this endpoint, the generated email will supply the DNS records needed to complete [Domain Authentication](https://sendgrid.com/docs/ui/account-and-settings/how-to-set-up-domain-authentication/). Passing both IDs will generate an email with the records needed to complete both setup steps.

You can retrieve all your domain IDs from the returned `id` fields for each domain using the "List all authenticated domains" endpoint. You can retrieve all of your link IDs using the "Retrieve all branded links" endpoint.
- **`sendgrid-pp-cli whitelabel get-authenticated-domain`** - **This endpoint allows you to retrieve a specific authenticated domain.**
- **`sendgrid-pp-cli whitelabel get-branded-link`** - **This endpoint allows you to retrieve a specific branded link by providing its ID.**

You can submit this request as one of your subusers if you include their ID in the `on-behalf-of` header in the request.
- **`sendgrid-pp-cli whitelabel get-reverse-dns`** - **This endpoint allows you to retrieve a reverse DNS record.**

You can retrieve the IDs associated with all your reverse DNS records using the "Retrieve all reverse DNS records" endpoint.
- **`sendgrid-pp-cli whitelabel list-all-authenticated-domain-with-user`** - **This endpoint allows you to retrieve all of the authenticated domains that have been assigned to a specific subuser.**

This functionality allows subusers to send mail using their parent's domain. Authenticated domains can be associated with (i.e. assigned to) subusers from a parent account, and a subuser can have up to five associated domains. 

To associate an authenticated domain with a subuser, the parent account must first authenticate and validate the domain. The parent may then associate the authenticated domain via the subuser management tools.

When selecting a domain to send email from, SendGrid checks for domains in the following order and chooses the first one that appears in the hierarchy: 
1. Domain assigned by the subuser that matches the email's `From` address domain. 
2. The subuser's default domain. 
3. Domain assigned by the parent user that matches the `From` address domain. 
4. Parent user's default domain. 
5. sendgrid.net
- **`sendgrid-pp-cli whitelabel list-authenticated-domain`** - **This endpoint allows you to retrieve a paginated list of all domains you have authenticated.**

You can use the `limit` query parameter to set the page size. If your list contains more items than the page size permits, you can make multiple requests. Use the `offset` query parameter to control the position in the list from which to start retrieving additional items.
- **`sendgrid-pp-cli whitelabel list-authenticated-domain-with-user`** - **This endpoint allows you to retrieve all of the authenticated domains that have been assigned to a specific subuser.**

Authenticated domains can be associated with (i.e. assigned to) subusers from a parent account. This functionality allows subusers to send mail using their parent's domain. To associate an authenticated domain with a subuser, the parent account must first authenticate and validate the domain. The parent may then associate the authenticated domain via the subuser management tools.

Note that if you used the [`/v3/whitelabel/domains/{domain_id}/subuser:add` endpoint]( https://www.twilio.com/docs/sendgrid/api-reference/domain-authentication/associate-an-authenticated-domain-with-a-subuser-multiple) to add multiple domains to the subuser, you can use the [`/v3/whitelabel/domains/subuser/all` endpoint](https://www.twilio.com/docs/sendgrid/api-reference/domain-authentication/list-the-authenticated-domain-associated-with-a-subuser-multiple) to list those associated domains.
- **`sendgrid-pp-cli whitelabel list-branded-link`** - **This endpoint allows you to retrieve all branded links**.

You can submit this request as one of your subusers if you include their ID in the `on-behalf-of` header in the request.
- **`sendgrid-pp-cli whitelabel list-default-authenticated-domain`** - **This endpoint allows you to retrieve the default authentication for a domain.**

When creating or updating a domain authentication, you can set the domain as a default. The default domain will be used to send all mail. If you have multiple authenticated domains, the authenticated domain matching the domain of the From address will be used, and the default will be overridden.

This endpoint will return a default domain and its details only if a default is set. You are not required to set a default. If you do not set a default domain, this endpoint will return general information about your domain authentication status.
- **`sendgrid-pp-cli whitelabel list-default-branded-link`** - **This endpoint allows you to retrieve the default branded link.**

The default branded link is the actual URL to be used when sending messages. If you have more than one branded link, the default is determined by the following order:

* The validated branded link marked as `default` (set when you call the "Create a branded link" endpoint or by calling the "Update a branded link" endpoint on an existing link)
* Legacy branded links (migrated from the whitelabel wizard)
* Default SendGrid-branded links (i.e., `100.ct.sendgrid.net`)

You can submit this request as one of your subusers if you include their ID in the `on-behalf-of` header in the request.
- **`sendgrid-pp-cli whitelabel list-reverse-dns`** - **This endpoint allows you to retrieve a paginated list of all the Reverse DNS records created by this account.**

You may include a search key by using the `ip` query string parameter. This enables you to perform a prefix search for a given IP segment (e.g., `?ip="192."`).

You can use the `limit` query parameter to set the page size. If your list contains more items than the page size permits, you can make multiple requests. Use the `offset` query parameter to control the position in the list from which to start retrieving additional items.
- **`sendgrid-pp-cli whitelabel list-subuser-branded-link`** - **This endpoint allows you to retrieve the branded link associated with a subuser.**

Link branding can be associated with subusers from the parent account. This functionality allows subusers to send mail using their parent's link branding. To associate link branding, the parent account must first create a branded link and then validate it. The parent may then associate that branded link with a subuser via the API or the [Subuser Management page of the Twilio SendGrid App](https://app.sendgrid.com/settings/subusers).
- **`sendgrid-pp-cli whitelabel set-up-reverse-dns`** - **This endpoint allows you to set up reverse DNS.**
- **`sendgrid-pp-cli whitelabel update-authenticated-domain`** - **This endpoint allows you to update the settings for an authenticated domain.**
- **`sendgrid-pp-cli whitelabel update-branded-link`** - **This endpoint allows you to update a specific branded link. You can use this endpoint to change a branded link's default status.**

You can submit this request as one of your subusers if you include their ID in the `on-behalf-of` header in the request.
- **`sendgrid-pp-cli whitelabel validate-authenticated-domain`** - **This endpoint allows you to validate an authenticated domain. If it fails, it will return an error message describing why the domain could not be validated.**
- **`sendgrid-pp-cli whitelabel validate-branded-link`** - **This endpoint allows you to validate a branded link.**

You can submit this request as one of your subusers if you include their ID in the `on-behalf-of` header in the request.
- **`sendgrid-pp-cli whitelabel validate-reverse-dns`** - **This endpoint allows you to validate a reverse DNS record.**

Always check the `valid` property of the response’s `validation_results.a_record` object. This field will indicate whether it was possible to validate the reverse DNS record. If the `validation_results.a_record.valid` is `false`, this indicates only that Twilio SendGrid could not determine the validity your reverse DNS record — it may still be valid.

If validity couldn’t be determined, you can check the value of `validation_results.a_record.reason` to find out why.

You can retrieve the IDs associated with all your reverse DNS records using the "Retrieve all reverse DNS records" endpoint.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sendgrid-pp-cli alerts list

# JSON for scripting and agents
sendgrid-pp-cli alerts list --json

# Filter to specific fields
sendgrid-pp-cli alerts list --json --select id,name,status

# Dry run — show the request without sending
sendgrid-pp-cli alerts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sendgrid-pp-cli alerts list --agent
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
sendgrid-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/twilio-sendgrid-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SENDGRID_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sendgrid-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SENDGRID_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 authorization required** — Set SENDGRID_API_KEY in the environment (export SENDGRID_API_KEY=SG.xxxxxx).
- **403 on a marketing endpoint** — Your API key lacks the marketing scope; create a key with marketing.read and marketing.send.
- **429 on activity endpoints** — Email Activity API is capped at 6 req/min as of 2025-12-09. Use activity tail which respects the cap.
- **Subuser on-behalf-of header returns 401 on mail/send** — SendGrid disallows on-behalf-of with mail/send. Use the subuser's own API key.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**sendgrid-cli**](https://github.com/sendgrid/sendgrid-cli) — Bash
- [**tddschn/sendgrid-cli**](https://github.com/tddschn/sendgrid-cli) — Python
- [**Garoth/sendgrid-mcp**](https://github.com/Garoth/sendgrid-mcp) — TypeScript
- [**garethcull/sendgrid-mcp**](https://github.com/garethcull/sendgrid-mcp) — Python
- [**deyikong/sendgrid-mcp**](https://github.com/deyikong/sendgrid-mcp) — JavaScript
- [**sendgrid-go**](https://github.com/sendgrid/sendgrid-go) — Go
- [**sendgrid-nodejs**](https://github.com/sendgrid/sendgrid-nodejs) — JavaScript
- [**sendgrid-python**](https://github.com/sendgrid/sendgrid-python) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
