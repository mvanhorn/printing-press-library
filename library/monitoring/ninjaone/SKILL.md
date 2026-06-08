---
name: pp-ninjaone
description: "Every NinjaOne endpoint plus fleet-wide cross-org commands the org-scoped console can't give you. Trigger phrases: `check patch compliance across all orgs`, `show me the alert storm`, `find stale devices in ninjaone`, `which devices are missing patches`, `triage today's ninjaone alerts`, `use ninjaone`, `run ninjaone`."
author: "\"Chris Carson\""
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ninjaone-pp-cli
    install:
      - kind: go
        bins: [ninjaone-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/monitoring/ninjaone/cmd/ninjaone-pp-cli
---

# NinjaOne — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ninjaone-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install ninjaone --cli-only
   ```
2. Verify: `ninjaone-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/ninjaone/cmd/ninjaone-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

A complete CLI and MCP for the NinjaOne RMM API: all 300+ endpoints reachable, the high-value ones promoted to ergonomic commands, and a local SQLite store that powers offline fleet-wide search and cross-org rollups. Commands like patch-gaps, alert-storms, and stale-devices answer questions the per-organization console and API cannot, and an adaptive Retry-After limiter keeps bulk operations alive under NinjaOne's throttling.

## When to Use This CLI

Use this CLI when an agent or technician needs to query, report on, or act across an entire NinjaOne fleet — especially across multiple client organizations at once. It is the right tool for patch-compliance reporting, alert-storm triage, stale-device hygiene, and any bulk device action that would otherwise require clicking through org-scoped dashboards. The local store makes fleet-wide questions cheap and keeps bulk operations from hitting rate limits.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for live remote-control sessions or screen sharing; that is the NinjaOne console's job.
- Do not use it to configure agent installers or deployment packages; use the NinjaOne web UI.
- Do not use it as a real-time monitoring dashboard; it is a query/report/act tool, not a live feed beyond `tail`.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-org fleet intelligence
- **`patch-gaps`** — See every device still missing or failing a patch across all your client organizations in one fleet-wide view.

  _Reach for this when an agent needs the org-by-org patch posture in one shot instead of paging the per-org dashboard._

  ```bash
  ninjaone-pp-cli patch-gaps --severity critical --agent
  ```
- **`patch-sweep`** — Scan and apply patches across a device cohort, serialized through an adaptive Retry-After limiter so bulk runs survive NinjaOne's throttling.

  _Use when an agent must remediate a cohort of devices and naive bulk apply would melt the API._

  ```bash
  ninjaone-pp-cli patch-sweep --df "status eq OFFLINE" --dry-run --agent
  ```
- **`patch-stuck`** — Surface KBs that have been failing or pending across multiple consecutive syncs, not just at this moment.

  _Use when an agent needs the chronically-broken patches worth manual intervention, not the transient ones._

  ```bash
  ninjaone-pp-cli patch-stuck --cycles 3 --agent
  ```

### Alert triage
- **`alert-storms`** — Collapse a flood of active alerts into ranked incidents grouped by organization, condition, and time window.

  _Reach for this during an alert flood to tell one site-wide incident from many unrelated alerts._

  ```bash
  ninjaone-pp-cli alert-storms --window 1h --agent
  ```
- **`alert-clear`** — Reset every alert matching a predicate (condition, organization, or age) in one throttled, dry-run-first operation.

  _Use to clear a weekend's worth of accumulated or auto-resolved alerts without clicking each one._

  ```bash
  ninjaone-pp-cli alert-clear --where "age>72h" --dry-run --agent
  ```
- **`alert-flappers`** — Rank conditions that repeatedly fire and auto-resolve over a window, by cycle count per device and condition.

  _Use to find noisy monitors worth tuning instead of resetting nightly._

  ```bash
  ninjaone-pp-cli alert-flappers --window 7d --agent
  ```

### Fleet hygiene
- **`stale-devices`** — Find stale or offline devices with no contact in N days across every organization, grouped by org, with an optional throttled cohort reboot.

  _Reach for this during fleet-hygiene sweeps to find decommission candidates fleet-wide._

  ```bash
  ninjaone-pp-cli stale-devices --offline-days 30 --agent
  ```
- **`cf-hygiene`** — Find devices and organizations missing required custom-field values (asset tag, warranty, owner) fleet-wide.

  _Use when an agent must enforce documentation SOPs across the whole fleet before an audit._

  ```bash
  ninjaone-pp-cli cf-hygiene --require assetTag,warrantyEnd --agent
  ```

## Command Reference

**activities** — Manage activities

- `ninjaone-pp-cli activities` — Returns activity log in reverse chronological order

**alert** — Manage alert

- `ninjaone-pp-cli alert <uid>` — Resets alert/condition by UID

**alerts** — Manage alerts

- `ninjaone-pp-cli alerts` — Returns list of active alerts/triggered conditions

**attachments** — Manage attachments

- `ninjaone-pp-cli attachments` — Upload temporary attachments

**automation** — Manage automation

- `ninjaone-pp-cli automation` — Returns list of all available automation scripts

**backup** — Backup

- `ninjaone-pp-cli backup get-integrity-check-jobs` — Returns a list of integrity check jobs.
- `ninjaone-pp-cli backup get-jobs` — Returns list of backup jobs
- `ninjaone-pp-cli backup set-bandwidth-throttle-for-device` — Sets the bandwidth throttle for a device
- `ninjaone-pp-cli backup submit-integrity-check-job` — Creates an integrity check job

**billing** — Billing

- `ninjaone-pp-cli billing activate-product` — Activates a previously deactivated product given its ID. **Note:** This endpoint returns 204 No Content on success.
- `ninjaone-pp-cli billing approve-invoices` — Approves one or more invoices by changing their status from PENDING to APPROVED.
- `ninjaone-pp-cli billing archive-invoices` — Archives one or more invoices by changing their status to ARCHIVED.
- `ninjaone-pp-cli billing create-1` — Creates an agreement with all his properties and products.
- `ninjaone-pp-cli billing create-account` — Creates a new account given its content
- `ninjaone-pp-cli billing create-adhoc-ticket-products` — Creates a ticket product that is not associated to a product and attaches it to a ticket.
- `ninjaone-pp-cli billing create-catalog-ticket-products` — Creates a ticket product that is associated to a product and attaches it to a ticket.
- `ninjaone-pp-cli billing create-invoice` — Creates a new invoice for the specified agreement.
- `ninjaone-pp-cli billing create-product` — Creates a new product in the catalog.
- `ninjaone-pp-cli billing deactivate-agreement` — Deactivates an agreement given its ID
- `ninjaone-pp-cli billing deactivate-product` — Deactivates a previously activated product given its ID. **Note:** This endpoint returns 204 No Content on success.
- `ninjaone-pp-cli billing delete-account` — Deletes a previously created account given its ID
- `ninjaone-pp-cli billing export-invoices` — Exports approved invoices to be marked as completed.
- `ninjaone-pp-cli billing find-agreement-by-id` — Retrieves an agreement with a specific ID.
- `ninjaone-pp-cli billing find-agreements` — Retrieves an agreement list given some filter parameters.
- `ninjaone-pp-cli billing get-account-by-id` — Retrieve a specific account given its ID
- `ninjaone-pp-cli billing get-all-accounts` — Returns a list with all existing accounts.
- `ninjaone-pp-cli billing get-by-ticket-id` — Returns a list with all existing products associated to a ticket given its ID.
- `ninjaone-pp-cli billing get-invoice-by-id` — Returns a single invoice given its ID
- `ninjaone-pp-cli billing get-invoices` — Returns a list of invoices with optional filters
- `ninjaone-pp-cli billing get-product-by-id` — Retrieves a product by its unique identifier.
- `ninjaone-pp-cli billing get-products` — Returns a list of products with optional filters.
- `ninjaone-pp-cli billing remove-ticket-products` — Removes a list of ticket products given its IDs from a ticket given its ID
- `ninjaone-pp-cli billing unarchive-invoices` — Unarchives one or more invoices by changing their status from ARCHIVED to PENDING.
- `ninjaone-pp-cli billing update-1` — Updates an agreement with all his properties and products.
- `ninjaone-pp-cli billing update-account` — Updates a previously created account given its ID and updated content
- `ninjaone-pp-cli billing update-invoice` — Updates a previously created invoice given its ID and updated content
- `ninjaone-pp-cli billing update-invoice-note` — Updates only the note field of an invoice. This operation is allowed even for COMPLETE invoices.
- `ninjaone-pp-cli billing update-product` — Updates a previously created product given its ID and updated content.
- `ninjaone-pp-cli billing update-psa-properties` — Updates the PSA billing properties of a ticket time entry.
- `ninjaone-pp-cli billing update-ticket-products` — Updates a ticket product from a ticket.

**checklist** — Manage checklist

- `ninjaone-pp-cli checklist archive-template` — Archive a checklist template by id
- `ninjaone-pp-cli checklist create-templates` — Creates multiple checklist templates
- `ninjaone-pp-cli checklist delete-template` — Delete a checklist template by id
- `ninjaone-pp-cli checklist delete-templates` — Deletes checklist templates by id
- `ninjaone-pp-cli checklist get-templates` — List checklists templates with given criteria
- `ninjaone-pp-cli checklist restore-template` — Restore a checklist template by id
- `ninjaone-pp-cli checklist update-templates` — Updates multiple checklist templates

**contact** — Manage contact

- `ninjaone-pp-cli contact delete` — Delete a contact by their ID
- `ninjaone-pp-cli contact get-by-id` — Get a contact by their ID
- `ninjaone-pp-cli contact update` — Update a contact by their ID

**contacts** — Manage contacts

- `ninjaone-pp-cli contacts create` — Create a new contact
- `ninjaone-pp-cli contacts get` — Get all contacts

**custom-fields** — Custom Fields

- `ninjaone-pp-cli custom-fields bulk-create-node-attributes` — Creates multiple custom fields in a single request. All operations succeed or all fail.
- `ninjaone-pp-cli custom-fields bulk-delete-node-attributes` — Deletes multiple custom fields in a single request. All operations succeed or all fail.
- `ninjaone-pp-cli custom-fields bulk-update-node-attributes` — Updates multiple custom fields in a single request. All operations succeed or all fail.
- `ninjaone-pp-cli custom-fields create-node-attribute` — Creates a new custom field (node attribute) with the specified configuration
- `ninjaone-pp-cli custom-fields delete-node-attribute-by-field-name` — Deletes an existing custom field identified by its unique field name
- `ninjaone-pp-cli custom-fields get-all-node-attributes` — Retrieves custom fields (node attributes) with pagination support. Maximum 500 items per page.
- `ninjaone-pp-cli custom-fields get-node-attribute-by-field-name` — Retrieves a specific custom field by its unique field name
- `ninjaone-pp-cli custom-fields get-node-attribute-signed-urls` — Get custom field signed urls
- `ninjaone-pp-cli custom-fields update-node-attribute-by-field-name` — Updates an existing custom field identified by its unique field name

**device** — Devices

- `ninjaone-pp-cli device get` — Returns device details
- `ninjaone-pp-cli device update` — Change device friendly name, user data, etc.

**device-custom-fields** — Manage device custom fields

- `ninjaone-pp-cli device-custom-fields` — Returns list of all custom fields

**devices** — Devices

- `ninjaone-pp-cli devices get` — Returns list of devices (basic node information)
- `ninjaone-pp-cli devices node-approval-operation` — Approve or reject devices that are waiting for approval
- `ninjaone-pp-cli devices search` — Returns list of entities matching search term

**devices-detailed** — Manage devices detailed

- `ninjaone-pp-cli devices-detailed` — Returns list of devices with additional information

**document-templates** — Document Templates

- `ninjaone-pp-cli document-templates archive` — Archives multiple document template by ids
- `ninjaone-pp-cli document-templates create` — Create document template
- `ninjaone-pp-cli document-templates delete` — Deletes a document template by id
- `ninjaone-pp-cli document-templates get` — Get document template
- `ninjaone-pp-cli document-templates get-with-attributes` — List document templates with fields
- `ninjaone-pp-cli document-templates restore` — Restores a document template by id
- `ninjaone-pp-cli document-templates update` — Updates a document template by id

**group** — Groups/Search


**groups** — Groups/Search

- `ninjaone-pp-cli groups` — List groups (saved searches)

**itam** — Manage itam

- `ninjaone-pp-cli itam create-asset-relationship` — Creates one or more relationships between assets. Returns both successful creations and any failures.
- `ninjaone-pp-cli itam create-relationship-type` — Creates a new relationship type to be used when creating relationships between assets.
- `ninjaone-pp-cli itam create-unmanaged-device-public-api` — Create an Unmanaged Device with the provided details
- `ninjaone-pp-cli itam decommission-unmanaged-device-public-api` — Decommission an Unmanaged Device with the provided id
- `ninjaone-pp-cli itam decommission-unmanaged-device-public-api-list` — Decommission an Unmanaged Device List with the provided ids
- `ninjaone-pp-cli itam delete-asset-relationships` — Deletes one or more relationships between assets by their unique identifiers.
- `ninjaone-pp-cli itam delete-unmanaged-device-public-api` — Delete an Unmanaged Device with the provided id
- `ninjaone-pp-cli itam get-all-relationship-types` — Returns a paginated list of all relationship types, including both system default and user created types
- `ninjaone-pp-cli itam get-all-relationships` — Returns a paginated list of all asset relationships, optionally filtered by relationship type.
- `ninjaone-pp-cli itam get-entity-relations` — Returns a paginated list of all asset relationships for a specific entity
- `ninjaone-pp-cli itam update-unmanaged-device-public-api` — Update an Unmanaged Device with the provided details

**knowledgebase** — Manage knowledgebase

- `ninjaone-pp-cli knowledgebase archive-knowledge-base-articles` — Archive knowledge base articles
- `ninjaone-pp-cli knowledgebase archive-knowledge-base-folders` — Archive knowledge base folders
- `ninjaone-pp-cli knowledgebase create-knowledge-base-articles` — Create knowledge base articles
- `ninjaone-pp-cli knowledgebase delete-knowledge-base-articles` — Delete knowledge base articles
- `ninjaone-pp-cli knowledgebase delete-knowledge-base-folders` — Delete knowledge base folders
- `ninjaone-pp-cli knowledgebase download-knowledge-base-article` — Download knowledge base article
- `ninjaone-pp-cli knowledgebase get-client-knowledge-base-articles` — Lists organization knowledge base articles
- `ninjaone-pp-cli knowledgebase get-global-knowledge-base-articles` — Lists global knowledge base articles
- `ninjaone-pp-cli knowledgebase get-knowledge-base-article-signed-urls` — Get knowledge base article signed urls
- `ninjaone-pp-cli knowledgebase get-knowledge-base-folder-content` — Returns knowledge base folder and its content
- `ninjaone-pp-cli knowledgebase get-knowledge-base-folder-path-content` — Returns knowledge base folder and its content
- `ninjaone-pp-cli knowledgebase move` — Move knowledge base folders and documents to another knowledge base folder
- `ninjaone-pp-cli knowledgebase restore-knowledge-base-articles` — Restore archived knowledge base articles
- `ninjaone-pp-cli knowledgebase restore-knowledge-base-folders` — Restore archived knowledge base folders
- `ninjaone-pp-cli knowledgebase update-knowledge-base-articles` — Update knowledge base articles
- `ninjaone-pp-cli knowledgebase upload-knowledge-base-articles` — Upload knowledge base articles

**locations** — Location

- `ninjaone-pp-cli locations` — Returns flat list of all locations for all organizations

**ninjaone-public-jobs** — Manage ninjaone public jobs

- `ninjaone-pp-cli ninjaone-public-jobs` — Returns list of running jobs

**noderole** — Manage noderole

- `ninjaone-pp-cli noderole create-node-roles` — Create multiple node roles
- `ninjaone-pp-cli noderole delete-node-roles` — Delete node roles
- `ninjaone-pp-cli noderole get-node-roles` — Get all node roles
- `ninjaone-pp-cli noderole update-node-roles` — Update multiple node roles

**notification-channels** — Manage notification channels

- `ninjaone-pp-cli notification-channels get` — Returns list of notification channels
- `ninjaone-pp-cli notification-channels get-enabled` — Returns list of enabled notification channels

**organization** — Organizations

- `ninjaone-pp-cli organization archive-checklists` — Archive multiple organization checklists
- `ninjaone-pp-cli organization archive-client-document` — Archives an organization document by id
- `ninjaone-pp-cli organization archive-multi-page-client-documents` — Archives multiple organization documents by id
- `ninjaone-pp-cli organization create-checklists` — Creates multiple organization checklists
- `ninjaone-pp-cli organization create-documents` — Creates organization documents and returns the documents created
- `ninjaone-pp-cli organization delete-client-checklist` — Deletes an organization checklist by id
- `ninjaone-pp-cli organization delete-client-checklists` — Deletes organization checklists by id
- `ninjaone-pp-cli organization delete-client-document` — Deletes an archived organization document by id
- `ninjaone-pp-cli organization get` — Returns organization details (policy mappings, locations)
- `ninjaone-pp-cli organization get-client-checklist` — Get a client checklist by id
- `ninjaone-pp-cli organization get-client-checklist-signed-urls` — Get organization checklist signed urls
- `ninjaone-pp-cli organization get-client-checklists` — List client checklists with given criteria
- `ninjaone-pp-cli organization get-client-document-signed-urls` — Get organization document signed urls
- `ninjaone-pp-cli organization get-client-documents-with-attribute-values` — List all organization documents with field values
- `ninjaone-pp-cli organization get-installer` — Generates and returns URL for installer with specified settings
- `ninjaone-pp-cli organization promote-client-checklists` — Promote organization checklists by id
- `ninjaone-pp-cli organization promote-client-checklists-1` — Promote organization checklists by id
- `ninjaone-pp-cli organization restore-checklists` — Restore multiple organization checklists
- `ninjaone-pp-cli organization restore-client-document` — Restores an organization document by id
- `ninjaone-pp-cli organization restore-multi-page-client-documents` — Restore multiple multi page organization documents
- `ninjaone-pp-cli organization update` — Change organization name, description and policy mappings
- `ninjaone-pp-cli organization update-checklists` — Updates multiple organization checklists
- `ninjaone-pp-cli organization update-documents` — Updates organization documents and returns the documents updated

**organizations** — Organizations

- `ninjaone-pp-cli organizations create` — Creates new organization with optional list of locations and policy mappings.
- `ninjaone-pp-cli organizations get` — Returns list of organizations (Brief mode)

**organizations-detailed** — Manage organizations detailed

- `ninjaone-pp-cli organizations-detailed` — Returns list of organizations with locations and policy mappings

**policies** — Manage policies

- `ninjaone-pp-cli policies create-policy` — Creates new policy using (New Root, Child, Copy)
- `ninjaone-pp-cli policies get` — Returns list of policies

**queries** — Queries

- `ninjaone-pp-cli queries get-antivirus-status-report` — Returns list of statues of antivirus software installed on devices
- `ninjaone-pp-cli queries get-antivirus-threats` — Returns list of antivirus threats
- `ninjaone-pp-cli queries get-computer-systems` — Returns computer systems information for devices
- `ninjaone-pp-cli queries get-custom-fields-detailed-report` — Returns Custom Fields report with additional information about each field
- `ninjaone-pp-cli queries get-custom-fields-report` — Returns Custom Fields report
- `ninjaone-pp-cli queries get-device-health-report` — Returns list of device health summary records
- `ninjaone-pp-cli queries get-device-usage` — Returns the backup usage by device
- `ninjaone-pp-cli queries get-disk-drives` — Returns list of physical disks
- `ninjaone-pp-cli queries get-installed-ospatches` — Returns patch installation history records (successful and failed)
- `ninjaone-pp-cli queries get-installed-software-patches` — Returns 3rd party software patch installation history records (successful and failed)
- `ninjaone-pp-cli queries get-last-logged-on-users-report` — Returns usernames and logon times
- `ninjaone-pp-cli queries get-network-interfaces` — Returns list of Network Interfaces for each device
- `ninjaone-pp-cli queries get-operating-systems` — Returns operating systems' for devices
- `ninjaone-pp-cli queries get-pending-failed-rejected-ospatches` — Returns list of OS patches for which there were no installation attempts
- `ninjaone-pp-cli queries get-pending-failed-rejected-software-patches` — Returns list of 3rd party Software patches for which there were no installation attempts
- `ninjaone-pp-cli queries get-policy-overrides-1` — Returns list of overridden policy sections for each device
- `ninjaone-pp-cli queries get-processors` — Returns list of processors
- `ninjaone-pp-cli queries get-raidcontroller-report` — Returns list of RAID controllers
- `ninjaone-pp-cli queries get-raiddrive-report` — Returns list of drives connected to RAID controllers
- `ninjaone-pp-cli queries get-scoped-custom-fields-detailed-report` — Returns report for Custom Fields defined at different scopes (device, location, organization)
- `ninjaone-pp-cli queries get-scoped-custom-fields-report` — Returns report for Custom Fields defined at different scopes (device, location, organization)
- `ninjaone-pp-cli queries get-software` — Returns list software installed on devices
- `ninjaone-pp-cli queries get-volumes` — Returns list of disk volumes
- `ninjaone-pp-cli queries get-windows-services-report` — Returns list of Windows Services and their statuses

**related-items** — Related Items

- `ninjaone-pp-cli related-items create` — Relate an attachment to an entity
- `ninjaone-pp-cli related-items create-for-entity` — Create a relation between two entities
- `ninjaone-pp-cli related-items create-for-entity-1` — Create multiple relations between two entities
- `ninjaone-pp-cli related-items create-secure-for-entity` — Create a relation to a secure value
- `ninjaone-pp-cli related-items delete` — Deletes related item
- `ninjaone-pp-cli related-items delete-relateditems` — Deletes related items associated with an entity
- `ninjaone-pp-cli related-items get-all` — List all related items
- `ninjaone-pp-cli related-items get-attachments-signed-urls` — Get related item attachments signed urls for an entity
- `ninjaone-pp-cli related-items get-for-host-entity` — List related items for a specific host entity filterable by scope
- `ninjaone-pp-cli related-items get-with-entity` — List related items for a specific related entity
- `ninjaone-pp-cli related-items get-with-entity-type` — List related entities for a related entity type
- `ninjaone-pp-cli related-items get-with-host-entity-type` — List relations and references for a host entity type

**roles** — Manage roles

- `ninjaone-pp-cli roles` — Returns list of device roles

**software-license** — Software Licenses

- `ninjaone-pp-cli software-license create` — Create a Software License with the provided name and description
- `ninjaone-pp-cli software-license delete` — Delete the Software License with the provided id
- `ninjaone-pp-cli software-license get-all` — Get All Software Licenses
- `ninjaone-pp-cli software-license get-by-id` — Get a Software License by Id
- `ninjaone-pp-cli software-license update` — Update a Software License with the provided metadata
- `ninjaone-pp-cli software-license upsert` — Create or Update a Software License in a simple way

**software-products** — Manage software products

- `ninjaone-pp-cli software-products` — Returns available software products (3rd party patching)

**staged-device** — Manage staged device

- `ninjaone-pp-cli staged-device` — Create staged device

**tab** — Manage tab

- `ninjaone-pp-cli tab create-custom-public-api` — Create a Custom Tab with the provided details
- `ninjaone-pp-cli tab delete-custom-public-api` — Delete a Custom Tab
- `ninjaone-pp-cli tab get-custom-public-api` — Gets a custom tab. NOTE: This will _not_ fetch tab extensions. You must use the GET tab/{tabId}/role/{roleId} for that
- `ninjaone-pp-cli tab get-summary-for-end-user` — Retrieve all of the custom tabs available to end user views
- `ninjaone-pp-cli tab get-summary-for-organization` — Retrieve all of the custom tabs available to organizations and locations
- `ninjaone-pp-cli tab get-summary-for-role` — Retrieve all of the custom tabs that would appear for the given role
- `ninjaone-pp-cli tab rename-custom-public-api` — Renames a Custom Tab
- `ninjaone-pp-cli tab update-custom-display` — Using this API it is possible to configure tabs to be hidden for roles and their children
- `ninjaone-pp-cli tab update-custom-public-api` — Update a Custom Tab.
- `ninjaone-pp-cli tab update-end-user-custom-order` — Update the order of custom tabs for end-user tabs. NOTE: All tabs defined for end-users must be specified in the payload
- `ninjaone-pp-cli tab update-organization-custom-order` — Update the order of custom tabs for organizations and locations.
- `ninjaone-pp-cli tab update-role-custom-order` — Update the order of custom tabs for a specific role. NOTE: Only tabs created on this role can be ordered.

**tag** — Manage tag

- `ninjaone-pp-cli tag batch-update` — Update tags for the supplied assetIds. Tags will be added and removed as specified
- `ninjaone-pp-cli tag create` — Create an Asset Tag with the provided name and description
- `ninjaone-pp-cli tag delete` — Delete Asset Tags having the provided ids
- `ninjaone-pp-cli tag delete-tagid` — Delete the Asset Tag with the provided id
- `ninjaone-pp-cli tag get` — Get a list of created Asset Tags
- `ninjaone-pp-cli tag merge` — Merges tags. Can merge into an existing or new tag depending on the input parameters
- `ninjaone-pp-cli tag set-for-asset` — Set the tags for an asset to exactly the supplied values
- `ninjaone-pp-cli tag update` — Update an Asset Tag with the provided metadata

**tasks** — Manage tasks

- `ninjaone-pp-cli tasks` — Returns list of registered scheduled tasks

**ticketing** — ticketing

- `ninjaone-pp-cli ticketing create-2` — Create a new ticket, does not accept files
- `ninjaone-pp-cli ticketing create-comment` — Add a new comment to a ticket, allows files
- `ninjaone-pp-cli ticketing get-all-statuses` — Get list of ticket status
- `ninjaone-pp-cli ticketing get-all-user-and-contacts` — Returns list of users (contacts, end-user, technician)
- `ninjaone-pp-cli ticketing get-boards` — Returns list of ticketing boards
- `ninjaone-pp-cli ticketing get-contacts-1` — Returns list of contacts
- `ninjaone-pp-cli ticketing get-ticket-attributes` — Returns list of the ticket attributes
- `ninjaone-pp-cli ticketing get-ticket-by-id` — Returns a ticket
- `ninjaone-pp-cli ticketing get-ticket-form-by-id` — Returns a ticket form with fields
- `ninjaone-pp-cli ticketing get-ticket-forms` — Returns list of ticket forms with their fields
- `ninjaone-pp-cli ticketing get-ticket-log-entries-by-ticket-id` — Returns list of the ticket log entries for a ticket
- `ninjaone-pp-cli ticketing get-tickets-by-board` — Run a board. Returns list of tickets matching the board condition and filters. Allows pagination
- `ninjaone-pp-cli ticketing update-2` — Change ticket fields. Does not accept comments

**user** — Users

- `ninjaone-pp-cli user add-role-members` — Add members to user role
- `ninjaone-pp-cli user create-end` — Create an end user
- `ninjaone-pp-cli user create-technician` — Create a new technician
- `ninjaone-pp-cli user delete-end` — Delete an end user
- `ninjaone-pp-cli user delete-technician` — Delete a technician by their ID
- `ninjaone-pp-cli user get-end` — Get details for a specific end user identifier
- `ninjaone-pp-cli user get-end-1` — Get all end users
- `ninjaone-pp-cli user get-node-custom-fields-3` — Returns list of end user custom fields
- `ninjaone-pp-cli user get-roles` — Get list of user roles
- `ninjaone-pp-cli user get-technician` — Get details for a specific technician identifier
- `ninjaone-pp-cli user get-technicians` — Get all technicians
- `ninjaone-pp-cli user patch-end` — Update a specific end user by their ID
- `ninjaone-pp-cli user patch-end-device-access` — Add or remove up to 100 accessible devices for a specific end user by their ID
- `ninjaone-pp-cli user remove-role-members` — Remove users from user role
- `ninjaone-pp-cli user update-node-attribute-values-3` — Update end user custom field values
- `ninjaone-pp-cli user update-technician` — Update technician by their ID

**users** — Users

- `ninjaone-pp-cli users` — Returns list of users

**vulnerability** — Manage vulnerability

- `ninjaone-pp-cli vulnerability fetch-all-scan-groups` — Fetches all Scan Groups.
- `ninjaone-pp-cli vulnerability fetch-scan-group-by-id` — Fetches a single Scan Group by ID.
- `ninjaone-pp-cli vulnerability update-scan-group` — Upload CSV to an existing scan group.

**webhook** — Webhook Endpoints

- `ninjaone-pp-cli webhook configure` — Creates or updates Webhook configuration for current application/client
- `ninjaone-pp-cli webhook disable` — Disables Webhook configuration for current application/client


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ninjaone-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Critical patch gaps as agent JSON, narrowed

```bash
ninjaone-pp-cli patch-gaps --severity critical --agent --select results.organizationName,results.systemName,results.kb,results.failedCount
```

Fleet-wide critical patch gaps with only the fields an agent needs, from the local store.

### Cluster the current alert storm

```bash
ninjaone-pp-cli alert-storms --window 1h --agent
```

Group active alerts into ranked incidents to find the one outage behind 50 alerts.

### Dry-run a cohort patch sweep

```bash
ninjaone-pp-cli patch-sweep --df "class eq WINDOWS_SERVER" --dry-run
```

Preview exactly which servers a patch sweep would scan and apply before mutating anything.

### Stale device sweep for one client

```bash
ninjaone-pp-cli stale-devices --offline-days 45 --org "Acme Corp"
```

Decommission candidates for a single organization, offline 45+ days.

### Find SOP custom-field gaps before an audit

```bash
ninjaone-pp-cli cf-hygiene --require assetTag,warrantyEnd --agent
```

List every device missing required documentation fields across the fleet.

## Auth Setup

NinjaOne uses OAuth2 client-credentials. Create a Client App ID under Administration -> Apps -> API, then set NINJAONE_CLIENT_ID and NINJAONE_CLIENT_SECRET. The CLI exchanges them for a bearer token at /ws/oauth/token (scopes monitoring, management, control) and refreshes it automatically. Set NINJAONE_INSTANCE if you are not on the US region (app.ninjarmm.com).

Run `ninjaone-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ninjaone-pp-cli activities --agent --select id,name,status
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
ninjaone-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ninjaone-pp-cli feedback --stdin < notes.txt
ninjaone-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/ninjaone-pp-cli/feedback.jsonl`. They are never POSTed unless `NINJAONE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NINJAONE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ninjaone-pp-cli profile save briefing --json
ninjaone-pp-cli --profile briefing activities
ninjaone-pp-cli profile list --json
ninjaone-pp-cli profile show briefing
ninjaone-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ninjaone-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/monitoring/ninjaone/cmd/ninjaone-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add ninjaone-pp-mcp -- ninjaone-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ninjaone-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ninjaone-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ninjaone-pp-cli <command> --help`.
