---
name: pp-klaviyo
description: "Klaviyo from the shell, with local customer-behavior analytics layered on top. Trigger phrases: `inspect a Klaviyo profile`, `deploy a Klaviyo campaign`, `check Klaviyo flow decay`, `reconcile Klaviyo campaign revenue`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - klaviyo-pp-cli
    install:
      - kind: go
        bins: [klaviyo-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/cmd/klaviyo-pp-cli
---

# Klaviyo — Printing Press CLI

This printed CLI wraps Klaviyo's JSON:API surface for profiles, events, campaigns, flows, segments, metrics, templates, and related marketing resources. It adds a local SQLite mirror plus compound commands for campaign deployment, flow decay, attribution, deduplication, reconciliation, and growth planning.

## When to Use This CLI

Use this CLI when an agent needs to inspect or operate Klaviyo without the web UI, especially for profiles, events, campaigns, flows, segments, templates, metrics, and compound growth workflows.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Campaign operations
- **`campaigns deploy`** — Create an email template, create a draft campaign, and assign the template to the campaign message in one audited workflow.

  _Agents can build a draft campaign without hand-stitching three endpoint calls._

  ```bash
  klaviyo-pp-cli campaigns deploy --template-html ./email.html --campaign-name "May offer" --list-id LIST_ID --subject "May offer" --from-email marketing@example.com --from-label "Marketing" --json
  ```
- **`campaigns image-swap`** — Find a campaign message template and replace an image URL inside the HTML while preserving the rest of the draft.

  _Agents can make safe creative swaps without rebuilding a whole campaign._

  ```bash
  klaviyo-pp-cli campaigns image-swap --campaign-id CAMPAIGN_ID --old-url https://cdn.example.com/old.jpg --new-url https://cdn.example.com/new.jpg --json
  ```

### Behavior graph analytics
- **`flow-decay`** — Identify flows whose open or click performance has decayed across recent time buckets using synced local data.

  _Agents can spot lifecycle automations that need refresh before revenue falls further._

  ```bash
  klaviyo-pp-cli flow-decay --days 90 --threshold 0.15 --json
  ```
- **`cohort`** — Group profiles by first event date and compute retention or repeat-action curves from synced profiles and events.

  _Agents can answer which acquisition cohorts keep buying without exporting CSVs._

  ```bash
  klaviyo-pp-cli cohort --metric "Placed Order" --interval month --json --select cohort,profiles,retained
  ```
- **`attribution`** — Join order events with campaign and flow attribution properties to summarize revenue by channel and source.

  _Agents can explain which automation or campaign generated revenue using local event evidence._

  ```bash
  klaviyo-pp-cli attribution --metric "Placed Order" --group-by flow --since 2026-01-01 --json
  ```

### Data hygiene
- **`dedup`** — Find profiles that appear duplicated by email, phone, or cross-channel collisions in the local profile mirror.

  _Agents can flag customer records that split behavior and revenue history across identities._

  ```bash
  klaviyo-pp-cli dedup --by email,phone --json
  ```
- **`reconcile`** — Compare campaign UTM evidence with local Klaviyo order events and optional Shopify credentials when available.

  _Agents can check whether campaign performance agrees with order evidence before reporting numbers._

  ```bash
  klaviyo-pp-cli reconcile --campaign-id CAMPAIGN_ID --since 2026-01-01 --json
  ```

### Growth planning
- **`plan brief-to-strategy`** — Turn a growth brief into a structured Klaviyo campaign, flow, segment, and experiment strategy.

  _Agents can convert strategy notes into a concrete Klaviyo execution plan._

  ```bash
  klaviyo-pp-cli plan brief-to-strategy --brief ./brief.md --json
  ```
- **`plan qa-gate`** — Run a launch-readiness checklist for links, offers, dates, timezone, fallback tokens, compliance, and deliverability flags.

  _Agents can block risky campaign launches with explicit findings instead of vague review notes._

  ```bash
  klaviyo-pp-cli plan qa-gate --campaign-id CAMPAIGN_ID --json
  ```

## Command Reference

**accounts** — accounts

- `klaviyo-pp-cli accounts get` — Retrieve the account(s) associated with a given private API key. This will return 1 account object within the array....
- `klaviyo-pp-cli accounts get-id` — Retrieve a single account object by its account ID. You can only request the account by which the private API key...

**back-in-stock-subscriptions** — Manage back in stock subscriptions

- `klaviyo-pp-cli back-in-stock-subscriptions` — Subscribe a profile to receive back in stock notifications. Check out [our Back in Stock API...

**campaign-clone** — Manage campaign clone

- `klaviyo-pp-cli campaign-clone` — Clones an existing campaign, returning a new campaign based on the original with a new ID and name.<br><br>*Rate...

**campaign-message-assign-template** — Manage campaign message assign template

- `klaviyo-pp-cli campaign-message-assign-template` — Creates a non-reusable version of the template and assigns it to the message.<br><br>*Rate limits*:<br>Burst:...

**campaign-messages** — Manage campaign messages

- `klaviyo-pp-cli campaign-messages get` — Returns a specific message based on a required id.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`...
- `klaviyo-pp-cli campaign-messages update` — Update a campaign message<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:** `campaigns:write`

**campaign-recipient-estimation-jobs** — Manage campaign recipient estimation jobs

- `klaviyo-pp-cli campaign-recipient-estimation-jobs get` — Retrieve the status of a recipient estimation job triggered with the `Create Campaign Recipient Estimation Job`...
- `klaviyo-pp-cli campaign-recipient-estimation-jobs refresh-campaign-recipient-estimation` — Trigger an asynchronous job to update the estimated number of recipients for the given campaign ID. Use the `Get...

**campaign-recipient-estimations** — Manage campaign recipient estimations

- `klaviyo-pp-cli campaign-recipient-estimations <id>` — Get the estimated recipient count for a campaign with the provided campaign ID. You can refresh this count by using...

**campaign-send-jobs** — Manage campaign send jobs

- `klaviyo-pp-cli campaign-send-jobs cancel-campaign-send` — Permanently cancel the campaign, setting the status to CANCELED or revert the campaign, setting the status back to...
- `klaviyo-pp-cli campaign-send-jobs get` — Get a campaign send job<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:** `campaigns:read`
- `klaviyo-pp-cli campaign-send-jobs send-campaign` — Trigger a campaign to send asynchronously<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:**...

**campaign-values-reports** — Manage campaign values reports

- `klaviyo-pp-cli campaign-values-reports` — Returns the requested campaign analytics values data<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady:...

**campaigns** — campaigns

- `klaviyo-pp-cli campaigns create` — Creates a campaign given a set of parameters, then returns it.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady:...
- `klaviyo-pp-cli campaigns delete` — Delete a campaign with the given campaign ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:**...
- `klaviyo-pp-cli campaigns get` — Returns some or all campaigns based on filters. A channel filter is required to list campaigns. Please provide...
- `klaviyo-pp-cli campaigns get-id` — Returns a specific campaign based on a required id.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`...
- `klaviyo-pp-cli campaigns update` — Update a campaign with the given campaign ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:**...

**catalog-categories** — Manage catalog categories

- `klaviyo-pp-cli catalog-categories create-catalog-category` — Create a new catalog category.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m` **Scopes:** `catalogs:write`
- `klaviyo-pp-cli catalog-categories delete-catalog-category` — Delete a catalog category using the given category ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`...
- `klaviyo-pp-cli catalog-categories get` — Get all catalog categories in an account. Catalog categories can be sorted by the following fields, in ascending and...
- `klaviyo-pp-cli catalog-categories get-catalog-category` — Get a catalog category with the given category ID.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`...
- `klaviyo-pp-cli catalog-categories update-catalog-category` — Update a catalog category with the given category ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`...

**catalog-category-bulk-create-jobs** — Manage catalog category bulk create jobs

- `klaviyo-pp-cli catalog-category-bulk-create-jobs bulk-create-catalog-categories` — Create a catalog category bulk create job to create a batch of catalog categories. Accepts up to 100 catalog...
- `klaviyo-pp-cli catalog-category-bulk-create-jobs get-bulk-create-categories-job` — Get a catalog category bulk create job with the given job ID. An `include` parameter can be provided to get the...
- `klaviyo-pp-cli catalog-category-bulk-create-jobs get-bulk-create-categories-jobs` — Get all catalog category bulk create jobs. Returns a maximum of 100 jobs per request.<br><br>*Rate...

**catalog-category-bulk-delete-jobs** — Manage catalog category bulk delete jobs

- `klaviyo-pp-cli catalog-category-bulk-delete-jobs bulk-delete-catalog-categories` — Create a catalog category bulk delete job to delete a batch of catalog categories. Accepts up to 100 catalog...
- `klaviyo-pp-cli catalog-category-bulk-delete-jobs get-bulk-delete-categories-job` — Get a catalog category bulk delete job with the given job ID.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady:...
- `klaviyo-pp-cli catalog-category-bulk-delete-jobs get-bulk-delete-categories-jobs` — Get all catalog category bulk delete jobs. Returns a maximum of 100 jobs per request.<br><br>*Rate...

**catalog-category-bulk-update-jobs** — Manage catalog category bulk update jobs

- `klaviyo-pp-cli catalog-category-bulk-update-jobs bulk-update-catalog-categories` — Create a catalog category bulk update job to update a batch of catalog categories. Accepts up to 100 catalog...
- `klaviyo-pp-cli catalog-category-bulk-update-jobs get-bulk-update-categories-job` — Get a catalog category bulk update job with the given job ID. An `include` parameter can be provided to get the...
- `klaviyo-pp-cli catalog-category-bulk-update-jobs get-bulk-update-categories-jobs` — Get all catalog category bulk update jobs. Returns a maximum of 100 jobs per request.<br><br>*Rate...

**catalog-item-bulk-create-jobs** — Manage catalog item bulk create jobs

- `klaviyo-pp-cli catalog-item-bulk-create-jobs bulk-create-catalog-items` — Create a catalog item bulk create job to create a batch of catalog items. Accepts up to 100 catalog items per...
- `klaviyo-pp-cli catalog-item-bulk-create-jobs get-bulk-create-catalog-items-job` — Get a catalog item bulk create job with the given job ID. An `include` parameter can be provided to get the...
- `klaviyo-pp-cli catalog-item-bulk-create-jobs get-bulk-create-catalog-items-jobs` — Get all catalog item bulk create jobs. Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst:...

**catalog-item-bulk-delete-jobs** — Manage catalog item bulk delete jobs

- `klaviyo-pp-cli catalog-item-bulk-delete-jobs bulk-delete-catalog-items` — Create a catalog item bulk delete job to delete a batch of catalog items. Accepts up to 100 catalog items per...
- `klaviyo-pp-cli catalog-item-bulk-delete-jobs get-bulk-delete-catalog-items-job` — Get a catalog item bulk delete job with the given job ID.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady:...
- `klaviyo-pp-cli catalog-item-bulk-delete-jobs get-bulk-delete-catalog-items-jobs` — Get all catalog item bulk delete jobs. Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst:...

**catalog-item-bulk-update-jobs** — Manage catalog item bulk update jobs

- `klaviyo-pp-cli catalog-item-bulk-update-jobs bulk-update-catalog-items` — Create a catalog item bulk update job to update a batch of catalog items. Accepts up to 100 catalog items per...
- `klaviyo-pp-cli catalog-item-bulk-update-jobs get-bulk-update-catalog-items-job` — Get a catalog item bulk update job with the given job ID. An `include` parameter can be provided to get the...
- `klaviyo-pp-cli catalog-item-bulk-update-jobs get-bulk-update-catalog-items-jobs` — Get all catalog item bulk update jobs. Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst:...

**catalog-items** — Manage catalog items

- `klaviyo-pp-cli catalog-items create` — Create a new catalog item.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m` **Scopes:** `catalogs:write`
- `klaviyo-pp-cli catalog-items delete` — Delete a catalog item with the given item ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m` **Scopes:**...
- `klaviyo-pp-cli catalog-items get` — Get all catalog items in an account. Catalog items can be sorted by the following fields, in ascending and...
- `klaviyo-pp-cli catalog-items get-catalogitems` — Get a specific catalog item with the given item ID.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`...
- `klaviyo-pp-cli catalog-items update` — Update a catalog item with the given item ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m` **Scopes:**...

**catalog-variant-bulk-create-jobs** — Manage catalog variant bulk create jobs

- `klaviyo-pp-cli catalog-variant-bulk-create-jobs bulk-create-catalog-variants` — Create a catalog variant bulk create job to create a batch of catalog variants. Accepts up to 100 catalog variants...
- `klaviyo-pp-cli catalog-variant-bulk-create-jobs get-bulk-create-variants-job` — Get a catalog variant bulk create job with the given job ID. An `include` parameter can be provided to get the...
- `klaviyo-pp-cli catalog-variant-bulk-create-jobs get-bulk-create-variants-jobs` — Get all catalog variant bulk create jobs. Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst:...

**catalog-variant-bulk-delete-jobs** — Manage catalog variant bulk delete jobs

- `klaviyo-pp-cli catalog-variant-bulk-delete-jobs bulk-delete-catalog-variants` — Create a catalog variant bulk delete job to delete a batch of catalog variants. Accepts up to 100 catalog variants...
- `klaviyo-pp-cli catalog-variant-bulk-delete-jobs get-bulk-delete-variants-job` — Get a catalog variant bulk delete job with the given job ID.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady:...
- `klaviyo-pp-cli catalog-variant-bulk-delete-jobs get-bulk-delete-variants-jobs` — Get all catalog variant bulk delete jobs. Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst:...

**catalog-variant-bulk-update-jobs** — Manage catalog variant bulk update jobs

- `klaviyo-pp-cli catalog-variant-bulk-update-jobs bulk-update-catalog-variants` — Create a catalog variant bulk update job to update a batch of catalog variants. Accepts up to 100 catalog variants...
- `klaviyo-pp-cli catalog-variant-bulk-update-jobs get-bulk-update-variants-job` — Get a catalog variate bulk update job with the given job ID. An `include` parameter can be provided to get the...
- `klaviyo-pp-cli catalog-variant-bulk-update-jobs get-bulk-update-variants-jobs` — Get all catalog variant bulk update jobs. Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst:...

**catalog-variants** — Manage catalog variants

- `klaviyo-pp-cli catalog-variants create` — Create a new variant for a related catalog item.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`...
- `klaviyo-pp-cli catalog-variants delete` — Delete a catalog item variant with the given variant ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`...
- `klaviyo-pp-cli catalog-variants get` — Get all variants in an account. Variants can be sorted by the following fields, in ascending and descending order:...
- `klaviyo-pp-cli catalog-variants get-catalogvariants` — Get a catalog item variant with the given variant ID.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`...
- `klaviyo-pp-cli catalog-variants update` — Update a catalog item variant with the given variant ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`...

**client** — client

- `klaviyo-pp-cli client bulk-create-events` — Create new events to track a profile's activity. This endpoint is specifically designed to be called from...
- `klaviyo-pp-cli client create-back-in-stock-subscription` — Subscribe a profile to receive back in stock notifications. Check out [our Back in Stock API...
- `klaviyo-pp-cli client create-event` — Create a new event to track a profile's activity. This endpoint is specifically designed to be called from...
- `klaviyo-pp-cli client create-profile` — Create or update properties about a profile without tracking an associated event. This endpoint is specifically...
- `klaviyo-pp-cli client create-push-token` — Create or update a push token. This endpoint is specifically designed to be called from our mobile SDKs...
- `klaviyo-pp-cli client create-review` — Create a review with the given ID. This endpoint is for client-side environments only.<br><br>*Rate...
- `klaviyo-pp-cli client create-subscription` — Creates a subscription and consent record for email and/or SMS channels based on the provided `email` and...
- `klaviyo-pp-cli client get-geofences` — Get all geofences in an account. Returns a paginated list of all geofences for the specified company. This is the GA...
- `klaviyo-pp-cli client get-review-values-reports` — Get all reviews values reports in an account.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`
- `klaviyo-pp-cli client get-reviews` — Get all reviews. This endpoint is for client-side environments only, for server-side use, refer to...
- `klaviyo-pp-cli client unregister-push-token` — Unregister a push token. This endpoint is specifically designed to be called from our mobile SDKs...

**conversation-messages** — Manage conversation messages

- `klaviyo-pp-cli conversation-messages` — Send an outbound message to a conversation. Requires OAuth authentication and account-level enablement. To request...

**coupon-code-bulk-create-jobs** — Manage coupon code bulk create jobs

- `klaviyo-pp-cli coupon-code-bulk-create-jobs bulk-create-coupon-codes` — Create a coupon-code-bulk-create-job to bulk create a list of coupon codes. Max number of coupon codes per job we...
- `klaviyo-pp-cli coupon-code-bulk-create-jobs get-bulk-create-coupon-code-jobs` — Get all coupon code bulk create jobs. Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst:...
- `klaviyo-pp-cli coupon-code-bulk-create-jobs get-bulk-create-coupon-codes-job` — Get a coupon code bulk create job with the given job ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`...

**coupon-codes** — Manage coupon codes

- `klaviyo-pp-cli coupon-codes create` — Synchronously creates a coupon code for the given coupon.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady:...
- `klaviyo-pp-cli coupon-codes delete` — Deletes a coupon code specified by the given identifier synchronously. If a profile has been assigned to the coupon...
- `klaviyo-pp-cli coupon-codes get` — Gets a list of coupon codes associated with a coupon/coupons or a profile/profiles. A coupon/coupons or a...
- `klaviyo-pp-cli coupon-codes get-couponcodes` — Returns a Coupon Code specified by the given identifier.<br><br>*Rate limits*:<br>Burst: `350/s`<br>Steady: `3500/m`...
- `klaviyo-pp-cli coupon-codes update` — Updates a coupon code specified by the given identifier synchronously. We allow updating the 'status' and...

**coupons** — coupons

- `klaviyo-pp-cli coupons create` — Creates a new coupon.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:** `coupons:write`
- `klaviyo-pp-cli coupons delete` — Delete the coupon with the given coupon ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:**...
- `klaviyo-pp-cli coupons get` — Get all coupons in an account. To learn more, see our [Coupons API...
- `klaviyo-pp-cli coupons get-id` — Get a specific coupon with the given coupon ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`...
- `klaviyo-pp-cli coupons update` — *Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:** `coupons:write`

**custom-metrics** — Manage custom metrics

- `klaviyo-pp-cli custom-metrics create` — Create a new custom metric. Custom metric objects must include a `name` and `definition`.<br><br>*Rate...
- `klaviyo-pp-cli custom-metrics delete` — Delete a custom metric with the given custom metric ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`...
- `klaviyo-pp-cli custom-metrics get` — Get all custom metrics in an account.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:** `metrics:read`
- `klaviyo-pp-cli custom-metrics get-custommetrics` — Get a custom metric with the given custom metric ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`...
- `klaviyo-pp-cli custom-metrics update` — Update a custom metric with the given custom metric ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady:...

**data-privacy-deletion-jobs** — Manage data privacy deletion jobs

- `klaviyo-pp-cli data-privacy-deletion-jobs` — Request a deletion for the profiles corresponding to one of the following identifiers: `email`, `phone_number`, or...

**data-source-record-bulk-create-jobs** — Manage data source record bulk create jobs

- `klaviyo-pp-cli data-source-record-bulk-create-jobs` — Create a bulk data source record import job to create a batch of records. Accepts up to 500 records per request. The...

**data-source-record-create-jobs** — Manage data source record create jobs

- `klaviyo-pp-cli data-source-record-create-jobs` — Create a data source record import job to create a single record. The maximum allowed payload size per-record is...

**data-sources** — Manage data sources

- `klaviyo-pp-cli data-sources create` — Create a new data source in an account<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:**...
- `klaviyo-pp-cli data-sources delete` — Delete a data source in an account.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:**...
- `klaviyo-pp-cli data-sources get` — Get all data sources in an account.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:**...
- `klaviyo-pp-cli data-sources get-datasources` — Retrieve a data source in an account.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:**...

**event-bulk-create-jobs** — Manage event bulk create jobs

- `klaviyo-pp-cli event-bulk-create-jobs` — Create a batch of events for one or more profiles. Note that this endpoint allows you to create new profiles or...

**events** — events

- `klaviyo-pp-cli events create` — Create a new event to track a profile's activity. Note that this endpoint allows you to create a new profile or...
- `klaviyo-pp-cli events get` — Get all events in an account Requests can be sorted by the following fields: `datetime`, `timestamp` [Custom...
- `klaviyo-pp-cli events get-id` — Get an event with the given event ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:**...

**flow-actions** — Manage flow actions

- `klaviyo-pp-cli flow-actions get` — Get a flow action from a flow with the given flow action ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`...
- `klaviyo-pp-cli flow-actions update` — Update a flow action.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:** `flows:write`

**flow-messages** — Manage flow messages

- `klaviyo-pp-cli flow-messages <id>` — Get a flow message from a flow with the given flow message ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady:...

**flow-series-reports** — Manage flow series reports

- `klaviyo-pp-cli flow-series-reports` — Returns the requested flow analytics series data<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `2/m`<br>Daily:...

**flow-values-reports** — Manage flow values reports

- `klaviyo-pp-cli flow-values-reports` — Returns the requested flow analytics values data<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `2/m`<br>Daily:...

**flows** — flows

- `klaviyo-pp-cli flows create` — Create a new flow using an encoded flow definition. New objects within the flow definition, such as actions, will...
- `klaviyo-pp-cli flows delete` — Delete a flow with the given flow ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:** `flows:write`
- `klaviyo-pp-cli flows get` — Get all flows in an account. Returns a maximum of 50 flows per request, which can be paginated with cursor-based...
- `klaviyo-pp-cli flows get-id` — Get a flow with the given flow ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:** `flows:read`
- `klaviyo-pp-cli flows update` — Update the status of a flow with the given flow ID, and all actions in that flow.<br><br>*Rate limits*:<br>Burst:...

**form-series-reports** — Manage form series reports

- `klaviyo-pp-cli form-series-reports` — Returns the requested form analytics series data.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `2/m`<br>Daily:...

**form-values-reports** — Manage form values reports

- `klaviyo-pp-cli form-values-reports` — Returns the requested form analytics values data.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `2/m`<br>Daily:...

**form-versions** — Manage form versions

- `klaviyo-pp-cli form-versions <id>` — Get the form version with the given ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:** `forms:read`

**forms** — forms

- `klaviyo-pp-cli forms create` — Create a new form.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:** `forms:write`
- `klaviyo-pp-cli forms delete` — Delete a given form.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:** `forms:write`
- `klaviyo-pp-cli forms get` — Get all forms in an account.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:** `forms:read`
- `klaviyo-pp-cli forms get-id` — Get the form with the given ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:** `forms:read`

**image-upload** — Manage image upload

- `klaviyo-pp-cli image-upload` — Upload an image from a file. If you want to import an image from an existing url or a data uri, use the Upload Image...

**images** — images

- `klaviyo-pp-cli images get` — Get all images in an account.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:** `images:read`
- `klaviyo-pp-cli images get-id` — Get the image with the given image ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:**...
- `klaviyo-pp-cli images update` — Update the image with the given image ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:**...
- `klaviyo-pp-cli images upload-from-url` — Import an image from a url or data uri. If you want to upload an image from a file, use the Upload Image From File...

**lists** — lists

- `klaviyo-pp-cli lists create` — Create a new list.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`<br>Daily: `150/d` **Scopes:** `lists:write`
- `klaviyo-pp-cli lists delete` — Delete a list with the given list ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:**...
- `klaviyo-pp-cli lists get` — Get all lists in an account. Filter to request a subset of all lists. Lists can be filtered by `id`, `name`,...
- `klaviyo-pp-cli lists get-id` — Get a list with the given list ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`<br><br>Rate limits when...
- `klaviyo-pp-cli lists update` — Update the name of a list with the given list ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`...

**mapped-metrics** — Manage mapped metrics

- `klaviyo-pp-cli mapped-metrics get` — Get all mapped metrics in an account.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:** `metrics:read`
- `klaviyo-pp-cli mapped-metrics get-mappedmetrics` — Get the mapped metric with the given ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:**...
- `klaviyo-pp-cli mapped-metrics update` — Update the mapped metric with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`<br>Daily: `30/d`...

**metric-aggregates** — Manage metric aggregates

- `klaviyo-pp-cli metric-aggregates` — Query and aggregate event data associated with a metric, including native Klaviyo metrics, integration-specific...

**metric-properties** — Manage metric properties

- `klaviyo-pp-cli metric-properties <id>` — Get a metric property with the given metric property ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`...

**metrics** — metrics

- `klaviyo-pp-cli metrics get` — Get all metrics in an account. Requests can be filtered by the following fields: integration `name`, integration...
- `klaviyo-pp-cli metrics get-id` — Get a metric with the given metric ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:**...

**profile-bulk-import-jobs** — Manage profile bulk import jobs

- `klaviyo-pp-cli profile-bulk-import-jobs bulk-import-profiles` — Create a bulk profile import job to create or update a batch of profiles. Accepts up to 10,000 profiles per request....
- `klaviyo-pp-cli profile-bulk-import-jobs get-bulk-import-profiles-job` — Get a bulk profile import job with the given job ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m`...
- `klaviyo-pp-cli profile-bulk-import-jobs get-bulk-import-profiles-jobs` — Get all bulk profile import jobs. Returns a maximum of 100 jobs per request.<br><br>*Rate limits*:<br>Burst:...

**profile-import** — Manage profile import

- `klaviyo-pp-cli profile-import` — Given a set of profile attributes and optionally an ID, create or update a profile. Returns 201 if a new profile was...

**profile-merge** — Manage profile merge

- `klaviyo-pp-cli profile-merge` — Merge a given related profile into a profile with the given profile ID. The profile provided under `relationships`...

**profile-subscription-bulk-create-jobs** — Manage profile subscription bulk create jobs

- `klaviyo-pp-cli profile-subscription-bulk-create-jobs` — Subscribe one or more profiles to email marketing, SMS marketing, WhatsApp, or push. If the provided list has double...

**profile-subscription-bulk-delete-jobs** — Manage profile subscription bulk delete jobs

- `klaviyo-pp-cli profile-subscription-bulk-delete-jobs` — > 🚧 > > Profiles not in the specified list will be globally unsubscribed. Always verify profile list membership...

**profile-suppression-bulk-create-jobs** — Manage profile suppression bulk create jobs

- `klaviyo-pp-cli profile-suppression-bulk-create-jobs bulk-suppress-profiles` — Manually suppress profiles by email address or specify a segment/list ID to suppress all current members of a...
- `klaviyo-pp-cli profile-suppression-bulk-create-jobs get-bulk-suppress-profiles-job` — Get the bulk suppress profiles job with the given job ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`...
- `klaviyo-pp-cli profile-suppression-bulk-create-jobs get-bulk-suppress-profiles-jobs` — Get the status of all bulk profile suppression jobs.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`...

**profile-suppression-bulk-delete-jobs** — Manage profile suppression bulk delete jobs

- `klaviyo-pp-cli profile-suppression-bulk-delete-jobs bulk-unsuppress-profiles` — Manually unsuppress profiles by email address or specify a segment/list ID to unsuppress all current members of a...
- `klaviyo-pp-cli profile-suppression-bulk-delete-jobs get-bulk-unsuppress-profiles-job` — Get the bulk unsuppress profiles job with the given job ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady:...
- `klaviyo-pp-cli profile-suppression-bulk-delete-jobs get-bulk-unsuppress-profiles-jobs` — Get all bulk unsuppress profiles jobs.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m` **Scopes:**...

**profiles** — profiles

- `klaviyo-pp-cli profiles create` — Create a new profile. Use the `additional-fields` parameter to include subscriptions and predictive analytics data...
- `klaviyo-pp-cli profiles get` — Get all profiles in an account. Profiles can be sorted by the following fields in ascending and descending order:...
- `klaviyo-pp-cli profiles get-id` — Get the profile with the given profile ID. Use the `additional-fields` parameter to include subscriptions and...
- `klaviyo-pp-cli profiles update` — Update the profile with the given profile ID. Use the `additional-fields` parameter to include subscriptions and...

**push-tokens** — Manage push tokens

- `klaviyo-pp-cli push-tokens create` — Create or update a push token. This endpoint can be used to migrate push tokens from another platform to Klaviyo....
- `klaviyo-pp-cli push-tokens delete` — Delete a specific push token based on its ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:**...
- `klaviyo-pp-cli push-tokens get` — Return push tokens associated with company.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:**...
- `klaviyo-pp-cli push-tokens get-pushtokens` — Return a specific push token based on its ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:**...

**reviews** — reviews

- `klaviyo-pp-cli reviews get` — Get all reviews.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:** `reviews:read`
- `klaviyo-pp-cli reviews get-id` — Get the review with the given ID.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:** `reviews:read`
- `klaviyo-pp-cli reviews update` — Update a review.<br><br>*Rate limits*:<br>Burst: `10/s`<br>Steady: `150/m` **Scopes:** `reviews:write`

**segment-series-reports** — Manage segment series reports

- `klaviyo-pp-cli segment-series-reports` — Returns the requested segment analytics series data.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady:...

**segment-values-reports** — Manage segment values reports

- `klaviyo-pp-cli segment-values-reports` — Returns the requested segment analytics values data.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady:...

**segments** — segments

- `klaviyo-pp-cli segments create` — Create a segment.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`<br>Daily: `100/d` **Scopes:** `segments:write`
- `klaviyo-pp-cli segments delete` — Delete a segment with the given segment ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:**...
- `klaviyo-pp-cli segments get` — Get all segments in an account. Filter to request a subset of all segments. Segments can be filtered by `name`,...
- `klaviyo-pp-cli segments get-id` — Get a segment with the given segment ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`<br><br>Rate...
- `klaviyo-pp-cli segments update` — Update a segment with the given segment ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m`<br>Daily:...

**tag-groups** — Manage tag groups

- `klaviyo-pp-cli tag-groups create` — Create a tag group. An account cannot have more than **50** unique tag groups. If `exclusive` is not specified...
- `klaviyo-pp-cli tag-groups delete` — Delete the tag group with the given tag group ID. Any tags inside that tag group, and any associations between those...
- `klaviyo-pp-cli tag-groups get` — List all tag groups in an account. Every account has one default tag group. Tag groups can be filtered by `name`,...
- `klaviyo-pp-cli tag-groups get-taggroups` — Retrieve the tag group with the given tag group ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m`...
- `klaviyo-pp-cli tag-groups update` — Update the tag group with the given tag group ID. Only a tag group's `name` can be changed. A tag group's...

**tags** — tags

- `klaviyo-pp-cli tags create` — Create a tag. An account cannot have more than **500** unique tags. A tag belongs to a single tag group. If...
- `klaviyo-pp-cli tags delete` — Delete the tag with the given tag ID. Any associations between the tag and other resources will also be...
- `klaviyo-pp-cli tags get` — List all tags in an account. Tags can be filtered by `name`, and sorted by `name` or `id` in ascending or descending...
- `klaviyo-pp-cli tags get-id` — Retrieve the tag with the given tag ID.<br><br>*Rate limits*:<br>Burst: `3/s`<br>Steady: `60/m` **Scopes:** `tags:read`
- `klaviyo-pp-cli tags update` — Update the tag with the given tag ID. Only a tag's `name` can be changed. A tag cannot be moved from one tag group...

**template-clone** — Manage template clone

- `klaviyo-pp-cli template-clone` — Create a clone of a template with the given template ID. If there are 1,000 or more templates in an account, cloning...

**template-render** — Manage template render

- `klaviyo-pp-cli template-render` — Render a template with the given template ID and context attribute. Returns the AMP, HTML, and plain text versions...

**template-universal-content** — Manage template universal content

- `klaviyo-pp-cli template-universal-content create-universal-content` — Create universal content. Currently supported block types are: `button`, `drop_shadow`, `horizontal_rule`, `html`,...
- `klaviyo-pp-cli template-universal-content delete-universal-content` — Delete the universal content with the given ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m`...
- `klaviyo-pp-cli template-universal-content get-all-universal-content` — Get all universal content in an account.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m` **Scopes:**...
- `klaviyo-pp-cli template-universal-content get-universal-content` — Get the universal content with the given ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m` **Scopes:**...
- `klaviyo-pp-cli template-universal-content update-universal-content` — Update universal content. The `definition` field can only be updated on the following block types at this time:...

**templates** — templates

- `klaviyo-pp-cli templates create` — Create a new HTML or drag-and-drop template. If there are 1,000 or more templates in an account, creation will fail...
- `klaviyo-pp-cli templates delete` — Delete a template with the given template ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m` **Scopes:**...
- `klaviyo-pp-cli templates get` — Get all templates in an account. Use `additional-fields[template]=definition` to include the full template...
- `klaviyo-pp-cli templates get-id` — Get a template with the given template ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m` **Scopes:**...
- `klaviyo-pp-cli templates update` — Update a template with the given template ID.<br><br>*Rate limits*:<br>Burst: `75/s`<br>Steady: `750/m` **Scopes:**...

**tracking-settings** — tracking settings

- `klaviyo-pp-cli tracking-settings get` — Get all UTM tracking settings in an account. Returns an array with a single tracking setting. More information about...
- `klaviyo-pp-cli tracking-settings get-trackingsettings` — Get the UTM tracking setting with the given account ID. More information about UTM tracking settings can be found...
- `klaviyo-pp-cli tracking-settings update` — Update the UTM tracking setting with the given account ID. More information about UTM tracking settings can be found...

**web-feeds** — web feeds

- `klaviyo-pp-cli web-feeds create` — Create a web feed.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m` **Scopes:** `web-feeds:write`
- `klaviyo-pp-cli web-feeds delete` — Delete the web feed with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m` **Scopes:**...
- `klaviyo-pp-cli web-feeds get` — Get all web feeds for an account.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m` **Scopes:** `web-feeds:read`
- `klaviyo-pp-cli web-feeds get-webfeeds` — Get the web feed with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m` **Scopes:** `web-feeds:read`
- `klaviyo-pp-cli web-feeds update` — Update the web feed with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m` **Scopes:**...

**webhook-topics** — Manage webhook topics

- `klaviyo-pp-cli webhook-topics get` — Get all webhook topics in a Klaviyo account.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m` **Scopes:**...
- `klaviyo-pp-cli webhook-topics get-webhooktopics` — Get the webhook topic with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m` **Scopes:**...

**webhooks** — webhooks

- `klaviyo-pp-cli webhooks create` — Create a new Webhook<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m` **Scopes:** `webhooks:write`
- `klaviyo-pp-cli webhooks delete` — Delete a webhook with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m` **Scopes:** `webhooks:write`
- `klaviyo-pp-cli webhooks get` — Get all webhooks in an account.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m` **Scopes:** `webhooks:read`
- `klaviyo-pp-cli webhooks get-id` — Get the webhook with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m` **Scopes:** `webhooks:read`
- `klaviyo-pp-cli webhooks update` — Update the webhook with the given ID.<br><br>*Rate limits*:<br>Burst: `1/s`<br>Steady: `15/m` **Scopes:**...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
klaviyo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Inspect a profile compactly

```bash
klaviyo-pp-cli profiles list --limit 1 --json --select data.id,data.email,data.first_name
```

Fetch one profile and select only the fields an agent needs.

### Find a revenue metric

```bash
klaviyo-pp-cli metrics list --json --select data.id,data.name | jq '.data[] | select(.name=="Placed Order")'
```

Locate the Placed Order metric before attribution or cohort work.

### Run a launch QA gate

```bash
klaviyo-pp-cli plan qa-gate --campaign-id CAMPAIGN_ID --json
```

Review offer, link, compliance, date, and deliverability findings before launch.

## Auth Setup

Set KLAVIYO_API_KEY to a private Klaviyo API key. Requests send Authorization: Klaviyo-API-Key <token> and use the revision pinned by the generated OpenAPI spec.

Run `klaviyo-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  klaviyo-pp-cli accounts get --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag

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
klaviyo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
klaviyo-pp-cli feedback --stdin < notes.txt
klaviyo-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.klaviyo-pp-cli/feedback.jsonl`. They are never POSTed unless `KLAVIYO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `KLAVIYO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
klaviyo-pp-cli profile save briefing --json
klaviyo-pp-cli --profile briefing accounts get
klaviyo-pp-cli profile list --json
klaviyo-pp-cli profile show briefing
klaviyo-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `klaviyo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/cmd/klaviyo-pp-cli@latest
   ```
3. Verify: `klaviyo-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/cmd/klaviyo-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add klaviyo-pp-mcp -- klaviyo-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which klaviyo-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   klaviyo-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `klaviyo-pp-cli <command> --help`.
