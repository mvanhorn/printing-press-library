---
name: pp-hubspot
description: "Every HubSpot CRM endpoint, plus a local SQLite mirror that answers compound pre-sale queries (stale leads, pipeline health, recent intake, dedup, Closed Won handoff, engagement decay) no other HubSpot tool returns in one call. Read-focused. Trigger phrases: `show stale HubSpot leads`, `check the HubSpot pipeline`, `who landed in HubSpot overnight`, `dedup HubSpot contacts`, `closed-won handoff to ClickUp`, `hubspot daily digest`, `use hubspot-pp-cli`, `run hubspot`."
author: "simplepathmedia"
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
   npx -y @mvanhorn/printing-press install hubspot --cli-only
   ```
2. Verify: `hubspot-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

A read-focused HubSpot CLI built for agents and automation flows. Every CRM object lives in a local SQLite cache after the first sync, so stale-lead, pipeline-health, recent-intake, dedup, Closed Won handoff, and engagement-decay queries become single-call commands instead of multi-step API orchestration. Outputs default to compact JSON so token cost stays predictable across thousands of agent invocations.

## When to Use This CLI

Use this CLI when an agent needs to answer pre-sale HubSpot questions ('which leads are stale', 'which deals are stuck', 'who landed overnight', 'are these duplicates') in one tool call instead of orchestrating the official HubSpot MCP's many small tools. The local SQLite mirror means lookups, filters, and joins happen offline at near-zero token cost, and only sync commands talk to HubSpot.

**Do NOT use this CLI for:**
- Marketing Hub email sends, sequences, automation — out of scope by design
- CMS, blog posts, landing pages, or HubSpot forms — wrong product surface
- Creating, updating, archiving, or merging CRM records — the spec ships those endpoints as `<resource>-crm post-…-create` / `patch-…-update` / `delete-…-archive` for completeness, but the intended workflow is read-only. Use the HubSpot UI or the official HubSpot MCP for writes; if you grant write scopes on the private app token, you own the blast radius.
- Post-Closed-Won customer records — those move to the operator's downstream system (ClickUp), so query them there.
- Building HubSpot workflows or automations — that is n8n's job.

## Unique Capabilities

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

## Command Reference

**associations-crm** — Manage crm

- `hubspot-pp-cli associations-crm delete-v4-objects-object-type-object-id-associations-to-object-type-to-object-id-archive` — deletes all associations between two records.
- `hubspot-pp-cli associations-crm get-v4-objects-object-type-object-id-associations-to-object-type-get-page` — Retrieve all associations between a specific record and an object type. Limit 500 per call.
- `hubspot-pp-cli associations-crm post-v4-associations-from-object-type-to-object-type-batch-archive-archive` — Batch delete associations for objects
- `hubspot-pp-cli associations-crm post-v4-associations-from-object-type-to-object-type-batch-associate-default-create-default` — Create the default (most generic) association type between two object types
- `hubspot-pp-cli associations-crm post-v4-associations-from-object-type-to-object-type-batch-create-create` — Batch create associations for objects
- `hubspot-pp-cli associations-crm post-v4-associations-from-object-type-to-object-type-batch-labels-archive-archive-labels` — Batch delete specific association labels for objects. Deleting an unlabeled association will also delete all labeled...
- `hubspot-pp-cli associations-crm post-v4-associations-from-object-type-to-object-type-batch-read-get-page` — Batch read associations for objects to specific object type. The 'after' field in a returned paging object can be...
- `hubspot-pp-cli associations-crm post-v4-associations-usage-high-usage-report-user-id-request` — Requests a report of all objects in the portal which have a high usage of associations
- `hubspot-pp-cli associations-crm put-v4-objects-from-object-type-from-object-id-associations-default-to-object-type-to-object-id-create-default` — Create the default (most generic) association type between two object types
- `hubspot-pp-cli associations-crm put-v4-objects-object-type-object-id-associations-to-object-type-to-object-id-create` — Set association labels between two records.

**batch** — Manage batch

