---
name: pp-nylas
description: "Every Nylas API, plus a local SQLite mirror, cross-grant search, and confirm-by-hash sending no other Nylas tool has. Trigger phrases: `search my Nylas inbox across all grants`, `what changed in Nylas in the last 2 hours`, `first-response time on my Nylas threads`, `replay a Nylas webhook locally`, `preview a Nylas send before it goes out`, `use nylas`, `run nylas-pp-cli`."
author: "Nathan Kettles"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - nylas-pp-cli
---

# Nylas — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `nylas-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install nylas --cli-only
   ```
2. Verify: `nylas-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/nylas/cmd/nylas-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for nylas-pp-cli when you need to query Nylas data across multiple grants, run repeat 'what changed since' polls cheaply, compose SQL over messages and events, or sample the wire payload of a send before it goes out. For pure single-call API parity with the official SDK, the official nylas-cli is also a fine choice.

## Unique Capabilities

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

## Command Reference

**admin** — Manage admin

- `nylas-pp-cli admin create-api-key` — <div id='admonition-warning'>⚠️ <b>Before you can use the Manage API Keys endpoints, you need to <a...
- `nylas-pp-cli admin create-domain` — <div id='admonition-warning'>⚠️ <b>Before you can use the Manage Domains endpoints, you need a <a...
- `nylas-pp-cli admin delete-api-key` — <div id='admonition-warning'>⚠️ <b>Before you can use the Manage API Keys endpoints, you need to <a...
- `nylas-pp-cli admin delete-domain` — <div id='admonition-warning'>⚠️ <b>Before you can use the Manage Domains endpoints, you need a <a...
- `nylas-pp-cli admin get-api-key` — <div id='admonition-warning'>⚠️ <b>Before you can use the Manage API Keys endpoints, you need to <a...
- `nylas-pp-cli admin get-api-keys` — <div id='admonition-warning'>⚠️ <b>Before you can use the Manage API Keys endpoints, you need to <a...
- `nylas-pp-cli admin get-domain` — <div id='admonition-warning'>⚠️ <b>Before you can use the Manage Domains endpoints, you need a <a...
- `nylas-pp-cli admin get-domain-info` — <div id='admonition-warning'>⚠️ <b>Before you can use the Manage Domains endpoints, you need a <a...
- `nylas-pp-cli admin list-domains` — <div id='admonition-warning'>⚠️ <b>Before you can use the Manage Domains endpoints, you need a <a...
- `nylas-pp-cli admin update-domain` — <div id='admonition-warning'>⚠️ <b>Before you can use the Manage Domains endpoints, you need a <a...
- `nylas-pp-cli admin verify-domain` — <div id='admonition-warning'>⚠️ <b>Before you can use the Manage Domains endpoints, you need a <a...

**applications** — In the context of the Nylas APIs, an "application" is the object record of your Nylas application.

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

- `nylas-pp-cli applications add-callback-uri` — Adds a callback URI to the specified Nylas application. Nylas uses callback URIs to redirect users to your project...
- `nylas-pp-cli applications delete-callback-uri` — Deletes the specified callback URI.
- `nylas-pp-cli applications get` — Gets the application object
- `nylas-pp-cli applications get-all-callback-uris` — Returns a list of callback URIs for the specified Nylas application.
- `nylas-pp-cli applications get-callback-uri` — Returns the specified callback URI.
- `nylas-pp-cli applications update` — Updates a Nylas application using the client ID associated with the specified API key. <div...
- `nylas-pp-cli applications update-callback-uri` — Updates the specified callback URI. If you don't define the `platform`, Nylas doesn't modify the existing settings....

**calendars** — The Nylas Calendar API allows you to create and manage calendars, and access the events they contain. Nylas uses the same commands to manage calendars across providers, and you can refer to specific calendars using the provider's `calendar_id`.

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

- `nylas-pp-cli calendars` — Returns availability information for the specified user or group of users. All participants' email addresses must be...

**channels** — Manage channels

