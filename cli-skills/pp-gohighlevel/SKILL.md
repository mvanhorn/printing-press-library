---
name: pp-gohighlevel
description: "Every HighLevel feature, plus offline state, multi-location rollups, SLA detection, and pipeline velocity that no... Trigger phrases: `show me unread across all my GoHighLevel locations`, `find stale opportunities in HighLevel`, `GHL pipeline velocity`, `bulk tag GoHighLevel contacts`, `GHL SLA breach`, `use gohighlevel`, `run gohighlevel`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - gohighlevel-pp-cli
    install:
      - kind: go
        bins: [gohighlevel-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/cmd/gohighlevel-pp-cli
---

# HighLevel — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `gohighlevel-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install gohighlevel --cli-only
   ```
2. Verify: `gohighlevel-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/cmd/gohighlevel-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

gohighlevel-pp-cli mirrors all 409 HighLevel REST endpoints as typed Cobra commands and MCP tools, then layers a local SQLite store on top so cross-location triage (`roster`, `unread`, `sla-breach`), pipeline analytics (`velocity`, `stale-opps`), and bulk operations with `--dry-run` (`bulk-tag`, `dedup`, `reconcile`) become one-line shell commands instead of 50 tab-opens.

## When to Use This CLI

Use this CLI when you need real cross-location analytics, scriptable bulk ops, or agent-native triage against HighLevel. The MCP servers in the ecosystem are excellent for per-call lookups inside Claude Desktop. This CLI is for the Monday-standup roster, the Tuesday pipeline review, the migration reconcile, and any agent that needs 'what changed since I last looked.'

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-location aggregation
- **`roster`** — One table summarizing operational health metrics (new leads, unread conversations, unpaid invoices, stale opportunities) across every sub-account in one row each.

  _When a user asks 'how are all my locations doing this morning', this is the one command that answers it without 50 round-trips._

  ```bash
  gohighlevel-pp-cli roster --metric leads-24h,unread,unpaid-invoices,stale-opps --json
  ```
- **`unread`** — List inbound conversations that have no outbound reply, optionally filtered by location, recency, and assignee.

  _Answers 'what conversation actually needs me right now' without paging through 10 inbox tabs._

  ```bash
  gohighlevel-pp-cli unread --location all --since 1h --assigned-to me --json
  ```

### Pipeline analytics
- **`stale-opps`** — Find pipeline opportunities with no stage change AND no message/note activity in N days, grouped by stage.

  _Replaces the manual Tuesday-morning kanban hunt for 'who's stuck and forgotten'._

  ```bash
  gohighlevel-pp-cli stale-opps --pipeline 'Sales 2026' --threshold 14 --no-activity --json
  ```
- **`velocity`** — Mean/p50/p90 time-in-stage per pipeline stage, plus count entered/exited and conversion percentage.

  _Replaces a $300/mo Airtable+Zapier mirror with a one-line shell command._

  ```bash
  gohighlevel-pp-cli velocity --pipeline 'Sales 2026' --json
  ```

### Triage and SLA
- **`sla-breach`** — Conversations across all locations where the last inbound message has no outbound reply within a threshold, optionally restricted to business hours.

  _Catches SLA breaches in real time instead of after the client complains._

  ```bash
  gohighlevel-pp-cli sla-breach --threshold 30m --business-hours --location all --json
  ```

### Bulk operations
- **`contacts dedup`** — Find duplicate contacts by phone or email, output merge candidates, optionally apply via the contacts merge endpoint.

  _Cleans up the post-migration mess that plagues every reseller migration._

  ```bash
  gohighlevel-pp-cli contacts dedup --by phone,email --json
  ```
- **`contacts bulk-tag`** — Apply or remove a tag across the results of a contact search, with dry-run preview and rate-limit-aware progress.

  _One-line 'retag this campaign cohort' instead of 500 manual UI clicks._

  ```bash
  gohighlevel-pp-cli contacts bulk-tag --from-search 'campaign:spring-2026' tested --json
  ```
- **`contacts reconcile`** — Diff a source CSV against the synced local contact set on a key column, output created/updated/missing/extra rows.

  _Closes the loop on every reseller migration so nothing falls through the cracks._

  ```bash
  gohighlevel-pp-cli contacts reconcile --source migration-source.csv --key email --json
  ```

### Agent-native plumbing
- **`agent-context`** — Structured JSON description of every command, flag, and auth env var so agents can introspect the live CLI without parsing --help or reading source.

  _Lets an agent discover what actions are available without scraping help text — the schema_version field signals breaking shape changes._

  ```bash
  gohighlevel-pp-cli agent-context --pretty
  ```

## Command Reference

**ad-publishing** — Manage ad publishing

- `gohighlevel-pp-cli ad-publishing fb-add-custom-audience-member` — Add a member to a Facebook custom audience
- `gohighlevel-pp-cli ad-publishing fb-batch-update-audience-members` — Add or remove members in bulk from a Facebook custom audience via CSV or smart lists
- `gohighlevel-pp-cli ad-publishing fb-create-conversation-form` — Create a new Facebook conversation lead form
- `gohighlevel-pp-cli ad-publishing fb-create-integration` — Create a Facebook ad integration for a location with page and ad account
- `gohighlevel-pp-cli ad-publishing fb-create-page-lead-form` — Create a new lead gen form on a Facebook page
- `gohighlevel-pp-cli ad-publishing fb-delete-ad` — Delete a Facebook ad by ID
- `gohighlevel-pp-cli ad-publishing fb-delete-ad-account` — Remove a Facebook ad account connection from a location
- `gohighlevel-pp-cli ad-publishing fb-delete-adset` — Delete a Facebook ad set by ID
- `gohighlevel-pp-cli ad-publishing fb-delete-campaign` — Delete a Facebook campaign by ID
- `gohighlevel-pp-cli ad-publishing fb-delete-custom-audience` — Delete a Facebook custom audience by ID
- `gohighlevel-pp-cli ad-publishing fb-delete-integration` — Remove the Facebook ad integration from a location
- `gohighlevel-pp-cli ad-publishing fb-delete-page` — Remove a Facebook page connection from a location
- `gohighlevel-pp-cli ad-publishing fb-duplicate-ad` — Duplicate an existing Facebook ad
- `gohighlevel-pp-cli ad-publishing fb-duplicate-adset` — Duplicate an existing Facebook ad set
- `gohighlevel-pp-cli ad-publishing fb-duplicate-campaign` — Duplicate an existing Facebook campaign
- `gohighlevel-pp-cli ad-publishing fb-get-ad-account` — Retrieve details of a specific Facebook ad account
- `gohighlevel-pp-cli ad-publishing fb-get-ad-accounts` — Retrieve Facebook ad accounts available for the connected user
- `gohighlevel-pp-cli ad-publishing fb-get-campaign` — Retrieve a Facebook campaign with its linked adsets and ads
- `gohighlevel-pp-cli ad-publishing fb-get-campaign-reporting` — Retrieve reporting metrics for a specific Facebook campaign
- `gohighlevel-pp-cli ad-publishing fb-get-conversation-forms` — Retrieve Facebook conversation lead forms for a location
- `gohighlevel-pp-cli ad-publishing fb-get-current-user` — Retrieve the authenticated Facebook user profile for a location
- `gohighlevel-pp-cli ad-publishing fb-get-custom-audience-by-id` — Retrieve a specific Facebook custom audience by its ID
- `gohighlevel-pp-cli ad-publishing fb-get-custom-audiences` — Retrieve Facebook custom audiences for a location
- `gohighlevel-pp-cli ad-publishing fb-get-entity` — Retrieve Facebook campaigns, adsets, or ads based on entity type
- `gohighlevel-pp-cli ad-publishing fb-get-instagram-accounts` — Retrieve Instagram accounts linked to a specific Facebook page
- `gohighlevel-pp-cli ad-publishing fb-get-integration` — Retrieve the Facebook ad integration details for a location
- `gohighlevel-pp-cli ad-publishing fb-get-lead-form` — Retrieve a specific Facebook lead form by its ID
- `gohighlevel-pp-cli ad-publishing fb-get-page-lead-forms` — Retrieve lead gen forms for a specific Facebook page
- `gohighlevel-pp-cli ad-publishing fb-get-pages` — Retrieve Facebook pages associated with the connected account
- `gohighlevel-pp-cli ad-publishing fb-get-pixels` — Retrieve Facebook conversion pixels for a location
- `gohighlevel-pp-cli ad-publishing fb-get-reporting` — Retrieve aggregated Facebook ad reporting metrics for a location
- `gohighlevel-pp-cli ad-publishing fb-get-reporting-list` — Retrieve a list of Facebook campaigns, adsets, or ads with reporting data
- `gohighlevel-pp-cli ad-publishing fb-pause-ad` — Pause a running Facebook ad
- `gohighlevel-pp-cli ad-publishing fb-pause-adset` — Pause a running Facebook ad set
- `gohighlevel-pp-cli ad-publishing fb-pause-campaign` — Pause a running Facebook campaign
- `gohighlevel-pp-cli ad-publishing fb-publish-campaign` — Publish a Facebook campaign and push it live to Facebook
- `gohighlevel-pp-cli ad-publishing fb-remove-custom-audience-member` — Remove a member from a Facebook custom audience
- `gohighlevel-pp-cli ad-publishing fb-resume-ad` — Resume a paused Facebook ad
- `gohighlevel-pp-cli ad-publishing fb-resume-adset` — Resume a paused Facebook ad set
- `gohighlevel-pp-cli ad-publishing fb-resume-campaign` — Resume a paused Facebook campaign
- `gohighlevel-pp-cli ad-publishing fb-search-targeting` — Search Facebook geo-locations and interests for ad targeting
- `gohighlevel-pp-cli ad-publishing fb-set-default-page` — Set the default Facebook page for a location
- `gohighlevel-pp-cli ad-publishing fb-update-custom-audience` — Update name or description of a Facebook custom audience
- `gohighlevel-pp-cli ad-publishing fb-upsert-ad` — Create or update a Facebook ad (v2)
- `gohighlevel-pp-cli ad-publishing fb-upsert-adset` — Create or update a Facebook ad set
- `gohighlevel-pp-cli ad-publishing fb-upsert-campaign` — Create or update a Facebook campaign
- `gohighlevel-pp-cli ad-publishing fb-upsert-pixel` — Create or update a Facebook conversion pixel configuration
- `gohighlevel-pp-cli ad-publishing google-create-integration` — Create a Google Ads integration for a location
- `gohighlevel-pp-cli ad-publishing google-create-offline-user-list-job` — Create a job to upload users to a Google customer match list
- `gohighlevel-pp-cli ad-publishing google-delete-ad-account` — Remove a Google Ads account connection from a location
- `gohighlevel-pp-cli ad-publishing google-delete-conversion` — Delete a Google Ads conversion action by ID
- `gohighlevel-pp-cli ad-publishing google-delete-segment` — Delete a Google Ads audience segment by ID
- `gohighlevel-pp-cli ad-publishing google-get-ad-account-details` — Retrieve details of a specific Google Ads account
- `gohighlevel-pp-cli ad-publishing google-get-ad-accounts` — Retrieve Google Ads accounts available for the connected user
- `gohighlevel-pp-cli ad-publishing google-get-assets` — Retrieve Google Ads creative assets for a location
- `gohighlevel-pp-cli ad-publishing google-get-audience-by-id` — Retrieve a specific Google Ads combined audience by ID
- `gohighlevel-pp-cli ad-publishing google-get-audiences` — Retrieve Google Ads combined audiences for a location
- `gohighlevel-pp-cli ad-publishing google-get-campaign-by-id` — Retrieve a specific Google Ads campaign by ID
- `gohighlevel-pp-cli ad-publishing google-get-campaign-reporting` — Retrieve reporting metrics for a specific Google campaign
- `gohighlevel-pp-cli ad-publishing google-get-conversion-by-id` — Retrieve a specific Google Ads conversion action by ID
- `gohighlevel-pp-cli ad-publishing google-get-conversion-goals` — Retrieve Google Ads conversion goals for a location
- `gohighlevel-pp-cli ad-publishing google-get-conversions` — Retrieve Google Ads conversion actions for a location
- `gohighlevel-pp-cli ad-publishing google-get-current-user` — Retrieve the authenticated Google user info for a location
- `gohighlevel-pp-cli ad-publishing google-get-entity` — Retrieve Google campaigns, ad groups, or ads based on entity type
- `gohighlevel-pp-cli ad-publishing google-get-integration` — Retrieve the Google Ads integration details for a location
- `gohighlevel-pp-cli ad-publishing google-get-keyword-ideas` — Retrieve keyword suggestions for Google Ads campaigns
- `gohighlevel-pp-cli ad-publishing google-get-reporting` — Retrieve aggregated Google Ads reporting metrics for a location
- `gohighlevel-pp-cli ad-publishing google-get-reporting-list` — Retrieve a list of Google campaigns or ad groups with reporting data
- `gohighlevel-pp-cli ad-publishing google-get-segment-by-id` — Retrieve a specific Google Ads audience segment by ID
- `gohighlevel-pp-cli ad-publishing google-get-segments` — Retrieve Google Ads audience segments for a location
- `gohighlevel-pp-cli ad-publishing google-get-target-interests` — Retrieve affinity and in-market audience options for Google Ads targeting
- `gohighlevel-pp-cli ad-publishing google-publish-ad` — Publish a Google ad and push it live
- `gohighlevel-pp-cli ad-publishing google-search-targeting` — Search Google geo-locations for ad targeting
- `gohighlevel-pp-cli ad-publishing google-upsert-assets` — Create or update Google Ads creative assets
- `gohighlevel-pp-cli ad-publishing google-upsert-audience` — Create or update a Google Ads combined audience
- `gohighlevel-pp-cli ad-publishing google-upsert-campaign` — Create or update a full Google Ads campaign structure
- `gohighlevel-pp-cli ad-publishing google-upsert-conversion` — Create or update a Google Ads conversion action
- `gohighlevel-pp-cli ad-publishing google-upsert-segment` — Create or update a Google Ads audience segment
- `gohighlevel-pp-cli ad-publishing li-create-integration` — Create a LinkedIn Ads integration for a location with ad account details
- `gohighlevel-pp-cli ad-publishing li-create-lead-form` — Create a new LinkedIn lead gen form for an ad account
- `gohighlevel-pp-cli ad-publishing li-delete-ad-account` — Remove a LinkedIn ad account connection from a location
- `gohighlevel-pp-cli ad-publishing li-get-ad-account-details` — Retrieve details of a specific LinkedIn ad account
- `gohighlevel-pp-cli ad-publishing li-get-ad-accounts` — Retrieve LinkedIn Ads accounts available for the connected user
- `gohighlevel-pp-cli ad-publishing li-get-ad-analytics` — Retrieve LinkedIn Ads analytics data with configurable pivot and time grouping
- `gohighlevel-pp-cli ad-publishing li-get-campaign-group` — Retrieve a LinkedIn ad campaign group by ID
- `gohighlevel-pp-cli ad-publishing li-get-campaign-group-reporting` — Retrieve reporting metrics for a specific LinkedIn campaign group
- `gohighlevel-pp-cli ad-publishing li-get-current-user` — Retrieve the authenticated LinkedIn user info for a location
- `gohighlevel-pp-cli ad-publishing li-get-integration` — Retrieve the LinkedIn Ads integration details for a location
- `gohighlevel-pp-cli ad-publishing li-get-lead-forms` — Retrieve LinkedIn lead gen forms for an ad account
- `gohighlevel-pp-cli ad-publishing li-get-reporting-list` — Retrieve a list of LinkedIn campaigns or campaign groups with reporting data
- `gohighlevel-pp-cli ad-publishing li-publish-campaign-group` — Publish a LinkedIn ad campaign group and push it live
- `gohighlevel-pp-cli ad-publishing li-search-targeting` — Search LinkedIn targeting facets such as locations, industries, and job titles
- `gohighlevel-pp-cli ad-publishing li-update-ad-status` — Pause or resume a LinkedIn ad, campaign, or ad group
- `gohighlevel-pp-cli ad-publishing li-upsert-campaign-group` — Create or update a LinkedIn ad campaign group with campaigns and ads

**affiliate-manager** — Documentation for Affiliate Manager API


**agent-studio** — Manage agent studio

- `gohighlevel-pp-cli agent-studio create-agent` — Creates a new agent with staging version. The agent will be created with an initial staging version that can later...
- `gohighlevel-pp-cli agent-studio delete-agent` — Deletes an agent and all its versions.
- `gohighlevel-pp-cli agent-studio execute-agent` — Executes the specified agent and returns a non-streaming JSON response with the complete agent output. The agent...
- `gohighlevel-pp-cli agent-studio execute-agent-deprecated` — **Deprecated endpoint - use POST /agent/:agentId/execute instead.** Executes the specified agent and returns a...
- `gohighlevel-pp-cli agent-studio get-agent-by-id` — Gets a specific agent by its ID for the specified location with all its versions. Returns complete agent metadata...
- `gohighlevel-pp-cli agent-studio get-agent-by-id-deprecated` — **Deprecated endpoint - use GET /agent/:agentId instead.** Gets a specific agent by its ID for the specified...
- `gohighlevel-pp-cli agent-studio get-agents` — Lists all active agents for the specified location. locationId is required parameter to ensure optimal performance....
- `gohighlevel-pp-cli agent-studio get-agents-deprecated` — **Deprecated endpoint - use GET /agent instead.** Lists all active agents that have a published production version...
- `gohighlevel-pp-cli agent-studio promote-and-publish` — Promotes a draft version to production.
- `gohighlevel-pp-cli agent-studio update-agent-metadata` — Updates agent metadata such as name, description, and status.
- `gohighlevel-pp-cli agent-studio update-agent-version` — Updates a specific agent version by versionId. Supports updating nodes, edges, variables, and configuration.

**associations** — Documentation for Associations API

- `gohighlevel-pp-cli associations create` — Allow you to create contact - contact , contact - custom objects associations, will add more in the...
- `gohighlevel-pp-cli associations create-relation` — Create Relation.Documentation Link - https://doc.clickup.com/8631005/d/h/87cpx-293776/cd0f4122abc04d3
- `gohighlevel-pp-cli associations delete` — Delete USER_DEFINED Association By Id, deleting an association will also all the relations for that association
- `gohighlevel-pp-cli associations delete-relation` — Delete Relation
- `gohighlevel-pp-cli associations find` — Get all associations for a sub-account / location
- `gohighlevel-pp-cli associations get-by-id` — Using this api you can get SYSTEM_DEFINED / USER_DEFINED association by id
- `gohighlevel-pp-cli associations get-by-object-keys` — Get association by object keys like contacts, custom objects and opportunities. Documentation Link -...
- `gohighlevel-pp-cli associations get-key-by-key-name` — Using this api you can get standard / user defined association by key
- `gohighlevel-pp-cli associations get-relations-by-record-id` — Get all relations by record Id
- `gohighlevel-pp-cli associations update` — Update Association , Allows you to update labels of an associations. Documentation Link -...

**blogs** — Documentation for Blogs

- `gohighlevel-pp-cli blogs check-url-slug-exists` — The 'Check url slug' API allows check the blog slug validation which is needed before publishing any blog post....
- `gohighlevel-pp-cli blogs create-post` — The 'Create Blog Post' API allows you create blog post for any given blog site. Please use blogs/post.write
- `gohighlevel-pp-cli blogs get` — The 'Get Blogs by Location ID' API allows you get blogs using Location ID.Please use blogs/list.readonly
- `gohighlevel-pp-cli blogs get-all-authors-by-location` — The 'Get all authors' Api return the blog authors for a given location ID. Please use 'blogs/author.readonly'
- `gohighlevel-pp-cli blogs get-all-categories-by-location` — The 'Get all categories' Api return the blog categoies for a given location ID. Please use 'blogs/category.readonly'
- `gohighlevel-pp-cli blogs get-post` — The 'Get Blog posts by Blog ID' API allows you get blog posts for any given blog site using blog ID.Please use...
- `gohighlevel-pp-cli blogs update-post` — The 'Update Blog Post' API allows you update blog post for any given blog site. Please use blogs/post-update.write

**brand-boards** — Documentation for Brand Boards API

- `gohighlevel-pp-cli brand-boards create` — Creates a new brand board with logos, colors, and fonts
- `gohighlevel-pp-cli brand-boards delete` — Deletes a Brand Board
- `gohighlevel-pp-cli brand-boards get-by-id` — Retrieves a specific Brand Board by its ID
- `gohighlevel-pp-cli brand-boards get-by-location` — Retrieves all Brand Boards for a specific location
- `gohighlevel-pp-cli brand-boards update` — Updates an existing Brand Board

**businesses** — Documentation for business API

- `gohighlevel-pp-cli businesses create-business` — Create Business
- `gohighlevel-pp-cli businesses delete-business` — Delete Business
- `gohighlevel-pp-cli businesses get-business` — Get Business
- `gohighlevel-pp-cli businesses get-by-location` — Get Businesses by Location
- `gohighlevel-pp-cli businesses update-business` — Update Business

**calendars** — Documentation for Calendars API

- `gohighlevel-pp-cli calendars add-to-schedule` — Associates a calendar with the given schedule by adding the calendarId to a schedule
- `gohighlevel-pp-cli calendars create` — Create calendar in a location.
- `gohighlevel-pp-cli calendars create-appointment` — Create appointment
- `gohighlevel-pp-cli calendars create-appointment-note` — Create Note
- `gohighlevel-pp-cli calendars create-block-slot` — Create block slot
- `gohighlevel-pp-cli calendars create-group` — Create Calendar Group
- `gohighlevel-pp-cli calendars create-resource` — Create calendar resource by resource type
- `gohighlevel-pp-cli calendars create-schedule` — Create new schedule with specified rules, timezone, location, user and calendar associations.
- `gohighlevel-pp-cli calendars delete` — Delete calendar by ID
- `gohighlevel-pp-cli calendars delete-appointment-note` — Delete Note
- `gohighlevel-pp-cli calendars delete-event` — Delete event by ID
- `gohighlevel-pp-cli calendars delete-group` — Delete Group
- `gohighlevel-pp-cli calendars delete-resource` — Delete calendar resource by ID
- `gohighlevel-pp-cli calendars delete-schedule` — Permanently remove a schedule and all its associated rules. This action cannot be undone.
- `gohighlevel-pp-cli calendars disable-group` — Disable Group
- `gohighlevel-pp-cli calendars edit-appointment` — Update appointment
- `gohighlevel-pp-cli calendars edit-block-slot` — Update block slot by ID
- `gohighlevel-pp-cli calendars edit-group` — Update Group by group ID
- `gohighlevel-pp-cli calendars fetch-resources` — List calendar resources by resource type and location ID
- `gohighlevel-pp-cli calendars get` — Get all calendars in a location.
- `gohighlevel-pp-cli calendars get-all-schedules` — Retrieve user availability schedules based on various filters including location, calendar, and user. Supports...
- `gohighlevel-pp-cli calendars get-appointment` — Get appointment by ID
- `gohighlevel-pp-cli calendars get-appointment-notes` — Get Appointment Notes
- `gohighlevel-pp-cli calendars get-blocked-slots` — Get Blocked Slots
- `gohighlevel-pp-cli calendars get-calendarid` — Get calendar by ID
- `gohighlevel-pp-cli calendars get-events` — Get Calendar Events
- `gohighlevel-pp-cli calendars get-groups` — Get all calendar groups in a location.
- `gohighlevel-pp-cli calendars get-resource` — Get calendar resource by ID
- `gohighlevel-pp-cli calendars get-schedule-by-id` — Retrieve a specific schedule by its unique identifier. Returns detailed information including rules, timezone, and...
- `gohighlevel-pp-cli calendars remove-from-schedule` — Removes the association between a team calendar and the given schedule by removing the calendarId from the schedule
- `gohighlevel-pp-cli calendars update` — Update calendar by ID.
- `gohighlevel-pp-cli calendars update-appointment-note` — Update Note
- `gohighlevel-pp-cli calendars update-resource` — Update calendar resource by ID
- `gohighlevel-pp-cli calendars update-schedule` — Modify an existing schedule by updating its rules, timezone, and name All fields are optional - only provided fields...
- `gohighlevel-pp-cli calendars validate-groups-slug` — Validate if group slug is available or not.

**campaigns** — Documentation for campaigns API

- `gohighlevel-pp-cli campaigns` — Get Campaigns

**companies** — Documentation for Companies API

- `gohighlevel-pp-cli companies <companyId>` — Get Comapny

**contacts** — Documentation for Contacts API

- `gohighlevel-pp-cli contacts add-remove-from-business` — Add/Remove Contacts From Business . Passing a `null` businessId will remove the businessId from the contacts
- `gohighlevel-pp-cli contacts create` — Please find the list of acceptable values for the `country` field <a...
- `gohighlevel-pp-cli contacts create-association` — Allows you to update tags to multiple contacts at once, you can add or remove tags from the contacts
- `gohighlevel-pp-cli contacts delete` — Delete Contact
- `gohighlevel-pp-cli contacts get` — Get Contacts **Note:** This API endpoint is deprecated. Please use the [Search...
- `gohighlevel-pp-cli contacts get-by-business-id` — Get Contacts By BusinessId
- `gohighlevel-pp-cli contacts get-contactid` — Get Contact
- `gohighlevel-pp-cli contacts get-duplicate` — Get Duplicate Contact.<br/><br/>If `Allow Duplicate Contact` is disabled under Settings, the global unique...
- `gohighlevel-pp-cli contacts search-advanced` — Search contacts based on combinations of advanced filters. Documentation Link -...
- `gohighlevel-pp-cli contacts update` — Please find the list of acceptable values for the `country` field <a...
- `gohighlevel-pp-cli contacts upsert` — Please find the list of acceptable values for the `country` field <a...

**conversation-ai** — Manage conversation ai

- `gohighlevel-pp-cli conversation-ai create-action` — Creates and attach a new action for an AI agent. Actions define specific tasks or behaviors that the agent can...
- `gohighlevel-pp-cli conversation-ai create-agent` — Creates a new AI agent for the location. The agent will be created with the specified configuration including name,...
- `gohighlevel-pp-cli conversation-ai delete-action` — Permanently deletes an action. This will remove the action from all associated agents and cannot be undone.
- `gohighlevel-pp-cli conversation-ai delete-agent` — Deletes an AI agent permanently. This action cannot be undone. All associated configurations and conversation...
- `gohighlevel-pp-cli conversation-ai get-action-by-id` — Retrieves detailed information about a specific action using its unique identifier. Returns the action...
- `gohighlevel-pp-cli conversation-ai get-agent` — Retrieves a specific AI agent by its ID. Returns the complete agent configuration including name, status, actions,...
- `gohighlevel-pp-cli conversation-ai get-generation-details` — Retrieves detailed information about AI responses including the System Prompt, Conversation history, Knowledge base,...
- `gohighlevel-pp-cli conversation-ai list-actions` — List for actions for an agent
- `gohighlevel-pp-cli conversation-ai search-agent` — Searches for AI agents based on various criteria including name, status, and configuration. Supports advanced...
- `gohighlevel-pp-cli conversation-ai update-action` — Updates an existing action's configuration. This includes modifying the action name, description, trigger...
- `gohighlevel-pp-cli conversation-ai update-agent` — Updates an existing AI agent's configuration. All fields in the agent configuration can be updated including name,...
- `gohighlevel-pp-cli conversation-ai update-followup-settings` — Update the followup settings for an action

**conversations** — Documentation for Conversations API

- `gohighlevel-pp-cli conversations add-an-inbound-message` — Post the necessary fields for the API to add a new inbound message. <br />
- `gohighlevel-pp-cli conversations add-an-outbound-message` — Post the necessary fields for the API to add a new outbound call.
- `gohighlevel-pp-cli conversations add-message-attachments` — Set attachments on an existing message (replaces existing). Maximum 5 URLs. Supported for TYPE_CUSTOM_CALL (34) and...
- `gohighlevel-pp-cli conversations cancel-scheduled-email-message` — Post the messageId for the API to delete a scheduled email message. <br />
- `gohighlevel-pp-cli conversations cancel-scheduled-message` — Post the messageId for the API to delete a scheduled message. <br />
- `gohighlevel-pp-cli conversations complete-file-upload` — Validates the uploaded file in GCS and returns the public URL. Call this endpoint after successfully uploading the...
- `gohighlevel-pp-cli conversations create` — Creates a new conversation with the data provided
- `gohighlevel-pp-cli conversations create-custom-subtype` — Create a new custom subtype for a location. Requires agency or account admin role.
- `gohighlevel-pp-cli conversations delete` — Delete the conversation details based on the conversation ID
- `gohighlevel-pp-cli conversations download-message-transcription` — Download the recording transcription for a message by passing the message id
- `gohighlevel-pp-cli conversations export-messages-by-location` — Export messages for a specific location with cursor-based pagination support. Response includes messageType...
- `gohighlevel-pp-cli conversations get` — Get the conversation details based on the conversation ID
- `gohighlevel-pp-cli conversations get-all-custom-subtypes` — Get all custom subtypes for a location
- `gohighlevel-pp-cli conversations get-contact-unsubscription-status` — Get all subscription statuses for a contact (all emails or specific email)
- `gohighlevel-pp-cli conversations get-email-by-id` — Get email by Id
- `gohighlevel-pp-cli conversations get-message` — Get message by message id.
- `gohighlevel-pp-cli conversations get-message-recording` — Get the recording for a message by passing the message id
- `gohighlevel-pp-cli conversations get-message-transcription` — Get the recording transcription for a message by passing the message id
- `gohighlevel-pp-cli conversations initiate-file-upload` — Generates a signed URL for direct file upload to Google Cloud Storage. Returns a signed URL valid for 15 minutes....
- `gohighlevel-pp-cli conversations live-chat-agent-typing` — Agent/AI-Bot will call this when they are typing a message in live chat message
- `gohighlevel-pp-cli conversations search` — Returns a list of all conversations matching the search criteria along with the sort and filter options selected.
- `gohighlevel-pp-cli conversations send-a-new-message` — Post the necessary fields for the API to send a new message.
- `gohighlevel-pp-cli conversations send-review-reply` — Post a reply to a customer review on Google My Business
- `gohighlevel-pp-cli conversations update` — Update the conversation details based on the conversation ID
- `gohighlevel-pp-cli conversations update-custom-subtype` — Update or archive a custom subtype. Requires agency or account admin role.
- `gohighlevel-pp-cli conversations update-message-status` — Post the necessary fields for the API to update message status.
- `gohighlevel-pp-cli conversations upload-file-attachments` — Post the necessary fields for the API to upload files. The files need to be a buffer with the key 'fileAttachment'....
- `gohighlevel-pp-cli conversations user-subscription-change` — Process subscription change initiated by a user (admin/agent). Supports individual custom subscription changes and...

**courses** — API Service for Courses and Memberships

- `gohighlevel-pp-cli courses` — Import Courses through public channels

**custom-fields** — Documentation for Sub-Account (Formerly location) API

- `gohighlevel-pp-cli custom-fields create` — <div> <p> Create Custom Field </p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...
- `gohighlevel-pp-cli custom-fields create-folder` — <div> <p> Create Custom Field Folder </p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...
- `gohighlevel-pp-cli custom-fields delete` — <div> <p> Delete Custom Field By Id </p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...
- `gohighlevel-pp-cli custom-fields delete-folder` — <div> <p> Create Custom Field Folder </p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...
- `gohighlevel-pp-cli custom-fields get-by-id` — <div> <p> Get Custom Field / Folder By Id.</p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...
- `gohighlevel-pp-cli custom-fields get-by-object-key` — <div> <p> Get Custom Fields By Object Key</p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...
- `gohighlevel-pp-cli custom-fields update` — <div> <p> Update Custom Field By Id </p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...
- `gohighlevel-pp-cli custom-fields update-folder` — <div> <p> Create Custom Field Folder </p> <div> <span style= 'display: inline-block; width: 25px; height: 25px;...

**custom-menus** — Documentation for Custom menus API

- `gohighlevel-pp-cli custom-menus create` — Creates a new custom menu for a company. Requires authentication and proper permissions. For Icon Usage Details...
- `gohighlevel-pp-cli custom-menus delete` — Removes a specific custom menu from the system. This operation requires authentication and proper permissions. The...
- `gohighlevel-pp-cli custom-menus get` — Fetches a collection of custom menus based on specified criteria. This endpoint allows clients to retrieve custom...
- `gohighlevel-pp-cli custom-menus get-by-id` — Fetches a single custom menus based on id. This endpoint allows clients to retrieve custom menu configurations,...
- `gohighlevel-pp-cli custom-menus update` — Updates an existing custom menu for a given company. Requires authentication and proper permissions.

**email** — Documentation for emails API

- `gohighlevel-pp-cli email` — Email Verification

**emails** — Documentation for emails API

- `gohighlevel-pp-cli emails create-template` — Create a new template
- `gohighlevel-pp-cli emails delete-template` — Delete a template
- `gohighlevel-pp-cli emails fetch-campaigns` — Get Campaigns
- `gohighlevel-pp-cli emails fetch-template` — Fetch email templates by location id
- `gohighlevel-pp-cli emails update-template` — Update a template

**forms** — Documentation for forms API

- `gohighlevel-pp-cli forms get` — Get Forms
- `gohighlevel-pp-cli forms get-submissions` — Get Forms Submissions
- `gohighlevel-pp-cli forms upload-to-custom-fields` — Post the necessary fields for the API to upload files. The files need to be a buffer with the key '< custom_field_id...

**funnels** — Documentation for funnels API

- `gohighlevel-pp-cli funnels create-redirect` — The 'Create Redirect' API Allows adding a new url redirect to the system. Use this endpoint to create a url redirect...
- `gohighlevel-pp-cli funnels delete-redirect-by-id` — The 'Delete Redirect By Id' API Allows deletion of a URL redirect from the system using its unique identifier. Use...
- `gohighlevel-pp-cli funnels fetch-redirects-list` — Retrieves a list of all URL redirects based on the given query parameters.
- `gohighlevel-pp-cli funnels get` — Retrieves a list of all funnels based on the given query parameters.
- `gohighlevel-pp-cli funnels get-pages-by-id` — Retrieves a list of all funnel pages based on the given query parameters.
- `gohighlevel-pp-cli funnels get-pages-count-by-id` — Retrieves count of all funnel pages based on the given query parameters.
- `gohighlevel-pp-cli funnels update-redirect-by-id` — The 'Update Redirect By Id' API Allows updating an existing URL redirect in the system. Use this endpoint to modify...

**invoices** — Documentation for invoice API

- `gohighlevel-pp-cli invoices auto-payment-schedule` — Manage Auto payment for an schedule invoice
- `gohighlevel-pp-cli invoices cancel-schedule` — API to cancel a scheduled invoice by schedule id
- `gohighlevel-pp-cli invoices create` — API to create an invoice
- `gohighlevel-pp-cli invoices create-estimate-template` — Create a new estimate template
- `gohighlevel-pp-cli invoices create-from-estimate` — Create a new invoice from an existing estimate
- `gohighlevel-pp-cli invoices create-new-estimate` — Create a new estimate with the provided details
- `gohighlevel-pp-cli invoices create-schedule` — API to create an invoice Schedule
- `gohighlevel-pp-cli invoices create-template` — API to create a template
- `gohighlevel-pp-cli invoices delete` — API to delete invoice by invoice id
- `gohighlevel-pp-cli invoices delete-estimate` — Delete an existing estimate
- `gohighlevel-pp-cli invoices delete-estimate-template` — Delete an existing estimate template
- `gohighlevel-pp-cli invoices delete-schedule` — API to delete an schedule by schedule id
- `gohighlevel-pp-cli invoices delete-template` — API to update an template by template id
- `gohighlevel-pp-cli invoices generate-estimate-number` — Get the next estimate number for the given location
- `gohighlevel-pp-cli invoices generate-number` — Get the next invoice number for the given location
- `gohighlevel-pp-cli invoices get` — API to get invoice by invoice id
- `gohighlevel-pp-cli invoices get-schedule` — API to get an schedule by schedule id
- `gohighlevel-pp-cli invoices get-settings` — Get the invoice settings for the given location
- `gohighlevel-pp-cli invoices get-template` — API to get an template by template id
- `gohighlevel-pp-cli invoices list` — API to get list of invoices
- `gohighlevel-pp-cli invoices list-estimate-templates` — Get a list of estimate templates or a specific template by ID
- `gohighlevel-pp-cli invoices list-estimates` — Get a paginated list of estimates
- `gohighlevel-pp-cli invoices list-schedules` — API to get list of schedules
- `gohighlevel-pp-cli invoices list-templates` — API to get list of templates
- `gohighlevel-pp-cli invoices preview-estimate-template` — Get a preview of an estimate template
- `gohighlevel-pp-cli invoices schedule-schedule` — API to schedule an schedule invoice to start sending to the customer
- `gohighlevel-pp-cli invoices send-estimate` — API to send estimate by estimate id
- `gohighlevel-pp-cli invoices text2pay` — API to create or update a text2pay invoice
- `gohighlevel-pp-cli invoices update` — API to update invoice by invoice id
- `gohighlevel-pp-cli invoices update-and-schedule-schedule` — API to update scheduled recurring invoice
- `gohighlevel-pp-cli invoices update-estimate` — Update an existing estimate with new details
- `gohighlevel-pp-cli invoices update-estimate-last-visited-at` — API to update estimate last visited at by estimate id
- `gohighlevel-pp-cli invoices update-estimate-template` — Update an existing estimate template
- `gohighlevel-pp-cli invoices update-last-visited-at` — API to update invoice last visited at by invoice id
- `gohighlevel-pp-cli invoices update-payment-methods-configuration` — API to update template late fees configuration by template id
- `gohighlevel-pp-cli invoices update-schedule` — API to update an schedule by schedule id
- `gohighlevel-pp-cli invoices update-template` — API to update an template by template id
- `gohighlevel-pp-cli invoices update-template-late-fees-configuration` — API to update template late fees configuration by template id

**knowledge-bases** — Documentation for Knowledge Base API

- `gohighlevel-pp-cli knowledge-bases create` — Create a new knowledge base (max 15 knowledge bases per location)
- `gohighlevel-pp-cli knowledge-bases create-knowledgebases` — Create a new FAQ inside knowledge base
- `gohighlevel-pp-cli knowledge-bases delete` — Delete a knowledge base
- `gohighlevel-pp-cli knowledge-bases delete-knowledgebases` — Delete an existing knowledge base FAQ
- `gohighlevel-pp-cli knowledge-bases delete-trained-urls-for` — Delete trained pages
- `gohighlevel-pp-cli knowledge-bases discover-website` — Start crawling and discover pages for training
- `gohighlevel-pp-cli knowledge-bases get-all-website-urls-data-by` — Get all trained page links by knowledge base
- `gohighlevel-pp-cli knowledge-bases get-by-id` — Get knowledge base by ID
- `gohighlevel-pp-cli knowledge-bases get-crawling-status-for-latest-operation` — Get crawling status for the latest operation
- `gohighlevel-pp-cli knowledge-bases list` — Retrieves FAQs for a knowledge base. Supports pagination using limit and lastFaqId parameters.
- `gohighlevel-pp-cli knowledge-bases list-all-paginated` — Get all knowledge bases for a location by location Id (paginated)
- `gohighlevel-pp-cli knowledge-bases train-discovered-urls` — Train discovered website pages and ingest into the knowledge base
- `gohighlevel-pp-cli knowledge-bases update` — Update a knowledge base
- `gohighlevel-pp-cli knowledge-bases update-knowledgebases` — Update an existing knowledge base FAQ

**links** — Manage links

- `gohighlevel-pp-cli links create` — Create Link
- `gohighlevel-pp-cli links delete` — Delete Link
- `gohighlevel-pp-cli links get` — Get Links
- `gohighlevel-pp-cli links get-by-id` — Get a single link by its ID
- `gohighlevel-pp-cli links search-trigger` — Get list of links by searching
- `gohighlevel-pp-cli links update` — Update Link

**locations** — Documentation for Sub-Account (Formerly location) API

- `gohighlevel-pp-cli locations create` — <div> <p>Create a new Sub-Account (Formerly Location) based on the data provided</p> <div> <span style= 'display:...
- `gohighlevel-pp-cli locations delete` — Delete a Sub-Account (Formerly Location) from the Agency
- `gohighlevel-pp-cli locations get` — Get details of a Sub-Account (Formerly Location) by passing the sub-account id
- `gohighlevel-pp-cli locations put` — Update a Sub-Account (Formerly Location) based on the data provided
- `gohighlevel-pp-cli locations search` — Search Sub-Account (Formerly Location)

**marketplace** — Documentation for Marketplace API

- `gohighlevel-pp-cli marketplace charge` — Create a new wallet charge
- `gohighlevel-pp-cli marketplace delete-charge` — Delete a wallet charge
- `gohighlevel-pp-cli marketplace get-charges` — Get all wallet charges
- `gohighlevel-pp-cli marketplace get-installer-details` — Fetches installer details for the authenticated user. This endpoint returns information about the company, location,...
- `gohighlevel-pp-cli marketplace get-rebilling-config-for-app` — Get rebilling config for an app subscription and usage plans for the authenticated sub-account. This endpoint...
- `gohighlevel-pp-cli marketplace get-specific-charge` — Get specific wallet charge details
- `gohighlevel-pp-cli marketplace has-funds` — Check if account has sufficient funds
- `gohighlevel-pp-cli marketplace migrate-connection` — Migrates an external authentication connection credentials (basic or oauth2) for a specific app and location. This...
- `gohighlevel-pp-cli marketplace uninstall-application` — Uninstalls an application from your company or a specific location. This will remove the application`s access and...

**medias** — Documentation for Files API

- `gohighlevel-pp-cli medias bulk-delete-objects` — Soft-deletes or trashes multiple files and folders in a single request
- `gohighlevel-pp-cli medias bulk-update-objects` — Updates metadata or status of multiple files and folders
- `gohighlevel-pp-cli medias create-folder` — Creates a new folder in the media storage
- `gohighlevel-pp-cli medias delete-content` — Deletes specific file or folder from the media storage
- `gohighlevel-pp-cli medias fetch-content` — Fetches list of files and folders from the media storage
- `gohighlevel-pp-cli medias update-object` — Updates a single file or folder by ID
- `gohighlevel-pp-cli medias upload-content` — If hosted is set to true then fileUrl is required. Else file is required. If adding a file, maximum allowed is 25 MB

**oauth** — Manage oauth

- `gohighlevel-pp-cli oauth get-access-token` — Use Access Tokens to access GoHighLevel resources on behalf of an authenticated location/company.
- `gohighlevel-pp-cli oauth get-installed-location` — This API allows you fetch location where app is installed upon
- `gohighlevel-pp-cli oauth get-location-access-token` — This API allows you to generate locationAccessToken from AgencyAccessToken

**objects** — Manage objects

- `gohighlevel-pp-cli objects create-custom-schema` — Allows you to create a custom object schema. To understand objects and records, please have a look at the...
- `gohighlevel-pp-cli objects get-by-location-id` — Get all objects for a location. Supported Objects are contact, opportunity, business and custom objects.To...
- `gohighlevel-pp-cli objects get-schema-by-key` — Retrieve Object Schema by key or ID. This will return the schema of the custom object, including all its fields and...
- `gohighlevel-pp-cli objects update-custom` — Update Custom Object Schema or standard object's like contact, opportunity, business searchable fields. To...

**opportunities** — Documentation for Opportunities API

- `gohighlevel-pp-cli opportunities create-opportunity` — Create Opportunity
- `gohighlevel-pp-cli opportunities delete-opportunity` — Delete Opportunity
- `gohighlevel-pp-cli opportunities get-lost-reason` — Get lost reason
- `gohighlevel-pp-cli opportunities get-opportunity` — Get Opportunity
- `gohighlevel-pp-cli opportunities get-pipelines` — Get Pipelines
- `gohighlevel-pp-cli opportunities search-advanced` — Search Opportunities based on combinations of advanced filters. Documentation Link -...
- `gohighlevel-pp-cli opportunities search-opportunity` — Search Opportunity
- `gohighlevel-pp-cli opportunities update-opportunity` — Update Opportunity
- `gohighlevel-pp-cli opportunities upsert-opportunity` — Upsert Opportunity

**payments** — Documentation for payments API

- `gohighlevel-pp-cli payments create-config` — API to create a new payment config for given location
- `gohighlevel-pp-cli payments create-coupon` — The 'Create Coupon' API allows you to create a new promotional coupon with customizable parameters such as discount...
- `gohighlevel-pp-cli payments create-integration` — API to create a new association for an app and location
- `gohighlevel-pp-cli payments create-integration-provider` — The 'Create White-label Integration Provider' API allows adding a new payment provider integration to the system...
- `gohighlevel-pp-cli payments create-order-fulfillment` — The 'Order Fulfillment' API facilitates the process of fulfilling an order.
- `gohighlevel-pp-cli payments custom-provider-marketplace-app-update-capabilities` — Toggle capabilities for the marketplace app tied to the OAuth client
- `gohighlevel-pp-cli payments delete-coupon` — The 'Delete Coupon' API allows you to permanently remove a coupon from your system using its unique identifier. Use...
- `gohighlevel-pp-cli payments delete-integration` — API to delete an association for an app and location
- `gohighlevel-pp-cli payments disconnect-config` — API to disconnect an existing payment config for given location
- `gohighlevel-pp-cli payments fetch-config` — API for fetching an existing payment config for given location
- `gohighlevel-pp-cli payments get-coupon` — The 'Get Coupon Details' API enables you to retrieve comprehensive information about a specific coupon using either...
- `gohighlevel-pp-cli payments get-order-by-id` — The 'Get Order by ID' API allows to retrieve information for a specific order using its unique identifier. Use this...
- `gohighlevel-pp-cli payments get-subscription-by-id` — The 'Get Subscription by ID' API allows to retrieve information for a specific subscription using its unique...
- `gohighlevel-pp-cli payments get-transaction-by-id` — The 'Get Transaction by ID' API allows to retrieve information for a specific transaction using its unique...
- `gohighlevel-pp-cli payments list-coupons` — The 'List Coupons' API allows you to retrieve a list of all coupons available in your location. Use this endpoint to...
- `gohighlevel-pp-cli payments list-integration-providers` — The 'List White-label Integration Providers' API allows to retrieve a paginated list of integration providers....
- `gohighlevel-pp-cli payments list-order-fulfillment` — List all fulfillment history of an order
- `gohighlevel-pp-cli payments list-order-notes` — List all notes of an order
- `gohighlevel-pp-cli payments list-orders` — The 'List Orders' API allows to retrieve a paginated list of orders. Customize your results by filtering orders...
- `gohighlevel-pp-cli payments list-subscriptions` — The 'List Subscriptions' API allows to retrieve a paginated list of subscriptions. Customize your results by...
- `gohighlevel-pp-cli payments list-transactions` — The 'List Transactions' API allows to retrieve a paginated list of transactions. Customize your results by filtering...
- `gohighlevel-pp-cli payments record-order` — The 'Record Order Payment' API allows to record a payment for an order. Use this endpoint to record payment for an...
- `gohighlevel-pp-cli payments update-coupon` — The 'Update Coupon' API enables you to modify existing coupon details such as discount values, validity periods,...

**phone-system** — Manage phone system

- `gohighlevel-pp-cli phone-system active-numbers` — Retrieve a paginated list of active phone numbers for a specific location. Supports filtering, pagination, and...
- `gohighlevel-pp-cli phone-system available-numbers` — Search for available phone numbers to purchase for a specific location. Supports filtering by number pattern, type,...
- `gohighlevel-pp-cli phone-system get-number-pool-list` — Get list of number pools
- `gohighlevel-pp-cli phone-system purchase-phone-number` — Purchase a phone number for a specific location.

**products** — Documentation for funnels API

- `gohighlevel-pp-cli products bulk-edit` — API to bulk edit products and their associated prices (max 30 entities)
- `gohighlevel-pp-cli products bulk-update` — API to bulk update products (price, availability, collections, delete)
- `gohighlevel-pp-cli products bulk-update-review` — Update one or multiple product reviews: status, reply, etc.
- `gohighlevel-pp-cli products create` — The 'Create Product' API allows adding a new product to the system. Use this endpoint to create a product with the...
- `gohighlevel-pp-cli products create-collection` — Create a new Product Collection for a specific location
- `gohighlevel-pp-cli products delete-by-id` — The 'Delete Product by ID' API allows deleting a specific product using its unique identifier. Use this endpoint to...
- `gohighlevel-pp-cli products delete-collection` — Delete specific product collection with Id :collectionId
- `gohighlevel-pp-cli products delete-review` — Delete specific product review
- `gohighlevel-pp-cli products get-by-id` — The 'Get Product by ID' API allows to retrieve information for a specific product using its unique identifier. Use...
- `gohighlevel-pp-cli products get-collection` — Internal API to fetch the Product Collections
- `gohighlevel-pp-cli products get-collection-id` — Get Details about individual product collection
- `gohighlevel-pp-cli products get-list-inventory` — The 'List Inventory API allows the user to retrieve a paginated list of inventory items. Use this endpoint to fetch...
- `gohighlevel-pp-cli products get-reviews` — API to fetch the Product Reviews
- `gohighlevel-pp-cli products get-reviews-count` — API to fetch the Review Count as per status
- `gohighlevel-pp-cli products get-store-stats` — API to fetch the total number of products, included in the store, and excluded from the store and other stats
- `gohighlevel-pp-cli products list-invoices` — The 'List Products' API allows to retrieve a paginated list of products. Customize your results by filtering...
- `gohighlevel-pp-cli products update-by-id` — The 'Update Product by ID' API allows modifying information for a specific product using its unique identifier. Use...
- `gohighlevel-pp-cli products update-collection` — Update a specific product collection with Id :collectionId
- `gohighlevel-pp-cli products update-display-priority` — API to set the display priority of products in a store
- `gohighlevel-pp-cli products update-inventory` — The Update Inventory API allows the user to bulk update the inventory for multiple items. Use this endpoint to...
- `gohighlevel-pp-cli products update-review` — Update status, reply, etc of a particular review
- `gohighlevel-pp-cli products update-store-status` — API to update the status of products in a particular store

**proposals** — Documentation for Documents and Contracts API

- `gohighlevel-pp-cli proposals list-documents-contracts` — List documents for a location
- `gohighlevel-pp-cli proposals list-documents-contracts-templates` — List document contract templates for a location
- `gohighlevel-pp-cli proposals send-documents-contracts` — Send document to a client
- `gohighlevel-pp-cli proposals send-documents-contracts-template` — Send template to a client

**saas** — API Service for SaaS

- `gohighlevel-pp-cli saas bulk-disable` — Disable SaaS for locations for given locationIds
- `gohighlevel-pp-cli saas bulk-enable` — Enable SaaS mode for multiple locations with support for both SaaS v1 and v2
- `gohighlevel-pp-cli saas enable-location` — <div> <p>Enable SaaS for Sub-Account (Formerly Location) based on the data provided</p> <div> <span style= 'display:...
- `gohighlevel-pp-cli saas generate-payment-link` — Update SaaS subscription for given locationId and customerId
- `gohighlevel-pp-cli saas get-agency-plans` — Fetch all agency subscription plans for a given company ID
- `gohighlevel-pp-cli saas get-location-subscription` — Fetch subscription details for a specific location from location metadata
- `gohighlevel-pp-cli saas get-locations` — Fetch all SaaS-activated locations for a company with pagination
- `gohighlevel-pp-cli saas get-plan` — Fetch a specific SaaS plan by plan ID
- `gohighlevel-pp-cli saas locations` — Get locations by stripeCustomerId or stripeSubscriptionId with companyId
- `gohighlevel-pp-cli saas pause-location` — Pause Sub account for given locationId
- `gohighlevel-pp-cli saas update-rebilling` — Bulk update rebilling for given locationIds

**saas-api** — Manage saas api

- `gohighlevel-pp-cli saas-api bulk-disable-saas-deprecated` — Disable SaaS for locations for given locationIds
- `gohighlevel-pp-cli saas-api bulk-enable-saas-deprecated` — Enable SaaS mode for multiple locations with support for both SaaS v1 and v2
- `gohighlevel-pp-cli saas-api enable-saas-location-deprecated` — <div> <p>Enable SaaS for Sub-Account (Formerly Location) based on the data provided</p> <div> <span style= 'display:...
- `gohighlevel-pp-cli saas-api get-agency-plans-deprecated` — Fetch all agency subscription plans for a given company ID
- `gohighlevel-pp-cli saas-api get-location-subscription-deprecated` — Fetch subscription details for a specific location from location metadata
- `gohighlevel-pp-cli saas-api get-saas-locations-deprecated` — Fetch all SaaS-activated locations for a company with pagination
- `gohighlevel-pp-cli saas-api get-saas-plan-deprecated` — Fetch a specific SaaS plan by plan ID
- `gohighlevel-pp-cli saas-api locations-deprecated` — Get locations by stripeCustomerId or stripeSubscriptionId with companyId
- `gohighlevel-pp-cli saas-api pause-location-deprecated` — Pause Sub account for given locationId
- `gohighlevel-pp-cli saas-api update-rebilling-deprecated` — Bulk update rebilling for given locationIds
- `gohighlevel-pp-cli saas-api update-saas-subscription-deprecated` — Update SaaS subscription for given locationId and customerId

**snapshots** — Documentation for Ad-publishing API

- `gohighlevel-pp-cli snapshots create-share-link` — Create a share link for snapshot
- `gohighlevel-pp-cli snapshots get-custom` — Get a list of all own and imported Snapshots
- `gohighlevel-pp-cli snapshots get-latest-push` — Get Latest Snapshot Push Status for a location id
- `gohighlevel-pp-cli snapshots get-push` — Get list of sub-accounts snapshot pushed in time period

**social-media-posting** — Manage social media posting

- `gohighlevel-pp-cli social-media-posting attach-facebook-page-group` — Attach facebook pages
- `gohighlevel-pp-cli social-media-posting attach-instagram-page-group` — Attach Instagram Professional Accounts
- `gohighlevel-pp-cli social-media-posting attach-linkedin-page-profile` — Attach linkedin pages and profile
- `gohighlevel-pp-cli social-media-posting attach-tiktok-profile` — Attach Tiktok profile
- `gohighlevel-pp-cli social-media-posting attach-twitter-profile` — <div><div> <span style= 'display: inline-block; width: 25px; height: 25px; background-color: red; color: black;...
- `gohighlevel-pp-cli social-media-posting get-facebook-page-group` — Get facebook pages
- `gohighlevel-pp-cli social-media-posting get-google-locations` — Get google business locations
- `gohighlevel-pp-cli social-media-posting get-instagram-page-group` — Get Instagram Professional Accounts
- `gohighlevel-pp-cli social-media-posting get-linkedin-page-profile` — Get Linkedin pages and profile
- `gohighlevel-pp-cli social-media-posting get-social-media-statistics` — Retrieve analytics data for multiple social media accounts. Provides metrics for the last 7 days with comparison to...
- `gohighlevel-pp-cli social-media-posting get-tiktok-business-profile` — Get Tiktok Business profile
- `gohighlevel-pp-cli social-media-posting get-tiktok-profile` — Get Tiktok profile
- `gohighlevel-pp-cli social-media-posting get-twitter-profile` — <div><div> <span style= 'display: inline-block; width: 25px; height: 25px; background-color: red; color: black;...
- `gohighlevel-pp-cli social-media-posting set-google-locations` — Set google business locations
- `gohighlevel-pp-cli social-media-posting start-facebook-oauth` — Open the API in a window with appropriate params and headers instead of using the Curl. User is navigated to...
- `gohighlevel-pp-cli social-media-posting start-google-oauth` — Open the API in a window with appropriate params and headers instead of using the Curl. User is navigated to Google...
- `gohighlevel-pp-cli social-media-posting start-instagram-oauth` — Open the API in a window with appropriate params and headers instead of using the Curl. User is navigated to...
- `gohighlevel-pp-cli social-media-posting start-linkedin-oauth` — Open the API in a window with appropriate params and headers instead of using the Curl. User is navigated to...
- `gohighlevel-pp-cli social-media-posting start-tiktok-business-oauth` — Open the API in a window with appropriate params and headers instead of using the Curl. User is navigated to...
- `gohighlevel-pp-cli social-media-posting start-tiktok-oauth` — Open the API in a window with appropriate params and headers instead of using the Curl. User is navigated to Tiktok...
- `gohighlevel-pp-cli social-media-posting start-twitter-oauth` — <div><div> <span style= 'display: inline-block; width: 25px; height: 25px; background-color: red; color: black;...

**store** — Documentation for products API

- `gohighlevel-pp-cli store create-setting` — Create or update store settings by altId and altType.
- `gohighlevel-pp-cli store create-shipping-carrier` — The 'Create Shipping Carrier' API allows adding a new shipping carrier.
- `gohighlevel-pp-cli store create-shipping-rate` — The 'Create Shipping Rate' API allows adding a new shipping rate.
- `gohighlevel-pp-cli store create-shipping-zone` — The 'Create Shipping Zone' API allows adding a new shipping zone.
- `gohighlevel-pp-cli store delete-shipping-carrier` — Delete specific shipping carrier with Id :shippingCarrierId
- `gohighlevel-pp-cli store delete-shipping-rate` — Delete specific shipping rate with Id :shippingRateId
- `gohighlevel-pp-cli store delete-shipping-zone` — Delete specific shipping zone with Id :shippingZoneId
- `gohighlevel-pp-cli store get-available-shipping-zones` — This return available shipping rates for country based on order amount
- `gohighlevel-pp-cli store get-settings` — Get store settings by altId and altType.
- `gohighlevel-pp-cli store get-shipping-carriers` — The 'List Shipping Carrier' API allows to retrieve a paginated list of shipping carrier.
- `gohighlevel-pp-cli store get-shipping-rates` — The 'List Shipping Rate' API allows to retrieve a paginated list of shipping rate.
- `gohighlevel-pp-cli store get-shipping-zones` — The 'List Shipping Zone' API allows to retrieve a paginated list of shipping zone.
- `gohighlevel-pp-cli store list-shipping-carriers` — The 'List Shipping Carrier' API allows to retrieve a list of shipping carrier.
- `gohighlevel-pp-cli store list-shipping-rates` — The 'List Shipping Rate' API allows to retrieve a list of shipping rate.
- `gohighlevel-pp-cli store list-shipping-zones` — The 'List Shipping Zone' API allows to retrieve a list of shipping zone.
- `gohighlevel-pp-cli store update-shipping-carrier` — The 'update Shipping Carrier' API allows update a shipping carrier to the system.
- `gohighlevel-pp-cli store update-shipping-rate` — The 'update Shipping Rate' API allows update a shipping rate to the system.
- `gohighlevel-pp-cli store update-shipping-zone` — The 'update Shipping Zone' API allows update a shipping zone to the system.

**surveys** — Documentation for surveys API

- `gohighlevel-pp-cli surveys get` — Get Surveys
- `gohighlevel-pp-cli surveys get-submissions` — Get Surveys Submissions

**users** — Documentation for users API

- `gohighlevel-pp-cli users create` — Create User
- `gohighlevel-pp-cli users delete` — Delete User
- `gohighlevel-pp-cli users filter-by-email` — Filter users by company ID, deleted status, and email array
- `gohighlevel-pp-cli users get` — Get User
- `gohighlevel-pp-cli users get-by-location` — Deprecated. Use `GET /users/search` instead. Pass `locationId` as a query parameter to filter results by location,...
- `gohighlevel-pp-cli users search` — Search Users
- `gohighlevel-pp-cli users update` — Update User

**voice-ai** — Documentation for Voice AI API

- `gohighlevel-pp-cli voice-ai create-action` — Create a new action for a voice AI agent. Actions define specific behaviors and capabilities for the agent during calls.
- `gohighlevel-pp-cli voice-ai create-agent` — Create a new voice AI agent configuration and settings
- `gohighlevel-pp-cli voice-ai delete-action` — Delete an existing action from a voice AI agent. This permanently removes the action and its configuration.
- `gohighlevel-pp-cli voice-ai delete-agent` — Delete a voice AI agent and all its configurations
- `gohighlevel-pp-cli voice-ai get-action` — Retrieve details of a specific action by its ID. Returns the action configuration including actionParameters.
- `gohighlevel-pp-cli voice-ai get-agent` — Retrieve detailed configuration and settings for a specific voice AI agent
- `gohighlevel-pp-cli voice-ai get-agents` — Retrieve a paginated list of agents for given location.
- `gohighlevel-pp-cli voice-ai get-call-log` — Returns a call log by callId.
- `gohighlevel-pp-cli voice-ai get-call-logs` — Returns call logs for Voice AI agents scoped to a location. Supports filtering by agent, contact, call type, action...
- `gohighlevel-pp-cli voice-ai patch-agent` — Partially update an existing voice AI agent
- `gohighlevel-pp-cli voice-ai update-action` — Update an existing action for a voice AI agent. Modifies the behavior and configuration of an agent action.

**workflows** — Documentation for workflows API

- `gohighlevel-pp-cli workflows` — Get Workflow


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
gohighlevel-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Monday-standup roster across all locations

```bash
gohighlevel-pp-cli roster --metric leads-24h,unread,unpaid-invoices,stale-opps --json --select rows.location_name,rows.unread,rows.stale_opps
```

Use `--select` with dotted paths to grab only the columns the standup deck needs — full roster output is verbose JSON.

### Triage what needs you right now

```bash
gohighlevel-pp-cli unread --location all --since 1h --assigned-to me --json
```

One shell command instead of 10 inbox tabs.

### Stale-opp hunt before pipeline review

```bash
gohighlevel-pp-cli stale-opps --pipeline 'Sales 2026' --threshold 14d --no-activity
```

Joins messages + notes locally so 'no activity' actually means no activity.

### Migration reconcile after bulk import

```bash
gohighlevel-pp-cli contacts reconcile --source migration-source.csv --key email --json
```

Pure local diff between source CSV and synced contacts — closes the migration loop.

### Bulk-retag a campaign cohort with dry-run

```bash
gohighlevel-pp-cli contacts bulk-tag --from-search 'campaign:spring-2026' tested --dry-run
```

Always dry-run first; bulk ops respect the 100/10s rate limit.

## Auth Setup

HighLevel uses OAuth 2.0 Bearer tokens or Private Integration Tokens (PITs). PITs are the simplest path: create one in Settings → Private Integrations, then `export HIGHLEVEL_TOKEN=<pit>`. Every request also requires a `Version: 2021-07-28` header — the CLI sets it automatically.

Run `gohighlevel-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  gohighlevel-pp-cli blogs get --location-id 550e8400-e29b-41d4-a716-446655440000 --skip 42 --limit 42 --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
gohighlevel-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
gohighlevel-pp-cli feedback --stdin < notes.txt
gohighlevel-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.gohighlevel-pp-cli/feedback.jsonl`. They are never POSTed unless `GOHIGHLEVEL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOHIGHLEVEL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
gohighlevel-pp-cli profile save briefing --json
gohighlevel-pp-cli --profile briefing blogs get --location-id 550e8400-e29b-41d4-a716-446655440000 --skip 42 --limit 42
gohighlevel-pp-cli profile list --json
gohighlevel-pp-cli profile show briefing
gohighlevel-pp-cli profile delete briefing --yes
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
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `gohighlevel-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/cmd/gohighlevel-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add gohighlevel-pp-mcp -- gohighlevel-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which gohighlevel-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   gohighlevel-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `gohighlevel-pp-cli <command> --help`.