- `hubspot-pp-cli batch post-crm-v3-properties-object-type-archive-archive` — Archive a provided list of properties. This method will return a 204 No Content response on success regardless of...
- `hubspot-pp-cli batch post-crm-v3-properties-object-type-create-create` — Create a batch of properties using the same rules as when creating an individual property.
- `hubspot-pp-cli batch post-crm-v3-properties-object-type-read-read` — Read a provided list of properties.

**calls-crm** — Manage crm

- `hubspot-pp-cli calls-crm delete-v3-objects-calls-call-id-archive` — Move an Object identified by `{callId}` to the recycling bin.
- `hubspot-pp-cli calls-crm get-v3-objects-calls-call-id-get-by-id` — Read an Object identified by `{callId}`. `{callId}` refers to the internal object ID by default, or optionally any...
- `hubspot-pp-cli calls-crm get-v3-objects-calls-get-page` — Read a page of calls. Control what is returned via the `properties` query param.
- `hubspot-pp-cli calls-crm patch-v3-objects-calls-call-id-update` — Perform a partial update of an Object identified by `{callId}`or optionally a unique property value as specified by...
- `hubspot-pp-cli calls-crm post-v3-objects-calls-batch-archive-archive` — Archive a batch of calls by ID. Deleted calls can be restored within 90 days of being deleted, but call recordings...
- `hubspot-pp-cli calls-crm post-v3-objects-calls-batch-create-create` — Create a batch of calls. The `inputs` array can contain a `properties` object to define property values for each...
- `hubspot-pp-cli calls-crm post-v3-objects-calls-batch-read-read` — Read a batch of calls by internal ID, or unique property values
- `hubspot-pp-cli calls-crm post-v3-objects-calls-batch-update-update` — Update a batch of calls by internal ID, or unique property values
- `hubspot-pp-cli calls-crm post-v3-objects-calls-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param....
- `hubspot-pp-cli calls-crm post-v3-objects-calls-create` — Create a call with the given properties and return a copy of the object, including the ID. Documentation and...
- `hubspot-pp-cli calls-crm post-v3-objects-calls-search-do-search` — Search for calls by filtering on properties, searching through associations, and sorting results. Learn more about...

**companies-crm** — Manage crm

- `hubspot-pp-cli companies-crm delete-v3-objects-companies-company-id-archive` — Delete a company by ID. Deleted companies can be restored within 90 days of deletion. Learn more about [restoring...
- `hubspot-pp-cli companies-crm get-v3-objects-companies-company-id-get-by-id` — Retrieve a company by its ID (`companyId`) or by a unique property (`idProperty`). You can specify what is returned...
- `hubspot-pp-cli companies-crm get-v3-objects-companies-get-page` — Retrieve all companies, using query parameters to control the information that gets returned.
- `hubspot-pp-cli companies-crm patch-v3-objects-companies-company-id-update` — Update a company by ID (`companyId`) or unique property value (`idProperty`). Provided property values will be...
- `hubspot-pp-cli companies-crm post-v3-objects-companies-batch-archive-archive` — Delete a batch of companies by ID. Deleted companies can be restored within 90 days of deletion. Learn more about...
- `hubspot-pp-cli companies-crm post-v3-objects-companies-batch-create-create` — Create a batch of companies. The `inputs` array can contain a `properties` object to define property values for each...
- `hubspot-pp-cli companies-crm post-v3-objects-companies-batch-read-read` — Retrieve a batch of companies by ID (`companyId`) or by a unique property (`idProperty`). You can specify what is...
- `hubspot-pp-cli companies-crm post-v3-objects-companies-batch-update-update` — Update a batch of companies by ID.
- `hubspot-pp-cli companies-crm post-v3-objects-companies-batch-upsert-upsert` — Create or update companies identified by a unique property value as specified by the `idProperty` query parameter....
- `hubspot-pp-cli companies-crm post-v3-objects-companies-create` — Create a single company. Include a `properties` object to define [property...
- `hubspot-pp-cli companies-crm post-v3-objects-companies-merge-merge` — Merge two company records. Learn more about [merging records](https://knowledge.hubspot.com/records/merge-records).
- `hubspot-pp-cli companies-crm post-v3-objects-companies-search-do-search` — Search for companies by filtering on properties, searching through associations, and sorting results. Learn more...

**crm** — Manage crm

- `hubspot-pp-cli crm delete-v3-objects-contacts-contact-id-archive` — Delete a contact by ID. Deleted contacts can be restored within 90 days of deletion. Learn more about the [data...
- `hubspot-pp-cli crm get-v3-objects-contacts-contact-id-get-by-id` — Retrieve a contact by its ID (`contactId`) or by a unique property (`idProperty`). You can specify what is returned...
- `hubspot-pp-cli crm get-v3-objects-contacts-get-page` — Retrieve all contacts, using query parameters to specify the information that gets returned.
- `hubspot-pp-cli crm patch-v3-objects-contacts-contact-id-update` — Update an existing contact, identified by ID or email/unique property value. To identify a contact by ID, include...
- `hubspot-pp-cli crm post-v3-objects-contacts-batch-archive-archive` — Archive a batch of contacts by ID. Archived contacts can be restored within 90 days of deletion. Learn more about...
- `hubspot-pp-cli crm post-v3-objects-contacts-batch-create-create` — Create a batch of contacts. The `inputs` array can contain a `properties` object to define property values for each...
- `hubspot-pp-cli crm post-v3-objects-contacts-batch-read-read` — Retrieve a batch of contacts by ID (`contactId`) or unique property value (`idProperty`).
- `hubspot-pp-cli crm post-v3-objects-contacts-batch-update-update` — Update a batch of contacts by ID (`contactId`) or unique property value (`idProperty`). Provided property values...
- `hubspot-pp-cli crm post-v3-objects-contacts-batch-upsert-upsert` — Upsert a batch of contacts. The `inputs` array can contain a `properties` object to define property values for each...
- `hubspot-pp-cli crm post-v3-objects-contacts-create` — Create a single contact. Include a `properties` object to define [property...
- `hubspot-pp-cli crm post-v3-objects-contacts-gdpr-delete-purge` — Permanently delete a contact and all associated content to follow GDPR. Use optional property `idProperty` set to...
- `hubspot-pp-cli crm post-v3-objects-contacts-merge-merge` — Merge two contact records. Learn more about [merging records](https://knowledge.hubspot.com/records/merge-records).
- `hubspot-pp-cli crm post-v3-objects-contacts-search-do-search` — Search for contacts by filtering on properties, searching through associations, and sorting results. Learn more...

**crm-lists-crm** — Manage crm

- `hubspot-pp-cli crm-lists-crm delete-v3-lists-folders-folder-id-v3-lists-folders-folder-id` — Delete v3 lists folders folder id v3 lists folders folder id
- `hubspot-pp-cli crm-lists-crm delete-v3-lists-list-id-memberships-v3-lists-list-id-memberships` — Delete v3 lists list id memberships v3 lists list id memberships
- `hubspot-pp-cli crm-lists-crm delete-v3-lists-list-id-schedule-conversion-v3-lists-list-id-schedule-conversion` — Delete v3 lists list id schedule conversion v3 lists list id schedule conversion
- `hubspot-pp-cli crm-lists-crm delete-v3-lists-list-id-v3-lists-list-id` — Delete v3 lists list id v3 lists list id
- `hubspot-pp-cli crm-lists-crm get-v3-lists-folders-v3-lists-folders` — Get v3 lists folders v3 lists folders
- `hubspot-pp-cli crm-lists-crm get-v3-lists-idmapping-v3-lists-idmapping` — Get v3 lists idmapping v3 lists idmapping
- `hubspot-pp-cli crm-lists-crm get-v3-lists-list-id-memberships-join-order-v3-lists-list-id-memberships-join-order` — Get v3 lists list id memberships join order v3 lists list id memberships join order
- `hubspot-pp-cli crm-lists-crm get-v3-lists-list-id-memberships-v3-lists-list-id-memberships` — Get v3 lists list id memberships v3 lists list id memberships
- `hubspot-pp-cli crm-lists-crm get-v3-lists-list-id-schedule-conversion-v3-lists-list-id-schedule-conversion` — Get v3 lists list id schedule conversion v3 lists list id schedule conversion
- `hubspot-pp-cli crm-lists-crm get-v3-lists-list-id-size-and-edits-history-between-v3-lists-list-id-size-and-edits-history-between` — Get v3 lists list id size and edits history between v3 lists list id size and edits history between
- `hubspot-pp-cli crm-lists-crm get-v3-lists-list-id-v3-lists-list-id` — Get v3 lists list id v3 lists list id
- `hubspot-pp-cli crm-lists-crm get-v3-lists-object-type-id-object-type-id-name-list-name-v3-lists-object-type-id-object-type-id-name-list-name` — Retrieve a specific list by its name and object type ID. This endpoint allows you to fetch details about a list,...
- `hubspot-pp-cli crm-lists-crm get-v3-lists-records-object-type-id-record-id-memberships-v3-lists-records-object-type-id-record-id-memberships` — Get v3 lists records object type id record id memberships v3 lists records object type id record id memberships
- `hubspot-pp-cli crm-lists-crm get-v3-lists-v3-lists` — Get v3 lists v3 lists
- `hubspot-pp-cli crm-lists-crm post-v3-lists-folders-v3-lists-folders` — Post v3 lists folders v3 lists folders
- `hubspot-pp-cli crm-lists-crm post-v3-lists-idmapping-v3-lists-idmapping` — Post v3 lists idmapping v3 lists idmapping
- `hubspot-pp-cli crm-lists-crm post-v3-lists-records-memberships-batch-read-v3-lists-records-memberships-batch-read` — Post v3 lists records memberships batch read v3 lists records memberships batch read
- `hubspot-pp-cli crm-lists-crm post-v3-lists-search-v3-lists-search` — Post v3 lists search v3 lists search
- `hubspot-pp-cli crm-lists-crm post-v3-lists-v3-lists` — Post v3 lists v3 lists
- `hubspot-pp-cli crm-lists-crm put-v3-lists-folders-folder-id-move-new-parent-folder-id-v3-lists-folders-folder-id-move-new-parent-folder-id` — Put v3 lists folders folder id move new parent folder id v3 lists folders folder id move new parent folder id
- `hubspot-pp-cli crm-lists-crm put-v3-lists-folders-folder-id-rename-v3-lists-folders-folder-id-rename` — Put v3 lists folders folder id rename v3 lists folders folder id rename
- `hubspot-pp-cli crm-lists-crm put-v3-lists-folders-move-list-v3-lists-folders-move-list` — Put v3 lists folders move list v3 lists folders move list
- `hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-memberships-add-and-remove-v3-lists-list-id-memberships-add-and-remove` — Put v3 lists list id memberships add and remove v3 lists list id memberships add and remove
- `hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-memberships-add-from-source-list-id-v3-lists-list-id-memberships-add-from-source-list-id` — Put v3 lists list id memberships add from source list id v3 lists list id memberships add from source list id
- `hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-memberships-add-v3-lists-list-id-memberships-add` — Put v3 lists list id memberships add v3 lists list id memberships add
- `hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-memberships-remove-v3-lists-list-id-memberships-remove` — Put v3 lists list id memberships remove v3 lists list id memberships remove
- `hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-restore-v3-lists-list-id-restore` — Put v3 lists list id restore v3 lists list id restore
- `hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-schedule-conversion-v3-lists-list-id-schedule-conversion` — Put v3 lists list id schedule conversion v3 lists list id schedule conversion
- `hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-update-list-filters-v3-lists-list-id-update-list-filters` — Put v3 lists list id update list filters v3 lists list id update list filters
- `hubspot-pp-cli crm-lists-crm put-v3-lists-list-id-update-list-name-v3-lists-list-id-update-list-name` — Put v3 lists list id update list name v3 lists list id update list name

**crm-owners-crm** — Manage crm

- `hubspot-pp-cli crm-owners-crm get-v3-owners-owner-id-get-by-id` — Retrieve details of a specific owner using either their 'id' or 'userId'.
- `hubspot-pp-cli crm-owners-crm get-v3-owners-v3-owners` — Get v3 owners v3 owners

**deals-crm** — Manage crm

- `hubspot-pp-cli deals-crm delete-v3-objects-0-3-deal-id-archive` — Move an Object identified by `{dealId}` to the recycling bin.
- `hubspot-pp-cli deals-crm get-v3-objects-0-3-deal-id-get-by-id` — Read an Object identified by `{dealId}`. `{dealId}` refers to the internal object ID by default, or optionally any...
- `hubspot-pp-cli deals-crm get-v3-objects-0-3-get-page` — Read a page of deals. Control what is returned via the `properties` query param.
- `hubspot-pp-cli deals-crm patch-v3-objects-0-3-deal-id-update` — Perform a partial update of an Object identified by `{dealId}`or optionally a unique property value as specified by...
- `hubspot-pp-cli deals-crm post-v3-objects-0-3-batch-archive-archive` — Archive multiple deals using their IDs.
- `hubspot-pp-cli deals-crm post-v3-objects-0-3-batch-create-create` — Create multiple deals in a single request.
- `hubspot-pp-cli deals-crm post-v3-objects-0-3-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value...
- `hubspot-pp-cli deals-crm post-v3-objects-0-3-batch-update-update` — Update multiple deals using their internal IDs or unique property values.
- `hubspot-pp-cli deals-crm post-v3-objects-0-3-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param....
- `hubspot-pp-cli deals-crm post-v3-objects-0-3-create` — Create a deal with the given properties and return a copy of the object, including the ID. Documentation and...
- `hubspot-pp-cli deals-crm post-v3-objects-0-3-merge-merge` — Combine two deals of the same type into a single deal.
- `hubspot-pp-cli deals-crm post-v3-objects-0-3-search-do-search` — Search for deals using various filters and criteria to retrieve specific records.

**emails-crm** — Manage crm

- `hubspot-pp-cli emails-crm delete-v3-objects-emails-email-id-archive` — Move an Object identified by `{emailId}` to the recycling bin.
- `hubspot-pp-cli emails-crm get-v3-objects-emails-email-id-get-by-id` — Read an Object identified by `{emailId}`. `{emailId}` refers to the internal object ID by default, or optionally any...
- `hubspot-pp-cli emails-crm get-v3-objects-emails-get-page` — Read a page of emails. Control what is returned via the `properties` query param.
- `hubspot-pp-cli emails-crm patch-v3-objects-emails-email-id-update` — Perform a partial update of an Object identified by `{emailId}`or optionally a unique property value as specified by...
- `hubspot-pp-cli emails-crm post-v3-objects-emails-batch-archive-archive` — Archive a batch of emails identified by their IDs.
- `hubspot-pp-cli emails-crm post-v3-objects-emails-batch-create-create` — Create a batch of emails with specified properties and return the created objects.
- `hubspot-pp-cli emails-crm post-v3-objects-emails-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value...
- `hubspot-pp-cli emails-crm post-v3-objects-emails-batch-update-update` — Update a batch of emails using their internal IDs or unique property values.
- `hubspot-pp-cli emails-crm post-v3-objects-emails-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param....
- `hubspot-pp-cli emails-crm post-v3-objects-emails-create` — Create a email with the given properties and return a copy of the object, including the ID. Documentation and...
- `hubspot-pp-cli emails-crm post-v3-objects-emails-search-do-search` — Perform a search for emails based on the provided query parameters and return matching results.

**groups** — Manage groups

- `hubspot-pp-cli groups delete-crm-v3-properties-object-type-name-archive` — Move a property group identified by {groupName} to the recycling bin.
- `hubspot-pp-cli groups get-crm-v3-properties-object-type-get-all` — Read all existing property groups for the specified object type and HubSpot account.
- `hubspot-pp-cli groups get-crm-v3-properties-object-type-name-get-by-name` — Read a property group identified by {groupName}.
- `hubspot-pp-cli groups patch-crm-v3-properties-object-type-name-update` — Perform a partial update of a property group identified by {groupName}. Provided fields will be overwritten.
- `hubspot-pp-cli groups post-crm-v3-properties-object-type-create` — Create and return a copy of a new property group.

**leads-crm** — Manage crm

- `hubspot-pp-cli leads-crm delete-v3-objects-leads-leads-id-archive` — Move an Object identified by `{leadsId}` to the recycling bin.
- `hubspot-pp-cli leads-crm get-v3-objects-leads-get-page` — Read a page of leads. Control what is returned via the `properties` query param.
- `hubspot-pp-cli leads-crm get-v3-objects-leads-leads-id-get-by-id` — Read an Object identified by `{leadsId}`. `{leadsId}` refers to the internal object ID by default, or optionally any...
- `hubspot-pp-cli leads-crm patch-v3-objects-leads-leads-id-update` — Perform a partial update of an Object identified by `{leadsId}`or optionally a unique property value as specified by...
- `hubspot-pp-cli leads-crm post-v3-objects-leads-batch-archive-archive` — Archive multiple leads by their IDs in a single request, moving them to the recycling bin.
- `hubspot-pp-cli leads-crm post-v3-objects-leads-batch-create-create` — Create multiple lead records in a single request by providing a batch of lead data. This endpoint allows for...
- `hubspot-pp-cli leads-crm post-v3-objects-leads-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value...
- `hubspot-pp-cli leads-crm post-v3-objects-leads-batch-update-update` — Update multiple lead records using their internal IDs or unique property values. This endpoint allows batch...
- `hubspot-pp-cli leads-crm post-v3-objects-leads-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param....
- `hubspot-pp-cli leads-crm post-v3-objects-leads-create` — Create a lead with the given properties and return a copy of the object, including the ID. Documentation and...
- `hubspot-pp-cli leads-crm post-v3-objects-leads-search-do-search` — Perform a search for leads based on the provided filter groups, properties, and sorting options. The request allows...

**meetings-crm** — Manage crm

- `hubspot-pp-cli meetings-crm delete-v3-objects-meetings-meeting-id-archive` — Move an Object identified by `{meetingId}` to the recycling bin.
- `hubspot-pp-cli meetings-crm get-v3-objects-meetings-get-page` — Read a page of meetings. Control what is returned via the `properties` query param.
- `hubspot-pp-cli meetings-crm get-v3-objects-meetings-meeting-id-get-by-id` — Read an Object identified by `{meetingId}`. `{meetingId}` refers to the internal object ID by default, or optionally...
- `hubspot-pp-cli meetings-crm patch-v3-objects-meetings-meeting-id-update` — Perform a partial update of an Object identified by `{meetingId}`or optionally a unique property value as specified...
- `hubspot-pp-cli meetings-crm post-v3-objects-meetings-batch-archive-archive` — Archive a batch of meetings by ID
- `hubspot-pp-cli meetings-crm post-v3-objects-meetings-batch-create-create` — Create a batch of meetings
- `hubspot-pp-cli meetings-crm post-v3-objects-meetings-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value...
- `hubspot-pp-cli meetings-crm post-v3-objects-meetings-batch-update-update` — Update a batch of meetings by internal ID, or unique property values
- `hubspot-pp-cli meetings-crm post-v3-objects-meetings-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param....
- `hubspot-pp-cli meetings-crm post-v3-objects-meetings-create` — Create a meeting with the given properties and return a copy of the object, including the ID. Documentation and...
- `hubspot-pp-cli meetings-crm post-v3-objects-meetings-search-do-search` — Post v3 objects meetings search do search

**notes-crm** — Manage crm

- `hubspot-pp-cli notes-crm delete-v3-objects-notes-note-id-archive` — Move an Object identified by `{noteId}` to the recycling bin.
- `hubspot-pp-cli notes-crm get-v3-objects-notes-get-page` — Read a page of notes. Control what is returned via the `properties` query param.
- `hubspot-pp-cli notes-crm get-v3-objects-notes-note-id-get-by-id` — Read an Object identified by `{noteId}`. `{noteId}` refers to the internal object ID by default, or optionally any...
- `hubspot-pp-cli notes-crm patch-v3-objects-notes-note-id-update` — Perform a partial update of an Object identified by `{noteId}`or optionally a unique property value as specified by...
- `hubspot-pp-cli notes-crm post-v3-objects-notes-batch-archive-archive` — Archive multiple notes by their IDs in a single request. This operation moves the specified notes to the recycling...
- `hubspot-pp-cli notes-crm post-v3-objects-notes-batch-create-create` — Create multiple notes in a single request by providing the necessary properties for each note. This operation...
- `hubspot-pp-cli notes-crm post-v3-objects-notes-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value...
- `hubspot-pp-cli notes-crm post-v3-objects-notes-batch-update-update` — Update multiple notes using their internal IDs or unique property values. This operation allows you to modify the...
- `hubspot-pp-cli notes-crm post-v3-objects-notes-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param....
- `hubspot-pp-cli notes-crm post-v3-objects-notes-create` — Create a note with the given properties and return a copy of the object, including the ID. Documentation and...
- `hubspot-pp-cli notes-crm post-v3-objects-notes-search-do-search` — Execute a search for notes using filters, sorting options, and other query parameters to refine the results. This...

**pipelines-crm** — Manage crm

- `hubspot-pp-cli pipelines-crm delete-v3-pipelines-object-type-pipeline-id-archive` — Delete a pipeline
- `hubspot-pp-cli pipelines-crm delete-v3-pipelines-object-type-pipeline-id-stages-stage-id-archive` — Delete a pipeline stage
- `hubspot-pp-cli pipelines-crm get-v3-pipelines-object-type-get-all` — Return all pipelines for the object type specified by `{objectType}`.
- `hubspot-pp-cli pipelines-crm get-v3-pipelines-object-type-pipeline-id-audit-get-audit` — Return a reverse chronological list of all mutations that have occurred on the pipeline identified by `{pipelineId}`.
- `hubspot-pp-cli pipelines-crm get-v3-pipelines-object-type-pipeline-id-get-by-id` — Return a single pipeline object identified by its unique `{pipelineId}`.
- `hubspot-pp-cli pipelines-crm get-v3-pipelines-object-type-pipeline-id-stages-get-all` — Return all the stages associated with the pipeline identified by `{pipelineId}`.
- `hubspot-pp-cli pipelines-crm get-v3-pipelines-object-type-pipeline-id-stages-stage-id-audit-get-audit` — Return a reverse chronological list of all mutations that have occurred on the pipeline stage identified by `{stageId}`.
- `hubspot-pp-cli pipelines-crm get-v3-pipelines-object-type-pipeline-id-stages-stage-id-get-by-id` — Return a pipeline stage by ID
- `hubspot-pp-cli pipelines-crm patch-v3-pipelines-object-type-pipeline-id-stages-stage-id-update` — Patch v3 pipelines object type pipeline id stages stage id update
- `hubspot-pp-cli pipelines-crm patch-v3-pipelines-object-type-pipeline-id-update` — Perform a partial update of the pipeline identified by `{pipelineId}`. The updated pipeline will be returned in the...
- `hubspot-pp-cli pipelines-crm post-v3-pipelines-object-type-create` — Create a new pipeline with the provided property values. The entire pipeline object, including its unique ID, will...
- `hubspot-pp-cli pipelines-crm post-v3-pipelines-object-type-pipeline-id-stages-create` — Create a pipeline stage
- `hubspot-pp-cli pipelines-crm put-v3-pipelines-object-type-pipeline-id-replace` — Replace a pipeline
- `hubspot-pp-cli pipelines-crm put-v3-pipelines-object-type-pipeline-id-stages-stage-id-replace` — Replace all the properties of an existing pipeline stage with the values provided. The updated stage will be...

**properties-crm** — Manage crm

- `hubspot-pp-cli properties-crm delete-v3-properties-object-type-property-name-archive` — Move a property identified by {propertyName} to the recycling bin.
- `hubspot-pp-cli properties-crm get-v3-properties-object-type-get-all` — Read all existing properties for the specified object type and HubSpot account.
- `hubspot-pp-cli properties-crm get-v3-properties-object-type-property-name-get-by-name` — Read a property identified by {propertyName}.
- `hubspot-pp-cli properties-crm patch-v3-properties-object-type-property-name-update` — Perform a partial update of a property identified by { propertyName }. Provided fields will be overwritten.
- `hubspot-pp-cli properties-crm post-v3-properties-object-type-create` — Create and return a copy of a new property for the specified object type.

**tasks-crm** — Manage crm

- `hubspot-pp-cli tasks-crm delete-v3-objects-tasks-task-id-archive` — Move an Object identified by `{taskId}` to the recycling bin.
- `hubspot-pp-cli tasks-crm get-v3-objects-tasks-get-page` — Read a page of tasks. Control what is returned via the `properties` query param.
- `hubspot-pp-cli tasks-crm get-v3-objects-tasks-task-id-get-by-id` — Read an Object identified by `{taskId}`. `{taskId}` refers to the internal object ID by default, or optionally any...
- `hubspot-pp-cli tasks-crm patch-v3-objects-tasks-task-id-update` — Perform a partial update of an Object identified by `{taskId}`or optionally a unique property value as specified by...
- `hubspot-pp-cli tasks-crm post-v3-objects-tasks-batch-archive-archive` — Archive a batch of tasks by their IDs, moving them to the recycling bin. This operation requires a list of task IDs...
- `hubspot-pp-cli tasks-crm post-v3-objects-tasks-batch-create-create` — Create multiple tasks in a single request by providing a batch of task properties and associations. This endpoint...
- `hubspot-pp-cli tasks-crm post-v3-objects-tasks-batch-read-read` — Retrieve records by record ID or include the `idProperty` parameter to retrieve records by a custom unique value...
- `hubspot-pp-cli tasks-crm post-v3-objects-tasks-batch-update-update` — Update multiple tasks in a single request using their internal IDs or unique property values. This operation allows...
- `hubspot-pp-cli tasks-crm post-v3-objects-tasks-batch-upsert-upsert` — Create or update records identified by a unique property value as specified by the `idProperty` query param....
- `hubspot-pp-cli tasks-crm post-v3-objects-tasks-create` — Create a task with the given properties and return a copy of the object, including the ID. Documentation and...
- `hubspot-pp-cli tasks-crm post-v3-objects-tasks-search-do-search` — Execute a search for tasks based on the provided criteria, including filters, properties, and sorting options. This...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
hubspot-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find this week's stale leads, just the fields you need

```bash
hubspot-pp-cli stale-leads --stage leadqualified --days 14 --agent --select rows.contactId,rows.email,rows.days_since_touch,rows.last_engagement_type
```

Narrowed projection over a deeply-nested response keeps the per-call token cost flat regardless of how many leads come back.

### Monday pipeline rollup as a single tool call

```bash
hubspot-pp-cli pipeline-health --agent --select rows.stage,rows.dealId,rows.dealname,rows.weighted_value,rows.age_in_stage_days
```

Combines deal amounts with stage probability into a one-shot weighted view. Agents asked 'where is revenue stuck?' get a stage-ranked answer in one call.

### Dedup an Apollo batch without burning rate limit

```bash
hubspot-pp-cli dedup --key email --threshold strict --agent
```

Local GROUP BY over the synced mirror returns duplicate clusters in milliseconds. Replaces the per-lead API probe that today burns most of an hourly flow's daily budget.

### Closed Won handoff bundle to ClickUp

```bash
hubspot-pp-cli closed-won-handoff --since 7d --format clickup
```

Joins property-history transitions with the association graph and serializes the full bundle in import-ready shape — bridges the 'pre-sale in HubSpot, post-sale elsewhere' rule in one command.

### Daily digest for the Monday standup

```bash
hubspot-pp-cli digest --since 24h --agent
```

One JSON object: new contacts, deals advanced, deals closed, top-5 stalest, top-5 newest intake. Composes the hot pre-sale queries into a single tool call.

## Auth Setup

Authenticate with a HubSpot private app token. Create the app at https://app.hubspot.com/private-apps, grant read scopes for contacts, deals, companies, leads, owners, properties, lists, associations, and engagements, then export `HUBSPOT_TOKEN`. No OAuth, no callback handshake — single bearer token, scoped per private app.

Run `hubspot-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  hubspot-pp-cli associations-crm delete-v4-objects-object-type-object-id-associations-to-object-type-to-object-id-archive mock-value mock-value mock-value mock-value --agent --select id,name,status
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
hubspot-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
hubspot-pp-cli feedback --stdin < notes.txt
hubspot-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.hubspot-pp-cli/feedback.jsonl`. They are never POSTed unless `HUBSPOT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HUBSPOT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
hubspot-pp-cli --profile briefing associations-crm delete-v4-objects-object-type-object-id-associations-to-object-type-to-object-id-archive mock-value mock-value mock-value mock-value
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
