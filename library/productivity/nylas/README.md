# Nylas CLI

**Every Nylas API, plus a local SQLite mirror, cross-grant search, and confirm-by-hash sending no other Nylas tool has.**

The official nylas-cli is stateless and single-grant. nylas-pp-cli adds a local store of messages, events, threads, and contacts across every connected grant, FTS5 search, a SQL escape hatch, response-time and contact-gravity analytics, persisted webhook replay, and confirm-by-hash sending. Every Nylas v3 endpoint is exposed as a typed command; the local-store commands answer questions the live API cannot.

Learn more at [Nylas](https://www.nylas.com/).

Created by [@natekettles](https://github.com/natekettles) (Nathan Kettles).

## Install

The recommended path installs both the `nylas-pp-cli` binary and the `pp-nylas` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install nylas
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install nylas --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install nylas --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install nylas --agent claude-code
npx -y @mvanhorn/printing-press-library install nylas --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/nylas/cmd/nylas-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nylas-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-nylas --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-nylas --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install nylas --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nylas-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `NYLAS_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "nylas": {
      "command": "nylas-pp-mcp",
      "env": {
        "NYLAS_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Nylas V3 uses a bearer API key (Authorization: Bearer <NYLAS_API_KEY>) for application-level access to every grant. Set NYLAS_API_KEY in your environment or run `nylas-pp-cli auth set-token`. For per-user OAuth flows, NYLAS_ACCESS_TOKEN is also supported. The grant_id for each connected mailbox is a path parameter on every per-grant endpoint, not an auth header.

## Quick Start

```bash
# store the Nylas API key locally (or set NYLAS_API_KEY)
nylas-pp-cli auth set-token <TOKEN>

# verify auth + reachability before any sync
nylas-pp-cli doctor --agent

# list every connected mailbox under your application
nylas-pp-cli grants get-all --agent

# build the local mirror for the last day across every grant
nylas-pp-cli sync --resources messages,events --since 24h

# now answer 'what changed' instantly without an API call
nylas-pp-cli since 2h --resource messages --agent --select id,subject,from,grant_id

# FTS5 search across every grant in one shot
nylas-pp-cli search "invoice overdue" --type messages --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`sync`** — Pull messages, events, threads, and contacts for one or all grants into a local SQLite mirror with per-(grant, resource) cursors so re-runs are incremental.

  _Reach for this when an agent needs to answer questions repeatedly without burning rate-limit on every call._

  ```bash
  nylas-pp-cli sync --resources messages,events --since 24h --agent
  ```
- **`since`** — Query the local mirror for resources changed in the last N (e.g. 2h, 24h) across all grants in one shot.

  _Cheapest path to 'what changed' polling without API round-trips._

  ```bash
  nylas-pp-cli since 2h --resource messages --agent --select id,subject,from,grant_id
  ```
- **`search`** — FTS5 search over messages, threads, events, and contacts spanning every connected grant in a single query (use --type to filter by resource).

  _Use this to find references anywhere across a tenant without N separate searches._

  ```bash
  nylas-pp-cli search "invoice overdue" --type messages --limit 50 --agent
  ```
- **`export`** — Stream the local mirror (or a filtered slice) to NDJSON for downstream analytics tools like DuckDB or notebooks.

  _Pipe Nylas data into duckdb or a notebook in one command._

  ```bash
  nylas-pp-cli export --resource messages --since 90d --format ndjson
  ```

### Cross-grant analytics
- **`gravity`** — Rank contacts by cross-grant interaction weight (sent + received + meeting-attended), unified by email address.

  _Surfaces 'who actually matters to this tenant' in one call for CRM hygiene or agent prioritisation._

  ```bash
  nylas-pp-cli gravity --top 25 --since 90d --agent
  ```
- **`response-time`** — Compute median and p90 first-response latency on threads where the grant-holder replied, sliced by grant or counterparty domain.

  _An SLA dashboard in one command, impossible against the live API in under five minutes of round-trips._

  ```bash
  nylas-pp-cli response-time --group-by domain --since 30d --agent
  ```

### Reliability & safety
- **`webhook-replay`** — Re-fire any past webhook delivery from the local store into a local handler URL, optionally filtered by trigger or grant.

  _Reproduce a production webhook bug in seconds without waiting for the next event to fire._

  ```bash
  nylas-pp-cli webhook-replay --since 24h --trigger message.created --to http://localhost:3000/hook
  ```
- **`grants messages send`** — Use --dry-run to print the exact wire payload for any grants messages send call; pair with --yes (or --agent) to bypass interactive confirmation only when you've reviewed the preview.

  _Every destructive action is reviewable; an agent can show the payload before sending._

  ```bash
  nylas-pp-cli grants messages send 550e8400-e29b-41d4-a716-446655440000 --to PII_EMAIL_EXAMPLE --body 'note' --dry-run
nylas-pp-cli grants messages send 550e8400-e29b-41d4-a716-446655440000 --to PII_EMAIL_EXAMPLE --body 'note' --yes
  ```
- **`grants messages send`** — Pass the global --idempotent flag so an already-existing create result is treated as a successful no-op, making the send safe to retry inside an agent loop.

  _Safe to call inside an agent retry loop._

  ```bash
  nylas-pp-cli grants messages send 550e8400-e29b-41d4-a716-446655440000 --to PII_EMAIL_EXAMPLE --body 'note' --idempotent --agent
  ```
- **`grants doctor`** — Health-check every grant: token expiry, missing scopes for advertised features, recent webhook failures, sync lag.

  _Tells you which mailboxes are quietly broken before users notice._

  ```bash
  nylas-pp-cli grants doctor --agent
  ```

### Agent-native plumbing
- **`sql`** — Read-only SQL query against the local mirror with --json output.

  _Anything we forgot to expose as a verb, an LLM can still answer through SQL._

  ```bash
  nylas-pp-cli sql "SELECT from_addr, COUNT(*) FROM messages GROUP BY 1 ORDER BY 2 DESC LIMIT 20" --agent
  ```
- **`--agent (global flag)`** — Single flag that forces --json --compact --no-input --no-color --yes defaults so any read command is LLM-safe in one switch.

  _One flag flips the entire CLI into LLM-safe mode; eliminates a class of prompt-eats-stdin bugs._

  ```bash
  nylas-pp-cli grants get-all --agent --limit 10
  ```

## Usage

Run `nylas-pp-cli --help` for the full command reference and flag list.

## Commands

### admin

Manage admin

- **`nylas-pp-cli admin create-api-key`** - <div id="admonition-warning">⚠️ <b>Before you can use the Manage API Keys endpoints, you need to <a href="/docs/reference/api/manage-api-keys/">create a Nylas Service Account</a></b>.</div>

Creates an API key for the specified Nylas application.
- **`nylas-pp-cli admin create-domain`** - <div id="admonition-warning">⚠️ <b>Before you can use the Manage Domains endpoints, you need a <a href="/docs/v3/auth/nylas-service-account/">Nylas Service Account</a></b>.</div>

Registers a new email domain for your organization. After creating a domain, you must
[verify its DNS records](/docs/v3/email/domains/) before you can use it with Transactional Send or Nylas Inbound.
- **`nylas-pp-cli admin delete-api-key`** - <div id="admonition-warning">⚠️ <b>Before you can use the Manage API Keys endpoints, you need to <a href="/docs/api/v3/admin/#tag--Manage-API-keys--nylas-service-account">create a Nylas Service Account</a></b>.</div>

Deletes the specified API key.
- **`nylas-pp-cli admin delete-domain`** - <div id="admonition-warning">⚠️ <b>Before you can use the Manage Domains endpoints, you need a <a href="/docs/v3/auth/nylas-service-account/">Nylas Service Account</a></b>.</div>

Deletes the specified domain. This action is irreversible.
- **`nylas-pp-cli admin get-api-key`** - <div id="admonition-warning">⚠️ <b>Before you can use the Manage API Keys endpoints, you need to <a href="/docs/api/v3/admin/#tag--Manage-API-keys--nylas-service-account">create a Nylas Service Account</a></b>.</div>

Returns the specified API key.
- **`nylas-pp-cli admin get-api-keys`** - <div id="admonition-warning">⚠️ <b>Before you can use the Manage API Keys endpoints, you need to <a href="/docs/api/v3/admin/#tag--Manage-API-keys--nylas-service-account">create a Nylas Service Account</a></b>.</div>

Returns a list of API keys associated with the specified Nylas application.
- **`nylas-pp-cli admin get-domain`** - <div id="admonition-warning">⚠️ <b>Before you can use the Manage Domains endpoints, you need a <a href="/docs/v3/auth/nylas-service-account/">Nylas Service Account</a></b>.</div>

Returns the specified domain.
- **`nylas-pp-cli admin get-domain-info`** - <div id="admonition-warning">⚠️ <b>Before you can use the Manage Domains endpoints, you need a <a href="/docs/v3/auth/nylas-service-account/">Nylas Service Account</a></b>.</div>

Returns the DNS record information and verification status for the specified verification type.
Use this endpoint to retrieve the DNS records you need to add at your DNS provider before
calling the [Verify domain](/docs/reference/api/manage-domains/verify-domain/) endpoint.
- **`nylas-pp-cli admin list-domains`** - <div id="admonition-warning">⚠️ <b>Before you can use the Manage Domains endpoints, you need a <a href="/docs/v3/auth/nylas-service-account/">Nylas Service Account</a></b>.</div>

Returns a list of all domains registered to your organization.
- **`nylas-pp-cli admin update-domain`** - <div id="admonition-warning">⚠️ <b>Before you can use the Manage Domains endpoints, you need a <a href="/docs/v3/auth/nylas-service-account/">Nylas Service Account</a></b>.</div>

Updates the specified domain. Currently, only the `name` field can be updated.
- **`nylas-pp-cli admin verify-domain`** - <div id="admonition-warning">⚠️ <b>Before you can use the Manage Domains endpoints, you need a <a href="/docs/v3/auth/nylas-service-account/">Nylas Service Account</a></b>.</div>

Triggers a verification check for the specified DNS record type. Before calling this endpoint,
add the required DNS records to your domain's DNS configuration. You can get the required records
by calling the [Get domain info](/docs/reference/api/manage-domains/get-domain-info/) endpoint.

### applications

In the context of the Nylas APIs, an "application" is the object record of your Nylas application.

<div id="admonition-info">🔍 <b>The term "application" can refer to any of three concepts</b>: your Nylas application, the project you're building ("your application" or "your app"), and applications that you use to connect to service providers ("provider auth applications"). We try to be specific in this documentation to avoid confusion. The API endpoints described here are for working with your Nylas application, specifically.</div>

The Nylas application is the central resource for your Nylas implementation. It collects the [connectors](/docs/reference/api/connectors-integrations/) that you use to store information about third party services that your application connects to, and stores the [grants](/docs/reference/api/manage-grants/) that you create when using connectors.

Nylas applications also allow you to define your specific branding, change the look and feel of the Nylas Hosted authentication flow, and list your application's callback URIs.

## Application callback URIs

Your Nylas application includes a list of allowed callback URIs. These are known URIs that Nylas can direct users to after authentication. You need to define at least _one_ callback URI so your users can complete the auth flow.

You must include any callback URIs you plan to use in this list. If an auth payload includes a callback URI that isn't on the list, the whole authentication flow fails.

## Application limitations

<div id="admonition-warning">⚠️ <b>You can create, edit, and delete applications from the Nylas Dashboard</b>. You <i>cannot</i> create, edit, or delete them using the Nylas APIs.</div>

Keep the following limitations in mind as you work with Nylas applications:

- Applications are the central resource that stores other Nylas resources. You _must_ create an application before you can create any other parts of your Nylas implementation.
- Applications can be associated with only one project at a time. While your project can have more than one Nylas application to provide different authentication experiences, you cannot share applications, connectors, or grants between more than one project.
- Applications cannot be nested, and cannot be set up with parent-child relationships.
- Your application must have _at least one_ callback URI, or else it cannot finish the authentication flow, which means it cannot create grants. Nylas requires grants to access user data.
  - In an ideal scenario, your application will have multiple callback URIs defined.

- **`nylas-pp-cli applications add-callback-uri`** - Adds a callback URI to the specified Nylas application. Nylas uses callback URIs to redirect users
to your project after they complete the authentication flow. If you don't specify a `platform`,
Nylas defaults to `web`.

The OAuth protocol requires that you include the `client_secret` field for calls made to this
endpoint, depending on the `platform` you define. If you're using
[OAuth 2.0 with PKCE](/docs/v3/auth/hosted-oauth-accesstoken/#create-grants-with-oauth-2.0-and-pkce)
and your platform is `js`, `ios`, `android`, or `desktop`, the `client_secret` field is not
required.
- **`nylas-pp-cli applications delete-callback-uri`** - Deletes the specified callback URI.
- **`nylas-pp-cli applications get`** - Gets the application object
- **`nylas-pp-cli applications get-all-callback-uris`** - Returns a list of callback URIs for the specified Nylas application.
- **`nylas-pp-cli applications get-callback-uri`** - Returns the specified callback URI.
- **`nylas-pp-cli applications update`** - Updates a Nylas application using the client ID associated with the specified API key.

<div id="admonition-warning">⚠️ <b>This endpoint will be removed in the future when application settings are available in the Nylas Dashboard</b>.</div>

When you make a `PATCH` request, Nylas replaces all data in the nested object with the information
included in your request. For more information, see
[Updating objects](/docs/reference/api/#updating-objects).
- **`nylas-pp-cli applications update-callback-uri`** - Updates the specified callback URI.

If you don't define the `platform`, Nylas doesn't modify the existing settings.

When you make a `PATCH` request, Nylas replaces all data in the nested object with the information
included in your request. For more information, see
[Updating objects](/docs/reference/api/#updating-objects).

### calendars

The Nylas Calendar API allows you to create and manage calendars, and access the events they contain. Nylas uses the same commands to manage calendars across providers, and you can refer to specific calendars using the provider's `calendar_id`.

Depending on the provider, a calendar might be accessed by only one person, or shared among several users. Some common calendars might include Personal, Work, or Shared calendars.

A calendar might contain overlapping events or conflicting schedules. You can use the [Get Availability endpoint](/docs/reference/api/calendar/post-availability/) to find the best time for an event by identifying time periods that have no conflicting events among all participants.

## Find a calendar ID

You must specify a `calendar_id` in all calls you make to the Nylas Calendar API. You can use `primary` to specify the primary calendar associated with a grant, or you can look up the ID of the calendar that you want to work with and use that.

For virtual calendars, the `primary` calendar is the first created for a virtual account. You _cannot_ delete a virtual calendar that's designated as `primary`.

For iCloud, there is no `primary` calendar.

## Free/Busy information

The [Get Free/Busy Schedule endpoint](/docs/reference/api/calendar/post-calendars-free-busy/) is available for all providers, including Virtual Calendars, except for iCloud.

The [Get Availability endpoint](/docs/reference/api/calendar/post-availability/) doesn't support the `free_busy` object.

## Virtual calendars

Nylas allows you to create virtual calendars for users and resources that might not have calendars on your providers (for example, external contractors or meeting rooms). You can use the Nylas Calendar and Events APIs with the virtual accounts that power virtual calendars, just like you would any other account. Virtual accounts don't provide email or contacts features, so you can't use them with the Email or Contacts APIs.

For more information, see [Using virtual calendars](/docs/v3/calendar/virtual-calendars/).

## Metadata on calendars

You can add metadata to new and existing calendars by including the `metadata` sub-object in your `POST`, `PUT`, or `PATCH` request. For more information, see the [Metadata documentation](/docs/dev-guide/metadata/).

## Calendar scopes

The table below lists the Calendar endpoints and which scopes they require. The table shortens the full scope URI for space reasons, so add the prefix for the provider when requesting scopes.

The ☑️ in each column indicates the most restrictive scope you can request for each provider and still use that API. More permissive scopes appear under the minimum option. If you're already using one of the permissive scopes, you don't need to add the more restrictive scope.

| Endpoint                                                                                                                               | Google Scopes</br>`https://www.googleapis.com/auth/...` | Microsoft Scopes</br>`https://graph.microsoft.com/...` |
| :------------------------------------------------------------------------------------------------------------------------------------- | :------------------------------------------------------ | :----------------------------------------------------- |
| **GET** `/calendars`</br>**GET** `/calendars/<CALENDAR_ID>`</br>**POST** `/calendars/free-busy`</br>**POST** `/calendars/availability` | `/calendar.readonly` ☑️</br>`/calendar`                 | `Calendars.Read` ☑️</br>`Calendars.ReadWrite`          |
| **POST** `/calendars`</br>**PUT** `/calendars/<CALENDAR_ID>`</br>**DELETE** `/calendars/<CALENDAR_ID>`                                 | `/calendar` ☑️                                          | `Calendars.ReadWrite` ☑️                               |

For more information about scopes, see [Using scopes to request user data](/docs/dev-guide/scopes/).

## Calendar activity notifications

You can subscribe to the following triggers so Nylas notifies you about changes to your users' calendar data:

- `calendar.created`
- `calendar.updated`
- `calendar.deleted`

For more information, see the [Calendar webhook notification schemas](/docs/reference/notifications/#calendar-notifications).

## Microsoft event considerations

Microsoft Outlook events are often shared across all calendars in a user's account. If the user creates an event on one of their calendars, you can retrieve it using another calendar ID from their grant.

- **`nylas-pp-cli calendars`** - Returns availability information for the specified user or group of users. All participants' email
addresses must be associated with valid Nylas grants, and should be unique within their application.

### channels

Manage channels

- **`nylas-pp-cli channels create-pubsub`** - Create a Pub/Sub channel in the specified application.
- **`nylas-pp-cli channels create-sns`** - Create an Amazon SNS notification channel in the specified application.

The `topic` must be a valid Amazon SNS topic ARN (starting with `arn:aws:sns:`).
- **`nylas-pp-cli channels delete-pubsub-by-id`** - Delete a specific Pub/Sub channel from a specific Nylas application.
- **`nylas-pp-cli channels delete-sns-by-id`** - Delete a specific Amazon SNS notification channel from a specific Nylas application.
- **`nylas-pp-cli channels get-pubsub`** - Get the Pub/Sub channels for an application.
- **`nylas-pp-cli channels get-pubsub-by-id`** - Get a specific Pub/Sub channel from a specific Nylas application.
- **`nylas-pp-cli channels get-sns`** - Get the Amazon SNS notification channels for an application.
- **`nylas-pp-cli channels get-sns-by-id`** - Get a specific Amazon SNS notification channel from a specific Nylas application.
- **`nylas-pp-cli channels put-pubsub-by-id`** - Updates the specified Pub/Sub channel.

When you make a `PUT` request, Nylas replaces all data in the nested object with the information
included in your request. For more information, see
[Updating objects](/docs/reference/api/#updating-objects).
- **`nylas-pp-cli channels put-sns-by-id`** - Updates the specified Amazon SNS notification channel.

When you make a `PUT` request, Nylas replaces all data in the nested object with the information
included in your request. For more information, see
[Updating objects](/docs/reference/api/#updating-objects).

### connect

Manage connect

- **`nylas-pp-cli connect byo-auth`** - Manually creates a grant using the Bring Your Own (BYO) Authentication flow. If you're handling the
OAuth flow in your own project or you want to migrate existing users, BYO Auth lets you provide
the user's `refresh_token` to create a grant.

If a user previously authenticated with your Nylas application using the same email address, Nylas
detects this and re-authenticates their existing grant instead of creating a new one. The API
response contains the user's existing `grant_id`.

### Supported providers

Pick the request body variant that matches your provider:

- **Refresh token** — OAuth providers (`google`, `microsoft`, `yahoo`, `zoom`) using a standard refresh token.
- **Credential override** — OAuth providers, but using a stored [credential record](/docs/reference/api/connector-credentials/) to swap in different client credentials.
- **Microsoft bulk auth** — Microsoft App Permissions. Requires a [connector credential](/docs/reference/api/connector-credentials/create_credential/) and the [admin consent flow](/docs/v3/auth/bulk-auth-grants/#make-a-microsoft-admin-consent-flow-request-using-nylas-apis).
- **Google bulk auth** — Google Service Accounts. Requires a [connector credential](/docs/reference/api/connector-credentials/create_credential/) and the [Service Account flow](/docs/v3/auth/bulk-auth-grants/#google-app-permission-via-nylas).
- **IMAP** — direct IMAP/SMTP credentials for any IMAP provider.
- **iCloud** — an iCloud email address plus an [Apple app password](https://support.apple.com/en-us/HT204397).
- **EWS** — on-premises Microsoft Exchange; hosted Exchange should use Microsoft Graph instead.
- **Virtual calendar** — a [Virtual Calendar](/docs/v3/calendar/virtual-calendars/) grant for scheduling without a third-party provider.
- **Zoom Meetings** — Zoom OAuth. Your OAuth app must include the [granular scopes](https://developers.zoom.us/docs/integrations/oauth-scopes-granular/) `meeting:write:meeting`, `meeting:update:meeting`, and `meeting:delete:meeting`.
- **Nylas (Agent Account)** — a fully Nylas-hosted [Agent Account](/docs/v3/agent-accounts/) email and calendar mailbox on a domain you've registered with Nylas.
- **`nylas-pp-cli connect exchange-oauth2-token`** - The standard OAuth token endpoint for Hosted Authentication. This endpoint doesn't require authentication, as it is part of the auth process.
You can pass one of the following `grant_type` values:
- `authorization_code`: Exchange the `code` Nylas returns from the OAuth 2.0 authorization flow for tokens (`access_token` and `refresh_token`). - `refresh_token`: Use the existing `refresh_token` for an existing grant to issue a new `access_token`. You _must_ pass your API key in the `client_secret` field.
 - `client_credentials`: Issue a new short-lived (1 hour) `access_token` using an existing `grant_id`. You _must_ pass your API key in the `client_secret` field. This is mainly used in Scheduler implementations.

This endpoint accepts both `application/json` and `application/x-www-form-urlencoded` request body types. The body parameters are the same for both, with the same naming conventions.
For more information, see the [Hosted authentication with access token documentation](/docs/v3/auth/hosted-oauth-accesstoken/).
### Failed token exchange requests
Each OAuth `code` is a unique, one-time-use credential. If your token exchange fails, you must restart the OAuth process. If you try to pass the original `code` in another token exchange request, the provider rejects the `code` and Nylas returns an error.
- **`nylas-pp-cli connect get-oauth2-flow`** - The initial OAuth 2.0 authorization request. Use this endpoint with the required query parameters to start the OAuth 2.0 process. The query parameters pass details to the Nylas API about how the user should authenticate, and where they should go after authenticating.
This endpoint supports the authorization code flow and optional PKCE settings for client-side only applications. For more information, see the  [Hosted OAuth with access token](/docs/v3/auth/hosted-oauth-accesstoken/) and [Hosted OAuth with access token and PKCE](/docs/v3/auth/hosted-oauth-accesstoken/#create-grants-with-oauth-2.0-and-pkce) documentation.
- **`nylas-pp-cli connect info-oauth2-token`** - Get info about a specific token based on the identifier you include. Use _either_ the ID Token or Access Token.</br></br>**Note**: Because Nylas uses the schema outlined in [RFC 9068](https://datatracker.ietf.org/doc/html/rfc9068#name-requesting-a-jwt-access-tok) to ensure that it is compatible with all OAuth libraries in all languages, the format for this endpoint is different from the other OAuth endpoints.
- **`nylas-pp-cli connect revoke-oauth2-token-and-grant`** - Revokes the specified OAuth access token. When you revoke the token, Nylas _doesn't_ revoke the
grant or the associated provider token. This means that a user can re-authenticate to get a new
access token for the existing grant, so their `grant_id` doesn't change.

If you revoke a Nylas access token, Nylas also revokes all child tokens and the parent
`refresh_token` attached to the access token.

### connectors

Manage connectors

- **`nylas-pp-cli connectors create`** - Create a connector in your Nylas application.

Connectors are how your Nylas application stores information it needs to connect to external
services. Creating a connector is the first step in setting up authentication for your project.
See [Supported providers](/docs/provider-guides/#supported-providers) for more information.
- **`nylas-pp-cli connectors delete-by-provider`** - Delete the existing connector for the provider you specify.
- **`nylas-pp-cli connectors get-all`** - List the connectors in your Nylas application.
- **`nylas-pp-cli connectors get-by-provider`** - Returns a connector for the specified provider.
- **`nylas-pp-cli connectors update-by-provider`** - Update the connector for the specified provider.

When you make a `PATCH` request, Nylas replaces all data in the nested object with the information
included in your request. For more information, see
[Updating objects](/docs/reference/api/#updating-objects).

### domains

Manage domains

### grants

Manage grants

- **`nylas-pp-cli grants delete-by-id`** - Delete an existing grant by ID. You cannot re-authenticate the deleted grant. If you try to re-authenticate it, Nylas creates a new grant instead.

**Before deleting a grant, consider whether re-authentication is the better option.** Deleting a grant is permanent: object IDs may change (especially for IMAP providers), sync state resets, and tracking links break. See [Handling expired grants](https://developer.nylas.com/docs/dev-guide/best-practices/grant-lifecycle/) for details.
- **`nylas-pp-cli grants get-all`** - Returns all grants in your Nylas application.
- **`nylas-pp-cli grants get-by-access-token`** - Gets a grant using current access token
- **`nylas-pp-cli grants get-by-id`** - Gets a grant with the provided ID.

If the grant's `grant_status` is `invalid`, the grant has expired and needs to be re-authenticated. See [Handling expired grants](https://developer.nylas.com/docs/dev-guide/best-practices/grant-lifecycle/) for best practices on detection and recovery.
- **`nylas-pp-cli grants patch-by-id`** - Updates the specified grant's stored settings or scope metadata.

**Common use cases:**

- **Rotate a refresh token** — If you obtain a new `refresh_token` from a provider (for example, after a user re-consents in your own OAuth flow), you can update the grant's stored token without deleting and recreating the grant. Pass the new token in `settings.refresh_token`.
- **Update stored scope list** — Update the `scope` array to reflect the scopes the grant currently holds. Note: this only updates the scope metadata stored by Nylas. It does **not** change the actual permissions the provider has granted. To change provider permissions, the user must re-authenticate through the provider's OAuth consent flow.

When you make a `PATCH` request, Nylas replaces all data in the nested object with the information
included in your request. For more information, see
[Updating objects](/docs/reference/api/#updating-objects).

### lists

The Lists endpoints let you manage typed collections of values (email addresses, domains, or top-level domains) that can be referenced by Rules using the `in_list` condition operator. Lists provide a way to maintain dynamic allow lists and block lists that are evaluated during inbound rule processing and outbound send evaluation without needing to update individual rules.

Each list has a `type` that determines what kind of values it accepts and which rule condition fields it can be matched against:

- **`domain`** — Domain names (for example, `example.com`). Matched against the `from.domain` or `recipient.domain` rule condition.
- **`tld`** — Top-level domains (for example, `com`, `xyz`). Matched against the `from.tld` or `recipient.tld` rule condition.
- **`address`** — Full email addresses (for example, `PII_EMAIL_EXAMPLE`). Matched against the `from.address` or `recipient.address` rule condition.

List `type` is set at creation time and cannot be changed. Items within a list are managed via the `/v3/lists/{list_id}/items` sub-resource endpoints. Values are automatically normalized (lowercased and trimmed) and validated against the list's `type`. Duplicate additions are silently ignored.

- **`nylas-pp-cli lists create`** - Creates a list for your application. Lists are typed collections of values (domains, TLDs, or email addresses)
that can be referenced by rules using the `in_list` condition operator.

The list's `type` is set at creation and cannot be changed.
- **`nylas-pp-cli lists delete`** - Deletes the specified list. This action is irreversible and cascades to all items in the list. Rules that
reference the list through an `in_list` condition no longer match its values after deletion.
- **`nylas-pp-cli lists get`** - Returns the specified list.
- **`nylas-pp-cli lists list-lists`** - Returns all lists for your application.
- **`nylas-pp-cli lists update`** - Updates the specified list. Only `name` and `description` can be updated. The list `type` is immutable after
creation.

### migration-tools

Manage migration tools

- **`nylas-pp-cli migration-tools migration-clone-single-account`** - Migrates a single v2 connected account to a v3 grant. Use this endpoint to migrate a few accounts
to v3 as a test, then use the
[Batch Clone endpoint](/docs/reference/api/app-migration/migration_snapshot_batch_clone/)
to migrate the rest.

Before you use this endpoint, your v2 Nylas applications need to be linked to their corresponding v3
applications and you need to have a working equivalent
[authentication connector](/docs/reference/api/connectors-integrations/) for each provider
you use. If you need help with this process, [learn how to get support](/docs/support/).

During migration, Nylas maps the v2 provider to its v3 equivalent using the connected account's
v2 authorization type and provider. All sensitive data (such as tokens and passwords) is encrypted
and securely transfered only within the Nylas infrastructure. No secrets exit the internal Nylas
network.

This endpoint is rate limited to 20 requests per second, per Nylas application ID.

### Create placeholder grants

Nylas can migrate existing Microsoft Graph, Office 365, and EWS accounts that use token
authentication. It can't fully migrate
[some types of Microsoft accounts](/docs/v2/upgrade-to-v3/upgrade/migrating-microsoft-accounts/#microsoft-account-import-compatibility-in-v3)
because of provider limitations and scope changes in v3.

By default, Nylas creates an invalid v3 Grant with fake credentials for accounts that it can't
automatically migrate. These grants act as "placeholders" that the user can manually authenticate
to later. You can turn this feature off for Microsoft accounts only by setting the `clone_exchange`
query parameter to `false`, or for all providers by setting `allow_invalid_grant_creation` to
`false`.
- **`nylas-pp-cli migration-tools migration-get-jobs`** - Get information about the migration jobs for your application, including progress and status, for currently running and finished jobs.

Migration Jobs are a background task. After you start one Nylas returns a success response, and the jobs run in the background until they finish. These jobs do not block other API calls, but also do not send a notification when complete. This API allows you to get general information about a them, so you can see their status. 

There are two types of migration job: "snapshot" and "migration". A Snapshot job takes a snapshot of the application's v2 account data, and prepares it for migration. A Migration job actually turns the v2 account data into v3 grants.

Jobs have a status, which can be one of the following:
- A _pending_ job is queued and waiting to start.
- A _running_ job is currently in progress.
- A _partial_ job has finished, but was not able to migrate all accounts successfully.
- A _completed_ job has finished and successfully migrated _all_ accounts.
- A _failed_ job has finished but did not migrate any accounts.

You can optionally filter the list of jobs to return only jobs with a specific status or type.

You can also use the `sort_by_completion` argument to return the list of jobs with pending and running jobs first, followed by failed, completed, and partial jobs. Finished jobs are ordered by their completion timestamp (`CompletedAt`) from the most recent to the oldest.

If you don't include any query parameters to filter the results, Nylas returns all jobs.

The API is rate limited to 20 requests per second per Nylas application ID.
- **`nylas-pp-cli migration-tools migration-import-v2-app`** - Import the settings from a v2 Nylas application to its linked v3 application. Before you use this
endpoint, make sure you
[link a v2 application to a v3 application](/docs/reference/api/app-migration/migration_link_v2v3_apps/).

This endpoint imports the following v2 settings, and sets them to the corresponding v3 application
settings:

  - Your website URL.
  - Your icon URL (used to customize the Hosted OAuth page).
  - Redirect URIs for OAuth.
  - Integrations and their settings.
    - Nylas migrates Google integrations to equivalent v3 connectors.
    - If you have v2 Microsoft integrations, you should create a new Azure auth app and update the
    connector settings. For detailed instructions, see
    [Migrating Microsoft accounts](/docs/v2/upgrade-to-v3/upgrade/migrating-microsoft-accounts/).
    - Nylas migrates IMAP, iCloud, and EWS integrations to v3 connectors with auto-generated default
    values.

Nylas automatically detects your application ID from the API key and finds its linked v2
application to begin the import process.

This endpoint is rate limited to 20 requests per second, per Nylas application ID.
- **`nylas-pp-cli migration-tools migration-link-v2v3-apps`** - Link an existing v2 application to an existing v3 application.

<div id="admonition-warning">⚠️<strong>Your v2 and v3 applications must be in the same data center region to use these tools. </strong> You can only use these tools to migrate a v2 application to a v3 application in the same region. You cannot use these to move a v2 application to a different region.</div>

This is the first step of the migration process, and is how you tell Nylas which v2 application you are migrating.

To use this API you need the v2 source application ID and secret, in the format you use to authorize v2 API calls, and the v3 API key from the v3 destination application.

To verify that you own the v2 source application you're linking, add the `BasicV2` API header. This extra header is required. The `BasicV2` API header contains a Base64-encoded `V2_APP_ID:V2_APP_SECRET`, which is also used for Basic auth for any v2 API call.

```bash
--header 'BasicV2: <base64-encoded V2_APP_ID:V2_APP_SECRET> // Use the -n flag when you Base64 encode
```

No further request body is required. Nylas detects the v3 application ID from the API key, and the v2 application ID from the BasicV2 header.

The API is rate limited to 20 requests per second per Nylas application ID.
- **`nylas-pp-cli migration-tools migration-snapshot-batch-clone`** - Starts a batch migration job to clone v2 connected accounts to v3 grants.

Before you use this endpoint, your v2 Nylas applications need to be linked to their corresponding v3
applications and you need to have a working equivalent 
[authentication connector](/docs/reference/api/connectors-integrations/) for each provider
you use. If you need help with this process, [learn how to get support](/docs/support/).

When you make a batch clone request, Nylas starts two background jobs:

  - A snapshot job that prepares non-sensitive v2 account data for migration.
  - A batch clone job that migrates the v2 account data to v3 grants.

Nylas handles all data and logic for these jobs and runs them in the background to they don't block
API responses.

You can filter for certain types of accounts by specifying properties in the body of your request.
Use these options when you want to try migrating certain types of accounts in smaller batches,
or to retry failed migrations. If you don't specify any filters, Nylas uses the default values.

During migration, Nylas maps the v2 provider to its v3 equivalent using the connected account's
v2 authorization type and provider. All sensitive data (such as tokens and passwords) is encrypted
and securely transfered only within the Nylas infrastructure. No secrets exit the internal Nylas
network.

This endpoint is rate limited to 20 requests per second, per Nylas application ID.

### Create placeholder grants

Nylas can migrate existing Microsoft Graph, Office 365, and EWS accounts that use token
authentication. It can't fully migrate
[some types of Microsoft accounts](/docs/v2/upgrade-to-v3/upgrade/migrating-microsoft-accounts/#microsoft-account-import-compatibility-in-v3)
because of provider limitations and scope changes in v3.

By default, Nylas creates an invalid v3 Grant with fake credentials for accounts that it can't
automatically migrate. These grants act as "placeholders" that the user can manually authenticate
to later. You can turn this feature off for Microsoft accounts only by setting the `clone_exchange`
query parameter to `false`, or for all providers by setting `allow_invalid_grant_creation` to
`false`.
- **`nylas-pp-cli migration-tools translate-v2id-to-provider-id`** - Use the connected account ID and a resource type, with an optional list of specific Nylas IDs, to get a response that contains a list of of Nylas IDs and their v3 Provider ID equivalents. Use this API as a one-time operation to translate v2 IDs into v3 Provider IDs. Do not use this API in your code logic as it very data intensive.

To use this endpoint, your v2 Nylas application needs to be linked to the equivalent v3 Nylas application. This endpoint does not work for objects in v2 accounts that have the provider set to `Outlook`.

By default, the API returns up to 3000 records for the requested resource type related to the v2 connected account, sorted by `created_at` date. If you specify a list of v2 Nylas IDs, the API returns the v3 Provider IDs for those specific IDs only.

Results are paginated, with a page size of 3000 results. If the response includes a `next_page_number` field, you can use that number in a request to get the next set of results. 

Also, there is a possibility to search results created only after certain Unix timestamp, in Nylas v2 database. To use this, add to body payload `start_from_timestamp` valid Unix timestamp.

The API is rate limited to 20 requests per second per Nylas application ID.

### IMAP folder resource ID

When you make a Translate ID request for an IMAP folder (`resource_type: folders`), Nylas returns its name in the `v3_resource_id` field. To get the resource ID for a specific folder, Base64 encode the folder name using the following format: `v0:<NYLAS_GRANT_ID>:<FOLDER_NAME>`.

### notetakers

Nylas Notetaker is a real-time meeting bot that you can invite to your online meetings. It records and transcribes your discussion, and delivers results to you using the Nylas API and webhook notifications.

The [`/v3/grants/<NYLAS_GRANT_ID>/notetakers` endpoints](/docs/reference/api/notetaker/) let you interact with Nylas Notetaker while referencing a specific grant so you can use Notetaker's [calendar sync features](/docs/v3/notetaker/calendar-sync/) for your authenticated users.

The [`/v3/notetakers` endpoints](/docs/reference/api/standalone-notetaker/) let you send Notetakers that aren't connected to a grant as **standalone Notetakers**.

## Calendar sync

Notetaker works with the Nylas Calendar API to automatically sync with calendars and events. When you sync Notetaker with a grant's calendar or event, Nylas tracks their join times and meeting links so the Notetaker is always sent to the meeting on time.

## Webhook notifications

Nylas sends [webhook notifications](/docs/reference/notifications/#notetaker-notifications) when a Notetaker bot is created, updated, or deleted, and when the recording media is available.

- **`nylas-pp-cli notetakers delete-standalone`** - Permanently deletes the specified Notetaker and all associated data, including any recordings, transcripts, thumbnails, summaries, and action items. This works regardless of the Notetaker's current state — scheduled, active, or completed. This is a hard delete and cannot be undone. Once deleted, Nylas cannot recover the Notetaker or any of its data.
- **`nylas-pp-cli notetakers get-all-standalone`** - Returns a list of standalone Notetaker bots.
- **`nylas-pp-cli notetakers get-standalone`** - Returns the specified Notetaker bot and its details.
- **`nylas-pp-cli notetakers invite-standalone`** - Adds a standalone Notetaker bot to the specified meeting.

<div id="admonition-info">ℹ️ <b>Nylas doesn't de-duplicate Notetaker bots</b>. Every <code>POST /v3/notetakers</code> request you make invites a new Notetaker to the specified meeting.</div>
- **`nylas-pp-cli notetakers update-standalone`** - Updates the specified scheduled standalone Notetaker bot.

### policies

The Policies endpoints let you define the operational configuration for Nylas Agent Accounts, including message limits, attachment constraints, spam detection settings, and linked rules for inbound message filtering. Each policy is scoped to your application and can be assigned to one or more Agent Accounts.

Policies let you control:

- **Limits** — Attachment sizes, counts, allowed types, total storage, daily message quotas, and retention periods for the inbox and spam folders.
- **Spam detection** — DNS-based block list (DNSBL) checking, header anomaly detection, and sensitivity tuning.
- **Options** — Additional folder creation and CIDR-based email aliasing.
- **Rules** — Link filtering rules (created via the Rules API) to automatically process inbound messages based on sender criteria.

Policy limits are validated against your plan's maximum values. If a limit field is omitted, it defaults to the plan maximum. If a requested value exceeds the plan limit, the API returns an error.

- **`nylas-pp-cli policies create-policy`** - Creates a policy for your application. Policies define message limits, spam detection settings, options, and linked
rules for Nylas Agent Accounts. The `application_id` and `organization_id` are derived from your API key, so you
don't need to include them in the request body — they are read-only.
- **`nylas-pp-cli policies delete-policy`** - Deletes the specified policy. This action is irreversible.
- **`nylas-pp-cli policies get-policy`** - Returns the specified policy.
- **`nylas-pp-cli policies list`** - Returns a list of all policies for your application.
- **`nylas-pp-cli policies update-policy`** - Updates the specified policy. All fields are optional — only provided fields are updated. The same plan-limit,
spam sensitivity, and retention-period validation applies as on create.

### providers

Manage providers

- **`nylas-pp-cli providers`** - Returns the provider if one is detected. This operation is rate limited to 20 calls per minute for each Nylas application ID.

### rules

The Rules endpoints let you define automated filtering and routing logic for Nylas Agent Accounts. Each rule specifies a `trigger` (`inbound` or `outbound`), matching conditions, and actions to perform when those conditions are met.

**Inbound rules** run when mail arrives. Conditions match the sender — `from.address`, `from.domain`, or `from.tld`. Actions include blocking the message at the SMTP level, marking as spam, assigning to a folder, marking as read or starred, archiving, and trashing.

**Outbound rules** run before a send is submitted to the email provider. Conditions can match sender data (`from.address`, `from.domain`, `from.tld`), recipient data (`recipient.address`, `recipient.domain`, `recipient.tld`), or the send type via `outbound.type` (`compose` for new messages, `reply` for replies). A `block` action on an outbound rule rejects the send with HTTP 403 before it reaches the provider, so no message is delivered and no sent copy is stored. Non-blocking actions (`mark_as_spam`, `archive`, `mark_as_read`, `mark_as_starred`, `assign_to_folder`, `trash`) apply to the stored sent copy. `recipient.*` fields match against **any** recipient — including To, CC, BCC, and SMTP envelope recipients.

Inbound and outbound rules are isolated. Inbound rules never run during sends, and outbound rules never run on message receipt, so stored sent copies aren't re-evaluated against inbound rules.

Rules are evaluated in priority order (lower numbers first). Inbound rules are applied through the recipient grant's policy. Outbound rules are currently evaluated from the sending application's enabled outbound rules. The `block` action is terminal and cannot be combined with other actions.

Rules support the `in_list` condition operator, which checks field values against Lists (managed via the Lists API) for dynamic, maintainable allow and block lists. `in_list` works for `from.*` and `recipient.*` fields but not for `outbound.type`, which accepts only `is` and `is_not`.

Rule executions are audited per grant. List evaluation records with `GET /v3/grants/{grant_id}/rule-evaluations` to see which inbound or outbound rules ran, what normalized input was considered, and which actions were applied.

- **`nylas-pp-cli rules create`** - Creates a rule for your application. A rule defines a `trigger` (`inbound` or `outbound`), conditions against
sender or recipient fields, and actions to apply when the conditions match. Inbound rules run on incoming
messages; outbound rules run on sends before they're submitted to the email provider. Inbound and outbound
rules are isolated — inbound rules never run during sends, and outbound rules never run on message receipt.

Inbound rules can match `from.address`, `from.domain`, or `from.tld`. Outbound rules can match
`from.address`, `from.domain`, `from.tld`, `recipient.address`, `recipient.domain`, `recipient.tld`, or
`outbound.type` (`compose` or `reply`). Link inbound rules to a policy to apply them to specific Agent
Accounts. Outbound rules are currently evaluated from the sending application's enabled outbound rules.

The `application_id` and `organization_id` are derived from your API key and are read-only.
- **`nylas-pp-cli rules delete`** - Deletes the specified rule. This action is irreversible. Policies that reference the rule no longer apply it
during inbound processing, and outbound sends no longer evaluate it after deletion.
- **`nylas-pp-cli rules get`** - Returns the specified rule.
- **`nylas-pp-cli rules list`** - Returns a list of all rules for your application.
- **`nylas-pp-cli rules update`** - Updates the specified rule. All fields are optional — only provided fields are updated. The same validation rules
apply as on create.

### scheduling

Manage scheduling

- **`nylas-pp-cli scheduling delete-bookings-id`** - Deletes the specified booking. Nylas also cancels the associated event on the provider.

Nylas validates the provided session ID and uses it to retrieve the related
[Configuration object](/docs/reference/api/configurations/). If you created a public
Configuration, you don't need to include the `Authorization` request header with a session ID, but
you do need to pass the Configuration object ID as a query parameter.
- **`nylas-pp-cli scheduling delete-session`** - Deletes a specific session.
- **`nylas-pp-cli scheduling get-availability`** - Gets available time slots within the given time range, using the rules defined in the specified
Configuration object. If the Configuration `type` is `group`, Nylas returns only valid group events
within the time range, including recurring events.

Nylas validates the provided session ID and uses it to retrieve the related
[Configuration object](/docs/reference/api/configurations/). If you created a public
Configuration, you don't need to include the `Authorization` request header with a session ID, but
you do need to pass the Configuration object ID as a query parameter.
- **`nylas-pp-cli scheduling get-bookings-id`** - Returns the specified Booking object.

Nylas validates the provided session ID and uses it to retrieve the related
[Configuration object](/docs/reference/api/configurations/). If you created a public
Configuration, you don't need to include the `Authorization` request header with a session ID, but
you do need to pass the Configuration object ID as a query parameter.
- **`nylas-pp-cli scheduling import-group-events`** - Imports existing events to your group events Configuration. You can import up to 20 events per
request.
- **`nylas-pp-cli scheduling patch-bookings-id`** - Reschedules the specified booking. Nylas also updates the associated event on the provider.

Nylas recommends against changing participants' email addresses while updating a booking, to avoid
confusion.

Nylas validates the provided session ID and uses it to retrieve the related
[Configuration object](/docs/reference/api/configurations/). If you created a public
Configuration, you don't need to include the `Authorization` request header with a session ID, but
you do need to pass the Configuration object ID as a query parameter.

When you make a `PATCH` request, Nylas replaces all data in the nested object with the information
included in your request. For more information, see
[Updating objects](/docs/reference/api/#updating-objects).
- **`nylas-pp-cli scheduling post-bookings`** - Books an event with the participants listed in the session's
[Configuration object](/docs/reference/api/configurations/), using the details from the
Configuration. The `start_time` and `end_time` must correspond to a valid time slot returned by
the
[Scheduling Availability endpoint](/docs/reference/api/availability/get-availability/)
using the same Configuration.

Nylas validates the session ID and uses it to retrieve the related Configuration object. If you
created a public Configuration, you don't need to include the `Authorization` request header with
a session ID, but you do need to pass the Configuration object ID as a query parameter.
- **`nylas-pp-cli scheduling post-sessions`** - Creates a new short-lived session that you can pass to the Scheduling Component to enforce user
authentication. Your request must include the ID of an existing Configuration object.
- **`nylas-pp-cli scheduling put-bookings-id`** - Confirms or cancels the specified pending booking. Nylas also updates the associated event on the
provider.

Nylas validates the provided session ID and uses it to retrieve the related
[Configuration object](/docs/reference/api/configurations/). If you created a public
Configuration, you don't need to include the `Authorization` request header with a session ID, but
you do need to pass the Configuration object ID as a query parameter.

When you make a `PUT` request, Nylas replaces all data in the nested object with the information
included in your request. For more information, see
[Updating objects](/docs/reference/api/#updating-objects).
- **`nylas-pp-cli scheduling redirect`** - Redirects an existing v2 Scheduling Page to a v3 URL.
- **`nylas-pp-cli scheduling validate-time-slot`** - Validates whether the selected group event or time slot has changed, and is invalid. This can happen
because...

- The group event was deleted.
- The group event was updated (for example, the start time was changed).
- The `capacity` for the event was changed.

### templates

Manage templates

- **`nylas-pp-cli templates create-app-level`** - Creates an application-level template.
- **`nylas-pp-cli templates delete-app-level`** - Deletes the specified application-level template.
- **`nylas-pp-cli templates get-app-level`** - Returns the specified application-level template.
- **`nylas-pp-cli templates list-app-level`** - Returns a list of application-level templates.
- **`nylas-pp-cli templates render-html`** - Renders the HTML content of an application-level template using the provided variables and specified
templating engine.
- **`nylas-pp-cli templates update-app-level`** - Updates the specified application-level template.

### webhooks

Manage webhooks

- **`nylas-pp-cli webhooks delete-by-id`** - Delete a webhook destination record.
- **`nylas-pp-cli webhooks get-by-id`** - Get the webhook destinations for an application ID by webhook ID
- **`nylas-pp-cli webhooks get-destinations-application`** - Get a list of all webhook destinations for an application id.
- **`nylas-pp-cli webhooks get-mock-payload`** - Use this endpoint to see example notification payloads for the different Nylas events you specify,
to the webhook URL you specify.
- **`nylas-pp-cli webhooks post-destinations`** - Creates a webhook destination with the specified URL and list of trigger types.

### Webhook destinations and retry logic

You should limit the number of webhook destinations you have for each trigger type. When Nylas
retries a webhook, the retry goes to all the destinations for that trigger type. This can result
in _a lot_ of notifications.

Some webhook testing tools rate-limit or block you if your endpoint generates too much traffic.
Nylas blocks Ngrok connections for this reason.

### Webhook notification header

Every webhook notification Nylas sends includes the `x-nylas-signature` header. If you're using
the Nylas SDKs, you might see `X-Nylas-Signature` instead.
- **`nylas-pp-cli webhooks post-new-secret`** - Update the webhook secret value for a destination. The previous value will immediately stop being used and the new value will take over.

### Webhook notification header

Every webhook notification Nylas sends includes the `x-nylas-signature` header. Depending on the SDK you're using, you might see `X-Nylas-Signature` instead.
- **`nylas-pp-cli webhooks put-by-id`** - Update the values in a specific webhook destination.

### Limitations

- You only need to specify fields that need to change when you make a request to this endpoint.
Empty fields in the request do not overwrite existing fields.
- You should limit how many webhook destinations you have for each trigger type. When Nylas retries
a webhook, the retry goes to _all destinations for the specific trigger type_. This can result in
a lot of notifications.
- Some webhook testing tools rate-limit or block you if your webhook destination endpoint generates
too much traffic. Nylas blocks Ngrok connections for this reason.
- **`nylas-pp-cli webhooks send-test-event`** - Use this endpoint to check if your project's webhook destination is configured correctly. Nylas
sends a test webhook payload to the webhook URL you specify, and listens for a success
acknowledgement.

The secret used is `mock-webhook-secret`.

### workflows

Manage workflows

- **`nylas-pp-cli workflows create`** - Creates an application-level workflow.

<div id="admonition-info">ℹ️ <b>You must have an existing <a href="/docs/reference/api/application-level-templates/">template</a> to create a workflow</b>.</div>
- **`nylas-pp-cli workflows delete`** - Deletes the specified application-level workflow.
- **`nylas-pp-cli workflows get`** - Returns the specified application-level workflow.
- **`nylas-pp-cli workflows list`** - Returns all application-level workflows.
- **`nylas-pp-cli workflows update`** - Updates the specified application-level workflow.

### workspaces

Workspaces group and organize grants in a Nylas application by a common attribute, such as the email address domain (for example, `nylas.com`).

## Assign grants to workspaces

Nylas offers two endpoints to manage workspaces and the grants they contain:

- [**Automatically Group Grants into Workspaces**](/docs/reference/api/workspaces/autogroup-workspace/): Starts a background job that, based on your filters, processes existing grants in your Nylas application and automatically sorts them to existing workspaces. If necessary, Nylas automatically creates new workspaces.
- [**Update Workspace Assignments**](/docs/reference/api/workspaces/manually-assign-workspace/): Manually specify a workspace ID and up to 500 grants to add or remove from that workspace.

## Workspace limitations

- Workspaces are designed to group grants by the top-level domain (TLD) of users' email addresses. Nylas allows you to manually assign grants to any workspace, but that workspace must have `auto_group` set to `false`.
- When `auto_group` is set to `false`, Nylas doesn't automatically assign grants to that workspace. You'll need to manually assign grants to the workspace using the [Update Workspace Assignments endpoint](/docs/reference/api/workspaces/manually-assign-workspace/).
- When `auto_group` is `true`, Nylas automatically assigns new grants to the workspace. You can move grants to a different workspace by making an [Update Workspace Assignments request](/docs/reference/api/workspaces/manually-assign-workspace/).

- **`nylas-pp-cli workspaces autogroup`** - Configures automatic grouping settings for new or existing workspaces, depending on the filters
set. If you don't set any filters, Nylas considers all grants.
- **`nylas-pp-cli workspaces create`** - Creates a workspace.
- **`nylas-pp-cli workspaces delete`** - Deletes the specified workspace.
- **`nylas-pp-cli workspaces get`** - Returns the specified workspace.
- **`nylas-pp-cli workspaces get-all`** - Returns all workspaces in your Nylas application. The application queried is determined based on
the API key you use to authorize your request.
- **`nylas-pp-cli workspaces update`** - Updates the specified workspace.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
nylas-pp-cli applications get

# JSON for scripting and agents
nylas-pp-cli applications get --json

# Filter to specific fields
nylas-pp-cli applications get --json --select id,name,status

# Dry run — show the request without sending
nylas-pp-cli applications get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
nylas-pp-cli applications get --agent
```

## Cookbook

Recipes that combine multiple commands. Each one assumes `NYLAS_API_KEY` (or `NYLAS_ACCESS_TOKEN`) is set; run `nylas-pp-cli doctor` first if anything misbehaves.

**1. First-time setup: connect a grant, mirror a week, search across it**

```bash
# Verify auth + reachability first
nylas-pp-cli doctor --agent

# List the grants under this application
nylas-pp-cli grants get-all --agent --select id,email,provider

# Pull a week of messages, events, threads, and contacts for every grant
nylas-pp-cli sync --resources messages,events,threads,contacts --since 7d

# Find every thread mentioning a topic across every grant in one shot
nylas-pp-cli search "renewal" --type threads --limit 25 --agent
```

**2. Inbox triage for an agent: surface what's new since the last run**

```bash
# Everything synced in the last two hours, JSON-shaped for an LLM
nylas-pp-cli since 2h --resource messages,threads --agent --select id,subject,from,grants_id

# Drill into a single message you want to act on
nylas-pp-cli grants messages get-id <GRANT_ID> <MESSAGE_ID> --agent

# Preview a reply before it goes out — payload first, hash to confirm
nylas-pp-cli grants messages send <GRANT_ID> --to PII_EMAIL_EXAMPLE --subject "re: renewal" --body "Looping in legal" --dry-run

# Once the preview looks right, send with idempotent retry semantics
nylas-pp-cli grants messages send <GRANT_ID> --to PII_EMAIL_EXAMPLE --subject "re: renewal" --body "Looping in legal" --idempotent --agent --yes
```

**3. Cross-grant analytics: who matters, how fast we reply**

```bash
# Top 25 counterparties by sent + received + meeting-attended weight
nylas-pp-cli gravity --top 25 --since 90d --agent

# Median + p90 first-response latency, sliced by counterparty domain
nylas-pp-cli response-time --group-by domain --since 30d --agent

# Ad-hoc SQL when the dimension you need isn't a verb
nylas-pp-cli sql "SELECT json_extract(data,'\$.from[0].email') AS sender, COUNT(*) AS n FROM grants_messages WHERE synced_at >= datetime('now','-7 day') GROUP BY 1 ORDER BY 2 DESC LIMIT 20" --agent
```

**4. Reliability: health check, replay a webhook, export for downstream tools**

```bash
# Health-check auth, reachability, and local cache state
nylas-pp-cli doctor --agent --fail-on stale

# Re-fire a past delivery into a local handler to reproduce a bug
nylas-pp-cli webhook-replay --since 24h --trigger message.created --to http://localhost:3000/hook

# Snapshot the local mirror for downstream analysis (DuckDB, notebooks, BI)
nylas-pp-cli export --resource messages --since 90d --format ndjson > messages.ndjson
```

**5. Scheduler: inspect configurations and bookings, audit rules**

```bash
# List every scheduling configuration under a grant
nylas-pp-cli grants scheduling get-configurations <GRANT_ID> --agent

# Look up a single booking by ID
nylas-pp-cli scheduling get-bookings-id <BOOKING_ID> --agent

# Audit the rule set (safety review for outbound block rules)
nylas-pp-cli rules list --agent --select id,name,trigger,enabled
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
nylas-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/nylas-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `NYLAS_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `nylas-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $NYLAS_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 from any command** — Run `nylas-pp-cli doctor` to confirm NYLAS_API_KEY is set and the key resolves; rotate via the Nylas Dashboard if needed.
- **send unintentionally fires before review** — Run any `grants messages send` with `--dry-run` first to print the wire payload; re-run with `--yes` once verified.
- **since/search returns zero rows** — Run `nylas-pp-cli sync` first; the local store is empty until you sync at least once.
- **rate-limited (429) during sync** — sync respects Retry-After; for tighter pacing pass `--concurrency 1` and re-run.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**nylas-cli**](https://github.com/nylas/cli) — TypeScript
- [**nylas-nodejs**](https://github.com/nylas/nylas-nodejs) — TypeScript
- [**nylas-python**](https://github.com/nylas/nylas-python) — Python
- [**nylas-ruby**](https://github.com/nylas/nylas-ruby) — Ruby
- [**nylas-java**](https://github.com/nylas/nylas-java) — Java

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