- `nylas-pp-cli channels create-pubsub` — Create a Pub/Sub channel in the specified application.
- `nylas-pp-cli channels create-sns` — Create an Amazon SNS notification channel in the specified application. The `topic` must be a valid Amazon SNS topic...
- `nylas-pp-cli channels delete-pubsub-by-id` — Delete a specific Pub/Sub channel from a specific Nylas application.
- `nylas-pp-cli channels delete-sns-by-id` — Delete a specific Amazon SNS notification channel from a specific Nylas application.
- `nylas-pp-cli channels get-pubsub` — Get the Pub/Sub channels for an application.
- `nylas-pp-cli channels get-pubsub-by-id` — Get a specific Pub/Sub channel from a specific Nylas application.
- `nylas-pp-cli channels get-sns` — Get the Amazon SNS notification channels for an application.
- `nylas-pp-cli channels get-sns-by-id` — Get a specific Amazon SNS notification channel from a specific Nylas application.
- `nylas-pp-cli channels put-pubsub-by-id` — Updates the specified Pub/Sub channel. When you make a `PUT` request, Nylas replaces all data in the nested object...
- `nylas-pp-cli channels put-sns-by-id` — Updates the specified Amazon SNS notification channel. When you make a `PUT` request, Nylas replaces all data in the...

**connect** — Manage connect

- `nylas-pp-cli connect byo-auth` — Manually creates a grant using the Bring Your Own (BYO) Authentication flow. If you're handling the OAuth flow in...
- `nylas-pp-cli connect exchange-oauth2-token` — The standard OAuth token endpoint for Hosted Authentication. This endpoint doesn't require authentication, as it is...
- `nylas-pp-cli connect get-oauth2-flow` — The initial OAuth 2.0 authorization request. Use this endpoint with the required query parameters to start the OAuth...
- `nylas-pp-cli connect info-oauth2-token` — Get info about a specific token based on the identifier you include. Use _either_ the ID Token or Access...
- `nylas-pp-cli connect revoke-oauth2-token-and-grant` — Revokes the specified OAuth access token. When you revoke the token, Nylas _doesn't_ revoke the grant or the...

**connectors** — Manage connectors

- `nylas-pp-cli connectors create` — Create a connector in your Nylas application. Connectors are how your Nylas application stores information it needs...
- `nylas-pp-cli connectors delete-by-provider` — Delete the existing connector for the provider you specify.
- `nylas-pp-cli connectors get-all` — List the connectors in your Nylas application.
- `nylas-pp-cli connectors get-by-provider` — Returns a connector for the specified provider.
- `nylas-pp-cli connectors update-by-provider` — Update the connector for the specified provider. When you make a `PATCH` request, Nylas replaces all data in the...

**domains** — Manage domains


**grants** — Manage grants

- `nylas-pp-cli grants delete-by-id` — Delete an existing grant by ID. You cannot re-authenticate the deleted grant. If you try to re-authenticate it,...
- `nylas-pp-cli grants get-all` — Returns all grants in your Nylas application.
- `nylas-pp-cli grants get-by-access-token` — Gets a grant using current access token
- `nylas-pp-cli grants get-by-id` — Gets a grant with the provided ID. If the grant's `grant_status` is `invalid`, the grant has expired and needs to be...
- `nylas-pp-cli grants patch-by-id` — Updates the specified grant's stored settings or scope metadata. **Common use cases:** - **Rotate a refresh token**...

**lists** — The Lists endpoints let you manage typed collections of values (email addresses, domains, or top-level domains) that can be referenced by Rules using the `in_list` condition operator. Lists provide a way to maintain dynamic allow lists and block lists that are evaluated during inbound rule processing and outbound send evaluation without needing to update individual rules.

Each list has a `type` that determines what kind of values it accepts and which rule condition fields it can be matched against:

- **`domain`** — Domain names (for example, `example.com`). Matched against the `from.domain` or `recipient.domain` rule condition.
- **`tld`** — Top-level domains (for example, `com`, `xyz`). Matched against the `from.tld` or `recipient.tld` rule condition.
- **`address`** — Full email addresses (for example, `PII_EMAIL_EXAMPLE`). Matched against the `from.address` or `recipient.address` rule condition.

List `type` is set at creation time and cannot be changed. Items within a list are managed via the `/v3/lists/{list_id}/items` sub-resource endpoints. Values are automatically normalized (lowercased and trimmed) and validated against the list's `type`. Duplicate additions are silently ignored.

- `nylas-pp-cli lists create` — Creates a list for your application. Lists are typed collections of values (domains, TLDs, or email addresses) that...
- `nylas-pp-cli lists delete` — Deletes the specified list. This action is irreversible and cascades to all items in the list. Rules that reference...
- `nylas-pp-cli lists get` — Returns the specified list.
- `nylas-pp-cli lists list-lists` — Returns all lists for your application.
- `nylas-pp-cli lists update` — Updates the specified list. Only `name` and `description` can be updated. The list `type` is immutable after creation.

**migration-tools** — Manage migration tools

- `nylas-pp-cli migration-tools migration-clone-single-account` — Migrates a single v2 connected account to a v3 grant. Use this endpoint to migrate a few accounts to v3 as a test,...
- `nylas-pp-cli migration-tools migration-get-jobs` — Get information about the migration jobs for your application, including progress and status, for currently running...
- `nylas-pp-cli migration-tools migration-import-v2-app` — Import the settings from a v2 Nylas application to its linked v3 application. Before you use this endpoint, make...
- `nylas-pp-cli migration-tools migration-link-v2v3-apps` — Link an existing v2 application to an existing v3 application. <div id='admonition-warning'>⚠️<strong>Your v2...
- `nylas-pp-cli migration-tools migration-snapshot-batch-clone` — Starts a batch migration job to clone v2 connected accounts to v3 grants. Before you use this endpoint, your v2...
- `nylas-pp-cli migration-tools translate-v2id-to-provider-id` — Use the connected account ID and a resource type, with an optional list of specific Nylas IDs, to get a response...

**notetakers** — Nylas Notetaker is a real-time meeting bot that you can invite to your online meetings. It records and transcribes your discussion, and delivers results to you using the Nylas API and webhook notifications.

The [`/v3/grants/<NYLAS_GRANT_ID>/notetakers` endpoints](/docs/reference/api/notetaker/) let you interact with Nylas Notetaker while referencing a specific grant so you can use Notetaker's [calendar sync features](/docs/v3/notetaker/calendar-sync/) for your authenticated users.

The [`/v3/notetakers` endpoints](/docs/reference/api/standalone-notetaker/) let you send Notetakers that aren't connected to a grant as **standalone Notetakers**.

## Calendar sync

Notetaker works with the Nylas Calendar API to automatically sync with calendars and events. When you sync Notetaker with a grant's calendar or event, Nylas tracks their join times and meeting links so the Notetaker is always sent to the meeting on time.

## Webhook notifications

