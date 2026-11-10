# Klaviyo CLI

**Klaviyo from the shell, with local customer-behavior analytics layered on top.**

This printed CLI wraps Klaviyo's JSON:API surface for profiles, events, campaigns, flows, segments, metrics, templates, and related marketing resources. It adds a local SQLite mirror plus compound commands for campaign deployment, flow decay, attribution, deduplication, reconciliation, and growth planning.

Generated with Printing Press 4.35.4 against Klaviyo API revision 2026-10-15. Every request sends `revision: 2026-10-15`.

At this revision, a campaign is a container. Messages carry status and send time; campaign-audiences carry included and excluded lists and segments; variations carry channel, template, subject, and sender. IDs issued before 2026-10-15 are now message IDs. Campaign workflows accept either a container ID or a historical campaign/message ID and resolve its audiences and variations. The raw `campaigns get-id` read expects a container ID and explains historical IDs on a 404.

Implicit local stores and response caches are scoped by a non-secret API-key fingerprint and honor `XDG_DATA_HOME` / `XDG_CACHE_HOME`, so data from separate Klaviyo accounts cannot share a default database or cache namespace.

Learn more at [Klaviyo](https://developers.klaviyo.com).

`template-render --stdin` reads its JSON request body from stdin. Use
`campaign-message-assign-template --stdin` for a complete JSON body, or its
existing named body flags. Mixing stdin with body flags fails before a template request;
template assignment still checks the selected template's unsubscribe links.

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Campaign unsubscribe safety

Use `{% unsubscribe %}` as a standalone link or put `{% unsubscribe_link %}` in the
`Unsubscribe` anchor's `href`. Put `{% manage_preferences_link %}` in the
`Manage Preferences` anchor's `href`. Kit tags such as `{{ unsubscribe_url }}`
and `{{ manage_preferences_url }}` render dead links in Klaviyo and fail the check.

`campaigns deploy`, `campaign-message-assign-template`, and
`campaign-send-jobs send-campaign` check template HTML before writing or sending.
The send preflight checks every email variation and its assigned template; SMS and push variations do not use email checks. `templates create`,
`templates update`, and `template-clone` allow flow and transactional templates
without an unsubscribe link, but print a warning on stderr and include it in
JSON output. `plan qa-gate` reads every email variation's assigned template when given
`--campaign-id` without `--html`; a fail verdict exits nonzero after printing
the JSON findings.

Use `klaviyo-pp-cli --agent template-clone --template-id TEMPLATE_ID [--name NAME]` to
clone a template. The command reads the source HTML for its unsubscribe warning
and sends the source ID in the clone request.

## Install

The recommended path installs both the `klaviyo-pp-cli` binary and the `pp-klaviyo` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install klaviyo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install klaviyo --cli-only
```

For skill only - installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install klaviyo --skill-only
```

To constrain the skill install to one or more specific agents (repeatable - agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install klaviyo --agent claude-code
npx -y @mvanhorn/printing-press-library install klaviyo --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/cmd/klaviyo-pp-cli@latest
```

This installs the CLI only - no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/klaviyo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install klaviyo --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-klaviyo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-klaviyo --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install klaviyo --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle - Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/klaviyo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Set `PRINTING_PRESS_CLIENT_PROFILE` to the already configured client profile
   (for example, `acme`) when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/cmd/klaviyo-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "klaviyo": {
      "command": "klaviyo-pp-mcp",
      "env": {
        "PRINTING_PRESS_CLIENT_PROFILE": "acme"
      }
    }
  }
}
```

</details>

The MCP server uses the generator's code-orchestration surface: `klaviyo_search` discovers endpoints, `klaviyo_get` returns GET endpoint metadata without an API call, `klaviyo_read` runs read-only requests, and `klaviyo_execute` handles mutations. It also exposes local `search`, read-only `sql`, `context`, and the hand-written command tools. There is no separate tool for each endpoint.

`klaviyo_execute` returns a preview until `confirm` is boolean `true`; the string `"true"` does not confirm a write. Confirmed campaign sends and template assignments run the same unsubscribe checks as the CLI. The server and its learning commands use the selected client profile.

## Authentication

Data commands require a tenant-gated client profile. Profiles store only an
exact `op://` or `env:NAME` credential reference plus trusted non-secret identity assertions;
credential bytes remain in memory.

```bash
klaviyo-pp-cli client add acme --credential-ref 'op://Example/Klaviyo/private-api-key' --expected-account-id 'trusted-account-id' --expected-org-name 'Acme'
klaviyo-pp-cli client validate acme
klaviyo-pp-cli --client-profile acme whoami --no-cache
```

Selection order is `--client-profile`, `PRINTING_PRESS_CLIENT_PROFILE`, then
the shared default set by `client set-default`. Legacy Klaviyo credentials,
config, and DB overrides cannot replace a selected profile.

`teach`, `recall`, `learnings`, and `playbook` also require the selected, verified client profile. Their database and learning journal belong to that profile, so learned IDs, queries, and playbooks do not cross clients.

On hosts without 1Password, use `--credential-ref env:KLAVIYO_API_KEY` with the
same expected identity assertions. The variable must be set and non-empty when
running data commands. `--username-ref` and named `--additional-credential-ref`
values also accept `env:NAME`; profiles and `client show` retain references only.

## Quick Start

```bash
# Select and live-verify the tenant before any data command.
export PRINTING_PRESS_CLIENT_PROFILE=acme
klaviyo-pp-cli client validate acme
klaviyo-pp-cli whoami --no-cache

# Fetch one profile and keep the response small for agents.
klaviyo-pp-cli profiles get --page-size 1 --json --select data.id,data.email

# Populate local metric data for search and analytics.
klaviyo-pp-cli sync metrics

# Use the local mirror for a compound revenue view.
klaviyo-pp-cli attribution --metric "Placed Order" --group-by flow --since 2026-01-01 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Campaign operations

- **`campaigns deploy`** - Create an email template and a campaign with an inline audience, message, and email variation, then verify that the result is an unscheduled draft.

  Repeat `--list-id` and `--segment-id` to include audiences; repeat `--exclude` for excluded list or segment IDs. `--preview-text`, `--reply-to`, and `--send-timezone` set the email preview, reply address, and timezone. The reply address defaults to the sender and the timezone defaults to the account timezone. Use `--template-file` for an HTML file or `--template-html` for inline HTML. Deployment never schedules or sends the campaign.

  ```bash
  klaviyo-pp-cli campaigns deploy --template-file ./email.html --campaign-name "Acme offer" \
    --list-id LIST_ID --list-id SECOND_LIST_ID --segment-id SEGMENT_ID --exclude EXCLUDED_ID \
    --subject "Acme offer" --preview-text "This week's offer" --from-email marketing@example.com \
    --from-label "Acme" --reply-to replies@example.com --send-timezone America/Chicago --json
  ```
- **`campaigns test-send`** - Run the same campaign checks as `plan qa-gate`, then send template previews for email variations to up to five recipients. Use it to review a draft in an inbox before launch; a failed QA verdict sends nothing.

  Repeat `--to` or use comma-separated addresses; without `--to`, recipients come from `KLAVIYO_TEST_SEND_TO`. `--variation-id` limits preview sends to one email variation while QA still checks the campaign. `--dry-run` runs QA and shows planned sends without sending. Preview POSTs are never retried; an ambiguous result says "unknown, check the inbox".

  `--brand` reads Klaviyo's brand kit (brand colors, logos, buttons, email defaults), warns on off-brand colors and old logos, and fails on a missing footer. Add `--brand-strict-neutrals` to flag grays that are not in the palette.

  ```bash
  klaviyo-pp-cli campaigns test-send --campaign-id CAMPAIGN_ID --to qa@example.com --json
  ```
- **`campaigns image-swap`** - Replace an image URL in every targeted email variation's template while preserving the rest of the draft.

  _Agents can make safe creative swaps without rebuilding a whole campaign._

  ```bash
  klaviyo-pp-cli campaigns image-swap --campaign-id CAMPAIGN_ID --old-url https://cdn.example.com/old.jpg --new-url https://cdn.example.com/new.jpg --json
  ```

- **`campaigns ab-test create`** - Add one to six subject-line alternatives to an unscheduled draft email message. The existing variation is A; new variations B through G share A's template and sender fields.

  Repeat `--subject` for each new variation. Preview text is copied from A unless `--preview-text` is repeated once per new subject. Empty subjects, duplicate subjects, scheduled messages and existing tests are refused. `--campaign-id` accepts a container with exactly one email message or a message ID; containers with several messages list the IDs to choose from.

  `--winner-metric` accepts `clicks` (default), `opens` or `orders`. `--test-percent` defaults to 20 and accepts 1 to 100; this share is split evenly across all variations, and the remaining share receives the winner. `--wait-hours` defaults to 4 and accepts 1 to 264. Apple Mail Privacy Protection inflates opens, so clicks or orders pick a truer winner.

  `--dry-run` reads current state and prints the plan without writing. Creates are attempted once; a failed create or settings update triggers best-effort deletion of this run's known new variations. An ambiguous create can leave an unconfirmed variation and reports that uncertainty. A successful command re-reads subjects, settings, draft status and send time. It never schedules or sends. Run `campaigns test-send --campaign-id acme-message-id` afterward to preview every variation.

  ```bash
  klaviyo-pp-cli campaigns ab-test create --campaign-id acme-message-id \
    --subject "Acme: a new way to start" --subject "Start with Acme today" \
    --preview-text "Your next step" --preview-text "See what's new" \
    --winner-metric clicks --test-percent 20 --wait-hours 4 --dry-run --json
  ```

- **`campaigns ab-test results`** - Read recipient counts, unique opens and clicks, rates, orders, revenue and revenue per recipient for each email variation. Unsent messages return `too_early` without querying reports. Missing variation rows produce zeros.

  `--metric` accepts `clicks`, `opens` or `orders`; it defaults to the test's winner metric, then clicks. The rate leader must beat every other variation in a two-sided pooled two-proportion z-test with p < 0.05 for `clear_winner`; otherwise the verdict is `no_clear_winner`. P-values, an opens privacy note and small-sample notes (fewer than 100 selected metric events) accompany the result. When the winner audience share is positive, the most-mailed variation is labelled as a Klaviyo winner inferred from recipient counts.

  Reports use the day before the send date through now in `KLAVIYO_TIMEZONE` (default UTC), capped to the provider's 364-day window. The conversion metric uses the existing Shopify Placed Order auto-resolution. JSON includes message ID, A/B settings, timeframe, variations, verdict, leader, p-values and notes.

  ```bash
  klaviyo-pp-cli campaigns ab-test results --campaign-id acme-message-id --metric clicks --json
  ```

### Flow and form operations

- **`flows insert-step`** - Preview one new action after an existing flow action. Use `--step-file` for a JSON action definition without links, or `--stdin` to read it from stdin; add `--apply` to insert it once and verify placement.

  `--after` identifies the existing action. `--link` selects `next`, `next_if_true`, `next_if_false`, or a multi-branch branch ID. Unsupported step types are refused, and timing or logic steps in a live flow require `--allow-live-action`.

  ```bash
  klaviyo-pp-cli flows insert-step FLOW_ID --after ACTION_ID --step-file ./step.json --json
  ```
- **`forms publish`** - Review live-versus-draft changes and the last 30 days of views, submits, and submit rate before publishing a form. The default is a preview; `--apply` publishes and re-reads the form to verify its status.

  ```bash
  klaviyo-pp-cli forms publish FORM_ID --apply --json
  ```
- **`forms unpublish`** - Take a live form down after reviewing its draft and performance evidence. The default is a preview; `--apply` unpublishes and verifies the status.

  ```bash
  klaviyo-pp-cli forms unpublish FORM_ID --apply --json
  ```

Both form commands refuse a draft A/B test. `--dry-run` always previews, even with `--apply`; ambiguous publish or unpublish requests are never retried.

### Behavior graph analytics
- **`flow-decay`** - Identify flows whose open or click performance has decayed across recent time buckets using synced local data.

  _Agents can spot lifecycle automations that need refresh before revenue falls further._

  ```bash
  klaviyo-pp-cli flow-decay --days 90 --threshold 0.15 --json
  ```
- **`cohort`** - Group profiles by first event date and compute retention or repeat-action curves from synced profiles and events.

  _Agents can answer which acquisition cohorts keep buying without exporting CSVs._

  ```bash
  klaviyo-pp-cli cohort --metric "Placed Order" --interval month --json --select cohort,profiles,retained
  ```
- **`attribution`** - Join order events with campaign and flow attribution properties to summarize revenue by channel and source.

  _Agents can explain which automation or campaign generated revenue using local event evidence._

  ```bash
  klaviyo-pp-cli attribution --metric "Placed Order" --group-by flow --since 2026-01-01 --json
  ```

### Reporting

- **`send-budget`** - Compare email sends in the current billing period with the plan cap and scheduled email message recipient estimates. Use it before scheduling more sends; later periods are shown against the same cap, and missing estimates remain unknown rather than zero.

  Missing or stale estimates are refreshed with estimation jobs by default. `--wait` bounds that work (default two minutes); `--no-refresh` uses GET requests only.

  ```bash
  klaviyo-pp-cli send-budget --no-refresh --json
  ```
- **`campaign-values-reports`** - Query campaign performance with a complete JSON:API report body. `--conversion-metric-id` defaults to the account's Shopify Placed Order metric.

  ```bash
  klaviyo-pp-cli campaign-values-reports --timeframe last_12_months --statistics recipients,opens,clicks,conversion_value --agent
  ```
- **`report targeting`** - Resolve included and excluded campaign audience IDs against both segments and lists, join sends, attributed revenue, and last-mailed dates, and return distinct never-mailed segment and list collections.

  ```bash
  klaviyo-pp-cli report targeting --timeframe last_12_months --agent
  ```

### Data hygiene
- **`list-quality-score`** - Score every list using API-valid collection fields plus paced per-list `profile_count` enrichment.

  _The count enrichment observes Klaviyo's 1/s burst and 15/m steady limit and the HTTP client honors `Retry-After`; large accounts can take several minutes._

  ```bash
  klaviyo-pp-cli list-quality-score --json --no-cache
  ```
- **`segments get --with-counts`** - Enrich a segment page with current `profile_count` values through the per-item endpoint. The `list` alias has the same behavior.

  _Use `--all` when every page is required. The enrichment is deliberately paced at the additional-field endpoint's 15/m steady limit._

  ```bash
  klaviyo-pp-cli segments get --all --with-counts --json --no-cache
  ```
- **`dedup`** - Find profiles that appear duplicated by email, phone, or cross-channel collisions in the local profile mirror.

  _Agents can flag customer records that split behavior and revenue history across identities._

  ```bash
  klaviyo-pp-cli dedup --by email,phone --json
  ```
- **`reconcile`** - Compare campaign UTM evidence with local Klaviyo order events and optional Shopify credentials when available.

  _Agents can check whether campaign performance agrees with order evidence before reporting numbers._

  ```bash
  klaviyo-pp-cli reconcile --campaign-id CAMPAIGN_ID --since 2026-01-01 --json
  ```

### Growth planning
- **`plan brief-to-strategy`** - Turn a growth brief into a structured Klaviyo campaign, flow, segment, and experiment strategy.

  _Agents can convert strategy notes into a concrete Klaviyo execution plan._

  ```bash
  klaviyo-pp-cli plan brief-to-strategy --brief ./brief.md --json
  ```
- **`plan qa-gate`** - Run a launch-readiness checklist for links, offers, dates, timezone, fallback tokens, unsubscribe placement, compliance, and deliverability flags.

  _Agents can block risky campaign launches with explicit findings instead of vague review notes._

  ```bash
  klaviyo-pp-cli plan qa-gate --campaign-id CAMPAIGN_ID --html ./email.html --json
  ```

  `--brand` reads Klaviyo's brand kit (brand colors, logos, buttons, email defaults), warns on off-brand colors and old logos, and fails on a missing footer. Add `--brand-strict-neutrals` to flag grays that are not in the palette.

## Usage

Run `klaviyo-pp-cli --help` for the full command reference and flag list.

## Commands

### accounts

accounts

- **`klaviyo-pp-cli accounts get`** - Retrieve the account(s) associated with a given private API key. This will return 1 account object within the array.

You can use this to retrieve account-specific data (contact information, timezone, currency, Public API key, etc.) or test if a Private API Key belongs to the correct account prior to performing subsequent actions with the API.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`accounts:read`
- **`klaviyo-pp-cli accounts get-id`** - Retrieve a single account object by its account ID. You can only request the account by which the private API key was generated.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`accounts:read`

### back-in-stock-subscriptions

Manage back in stock subscriptions

- **`klaviyo-pp-cli back-in-stock-subscriptions`** - Subscribe a profile to receive back in stock notifications. Check out [our Back in Stock API guide](https://developers.klaviyo.com/en/docs/how_to_set_up_custom_back_in_stock) for more details.

This endpoint is specifically designed to be called from server-side applications. To create subscriptions from client-side contexts, use [POST /client/back-in-stock-subscriptions](https://developers.klaviyo.com/en/reference/create_client_back_in_stock_subscription).<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:write`
`profiles:write`

### campaign-clone

Manage campaign clone

- **`klaviyo-pp-cli campaign-clone`** - Clones an existing campaign, returning a new campaign based on the original with a new ID and name.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:write`

### campaign-message-assign-template

Manage campaign message assign template

- **`klaviyo-pp-cli campaign-message-assign-template`** - Creates a non-reusable template copy and assigns it to an email variation. `--data-id` is a variation ID; a message ID is accepted only when that message has exactly one email variation. `--stdin` resolves the ID while preserving the request body and checking unsubscribe links.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:write`

### campaign-messages

Manage campaign messages

- **`klaviyo-pp-cli campaign-messages get`** - Lists campaign messages with optional filters. Messages carry status and send time; channel and content belong to their variations.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:read`
- **`klaviyo-pp-cli campaign-messages update`** - Update a campaign message<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:write`

### campaign-recipient-estimation-jobs

Manage campaign recipient estimation jobs

- **`klaviyo-pp-cli campaign-recipient-estimation-jobs get`** - Retrieve the status of a recipient estimation job triggered
with the `Create Campaign Recipient Estimation Job` endpoint.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:read`
- **`klaviyo-pp-cli campaign-recipient-estimation-jobs refresh-campaign-recipient-estimation`** - Trigger an asynchronous job to update the estimated number of recipients
for the given campaign ID. Use the `Get Campaign Recipient Estimation
Job` endpoint to retrieve the status of this estimation job. Use the
`Get Campaign Recipient Estimation` endpoint to retrieve the estimated
recipient count for a given campaign.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:write`

### campaign-recipient-estimations

Manage campaign recipient estimations

- **`klaviyo-pp-cli campaign-recipient-estimations <id>`** - Get the estimated recipient count for a campaign with the provided campaign ID.
You can refresh this count by using the `Create Campaign Recipient Estimation Job` endpoint.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:read`

### campaign-send-jobs

Manage campaign send jobs

- **`klaviyo-pp-cli campaign-send-jobs cancel-campaign-send`** - Permanently cancel the campaign, setting the status to CANCELED or
revert the campaign, setting the status back to DRAFT<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:write`
- **`klaviyo-pp-cli campaign-send-jobs get`** - Get a campaign send job<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:read`
- **`klaviyo-pp-cli campaign-send-jobs send-campaign`** - Trigger a campaign to send asynchronously<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:write`

### campaign-values-reports

Manage campaign values reports

- **`klaviyo-pp-cli campaign-values-reports`** - Returns the requested campaign analytics values data<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `2/m`<br>Daily: `225/d`

**Scopes:**
`campaigns:read`

### campaigns

Campaign containers, messages, audiences, and variations use the 2026-10-15 model described above. Use `campaigns deploy` for an unscheduled email draft and `campaigns test-send` for QA followed by inbox previews. Use `campaigns ab-test create` for subject-line alternatives and `campaigns ab-test results` to compare their performance.

`campaigns create` translates legacy `--data-attributes-name` and `--data-attributes-campaign-messages` into the new definition, messages, and variations. A legacy audience object becomes one inline audience. `--stdin` bodies pass through unchanged and must already use the new model.

The flags `--data-attributes-send-options`, `--data-attributes-send-strategy`, and `--data-attributes-tracking-options` were removed. Put campaign `send_settings` in `--data-attributes-definition`; use `--data-attributes-messages` for supported message settings such as `is_add_utm` and `throttling_type`.

- **`klaviyo-pp-cli campaigns create`** - Creates a campaign given a set of parameters, then returns it.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:write`
- **`klaviyo-pp-cli campaigns delete`** - Delete a campaign with the given campaign ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:write`
- **`klaviyo-pp-cli campaigns get`** - Lists campaign containers with optional filters; no channel filter is required or supported on the campaign collection. Read message status and send time from messages and channel from variations.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:read`
- **`klaviyo-pp-cli campaigns get-id`** - Returns a campaign container by ID. A 404 explains that IDs from before 2026-10-15 are now message IDs.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:read`
- **`klaviyo-pp-cli campaigns update`** - Update a campaign with the given campaign ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`campaigns:write`

### catalog-categories

Manage catalog categories

- **`klaviyo-pp-cli catalog-categories create-catalog-category`** - Create a new catalog category.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-categories delete-catalog-category`** - Delete a catalog category using the given category ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-categories get`** - Get all catalog categories in an account.

Catalog categories can be sorted by the following fields, in ascending and descending order:
`created`

Currently, the only supported integration type is `$custom`, and the only supported catalog type is `$default`.

Returns a maximum of 100 categories per request.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-categories get-catalog-category`** - Get a catalog category with the given category ID.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-categories update-catalog-category`** - Update a catalog category with the given category ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`

### catalog-category-bulk-create-jobs

Manage catalog category bulk create jobs

- **`klaviyo-pp-cli catalog-category-bulk-create-jobs bulk-create-catalog-categories`** - Create a catalog category bulk create job to create a batch of catalog categories.

Accepts up to 100 catalog categories per request. The maximum allowed payload size is 5MB.
The maximum number of jobs in progress at one time is 500.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-category-bulk-create-jobs get-bulk-create-categories-job`** - Get a catalog category bulk create job with the given job ID.

An `include` parameter can be provided to get the following related resource data: `categories`.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-category-bulk-create-jobs get-bulk-create-categories-jobs`** - Get all catalog category bulk create jobs.

Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`

### catalog-category-bulk-delete-jobs

Manage catalog category bulk delete jobs

- **`klaviyo-pp-cli catalog-category-bulk-delete-jobs bulk-delete-catalog-categories`** - Create a catalog category bulk delete job to delete a batch of catalog categories.

Accepts up to 100 catalog categories per request. The maximum allowed payload size is 5MB.
The maximum number of jobs in progress at one time is 500.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-category-bulk-delete-jobs get-bulk-delete-categories-job`** - Get a catalog category bulk delete job with the given job ID.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-category-bulk-delete-jobs get-bulk-delete-categories-jobs`** - Get all catalog category bulk delete jobs.

Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`

### catalog-category-bulk-update-jobs

Manage catalog category bulk update jobs

- **`klaviyo-pp-cli catalog-category-bulk-update-jobs bulk-update-catalog-categories`** - Create a catalog category bulk update job to update a batch of catalog categories.

Accepts up to 100 catalog categories per request. The maximum allowed payload size is 5MB.
The maximum number of jobs in progress at one time is 500.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-category-bulk-update-jobs get-bulk-update-categories-job`** - Get a catalog category bulk update job with the given job ID.

An `include` parameter can be provided to get the following related resource data: `categories`.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-category-bulk-update-jobs get-bulk-update-categories-jobs`** - Get all catalog category bulk update jobs.

Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`

### catalog-item-bulk-create-jobs

Manage catalog item bulk create jobs

- **`klaviyo-pp-cli catalog-item-bulk-create-jobs bulk-create-catalog-items`** - Create a catalog item bulk create job to create a batch of catalog items.

Accepts up to 100 catalog items per request. The maximum allowed payload size is 5MB.
The maximum number of jobs in progress at one time is 500.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-item-bulk-create-jobs get-bulk-create-catalog-items-job`** - Get a catalog item bulk create job with the given job ID.

An `include` parameter can be provided to get the following related resource data: `items`.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-item-bulk-create-jobs get-bulk-create-catalog-items-jobs`** - Get all catalog item bulk create jobs.

Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`

### catalog-item-bulk-delete-jobs

Manage catalog item bulk delete jobs

- **`klaviyo-pp-cli catalog-item-bulk-delete-jobs bulk-delete-catalog-items`** - Create a catalog item bulk delete job to delete a batch of catalog items.

Accepts up to 100 catalog items per request. The maximum allowed payload size is 5MB.
The maximum number of jobs in progress at one time is 500.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-item-bulk-delete-jobs get-bulk-delete-catalog-items-job`** - Get a catalog item bulk delete job with the given job ID.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-item-bulk-delete-jobs get-bulk-delete-catalog-items-jobs`** - Get all catalog item bulk delete jobs.

Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`

### catalog-item-bulk-update-jobs

Manage catalog item bulk update jobs

- **`klaviyo-pp-cli catalog-item-bulk-update-jobs bulk-update-catalog-items`** - Create a catalog item bulk update job to update a batch of catalog items.

Accepts up to 100 catalog items per request. The maximum allowed payload size is 5MB.
The maximum number of jobs in progress at one time is 500.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-item-bulk-update-jobs get-bulk-update-catalog-items-job`** - Get a catalog item bulk update job with the given job ID.

An `include` parameter can be provided to get the following related resource data: `items`.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-item-bulk-update-jobs get-bulk-update-catalog-items-jobs`** - Get all catalog item bulk update jobs.

Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`

### catalog-items

Manage catalog items

- **`klaviyo-pp-cli catalog-items create`** - Create a new catalog item.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-items delete`** - Delete a catalog item with the given item ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-items get`** - Get all catalog items in an account.

Catalog items can be sorted by the following fields, in ascending and descending order:
`created`

Currently, the only supported integration type is `$custom`, and the only supported catalog type is `$default`.

Returns a maximum of 100 items per request.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-items get-catalogitems`** - Get a specific catalog item with the given item ID.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-items update`** - Update a catalog item with the given item ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`

### catalog-variant-bulk-create-jobs

Manage catalog variant bulk create jobs

- **`klaviyo-pp-cli catalog-variant-bulk-create-jobs bulk-create-catalog-variants`** - Create a catalog variant bulk create job to create a batch of catalog variants.

Accepts up to 100 catalog variants per request. The maximum allowed payload size is 5MB.
The maximum number of jobs in progress at one time is 500.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-variant-bulk-create-jobs get-bulk-create-variants-job`** - Get a catalog variant bulk create job with the given job ID.

An `include` parameter can be provided to get the following related resource data: `variants`.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-variant-bulk-create-jobs get-bulk-create-variants-jobs`** - Get all catalog variant bulk create jobs.

Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`

### catalog-variant-bulk-delete-jobs

Manage catalog variant bulk delete jobs

- **`klaviyo-pp-cli catalog-variant-bulk-delete-jobs bulk-delete-catalog-variants`** - Create a catalog variant bulk delete job to delete a batch of catalog variants.

Accepts up to 100 catalog variants per request. The maximum allowed payload size is 5MB.
The maximum number of jobs in progress at one time is 500.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-variant-bulk-delete-jobs get-bulk-delete-variants-job`** - Get a catalog variant bulk delete job with the given job ID.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-variant-bulk-delete-jobs get-bulk-delete-variants-jobs`** - Get all catalog variant bulk delete jobs.

Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`

### catalog-variant-bulk-update-jobs

Manage catalog variant bulk update jobs

- **`klaviyo-pp-cli catalog-variant-bulk-update-jobs bulk-update-catalog-variants`** - Create a catalog variant bulk update job to update a batch of catalog variants.

Accepts up to 100 catalog variants per request. The maximum allowed payload size is 5MB.
The maximum number of jobs in progress at one time is 500.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-variant-bulk-update-jobs get-bulk-update-variants-job`** - Get a catalog variate bulk update job with the given job ID.

An `include` parameter can be provided to get the following related resource data: `variants`.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-variant-bulk-update-jobs get-bulk-update-variants-jobs`** - Get all catalog variant bulk update jobs.

Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`

### catalog-variants

Manage catalog variants

- **`klaviyo-pp-cli catalog-variants create`** - Create a new variant for a related catalog item.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-variants delete`** - Delete a catalog item variant with the given variant ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`
- **`klaviyo-pp-cli catalog-variants get`** - Get all variants in an account.

Variants can be sorted by the following fields, in ascending and descending order:
`created`

Currently, the only supported integration type is `$custom`, and the only supported catalog type is `$default`.

Returns a maximum of 100 variants per request.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-variants get-catalogvariants`** - Get a catalog item variant with the given variant ID.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:read`
- **`klaviyo-pp-cli catalog-variants update`** - Update a catalog item variant with the given variant ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`catalogs:write`

### client

client

- **`klaviyo-pp-cli client bulk-create-events`** - Create new events to track a profile's activity.

This endpoint is specifically designed to be called from publicly-browseable, client-side environments only and requires a [public API key (site ID)](https://www.klaviyo.com/settings/account/api-keys). Never use a private API key with our client-side endpoints.

Do not use this endpoint from server-side applications.
To create events from server-side applications, instead use [POST /api/event-bulk-create-jobs](https://developers.klaviyo.com/en/reference/bulk_create_events).

Accepts a maximum of `1000` events per request.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`events:write`
- **`klaviyo-pp-cli client create-back-in-stock-subscription`** - Subscribe a profile to receive back in stock notifications. Check out [our Back in Stock API guide](https://developers.klaviyo.com/en/docs/how_to_set_up_custom_back_in_stock) for more details.

This endpoint is specifically designed to be called from publicly-browseable, client-side environments only and requires a [public API key (site ID)](https://www.klaviyo.com/settings/account/api-keys). Never use a private API key with our client-side endpoints.

Do not use this endpoint from server-side applications.
To create back in stock subscriptions from server-side applications, instead use [POST /api/back-in-stock-subscriptions](https://developers.klaviyo.com/en/reference/create_back_in_stock_subscription).<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`catalogs:write`
`profiles:write`
- **`klaviyo-pp-cli client create-event`** - Create a new event to track a profile's activity.

This endpoint is specifically designed to be called from publicly-browseable, client-side environments only and requires a [public API key (site ID)](https://www.klaviyo.com/settings/account/api-keys). Never use a private API key with our client-side endpoints.

Do not use this endpoint from server-side applications.
To create events from server-side applications, instead use [POST /api/events](https://developers.klaviyo.com/en/reference/create_event).

Note that to update a profile's existing identifiers (e.g., email), you must use a server-side endpoint authenticated by a private API key. Attempts to do so via client-side endpoints will return a 202, however the identifier field(s) will not be updated.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`events:write`
- **`klaviyo-pp-cli client create-profile`** - Create or update properties about a profile without tracking an associated event.

This endpoint is specifically designed to be called from publicly-browseable, client-side environments only and requires a [public API key (site ID)](https://www.klaviyo.com/settings/account/api-keys). Never use a private API key with our client-side endpoints.

Do not use this endpoint from server-side applications.
To create or update profiles from server-side applications, instead use [POST /api/profile-import](https://developers.klaviyo.com/en/reference/create_or_update_profile).

Note that to update a profile's existing identifiers (e.g., email), you must use a server-side endpoint authenticated by a private API key. Attempts to do so via client-side endpoints will return a 202, however the identifier field(s) will not be updated.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`profiles:write`
- **`klaviyo-pp-cli client create-push-token`** - Create or update a push token.

This endpoint is specifically designed to be called from our mobile SDKs ([iOS](https://github.com/klaviyo/klaviyo-swift-sdk) and [Android](https://github.com/klaviyo/klaviyo-android-sdk)) and requires a [public API key (site ID)](https://www.klaviyo.com/settings/account/api-keys). Never use a private API key with our client-side endpoints.
You must have push notifications enabled to use this endpoint.

To migrate push tokens from another platform to Klaviyo, please use our server-side [POST /api/push-tokens](https://developers.klaviyo.com/en/reference/create_push_token) endpoint instead.<br><br>*Rate limits*:<br>Burst: `150/s`<br>Steady: `1400/m`
- **`klaviyo-pp-cli client create-review`** - Create a review with the given ID. This endpoint is for client-side environments only.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`
- **`klaviyo-pp-cli client create-subscription`** - Creates a subscription and consent record for email and/or SMS channels based on the provided `email` and `phone_number` attributes, respectively. One of either `email` or `phone_number` must be provided.

This endpoint is specifically designed to be called from publicly-browseable, client-side environments only and requires a [public API key (site ID)](https://www.klaviyo.com/settings/account/api-keys). Never use a private API key with our client-side endpoints.

Do not use this endpoint from server-side applications.
To subscribe profiles from server-side applications, instead use [POST /api/profile-subscription-bulk-create-jobs](https://developers.klaviyo.com/en/reference/subscribe_profiles).

Profiles can be opted into multiple channels: email marketing, SMS marketing, and SMS transactional. You can specify the channel(s) to subscribe the profile to by providing a subscriptions object in the profile attributes.

If you include a subscriptions object, only channels in that object will be subscribed.  You can use this to update `email` or `phone` on the profile without subscribing them, for example, by setting the profile property but omitting that channel in the subscriptions object. If a subscriptions object is not provided, subscriptions are defaulted to `MARKETING`.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `750/m`

**Scopes:**
`subscriptions:write`
- **`klaviyo-pp-cli client get-geofences`** - Get all geofences in an account.

Returns a paginated list of all geofences for the specified company.
This is the GA API endpoint designed for mobile SDK consumption.
No authentication required.

Returns a maximum of 100 results per page (default 20).

This API supports filtering via header instead of query param. Provide
`X-Klaviyo-API-Filters` header to filter geofences. We don't use regular
query param filters here because lat and long are sensitive information.

Supported filters:
- `lat` (equals) - Latitude coordinate for distance-based sorting
- `lng` (equals) - Longitude coordinate for distance-based sorting

When both lat and lng are provided, geofences are returned sorted by
distance from the specified coordinates (closest first).

Example filter header:
`X-Klaviyo-API-Filters: and(equals(lat,40.7128),equals(lng,-74.0060))`<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`
- **`klaviyo-pp-cli client get-review-values-reports`** - Get all reviews values reports in an account.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`
- **`klaviyo-pp-cli client get-reviews`** - Get all reviews. This endpoint is for client-side environments only, for server-side use, refer to https://developers.klaviyo.com/en/reference/get_reviews<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`
- **`klaviyo-pp-cli client unregister-push-token`** - Unregister a push token.

This endpoint is specifically designed to be called from our mobile SDKs ([iOS](https://github.com/klaviyo/klaviyo-swift-sdk) and [Android](https://github.com/klaviyo/klaviyo-android-sdk)) and requires a [public API key (site ID)](https://www.klaviyo.com/settings/account/api-keys). Never use a private API key with our client-side endpoints.
You must have push notifications enabled to use this endpoint.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

### conversation-messages

Manage conversation messages

- **`klaviyo-pp-cli conversation-messages`** - Send an outbound message to a conversation.

Requires OAuth authentication and account-level enablement. To request access, reach out in the [developer community](https://community.klaviyo.com/groups/developer-group-64).<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`conversations:write`

### coupon-code-bulk-create-jobs

Manage coupon code bulk create jobs

- **`klaviyo-pp-cli coupon-code-bulk-create-jobs bulk-create-coupon-codes`** - Create a coupon-code-bulk-create-job to bulk create a list of coupon codes.

Max number of coupon codes per job we allow for is 1000.
Max number of jobs queued at once we allow for is 100.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`coupon-codes:write`
- **`klaviyo-pp-cli coupon-code-bulk-create-jobs get-bulk-create-coupon-code-jobs`** - Get all coupon code bulk create jobs.

Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`coupon-codes:read`
- **`klaviyo-pp-cli coupon-code-bulk-create-jobs get-bulk-create-coupon-codes-job`** - Get a coupon code bulk create job with the given job ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`coupon-codes:read`

### coupon-codes

Manage coupon codes

- **`klaviyo-pp-cli coupon-codes create`** - Synchronously creates a coupon code for the given coupon.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`coupon-codes:write`
- **`klaviyo-pp-cli coupon-codes delete`** - Deletes a coupon code specified by the given identifier synchronously. If a profile has been assigned to the
coupon code, an exception will be raised<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`coupon-codes:write`
- **`klaviyo-pp-cli coupon-codes get`** - Gets a list of coupon codes associated with a coupon/coupons or a profile/profiles.

A coupon/coupons or a profile/profiles must be provided as required filter params.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`coupon-codes:read`
- **`klaviyo-pp-cli coupon-codes get-couponcodes`** - Returns a Coupon Code specified by the given identifier.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`coupon-codes:read`
- **`klaviyo-pp-cli coupon-codes update`** - Updates a coupon code specified by the given identifier synchronously. We allow updating the 'status' and
'expires_at' of coupon codes.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`coupon-codes:write`

### coupons

coupons

- **`klaviyo-pp-cli coupons create`** - Creates a new coupon.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`coupons:write`
- **`klaviyo-pp-cli coupons delete`** - Delete the coupon with the given coupon ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`coupons:write`
- **`klaviyo-pp-cli coupons get`** - Get all coupons in an account.

To learn more, see our [Coupons API guide](https://developers.klaviyo.com/en/docs/use_klaviyos_coupons_api).<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`coupons:read`
- **`klaviyo-pp-cli coupons get-id`** - Get a specific coupon with the given coupon ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`coupons:read`
- **`klaviyo-pp-cli coupons update`** - *Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`coupons:write`

### custom-metrics

Manage custom metrics

- **`klaviyo-pp-cli custom-metrics create`** - Create a new custom metric.

Custom metric objects must include a `name` and `definition`.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`<br>Daily: `15/d`

**Scopes:**
`metrics:write`
- **`klaviyo-pp-cli custom-metrics delete`** - Delete a custom metric with the given custom metric ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`metrics:write`
- **`klaviyo-pp-cli custom-metrics get`** - Get all custom metrics in an account.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`metrics:read`
- **`klaviyo-pp-cli custom-metrics get-custommetrics`** - Get a custom metric with the given custom metric ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`metrics:read`
- **`klaviyo-pp-cli custom-metrics update`** - Update a custom metric with the given custom metric ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`<br>Daily: `15/d`

**Scopes:**
`metrics:write`

### data-privacy-deletion-jobs

Manage data privacy deletion jobs

- **`klaviyo-pp-cli data-privacy-deletion-jobs`** - Request a deletion for the profiles corresponding to one of the following identifiers: `email`, `phone_number`, or `id`. If multiple identifiers are provided, we will return an error.

All profiles that match the provided identifier will be deleted.

The deletion occurs asynchronously; however, once it has completed, the deleted profile will appear on the [Deleted Profiles page](https://www.klaviyo.com/account/deleted).

For more information on the deletion process, please refer to our [Help Center docs on how to handle GDPR and CCPA deletion requests](https://help.klaviyo.com/hc/en-us/articles/360004217631-How-to-Handle-GDPR-Requests#record-gdpr-and-ccpa%20%20-deletion-requests2).<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`data-privacy:write`

### data-source-record-bulk-create-jobs

Manage data source record bulk create jobs

- **`klaviyo-pp-cli data-source-record-bulk-create-jobs`** - Create a bulk data source record import job to create a batch of records.

Accepts up to 500 records per request. The maximum allowed payload size is 4MB. The maximum allowed payload size per-record is 512KB.

To learn more, see our [Custom Objects API overview](https://developers.klaviyo.com/en/reference/custom_objects_api_overview).<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `15/m`

**Scopes:**
`custom-objects:write`

### data-source-record-create-jobs

Manage data source record create jobs

- **`klaviyo-pp-cli data-source-record-create-jobs`** - Create a data source record import job to create a single record.

The maximum allowed payload size per-record is 512KB.

To learn more, see our [Custom Objects API overview](https://developers.klaviyo.com/en/reference/custom_objects_api_overview).<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`custom-objects:write`

### data-sources

Manage data sources

- **`klaviyo-pp-cli data-sources create`** - Create a new data source in an account<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`custom-objects:write`
- **`klaviyo-pp-cli data-sources delete`** - Delete a data source in an account.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`custom-objects:write`
- **`klaviyo-pp-cli data-sources get`** - Get all data sources in an account.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`custom-objects:read`
- **`klaviyo-pp-cli data-sources get-datasources`** - Retrieve a data source in an account.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`custom-objects:read`

### event-bulk-create-jobs

Manage event bulk create jobs

- **`klaviyo-pp-cli event-bulk-create-jobs`** - Create a batch of events for one or more profiles.

Note that this endpoint allows you to create new profiles or update existing profile properties.

At a minimum, profile and metric objects should include at least one profile identifier (e.g., `id`, `email`, or `phone_number`) and the metric `name`, respectively.

Accepts up to 1,000 events per request. The maximum allowed payload size is 5MB. A single string cannot exceed 100KB.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`events:write`

### events

events

- **`klaviyo-pp-cli events create`** - Create a new event to track a profile's activity.

Note that this endpoint allows you to create a new profile or update an existing profile's properties.

At a minimum, profile and metric objects should include at least one profile identifier (e.g., `id`, `email`, or `phone_number`) and the metric `name`, respectively.

Successful response indicates that the event was validated and submitted for processing, but does not guarantee that processing is complete.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`events:write`
- **`klaviyo-pp-cli events get`** - Get all events in an account

Requests can be sorted by the following fields:
`datetime`, `timestamp`

[Custom metrics](https://developers.klaviyo.com/en/reference/custom_metrics_api_overview) are not supported in the `metric_id` filter.

Returns a maximum of 200 events per page.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`

**Scopes:**
`events:read`
- **`klaviyo-pp-cli events get-id`** - Get an event with the given event ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`events:read`

### flow-actions

Manage flow actions

- **`klaviyo-pp-cli flow-actions get`** - Get a flow action from a flow with the given flow action ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`flows:read`
- **`klaviyo-pp-cli flow-actions update`** - Update a flow action.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`flows:write`

### flow-messages

Manage flow messages

- **`klaviyo-pp-cli flow-messages <id>`** - Get a flow message from a flow with the given flow message ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`flows:read`

- **`klaviyo-pp-cli flow-messages template get-for-flow-message <id>`** - Read the template related to a flow message.
- **`klaviyo-pp-cli flow-messages flow-action get-action-for-flow-message <id>`** - Read the flow action related to a flow message.

### flow-series-reports

Manage flow series reports

- **`klaviyo-pp-cli flow-series-reports`** - Returns the requested flow analytics series data<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `2/m`<br>Daily: `225/d`

**Scopes:**
`flows:read`

### flow-values-reports

Manage flow values reports

- **`klaviyo-pp-cli flow-values-reports`** - Returns the requested flow analytics values data<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `2/m`<br>Daily: `225/d`

**Scopes:**
`flows:read`

### flows

flows

- **`klaviyo-pp-cli flows create`** - Create a new flow using an encoded flow definition.

New objects within the flow definition, such as actions, will need to use a
`temporary_id` field for identification. These will be replaced with traditional `id` fields
after successful creation.

A successful request will return the new definition to you.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`<br>Daily: `100/d`

**Scopes:**
`flows:write`
- **`klaviyo-pp-cli flows delete`** - Delete a flow with the given flow ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`flows:write`
- **`klaviyo-pp-cli flows get`** - Get all flows in an account.

Returns a maximum of 50 flows per request, which can be paginated with cursor-based pagination.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`flows:read`
- **`klaviyo-pp-cli flows get-id`** - Get a flow with the given flow ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`flows:read`
- **`klaviyo-pp-cli flows update`** - Update the status of a flow with the given flow ID, and all actions in that flow.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`flows:write`

### form-series-reports

Manage form series reports

- **`klaviyo-pp-cli form-series-reports`** - Returns the requested form analytics series data.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `2/m`<br>Daily: `225/d`

**Scopes:**
`forms:read`

### form-values-reports

Manage form values reports

- **`klaviyo-pp-cli form-values-reports`** - Returns the requested form analytics values data.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `2/m`<br>Daily: `225/d`

**Scopes:**
`forms:read`

### form-versions

Manage form versions

- **`klaviyo-pp-cli form-versions <id>`** - Get the form version with the given ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`forms:read`

- **`klaviyo-pp-cli form-versions form get-for-version <id>`** - Read the form associated with a form version.
- **`klaviyo-pp-cli form-versions relationships get-form-id-for-form-version <id>`** - Read only the associated form ID.

### forms

forms

- **`klaviyo-pp-cli forms create`** - Create a new form.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`forms:write`
- **`klaviyo-pp-cli forms delete`** - Delete a given form.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`forms:write`
- **`klaviyo-pp-cli forms get`** - Get all forms in an account.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`forms:read`
- **`klaviyo-pp-cli forms get-id`** - Get the form with the given ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`forms:read`

### image-upload

Manage image upload

- **`klaviyo-pp-cli image-upload`** - Upload an image from a file.

If you want to import an image from an existing url or a data uri, use the Upload Image From URL endpoint instead.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `100/m`<br>Daily: `100/d`

**Scopes:**
`images:write`

### images

images

- **`klaviyo-pp-cli images get`** - Get all images in an account.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`images:read`
- **`klaviyo-pp-cli images get-id`** - Get the image with the given image ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`images:read`
- **`klaviyo-pp-cli images update`** - Update the image with the given image ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`images:write`
- **`klaviyo-pp-cli images upload-from-url`** - Import an image from a url or data uri.

If you want to upload an image from a file, use the Upload Image From File endpoint instead.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `100/m`<br>Daily: `100/d`

**Scopes:**
`images:write`

### lists

lists

- **`klaviyo-pp-cli lists create`** - Create a new list.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`<br>Daily: `150/d`

**Scopes:**
`lists:write`
- **`klaviyo-pp-cli lists delete`** - Delete a list with the given list ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`lists:write`
- **`klaviyo-pp-cli lists get`** - Get all lists in an account.

Filter to request a subset of all lists. Lists can be filtered by `id`, `name`, `created`, and `updated` fields.

Returns a maximum of 10 results per page.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`lists:read`
- **`klaviyo-pp-cli lists get-id`** - Get a list with the given list ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`<br><br>Rate limits when using the `additional-fields[list]=profile_count` parameter in your API request:<br>Burst: `1/s`<br>Steady: `15/m`<br><br>To learn more about how the `additional-fields` parameter impacts rate limits, check out our [Rate limits, status codes, and errors](https://developers.klaviyo.com/en/v2026-10-15/docs/rate_limits_and_error_handling) guide.

**Scopes:**
`lists:read`
- **`klaviyo-pp-cli lists update`** - Update the name of a list with the given list ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`lists:write`

### mapped-metrics

Manage mapped metrics

- **`klaviyo-pp-cli mapped-metrics get`** - Get all mapped metrics in an account.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`metrics:read`
- **`klaviyo-pp-cli mapped-metrics get-mappedmetrics`** - Get the mapped metric with the given ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`metrics:read`
- **`klaviyo-pp-cli mapped-metrics update`** - Update the mapped metric with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`<br>Daily: `30/d`

**Scopes:**
`metrics:write`

### metric-aggregates

Manage metric aggregates

- **`klaviyo-pp-cli metric-aggregates`** - Query and aggregate event data associated with a metric, including native Klaviyo metrics, integration-specific metrics, and custom events (not to be confused with [custom metrics](https://developers.klaviyo.com/en/reference/custom_metrics_api_overview), which are not supported at this time). Queries must be passed in the JSON body of your `POST` request.

To request campaign and flow performance data that matches the data shown in Klaviyo's UI, we recommend the [Reporting API](https://developers.klaviyo.com/en/reference/reporting_api_overview).

Results can be filtered and grouped by time, event, or profile dimensions.

To learn more about how to use this endpoint, check out our new [Using the Query Metric Aggregates Endpoint guide](https://developers.klaviyo.com/en/docs/using-the-query-metric-aggregates-endpoint).

For a comprehensive list of request body parameters, native Klaviyo metrics, and their associated attributes for grouping and filtering, please refer to the [metrics attributes guide](https://developers.klaviyo.com/en/docs/supported_metrics_and_attributes).<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`metrics:read`

### metric-properties

Manage metric properties

- **`klaviyo-pp-cli metric-properties <id>`** - Get a metric property with the given metric property ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`metrics:read`

- **`klaviyo-pp-cli metric-properties metric get-for-property <id>`** - Read the metric associated with a metric property.
- **`klaviyo-pp-cli metric-properties relationships get-metric-id-for-metric-property <id>`** - Read only the associated metric ID.

### metrics

metrics

- **`klaviyo-pp-cli metrics get`** - Get all metrics in an account.

Requests can be filtered by the following fields:
integration `name`, integration `category`

Returns a maximum of 200 results per page.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`metrics:read`
- **`klaviyo-pp-cli metrics get-id`** - Get a metric with the given metric ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`metrics:read`

### profile-bulk-import-jobs

Manage profile bulk import jobs

- **`klaviyo-pp-cli profile-bulk-import-jobs bulk-import-profiles`** - Create a bulk profile import job to create or update a batch of profiles.

Accepts up to 10,000 profiles per request. The maximum allowed payload size is 5MB. The maximum allowed payload size per-profile is 100KB.

To learn more, see our [Bulk Profile Import API guide](https://developers.klaviyo.com/en/docs/use_klaviyos_bulk_profile_import_api).<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`lists:write`
`profiles:write`
- **`klaviyo-pp-cli profile-bulk-import-jobs get-bulk-import-profiles-job`** - Get a bulk profile import job with the given job ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`lists:read`
`profiles:read`
- **`klaviyo-pp-cli profile-bulk-import-jobs get-bulk-import-profiles-jobs`** - Get all bulk profile import jobs.

Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`lists:read`
`profiles:read`

### profile-import

Manage profile import

- **`klaviyo-pp-cli profile-import`** - Given a set of profile attributes and optionally an ID, create or update a profile.

Returns 201 if a new profile was created, 200 if an existing profile was updated.

Use the `additional-fields` parameter to include subscriptions and predictive analytics data in your response.

Note that setting a field to `null` will clear out the field, whereas not including a field in your request will leave it unchanged.

The maximum allowed payload size is 100KB.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`profiles:write`

### profile-merge

Manage profile merge

- **`klaviyo-pp-cli profile-merge`** - Merge a given related profile into a profile with the given profile ID.

The profile provided under `relationships` (the "source" profile) will be merged into the profile provided by the ID in the base data object (the "destination" profile).
This endpoint queues an asynchronous task which will merge data from the source profile into the destination profile, deleting the source profile in the process. This endpoint accepts only one source profile.

To learn more about how profile data is preserved or overwritten during a merge, please [visit our Help Center](https://help.klaviyo.com/hc/en-us/articles/115005073847#merge-2-profiles3).<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`profiles:write`

### profile-subscription-bulk-create-jobs

Manage profile subscription bulk create jobs

- **`klaviyo-pp-cli profile-subscription-bulk-create-jobs`** - Subscribe one or more profiles to email marketing, SMS marketing, WhatsApp, or push. If the provided list has double opt-in enabled, profiles will receive a message requiring their confirmation before subscribing. Otherwise, profiles will be immediately subscribed without receiving a confirmation message.
Learn more about [consent in this guide](https://developers.klaviyo.com/en/docs/collect_email_and_sms_consent_via_api).

If a list is not provided, the opt-in process used will be determined by the [account-level default opt-in setting](https://www.klaviyo.com/settings/account/api-keys).

To add someone to a list without changing their subscription status, use [Add Profile to List](https://developers.klaviyo.com/en/reference/create_list_relationships).

This API will remove any `UNSUBSCRIBE`, `SPAM_REPORT` or `USER_SUPPRESSED` suppressions from the provided profiles. Learn more about [suppressed profiles](https://help.klaviyo.com/hc/en-us/articles/115005246108-Understanding-suppressed-email-profiles#what-is-a-suppressed-profile-1).

Maximum number of profiles can be submitted for subscription: 1000

This endpoint now supports a `historical_import` flag. If this flag is set `true`, profiles being subscribed will bypass double opt-in emails and be subscribed immediately. They will also bypass any associated "Added to list" flows. This is useful for importing historical data where you have already collected consent. If `historical_import` is set to true, the `consented_at` field is required and must be in the past.

Push tokens provided in `push_tokens` will be registered for each profile as long as push subscriptions are consented to.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`lists:write`
`profiles:write`
`subscriptions:write`

### profile-subscription-bulk-delete-jobs

Manage profile subscription bulk delete jobs

- **`klaviyo-pp-cli profile-subscription-bulk-delete-jobs`** - > 🚧
>
> Profiles not in the specified list will be globally unsubscribed. Always verify profile list membership before calling this endpoint to avoid unintended global unsubscribes.

Unsubscribe one or more profiles from email marketing, SMS marketing, push marketing, or a combination. Learn more about [consent in this guide](https://developers.klaviyo.com/en/docs/collect_email_and_sms_consent_via_api).

Push tokens provided in `subscriptions.push.tokens` will be removed for the specified profiles.

To remove someone from a list without changing their subscription status, use [Remove Profiles from List](https://developers.klaviyo.com/en/reference/remove_profiles_from_list).

Maximum number of profiles per call: 100<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`lists:write`
`profiles:write`
`subscriptions:write`

### profile-suppression-bulk-create-jobs

Manage profile suppression bulk create jobs

- **`klaviyo-pp-cli profile-suppression-bulk-create-jobs bulk-suppress-profiles`** - Manually suppress profiles by email address or specify a segment/list ID to suppress all current members of a segment/list.

Suppressed profiles cannot receive email marketing, independent of their consent status. To learn more, see our guides on [email suppressions](https://help.klaviyo.com/hc/en-us/articles/115005246108#what-is-a-suppressed-profile-1) and [collecting consent](https://developers.klaviyo.com/en/docs/collect_email_and_sms_consent_via_api).

Email address per request limit: 100<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`profiles:write`
`subscriptions:write`
- **`klaviyo-pp-cli profile-suppression-bulk-create-jobs get-bulk-suppress-profiles-job`** - Get the bulk suppress profiles job with the given job ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`subscriptions:read`
- **`klaviyo-pp-cli profile-suppression-bulk-create-jobs get-bulk-suppress-profiles-jobs`** - Get the status of all bulk profile suppression jobs.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`subscriptions:read`

### profile-suppression-bulk-delete-jobs

Manage profile suppression bulk delete jobs

- **`klaviyo-pp-cli profile-suppression-bulk-delete-jobs bulk-unsuppress-profiles`** - Manually unsuppress profiles by email address or specify a segment/list ID to unsuppress all current members of a segment/list.

This only removes suppressions with reason USER_SUPPRESSED ; unsubscribed profiles and suppressed profiles with reason INVALID_EMAIL or HARD_BOUNCE remain unchanged. To learn more, see our guides on [email suppressions](https://help.klaviyo.com/hc/en-us/articles/115005246108#what-is-a-suppressed-profile-1) and [collecting consent](https://developers.klaviyo.com/en/docs/collect_email_and_sms_consent_via_api).

Email address per request limit: 100<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`subscriptions:write`
- **`klaviyo-pp-cli profile-suppression-bulk-delete-jobs get-bulk-unsuppress-profiles-job`** - Get the bulk unsuppress profiles job with the given job ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`subscriptions:read`
- **`klaviyo-pp-cli profile-suppression-bulk-delete-jobs get-bulk-unsuppress-profiles-jobs`** - Get all bulk unsuppress profiles jobs.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`subscriptions:read`

### profiles

profiles

- **`klaviyo-pp-cli profiles create`** - Create a new profile.

Use the `additional-fields` parameter to include subscriptions and predictive analytics data in your response.

The maximum allowed payload size is 100KB.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`profiles:write`
- **`klaviyo-pp-cli profiles get`** - Get all profiles in an account.

Profiles can be sorted by the following fields in ascending and descending order: `id`, `created`, `updated`, `email`, `subscriptions.email.marketing.suppression.timestamp`, `subscriptions.email.marketing.list_suppressions.timestamp`

Use the `additional-fields` parameter to include subscriptions and predictive analytics data in your response.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`<br><br>Rate limits when using the `additional-fields[profile]=predictive_analytics` parameter in your API request:<br>Burst: `10/s`<br>Steady: `150/m`<br><br>To learn more about how the `additional-fields` parameter impacts rate limits, check out our [Rate limits, status codes, and errors](https://developers.klaviyo.com/en/v2026-10-15/docs/rate_limits_and_error_handling) guide.

**Scopes:**
`profiles:read`
- **`klaviyo-pp-cli profiles get-id`** - Get the profile with the given profile ID.

Use the `additional-fields` parameter to include subscriptions and predictive analytics data in your response.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`<br><br>Rate limits when using the `include=list` parameter in your API request:<br>Burst: `1/s`<br>Steady: `15/m`<br><br>Rate limits when using the `include=segment` parameter in your API request:<br>Burst: `1/s`<br>Steady: `15/m`<br><br>To learn more about how the `include` parameter impacts rate limits, check out our [Rate limits, status codes, and errors](https://developers.klaviyo.com/en/v2026-10-15/docs/rate_limits_and_error_handling) guide.

**Scopes:**
`profiles:read`
- **`klaviyo-pp-cli profiles update`** - Update the profile with the given profile ID.

Use the `additional-fields` parameter to include subscriptions and predictive analytics data in your response.

Note that setting a field to `null` will clear out the field, whereas not including a field in your request will leave it unchanged.

The maximum allowed payload size is 100KB.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`profiles:write`

### push-tokens

Manage push tokens

- **`klaviyo-pp-cli push-tokens create`** - Create or update a push token.

This endpoint can be used to migrate push tokens from another platform to Klaviyo. Please use our mobile SDKs ([iOS](https://github.com/klaviyo/klaviyo-swift-sdk) and [Android](https://github.com/klaviyo/klaviyo-android-sdk)) to create push tokens from users' devices.

The maximum allowed payload size is 100KB.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`profiles:write`
`push-tokens:write`
- **`klaviyo-pp-cli push-tokens delete`** - Delete a specific push token based on its ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`push-tokens:write`
- **`klaviyo-pp-cli push-tokens get`** - Return push tokens associated with company.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`profiles:read`
`push-tokens:read`
- **`klaviyo-pp-cli push-tokens get-pushtokens`** - Return a specific push token based on its ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`profiles:read`
`push-tokens:read`

### reviews

reviews

- **`klaviyo-pp-cli reviews get`** - Get all reviews.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`reviews:read`
- **`klaviyo-pp-cli reviews get-id`** - Get the review with the given ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`reviews:read`
- **`klaviyo-pp-cli reviews update`** - Update a review.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`reviews:write`

### segment-series-reports

Manage segment series reports

- **`klaviyo-pp-cli segment-series-reports`** - Returns the requested segment analytics series data.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `2/m`<br>Daily: `225/d`

**Scopes:**
`segments:read`

### segment-values-reports

Manage segment values reports

- **`klaviyo-pp-cli segment-values-reports`** - Returns the requested segment analytics values data.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `2/m`<br>Daily: `225/d`

**Scopes:**
`segments:read`

### segments

segments

- **`klaviyo-pp-cli segments create`** - Create a segment.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`<br>Daily: `100/d`

**Scopes:**
`segments:write`
- **`klaviyo-pp-cli segments delete`** - Delete a segment with the given segment ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`segments:write`
- **`klaviyo-pp-cli segments get`** - Get all segments in an account.

Filter to request a subset of all segments. Segments can be filtered by `name`, `created`, and `updated` fields.

Add `--with-counts` to enrich each returned segment with `profile_count`. Because Klaviyo rate-limits that additional field to 1 request/second burst and 15 requests/minute steady, the CLI spaces per-item requests and honors `Retry-After`. Add `--all` separately to fetch and enrich every page.

Returns a maximum of 10 results per page.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`segments:read`
- **`klaviyo-pp-cli segments get-id`** - Get a segment with the given segment ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`<br><br>Rate limits when using the `additional-fields[segment]=profile_count` parameter in your API request:<br>Burst: `1/s`<br>Steady: `15/m`<br><br>To learn more about how the `additional-fields` parameter impacts rate limits, check out our [Rate limits, status codes, and errors](https://developers.klaviyo.com/en/v2026-10-15/docs/rate_limits_and_error_handling) guide.

**Scopes:**
`segments:read`
- **`klaviyo-pp-cli segments update`** - Update a segment with the given segment ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`<br>Daily: `100/d`

**Scopes:**
`segments:write`

### tag-groups

Manage tag groups

- **`klaviyo-pp-cli tag-groups create`** - Create a tag group. An account cannot have more than **50** unique tag groups.

If `exclusive` is not specified `true` or `false`, the tag group defaults to non-exclusive.

If a tag group is non-exclusive, any given related resource (campaign, flow, etc.)
can be linked to multiple tags from that tag group.
If a tag group is exclusive, any given related resource can only be linked to one tag from that tag group.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`tags:read`
`tags:write`
- **`klaviyo-pp-cli tag-groups delete`** - Delete the tag group with the given tag group ID.

Any tags inside that tag group, and any associations between those tags and other resources, will also be removed. The default tag group cannot be deleted.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`tags:read`
`tags:write`
- **`klaviyo-pp-cli tag-groups get`** - List all tag groups in an account. Every account has one default tag group.

Tag groups can be filtered by `name`, `exclusive`, and `default`, and sorted by `name` or `id` in ascending or descending order.

Returns a maximum of 25 tag groups per request, which can be paginated with
[cursor-based pagination](https://developers.klaviyo.com/en/v2022-10-17/reference/api_overview#pagination).<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`tags:read`
- **`klaviyo-pp-cli tag-groups get-taggroups`** - Retrieve the tag group with the given tag group ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`tags:read`
- **`klaviyo-pp-cli tag-groups update`** - Update the tag group with the given tag group ID.

Only a tag group's `name` can be changed. A tag group's `exclusive` or `default` value cannot be changed.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`tags:read`
`tags:write`

### tags

tags

- **`klaviyo-pp-cli tags create`** - Create a tag. An account cannot have more than **500** unique tags.

A tag belongs to a single tag group. If `relationships.tag-group.data.id` is not specified,
the tag is added to the account's default tag group.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`tags:read`
`tags:write`
- **`klaviyo-pp-cli tags delete`** - Delete the tag with the given tag ID. Any associations between the tag and other resources will also be removed.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`tags:read`
`tags:write`
- **`klaviyo-pp-cli tags get`** - List all tags in an account.

Tags can be filtered by `name`, and sorted by `name` or `id` in ascending or descending order.

Returns a maximum of 50 tags per request, which can be paginated with
[cursor-based pagination](https://developers.klaviyo.com/en/v2022-10-17/reference/api_overview#pagination).<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`tags:read`
- **`klaviyo-pp-cli tags get-id`** - Retrieve the tag with the given tag ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`tags:read`
- **`klaviyo-pp-cli tags update`** - Update the tag with the given tag ID.

Only a tag's `name` can be changed. A tag cannot be moved from one tag group to another.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`tags:read`
`tags:write`

### template-clone

Manage template clone

- **`klaviyo-pp-cli template-clone`** - Create a clone of a template with the given template ID.

If there are 1,000 or more templates in an account, cloning will fail as there is a limit of 1,000 templates
that can be created via the API.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`templates:write`

### template-render

Manage template render

- **`klaviyo-pp-cli template-render`** - Render a template with the given template ID and context attribute. Returns the AMP, HTML, and plain text versions of the email template.

**Request body parameters** (nested under `attributes`):

* `return_fields`: Request specific fields using [sparse fieldsets](https://developers.klaviyo.com/en/reference/api_overview#sparse-fieldsets).

* `context`: This is the context your email template will be rendered with. You must pass in a `context` object as a JSON object.

Email templates are rendered with contexts in a similar manner to Django templates. Nested template variables can be referenced via dot notation. Template variables without corresponding `context` values are treated as `FALSE` and output nothing.

Ex. `{ "name" : "George Washington", "state" : "VA" }`<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`

**Scopes:**
`templates:read`

### template-universal-content

Manage template universal content

- **`klaviyo-pp-cli template-universal-content create-universal-content`** - Create universal content. Currently supported block types are: `button`, `drop_shadow`, `horizontal_rule`, `html`, `image`, `spacer`, and `text`.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`templates:write`
- **`klaviyo-pp-cli template-universal-content delete-universal-content`** - Delete the universal content with the given ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`templates:write`
- **`klaviyo-pp-cli template-universal-content get-all-universal-content`** - Get all universal content in an account.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`templates:read`
- **`klaviyo-pp-cli template-universal-content get-universal-content`** - Get the universal content with the given ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`templates:read`
- **`klaviyo-pp-cli template-universal-content update-universal-content`** - Update universal content. The `definition` field can only be updated on the following block types at this time: `button`, `drop_shadow`, `horizontal_rule`, `html`, `image`, `spacer`, and `text`.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`templates:write`

### templates

templates

- **`klaviyo-pp-cli templates create`** - Create a new HTML or drag-and-drop template.

If there are 1,000 or more templates in an account, creation will fail as there is a limit of 1,000 templates
that can be created via the API.

Request specific fields using [sparse fieldsets](https://developers.klaviyo.com/en/reference/api_overview#sparse-fieldsets).<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`templates:write`
- **`klaviyo-pp-cli templates delete`** - Delete a template with the given template ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`templates:write`
- **`klaviyo-pp-cli templates get`** - Get all templates in an account.

Use `additional-fields[template]=definition` to include the full template
definition for SYSTEM_DRAGGABLE templates.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`templates:read`
- **`klaviyo-pp-cli templates get-id`** - Get a template with the given template ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`templates:read`
- **`klaviyo-pp-cli templates update`** - Update a template with the given template ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`

**Scopes:**
`templates:write`

### tracking-settings

tracking settings

- **`klaviyo-pp-cli tracking-settings get`** - Get all UTM tracking settings in an account. Returns an array with a single tracking setting.

More information about UTM tracking settings can be found [here](https://help.klaviyo.com/hc/en-us/articles/115005247808).<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`tracking-settings:read`
- **`klaviyo-pp-cli tracking-settings get-trackingsettings`** - Get the UTM tracking setting with the given account ID.

More information about UTM tracking settings can be found [here](https://help.klaviyo.com/hc/en-us/articles/115005247808).<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`tracking-settings:read`
- **`klaviyo-pp-cli tracking-settings update`** - Update the UTM tracking setting with the given account ID.

More information about UTM tracking settings can be found [here](https://help.klaviyo.com/hc/en-us/articles/115005247808).<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`

**Scopes:**
`tracking-settings:write`

### web-feeds

web feeds

- **`klaviyo-pp-cli web-feeds create`** - Create a web feed.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`web-feeds:write`
- **`klaviyo-pp-cli web-feeds delete`** - Delete the web feed with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`web-feeds:write`
- **`klaviyo-pp-cli web-feeds get`** - Get all web feeds for an account.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`web-feeds:read`
- **`klaviyo-pp-cli web-feeds get-webfeeds`** - Get the web feed with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`web-feeds:read`
- **`klaviyo-pp-cli web-feeds update`** - Update the web feed with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`web-feeds:write`

### webhook-topics

Manage webhook topics

- **`klaviyo-pp-cli webhook-topics get`** - Get all webhook topics in a Klaviyo account.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`webhooks:read`
- **`klaviyo-pp-cli webhook-topics get-webhooktopics`** - Get the webhook topic with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`webhooks:read`

### webhooks

webhooks

- **`klaviyo-pp-cli webhooks create`** - Create a new Webhook<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`webhooks:write`
- **`klaviyo-pp-cli webhooks delete`** - Delete a webhook with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`webhooks:write`
- **`klaviyo-pp-cli webhooks get`** - Get all webhooks in an account.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`webhooks:read`
- **`klaviyo-pp-cli webhooks get-id`** - Get the webhook with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`webhooks:read`
- **`klaviyo-pp-cli webhooks update`** - Update the webhook with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`

**Scopes:**
`webhooks:write`

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
klaviyo-pp-cli accounts get

# JSON for scripting and agents
klaviyo-pp-cli accounts get --json

# Filter to specific fields
klaviyo-pp-cli accounts get --json --select id,name,status

# Dry run - show the request without sending
klaviyo-pp-cli accounts get --dry-run

# Agent mode - JSON + compact + no prompts in one flag
klaviyo-pp-cli accounts get --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
klaviyo-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Shared client profiles live under the XDG config root. The only supported MCP
selector is `PRINTING_PRESS_CLIENT_PROFILE`; credential bytes resolve in memory
from the profile's exact 1Password reference or named environment variable.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `klaviyo-pp-cli client validate <profile>` and then
  `klaviyo-pp-cli --client-profile <profile> doctor --no-cache`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 or 403 from every endpoint** - Confirm the profile points to the exact
  private-key field and that the key can read the account identity endpoint.
- **Empty local search results** - Run sync for the relevant resources first, such as sync metrics, sync profiles, or sync events.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Klaviyo hosted MCP**](https://developers.klaviyo.com/) - mcp
- [**klaviyo-api-python**](https://github.com/klaviyo/klaviyo-api-python) - Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
