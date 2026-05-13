# HubSpot CLI

**Every HubSpot CRM endpoint, plus a local SQLite mirror that answers compound pre-sale queries no other HubSpot tool can return in one call.**

A read-focused HubSpot CLI built for agents and automation flows. Every CRM object lives in a local SQLite cache after the first sync, so stale-lead, pipeline-health, recent-intake, dedup, Closed Won handoff, and engagement-decay queries become single-call commands instead of multi-step API orchestration. Outputs default to compact JSON so token cost stays predictable across thousands of agent invocations.

Learn more at [HubSpot](https://developers.hubspot.com/docs/api).

Printed by [@simplepathmedia](https://github.com/simplepathmedia) (simplepathmedia).

## Install

The recommended path installs both the `hubspot-pp-cli` binary and the `pp-hubspot` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install hubspot
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install hubspot --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hubspot-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-hubspot --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-hubspot --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-hubspot skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-hubspot. The skill defines how its required CLI can be installed.
```

## Authentication

Authenticate with a HubSpot private app token. Create the app at https://app.hubspot.com/private-apps, grant read scopes for contacts, deals, companies, leads, owners, properties, lists, associations, and engagements, then export `HUBSPOT_TOKEN`. No OAuth, no callback handshake — single bearer token, scoped per private app.

## Quick Start

```bash
# Confirm the token is valid and the scopes cover the resources you sync.
hubspot-pp-cli doctor


# Smoke test the live API by fetching the contact lifecycle pipeline.
# Then run `hubspot-pp-cli sync` to populate the local SQLite mirror for compound queries.
hubspot-pp-cli pipelines-crm get-v3-pipelines-object-type-get-all contacts --agent


# First pre-sale compound query: leads in the default qualified stage untouched for two weeks.
hubspot-pp-cli stale-leads --days 14 --agent


# Monday pipeline review in one call — only the fields an agent needs.
hubspot-pp-cli pipeline-health --agent --select rows.dealId,rows.stage,rows.weighted_value


# End-of-week handoff bundle in ClickUp-import shape.
hubspot-pp-cli closed-won-handoff --since 7d --format clickup

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Pre-sale compound queries
- **`stale-leads`** — Surface contacts in a given lifecycle stage with no engagement for N days, ranked by days-since-last-touch.

  _When an agent is asked 'which leads need a call this week?', this returns the ranked answer in one tool call instead of orchestrating contacts.search + per-contact engagement fetches._

  ```bash
  hubspot-pp-cli stale-leads --stage leadqualified --days 14 --agent
  ```
- **`pipeline-health`** — Every deal by stage with age-in-stage, weighted value (amount × stage probability), and days since last engagement.

  _The Monday pipeline review in one tool call — agents asked 'where is the pipeline stuck?' get a stage-ranked answer with weighted value attached._

  ```bash
  hubspot-pp-cli pipeline-health --agent --select rows.dealId,rows.stage,rows.weighted_value,rows.days_since_touch
  ```
- **`recent-intake`** — New contacts in the last N hours with full source attribution: original-source, latest-source, UTM medium, UTM campaign.

  _Answers 'what landed overnight and where did it come from?' without burning 4+ API calls per lead and without re-implementing the source-attribution property tree._

  ```bash
  hubspot-pp-cli recent-intake --hours 24 --source apollo --agent
  ```
- **`dedup`** — Find clustered duplicate contacts by normalized email, e164 phone, or registrable company domain.

  _Bulk dedup answer for n8n flows or agents staging Apollo enrichment — one local query versus N×3 round-trips._

  ```bash
  hubspot-pp-cli dedup --key email --threshold strict --agent
  ```
- **`engagement-decay`** — Rank contacts whose recent engagement frequency dropped vs the prior window.

  _Surfaces leads going cold before they go silent — agents can flag at-risk contacts proactively._

  ```bash
  hubspot-pp-cli engagement-decay --days 30 --window 7 --agent --select rows.contactId,rows.prev_count,rows.curr_count,rows.delta
  ```
- **`digest`** — Composite Monday-morning rollup: new contacts, deals advanced, deals closed, stalest leads, recent intake — one command, one JSON object.

  _The Monday-morning ritual as a single tool call — agents performing daily standup intelligence get the full snapshot without orchestrating four separate queries._

  ```bash
  hubspot-pp-cli digest --since 24h --agent
  ```

### Cross-system bridges
- **`closed-won-handoff`** — Every contact whose deal closed Won in the window, with full property bundle and associated company/engagement summary, in ClickUp-import shape.

  _Bridges the architectural rule 'pre-sale lives in HubSpot, post-sale lives elsewhere' in one command — agents performing weekly handoff stop hand-stitching the bundle._

  ```bash
  hubspot-pp-cli closed-won-handoff --since 7d --format clickup
  ```

### Cross-entity graph
- **`lead-trace`** — Walk associations from a contact through its deals, companies, and engagement timeline; emit one composite JSON.

  _Single-call lead triage: 'tell me everything about this contact' returns the full neighborhood in one shot instead of 5+ chained calls._

  ```bash
  hubspot-pp-cli lead-trace 12345 --agent
  ```

### Source attribution
- **`utm-cohort`** — Cohort rollup by UTM campaign: size, percent at each pipeline stage, percent Closed Won, average deal amount.

  _Answers 'did source X actually convert?' in one call when an agent is deciding which intake channel to scale._

  ```bash
  hubspot-pp-cli utm-cohort --campaign google-maps-realtors --agent
  ```

## Usage

Run `hubspot-pp-cli --help` for the full command reference and flag list.

## Commands

### associations-crm

Manage crm

- **`hubspot-pp-cli associations-crm delete-v4-objects-object-type-object-id-associations-to-object-type-to-object-id-archive`** - deletes all associations between two records.
- **`hubspot-pp-cli associations-crm get-v4-objects-object-type-object-id-associations-to-object-type-get-page`** - Retrieve all associations between a specific record and an object type. Limit 500 per call.
- **`hubspot-pp-cli associations-crm post-v4-associations-from-object-type-to-object-type-batch-archive-archive`** - Batch delete associations for objects
- **`hubspot-pp-cli associations-crm post-v4-associations-from-object-type-to-object-type-batch-associate-default-create-default`** - Create the default (most generic) association type between two object types
- **`hubspot-pp-cli associations-crm post-v4-associations-from-object-type-to-object-type-batch-create-create`** - Batch create associations for objects
- **`hubspot-pp-cli associations-crm post-v4-associations-from-object-type-to-object-type-batch-labels-archive-archive-labels`** - Batch delete specific association labels for objects. Deleting an unlabeled association will also delete all labeled associations between those two objects
- **`hubspot-pp-cli associations-crm post-v4-associations-from-object-type-to-object-type-batch-read-get-page`** - Batch read associations for objects to specific object type. The 'after' field in a returned paging object  can be added alongside the 'id' to retrieve the next page of associations from that objectId. The 'link' field is deprecated and should be ignored. Note: The 'paging' field will only be present if there are more pages and absent otherwise.
- **`hubspot-pp-cli associations-crm post-v4-associations-usage-high-usage-report-user-id-request`** - Requests a report of all objects in the portal which have a high usage of associations
- **`hubspot-pp-cli associations-crm put-v4-objects-from-object-type-from-object-id-associations-default-to-object-type-to-object-id-create-default`** - Create the default (most generic) association type between two object types
- **`hubspot-pp-cli associations-crm put-v4-objects-object-type-object-id-associations-to-object-type-to-object-id-create`** - Set association labels between two records.

### batch

Manage batch

- **`hubspot-pp-cli batch post-crm-v3-properties-object-type-archive-archive`** - Archive a provided list of properties. This method will return a 204 No Content response on success regardless of the initial state of the property (e.g. active, already archived, non-existent).
- **`hubspot-pp-cli batch post-crm-v3-properties-object-type-create-create`** - Create a batch of properties using the same rules as when creating an individual property.
- **`hubspot-pp-cli batch post-crm-v3-properties-object-type-read-read`** - Read a provided list of properties.

### calls-crm

Manage crm

- **`hubspot-pp-cli calls-crm delete-v3-objects-calls-call-id-archive`** - Move an Object identified by `{callId}` to the recycling bin.
- **`hubspot-pp-cli calls-crm get-v3-objects-calls-call-id-get-by-id`** - Read an Object identified by `{callId}`. `{callId}` refers to the internal object ID by default, or optionally any unique property value as specified by the `idProperty` query param.  Control what is returned via the `properties` query param.
- **`hubspot-pp-cli calls-crm get-v3-objects-calls-get-page`** - Read a page of calls. Control what is returned via the `properties` query param.
- **`hubspot-pp-cli calls-crm patch-v3-objects-calls-call-id-update`** - Perform a partial update of an Object identified by `{callId}`or optionally a unique property value as specified by the `idProperty` query param. `{callId}` refers to the internal object ID by default, and the `idProperty` query param refers to a property whose values are unique for the object. Provided property values will be overwritten. Read-only and non-existent properties will result in an error. Properties values can be cleared by passing an empty string.
- **`hubspot-pp-cli calls-crm post-v3-objects-calls-batch-archive-archive`** - Archive a batch of calls by ID. Deleted calls can be restored within 90 days of being deleted, but call recordings recording will be permanently deleted. Learn more about [restoring activity records](https://knowledge.hubspot.com/records/restore-deleted-activity-in-a-record).
- **`hubspot-pp-cli calls-crm post-v3-objects-calls-batch-create-create`** - Create a batch of calls. The `inputs` array can contain a `properties` object to define property values for each record, along with an `associations` array to define [associations](https://developers.hubspot.com/docs/guides/api/crm/associations/associations-v4) with other CRM records.
- **`hubspot-pp-cli calls-crm post-v3-objects-calls-batch-read-read`** - Read a batch of calls by internal ID, or unique property values
- **`hubspot-pp-cli calls-crm post-v3-objects-calls-batch-update-update`** - Update a batch of calls by internal ID, or unique property values
- **`hubspot-pp-cli calls-crm post-v3-objects-calls-batch-upsert-upsert`** - Create or update records identified by a unique property value as specified by the `idProperty` query param. `idProperty` query param refers to a property whose values are unique for the object.
- **`hubspot-pp-cli calls-crm post-v3-objects-calls-create`** - Create a call with the given properties and return a copy of the object, including the ID. Documentation and examples for creating standard calls is provided.
- **`hubspot-pp-cli calls-crm post-v3-objects-calls-search-do-search`** - Search for calls by filtering on properties, searching through associations, and sorting results. Learn more about [CRM search](https://developers.hubspot.com/docs/guides/api/crm/search#make-a-search-request).

### companies-crm

Manage crm

- **`hubspot-pp-cli companies-crm delete-v3-objects-companies-company-id-archive`** - Delete a company by ID. Deleted companies can be restored within 90 days of deletion. Learn more about [restoring records](https://knowledge.hubspot.com/records/restore-deleted-records).
- **`hubspot-pp-cli companies-crm get-v3-objects-companies-company-id-get-by-id`** - Retrieve a company by its ID (`companyId`) or by a unique property (`idProperty`). You can specify what is returned using the `properties` query parameter.
- **`hubspot-pp-cli companies-crm get-v3-objects-companies-get-page`** - Retrieve all companies, using query parameters to control the information that gets returned.
- **`hubspot-pp-cli companies-crm patch-v3-objects-companies-company-id-update`** - Update a company by ID (`companyId`) or unique property value (`idProperty`). Provided property values will be overwritten. Read-only and non-existent properties will result in an error. Properties values can be cleared by passing an empty string.
- **`hubspot-pp-cli companies-crm post-v3-objects-companies-batch-archive-archive`** - Delete a batch of companies by ID. Deleted companies can be restored within 90 days of deletion. Learn more about [restoring records](https://knowledge.hubspot.com/records/restore-deleted-records).
- **`hubspot-pp-cli companies-crm post-v3-objects-companies-batch-create-create`** - Create a batch of companies. The `inputs` array can contain a `properties` object to define property values for each company, along with an `associations` array to define [associations](https://developers.hubspot.com/docs/guides/api/crm/associations/associations-v4) with other CRM records.
- **`hubspot-pp-cli companies-crm post-v3-objects-companies-batch-read-read`** - Retrieve a batch of companies by ID (`companyId`) or by a unique property (`idProperty`). You can specify what is returned using the `properties` query parameter.
- **`hubspot-pp-cli companies-crm post-v3-objects-companies-batch-update-update`** - Update a batch of companies by ID.
- **`hubspot-pp-cli companies-crm post-v3-objects-companies-batch-upsert-upsert`** - Create or update companies identified by a unique property value as specified by the `idProperty` query parameter. `idProperty` query param refers to a property whose values are unique for the object.
- **`hubspot-pp-cli companies-crm post-v3-objects-companies-create`** - Create a single company. Include a `properties` object to define [property values](https://developers.hubspot.com/docs/guides/api/crm/properties) for the company, along with an `associations` array to define [associations](https://developers.hubspot.com/docs/guides/api/crm/associations/associations-v4) with other CRM records.
- **`hubspot-pp-cli companies-crm post-v3-objects-companies-merge-merge`** - Merge two company records. Learn more about [merging records](https://knowledge.hubspot.com/records/merge-records).
- **`hubspot-pp-cli companies-crm post-v3-objects-companies-search-do-search`** - Search for companies by filtering on properties, searching through associations, and sorting results. Learn more about [CRM search](https://developers.hubspot.com/docs/guides/api/crm/search#make-a-search-request).

### crm

Manage crm

- **`hubspot-pp-cli crm delete-v3-objects-contacts-contact-id-archive`** - Delete a contact by ID. Deleted contacts can be restored within 90 days of deletion. Learn more about the [data impacted by contact deletions](https://knowledge.hubspot.com/privacy-and-consent/understand-restorable-and-permanent-contact-deletions) and how to [restore archived records](https://knowledge.hubspot.com/records/restore-deleted-records).
- **`hubspot-pp-cli crm get-v3-objects-contacts-contact-id-get-by-id`** - Retrieve a contact by its ID (`contactId`) or by a unique property (`idProperty`). You can specify what is returned using the `properties` query parameter.
- **`hubspot-pp-cli crm get-v3-objects-contacts-get-page`** - Retrieve all contacts, using query parameters to specify the information that gets returned.
- **`hubspot-pp-cli crm patch-v3-objects-contacts-contact-id-update`** - Update an existing contact, identified by ID or email/unique property value. To identify a contact by ID, include the ID in the request URL path. To identify a contact by their email or other unique property, include the email/property value in the request URL path, and add the `idProperty` query parameter (`/crm/v3/objects/contacts/jon@website.com?idProperty=email`). Provided property values will be overwritten. Read-only and non-existent properties will result in an error. Properties values can be cleared by passing an empty string.
- **`hubspot-pp-cli crm post-v3-objects-contacts-batch-archive-archive`** - Archive a batch of contacts by ID. Archived contacts can be restored within 90 days of deletion. Learn more about the [data impacted by contact deletions](https://knowledge.hubspot.com/privacy-and-consent/understand-restorable-and-permanent-contact-deletions) and how to [restore archived records](https://knowledge.hubspot.com/records/restore-deleted-records).
- **`hubspot-pp-cli crm post-v3-objects-contacts-batch-create-create`** - Create a batch of contacts. The `inputs` array can contain a `properties` object to define property values for each record, along with an `associations` array to define [associations](https://developers.hubspot.com/docs/guides/api/crm/associations/associations-v4) with other CRM records.
- **`hubspot-pp-cli crm post-v3-objects-contacts-batch-read-read`** - Retrieve a batch of contacts by ID (`contactId`) or unique property value (`idProperty`).
- **`hubspot-pp-cli crm post-v3-objects-contacts-batch-update-update`** - Update a batch of contacts by ID (`contactId`) or unique property value (`idProperty`). Provided property values will be overwritten. Read-only and non-existent properties will result in an error. Properties values can be cleared by passing an empty string.
- **`hubspot-pp-cli crm post-v3-objects-contacts-batch-upsert-upsert`** - Upsert a batch of contacts. The `inputs` array can contain a `properties` object to define property values for each record.
- **`hubspot-pp-cli crm post-v3-objects-contacts-create`** - Create a single contact. Include a `properties` object to define [property values](https://developers.hubspot.com/docs/guides/api/crm/properties) for the contact, along with an `associations` array to define [associations](https://developers.hubspot.com/docs/guides/api/crm/associations/associations-v4) with other CRM records.
- **`hubspot-pp-cli crm post-v3-objects-contacts-gdpr-delete-purge`** - Permanently delete a contact and all associated content to follow GDPR. Use optional property `idProperty` set to `email` to identify contact by email address. If email address is not found, the email address will be added to a blocklist and prevent it from being used in the future. Learn more about [permanently deleting contacts](https://knowledge.hubspot.com/privacy-and-consent/how-do-i-perform-a-gdpr-delete-in-hubspot).
- **`hubspot-pp-cli crm post-v3-objects-contacts-merge-merge`** - Merge two contact records. Learn more about [merging records](https://knowledge.hubspot.com/records/merge-records).
- **`hubspot-pp-cli crm post-v3-objects-contacts-search-do-search`** - Search for contacts by filtering on properties, searching through associations, and sorting results. Learn more about [CRM search](https://developers.hubspot.com/docs/guides/api/crm/search#make-a-search-request).

### crm-lists-crm

Manage crm

- **`hubspot-pp-cli crm-lists-crm delete-v3-lists-folders-folder-id-v3-lists-folders-folder-id`** - Delete v3 lists folders folder id v3 lists folders folder id
- **`hubspot-pp-cli crm-lists-crm delete-v3-lists-list-id-memberships-v3-lists-list-id-memberships`** - Delete v3 lists list id memberships v3 lists list id memberships
- **`hubspot-pp-cli crm-lists-crm delete-v3-lists-list-id-schedule-conversion-v3-lists-list-id-schedule-conversion`** - Delete v3 lists list id schedule conversion v3 lists list id schedule conversion
- **`hubspot-pp-cli crm-lists-crm delete-v3-lists-list-id-v3-lists-list-id`** - Delete v3 lists list id v3 lists list id
- **`hubspot-pp-cli crm-lists-crm get-v3-lists-folders-v3-lists-folders`** - Get v3 lists folders v3 lists folders
- **`hubspot-pp-cli crm-lists-crm get-v3-lists-idmapping-v3-lists-idmapping`** - Get v3 lists idmapping v3 lists idmapping
- **`hubspot-pp-cli crm-lists-crm get-v3-lists-list-id-memberships-join-order-v3-lists-list-id-memberships-join-order`** - Get v3 lists list id memberships join order v3 lists list id memberships join order
- **`hubspot-pp-cli crm-lists-crm get-v3-lists-list-id-memberships-v3-lists-list-id-memberships`** - Get v3 lists list id memberships v3 lists list id memberships
- **`hubspot-pp-cli crm-lists-crm get-v3-lists-list-id-schedule-conversion-v3-lists-list-id-schedule-conversion`** - Get v3 lists list id schedule conversion v3 lists list id schedule conversion
- **`hubspot-pp-cli crm-lists-crm get-v3-lists-list-id-size-and-edits-history-between-v3-lists-list-id-size-and-edits-history-between`** - Get v3 lists list id size and edits history between v3 lists list id size and edits history between
- **`hubspot-pp-cli crm-lists-crm get-v3-lists-list-id-v3-lists-list-id`** - Get v3 lists list id v3 lists list id
- **`hubspot-pp-cli crm-lists-crm get-v3-lists-object-type-id-object-type-id-name-list-name-v3-lists-object-type-id-object-type-id-name-list-name`** - Retrieve a specific list by its name and object type ID. This endpoint allows you to fetch details about a list, including its properties and optionally its filters. It is useful for accessing list information based on specific criteria.
- **`hubspot-pp-cli crm-lists-crm get-v3-lists-records-object-type-id-record-id-memberships-v3-lists-records-object-type-id-record-id-memberships`** - Get v3 lists records object type id record id memberships v3 lists records object type id record id memberships
- **`hubspot-pp-cli crm-lists-crm get-v3-lists-v3-lists`** - Get v3 lists v3 lists
- **`hubspot-pp-cli crm-lists-crm post-v3-lists-folders-v3-lists-folders`** - Post v3 lists folders v3 lists folders
- **`hubspot-pp-cli crm-lists-crm post-v3-lists-idmapping-v3-lists-idmapping`** - Post v3 lists idmapping v3 lists idmapping
- **`hubspot-pp-cli crm-lists-crm post-v3-lists-records-memberships-batch-read-v3-lists-records-memberships-batch-read`** - Post v3 lists records memberships batch read v3 lists records memberships batch read
- **`hubspot-pp-cli crm-lists-crm post-v3-lists-search-v3-lists-search`** - Post v3 lists search v3 lists search
- **`hubspot-pp-cli crm-lists-crm post-v3-lists-v3-lists`** - Post v3 lists v3 lists
- **`hubspot-pp-cli crm-lists-crm put-v3-lists-folders-folder-id-move-new-parent-folder-id-v3-lists-folders-folder-id-move-new-parent-folder-id`** - Put v3 lists folders folder id move new parent folder id v3 lists folders folder id move new parent folder id
- **`hubspot-pp-cli crm-lists-crm put-v3-lists-folders-folder-id-rename-v3-lists-folders-folder-id-rename`** - Put v3 lists folders folder id rename v3 lists folders folder id rename
- **`hubspot-pp-cli crm-lists-crm put-v3-lists-folders-move-list-v3-lists-folders-move-list`** - Put v3 lists folders move list v3 lists folders move list
- **`hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-memberships-add-and-remove-v3-lists-list-id-memberships-add-and-remove`** - Put v3 lists list id memberships add and remove v3 lists list id memberships add and remove
- **`hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-memberships-add-from-source-list-id-v3-lists-list-id-memberships-add-from-source-list-id`** - Put v3 lists list id memberships add from source list id v3 lists list id memberships add from source list id
- **`hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-memberships-add-v3-lists-list-id-memberships-add`** - Put v3 lists list id memberships add v3 lists list id memberships add
- **`hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-memberships-remove-v3-lists-list-id-memberships-remove`** - Put v3 lists list id memberships remove v3 lists list id memberships remove
- **`hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-restore-v3-lists-list-id-restore`** - Put v3 lists list id restore v3 lists list id restore
- **`hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-schedule-conversion-v3-lists-list-id-schedule-conversion`** - Put v3 lists list id schedule conversion v3 lists list id schedule conversion
- **`hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-update-list-filters-v3-lists-list-id-update-list-filters`** - Put v3 lists list id update list filters v3 lists list id update list filters
- **`hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-update-list-name-v3-lists-list-id-update-list-name`** - Put v3 lists list id update list name v3 lists list id update list name

### crm-owners-crm

Manage crm

- **`hubspot-pp-cli crm-owners-crm get-v3-owners-owner-id-get-by-id`** - Retrieve details of a specific owner using either their 'id' or 'userId'.
- **`hubspot-pp-cli crm-owners-crm get-v3-owners-v3-owners`** - Get v3 owners v3 owners

### deals-crm

Manage crm

- **`hubspot-pp-cli deals-crm delete-v3-objects-0-3-deal-id-archive`** - Move an Object identified by `{dealId}` to the recycling bin.
- **`hubspot-pp-cli deals-crm get-v3-objects-0-3-deal-id-get-by-id`** - Read an Object identified by `{dealId}`. `{dealId}` refers to the internal object ID by default, or optionally any unique property value as specified by the `idProperty` query param.  Control what is returned via the `properties` query param.
- **`hubspot-pp-cli deals-crm get-v3-objects-0-3-get-page`** - Read a page of deals. Control what is returned via the `properties` query param.
- **`hubspot-pp-cli deals-crm patch-v3-objects-0-3-deal-id-update`** - Perform a partial update of an Object identified by `{dealId}`or optionally a unique property value as specified by the `idProperty` query param. `{dealId}` refers to the internal object ID by default, and the `idProperty` query param refers to a property whose values are unique for the object. Provided property values will be overwritten. Read-only and non-existent properties will result in an error. Properties values can be cleared by passing an empty string.
- **`hubspot-pp-cli deals-crm post-v3-objects-0-3-batch-archive-archive`** - Archive multiple deals using their IDs.
- **`hubspot-pp-cli deals-crm post-v3-objects-0-3-batch-create-create`** - Create multiple deals in a single request.
- **`hubspot-pp-cli deals-crm post-v3-objects-0-3-batch-read-read`** - Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value property.
- **`hubspot-pp-cli deals-crm post-v3-objects-0-3-batch-update-update`** - Update multiple deals using their internal IDs or unique property values.
- **`hubspot-pp-cli deals-crm post-v3-objects-0-3-batch-upsert-upsert`** - Create or update records identified by a unique property value as specified by the `idProperty` query param. `idProperty` query param refers to a property whose values are unique for the object.
- **`hubspot-pp-cli deals-crm post-v3-objects-0-3-create`** - Create a deal with the given properties and return a copy of the object, including the ID. Documentation and examples for creating standard deals is provided.
- **`hubspot-pp-cli deals-crm post-v3-objects-0-3-merge-merge`** - Combine two deals of the same type into a single deal.
- **`hubspot-pp-cli deals-crm post-v3-objects-0-3-search-do-search`** - Search for deals using various filters and criteria to retrieve specific records.

### emails-crm

Manage crm

- **`hubspot-pp-cli emails-crm delete-v3-objects-emails-email-id-archive`** - Move an Object identified by `{emailId}` to the recycling bin.
- **`hubspot-pp-cli emails-crm get-v3-objects-emails-email-id-get-by-id`** - Read an Object identified by `{emailId}`. `{emailId}` refers to the internal object ID by default, or optionally any unique property value as specified by the `idProperty` query param.  Control what is returned via the `properties` query param.
- **`hubspot-pp-cli emails-crm get-v3-objects-emails-get-page`** - Read a page of emails. Control what is returned via the `properties` query param.
- **`hubspot-pp-cli emails-crm patch-v3-objects-emails-email-id-update`** - Perform a partial update of an Object identified by `{emailId}`or optionally a unique property value as specified by the `idProperty` query param. `{emailId}` refers to the internal object ID by default, and the `idProperty` query param refers to a property whose values are unique for the object. Provided property values will be overwritten. Read-only and non-existent properties will result in an error. Properties values can be cleared by passing an empty string.
- **`hubspot-pp-cli emails-crm post-v3-objects-emails-batch-archive-archive`** - Archive a batch of emails identified by their IDs.
- **`hubspot-pp-cli emails-crm post-v3-objects-emails-batch-create-create`** - Create a batch of emails with specified properties and return the created objects.
- **`hubspot-pp-cli emails-crm post-v3-objects-emails-batch-read-read`** - Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value property.
- **`hubspot-pp-cli emails-crm post-v3-objects-emails-batch-update-update`** - Update a batch of emails using their internal IDs or unique property values.
- **`hubspot-pp-cli emails-crm post-v3-objects-emails-batch-upsert-upsert`** - Create or update records identified by a unique property value as specified by the `idProperty` query param. `idProperty` query param refers to a property whose values are unique for the object.
- **`hubspot-pp-cli emails-crm post-v3-objects-emails-create`** - Create a email with the given properties and return a copy of the object, including the ID. Documentation and examples for creating standard emails is provided.
- **`hubspot-pp-cli emails-crm post-v3-objects-emails-search-do-search`** - Perform a search for emails based on the provided query parameters and return matching results.

### groups

Manage groups

- **`hubspot-pp-cli groups delete-crm-v3-properties-object-type-name-archive`** - Move a property group identified by {groupName} to the recycling bin.
- **`hubspot-pp-cli groups get-crm-v3-properties-object-type-get-all`** - Read all existing property groups for the specified object type and HubSpot account.
- **`hubspot-pp-cli groups get-crm-v3-properties-object-type-name-get-by-name`** - Read a property group identified by {groupName}.
- **`hubspot-pp-cli groups patch-crm-v3-properties-object-type-name-update`** - Perform a partial update of a property group identified by {groupName}. Provided fields will be overwritten.
- **`hubspot-pp-cli groups post-crm-v3-properties-object-type-create`** - Create and return a copy of a new property group.

### leads-crm

Manage crm

- **`hubspot-pp-cli leads-crm delete-v3-objects-leads-leads-id-archive`** - Move an Object identified by `{leadsId}` to the recycling bin.
- **`hubspot-pp-cli leads-crm get-v3-objects-leads-get-page`** - Read a page of leads. Control what is returned via the `properties` query param.
- **`hubspot-pp-cli leads-crm get-v3-objects-leads-leads-id-get-by-id`** - Read an Object identified by `{leadsId}`. `{leadsId}` refers to the internal object ID by default, or optionally any unique property value as specified by the `idProperty` query param.  Control what is returned via the `properties` query param.
- **`hubspot-pp-cli leads-crm patch-v3-objects-leads-leads-id-update`** - Perform a partial update of an Object identified by `{leadsId}`or optionally a unique property value as specified by the `idProperty` query param. `{leadsId}` refers to the internal object ID by default, and the `idProperty` query param refers to a property whose values are unique for the object. Provided property values will be overwritten. Read-only and non-existent properties will result in an error. Properties values can be cleared by passing an empty string.
- **`hubspot-pp-cli leads-crm post-v3-objects-leads-batch-archive-archive`** - Archive multiple leads by their IDs in a single request, moving them to the recycling bin.
- **`hubspot-pp-cli leads-crm post-v3-objects-leads-batch-create-create`** - Create multiple lead records in a single request by providing a batch of lead data. This endpoint allows for efficient creation of leads by processing them together, which can be useful for syncing data from other systems or importing large datasets.
- **`hubspot-pp-cli leads-crm post-v3-objects-leads-batch-read-read`** - Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value property.
- **`hubspot-pp-cli leads-crm post-v3-objects-leads-batch-update-update`** - Update multiple lead records using their internal IDs or unique property values. This endpoint allows batch processing of updates, where each lead's properties can be modified based on the provided input. Ensure that the properties being updated exist on the lead objects to avoid errors.
- **`hubspot-pp-cli leads-crm post-v3-objects-leads-batch-upsert-upsert`** - Create or update records identified by a unique property value as specified by the `idProperty` query param. `idProperty` query param refers to a property whose values are unique for the object.
- **`hubspot-pp-cli leads-crm post-v3-objects-leads-create`** - Create a lead with the given properties and return a copy of the object, including the ID. Documentation and examples for creating standard leads is provided.
- **`hubspot-pp-cli leads-crm post-v3-objects-leads-search-do-search`** - Perform a search for leads based on the provided filter groups, properties, and sorting options. The request allows for pagination and can return up to 200 results per page.

### meetings-crm

Manage crm

- **`hubspot-pp-cli meetings-crm delete-v3-objects-meetings-meeting-id-archive`** - Move an Object identified by `{meetingId}` to the recycling bin.
- **`hubspot-pp-cli meetings-crm get-v3-objects-meetings-get-page`** - Read a page of meetings. Control what is returned via the `properties` query param.
- **`hubspot-pp-cli meetings-crm get-v3-objects-meetings-meeting-id-get-by-id`** - Read an Object identified by `{meetingId}`. `{meetingId}` refers to the internal object ID by default, or optionally any unique property value as specified by the `idProperty` query param.  Control what is returned via the `properties` query param.
- **`hubspot-pp-cli meetings-crm patch-v3-objects-meetings-meeting-id-update`** - Perform a partial update of an Object identified by `{meetingId}`or optionally a unique property value as specified by the `idProperty` query param. `{meetingId}` refers to the internal object ID by default, and the `idProperty` query param refers to a property whose values are unique for the object. Provided property values will be overwritten. Read-only and non-existent properties will result in an error. Properties values can be cleared by passing an empty string.
- **`hubspot-pp-cli meetings-crm post-v3-objects-meetings-batch-archive-archive`** - Archive a batch of meetings by ID
- **`hubspot-pp-cli meetings-crm post-v3-objects-meetings-batch-create-create`** - Create a batch of meetings
- **`hubspot-pp-cli meetings-crm post-v3-objects-meetings-batch-read-read`** - Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value property.
- **`hubspot-pp-cli meetings-crm post-v3-objects-meetings-batch-update-update`** - Update a batch of meetings by internal ID, or unique property values
- **`hubspot-pp-cli meetings-crm post-v3-objects-meetings-batch-upsert-upsert`** - Create or update records identified by a unique property value as specified by the `idProperty` query param. `idProperty` query param refers to a property whose values are unique for the object.
- **`hubspot-pp-cli meetings-crm post-v3-objects-meetings-create`** - Create a meeting with the given properties and return a copy of the object, including the ID. Documentation and examples for creating standard meetings is provided.
- **`hubspot-pp-cli meetings-crm post-v3-objects-meetings-search-do-search`** - Post v3 objects meetings search do search

### notes-crm

Manage crm

- **`hubspot-pp-cli notes-crm delete-v3-objects-notes-note-id-archive`** - Move an Object identified by `{noteId}` to the recycling bin.
- **`hubspot-pp-cli notes-crm get-v3-objects-notes-get-page`** - Read a page of notes. Control what is returned via the `properties` query param.
- **`hubspot-pp-cli notes-crm get-v3-objects-notes-note-id-get-by-id`** - Read an Object identified by `{noteId}`. `{noteId}` refers to the internal object ID by default, or optionally any unique property value as specified by the `idProperty` query param.  Control what is returned via the `properties` query param.
- **`hubspot-pp-cli notes-crm patch-v3-objects-notes-note-id-update`** - Perform a partial update of an Object identified by `{noteId}`or optionally a unique property value as specified by the `idProperty` query param. `{noteId}` refers to the internal object ID by default, and the `idProperty` query param refers to a property whose values are unique for the object. Provided property values will be overwritten. Read-only and non-existent properties will result in an error. Properties values can be cleared by passing an empty string.
- **`hubspot-pp-cli notes-crm post-v3-objects-notes-batch-archive-archive`** - Archive multiple notes by their IDs in a single request. This operation moves the specified notes to the recycling bin, making them inaccessible from regular queries.
- **`hubspot-pp-cli notes-crm post-v3-objects-notes-batch-create-create`** - Create multiple notes in a single request by providing the necessary properties for each note. This operation returns the created notes with their unique identifiers.
- **`hubspot-pp-cli notes-crm post-v3-objects-notes-batch-read-read`** - Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value property.
- **`hubspot-pp-cli notes-crm post-v3-objects-notes-batch-update-update`** - Update multiple notes using their internal IDs or unique property values. This operation allows you to modify the properties of several notes in a single request, streamlining the process of managing note data in bulk.
- **`hubspot-pp-cli notes-crm post-v3-objects-notes-batch-upsert-upsert`** - Create or update records identified by a unique property value as specified by the `idProperty` query param. `idProperty` query param refers to a property whose values are unique for the object.
- **`hubspot-pp-cli notes-crm post-v3-objects-notes-create`** - Create a note with the given properties and return a copy of the object, including the ID. Documentation and examples for creating standard notes is provided.
- **`hubspot-pp-cli notes-crm post-v3-objects-notes-search-do-search`** - Execute a search for notes using filters, sorting options, and other query parameters to refine the results. This endpoint allows for complex queries to locate specific notes within the CRM system.

### pipelines-crm

Manage crm

- **`hubspot-pp-cli pipelines-crm delete-v3-pipelines-object-type-pipeline-id-archive`** - Delete a pipeline
- **`hubspot-pp-cli pipelines-crm delete-v3-pipelines-object-type-pipeline-id-stages-stage-id-archive`** - Delete a pipeline stage
- **`hubspot-pp-cli pipelines-crm get-v3-pipelines-object-type-get-all`** - Return all pipelines for the object type specified by `{objectType}`.
- **`hubspot-pp-cli pipelines-crm get-v3-pipelines-object-type-pipeline-id-audit-get-audit`** - Return a reverse chronological list of all mutations that have occurred on the pipeline identified by `{pipelineId}`.
- **`hubspot-pp-cli pipelines-crm get-v3-pipelines-object-type-pipeline-id-get-by-id`** - Return a single pipeline object identified by its unique `{pipelineId}`.
- **`hubspot-pp-cli pipelines-crm get-v3-pipelines-object-type-pipeline-id-stages-get-all`** - Return all the stages associated with the pipeline identified by `{pipelineId}`.
- **`hubspot-pp-cli pipelines-crm get-v3-pipelines-object-type-pipeline-id-stages-stage-id-audit-get-audit`** - Return a reverse chronological list of all mutations that have occurred on the pipeline stage identified by `{stageId}`.
- **`hubspot-pp-cli pipelines-crm get-v3-pipelines-object-type-pipeline-id-stages-stage-id-get-by-id`** - Return a pipeline stage by ID
- **`hubspot-pp-cli pipelines-crm patch-v3-pipelines-object-type-pipeline-id-stages-stage-id-update`** - Patch v3 pipelines object type pipeline id stages stage id update
- **`hubspot-pp-cli pipelines-crm patch-v3-pipelines-object-type-pipeline-id-update`** - Perform a partial update of the pipeline identified by `{pipelineId}`. The updated pipeline will be returned in the response.
- **`hubspot-pp-cli pipelines-crm post-v3-pipelines-object-type-create`** - Create a new pipeline with the provided property values. The entire pipeline object, including its unique ID, will be returned in the response.
- **`hubspot-pp-cli pipelines-crm post-v3-pipelines-object-type-pipeline-id-stages-create`** - Create a pipeline stage
- **`hubspot-pp-cli pipelines-crm put-v3-pipelines-object-type-pipeline-id-replace`** - Replace a pipeline
- **`hubspot-pp-cli pipelines-crm put-v3-pipelines-object-type-pipeline-id-stages-stage-id-replace`** - Replace all the properties of an existing pipeline stage with the values provided. The updated stage will be returned in the response.

### properties-crm

Manage crm

- **`hubspot-pp-cli properties-crm delete-v3-properties-object-type-property-name-archive`** - Move a property identified by {propertyName} to the recycling bin.
- **`hubspot-pp-cli properties-crm get-v3-properties-object-type-get-all`** - Read all existing properties for the specified object type and HubSpot account.
- **`hubspot-pp-cli properties-crm get-v3-properties-object-type-property-name-get-by-name`** - Read a property identified by {propertyName}.
- **`hubspot-pp-cli properties-crm patch-v3-properties-object-type-property-name-update`** - Perform a partial update of a property identified by { propertyName }. Provided fields will be overwritten.
- **`hubspot-pp-cli properties-crm post-v3-properties-object-type-create`** - Create and return a copy of a new property for the specified object type.

### tasks-crm

Manage crm

- **`hubspot-pp-cli tasks-crm delete-v3-objects-tasks-task-id-archive`** - Move an Object identified by `{taskId}` to the recycling bin.
- **`hubspot-pp-cli tasks-crm get-v3-objects-tasks-get-page`** - Read a page of tasks. Control what is returned via the `properties` query param.
- **`hubspot-pp-cli tasks-crm get-v3-objects-tasks-task-id-get-by-id`** - Read an Object identified by `{taskId}`. `{taskId}` refers to the internal object ID by default, or optionally any unique property value as specified by the `idProperty` query param.  Control what is returned via the `properties` query param.
- **`hubspot-pp-cli tasks-crm patch-v3-objects-tasks-task-id-update`** - Perform a partial update of an Object identified by `{taskId}`or optionally a unique property value as specified by the `idProperty` query param. `{taskId}` refers to the internal object ID by default, and the `idProperty` query param refers to a property whose values are unique for the object. Provided property values will be overwritten. Read-only and non-existent properties will result in an error. Properties values can be cleared by passing an empty string.
- **`hubspot-pp-cli tasks-crm post-v3-objects-tasks-batch-archive-archive`** - Archive a batch of tasks by their IDs, moving them to the recycling bin. This operation requires a list of task IDs to be provided in the request body.
- **`hubspot-pp-cli tasks-crm post-v3-objects-tasks-batch-create-create`** - Create multiple tasks in a single request by providing a batch of task properties and associations. This endpoint allows for efficient task creation by processing multiple tasks together.
- **`hubspot-pp-cli tasks-crm post-v3-objects-tasks-batch-read-read`** - Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value property.
- **`hubspot-pp-cli tasks-crm post-v3-objects-tasks-batch-update-update`** - Update multiple tasks in a single request using their internal IDs or unique property values. This operation allows you to modify the properties of each task in the batch, ensuring efficient management of task data.
- **`hubspot-pp-cli tasks-crm post-v3-objects-tasks-batch-upsert-upsert`** - Create or update records identified by a unique property value as specified by the `idProperty` query param. `idProperty` query param refers to a property whose values are unique for the object.
- **`hubspot-pp-cli tasks-crm post-v3-objects-tasks-create`** - Create a task with the given properties and return a copy of the object, including the ID. Documentation and examples for creating standard tasks is provided.
- **`hubspot-pp-cli tasks-crm post-v3-objects-tasks-search-do-search`** - Execute a search for tasks based on the provided criteria, including filters, properties, and sorting options. This allows for retrieving tasks that match specific conditions or property values.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
hubspot-pp-cli associations-crm delete-v4-objects-object-type-object-id-associations-to-object-type-to-object-id-archive mock-value mock-value mock-value mock-value

# JSON for scripting and agents
hubspot-pp-cli associations-crm delete-v4-objects-object-type-object-id-associations-to-object-type-to-object-id-archive mock-value mock-value mock-value mock-value --json

# Filter to specific fields
hubspot-pp-cli associations-crm delete-v4-objects-object-type-object-id-associations-to-object-type-to-object-id-archive mock-value mock-value mock-value mock-value --json --select id,name,status

# Dry run — show the request without sending
hubspot-pp-cli associations-crm delete-v4-objects-object-type-object-id-associations-to-object-type-to-object-id-archive mock-value mock-value mock-value mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
hubspot-pp-cli associations-crm delete-v4-objects-object-type-object-id-associations-to-object-type-to-object-id-archive mock-value mock-value mock-value mock-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-hubspot -g
```

Then invoke `/pp-hubspot <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-mcp@latest
```

Then register it:

```bash
claude mcp add hubspot hubspot-pp-mcp -e HUBSPOT_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hubspot-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `HUBSPOT_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "hubspot": {
      "command": "hubspot-pp-mcp",
      "env": {
        "HUBSPOT_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
hubspot-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/hubspot-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `HUBSPOT_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `hubspot-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $HUBSPOT_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports `INVALID_AUTHENTICATION`** — Confirm `HUBSPOT_TOKEN` is exported and starts with `pat-` (private app token). OAuth tokens are not supported.
- **`stale-leads` returns empty after sync** — The `--stage` flag matches HubSpot internal names like `leadqualified`, not display labels. List the real stage names in your portal with `hubspot-pp-cli sql "SELECT DISTINCT json_extract(data,'$.properties.lifecyclestage') AS stage FROM crm WHERE stage IS NOT NULL"`.
- **Sync stops mid-page with `RATE_LIMIT_EXCEEDED`** — The built-in adaptive limiter backs off automatically. To stay below the burst ceiling on a low-tier portal, rerun with `--concurrency 2` instead of the default `4`.
- **`recent-intake` shows contacts but no UTM properties** — Inspect the synced properties with `hubspot-pp-cli sql "SELECT json_extract(data,'$.properties') FROM crm LIMIT 1"`; if `hs_analytics_source` and `utm_*` are missing, your private app token does not have the relevant property-read scope.
- **`dedup` clusters look wrong** — Phone normalization assumes US/CA E.164. Pass `--key email` or `--key domain` to switch the cluster key, and re-run `sync --resources contacts --full` if the cache was built before this version.
- **A `<resource>-crm post-…-create` / `patch-…-update` / `delete-…-archive` command returns `403`** — Expected. This CLI is designed for read-only pre-sale workflows; mutate via the official HubSpot UI or MCP. Grant write scopes on your private app token only when you understand the blast radius.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**shinzo-labs/hubspot-mcp**](https://github.com/shinzo-labs/hubspot-mcp) — TypeScript
- [**dipankar/hubspot-cli**](https://github.com/dipankar/hubspot-cli) — Go
- [**baryhuang/mcp-hubspot**](https://github.com/baryhuang/mcp-hubspot) — Python
- [**lkm1developer/hubspot-mcp-server**](https://github.com/lkm1developer/hubspot-mcp-server) — TypeScript
- [**open-cli-collective/hubspot-cli**](https://github.com/open-cli-collective/hubspot-cli) — TypeScript
- [**silverbackstudio/hubspot-tools**](https://github.com/silverbackstudio/hubspot-tools) — JavaScript
- [**HubSpot/hubspot-api-nodejs**](https://github.com/HubSpot/hubspot-api-nodejs) — TypeScript
- [**HubSpot/hubspot-api-python**](https://github.com/HubSpot/hubspot-api-python) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