Nylas sends [webhook notifications](/docs/reference/notifications/#notetaker-notifications) when a Notetaker bot is created, updated, or deleted, and when the recording media is available.

- `nylas-pp-cli notetakers delete-standalone` — Permanently deletes the specified Notetaker and all associated data, including any recordings, transcripts,...
- `nylas-pp-cli notetakers get-all-standalone` — Returns a list of standalone Notetaker bots.
- `nylas-pp-cli notetakers get-standalone` — Returns the specified Notetaker bot and its details.
- `nylas-pp-cli notetakers invite-standalone` — Adds a standalone Notetaker bot to the specified meeting. <div id='admonition-info'>ℹ️ <b>Nylas doesn't...
- `nylas-pp-cli notetakers update-standalone` — Updates the specified scheduled standalone Notetaker bot.

**policies** — The Policies endpoints let you define the operational configuration for Nylas Agent Accounts, including message limits, attachment constraints, spam detection settings, and linked rules for inbound message filtering. Each policy is scoped to your application and can be assigned to one or more Agent Accounts.

Policies let you control:

- **Limits** — Attachment sizes, counts, allowed types, total storage, daily message quotas, and retention periods for the inbox and spam folders.
- **Spam detection** — DNS-based block list (DNSBL) checking, header anomaly detection, and sensitivity tuning.
- **Options** — Additional folder creation and CIDR-based email aliasing.
- **Rules** — Link filtering rules (created via the Rules API) to automatically process inbound messages based on sender criteria.

Policy limits are validated against your plan's maximum values. If a limit field is omitted, it defaults to the plan maximum. If a requested value exceeds the plan limit, the API returns an error.

- `nylas-pp-cli policies create-policy` — Creates a policy for your application. Policies define message limits, spam detection settings, options, and linked...
- `nylas-pp-cli policies delete-policy` — Deletes the specified policy. This action is irreversible.
- `nylas-pp-cli policies get-policy` — Returns the specified policy.
- `nylas-pp-cli policies list` — Returns a list of all policies for your application.
- `nylas-pp-cli policies update-policy` — Updates the specified policy. All fields are optional — only provided fields are updated. The same plan-limit,...

**providers** — Manage providers

- `nylas-pp-cli providers` — Returns the provider if one is detected. This operation is rate limited to 20 calls per minute for each Nylas...

**rules** — The Rules endpoints let you define automated filtering and routing logic for Nylas Agent Accounts. Each rule specifies a `trigger` (`inbound` or `outbound`), matching conditions, and actions to perform when those conditions are met.

**Inbound rules** run when mail arrives. Conditions match the sender — `from.address`, `from.domain`, or `from.tld`. Actions include blocking the message at the SMTP level, marking as spam, assigning to a folder, marking as read or starred, archiving, and trashing.

**Outbound rules** run before a send is submitted to the email provider. Conditions can match sender data (`from.address`, `from.domain`, `from.tld`), recipient data (`recipient.address`, `recipient.domain`, `recipient.tld`), or the send type via `outbound.type` (`compose` for new messages, `reply` for replies). A `block` action on an outbound rule rejects the send with HTTP 403 before it reaches the provider, so no message is delivered and no sent copy is stored. Non-blocking actions (`mark_as_spam`, `archive`, `mark_as_read`, `mark_as_starred`, `assign_to_folder`, `trash`) apply to the stored sent copy. `recipient.*` fields match against **any** recipient — including To, CC, BCC, and SMTP envelope recipients.

Inbound and outbound rules are isolated. Inbound rules never run during sends, and outbound rules never run on message receipt, so stored sent copies aren't re-evaluated against inbound rules.

Rules are evaluated in priority order (lower numbers first). Inbound rules are applied through the recipient grant's policy. Outbound rules are currently evaluated from the sending application's enabled outbound rules. The `block` action is terminal and cannot be combined with other actions.

Rules support the `in_list` condition operator, which checks field values against Lists (managed via the Lists API) for dynamic, maintainable allow and block lists. `in_list` works for `from.*` and `recipient.*` fields but not for `outbound.type`, which accepts only `is` and `is_not`.

Rule executions are audited per grant. List evaluation records with `GET /v3/grants/{grant_id}/rule-evaluations` to see which inbound or outbound rules ran, what normalized input was considered, and which actions were applied.

- `nylas-pp-cli rules create` — Creates a rule for your application. A rule defines a `trigger` (`inbound` or `outbound`), conditions against sender...
- `nylas-pp-cli rules delete` — Deletes the specified rule. This action is irreversible. Policies that reference the rule no longer apply it during...
- `nylas-pp-cli rules get` — Returns the specified rule.
- `nylas-pp-cli rules list` — Returns a list of all rules for your application.
- `nylas-pp-cli rules update` — Updates the specified rule. All fields are optional — only provided fields are updated. The same validation rules...

**scheduling** — Manage scheduling

- `nylas-pp-cli scheduling delete-bookings-id` — Deletes the specified booking. Nylas also cancels the associated event on the provider. Nylas validates the provided...
- `nylas-pp-cli scheduling delete-session` — Deletes a specific session.
- `nylas-pp-cli scheduling get-availability` — Gets available time slots within the given time range, using the rules defined in the specified Configuration...
- `nylas-pp-cli scheduling get-bookings-id` — Returns the specified Booking object. Nylas validates the provided session ID and uses it to retrieve the related...
- `nylas-pp-cli scheduling import-group-events` — Imports existing events to your group events Configuration. You can import up to 20 events per request.
- `nylas-pp-cli scheduling patch-bookings-id` — Reschedules the specified booking. Nylas also updates the associated event on the provider. Nylas recommends against...
- `nylas-pp-cli scheduling post-bookings` — Books an event with the participants listed in the session's [Configuration...
- `nylas-pp-cli scheduling post-sessions` — Creates a new short-lived session that you can pass to the Scheduling Component to enforce user authentication. Your...
- `nylas-pp-cli scheduling put-bookings-id` — Confirms or cancels the specified pending booking. Nylas also updates the associated event on the provider. Nylas...
- `nylas-pp-cli scheduling redirect` — Redirects an existing v2 Scheduling Page to a v3 URL.
- `nylas-pp-cli scheduling validate-time-slot` — Validates whether the selected group event or time slot has changed, and is invalid. This can happen because... -...

**templates** — Manage templates

- `nylas-pp-cli templates create-app-level` — Creates an application-level template.
- `nylas-pp-cli templates delete-app-level` — Deletes the specified application-level template.
- `nylas-pp-cli templates get-app-level` — Returns the specified application-level template.
- `nylas-pp-cli templates list-app-level` — Returns a list of application-level templates.
- `nylas-pp-cli templates render-html` — Renders the HTML content of an application-level template using the provided variables and specified templating engine.
- `nylas-pp-cli templates update-app-level` — Updates the specified application-level template.

**webhooks** — Manage webhooks

- `nylas-pp-cli webhooks delete-by-id` — Delete a webhook destination record.
- `nylas-pp-cli webhooks get-by-id` — Get the webhook destinations for an application ID by webhook ID
- `nylas-pp-cli webhooks get-destinations-application` — Get a list of all webhook destinations for an application id.
- `nylas-pp-cli webhooks get-mock-payload` — Use this endpoint to see example notification payloads for the different Nylas events you specify, to the webhook...
- `nylas-pp-cli webhooks post-destinations` — Creates a webhook destination with the specified URL and list of trigger types. ### Webhook destinations and retry...
- `nylas-pp-cli webhooks post-new-secret` — Update the webhook secret value for a destination. The previous value will immediately stop being used and the new...
- `nylas-pp-cli webhooks put-by-id` — Update the values in a specific webhook destination. ### Limitations - You only need to specify fields that need to...
- `nylas-pp-cli webhooks send-test-event` — Use this endpoint to check if your project's webhook destination is configured correctly. Nylas sends a test webhook...

**workflows** — Manage workflows

- `nylas-pp-cli workflows create` — Creates an application-level workflow. <div id='admonition-info'>ℹ️ <b>You must have an existing <a...
- `nylas-pp-cli workflows delete` — Deletes the specified application-level workflow.
- `nylas-pp-cli workflows get` — Returns the specified application-level workflow.
- `nylas-pp-cli workflows list` — Returns all application-level workflows.
- `nylas-pp-cli workflows update` — Updates the specified application-level workflow.

**workspaces** — Workspaces group and organize grants in a Nylas application by a common attribute, such as the email address domain (for example, `nylas.com`).

## Assign grants to workspaces

Nylas offers two endpoints to manage workspaces and the grants they contain:

- [**Automatically Group Grants into Workspaces**](/docs/reference/api/workspaces/autogroup-workspace/): Starts a background job that, based on your filters, processes existing grants in your Nylas application and automatically sorts them to existing workspaces. If necessary, Nylas automatically creates new workspaces.
- [**Update Workspace Assignments**](/docs/reference/api/workspaces/manually-assign-workspace/): Manually specify a workspace ID and up to 500 grants to add or remove from that workspace.

## Workspace limitations

- Workspaces are designed to group grants by the top-level domain (TLD) of users' email addresses. Nylas allows you to manually assign grants to any workspace, but that workspace must have `auto_group` set to `false`.
- When `auto_group` is set to `false`, Nylas doesn't automatically assign grants to that workspace. You'll need to manually assign grants to the workspace using the [Update Workspace Assignments endpoint](/docs/reference/api/workspaces/manually-assign-workspace/).
- When `auto_group` is `true`, Nylas automatically assigns new grants to the workspace. You can move grants to a different workspace by making an [Update Workspace Assignments request](/docs/reference/api/workspaces/manually-assign-workspace/).

- `nylas-pp-cli workspaces autogroup` — Configures automatic grouping settings for new or existing workspaces, depending on the filters set. If you don't...
- `nylas-pp-cli workspaces create` — Creates a workspace.
- `nylas-pp-cli workspaces delete` — Deletes the specified workspace.
- `nylas-pp-cli workspaces get` — Returns the specified workspace.
- `nylas-pp-cli workspaces get-all` — Returns all workspaces in your Nylas application. The application queried is determined based on the API key you use...
- `nylas-pp-cli workspaces update` — Updates the specified workspace.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
nylas-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily inbox triage across every grant

```bash
nylas-pp-cli sync --resources messages --since 24h
nylas-pp-cli since 2h --resource messages --agent --select id,subject,from,grant_id,unread | jq '.[] | select(.unread)'
```

Sync, then filter the mirror to the unread messages since 2h ago — answering 'what changed' from local data.

### Find every reference to a topic across mailboxes

```bash
nylas-pp-cli search "renewal" --type messages --limit 100 --agent --select grant_id,subject,from,date
```

FTS5 full-text search across the local mirror; one query spans every connected grant.

### SLA dashboard on first-response time

```bash
nylas-pp-cli response-time --group-by domain --since 30d --agent --select domain,p50,p90,n
```

Compute first-response latency from the local thread timeline, grouped by counterparty domain.

### Safe send: preview before sending

```bash
nylas-pp-cli grants messages send 550e8400-e29b-41d4-a716-446655440000 --to PII_EMAIL_EXAMPLE --subject "Q3" --body "..." --dry-run
nylas-pp-cli grants messages send 550e8400-e29b-41d4-a716-446655440000 --to PII_EMAIL_EXAMPLE --subject "Q3" --body "..." --yes
```

Two-step send: --dry-run prints the wire payload without calling the API; re-run with --yes to actually send.

### Replay a production webhook locally

```bash
nylas-pp-cli webhook-replay --since 24h --trigger message.created --to http://localhost:3000/hook
```

Re-fires every persisted message.created delivery from the last day into your local handler — no production wait required.

## Auth Setup

Nylas V3 uses a bearer API key (Authorization: Bearer <NYLAS_API_KEY>) for application-level access to every grant. Set NYLAS_API_KEY in your environment or run `nylas-pp-cli auth set-token`. For per-user OAuth flows, NYLAS_ACCESS_TOKEN is also supported. The grant_id for each connected mailbox is a path parameter on every per-grant endpoint, not an auth header.

Run `nylas-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  nylas-pp-cli applications get --agent --select id,name,status
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
nylas-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
nylas-pp-cli feedback --stdin < notes.txt
nylas-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.nylas-pp-cli/feedback.jsonl`. They are never POSTed unless `NYLAS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NYLAS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
nylas-pp-cli profile save briefing --json
nylas-pp-cli --profile briefing applications get
nylas-pp-cli profile list --json
nylas-pp-cli profile show briefing
nylas-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

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

1. **Empty, `help`, or `--help`** → show `nylas-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add nylas-pp-mcp -- nylas-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which nylas-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   nylas-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `nylas-pp-cli <command> --help`.
