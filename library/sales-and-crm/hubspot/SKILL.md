---
name: pp-hubspot
description: "Every HubSpot Sales Hub feature, plus offline cross-object queries, property-change-history reporting Trigger phrases: `find meetings ever scheduled`, `monthly meeting outcome report`, `hubspot stale leads`, `who do I call today hubspot`, `engagements timeline for this contact`, `use hubspot-pp-cli`, `run hubspot`."
author: "Damien Stevens"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - hubspot-pp-cli
    install:
      - kind: go
        bins: [hubspot-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-cli
---

# HubSpot — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `hubspot-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install hubspot --cli-only
   ```
2. Verify: `hubspot-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

HubSpot's own CLI (`hs`) only covers CMS — there has never been a sales/CRM CLI from HubSpot itself. This one mirrors your CRM into local SQLite so commands like `nurture-mine`, `stale deals`, `owner-load`, and `pipeline-health` answer cross-table questions instantly and offline. New in this reprint: `sync --with-history` persists per-property snapshots into a shared property-history table, and `meetings ever-had` / `meetings status-report` answer questions HubSpot's standard search API physically cannot — 'every meeting that was EVER status X in month Y, even after it flipped.'

## When to Use This CLI

Use hubspot-pp-cli whenever an agent or human needs Sales Hub data without paying for a live API round trip per query. It is the right pick for nurture loops, pipeline-health dashboards, stale-prospect detection, bulk property updates from CSV, monthly property-history reports (`meetings status-report`, `meetings ever-had`), and any cross-object join (engagements × deals × owners) that the HubSpot web UI cannot answer in one screen. ANTI-TRIGGERS — do NOT use this CLI for: HubSpot CMS / themes / serverless functions (use the official `hs` CLI), Marketing emails / campaigns / forms / ads, Conversations / Inbox / Support workflows, Commerce Hub (orders / invoices / subscriptions / payments / carts / contracts).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`stale`** — Find contacts or deals with no engagement in N days, scoped by owner or pipeline stage — instantly, offline.

  _Use this when the user asks 'what's gone cold' — works after one sync, no API quota burn._

  ```bash
  hubspot-pp-cli stale deals --days 21 --owner me --json
  ```
- **`owner-load`** — Open deals per rep per pipeline stage with $ totals, count, and oldest-deal age — the Monday-morning sales-lead report.

  _Pick this over per-rep API calls when surfacing 'who has too many in-flight deals' or 'where is rep capacity hot'._

  ```bash
  hubspot-pp-cli owner-load --pipeline default --json
  ```
- **`pipeline-health`** — Per-stage rollup of count, $ total, $ at risk (idle deals near their close date), and the oldest-stuck deal — one query for a sales-ops dashboard. Closed Lost weighted at probability 0, Closed Won at 1.0.

  _Use this for forecast-vs-reality checks before a pipeline review._

  ```bash
  hubspot-pp-cli pipeline-health default --idle-days 14 --json
  ```
- **`nurture queue`** — Ranked 'who to contact today' list scored by stale-days × deal amount × stage probability, with the rationale exposed as columns.

  _Reach for this in the nurture skill or any daily-touch-list loop where an agent needs a priority order with reasons attached._

  ```bash
  hubspot-pp-cli nurture queue --owner me --top 20 --agent
  ```
- **`deals top`** — Composite-ranked top-N deals by (signal × amount × stage-probability × inverse-days-since-contact) with the score breakdown exposed as columns.

  _Pick this for sales-lead Monday reviews or when an agent needs the top opportunities ranked by signal-weighted score, not raw amount._

  ```bash
  hubspot-pp-cli deals top --top 5 --pipeline default --json
  ```

### Property history & audit
- **`meetings history`** — Show the full timeline of property changes for a single meeting (outcome, title, owner, custom fields) — when each value was set, by whom, and from what source.

  _Reach for this when investigating 'when did this meeting flip from Scheduled to No Show, and who changed it' — the audit-trail question the HubSpot UI buries inside per-property timelines._

  ```bash
  hubspot-pp-cli meetings history 53612340987612345 --json
  ```
- **`meetings ever-had`** — Find every meeting whose given property was EVER set to a given value within a date range — even if it has since changed.

  _Pick this when a customer or audit asks 'every meeting that was at some point in status X during month Y' — the only path through the property-history snapshot table._

  ```bash
  hubspot-pp-cli meetings ever-had --property hs_meeting_outcome --value Scheduled --from 2026-04-01 --to 2026-04-30 --json
  ```
- **`meetings status-report`** — Composes the meetings ever-had query into the canonical monthly-report shape: every meeting that touched the given status in the given month, with owner, title, current status, and the timestamp of the original status set.

  _Use this once per month per customer report — one command replaces a Python pull + a HubSpot export + a manual cross-reference._

  ```bash
  hubspot-pp-cli meetings status-report --status scheduled --month 2026-04 --csv
  ```
- **`sync --with-history`** — Opt-in sync flag that requests propertiesWithHistory for the named properties on meetings (and on deals, contacts, companies when scoped to those). Persists per-property snapshots into the shared hubspot_property_history table.

  _Reach for this when you need property change history retained. Without --with-history sync only captures current values; with it, every read after that point can answer 'was this property ever value X' questions._

  ```bash
  hubspot-pp-cli sync --resources hubspot-meetings-crm --with-history hs_meeting_outcome,hs_meeting_title,hubspot_owner_id
  ```

### Cross-object intelligence
- **`engagements of`** — Unified chronological timeline of every call, email, meeting, note, and task touching a contact, deal, or company.

  _Use this when an agent or human asks 'what touched this prospect/deal' — one command instead of five paginated API calls._

  ```bash
  hubspot-pp-cli engagements of contact:12345 --since 30d --json
  ```
- **`notes signals`** — Scan note bodies for buying / lost signals (meeting scheduled, budget approved, no response, competitor chosen) and emit per-deal signal counts with the source note id.

  _Use this when summarizing what's heating up or cooling off in the pipeline without re-reading every note._

  ```bash
  hubspot-pp-cli notes signals --pipeline default --since 30d --json
  ```
- **`since`** — What changed across contacts, deals, and engagements since a given timestamp — agent-friendly cross-object delta.

  _Reach for this in agent loops that need to react to CRM activity since the last run without paginating every collection._

  ```bash
  hubspot-pp-cli since 24h --types deals,engagements --owner me --json
  ```

### Bulk operations
- **`nurture-mine`** — Surface the contacts assigned to you that have gone cold but still have open deals — the daily 'who do I call' list, computed across local SQLite.

  _Reach for this when an agent or human needs the daily Titans-style nurture queue without round-tripping HubSpot for every prospect._

  ```bash
  hubspot-pp-cli nurture-mine --owner me --stale-days 14 --agent
  ```
- **`contacts bulk-update`** — Apply a CSV of property changes to many contacts at once, pre-validating each row against HubSpot's property schema (types, picklists) before any mutation.

  _Pick this over the HubSpot UI when sales-ops needs to update hundreds of contacts and demands a pre-flight error report rather than silent failures._

  ```bash
  hubspot-pp-cli contacts bulk-update --from-csv people.csv --map email=Email,lifecyclestage=Stage --dry-run
  ```

## Command Reference

**batch** — Manage batch

- `hubspot-pp-cli batch post-crm-v3-objects-object-type-archive-archive` — Archive a batch of objects by ID
- `hubspot-pp-cli batch post-crm-v3-objects-object-type-create-create` — Create a batch of objects
- `hubspot-pp-cli batch post-crm-v3-objects-object-type-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value
- `hubspot-pp-cli batch post-crm-v3-objects-object-type-update-update` — Update a batch of objects by internal ID, or unique property values
- `hubspot-pp-cli batch post-crm-v3-objects-object-type-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param.

**crm** — Manage crm

- `hubspot-pp-cli crm delete-v4-objects-object-type-object-id-associations-to-object-type-to-object-id-archive` — deletes all associations between two records.
- `hubspot-pp-cli crm get-v4-objects-object-type-object-id-associations-to-object-type-get-page` — Retrieve all associations between a specific record and an object type. Limit 500 per call.
- `hubspot-pp-cli crm post-v4-associations-from-object-type-to-object-type-batch-archive-archive` — Batch delete associations for objects
- `hubspot-pp-cli crm post-v4-associations-from-object-type-to-object-type-batch-associate-default-create-default` — Create the default (most generic) association type between two object types
- `hubspot-pp-cli crm post-v4-associations-from-object-type-to-object-type-batch-create-create` — Batch create associations for objects
- `hubspot-pp-cli crm post-v4-associations-from-object-type-to-object-type-batch-labels-archive-archive-labels` — Batch delete specific association labels for objects.
- `hubspot-pp-cli crm post-v4-associations-from-object-type-to-object-type-batch-read-get-page` — Batch read associations for objects to specific object type.
- `hubspot-pp-cli crm post-v4-associations-usage-high-usage-report-user-id-request` — Requests a report of all objects in the portal which have a high usage of associations
- `hubspot-pp-cli crm put-v4-objects-from-object-type-from-object-id-associations-default-to-object-type-to-object-id-create-default` — Create the default (most generic) association type between two object types
- `hubspot-pp-cli crm put-v4-objects-object-type-object-id-associations-to-object-type-to-object-id-create` — Set association labels between two records.

**groups** — Manage groups

- `hubspot-pp-cli groups delete-crm-v3-properties-object-type-name-archive` — Move a property group identified by {groupName} to the recycling bin.
- `hubspot-pp-cli groups get-crm-v3-properties-object-type-get-all` — Read all existing property groups for the specified object type and HubSpot account.
- `hubspot-pp-cli groups get-crm-v3-properties-object-type-name-get-by-name` — Read a property group identified by {groupName}.
- `hubspot-pp-cli groups patch-crm-v3-properties-object-type-name-update` — Perform a partial update of a property group identified by {groupName}. Provided fields will be overwritten.
- `hubspot-pp-cli groups post-crm-v3-properties-object-type-create` — Create and return a copy of a new property group.

**hubspot-calls-crm** — Manage hubspot calls crm

- `hubspot-pp-cli hubspot-calls-crm delete-v3-objects-calls-call-id-archive` — Move an Object identified by `{callId}` to the recycling bin.
- `hubspot-pp-cli hubspot-calls-crm get-v3-objects-calls-call-id-get-by-id` — Read an Object identified by `{callId}`.
- `hubspot-pp-cli hubspot-calls-crm get-v3-objects-calls-get-page` — Read a page of calls. Control what is returned via the `properties` query param.
- `hubspot-pp-cli hubspot-calls-crm patch-v3-objects-calls-call-id-update` — Perform a partial update of an Object identified by `{callId}`or optionally a unique property value as specified by the
- `hubspot-pp-cli hubspot-calls-crm post-v3-objects-calls-batch-archive-archive` — Archive a batch of calls by ID.
- `hubspot-pp-cli hubspot-calls-crm post-v3-objects-calls-batch-create-create` — Create a batch of calls.
- `hubspot-pp-cli hubspot-calls-crm post-v3-objects-calls-batch-read-read` — Read a batch of calls by internal ID, or unique property values
- `hubspot-pp-cli hubspot-calls-crm post-v3-objects-calls-batch-update-update` — Update a batch of calls by internal ID, or unique property values
- `hubspot-pp-cli hubspot-calls-crm post-v3-objects-calls-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param.
- `hubspot-pp-cli hubspot-calls-crm post-v3-objects-calls-create` — Create a call with the given properties and return a copy of the object, including the ID.
- `hubspot-pp-cli hubspot-calls-crm post-v3-objects-calls-search-do-search` — Search for calls by filtering on properties, searching through associations, and sorting results.

**hubspot-companies-crm** — Manage hubspot companies crm

- `hubspot-pp-cli hubspot-companies-crm delete-v3-objects-companies-company-id-archive` — Delete a company by ID. Deleted companies can be restored within 90 days of deletion.
- `hubspot-pp-cli hubspot-companies-crm get-v3-objects-companies-company-id-get-by-id` — Retrieve a company by its ID (`companyId`) or by a unique property (`idProperty`).
- `hubspot-pp-cli hubspot-companies-crm get-v3-objects-companies-get-page` — Retrieve all companies, using query parameters to control the information that gets returned.
- `hubspot-pp-cli hubspot-companies-crm patch-v3-objects-companies-company-id-update` — Update a company by ID (`companyId`) or unique property value (`idProperty`).
- `hubspot-pp-cli hubspot-companies-crm post-v3-objects-companies-batch-archive-archive` — Delete a batch of companies by ID. Deleted companies can be restored within 90 days of deletion.
- `hubspot-pp-cli hubspot-companies-crm post-v3-objects-companies-batch-create-create` — Create a batch of companies.
- `hubspot-pp-cli hubspot-companies-crm post-v3-objects-companies-batch-read-read` — Retrieve a batch of companies by ID (`companyId`) or by a unique property (`idProperty`).
- `hubspot-pp-cli hubspot-companies-crm post-v3-objects-companies-batch-update-update` — Update a batch of companies by ID.
- `hubspot-pp-cli hubspot-companies-crm post-v3-objects-companies-batch-upsert-upsert` — Create or update companies identified by a unique property value as specified by the `idProperty` query parameter.
- `hubspot-pp-cli hubspot-companies-crm post-v3-objects-companies-create` — Create a single company. Include a `properties` object to define [property values](https://developers.hubspot.
- `hubspot-pp-cli hubspot-companies-crm post-v3-objects-companies-merge-merge` — Merge two company records. Learn more about [merging records](https://knowledge.hubspot.com/records/merge-records).
- `hubspot-pp-cli hubspot-companies-crm post-v3-objects-companies-search-do-search` — Search for companies by filtering on properties, searching through associations, and sorting results.

**hubspot-contacts-crm** — Manage hubspot contacts crm

- `hubspot-pp-cli hubspot-contacts-crm delete-v3-objects-contacts-contact-id` — Move an Object identified by `{contactId}` to the recycling bin.
- `hubspot-pp-cli hubspot-contacts-crm get-v3-objects-contacts` — Read a page of contacts. Control what is returned via the `properties` query param.
- `hubspot-pp-cli hubspot-contacts-crm get-v3-objects-contacts-contact-id` — Read an Object identified by `{contactId}`.
- `hubspot-pp-cli hubspot-contacts-crm patch-v3-objects-contacts-contact-id` — Perform a partial update of an Object identified by `{contactId}`.
- `hubspot-pp-cli hubspot-contacts-crm post-v3-objects-contacts` — Create a contact with the given properties and return a copy of the object, including the ID.
- `hubspot-pp-cli hubspot-contacts-crm post-v3-objects-contacts-batch-archive` — Archive a batch of contacts by ID
- `hubspot-pp-cli hubspot-contacts-crm post-v3-objects-contacts-batch-create` — Create a batch of contacts
- `hubspot-pp-cli hubspot-contacts-crm post-v3-objects-contacts-batch-read` — Read a batch of contacts by internal ID, or unique property values
- `hubspot-pp-cli hubspot-contacts-crm post-v3-objects-contacts-batch-update` — Update a batch of contacts
- `hubspot-pp-cli hubspot-contacts-crm post-v3-objects-contacts-gdpr-delete` — Permanently delete a contact and all associated content to follow GDPR.
- `hubspot-pp-cli hubspot-contacts-crm post-v3-objects-contacts-merge` — Merge two contacts with same type
- `hubspot-pp-cli hubspot-contacts-crm post-v3-objects-contacts-search` — Post v3 objects contacts search

**hubspot-deals-crm** — Manage hubspot deals crm

- `hubspot-pp-cli hubspot-deals-crm delete-v3-objects-0-3-deal-id-archive` — Move an Object identified by `{dealId}` to the recycling bin.
- `hubspot-pp-cli hubspot-deals-crm get-v3-objects-0-3-deal-id-get-by-id` — Read an Object identified by `{dealId}`.
- `hubspot-pp-cli hubspot-deals-crm get-v3-objects-0-3-get-page` — Read a page of deals. Control what is returned via the `properties` query param.
- `hubspot-pp-cli hubspot-deals-crm patch-v3-objects-0-3-deal-id-update` — Perform a partial update of an Object identified by `{dealId}`or optionally a unique property value as specified by the
- `hubspot-pp-cli hubspot-deals-crm post-v3-objects-0-3-batch-archive-archive` — Archive multiple deals using their IDs.
- `hubspot-pp-cli hubspot-deals-crm post-v3-objects-0-3-batch-create-create` — Create multiple deals in a single request.
- `hubspot-pp-cli hubspot-deals-crm post-v3-objects-0-3-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value
- `hubspot-pp-cli hubspot-deals-crm post-v3-objects-0-3-batch-update-update` — Update multiple deals using their internal IDs or unique property values.
- `hubspot-pp-cli hubspot-deals-crm post-v3-objects-0-3-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param.
- `hubspot-pp-cli hubspot-deals-crm post-v3-objects-0-3-create` — Create a deal with the given properties and return a copy of the object, including the ID.
- `hubspot-pp-cli hubspot-deals-crm post-v3-objects-0-3-merge-merge` — Combine two deals of the same type into a single deal.
- `hubspot-pp-cli hubspot-deals-crm post-v3-objects-0-3-search-do-search` — Search for deals using various filters and criteria to retrieve specific records.

**hubspot-emails-crm** — Manage hubspot emails crm

- `hubspot-pp-cli hubspot-emails-crm delete-v3-objects-emails-email-id-archive` — Move an Object identified by `{emailId}` to the recycling bin.
- `hubspot-pp-cli hubspot-emails-crm get-v3-objects-emails-email-id-get-by-id` — Read an Object identified by `{emailId}`.
- `hubspot-pp-cli hubspot-emails-crm get-v3-objects-emails-get-page` — Read a page of emails. Control what is returned via the `properties` query param.
- `hubspot-pp-cli hubspot-emails-crm patch-v3-objects-emails-email-id-update` — Perform a partial update of an Object identified by `{emailId}`or optionally a unique property value as specified by
- `hubspot-pp-cli hubspot-emails-crm post-v3-objects-emails-batch-archive-archive` — Archive a batch of emails identified by their IDs.
- `hubspot-pp-cli hubspot-emails-crm post-v3-objects-emails-batch-create-create` — Create a batch of emails with specified properties and return the created objects.
- `hubspot-pp-cli hubspot-emails-crm post-v3-objects-emails-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value
- `hubspot-pp-cli hubspot-emails-crm post-v3-objects-emails-batch-update-update` — Update a batch of emails using their internal IDs or unique property values.
- `hubspot-pp-cli hubspot-emails-crm post-v3-objects-emails-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param.
- `hubspot-pp-cli hubspot-emails-crm post-v3-objects-emails-create` — Create a email with the given properties and return a copy of the object, including the ID.
- `hubspot-pp-cli hubspot-emails-crm post-v3-objects-emails-search-do-search` — Perform a search for emails based on the provided query parameters and return matching results.

**hubspot-imports-crm** — Manage hubspot imports crm

- `hubspot-pp-cli hubspot-imports-crm get-v3-imports-import-id-errors-v3-imports-import-id-errors` — Get v3 imports import id errors v3 imports import id errors
- `hubspot-pp-cli hubspot-imports-crm get-v3-imports-import-id-v3-imports-import-id` — Get v3 imports import id v3 imports import id
- `hubspot-pp-cli hubspot-imports-crm get-v3-imports-v3-imports` — Get v3 imports v3 imports
- `hubspot-pp-cli hubspot-imports-crm post-v3-imports-import-id-cancel-v3-imports-import-id-cancel` — Post v3 imports import id cancel v3 imports import id cancel
- `hubspot-pp-cli hubspot-imports-crm post-v3-imports-v3-imports` — Post v3 imports v3 imports

**hubspot-leads-crm** — Manage hubspot leads crm

- `hubspot-pp-cli hubspot-leads-crm delete-v3-objects-leads-leads-id-archive` — Move an Object identified by `{leadsId}` to the recycling bin.
- `hubspot-pp-cli hubspot-leads-crm get-v3-objects-leads-get-page` — Read a page of leads. Control what is returned via the `properties` query param.
- `hubspot-pp-cli hubspot-leads-crm get-v3-objects-leads-leads-id-get-by-id` — Read an Object identified by `{leadsId}`.
- `hubspot-pp-cli hubspot-leads-crm patch-v3-objects-leads-leads-id-update` — Perform a partial update of an Object identified by `{leadsId}`or optionally a unique property value as specified by
- `hubspot-pp-cli hubspot-leads-crm post-v3-objects-leads-batch-archive-archive` — Archive multiple leads by their IDs in a single request, moving them to the recycling bin.
- `hubspot-pp-cli hubspot-leads-crm post-v3-objects-leads-batch-create-create` — Create multiple lead records in a single request by providing a batch of lead data.
- `hubspot-pp-cli hubspot-leads-crm post-v3-objects-leads-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value
- `hubspot-pp-cli hubspot-leads-crm post-v3-objects-leads-batch-update-update` — Update multiple lead records using their internal IDs or unique property values.
- `hubspot-pp-cli hubspot-leads-crm post-v3-objects-leads-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param.
- `hubspot-pp-cli hubspot-leads-crm post-v3-objects-leads-create` — Create a lead with the given properties and return a copy of the object, including the ID.
- `hubspot-pp-cli hubspot-leads-crm post-v3-objects-leads-search-do-search` — Perform a search for leads based on the provided filter groups, properties, and sorting options.

**hubspot-line-items-crm** — Manage hubspot line items crm

- `hubspot-pp-cli hubspot-line-items-crm delete-v3-objects-line-items-line-item-id-archive` — Move an Object identified by `{lineItemId}` to the recycling bin.
- `hubspot-pp-cli hubspot-line-items-crm get-v3-objects-line-items-get-page` — Read a page of line items. Control what is returned via the `properties` query param.
- `hubspot-pp-cli hubspot-line-items-crm get-v3-objects-line-items-line-item-id-get-by-id` — Read an Object identified by `{lineItemId}`.
- `hubspot-pp-cli hubspot-line-items-crm patch-v3-objects-line-items-line-item-id-update` — Perform a partial update of an Object identified by `{lineItemId}`or optionally a unique property value as specified by
- `hubspot-pp-cli hubspot-line-items-crm post-v3-objects-line-items-batch-archive-archive` — Archive multiple line items simultaneously by specifying their IDs in the request body.
- `hubspot-pp-cli hubspot-line-items-crm post-v3-objects-line-items-batch-create-create` — Create multiple line items in a single request by providing the necessary properties and associations for each item.
- `hubspot-pp-cli hubspot-line-items-crm post-v3-objects-line-items-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value
- `hubspot-pp-cli hubspot-line-items-crm post-v3-objects-line-items-batch-update-update` — Update multiple line items using their internal IDs or unique property values.
- `hubspot-pp-cli hubspot-line-items-crm post-v3-objects-line-items-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param.
- `hubspot-pp-cli hubspot-line-items-crm post-v3-objects-line-items-create` — Create a line item with the given properties and return a copy of the object, including the ID.
- `hubspot-pp-cli hubspot-line-items-crm post-v3-objects-line-items-search-do-search` — Execute a search for line items based on filters, properties, and sorting options provided in the request body.

**hubspot-lists-crm** — Manage hubspot lists crm

- `hubspot-pp-cli hubspot-lists-crm delete-v3-lists-folders-folder-id-v3-lists-folders-folder-id` — Delete v3 lists folders folder id v3 lists folders folder id
- `hubspot-pp-cli hubspot-lists-crm delete-v3-lists-list-id-memberships-v3-lists-list-id-memberships` — Delete v3 lists list id memberships v3 lists list id memberships
- `hubspot-pp-cli hubspot-lists-crm delete-v3-lists-list-id-schedule-conversion-v3-lists-list-id-schedule-conversion` — Delete v3 lists list id schedule conversion v3 lists list id schedule conversion
- `hubspot-pp-cli hubspot-lists-crm delete-v3-lists-list-id-v3-lists-list-id` — Delete v3 lists list id v3 lists list id
- `hubspot-pp-cli hubspot-lists-crm get-v3-lists-folders-v3-lists-folders` — Get v3 lists folders v3 lists folders
- `hubspot-pp-cli hubspot-lists-crm get-v3-lists-idmapping-v3-lists-idmapping` — Get v3 lists idmapping v3 lists idmapping
- `hubspot-pp-cli hubspot-lists-crm get-v3-lists-list-id-memberships-join-order-v3-lists-list-id-memberships-join-order` — Get v3 lists list id memberships join order v3 lists list id memberships join order
- `hubspot-pp-cli hubspot-lists-crm get-v3-lists-list-id-memberships-v3-lists-list-id-memberships` — Get v3 lists list id memberships v3 lists list id memberships
- `hubspot-pp-cli hubspot-lists-crm get-v3-lists-list-id-schedule-conversion-v3-lists-list-id-schedule-conversion` — Get v3 lists list id schedule conversion v3 lists list id schedule conversion
- `hubspot-pp-cli hubspot-lists-crm get-v3-lists-list-id-size-and-edits-history-between-v3-lists-list-id-size-and-edits-history-between` — Get v3 lists list id size and edits history between v3 lists list id size and edits history between
- `hubspot-pp-cli hubspot-lists-crm get-v3-lists-list-id-v3-lists-list-id` — Get v3 lists list id v3 lists list id
- `hubspot-pp-cli hubspot-lists-crm get-v3-lists-object-type-id-object-type-id-name-list-name-v3-lists-object-type-id-object-type-id-name-list-name` — Retrieve a specific list by its name and object type ID.
- `hubspot-pp-cli hubspot-lists-crm get-v3-lists-records-object-type-id-record-id-memberships-v3-lists-records-object-type-id-record-id-memberships` — Get v3 lists records object type id record id memberships v3 lists records object type id record id memberships
- `hubspot-pp-cli hubspot-lists-crm get-v3-lists-v3-lists` — Get v3 lists v3 lists
- `hubspot-pp-cli hubspot-lists-crm post-v3-lists-folders-v3-lists-folders` — Post v3 lists folders v3 lists folders
- `hubspot-pp-cli hubspot-lists-crm post-v3-lists-idmapping-v3-lists-idmapping` — Post v3 lists idmapping v3 lists idmapping
- `hubspot-pp-cli hubspot-lists-crm post-v3-lists-records-memberships-batch-read-v3-lists-records-memberships-batch-read` — Post v3 lists records memberships batch read v3 lists records memberships batch read
- `hubspot-pp-cli hubspot-lists-crm post-v3-lists-search-v3-lists-search` — Post v3 lists search v3 lists search
- `hubspot-pp-cli hubspot-lists-crm post-v3-lists-v3-lists` — Post v3 lists v3 lists
- `hubspot-pp-cli hubspot-lists-crm put-v3-lists-folders-folder-id-move-new-parent-folder-id-v3-lists-folders-folder-id-move-new-parent-folder-id` — Put v3 lists folders folder id move new parent folder id v3 lists folders folder id move new parent folder id
- `hubspot-pp-cli hubspot-lists-crm put-v3-lists-folders-folder-id-rename-v3-lists-folders-folder-id-rename` — Put v3 lists folders folder id rename v3 lists folders folder id rename
- `hubspot-pp-cli hubspot-lists-crm put-v3-lists-folders-move-list-v3-lists-folders-move-list` — Put v3 lists folders move list v3 lists folders move list
- `hubspot-pp-cli hubspot-lists-crm put-v3-lists-list-id-memberships-add-and-remove-v3-lists-list-id-memberships-add-and-remove` — Put v3 lists list id memberships add and remove v3 lists list id memberships add and remove
- `hubspot-pp-cli hubspot-lists-crm put-v3-lists-list-id-memberships-add-from-source-list-id-v3-lists-list-id-memberships-add-from-source-list-id` — Put v3 lists list id memberships add from source list id v3 lists list id memberships add from source list id
- `hubspot-pp-cli hubspot-lists-crm put-v3-lists-list-id-memberships-add-v3-lists-list-id-memberships-add` — Put v3 lists list id memberships add v3 lists list id memberships add
- `hubspot-pp-cli hubspot-lists-crm put-v3-lists-list-id-memberships-remove-v3-lists-list-id-memberships-remove` — Put v3 lists list id memberships remove v3 lists list id memberships remove
- `hubspot-pp-cli hubspot-lists-crm put-v3-lists-list-id-restore-v3-lists-list-id-restore` — Put v3 lists list id restore v3 lists list id restore
- `hubspot-pp-cli hubspot-lists-crm put-v3-lists-list-id-schedule-conversion-v3-lists-list-id-schedule-conversion` — Put v3 lists list id schedule conversion v3 lists list id schedule conversion
- `hubspot-pp-cli hubspot-lists-crm put-v3-lists-list-id-update-list-filters-v3-lists-list-id-update-list-filters` — Put v3 lists list id update list filters v3 lists list id update list filters
- `hubspot-pp-cli hubspot-lists-crm put-v3-lists-list-id-update-list-name-v3-lists-list-id-update-list-name` — Put v3 lists list id update list name v3 lists list id update list name

**hubspot-meetings-crm** — Manage hubspot meetings crm

- `hubspot-pp-cli hubspot-meetings-crm delete-v3-objects-meetings-meeting-id-archive` — Move an Object identified by `{meetingId}` to the recycling bin.
- `hubspot-pp-cli hubspot-meetings-crm get-v3-objects-meetings-get-page` — Read a page of meetings. Control what is returned via the `properties` query param.
- `hubspot-pp-cli hubspot-meetings-crm get-v3-objects-meetings-meeting-id-get-by-id` — Read an Object identified by `{meetingId}`.
- `hubspot-pp-cli hubspot-meetings-crm patch-v3-objects-meetings-meeting-id-update` — Perform a partial update of an Object identified by `{meetingId}`or optionally a unique property value as specified by
- `hubspot-pp-cli hubspot-meetings-crm post-v3-objects-meetings-batch-archive-archive` — Archive a batch of meetings by ID
- `hubspot-pp-cli hubspot-meetings-crm post-v3-objects-meetings-batch-create-create` — Create a batch of meetings
- `hubspot-pp-cli hubspot-meetings-crm post-v3-objects-meetings-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value
- `hubspot-pp-cli hubspot-meetings-crm post-v3-objects-meetings-batch-update-update` — Update a batch of meetings by internal ID, or unique property values
- `hubspot-pp-cli hubspot-meetings-crm post-v3-objects-meetings-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param.
- `hubspot-pp-cli hubspot-meetings-crm post-v3-objects-meetings-create` — Create a meeting with the given properties and return a copy of the object, including the ID.
- `hubspot-pp-cli hubspot-meetings-crm post-v3-objects-meetings-search-do-search` — Post v3 objects meetings search do search

**hubspot-notes-crm** — Manage hubspot notes crm

- `hubspot-pp-cli hubspot-notes-crm delete-v3-objects-notes-note-id-archive` — Move an Object identified by `{noteId}` to the recycling bin.
- `hubspot-pp-cli hubspot-notes-crm get-v3-objects-notes-get-page` — Read a page of notes. Control what is returned via the `properties` query param.
- `hubspot-pp-cli hubspot-notes-crm get-v3-objects-notes-note-id-get-by-id` — Read an Object identified by `{noteId}`.
- `hubspot-pp-cli hubspot-notes-crm patch-v3-objects-notes-note-id-update` — Perform a partial update of an Object identified by `{noteId}`or optionally a unique property value as specified by the
- `hubspot-pp-cli hubspot-notes-crm post-v3-objects-notes-batch-archive-archive` — Archive multiple notes by their IDs in a single request.
- `hubspot-pp-cli hubspot-notes-crm post-v3-objects-notes-batch-create-create` — Create multiple notes in a single request by providing the necessary properties for each note.
- `hubspot-pp-cli hubspot-notes-crm post-v3-objects-notes-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value
- `hubspot-pp-cli hubspot-notes-crm post-v3-objects-notes-batch-update-update` — Update multiple notes using their internal IDs or unique property values.
- `hubspot-pp-cli hubspot-notes-crm post-v3-objects-notes-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param.
- `hubspot-pp-cli hubspot-notes-crm post-v3-objects-notes-create` — Create a note with the given properties and return a copy of the object, including the ID.
- `hubspot-pp-cli hubspot-notes-crm post-v3-objects-notes-search-do-search` — Execute a search for notes using filters, sorting options, and other query parameters to refine the results.

**hubspot-objects-crm** — Manage hubspot objects crm

- `hubspot-pp-cli hubspot-objects-crm delete-v3-objects-object-type-object-id-archive` — Move an Object identified by `{objectId}` to the recycling bin.
- `hubspot-pp-cli hubspot-objects-crm get-v3-objects-object-type-get-page` — Read a page of objects. Control what is returned via the `properties` query param.
- `hubspot-pp-cli hubspot-objects-crm get-v3-objects-object-type-object-id-get-by-id` — Read an Object identified by `{objectId}`.
- `hubspot-pp-cli hubspot-objects-crm patch-v3-objects-object-type-object-id-update` — Perform a partial update of an Object identified by `{objectId}`or optionally a unique property value as specified by
- `hubspot-pp-cli hubspot-objects-crm post-v3-objects-object-type-create` — Create a CRM object with the given properties and return a copy of the object, including the ID.

**hubspot-owners-crm** — Manage hubspot owners crm

- `hubspot-pp-cli hubspot-owners-crm get-v3-owners-owner-id-get-by-id` — Retrieve details of a specific owner using either their 'id' or 'userId'.
- `hubspot-pp-cli hubspot-owners-crm get-v3-owners-v3-owners` — Get v3 owners v3 owners

**hubspot-pipelines-crm** — Manage hubspot pipelines crm

- `hubspot-pp-cli hubspot-pipelines-crm delete-v3-pipelines-object-type-pipeline-id-archive` — Delete a pipeline
- `hubspot-pp-cli hubspot-pipelines-crm delete-v3-pipelines-object-type-pipeline-id-stages-stage-id-archive` — Delete a pipeline stage
- `hubspot-pp-cli hubspot-pipelines-crm get-v3-pipelines-object-type-get-all` — Return all pipelines for the object type specified by `{objectType}`.
- `hubspot-pp-cli hubspot-pipelines-crm get-v3-pipelines-object-type-pipeline-id-audit-get-audit` — Return a reverse chronological list of all mutations that have occurred on the pipeline identified by `{pipelineId}`.
- `hubspot-pp-cli hubspot-pipelines-crm get-v3-pipelines-object-type-pipeline-id-get-by-id` — Return a single pipeline object identified by its unique `{pipelineId}`.
- `hubspot-pp-cli hubspot-pipelines-crm get-v3-pipelines-object-type-pipeline-id-stages-get-all` — Return all the stages associated with the pipeline identified by `{pipelineId}`.
- `hubspot-pp-cli hubspot-pipelines-crm get-v3-pipelines-object-type-pipeline-id-stages-stage-id-audit-get-audit` — Return a reverse chronological list of all mutations that have occurred on the pipeline stage identified by `{stageId}`.
- `hubspot-pp-cli hubspot-pipelines-crm get-v3-pipelines-object-type-pipeline-id-stages-stage-id-get-by-id` — Return a pipeline stage by ID
- `hubspot-pp-cli hubspot-pipelines-crm patch-v3-pipelines-object-type-pipeline-id-stages-stage-id-update` — Patch v3 pipelines object type pipeline id stages stage id update
- `hubspot-pp-cli hubspot-pipelines-crm patch-v3-pipelines-object-type-pipeline-id-update` — Perform a partial update of the pipeline identified by `{pipelineId}`.
- `hubspot-pp-cli hubspot-pipelines-crm post-v3-pipelines-object-type-create` — Create a new pipeline with the provided property values.
- `hubspot-pp-cli hubspot-pipelines-crm post-v3-pipelines-object-type-pipeline-id-stages-create` — Create a pipeline stage
- `hubspot-pp-cli hubspot-pipelines-crm put-v3-pipelines-object-type-pipeline-id-replace` — Replace a pipeline
- `hubspot-pp-cli hubspot-pipelines-crm put-v3-pipelines-object-type-pipeline-id-stages-stage-id-replace` — Replace all the properties of an existing pipeline stage with the values provided.

**hubspot-products-crm** — Manage hubspot products crm

- `hubspot-pp-cli hubspot-products-crm delete-v3-objects-products-product-id-archive` — Move an Object identified by `{productId}` to the recycling bin.
- `hubspot-pp-cli hubspot-products-crm get-v3-objects-products-get-page` — Read a page of products. Control what is returned via the `properties` query param.
- `hubspot-pp-cli hubspot-products-crm get-v3-objects-products-product-id-get-by-id` — Read an Object identified by `{productId}`.
- `hubspot-pp-cli hubspot-products-crm patch-v3-objects-products-product-id-update` — Perform a partial update of an Object identified by `{productId}`or optionally a unique property value as specified by
- `hubspot-pp-cli hubspot-products-crm post-v3-objects-products-batch-archive-archive` — Archive multiple products at once by providing their IDs.
- `hubspot-pp-cli hubspot-products-crm post-v3-objects-products-batch-create-create` — Create multiple products in a single request by specifying their properties
- `hubspot-pp-cli hubspot-products-crm post-v3-objects-products-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value
- `hubspot-pp-cli hubspot-products-crm post-v3-objects-products-batch-update-update` — Update multiple products in a single request using their internal IDs or unique property values.
- `hubspot-pp-cli hubspot-products-crm post-v3-objects-products-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param.
- `hubspot-pp-cli hubspot-products-crm post-v3-objects-products-create` — Create a product with the given properties and return a copy of the object, including the ID.
- `hubspot-pp-cli hubspot-products-crm post-v3-objects-products-search-do-search` — Execute a search for products based on defined filters, properties, and sorting options.

**hubspot-properties-batch** — Manage hubspot properties batch

- `hubspot-pp-cli hubspot-properties-batch post-crm-v3-properties-object-type-archive-archive` — Archive a provided list of properties.
- `hubspot-pp-cli hubspot-properties-batch post-crm-v3-properties-object-type-create-create` — Create a batch of properties using the same rules as when creating an individual property.
- `hubspot-pp-cli hubspot-properties-batch post-crm-v3-properties-object-type-read-read` — Read a provided list of properties.

**hubspot-properties-crm** — Manage hubspot properties crm

- `hubspot-pp-cli hubspot-properties-crm delete-v3-properties-object-type-property-name-archive` — Move a property identified by {propertyName} to the recycling bin.
- `hubspot-pp-cli hubspot-properties-crm get-v3-properties-object-type-get-all` — Read all existing properties for the specified object type and HubSpot account.
- `hubspot-pp-cli hubspot-properties-crm get-v3-properties-object-type-property-name-get-by-name` — Read a property identified by {propertyName}.
- `hubspot-pp-cli hubspot-properties-crm patch-v3-properties-object-type-property-name-update` — Perform a partial update of a property identified by { propertyName }. Provided fields will be overwritten.
- `hubspot-pp-cli hubspot-properties-crm post-v3-properties-object-type-create` — Create and return a copy of a new property for the specified object type.

**hubspot-quotes-crm** — Manage hubspot quotes crm

- `hubspot-pp-cli hubspot-quotes-crm delete-v3-objects-quotes-quote-id-archive` — Move an Object identified by `{quoteId}` to the recycling bin.
- `hubspot-pp-cli hubspot-quotes-crm get-v3-objects-quotes-get-page` — Read a page of quotes. Control what is returned via the `properties` query param.
- `hubspot-pp-cli hubspot-quotes-crm get-v3-objects-quotes-quote-id-get-by-id` — Read an Object identified by `{quoteId}`.
- `hubspot-pp-cli hubspot-quotes-crm patch-v3-objects-quotes-quote-id-update` — Perform a partial update of an Object identified by `{quoteId}`or optionally a unique property value as specified by
- `hubspot-pp-cli hubspot-quotes-crm post-v3-objects-quotes-batch-archive-archive` — Archive multiple quotes by their IDs in a single request, effectively moving them to the recycling bin.
- `hubspot-pp-cli hubspot-quotes-crm post-v3-objects-quotes-batch-create-create` — Create multiple quotes in a single request by providing a batch of quote objects
- `hubspot-pp-cli hubspot-quotes-crm post-v3-objects-quotes-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value
- `hubspot-pp-cli hubspot-quotes-crm post-v3-objects-quotes-batch-update-update` — Update multiple quotes using their internal IDs or unique property values.
- `hubspot-pp-cli hubspot-quotes-crm post-v3-objects-quotes-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param.
- `hubspot-pp-cli hubspot-quotes-crm post-v3-objects-quotes-create` — Create a quote with the given properties and return a copy of the object, including the ID.
- `hubspot-pp-cli hubspot-quotes-crm post-v3-objects-quotes-search-do-search` — Execute a search for quotes based on the criteria defined in the request body, such as filters, properties

**hubspot-tasks-crm** — Manage hubspot tasks crm

- `hubspot-pp-cli hubspot-tasks-crm delete-v3-objects-tasks-task-id-archive` — Move an Object identified by `{taskId}` to the recycling bin.
- `hubspot-pp-cli hubspot-tasks-crm get-v3-objects-tasks-get-page` — Read a page of tasks. Control what is returned via the `properties` query param.
- `hubspot-pp-cli hubspot-tasks-crm get-v3-objects-tasks-task-id-get-by-id` — Read an Object identified by `{taskId}`.
- `hubspot-pp-cli hubspot-tasks-crm patch-v3-objects-tasks-task-id-update` — Perform a partial update of an Object identified by `{taskId}`or optionally a unique property value as specified by the
- `hubspot-pp-cli hubspot-tasks-crm post-v3-objects-tasks-batch-archive-archive` — Archive a batch of tasks by their IDs, moving them to the recycling bin.
- `hubspot-pp-cli hubspot-tasks-crm post-v3-objects-tasks-batch-create-create` — Create multiple tasks in a single request by providing a batch of task properties and associations.
- `hubspot-pp-cli hubspot-tasks-crm post-v3-objects-tasks-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value
- `hubspot-pp-cli hubspot-tasks-crm post-v3-objects-tasks-batch-update-update` — Update multiple tasks in a single request using their internal IDs or unique property values.
- `hubspot-pp-cli hubspot-tasks-crm post-v3-objects-tasks-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param.
- `hubspot-pp-cli hubspot-tasks-crm post-v3-objects-tasks-create` — Create a task with the given properties and return a copy of the object, including the ID.
- `hubspot-pp-cli hubspot-tasks-crm post-v3-objects-tasks-search-do-search` — Execute a search for tasks based on the provided criteria, including filters, properties, and sorting options.

**hubspot-tickets-crm** — Manage hubspot tickets crm

- `hubspot-pp-cli hubspot-tickets-crm delete-v3-objects-tickets-ticket-id-archive` — Move an Object identified by `{ticketId}` to the recycling bin.
- `hubspot-pp-cli hubspot-tickets-crm get-v3-objects-tickets-get-page` — Read a page of tickets. Control what is returned via the `properties` query param.
- `hubspot-pp-cli hubspot-tickets-crm get-v3-objects-tickets-ticket-id-get-by-id` — Read an Object identified by `{ticketId}`.
- `hubspot-pp-cli hubspot-tickets-crm patch-v3-objects-tickets-ticket-id-update` — Perform a partial update of an Object identified by `{ticketId}`or optionally a unique property value as specified by
- `hubspot-pp-cli hubspot-tickets-crm post-v3-objects-tickets-batch-archive-archive` — Delete a batch of tickets by ID. Deleted tickets can be restored within 90 days of deletion.
- `hubspot-pp-cli hubspot-tickets-crm post-v3-objects-tickets-batch-create-create` — Create a batch of tickets.
- `hubspot-pp-cli hubspot-tickets-crm post-v3-objects-tickets-batch-read-read` — Retrieve a batch of tickets by ID (`ticketId`) or unique property value (`idProperty`).
- `hubspot-pp-cli hubspot-tickets-crm post-v3-objects-tickets-batch-update-update` — Update a batch of tickets by ID (`ticketId`) or unique property value (`idProperty`).
- `hubspot-pp-cli hubspot-tickets-crm post-v3-objects-tickets-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param.
- `hubspot-pp-cli hubspot-tickets-crm post-v3-objects-tickets-create` — Create a ticket with the given properties and return a copy of the object, including the ID.
- `hubspot-pp-cli hubspot-tickets-crm post-v3-objects-tickets-merge-merge` — Merge two tickets, combining them into one ticket record.
- `hubspot-pp-cli hubspot-tickets-crm post-v3-objects-tickets-search-do-search` — Search for tickets by filtering on properties, searching through associations, and sorting results.

**objects_search** — Manage objects search

- `hubspot-pp-cli objects-search <objectType>` — Post crm v3 objects object type search do search


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
hubspot-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Monthly customer report — every meeting EVER scheduled in April

```bash
hubspot-pp-cli sync --resources hubspot-meetings-crm --with-history hs_meeting_outcome,hs_meeting_title,hubspot_owner_id && hubspot-pp-cli meetings status-report --status scheduled --month 2026-04 --csv > april-scheduled.csv
```

Two commands: refresh meetings with property history retained, then emit the canonical CSV the customer expects. Includes every meeting that was set to Scheduled at any point in April, even if it later flipped to No Show or Completed — only the property-history snapshot table can answer this.

### Investigate a single meeting's status history

```bash
hubspot-pp-cli meetings ever-had --property hs_meeting_outcome --value Scheduled --from 2026-04-01 --to 2026-04-30 --agent --select id,property,value,timestamp,source_type --json
```

Find every meeting touching status Scheduled in April; agent-friendly dotted-path --select keeps only the columns the agent needs, --agent enables compact output.

### Today's nurture queue for me

```bash
hubspot-pp-cli nurture queue --owner me --top 20 --agent
```

Ranked daily contact list with stale-days, deal $, and stage probability columns — drop straight into `/nurture today`.

### Cross-object timeline for a deal

```bash
hubspot-pp-cli engagements of deal:1234567 --since 90d --json --select id,type,timestamp,subject,from_owner
```

Every call, email, meeting, note, and task touching the deal in the last 90 days — one query replaces five paginated API calls; --select keeps the agent context tight.

### Find every cold deal in my pipeline

```bash
hubspot-pp-cli stale deals --days 21 --owner me --json --select id,name,amount,stage,idle_days
```

Open deals that have not had an engagement in 21 days, scoped to me, with only the fields an agent needs.

### Bulk-update lifecycle stage from a campaign CSV

```bash
hubspot-pp-cli contacts bulk-update --from-csv titans.csv --map email=email,lifecyclestage=Stage --dry-run
```

Validate the CSV against the local properties schema before any write; drop --dry-run when the report is clean.

## Auth Setup

Authenticate with a HubSpot Private App access token (prefix `pat-…`). Create one at https://app.hubspot.com/private-apps with CRM read+write scopes, then export it as `HUBSPOT_ACCESS_TOKEN`. The token's scopes determine which commands work — `doctor` reports them.

Run `hubspot-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  hubspot-pp-cli batch post-crm-v3-objects-object-type-archive-archive mock-value --agent --select id,name,status
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
hubspot-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
hubspot-pp-cli feedback --stdin < notes.txt
hubspot-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/hubspot-pp-cli/feedback.jsonl`. They are never POSTed unless `HUBSPOT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HUBSPOT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
hubspot-pp-cli profile save briefing --json
hubspot-pp-cli --profile briefing batch post-crm-v3-objects-object-type-archive-archive mock-value
hubspot-pp-cli profile list --json
hubspot-pp-cli profile show briefing
hubspot-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `hubspot-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add hubspot-pp-mcp -- hubspot-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which hubspot-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   hubspot-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `hubspot-pp-cli <command> --help`.
