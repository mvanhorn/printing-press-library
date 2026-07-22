# Pipedrive CLI



Learn more at [Pipedrive](https://developers.pipedrive.com).

Created by [@debgotwired](https://github.com/debgotwired) (Deb Mukherjee).

## Install

The recommended path installs both the `pipedrive-pp-cli` binary and the `pp-pipedrive` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install pipedrive
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install pipedrive --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install pipedrive --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install pipedrive --agent claude-code
npx -y @mvanhorn/printing-press-library install pipedrive --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/pipedrive/cmd/pipedrive-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pipedrive-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install pipedrive --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pipedrive --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pipedrive --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install pipedrive --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pipedrive-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PIPEDRIVE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/pipedrive/cmd/pipedrive-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pipedrive": {
      "command": "pipedrive-pp-mcp",
      "env": {
        "PIPEDRIVE_API_KEY": "<your-key>"
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

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export PIPEDRIVE_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/github.com/mvanhorn/printing-press-library/library/sales-and-crm/pipedrive/config.toml`.

### 3. Verify Setup

```bash
pipedrive-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
pipedrive-pp-cli activity-fields
```

## Usage

Run `pipedrive-pp-cli --help` for the full command reference and flag list.

## Commands

### activity-fields

Activity fields represent different fields that an activity has.

- **`pipedrive-pp-cli activity-fields`** - Returns all activity fields.

### activity-types

Activity types represent different kinds of activities that can be stored. Each activity type is presented to the user with an icon and a name. Additionally, a color can be defined (not implemented in the Pipedrive app as of today). Activity types are linked to activities via `ActivityType.key_string = Activity.type`. The `key_string` will be generated by the API based on the given name of the activity type upon creation, and cannot be changed. Activity types should be presented to the user in an ordered manner, using the `ActivityType.order_nr` value.

- **`pipedrive-pp-cli activity-types add`** - Adds a new activity type.
- **`pipedrive-pp-cli activity-types delete`** - Marks an activity type as deleted.
- **`pipedrive-pp-cli activity-types get`** - Returns all activity types.
- **`pipedrive-pp-cli activity-types update`** - Updates an activity type.

### billing

Billing is responsible for handling your subscriptions, payments, plans and add-ons.

- **`pipedrive-pp-cli billing`** - Returns the add-ons for a single company.

### call-logs

Call logs describe the outcome of a phone call managed by an integrated provider. Since these logs are also considered activities, they can be associated with a deal or a lead, a person and/or an organization. Call logs do differ from other activities, as they only receive the information needed to describe the phone call.

- **`pipedrive-pp-cli call-logs add`** - Adds a new call log.
- **`pipedrive-pp-cli call-logs delete`** - Deletes a call log. If there is an audio recording attached to it, it will also be deleted. The related activity will not be removed by this request. If you want to remove the related activities, please use the endpoint which is specific for activities.
- **`pipedrive-pp-cli call-logs get`** - Returns details of a specific call log.
- **`pipedrive-pp-cli call-logs get-user`** - Returns all call logs assigned to a particular user.

### channels

Channels API allows you to integrate your existing messaging channels into Pipedrive through [Messaging app extension](https://pipedrive.readme.io/docs/messaging-app-extension). It enables you to manage and interact with the channel’s conversations, participants and messages inside Pipedrive Messaging inbox: get the historical conversation, receive and send new messages. These endpoints are accessible only through **Messengers integration** OAuth scope together with Messaging manifest in building the [Messaging app extension](https://pipedrive.readme.io/docs/messaging-app-extension).

- **`pipedrive-pp-cli channels add`** - Adds a new messaging channel, only admins are able to register new channels. It will use the getConversations endpoint to fetch conversations, participants and messages afterward. To use the endpoint, you need to have **Messengers integration** OAuth scope enabled and the Messaging manifest ready for the [Messaging app extension](https://pipedrive.readme.io/docs/messaging-app-extension).
- **`pipedrive-pp-cli channels delete`** - Deletes an existing messenger’s channel and all related entities (conversations and messages). To use the endpoint, you need to have **Messengers integration** OAuth scope enabled and the Messaging manifest ready for the [Messaging app extension](https://pipedrive.readme.io/docs/messaging-app-extension).
- **`pipedrive-pp-cli channels receive-message`** - Adds a message to a conversation. To use the endpoint, you need to have **Messengers integration** OAuth scope enabled and the Messaging manifest ready for the [Messaging app extension](https://pipedrive.readme.io/docs/messaging-app-extension).

### currencies

Supported currencies which can be used to represent the monetary value of a deal, or a value of any monetary type custom field. The `Currency.code` field must be used to point to a currency. `Currency.code` is the ISO-4217 format currency code for non-custom currencies. You can differentiate custom and non-custom currencies using the `is_custom_flag` property. For custom currencies, it is intended that the formatted sums are displayed in the UI using the following format: [sum][non-breaking space character][currency.symbol], for example: 500 users. Custom currencies cannot be added or removed via the API yet — rather the admin users of the account must configure them from the Pipedrive app.

- **`pipedrive-pp-cli currencies`** - Returns all supported currencies in given account which should be used when saving monetary values with other objects. The `code` parameter of the returning objects is the currency code according to ISO 4217 for all non-custom currencies.

### deal-fields

Deal fields represent the near-complete schema for a deal in the context of the company of the authorized user. Each company can have a different schema for their deals, with various custom fields. In the context of using deal fields as a schema for defining the data fields of a deal, it must be kept in mind that some types of custom fields can have additional data fields which are not separate deal fields per se. Such is the case with monetary, daterange and timerange fields – each of these fields will have one additional data field in addition to the one presented in the context of deal fields. For example, if there is a monetary field with the key `ffk9s9` stored on the account, `ffk9s9` would hold the numeric value of the field, and `ffk9s9_currency` would hold the ISO currency code that goes along with the numeric value. To find out which data fields are available, fetch one deal and list its keys.

- **`pipedrive-pp-cli deal-fields add`** - Adds a new deal field. For more information, see the tutorial for <a href="https://pipedrive.readme.io/docs/adding-a-new-custom-field" target="_blank" rel="noopener noreferrer">adding a new custom field</a>.
- **`pipedrive-pp-cli deal-fields delete`** - Marks multiple deal fields as deleted.
- **`pipedrive-pp-cli deal-fields delete-dealfields`** - Marks a field as deleted. For more information, see the tutorial for <a href="https://pipedrive.readme.io/docs/deleting-a-custom-field" target="_blank" rel="noopener noreferrer">deleting a custom field</a>.
- **`pipedrive-pp-cli deal-fields get`** - Returns data about all deal fields.
- **`pipedrive-pp-cli deal-fields get-dealfields`** - Returns data about a specific deal field.
- **`pipedrive-pp-cli deal-fields update`** - Updates a deal field. For more information, see the tutorial for <a href=" https://pipedrive.readme.io/docs/updating-custom-field-value " target="_blank" rel="noopener noreferrer">updating custom fields' values</a>.

### deals

Deals represent ongoing, lost or won sales to an organization or to a person. Each deal has a monetary value and must be placed in a stage. Deals can be owned by a user, and followed by one or many users. Each deal consists of standard data fields but can also contain a number of custom fields. The custom fields can be recognized by long hashes as keys. These hashes can be mapped against `DealField.key`. The corresponding label for each such custom field can be obtained from `DealField.name`.

- **`pipedrive-pp-cli deals get-archived`** - Returns all archived deals.
- **`pipedrive-pp-cli deals get-archived-summary`** - Returns a summary of all archived deals.
- **`pipedrive-pp-cli deals get-archived-timeline`** - Returns archived open and won deals, grouped by a defined interval of time set in a date-type dealField (`field_key`) — e.g. when month is the chosen interval, and 3 months are asked starting from January 1st, 2012, deals are returned grouped into 3 groups — January, February and March — based on the value of the given `field_key`.
- **`pipedrive-pp-cli deals get-summary`** - Returns a summary of all not archived deals.
- **`pipedrive-pp-cli deals get-timeline`** - Returns not archived open and won deals, grouped by a defined interval of time set in a date-type dealField (`field_key`) — e.g. when month is the chosen interval, and 3 months are asked starting from January 1st, 2012, deals are returned grouped into 3 groups — January, February and March — based on the value of the given `field_key`.

### files

Files are documents of any kind (images, spreadsheets, text files, etc.) that are uploaded to Pipedrive, and usually associated with a particular deal, person, organization, product, note or activity. Remote files can only be associated with a particular deal, person or organization. Note that the API currently does not support downloading files although it lets you retrieve a file’s meta-info along with a URL which can be used to download the file by using a standard HTTP GET request.

- **`pipedrive-pp-cli files add`** - Lets you upload a file and associate it with a deal, person, organization, activity, product or lead. For more information, see the tutorial for <a href="https://pipedrive.readme.io/docs/adding-a-file" target="_blank" rel="noopener noreferrer">adding a file</a>.
- **`pipedrive-pp-cli files add-and-link-it`** - Creates a new empty file in the remote location (`googledrive`) that will be linked to the item you supply. For more information, see the tutorial for <a href="https://pipedrive.readme.io/docs/adding-a-remote-file" target="_blank" rel="noopener noreferrer">adding a remote file</a>.
- **`pipedrive-pp-cli files delete`** - Marks a file as deleted. After 30 days, the file will be permanently deleted.
- **`pipedrive-pp-cli files get`** - Returns data about all files.
- **`pipedrive-pp-cli files get-id`** - Returns data about a specific file.
- **`pipedrive-pp-cli files link-to-item`** - Links an existing remote file (`googledrive`) to the item you supply. For more information, see the tutorial for <a href="https://pipedrive.readme.io/docs/adding-a-remote-file" target="_blank" rel="noopener noreferrer">adding a remote file</a>.
- **`pipedrive-pp-cli files update`** - Updates the properties of a file.

### filters

Each filter is essentially a set of data validation conditions. A filter of the same kind can be applied when fetching a list of deals, leads, persons, organizations or products in the context of a pipeline. Filters are limited to a maximum of 16 conditions. When applied, only items matching the conditions of the filter are returned. Detailed definitions of filter conditions and additional functionality is not yet available.

- **`pipedrive-pp-cli filters add`** - Adds a new filter, returns the ID upon success. Note that in the conditions JSON object only one first-level condition group is supported, and it must be glued with 'AND', and only two second level condition groups are supported of which one must be glued with 'AND' and the second with 'OR'. Other combinations do not work (yet) but the syntax supports introducing them in future. For more information, see the tutorial for <a href="https://pipedrive.readme.io/docs/adding-a-filter" target="_blank" rel="noopener noreferrer">adding a filter</a>.
- **`pipedrive-pp-cli filters delete`** - Marks multiple filters as deleted.
- **`pipedrive-pp-cli filters delete-id`** - Marks a filter as deleted.
- **`pipedrive-pp-cli filters get`** - Returns data about all filters.
- **`pipedrive-pp-cli filters get-helpers`** - Returns all supported filter helpers. It helps to know what conditions and helpers are available when you want to <a href="/docs/api/v1/Filters#addFilter">add</a> or <a href="/docs/api/v1/Filters#updateFilter">update</a> filters. For more information, see the tutorial for <a href="https://pipedrive.readme.io/docs/adding-a-filter" target="_blank" rel="noopener noreferrer">adding a filter</a>.
- **`pipedrive-pp-cli filters get-id`** - Returns data about a specific filter. Note that this also returns the condition lines of the filter.
- **`pipedrive-pp-cli filters update`** - Updates an existing filter.

### goals

Goals help your team meet your sales targets. There are three types of goals - company, team and user.

- **`pipedrive-pp-cli goals add`** - Adds a new goal. Along with adding a new goal, a report is created to track the progress of your goal.
- **`pipedrive-pp-cli goals delete`** - Marks a goal as deleted.
- **`pipedrive-pp-cli goals get`** - Returns data about goals based on criteria. For searching, append `{searchField}={searchValue}` to the URL, where `searchField` can be any one of the lowest-level fields in dot-notation (e.g. `type.params.pipeline_id`; `title`). `searchValue` should be the value you are looking for on that field. Additionally, `is_active=<true|false>` can be provided to search for only active/inactive goals. When providing `period.start`, `period.end` must also be provided and vice versa.
- **`pipedrive-pp-cli goals update`** - Updates an existing goal.

### lead-fields

Lead fields represent the near-complete schema for a lead in the context of the company of the authorized user. Each company can have a different schema for their leads, with various custom fields. In the context of using lead fields as a schema for defining the data fields of a lead, it must be kept in mind that some types of custom fields can have additional data fields which are not separate lead fields per se. Such is the case with monetary, daterange and timerange fields – each of these fields will have one additional data field in addition to the one presented in the context of lead fields. For example, if there is a monetary field with the key `ffk9s9` stored on the account, `ffk9s9` would hold the numeric value of the field, and `ffk9s9_currency` would hold the ISO currency code that goes along with the numeric value. To find out which data fields are available, fetch one lead and list its keys.

- **`pipedrive-pp-cli lead-fields`** - Returns data about all lead fields.

### lead-labels

Lead labels allow you to visually categorize your leads. There are three default lead labels: hot, cold, and warm, but you can add as many new custom labels as you want.

- **`pipedrive-pp-cli lead-labels add`** - Creates a lead label.
- **`pipedrive-pp-cli lead-labels delete`** - Deletes a specific lead label.
- **`pipedrive-pp-cli lead-labels get`** - Returns details of all lead labels. This endpoint does not support pagination and all labels are always returned.
- **`pipedrive-pp-cli lead-labels update`** - Updates one or more properties of a lead label. Only properties included in the request will be updated.

### lead-sources

A lead source indicates where your lead came from. Currently, these are the possible lead sources: `Manually created`, `Deal`, `Web forms`, `Prospector`, `Leadbooster`, `Live chat`, `Import`, `Website visitors`, `Workflow automation`, and `API`. Lead sources are pre-defined and cannot be edited. Please note that leads sourced from the Chatbot feature are assigned the value `Leadbooster`. Please also note that this list is not final and new sources may be added as needed.

- **`pipedrive-pp-cli lead-sources`** - Returns all lead sources. Please note that the list of lead sources is fixed, it cannot be modified. All leads created through the Pipedrive API will have a lead source `API` assigned.

### leads

Leads are potential deals stored in Leads Inbox before they are archived or converted to a deal. Each lead needs to be named (using the `title` field) and be linked to a person or an organization. In addition to that, a lead can contain most of the fields a deal can (such as `value` or `expected_close_date`).

- **`pipedrive-pp-cli leads add`** - Creates a lead. A lead always has to be linked to a person or an organization or both. All leads created through the Pipedrive API will have a lead source and origin set to `API`. Here's the tutorial for <a href="https://pipedrive.readme.io/docs/adding-a-lead" target="_blank" rel="noopener noreferrer">adding a lead</a>. If a lead contains custom fields, the fields' values will be included in the response in the same format as with the `Deals` endpoints. If a custom field's value hasn't been set for the lead, it won't appear in the response. Please note that leads do not have a separate set of custom fields, instead they inherit the custom fields' structure from deals. See an example given in the <a href="https://pipedrive.readme.io/docs/updating-custom-field-value" target="_blank" rel="noopener noreferrer">updating custom fields' values tutorial</a>.
- **`pipedrive-pp-cli leads delete`** - Deletes a specific lead.
- **`pipedrive-pp-cli leads get`** - Returns multiple not archived leads. Leads are sorted by the time they were created, from oldest to newest. Pagination can be controlled using `limit` and `start` query parameters. If a lead contains custom fields, the fields' values will be included in the response in the same format as with the `Deals` endpoints. If a custom field's value hasn't been set for the lead, it won't appear in the response. Please note that leads do not have a separate set of custom fields, instead they inherit the custom fields' structure from deals.
- **`pipedrive-pp-cli leads get-archived`** - Returns multiple archived leads. Leads are sorted by the time they were created, from oldest to newest. Pagination can be controlled using `limit` and `start` query parameters. If a lead contains custom fields, the fields' values will be included in the response in the same format as with the `Deals` endpoints. If a custom field's value hasn't been set for the lead, it won't appear in the response. Please note that leads do not have a separate set of custom fields, instead they inherit the custom fields' structure from deals.
- **`pipedrive-pp-cli leads get-id`** - Returns details of a specific lead. If a lead contains custom fields, the fields' values will be included in the response in the same format as with the `Deals` endpoints. If a custom field's value hasn't been set for the lead, it won't appear in the response. Please note that leads do not have a separate set of custom fields, instead they inherit the custom fields’ structure from deals.
- **`pipedrive-pp-cli leads search`** - Searches all leads by title, notes and/or custom fields. This endpoint is a wrapper of <a href="https://developers.pipedrive.com/docs/api/v1/ItemSearch#searchItem">/v1/itemSearch</a> with a narrower OAuth scope. Found leads can be filtered by the person ID and the organization ID.
- **`pipedrive-pp-cli leads update`** - Updates one or more properties of a lead. Only properties included in the request will be updated. Send `null` to unset a property (applicable for example for `value`, `person_id` or `organization_id`). If a lead contains custom fields, the fields' values will be included in the response in the same format as with the `Deals` endpoints. If a custom field's value hasn't been set for the lead, it won't appear in the response. Please note that leads do not have a separate set of custom fields, instead they inherit the custom fields’ structure from deals. See an example given in the <a href="https://pipedrive.readme.io/docs/updating-custom-field-value" target="_blank" rel="noopener noreferrer">updating custom fields’ values tutorial</a>.

### legacy-teams

Legacy teams allow you to form groups of users withing the organization for more efficient management. Previously Legacy Teams were called Teams and occupied the `v1/teams*` path. They're being deprecated because we are preparing for an upgraded version of the Teams API, which requires migrating the current functionality to a new path URL `v1/legacyTeams*`. The functionality and [OAuth scopes](https://pipedrive.readme.io/docs/marketplace-scopes-and-permissions-explanations) of all the Teams API endpoints will remain the same.

- **`pipedrive-pp-cli legacy-teams add-team`** - Adds a new team to the company and returns the created object.
- **`pipedrive-pp-cli legacy-teams get-team`** - Returns data about a specific team.
- **`pipedrive-pp-cli legacy-teams get-teams`** - Returns data about teams within the company.
- **`pipedrive-pp-cli legacy-teams get-user-teams`** - Returns data about all teams which have the specified user as a member.
- **`pipedrive-pp-cli legacy-teams update-team`** - Updates an existing team and returns the updated object.

### mailbox

Mailbox was designed to be the email control hub inside Pipedrive. Pipedrive supports all major providers (including Gmail, Outlook and also custom IMAP/SMTP). There are 2 options for syncing user emails: 2-way sync: Mail Connection is established with the mail provider (example Gmail). There can be only 1 active Mail Connection per user in company. 1-way sync: SmartBCC feature which stores the copies of email messages to Pipedrive by adding the SmartBCC specific address to mail recipients.

- **`pipedrive-pp-cli mailbox delete-mail-thread`** - Marks a mail thread as deleted.
- **`pipedrive-pp-cli mailbox get-mail-message`** - Returns data about a specific mail message.
- **`pipedrive-pp-cli mailbox get-mail-thread`** - Returns a specific mail thread.
- **`pipedrive-pp-cli mailbox get-mail-thread-messages`** - Returns all the mail messages inside a specified mail thread.
- **`pipedrive-pp-cli mailbox get-mail-threads`** - Returns mail threads in a specified folder ordered by the most recent message within.
- **`pipedrive-pp-cli mailbox update-mail-thread-details`** - Updates the properties of a mail thread.

### meetings

Meetings API allows integrating video calling apps into Pipedrive through [Video Calling App extension](https://pipedrive.readme.io/docs/video-calling-app-extension). It enables you to manage and interact with your video calls and meetings inside Pipedrive. These endpoints are accessible only through apps with video calls integration [OAuth scope](https://pipedrive.readme.io/docs/marketplace-scopes-and-permissions-explanations).

- **`pipedrive-pp-cli meetings delete-user-provider-link`** - A video calling provider must call this endpoint to remove the link between a user and the installed video calling app.
- **`pipedrive-pp-cli meetings save-user-provider-link`** - A video calling provider must call this endpoint after a user has installed the video calling app so that the new user's information is sent.

### note-fields

Note fields represent different fields that a note has.

- **`pipedrive-pp-cli note-fields`** - Returns data about all note fields.

### notes

Notes are pieces of textual (HTML-formatted) information that can be attached to deals, persons and organizations. Notes are usually displayed in the UI in chronological order – newest first – and in context with other updates regarding the item they are attached to. The maximum note size is approximately 100,000 characters (or 100KB per note).

- **`pipedrive-pp-cli notes add`** - Adds a new note.
- **`pipedrive-pp-cli notes delete`** - Deletes a specific note.
- **`pipedrive-pp-cli notes get`** - Returns all notes.
- **`pipedrive-pp-cli notes get-id`** - Returns details about a specific note.
- **`pipedrive-pp-cli notes update`** - Updates a note.

### oauth

Using OAuth 2.0 is necessary for developing apps that are available in the Pipedrive Marketplace. Authorization via OAuth 2.0 is a well-known and stable way to get fine-grained access to an API. To retrieve OAuth2 tokens you should send requests to the `https://oauth.pipedrive.com` domain. After registering the app, you must add the necessary server-side logic to your app to establish the OAuth flow. Please read more about authorization step on the [Pipedrive Developers page](https://pipedrive.readme.io/docs/marketplace-oauth-authorization).

- **`pipedrive-pp-cli oauth authorize`** - Authorize a user by redirecting them to the Pipedrive OAuth authorization page and request their permissions to act on their behalf. This step is necessary to implement only when you allow app installation outside of the Marketplace.
- **`pipedrive-pp-cli oauth get-tokens`** - After the customer has confirmed the app installation, you will need to exchange the `authorization_code` to a pair of access and refresh tokens. Using an access token, you can access the user's data through the API.
- **`pipedrive-pp-cli oauth refresh-tokens`** - The `access_token` has a lifetime. After a period of time, which was returned to you in `expires_in` JSON property, the `access_token` will be invalid, and you can no longer use it to get data from our API. To refresh the `access_token`, you must use the `refresh_token`.

### organization-fields

Organization fields represent the near-complete schema for an organization in the context of the company of the authorized user. Each company can have a different schema for their organizations, with various custom fields. In the context of using organization fields as a schema for defining the data fields of an organization, it must be kept in mind that some types of custom fields can have additional data fields which are not separate organization fields per se. Such is the case with monetary, daterange and timerange fields – each of these fields will have one additional data field in addition to the one presented in the context of organization fields. For example, if there is a monetary field with the key `ffk9s9` stored on the account, `ffk9s9` would hold the numeric value of the field, and `ffk9s9_currency` would hold the ISO currency code that goes along with the numeric value. To find out which data fields are available, fetch one organization and list its keys.

- **`pipedrive-pp-cli organization-fields add`** - Adds a new organization field. For more information, see the tutorial for <a href="https://pipedrive.readme.io/docs/adding-a-new-custom-field" target="_blank" rel="noopener noreferrer">adding a new custom field</a>.
- **`pipedrive-pp-cli organization-fields delete`** - Delete multiple organization fields in bulk
- **`pipedrive-pp-cli organization-fields delete-organizationfields`** - Marks a field as deleted. For more information, see the tutorial for <a href="https://pipedrive.readme.io/docs/deleting-a-custom-field" target="_blank" rel="noopener noreferrer">deleting a custom field</a>.
- **`pipedrive-pp-cli organization-fields get`** - Returns data about all organization fields.
- **`pipedrive-pp-cli organization-fields get-organizationfields`** - Returns data about a specific organization field.
- **`pipedrive-pp-cli organization-fields update`** - Updates an organization field. For more information, see the tutorial for <a href=" https://pipedrive.readme.io/docs/updating-custom-field-value " target="_blank" rel="noopener noreferrer">updating custom fields' values</a>.

### organization-relationships

Organization relationships represent how different organizations are related to each other. The relationship can be hierarchical (parent-child companies) or lateral as defined by the `type` field - either `parent` or `related`.

- **`pipedrive-pp-cli organization-relationships add`** - Creates and returns an organization relationship.
- **`pipedrive-pp-cli organization-relationships delete`** - Deletes an organization relationship and returns the deleted ID.
- **`pipedrive-pp-cli organization-relationships get`** - Gets all of the relationships for a supplied organization ID.
- **`pipedrive-pp-cli organization-relationships get-organizationrelationships`** - Finds and returns an organization relationship from its ID.
- **`pipedrive-pp-cli organization-relationships update`** - Updates and returns an organization relationship.

### organizations

Organizations are companies and other kinds of organizations you are making deals with. Persons can be associated with organizations so that each organization can contain one or more persons.


### permission-sets

Permission sets define what users in the account can do: which actions they are allowed to perform and which features they can access. Permission sets are app-specific, where apps are large parts of functionality, e.g., sales app, which allows accessing sales data, global permissions, which oversee cross-product features (for example contacts, insights, products) or account settings, which provides access to billing, user management, company settings and security center. Some permission sets with types such as admin and regular are pre-created for the account, while other custom ones can be created by users (depending on the tier the account is on).

- **`pipedrive-pp-cli permission-sets get`** - Returns data about all permission sets.
- **`pipedrive-pp-cli permission-sets get-permissionsets`** - Returns data about a specific permission set.

### person-fields

Person fields represent the near-complete schema for a person in the context of the company of the authorized user. Each company can have a different schema for their persons, with various custom fields. In the context of using person fields as a schema for defining the data fields of a person, it must be kept in mind that some types of custom fields can have additional data fields which are not separate person fields per se. Such is the case with monetary, daterange and timerange fields – each of these fields will have one additional data field in addition to the one presented in the context of person fields. For example, if there is a monetary field with the key `ffk9s9` stored on the account, `ffk9s9` would hold the numeric value of the field, and `ffk9s9_currency` would hold the ISO currency code that goes along with the numeric value. To find out which data fields are available, fetch one person and list its keys.

- **`pipedrive-pp-cli person-fields add`** - Adds a new person field. For more information, see the tutorial for <a href="https://pipedrive.readme.io/docs/adding-a-new-custom-field" target="_blank" rel="noopener noreferrer">adding a new custom field</a>.
- **`pipedrive-pp-cli person-fields delete`** - Delete multiple person fields in bulk
- **`pipedrive-pp-cli person-fields delete-personfields`** - Marks a field as deleted. For more information, see the tutorial for <a href="https://pipedrive.readme.io/docs/deleting-a-custom-field" target="_blank" rel="noopener noreferrer">deleting a custom field</a>.
- **`pipedrive-pp-cli person-fields get`** - Returns data about all person fields.<br>If a company uses the [Campaigns product](https://pipedrive.readme.io/docs/campaigns-in-pipedrive-api), then this endpoint will also return the `data.marketing_status` field.
- **`pipedrive-pp-cli person-fields get-personfields`** - Returns data about a specific person field.
- **`pipedrive-pp-cli person-fields update`** - Updates a person field. For more information, see the tutorial for <a href=" https://pipedrive.readme.io/docs/updating-custom-field-value " target="_blank" rel="noopener noreferrer">updating custom fields' values</a>.

### persons

Persons are your contacts, the customers you are doing deals with. Each person can belong to an organization. Persons should not be confused with users.


### pipelines

Pipelines are essentially ordered collections of stages.


### product-fields

Product fields represent the near-complete schema for a product in the context of the company of the authorized user. Each company can have a different schema for their products, with various custom fields. In the context of using product fields as a schema for defining the data fields of a product, it must be kept in mind that some types of custom fields can have additional data fields which are not separate product fields per se. Such is the case with monetary, daterange and timerange fields – each of these fields will have one additional data field in addition to the one presented in the context of product fields. For example, if there is a monetary field with the key `ffk9s9` stored on the account, `ffk9s9` would hold the numeric value of the field, and `ffk9s9_currency` would hold the ISO currency code that goes along with the numeric value. To find out which data fields are available, fetch one product and list its keys.

- **`pipedrive-pp-cli product-fields add`** - Adds a new product field. For more information, see the tutorial for <a href="https://pipedrive.readme.io/docs/adding-a-new-custom-field" target="_blank" rel="noopener noreferrer">adding a new custom field</a>.
- **`pipedrive-pp-cli product-fields delete`** - Delete multiple product fields in bulk
- **`pipedrive-pp-cli product-fields delete-productfields`** - Marks a product field as deleted. For more information, see the tutorial for <a href="https://pipedrive.readme.io/docs/deleting-a-custom-field" target="_blank" rel="noopener noreferrer">deleting a custom field</a>.
- **`pipedrive-pp-cli product-fields get`** - Returns data about all product fields.
- **`pipedrive-pp-cli product-fields get-productfields`** - Returns data about a specific product field.
- **`pipedrive-pp-cli product-fields update`** - Updates a product field. For more information, see the tutorial for <a href=" https://pipedrive.readme.io/docs/updating-custom-field-value " target="_blank" rel="noopener noreferrer">updating custom fields' values</a>.

### products

Products are the goods or services you are dealing with. Each product can have N different price points - firstly, each product can have a price in N different currencies, and secondly, each product can have N variations of itself, each having N prices in different currencies. Note that only one price per variation per currency is supported. Products can be instantiated to deals. In the context of instatiation, a custom price, quantity, duration and discount can be applied.


### project-templates

Project templates allow you to have reusable and dynamic structure to simplify creation of a project. Project template can contain information about activities, tasks and groups that will be used when creating a project.

- **`pipedrive-pp-cli project-templates get`** - Returns all not deleted project templates. This is a cursor-paginated endpoint. For more information, please refer to our documentation on <a href="https://pipedrive.readme.io/docs/core-api-concepts-pagination" target="_blank" rel="noopener noreferrer">pagination</a>.
- **`pipedrive-pp-cli project-templates get-projecttemplates`** - Returns the details of a specific project template.

### projects

Projects represent ongoing, completed or canceled projects attached to an organization, person or to deals. Each project has an owner and must be placed in a phase. Each project consists of standard data fields but can also contain a number of custom fields. The custom fields can be recognized by long hashes as keys.

- **`pipedrive-pp-cli projects add`** - Adds a new project. Note that you can supply additional custom fields along with the request that are not described here. These custom fields are different for each Pipedrive account and can be recognized by long hashes as keys.
- **`pipedrive-pp-cli projects delete`** - Marks a project as deleted.
- **`pipedrive-pp-cli projects get`** - Returns all projects. This is a cursor-paginated endpoint. For more information, please refer to our documentation on <a href="https://pipedrive.readme.io/docs/core-api-concepts-pagination" target="_blank" rel="noopener noreferrer">pagination</a>.
- **`pipedrive-pp-cli projects get-board`** - Returns the details of a specific project board.
- **`pipedrive-pp-cli projects get-boards`** - Returns all projects boards that are not deleted.
- **`pipedrive-pp-cli projects get-id`** - Returns the details of a specific project. Also note that custom fields appear as long hashes in the resulting data. These hashes can be mapped against the `key` value of project fields.
- **`pipedrive-pp-cli projects get-phase`** - Returns the details of a specific project phase.
- **`pipedrive-pp-cli projects get-phases`** - Returns all active project phases under a specific board.
- **`pipedrive-pp-cli projects update`** - Updates a project.

### recents

Recent changes across all item types in Pipedrive (deals, persons, etc).

- **`pipedrive-pp-cli recents`** - Returns data about all recent changes occurred after the given timestamp.

### roles

Roles are a part of the Visibility groups’ feature that allow the admin user to categorize other users and dictate what items they will be allowed access to see.

- **`pipedrive-pp-cli roles add`** - Adds a new role.
- **`pipedrive-pp-cli roles delete`** - Marks a role as deleted.
- **`pipedrive-pp-cli roles get`** - Returns all the roles within the company.
- **`pipedrive-pp-cli roles get-id`** - Returns the details of a specific role.
- **`pipedrive-pp-cli roles update`** - Updates the parent role and/or the name of a specific role.

### stages

Stage is a logical component of a pipeline, and essentially a bucket that can hold a number of deals. In the context of the pipeline a stage belongs to, it has an order number which defines the order of stages in that pipeline.


### tasks

Tasks represent actions that need to be completed and must be associated with a project. Tasks have an optional due date, can be assigned to a user and can have subtasks.

- **`pipedrive-pp-cli tasks add`** - Adds a new task.
- **`pipedrive-pp-cli tasks delete`** - Marks a task as deleted. If the task has subtasks then those will also be deleted.
- **`pipedrive-pp-cli tasks get`** - Returns all tasks. This is a cursor-paginated endpoint. For more information, please refer to our documentation on <a href="https://pipedrive.readme.io/docs/core-api-concepts-pagination" target="_blank" rel="noopener noreferrer">pagination</a>.
- **`pipedrive-pp-cli tasks get-id`** - Returns the details of a specific task.
- **`pipedrive-pp-cli tasks update`** - Updates a task.

### user-connections

Manage user connections.

- **`pipedrive-pp-cli user-connections`** - Returns data about all connections for the authorized user.

### user-settings

View user settings.

- **`pipedrive-pp-cli user-settings`** - Lists the settings of an authorized user. Example response contains a shortened list of settings.

### users

Users are people with access to your Pipedrive account. A user may belong to one or many Pipedrive accounts, so deleting a user from one Pipedrive account will not remove the user from the data store if he/she is connected to multiple accounts. Users should not be confused with persons.

- **`pipedrive-pp-cli users add`** - Adds a new user to the company, returns the ID upon success.
- **`pipedrive-pp-cli users find-by-name`** - Finds users by their name.
- **`pipedrive-pp-cli users get`** - Returns data about all users within the company.
- **`pipedrive-pp-cli users get-current`** - Returns data about an authorized user within the company with bound company data: company ID, company name, and domain. Note that the `locale` property means 'Date/number format' in the Pipedrive account settings, not the chosen language.
- **`pipedrive-pp-cli users get-id`** - Returns data about a specific user within the company.
- **`pipedrive-pp-cli users update`** - Updates the properties of a user. Currently, only `active_flag` can be updated.

### webhooks

See <a href="https://pipedrive.readme.io/docs/guide-for-webhooks-v2?ref=api_reference" target="_blank" rel="noopener noreferrer">the guide for Webhooks</a> for more information.

- **`pipedrive-pp-cli webhooks add`** - Creates a new Webhook and returns its details. Note that specifying an event which triggers the Webhook combines 2 parameters - `event_action` and `event_object`. E.g., use `*.*` for getting notifications about all events, `create.deal` for any newly added deals, `delete.persons` for any deleted persons, etc. See <a href="https://pipedrive.readme.io/docs/guide-for-webhooks-v2?ref=api_reference" target="_blank" rel="noopener noreferrer">the guide for Webhooks</a> for more details.
- **`pipedrive-pp-cli webhooks delete`** - Deletes the specified Webhook.
- **`pipedrive-pp-cli webhooks get`** - Returns data about all the Webhooks of a company.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pipedrive-pp-cli activity-fields

# JSON for scripting and agents
pipedrive-pp-cli activity-fields --json

# Filter to specific fields
pipedrive-pp-cli activity-fields --json --select id,name,status

# Dry run — show the request without sending
pipedrive-pp-cli activity-fields --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pipedrive-pp-cli activity-fields --agent
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
pipedrive-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/github.com/mvanhorn/printing-press-library/library/sales-and-crm/pipedrive/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PIPEDRIVE_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `pipedrive-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `pipedrive-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PIPEDRIVE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
