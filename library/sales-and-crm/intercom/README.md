# Intercom CLI

**Every Intercom resource as a typed CLI with offline sync and EU/AU regions.**

This CLI exposes the full Intercom REST API as a single Go binary. Every endpoint is a subcommand. A `sync` command mirrors conversations, contacts, companies, tickets, and articles into a local SQLite database so offline `search` and four headline transcendence commands — `incident-tag`, `articles pull/push`, `contact 360`, and `conversations sla` — answer the questions the API alone can't. Region-aware (`--region us|eu|au`) and built for agents (JSON-first, typed exit codes, MCP-mirrored).

Learn more at [Intercom](https://developers.intercom.com).

Created by [@rob-coco](https://github.com/rob-coco) (Rob Zehner).

## Install

The recommended path installs both the `intercom-pp-cli` binary and the `pp-intercom` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install intercom
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install intercom --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install intercom --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install intercom --agent claude-code
npx -y @mvanhorn/printing-press-library install intercom --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/intercom/cmd/intercom-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/intercom-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install intercom --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-intercom --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-intercom --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install intercom --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/intercom-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `INTERCOM_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "intercom": {
      "command": "intercom-pp-mcp",
      "env": {
        "INTERCOM_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Intercom uses Bearer access tokens (PAT-style). Export `INTERCOM_ACCESS_TOKEN` or run `intercom-pp-cli auth set-token "<token>"` to save it to your config. The CLI pins the `Intercom-Version: 2.13` header on every request. Tokens are workspace-scoped; pick the right region with `--region us|eu|au` or `INTERCOM_REGION`.

## Quick Start

```bash
# Save your access token; or just export INTERCOM_ACCESS_TOKEN; verify with `doctor`.
intercom-pp-cli auth set-token "$INTERCOM_ACCESS_TOKEN"

# Confirm auth + reachability before anything destructive.
intercom-pp-cli doctor

# Mirror the four big resources into the local store so offline queries work.
intercom-pp-cli sync --resources contacts,conversations,companies --since 30d

# One nested payload joining a contact's companies, conversations, tickets, notes, and tags.
intercom-pp-cli contact 360 user@example.com --agent

# SLA metrics computed locally from synced conversations + parts.
intercom-pp-cli conversations sla --group-by team --metric first-response,resolution --since 7d --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Incident response and bulk ops
- **`conversations incident-tag`** — Tag every conversation mentioning a phrase in a time window with one safe-by-default command.

  _When an incident hits, tagging 50 conversations by hand is the bottleneck; this is the single command that closes the loop without spreadsheets._

  ```bash
  intercom-pp-cli conversations incident-tag --mentions "checkout fails" --since 24h --tag incident-2026-05-24 --apply
  ```

### Help center as code
- **`articles pull`** — Flatten every help-center article into a markdown tree you can edit in git, then push changes back.

  _Lets docs teams version-control help-center content alongside product code; multilingual articles land as sibling files._

  ```bash
  intercom-pp-cli articles pull --to ./articles/
  ```

### Local-only analytics
- **`contact 360`** — One nested payload joining a contact across companies, conversations, tickets, notes, and tags.

  _Replaces 4-6 separate API calls and a mental merge; this is the lookup an agent makes before every triage decision._

  ```bash
  intercom-pp-cli contact 360 mei@example.com --agent
  ```
- **`conversations sla`** — First-response and resolution-time metrics grouped by team or admin, computed over the local store.

  _Replaces the weekly export-to-spreadsheet ritual; the API alone can't compute this in one call._

  ```bash
  intercom-pp-cli conversations sla --group-by team --metric first-response,resolution --since 7d --agent
  ```

## Recipes

### Tag every conversation mentioning an outage

```bash
intercom-pp-cli conversations incident-tag --mentions "checkout fails" --since 24h --tag incident-2026-05-24 --apply
```

Search server-side, dry-run by default, --apply fans out tag mutations through the adaptive limiter.

### Find slow-response teams this week

```bash
intercom-pp-cli conversations sla --group-by team --metric first-response,resolution --since 7d --agent --select team,first_response_p50_minutes,resolution_p90_hours
```

Local SQL over conversations + parts; --select narrows the nested payload so agents don't burn context.

### Pull every help-center article into git

```bash
intercom-pp-cli articles pull --to ./articles/ && git add ./articles/ && git commit -m 'Snapshot Intercom articles'
```

Each article lands as `<id>-<slug>.<locale>.md` with YAML frontmatter; multilingual variants are sibling files.

### Resolve a contact before triaging

```bash
intercom-pp-cli contact 360 mei@example.com --agent --select contact.id,companies,open_conversations,recent_tickets
```

Joins five entities locally; replaces 4-6 separate API calls and removes the agent's need to template Intercom's nested search predicate.

### List conversations updated this week with --select to keep payload small

```bash
intercom-pp-cli conversations list --agent --select conversations.id,conversations.state,conversations.assignee.id,conversations.tags.id,conversations.title
```

Generated endpoint mirror; `--select` uses dotted paths to walk Intercom's nested envelope so agents don't load 30 KB per conversation.

## Known Gaps

These are real user-facing constraints discovered during live verification — not CLI defects, but worth knowing before you reach for the commands:

- **`contacts search`, `conversations search`, `tickets search`** require a JSON predicate, not a plain string. Pass `--query '{"field":"role","operator":"=","value":"user"}'` (Intercom's nested-predicate shape). A bare keyword string returns `HTTP 400: Invalid query. Ensure 'field', 'operator', 'value' are present`.
- **`internal-articles list` and `internal-articles search`** require Intercom API version 2.14 or later. This CLI pins to 2.13, so these two commands return `HTTP 400: Requested resource is not available in current API version`. Use `articles list` (the public help-center endpoint) instead, or wait for the next regen against a 2.14+ spec.
- **`news list-items` and `news list-newsfeeds`** require the Intercom News feature, which is plan-gated. Workspaces without the News add-on get `HTTP 404 /news/news_items`. Check your workspace plan before reaching for these.
- **`events list-data --type <value>`** expects the `--type` flag to be an Intercom enum (`user`, `admin`, etc.), not free text. Unsupported values return `HTTP 422: unsupported type [...]`.
- **`admins list-activity-logs --created-at-after <value>`** expects a Unix epoch integer (e.g. `1730000000`), not a date string.

## Usage

Run `intercom-pp-cli --help` for the full command reference and flag list.

## Commands

### admins

Everything about your Admins

- **`intercom-pp-cli admins list`** - You can fetch a list of admins for a given workspace.
- **`intercom-pp-cli admins list-activity-logs`** - You can get a log of activities by all admins in an app.
- **`intercom-pp-cli admins retrieve`** - You can retrieve the details of a single admin.

### ai

Manage ai

- **`intercom-pp-cli ai create-content-import-source`** - You can create a new content import source by sending a POST request to this endpoint.
- **`intercom-pp-cli ai create-external-page`** - You can create a new external page by sending a POST request to this endpoint. If an external page already exists with the specified source_id and external_id, it will be updated instead.
- **`intercom-pp-cli ai delete-content-import-source`** - You can delete a content import source by making a DELETE request this endpoint. This will also delete all external pages that were imported from this source.
- **`intercom-pp-cli ai delete-external-page`** - Sending a DELETE request for an external page will remove it from the content library UI and from being used for AI answers.
- **`intercom-pp-cli ai get-content-import-source`** - Retrieve a content import source
- **`intercom-pp-cli ai get-external-page`** - You can retrieve an external page.
- **`intercom-pp-cli ai list-content-import-sources`** - You can retrieve a list of all content import sources for a workspace.
- **`intercom-pp-cli ai list-external-pages`** - You can retrieve a list of all external pages for a workspace.
- **`intercom-pp-cli ai update-content-import-source`** - You can update an existing content import source.
- **`intercom-pp-cli ai update-external-page`** - You can update an existing external page (if it was created via the API).

### articles

Everything about your Articles

- **`intercom-pp-cli articles create`** - You can create a new article by making a POST request to `https://api.intercom.io/articles`.
- **`intercom-pp-cli articles delete`** - You can delete a single article by making a DELETE request to `https://api.intercom.io/articles/<id>`.
- **`intercom-pp-cli articles list`** - You can fetch a list of all articles by making a GET request to `https://api.intercom.io/articles`.

> 📘 How are the articles sorted and ordered?
>
> Articles will be returned in descending order on the `updated_at` attribute. This means if you need to iterate through results then we'll show the most recently updated articles first.
- **`intercom-pp-cli articles retrieve`** - You can fetch the details of a single article by making a GET request to `https://api.intercom.io/articles/<id>`.
- **`intercom-pp-cli articles search`** - You can search for articles by making a GET request to `https://api.intercom.io/articles/search`.
- **`intercom-pp-cli articles update`** - You can update the details of a single article by making a PUT request to `https://api.intercom.io/articles/<id>`.

### companies

Everything about your Companies

- **`intercom-pp-cli companies create-or-update-company`** - You can create or update a company.

Companies will be only visible in Intercom when there is at least one associated user.

Companies are looked up via `company_id` in a `POST` request, if not found via `company_id`, the new company will be created, if found, that company will be updated.

{% admonition type="warning" name="Using `company_id`" %}
  You can set a unique `company_id` value when creating a company. However, it is not possible to update `company_id`. Be sure to set a unique value once upon creation of the company.
{% /admonition %}
- **`intercom-pp-cli companies delete-company`** - Delete a single company.

This endpoint does not permanently remove the company. It archives the company record and detaches any contacts attached to it; the contacts themselves are not deleted. A `company.deleted` webhook is sent once archival completes.

The endpoint returns `200` with `"deleted": true` as soon as the request is accepted — archival is processed asynchronously.

{% admonition type="warning" %}
Third-party integrations that sync companies into Intercom (for example, Salesforce or Chargebee) will recreate any company deleted through this endpoint on their next sync. To prevent recreation, remove or filter the company at the source integration before deleting it via the API.
{% /admonition %}
- **`intercom-pp-cli companies list-all`** - You can list companies. The company list is sorted by the `last_request_at` field and by default is ordered descending, most recently requested first.

Note that the API does not include companies who have no associated users in list responses.

When using the Companies endpoint and the pages object to iterate through the returned companies, there is a limit of 10,000 Companies that can be returned. If you need to list or iterate on more than 10,000 Companies, please use the [Scroll API](https://developers.intercom.com/reference#iterating-over-all-companies).
{% admonition type="warning" name="Pagination" %}
  You can use pagination to limit the number of results returned. The default is `20` results per page.
  See the [pagination section](https://developers.intercom.com/docs/build-an-integration/learn-more/rest-apis/pagination/#pagination-for-list-apis) for more details on how to use the `starting_after` param.
{% /admonition %}
- **`intercom-pp-cli companies retrieve-acompany-by-id`** - You can fetch a single company.
- **`intercom-pp-cli companies retrieve-company`** - You can fetch a single company by passing in `company_id` or `name`.

  `https://api.intercom.io/companies?name={name}`

  `https://api.intercom.io/companies?company_id={company_id}`

You can fetch all companies and filter by `segment_id` or `tag_id` as a query parameter.

  `https://api.intercom.io/companies?tag_id={tag_id}`

  `https://api.intercom.io/companies?segment_id={segment_id}`
- **`intercom-pp-cli companies scroll-over-all`** - The `list all companies` functionality does not work well for huge datasets, and can result in errors and performance problems when paging deeply. The Scroll API provides an efficient mechanism for iterating over all companies in a dataset.

- Each app can only have 1 scroll open at a time. You'll get an error message if you try to have more than one open per app.
- If the scroll isn't used for 1 minute, it expires and calls with that scroll param will fail
- If the end of the scroll is reached, "companies" will be empty and the scroll parameter will expire

{% admonition type="info" name="Scroll Parameter" %}
  You can get the first page of companies by simply sending a GET request to the scroll endpoint.
  For subsequent requests you will need to use the scroll parameter from the response.
{% /admonition %}
{% admonition type="danger" name="Scroll network timeouts" %}
  Since scroll is often used on large datasets network errors such as timeouts can be encountered. When this occurs you will see a HTTP 500 error with the following message:
  "Request failed due to an internal network error. Please restart the scroll operation."
  If this happens, you will need to restart your scroll query: It is not possible to continue from a specific point when using scroll.
{% /admonition %}
- **`intercom-pp-cli companies update-company`** - You can update a single company using the Intercom provisioned `id`.

{% admonition type="warning" name="Using `company_id`" %}
  When updating a company it is not possible to update `company_id`. This can only be set once upon creation of the company.
{% /admonition %}

### contacts

Everything about your contacts

- **`intercom-pp-cli contacts create`** - You can create a new contact (ie. user or lead).
- **`intercom-pp-cli contacts delete`** - You can delete a single contact.
- **`intercom-pp-cli contacts list`** - You can fetch a list of all contacts (ie. users or leads) in your workspace.
{% admonition type="info" name="Merged contacts" %}
  Contacts that have been merged (via POST /contacts/merge) will not appear in list results. Only the target contact from the merge remains accessible.
{% /admonition %}
{% admonition type="warning" name="Pagination" %}
  You can use pagination to limit the number of results returned. The default is `50` results per page.
  See the [pagination section](https://developers.intercom.com/docs/build-an-integration/learn-more/rest-apis/pagination/#pagination-for-list-apis) for more details on how to use the `starting_after` param.
{% /admonition %}
- **`intercom-pp-cli contacts merge`** - You can merge a contact with a `role` of `lead` into a contact with a `role` of `user`.

{% admonition type="warning" name="Merged contacts are not retrievable via the API" %}
  Once a merge is completed, the source contact (`from`) is permanently removed from the active contact list. This means:
  - **GET /contacts/{id}** — Requesting the source contact by its original ID will return a `404 Not Found` error.
  - **POST /contacts/search** — The source contact will not appear in search results, including queries filtered by `updated_at`.
  - **GET /contacts** — The source contact will not appear in list results.

  Only the target contact (`into`) remains accessible. If your application stores contact IDs, update them to use the target contact's ID after a merge.
{% /admonition %}
- **`intercom-pp-cli contacts search`** - You can search for multiple contacts by the value of their attributes in order to fetch exactly who you want.

To search for contacts, you need to send a `POST` request to `https://api.intercom.io/contacts/search`.

This will accept a query object in the body which will define your filters in order to search for contacts.

{% admonition type="warning" name="Optimizing search queries" %}
  Search queries can be complex, so optimizing them can help the performance of your search.
  Use the `AND` and `OR` operators to combine multiple filters to get the exact results you need and utilize
  pagination to limit the number of results returned. The default is `50` results per page.
  See the [pagination section](https://developers.intercom.com/docs/build-an-integration/learn-more/rest-apis/pagination/#example-search-conversations-request) for more details on how to use the `starting_after` param.
{% /admonition %}
### Merged Contacts

Contacts that have been merged (via POST /contacts/merge) are excluded from search results. If a contact was recently merged into another, it will no longer appear in queries filtered by `updated_at` or any other field. Only the target contact from the merge remains searchable.

### Contact Creation Delay

If a contact has recently been created, there is a possibility that it will not yet be available when searching. This means that it may not appear in the response. This delay can take a few minutes. If you need to be instantly notified it is recommended to use webhooks and iterate to see if they match your search filters.

### Nesting & Limitations

You can nest these filters in order to get even more granular insights that pinpoint exactly what you need. Example: (1 OR 2) AND (3 OR 4).
There are some limitations to the amount of multiple's there can be:
* There's a limit of max 2 nested filters
* There's a limit of max 15 filters for each AND or OR group

### Searching for Timestamp Fields

All timestamp fields (created_at, updated_at etc.) are indexed as Dates for Contact Search queries; Datetime queries are not currently supported. This means you can only query for timestamp fields by day - not hour, minute or second.
For example, if you search for all Contacts with a created_at value greater (>) than 1577869200 (the UNIX timestamp for January 1st, 2020 9:00 AM), that will be interpreted as 1577836800 (January 1st, 2020 12:00 AM). The search results will then include Contacts created from January 2nd, 2020 12:00 AM onwards.
If you'd like to get contacts created on January 1st, 2020 you should search with a created_at value equal (=) to 1577836800 (January 1st, 2020 12:00 AM).
This behaviour applies only to timestamps used in search queries. The search results will still contain the full UNIX timestamp and be sorted accordingly.

### Accepted Fields

Most key listed as part of the Contacts Model are searchable, whether writeable or not. The value you search for has to match the accepted type, otherwise the query will fail (ie. as `created_at` accepts a date, the `value` cannot be a string such as `"foorbar"`).

| Field                              | Type                           |
| ---------------------------------- | ------------------------------ |
| id                                 | String                         |
| role                               | String<br>Accepts user or lead |
| name                               | String                         |
| avatar                             | String                         |
| owner_id                           | Integer                        |
| email                              | String                         |
| email_domain                       | String                         |
| phone                              | String                         |
| external_id                        | String                         |
| created_at                         | Date (UNIX Timestamp)          |
| signed_up_at                       | Date (UNIX Timestamp)          |
| updated_at                         | Date (UNIX Timestamp)          |
| last_seen_at                       | Date (UNIX Timestamp)          |
| last_contacted_at                  | Date (UNIX Timestamp)          |
| last_replied_at                    | Date (UNIX Timestamp)          |
| last_email_opened_at               | Date (UNIX Timestamp)          |
| last_email_clicked_at              | Date (UNIX Timestamp)          |
| language_override                  | String                         |
| browser                            | String                         |
| browser_language                   | String                         |
| os                                 | String                         |
| location.country                   | String                         |
| location.region                    | String                         |
| location.city                      | String                         |
| unsubscribed_from_emails           | Boolean                        |
| marked_email_as_spam               | Boolean                        |
| has_hard_bounced                   | Boolean                        |
| ios_last_seen_at                   | Date (UNIX Timestamp)          |
| ios_app_version                    | String                         |
| ios_device                         | String                         |
| ios_app_device                     | String                         |
| ios_os_version                     | String                         |
| ios_app_name                       | String                         |
| ios_sdk_version                    | String                         |
| android_last_seen_at               | Date (UNIX Timestamp)          |
| android_app_version                | String                         |
| android_device                     | String                         |
| android_app_name                   | String                         |
| andoid_sdk_version                 | String                         |
| segment_id                         | String                         |
| tag_id                             | String                         |
| custom_attributes.{attribute_name} | String                         |

### Accepted Operators

{% admonition type="warning" name="Searching based on `created_at`" %}
  You cannot use the `<=` or `>=` operators to search by `created_at`.
{% /admonition %}

The table below shows the operators you can use to define how you want to search for the value.  The operator should be put in as a string (`"="`). The operator has to be compatible with the field's type (eg. you cannot search with `>` for a given string value as it's only compatible for integer's and dates).

| Operator | Valid Types                      | Description                                                      |
| :------- | :------------------------------- | :--------------------------------------------------------------- |
| =        | All                              | Equals                                                           |
| !=       | All                              | Doesn't Equal                                                    |
| IN       | All                              | In<br>Shortcut for `OR` queries<br>Values must be in Array       |
| NIN      | All                              | Not In<br>Shortcut for `OR !` queries<br>Values must be in Array |
| >        | Integer<br>Date (UNIX Timestamp) | Greater than                                                     |
| <       | Integer<br>Date (UNIX Timestamp) | Lower than                                                       |
| ~        | String                           | Contains                                                         |
| !~       | String                           | Doesn't Contain                                                  |
| ^        | String                           | Starts With                                                      |
| $        | String                           | Ends With                                                        |
- **`intercom-pp-cli contacts show`** - You can fetch the details of a single contact.

{% admonition type="warning" name="Merged contacts" %}
  If a contact has been merged into another contact via the Merge endpoint (POST /contacts/merge), requesting it by its original ID will return a `404 Not Found` error. Use the merged-into contact's ID instead.
{% /admonition %}
- **`intercom-pp-cli contacts show-by-external-id`** - You can fetch the details of a single contact by external ID. Note that this endpoint only supports users and not leads.
- **`intercom-pp-cli contacts update`** - You can update an existing contact (ie. user or lead).

{% admonition type="info" %}
  This endpoint handles both **contact updates** and **custom object associations**.

  See _`update a contact with an association to a custom object instance`_ in the request/response examples to see the custom object association format.
{% /admonition %}

### conversations

Everything about your Conversations

- **`intercom-pp-cli conversations create`** - You can create a conversation that has been initiated by a contact (ie. user or lead).
The conversation can be an in-app message only.

{% admonition type="info" name="Sending for visitors" %}
You can also send a message from a visitor by specifying their `user_id` or `id` value in the `from` field, along with a `type` field value of `contact`.
This visitor will be automatically converted to a contact with a lead role once the conversation is created.
{% /admonition %}

This will return the Message model that has been created.
- **`intercom-pp-cli conversations delete`** - {% admonition type="warning" name="Irreversible operation" %}
Deleting a conversation is permanent and cannot be reversed.
{% /admonition %}

Deleting a conversation permanently removes it from the inbox. All sensitive data is deleted, including admin and user replies, conversation attributes, uploads, and related content. The conversation will still appear in reporting, though some data may be incomplete due to the deletion.
- **`intercom-pp-cli conversations list`** - You can fetch a list of all conversations.

You can optionally request the result page size and the cursor to start after to fetch the result.
{% admonition type="warning" name="Pagination" %}
  You can use pagination to limit the number of results returned. The default is `20` results per page.
  See the [pagination section](https://developers.intercom.com/docs/build-an-integration/learn-more/rest-apis/pagination/#pagination-for-list-apis) for more details on how to use the `starting_after` param.
{% /admonition %}
- **`intercom-pp-cli conversations redact`** - You can redact a conversation part or the source message of a conversation (as seen in the source object).

{% admonition type="info" name="Redacting parts and messages" %}
If you are redacting a conversation part, it must have a `body`. If you are redacting a source message, it must have been created by a contact. We will return a `conversation_part_not_redactable` error if these criteria are not met.
{% /admonition %}
- **`intercom-pp-cli conversations retrieve`** - You can fetch the details of a single conversation.

This will return a single Conversation model with all its conversation parts.

{% admonition type="warning" name="Hard limit of 500 parts" %}
The maximum number of conversation parts that can be returned via the API is 500. If you have more than that we will return the 500 most recent conversation parts.
{% /admonition %}

For AI agent conversation metadata, please note that you need to have the agent enabled in your workspace, which is a [paid feature](https://www.intercom.com/help/en/articles/8205718-fin-resolutions#h_97f8c2e671).
- **`intercom-pp-cli conversations search`** - You can search for multiple conversations by the value of their attributes in order to fetch exactly which ones you want.

To search for conversations, you need to send a `POST` request to `https://api.intercom.io/conversations/search`.

This will accept a query object in the body which will define your filters in order to search for conversations.
{% admonition type="warning" name="Optimizing search queries" %}
  Search queries can be complex, so optimizing them can help the performance of your search.
  Use the `AND` and `OR` operators to combine multiple filters to get the exact results you need and utilize
  pagination to limit the number of results returned. The default is `20` results per page and maximum is `150`.
  See the [pagination section](https://developers.intercom.com/docs/build-an-integration/learn-more/rest-apis/pagination/#example-search-conversations-request) for more details on how to use the `starting_after` param.
{% /admonition %}

### Nesting & Limitations

You can nest these filters in order to get even more granular insights that pinpoint exactly what you need. Example: (1 OR 2) AND (3 OR 4).
There are some limitations to the amount of multiple's there can be:
- There's a limit of max 2 nested filters
- There's a limit of max 15 filters for each AND or OR group

### Accepted Fields

Most keys listed as part of the conversation model are searchable, whether writeable or not. The value you search for has to match the accepted type, otherwise the query will fail (ie. as `created_at` accepts a date, the `value` cannot be a string such as `"foorbar"`).
The `source.body` field is unique as the search will not be performed against the entire value, but instead against every element of the value separately. For example, when searching for a conversation with a `"I need support"` body - the query should contain a `=` operator with the value `"support"` for such conversation to be returned. A query with a `=` operator and a `"need support"` value will not yield a result.

| Field                                     | Type                                                                                                                                                   |
| :---------------------------------------- | :----------------------------------------------------------------------------------------------------------------------------------------------------- |
| id                                        | String                                                                                                                                                 |
| created_at                                | Date (UNIX timestamp)                                                                                                                                  |
| updated_at                                | Date (UNIX timestamp)                                                                                                                                  |
| source.type                               | String<br>Accepted fields are `conversation`, `email`, `facebook`, `instagram`, `phone_call`, `phone_switch`, `push`, `sms`, `twitter` and `whatsapp`. |
| source.id                                 | String                                                                                                                                                 |
| source.delivered_as                       | String                                                                                                                                                 |
| source.subject                            | String                                                                                                                                                 |
| source.body                               | String                                                                                                                                                 |
| source.author.id                          | String                                                                                                                                                 |
| source.author.type                        | String                                                                                                                                                 |
| source.author.name                        | String                                                                                                                                                 |
| source.author.email                       | String                                                                                                                                                 |
| source.url                                | String                                                                                                                                                 |
| contact_ids                               | String                                                                                                                                                 |
| teammate_ids                              | String                                                                                                                                                 |
| admin_assignee_id                         | Integer                                                                                                                                                |
| team_assignee_id                          | Integer                                                                                                                                                |
| channel_initiated                         | String                                                                                                                                                 |
| open                                      | Boolean                                                                                                                                                |
| read                                      | Boolean                                                                                                                                                |
| state                                     | String                                                                                                                                                 |
| waiting_since                             | Date (UNIX timestamp)                                                                                                                                  |
| snoozed_until                             | Date (UNIX timestamp)                                                                                                                                  |
| tag_ids                                   | String                                                                                                                                                 |
| priority                                  | String                                                                                                                                                 |
| statistics.time_to_assignment             | Integer                                                                                                                                                |
| statistics.time_to_admin_reply            | Integer                                                                                                                                                |
| statistics.time_to_first_close            | Integer                                                                                                                                                |
| statistics.time_to_last_close             | Integer                                                                                                                                                |
| statistics.median_time_to_reply           | Integer                                                                                                                                                |
| statistics.first_contact_reply_at         | Date (UNIX timestamp)                                                                                                                                  |
| statistics.first_assignment_at            | Date (UNIX timestamp)                                                                                                                                  |
| statistics.first_admin_reply_at           | Date (UNIX timestamp)                                                                                                                                  |
| statistics.first_close_at                 | Date (UNIX timestamp)                                                                                                                                  |
| statistics.last_assignment_at             | Date (UNIX timestamp)                                                                                                                                  |
| statistics.last_assignment_admin_reply_at | Date (UNIX timestamp)                                                                                                                                  |
| statistics.last_contact_reply_at          | Date (UNIX timestamp)                                                                                                                                  |
| statistics.last_admin_reply_at            | Date (UNIX timestamp)                                                                                                                                  |
| statistics.last_close_at                  | Date (UNIX timestamp)                                                                                                                                  |
| statistics.last_closed_by_id              | String                                                                                                                                                 |
| statistics.count_reopens                  | Integer                                                                                                                                                |
| statistics.count_assignments              | Integer                                                                                                                                                |
| statistics.count_conversation_parts       | Integer                                                                                                                                                |
| conversation_rating.requested_at          | Date (UNIX timestamp)                                                                                                                                  |
| conversation_rating.replied_at            | Date (UNIX timestamp)                                                                                                                                  |
| conversation_rating.score                 | Integer                                                                                                                                                |
| conversation_rating.remark                | String                                                                                                                                                 |
| conversation_rating.contact_id            | String                                                                                                                                                 |
| conversation_rating.admin_d               | String                                                                                                                                                 |
| ai_agent_participated                     | Boolean                                                                                                                                                |
| ai_agent.resolution_state                 | String                                                                                                                                                 |
| ai_agent.last_answer_type                 | String                                                                                                                                                 |
| ai_agent.rating                           | Integer                                                                                                                                                |
| ai_agent.rating_remark                    | String                                                                                                                                                 |
| ai_agent.source_type                      | String                                                                                                                                                 |
| ai_agent.source_title                     | String                                                                                                                                                 |

### Accepted Operators

The table below shows the operators you can use to define how you want to search for the value.  The operator should be put in as a string (`"="`). The operator has to be compatible with the field's type  (eg. you cannot search with `>` for a given string value as it's only compatible for integer's and dates).

| Operator | Valid Types                    | Description                                                  |
| :------- | :----------------------------- | :----------------------------------------------------------- |
| =        | All                            | Equals                                                       |
| !=       | All                            | Doesn't Equal                                                |
| IN       | All                            | In  Shortcut for `OR` queries  Values most be in Array       |
| NIN      | All                            | Not In  Shortcut for `OR !` queries  Values must be in Array |
| >        | Integer  Date (UNIX Timestamp) | Greater (or equal) than                                      |
| <       | Integer  Date (UNIX Timestamp) | Lower (or equal) than                                        |
| ~        | String                         | Contains                                                     |
| !~       | String                         | Doesn't Contain                                              |
| ^        | String                         | Starts With                                                  |
| $        | String                         | Ends With                                                    |
- **`intercom-pp-cli conversations update`** - You can update an existing conversation.

{% admonition type="info" name="Replying and other actions" %}
If you want to reply to a coveration or take an action such as assign, unassign, open, close or snooze, take a look at the reply and manage endpoints.
{% /admonition %}

{% admonition type="info" %}
  This endpoint handles both **conversation updates** and **custom object associations**.

  See _`update a conversation with an association to a custom object instance`_ in the request/response examples to see the custom object association format.
{% /admonition %}

### custom-object-instances

Everything about your Custom Object instances.
{% admonition type="warning" name="Permission Requirements" %}
  From now on, to access this endpoint, you need additional permissions. Please head over to the [Developer Hub](https://app.intercom.com/a/apps/_/developer-hub) app package authentication settings to configure the required permissions.
{% /admonition %}

- **`intercom-pp-cli custom-object-instances create`** - Create or update a custom object instance
- **`intercom-pp-cli custom-object-instances delete-by-external-id`** - Delete a single Custom Object instance using the Intercom defined id.
- **`intercom-pp-cli custom-object-instances delete-by-id`** - Delete a single Custom Object instance by external_id.
- **`intercom-pp-cli custom-object-instances get-by-external-id`** - Fetch a Custom Object Instance by external_id.
- **`intercom-pp-cli custom-object-instances get-by-id`** - Fetch a Custom Object Instance by id.

### data-attributes

Everything about your Data Attributes

- **`intercom-pp-cli data-attributes create`** - You can create a data attributes for a `contact` or a `company`.
- **`intercom-pp-cli data-attributes lis`** - You can fetch a list of all data attributes belonging to a workspace for contacts, companies or conversations.
- **`intercom-pp-cli data-attributes update`** - You can update a data attribute.

> 🚧 Updating the data type is not possible
>
> It is currently a dangerous action to execute changing a data attribute's type via the API. You will need to update the type via the UI instead.

### download

Manage download

- **`intercom-pp-cli download <job_identifier>`** - When a job has a status of complete, and thus a filled download_url, you can download your data by hitting that provided URL, formatted like so: https://api.intercom.io/download/content/data/xyz1234.

Your exported message data will be streamed continuously back down to you in a gzipped CSV format.

> 📘 Octet header required
>
> You will have to specify the header Accept: `application/octet-stream` when hitting this endpoint.

### events

Manage events

- **`intercom-pp-cli events create-data`** - You will need an Access Token that has write permissions to send Events. Once you have a key you can submit events via POST to the Events resource, which is located at https://api.intercom.io/events, or you can send events using one of the client libraries. When working with the HTTP API directly a client should send the event with a `Content-Type` of `application/json`.

When using the JavaScript API, [adding the code to your app](http://docs.intercom.io/configuring-Intercom/tracking-user-events-in-your-app) makes the Events API available. Once added, you can submit an event using the `trackEvent` method. This will associate the event with the Lead or currently logged-in user or logged-out visitor/lead and send it to Intercom. The final parameter is a map that can be used to send optional metadata about the event.

With the Ruby client you pass a hash describing the event to `Intercom::Event.create`, or call the `track_user` method directly on the current user object (e.g. `user.track_event`).

**NB: For the JSON object types, please note that we do not currently support nested JSON structure.**

| Type            | Description                                                                                                                                                                                                     | Example                                                                           |
| :-------------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :-------------------------------------------------------------------------------- |
| String          | The value is a JSON String                                                                                                                                                                                      | `"source":"desktop"`                                                              |
| Number          | The value is a JSON Number                                                                                                                                                                                      | `"load": 3.67`                                                                    |
| Date            | The key ends with the String `_date` and the value is a [Unix timestamp](http://en.wikipedia.org/wiki/Unix_time), assumed to be in the [UTC](http://en.wikipedia.org/wiki/Coordinated_Universal_Time) timezone. | `"contact_date": 1392036272`                                                      |
| Link            | The value is a HTTP or HTTPS URI.                                                                                                                                                                               | `"article": "https://example.org/ab1de.html"`                                     |
| Rich Link       | The value is a JSON object that contains `url` and `value` keys.                                                                                                                                                | `"article": {"url": "https://example.org/ab1de.html", "value":"the dude abides"}` |
| Monetary Amount | The value is a JSON object that contains `amount` and `currency` keys. The `amount` key is a positive integer representing the amount in cents. The price in the example to the right denotes €349.99.          | `"price": {"amount": 34999, "currency": "eur"}`                                   |

**Lead Events**

When submitting events for Leads, you will need to specify the Lead's `id`.

**Metadata behaviour**

- We currently limit the number of tracked metadata keys to 10 per event. Once the quota is reached, we ignore any further keys we receive. The first 10 metadata keys are determined by the order in which they are sent in with the event.
- It is not possible to change the metadata keys once the event has been sent. A new event will need to be created with the new keys and you can archive the old one.
- There might be up to 24 hrs delay when you send a new metadata for an existing event.

**Event de-duplication**

The API may detect and ignore duplicate events. Each event is uniquely identified as a combination of the following data - the Workspace identifier, the Contact external identifier, the Data Event name and the Data Event created time. As a result, it is **strongly recommended** to send a second granularity Unix timestamp in the `created_at` field.

Duplicated events are responded to using the normal `202 Accepted` code - an error is not thrown, however repeat requests will be counted against any rate limit that is in place.

### HTTP API Responses

- Successful responses to submitted events return `202 Accepted` with an empty body.
- Unauthorised access will be rejected with a `401 Unauthorized` or `403 Forbidden` response code.
- Events sent about users that cannot be found will return a `404 Not Found`.
- Event lists containing duplicate events will have those duplicates ignored.
- Server errors will return a `500` response code and may contain an error message in the body.
- **`intercom-pp-cli events data-summaries`** - Create event summaries for a user. Event summaries are used to track the number of times an event has occurred, the first time it occurred and the last time it occurred.
- **`intercom-pp-cli events lis-data`** - > 🚧
>
> Please note that you can only 'list' events that are less than 90 days old. Event counts and summaries will still include your events older than 90 days but you cannot 'list' these events individually if they are older than 90 days

The events belonging to a customer can be listed by sending a GET request to `https://api.intercom.io/events` with a user or lead identifier along with a `type` parameter. The identifier parameter can be one of `user_id`, `email` or `intercom_user_id`. The `type` parameter value must be `user`.

- `https://api.intercom.io/events?type=user&user_id={user_id}`
- `https://api.intercom.io/events?type=user&email={email}`
- `https://api.intercom.io/events?type=user&intercom_user_id={id}` (this call can be used to list leads)

The `email` parameter value should be [url encoded](http://en.wikipedia.org/wiki/Percent-encoding) when sending.

You can optionally define the result page size as well with the `per_page` parameter.

### help-center

Everything about your Help Center

- **`intercom-pp-cli help-center create-collection`** - You can create a new collection by making a POST request to `https://api.intercom.io/help_center/collections.`
- **`intercom-pp-cli help-center delete-collection`** - You can delete a single collection by making a DELETE request to `https://api.intercom.io/collections/<id>`.
- **`intercom-pp-cli help-center list`** - You can list all Help Centers by making a GET request to `https://api.intercom.io/help_center/help_centers`.
- **`intercom-pp-cli help-center list-all-collections`** - You can fetch a list of all collections by making a GET request to `https://api.intercom.io/help_center/collections`.

Collections will be returned in descending order on the `updated_at` attribute. This means if you need to iterate through results then we'll show the most recently updated collections first.
- **`intercom-pp-cli help-center retrieve`** - You can fetch the details of a single Help Center by making a GET request to `https://api.intercom.io/help_center/help_center/<id>`.
- **`intercom-pp-cli help-center retrieve-collection`** - You can fetch the details of a single collection by making a GET request to `https://api.intercom.io/help_center/collections/<id>`.
- **`intercom-pp-cli help-center update-collection`** - You can update the details of a single collection by making a PUT request to `https://api.intercom.io/collections/<id>`.

### intercom-export

Manage intercom export

- **`intercom-pp-cli intercom-export cancel-data`** - Cancel content data export
- **`intercom-pp-cli intercom-export create-data`** - To create your export job, you need to send a `POST` request to the export endpoint `https://api.intercom.io/export/content/data`.

This endpoint exports **message delivery and engagement data** for outbound content (Emails, Posts, Custom Bots, Surveys, Tours, Series, and more). The exported data includes who received each message, when they received it, and how they engaged with it (opens, clicks, replies, completions, dismissals, unsubscribes, and bounces). It does not export raw message or conversation content.

The only parameters you need to provide are the range of dates that you want exported.

>🚧 Limit of one active job
>
> You can only have one active job per workspace. You will receive a HTTP status code of 429 with the message Exceeded rate limit of 1 pending message data export jobs if you attempt to create a second concurrent job.

>❗️ Updated_at not included
>
> It should be noted that the timeframe only includes messages sent during the time period and not messages that were only updated during this period. For example, if a message was updated yesterday but sent two days ago, you would need to set the created_at_after date before the message was sent to include that in your retrieval job.

>📘 Date ranges are inclusive
>
> Requesting data for 2018-06-01 until 2018-06-30 will get all data for those days including those specified - e.g. 2018-06-01 00:00:00 until 2018-06-30 23:59:99.
- **`intercom-pp-cli intercom-export get-data`** - You can view the status of your job by sending a `GET` request to the URL
`https://api.intercom.io/export/content/data/{job_identifier}` - the `{job_identifier}` is the value returned in the response when you first created the export job. More on it can be seen in the Export Job Model.

> 🚧 Jobs expire after two days
> All jobs that have completed processing (and are thus available to download from the provided URL) will have an expiry limit of two days from when the export ob completed. After this, the data will no longer be available.

### internal-articles

Everything about your Internal Articles

- **`intercom-pp-cli internal-articles create`** - You can create a new internal article by making a POST request to `https://api.intercom.io/internal_articles`.
- **`intercom-pp-cli internal-articles delete`** - You can delete a single internal article by making a DELETE request to `https://api.intercom.io/internal_articles/<id>`.
- **`intercom-pp-cli internal-articles list`** - You can fetch a list of all internal articles by making a GET request to `https://api.intercom.io/internal_articles`.
- **`intercom-pp-cli internal-articles retrieve`** - You can fetch the details of a single internal article by making a GET request to `https://api.intercom.io/internal_articles/<id>`.
- **`intercom-pp-cli internal-articles search`** - You can search for internal articles by making a GET request to `https://api.intercom.io/internal_articles/search`.
- **`intercom-pp-cli internal-articles update`** - You can update the details of a single internal article by making a PUT request to `https://api.intercom.io/internal_articles/<id>`.

### me

Manage me

- **`intercom-pp-cli me`** - You can view the currently authorised admin along with the embedded app object (a "workspace" in legacy terminology).

> 🚧 Single Sign On
>
> If you are building a custom "Log in with Intercom" flow for your site, and you call the `/me` endpoint to identify the logged-in user, you should not accept any sign-ins from users with unverified email addresses as it poses a potential impersonation security risk.

### messages

Everything about your messages

- **`intercom-pp-cli messages`** - You can create a message that has been initiated by an admin. The conversation can be either an in-app message or an email.

> 🚧 Sending for visitors
>
> There can be a short delay between when a contact is created and when a contact becomes available to be messaged through the API. A 404 Not Found error will be returned in this case.

This will return the Message model that has been created.

> 🚧 Retrieving Associated Conversations
>
> As this is a message, there will be no conversation present until the contact responds. Once they do, you will have to search for a contact's conversations with the id of the message.

### news

Everything about your News

- **`intercom-pp-cli news create-item`** - You can create a news item
- **`intercom-pp-cli news delete-item`** - You can delete a single news item.
- **`intercom-pp-cli news list-items`** - You can fetch a list of all news items
- **`intercom-pp-cli news list-live-newsfeed-items`** - You can fetch a list of all news items that are live on a given newsfeed
- **`intercom-pp-cli news list-newsfeeds`** - You can fetch a list of all newsfeeds
- **`intercom-pp-cli news retrieve-item`** - You can fetch the details of a single news item.
- **`intercom-pp-cli news retrieve-newsfeed`** - You can fetch the details of a single newsfeed
- **`intercom-pp-cli news update-item`** - Update a news item

### notes

Everything about your Notes

- **`intercom-pp-cli notes <id>`** - You can fetch the details of a single note.

### phone-call-redirects

Manage phone call redirects

- **`intercom-pp-cli phone-call-redirects`** - You can use the API to deflect phone calls to the Intercom Messenger.
Calling this endpoint will send an SMS with a link to the Messenger to the phone number specified.

If custom attributes are specified, they will be added to the user or lead's custom data attributes.

### segments

Everything about your Segments

- **`intercom-pp-cli segments list`** - You can fetch a list of all segments.
- **`intercom-pp-cli segments retrieve`** - You can fetch the details of a single segment.

### subscription-types

Everything about subscription types

- **`intercom-pp-cli subscription-types`** - You can list all subscription types. A list of subscription type objects will be returned.

### tags

Everything about tags

- **`intercom-pp-cli tags create`** - You can use this endpoint to perform the following operations:

  **1. Create a new tag:** You can create a new tag by passing in the tag name as specified in "Create or Update Tag Request Payload" described below.

  **2. Update an existing tag:** You can update an existing tag by passing the id of the tag as specified in "Create or Update Tag Request Payload" described below.

  **3. Tag Companies:** You can tag single company or a list of companies. You can tag a company by passing in the tag name and the company details as specified in "Tag Company Request Payload" described below. Also, if the tag doesn't exist then a new one will be created automatically.

  **4. Untag Companies:** You can untag a single company or a list of companies. You can untag a company by passing in the tag id and the company details as specified in "Untag Company Request Payload" described below.

  **5. Tag Multiple Users:** You can tag a list of users. You can tag the users by passing in the tag name and the user details as specified in "Tag Users Request Payload" described below.

Each operation will return a tag object.
- **`intercom-pp-cli tags delete`** - You can delete the details of tags that are on the workspace by passing in the id.
- **`intercom-pp-cli tags find`** - You can fetch the details of tags that are on the workspace by their id.
This will return a tag object.
- **`intercom-pp-cli tags list`** - You can fetch a list of all tags for a given workspace.

### teams

Everything about your Teams

- **`intercom-pp-cli teams list`** - This will return a list of team objects for the App.
- **`intercom-pp-cli teams retrieve`** - You can fetch the details of a single team, containing an array of admins that belong to this team.

### ticket-states

Everything about your ticket states

- **`intercom-pp-cli ticket-states`** - You can get a list of all ticket states for a workspace.

### ticket-types

Everything about your ticket types

- **`intercom-pp-cli ticket-types create`** - You can create a new ticket type.
> 📘 Creating ticket types.
>
> Every ticket type will be created with two default attributes: _default_title_ and _default_description_.
> For the `icon` propery, use an emoji from [Twemoji Cheatsheet](https://twemoji-cheatsheet.vercel.app/)
- **`intercom-pp-cli ticket-types get`** - You can fetch the details of a single ticket type.
- **`intercom-pp-cli ticket-types list`** - You can get a list of all ticket types for a workspace.
- **`intercom-pp-cli ticket-types update`** - You can update a ticket type.

> 📘 Updating a ticket type.
>
> For the `icon` propery, use an emoji from [Twemoji Cheatsheet](https://twemoji-cheatsheet.vercel.app/)

### tickets

Everything about your tickets

- **`intercom-pp-cli tickets create`** - You can create a new ticket.
- **`intercom-pp-cli tickets delete`** - {% admonition type="warning" name="Irreversible operation" %}
Deleting a ticket is permanent and cannot be reversed.
{% /admonition %}

Deleting a ticket permanently removes it from the inbox. All sensitive data is deleted, including admin and user replies, ticket attributes, uploads, and related content. The ticket will still appear in reporting, though some data may be incomplete due to the deletion.
- **`intercom-pp-cli tickets get`** - You can fetch the details of a single ticket.
- **`intercom-pp-cli tickets search`** - You can search for multiple tickets by the value of their attributes in order to fetch exactly which ones you want.

To search for tickets, you send a `POST` request to `https://api.intercom.io/tickets/search`.

This will accept a query object in the body which will define your filters.
{% admonition type="warning" name="Optimizing search queries" %}
  Search queries can be complex, so optimizing them can help the performance of your search.
  Use the `AND` and `OR` operators to combine multiple filters to get the exact results you need and utilize
  pagination to limit the number of results returned. The default is `20` results per page.
  See the [pagination section](https://developers.intercom.com/docs/build-an-integration/learn-more/rest-apis/pagination/#example-search-conversations-request) for more details on how to use the `starting_after` param.
{% /admonition %}

### Nesting & Limitations

You can nest these filters in order to get even more granular insights that pinpoint exactly what you need. Example: (1 OR 2) AND (3 OR 4).
There are some limitations to the amount of multiples there can be:
- There's a limit of max 2 nested filters
- There's a limit of max 15 filters for each AND or OR group

### Accepted Fields

Most keys listed as part of the Ticket model are searchable, whether writeable or not. The value you search for has to match the accepted type, otherwise the query will fail (ie. as `created_at` accepts a date, the `value` cannot be a string such as `"foobar"`).
The `source.body` field is unique as the search will not be performed against the entire value, but instead against every element of the value separately. For example, when searching for a conversation with a `"I need support"` body - the query should contain a `=` operator with the value `"support"` for such conversation to be returned. A query with a `=` operator and a `"need support"` value will not yield a result.

| Field                                     | Type                                                                                     |
| :---------------------------------------- | :--------------------------------------------------------------------------------------- |
| id                                        | String                                                                                   |
| created_at                                | Date (UNIX timestamp)                                                                    |
| updated_at                                | Date (UNIX timestamp)                                                                    |
| title                           | String                                                                                   |
| description                     | String                                                                                   |
| category                                  | String                                                                                   |
| ticket_type_id                            | String                                                                                   |
| contact_ids                               | String                                                                                   |
| teammate_ids                              | String                                                                                   |
| admin_assignee_id                         | String                                                                                   |
| team_assignee_id                          | String                                                                                   |
| open                                      | Boolean                                                                                  |
| state                                     | String                                                                                   |
| snoozed_until                             | Date (UNIX timestamp)                                                                    |
| ticket_attribute.{id}                     | String or Boolean or Date (UNIX timestamp) or Float or Integer                           |

### Accepted Operators

{% admonition type="info" name="Searching based on `created_at`" %}
  You may use the `<=` or `>=` operators to search by `created_at`.
{% /admonition %}

The table below shows the operators you can use to define how you want to search for the value.  The operator should be put in as a string (`"="`). The operator has to be compatible with the field's type  (eg. you cannot search with `>` for a given string value as it's only compatible for integer's and dates).

| Operator | Valid Types                    | Description                                                  |
| :------- | :----------------------------- | :----------------------------------------------------------- |
| =        | All                            | Equals                                                       |
| !=       | All                            | Doesn't Equal                                                |
| IN       | All                            | In  Shortcut for `OR` queries  Values most be in Array       |
| NIN      | All                            | Not In  Shortcut for `OR !` queries  Values must be in Array |
| >        | Integer  Date (UNIX Timestamp) | Greater (or equal) than                                      |
| <       | Integer  Date (UNIX Timestamp) | Lower (or equal) than                                        |
| ~        | String                         | Contains                                                     |
| !~       | String                         | Doesn't Contain                                              |
| ^        | String                         | Starts With                                                  |
| $        | String                         | Ends With                                                    |
- **`intercom-pp-cli tickets update`** - You can update a ticket.

### visitors

Everything about your Visitors

- **`intercom-pp-cli visitors convert`** - You can merge a Visitor to a Contact of role type `lead` or `user`.

> 📘 What happens upon a visitor being converted?
>
> If the User exists, then the Visitor will be merged into it, the Visitor deleted and the User returned. If the User does not exist, the Visitor will be converted to a User, with the User identifiers replacing it's Visitor identifiers.
- **`intercom-pp-cli visitors retrieve-with-user-id`** - You can fetch the details of a single visitor.
- **`intercom-pp-cli visitors update`** - Sending a PUT request to `/visitors` will result in an update of an existing Visitor.

**Option 1.** You can update a visitor by passing in the `user_id` of the visitor in the Request body.

**Option 2.** You can update a visitor by passing in the `id` of the visitor in the Request body.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
intercom-pp-cli admins list

# JSON for scripting and agents
intercom-pp-cli admins list --json

# Filter to specific fields
intercom-pp-cli admins list --json --select id,name,status

# Dry run — show the request without sending
intercom-pp-cli admins list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
intercom-pp-cli admins list --agent
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
intercom-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/intercom-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `INTERCOM_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `intercom-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $INTERCOM_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every call** — Your token might be wrong region. Re-set it with `intercom-pp-cli auth set-token <token>` and add `--region eu` (or `au`) on every command, or `export INTERCOM_REGION=eu`.
- **429 rate-limited mid-sync** — Sync retries automatically with adaptive backoff respecting `X-RateLimit-Reset`. Re-run after a few seconds, or pass `--max-pages` to cap fan-out.
- **`conversations sla` returns empty rows** — Run `sync --resources conversations,conversation_parts` first — SLA metrics need parts to compute first-response time.
- **`articles push` reports conflicts** — Articles changed in Intercom since your last `pull`. Re-pull to refresh the manifest, then re-edit.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**intercom-client**](https://github.com/intercom/intercom-node) — TypeScript (389 stars)
- [**python-intercom**](https://github.com/intercom/python-intercom) — Python (243 stars)
- [**kaosensei/intercom-mcp**](https://github.com/kaosensei/intercom-mcp) — JavaScript (8 stars)
- [**raoulbia-ai/mcp-server-for-intercom**](https://github.com/raoulbia-ai/mcp-server-for-intercom) — TypeScript (8 stars)
- [**fabian1710/mcp-intercom**](https://github.com/fabian1710/mcp-intercom) — TypeScript (8 stars)
- [**intercom-mcp-server**](https://github.com/intercom/intercom-mcp-server) — TypeScript (5 stars)
- [**kyoji2/intercom-cli**](https://github.com/kyoji2/intercom-cli) — TypeScript (2 stars)
- [**fast-intercom-mcp**](https://github.com/evolsb/fast-intercom-mcp) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
