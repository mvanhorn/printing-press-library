---
name: pp-square
description: "The current Square v2 API surface, plus local reconciliation and retained history. Trigger phrases: `reconcile my Square close`, `find Square inventory drift`, `trace this Square customer`, `check Square webhook health`, `review Square service operations`, `use the Square API`, `run the Square CLI`."
author: "matthew.martin"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - square-pp-cli
    install:
      - kind: go
        bins: [square-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/square/cmd/square-pp-cli
---

# Square — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `square-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install square --cli-only
   ```
2. Verify: `square-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/square/cmd/square-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Use one scriptable CLI for payments, orders, catalog, inventory, customers, bookings, staff, and developer operations. Sync selected resources locally, then run close reconciliation, inventory drift, customer timelines, webhook health checks, and service reviews with compact agent-ready output.

## When to Use This CLI

Use this CLI for Square API automation, operator investigations, local history, and cross-resource analysis involving payments, orders, inventory, customers, bookings, or integrations. Prefer it when a repeatable command, offline search, or compact agent result is more useful than clicking through the Square Dashboard or writing a one-off SDK script.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to collect or handle raw card numbers; use Square's approved Web Payments or Mobile Payments SDKs.
- Do not run production payment, refund, payout, or destructive mutations without explicit human approval and a reviewed request.
- Do not use this CLI for Square Dashboard-only features that are absent from the public API.
- Do not treat mock verification as proof that a production Square account or location is configured correctly.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Operational reconciliation
- **`reconcile close`** — Explain the difference between sales, collected payments, refunds, disputes, and payouts for each location.

  _Use this when an operator needs an explainable close instead of several unrelated API responses._

  ```bash
  square-pp-cli reconcile close --since 24h --agent
  ```
- **`inventory drift`** — See which prices, variations, availability settings, and location counts changed since an earlier snapshot.

  _Use this before opening or publishing a menu when current-state API calls cannot explain what changed._

  ```bash
  square-pp-cli inventory drift --since 7d --agent
  ```

### Connected customer operations
- **`customer timeline`** — Follow one customer's orders, payments, refunds, loyalty activity, invoices, and bookings in chronological order.

  _Use this for customer-service investigations that otherwise require carrying IDs between services._

  ```bash
  square-pp-cli customer timeline CUSTOMER_ABC123 --agent
  ```
- **`service review`** — Review completed, unpaid, and missed appointments alongside staff and location utilization.

  _Use this for a service business's weekly operating review instead of assembling separate reports._

  ```bash
  square-pp-cli service review --since 7d --agent
  ```

### Integration confidence
- **`webhook health`** — Measure duplicate locally ingested webhook deliveries, ordering problems, delivery lag, gaps, and subscription changes.

  _Use this before releases or during incidents when a subscription listing cannot reveal delivery behavior._

  ```bash
  square-pp-cli webhook health --since 24h --agent
  ```
- **`request check`** — Validate a planned Square request, environment, API version, mutation policy, and idempotency before anything is sent.

  _Use this before a risky sandbox or production call when schema validity alone is not enough._

  ```bash
  square-pp-cli request check --method POST --path /v2/payments --body payment-request.json --agent
  ```

## Command Reference

**apple-pay** — Manage apple pay

- `square-pp-cli apple-pay` — Activates a domain for use with Apple Pay on the Web and Square.

**bank-accounts** — Manage bank accounts

- `square-pp-cli bank-accounts create` — Store a bank account on file for a square account
- `square-pp-cli bank-accounts get` — Retrieve details of a [BankAccount](entity:BankAccount) bank account linked to a Square account.
- `square-pp-cli bank-accounts get-by-v1-id` — Returns details of a [BankAccount](entity:BankAccount) identified by V1 bank account ID.
- `square-pp-cli bank-accounts list` — Returns a list of [BankAccount](entity:BankAccount) objects linked to a Square account.

**bookings** — Manage bookings

- `square-pp-cli bookings bulk-delete-custom-attributes` — Bulk deletes bookings custom attributes.
- `square-pp-cli bookings bulk-retrieve` — Bulk-Retrieves a list of bookings by booking IDs.
- `square-pp-cli bookings bulk-retrieve-team-member-profiles` — Retrieves one or more team members' booking profiles.
- `square-pp-cli bookings bulk-upsert-custom-attributes` — Bulk upserts bookings custom attributes.
- `square-pp-cli bookings create` — Creates a booking. The required input must include the following: - `Booking.location_id` - `Booking.
- `square-pp-cli bookings create-custom-attribute-definition` — Creates a bookings custom attribute definition.
- `square-pp-cli bookings delete-custom-attribute-definition` — Deletes a bookings custom attribute definition.
- `square-pp-cli bookings list` — Retrieve a collection of bookings.
- `square-pp-cli bookings list-custom-attribute-definitions` — Get all bookings custom attribute definitions.
- `square-pp-cli bookings list-location-profiles` — Lists location booking profiles of a seller.
- `square-pp-cli bookings list-team-member-profiles` — Lists booking profiles for team members.
- `square-pp-cli bookings retrieve` — Retrieves a booking. To call this endpoint with buyer-level permissions, set `APPOINTMENTS_READ` for the OAuth scope.
- `square-pp-cli bookings retrieve-business-profile` — Retrieves a seller's booking profile.
- `square-pp-cli bookings retrieve-custom-attribute-definition` — Retrieves a bookings custom attribute definition.
- `square-pp-cli bookings retrieve-location-profile` — Retrieves a seller's location booking profile.
- `square-pp-cli bookings retrieve-team-member-profile` — Retrieves a team member's booking profile.
- `square-pp-cli bookings search-availability` — Searches for availabilities for booking.
- `square-pp-cli bookings update` — Updates a booking. To call this endpoint with buyer-level permissions, set `APPOINTMENTS_WRITE` for the OAuth scope.
- `square-pp-cli bookings update-custom-attribute-definition` — Updates a bookings custom attribute definition.

**cards** — Manage cards

- `square-pp-cli cards create` — Adds a card on file to an existing merchant.
- `square-pp-cli cards list` — Retrieves a list of cards owned by the account making the request. A max of 25 cards will be returned.
- `square-pp-cli cards retrieve` — Retrieves details for a specific Card.

**cash-drawers** — Manage cash drawers

- `square-pp-cli cash-drawers list-shift-events` — Provides a paginated list of events for a single cash drawer shift.
- `square-pp-cli cash-drawers list-shifts` — Provides the details for all of the cash drawer shifts for a location in a date range.
- `square-pp-cli cash-drawers retrieve-shift` — Provides the summary details for a single cash drawer shift.

**catalog** — Manage catalog

- `square-pp-cli catalog batch-delete-objects` — Deletes a set of [CatalogItem](entity:CatalogItem)
- `square-pp-cli catalog batch-retrieve-objects` — Returns a set of objects based on the provided ID.
- `square-pp-cli catalog batch-upsert-objects` — Creates or updates up to 10,000 target objects based on the provided list of objects.
- `square-pp-cli catalog create-image` — Uploads an image file to be represented by a [CatalogImage](entity:CatalogImage)
- `square-pp-cli catalog delete-object` — Deletes a single [CatalogObject](entity:CatalogObject)
- `square-pp-cli catalog get` — Retrieves information about the Square Catalog API
- `square-pp-cli catalog list` — Returns a list of all [CatalogObject](entity:CatalogObject)s of the specified types in the catalog.
- `square-pp-cli catalog retrieve-object` — Returns a single [CatalogItem](entity:CatalogItem) as a [CatalogObject](entity:CatalogObject) based on the provided ID.
- `square-pp-cli catalog search-items` — Searches for catalog items or item variations by matching supported search attribute values
- `square-pp-cli catalog search-objects` — Searches for [CatalogObject](entity:CatalogObject) of any type by matching supported search attribute values
- `square-pp-cli catalog update-image` — Uploads a new image file to replace the existing one in the specified [CatalogImage](entity:CatalogImage) object.
- `square-pp-cli catalog update-item-modifier-lists` — Updates the [CatalogModifierList](entity:CatalogModifierList) objects that apply to the targeted [CatalogItem](entity
- `square-pp-cli catalog update-item-taxes` — Updates the [CatalogTax](entity:CatalogTax) objects that apply to the targeted [CatalogItem](entity:CatalogItem)
- `square-pp-cli catalog upsert-object` — Creates a new or updates the specified [CatalogObject](entity:CatalogObject).

**channels** — Manage channels

- `square-pp-cli channels bulk-retrieve` — Bulk retrieve channels
- `square-pp-cli channels list` — List channels
- `square-pp-cli channels retrieve` — Retrieve channel

**customers** — Manage customers

- `square-pp-cli customers bulk-create` — Creates multiple [customer profiles](entity:Customer) for a business.
- `square-pp-cli customers bulk-delete` — Deletes multiple customer profiles. The endpoint takes a list of customer IDs and returns a map of responses.
- `square-pp-cli customers bulk-retrieve` — Retrieves multiple customer profiles. This endpoint takes a list of customer IDs and returns a map of responses.
- `square-pp-cli customers bulk-update` — Updates multiple customer profiles.
- `square-pp-cli customers bulk-upsert-custom-attributes` — Creates or updates [custom attributes](entity:CustomAttribute) for customer profiles as a bulk operation.
- `square-pp-cli customers create` — Creates a new customer for a business.
- `square-pp-cli customers create-custom-attribute-definition` — Creates a customer-related [custom attribute definition](entity:CustomAttributeDefinition) for a Square seller account.
- `square-pp-cli customers create-group` — Creates a new customer group for a business. The request must include the `name` value of the group.
- `square-pp-cli customers delete` — Deletes a customer profile from a business.
- `square-pp-cli customers delete-custom-attribute-definition` — Deletes a customer-related [custom attribute definition](entity:CustomAttributeDefinition) from a Square seller account.
- `square-pp-cli customers delete-group` — Deletes a customer group as identified by the `group_id` value.
- `square-pp-cli customers list` — Lists customer profiles associated with a Square account.
- `square-pp-cli customers list-custom-attribute-definitions` — Lists the customer-related [custom attribute definitions](entity:CustomAttributeDefinition)
- `square-pp-cli customers list-groups` — Retrieves the list of customer groups of a business.
- `square-pp-cli customers list-segments` — Retrieves the list of customer segments of a business.
- `square-pp-cli customers retrieve` — Returns details for a single customer.
- `square-pp-cli customers retrieve-custom-attribute-definition` — Retrieves a customer-related [custom attribute definition](entity:CustomAttributeDefinition)
- `square-pp-cli customers retrieve-group` — Retrieves a specific customer group as identified by the `group_id` value.
- `square-pp-cli customers retrieve-segment` — Retrieves a specific customer segment as identified by the `segment_id` value.
- `square-pp-cli customers search` — Searches the customer profiles associated with a Square account using one or more supported query filters.
- `square-pp-cli customers update` — Updates a customer profile.
- `square-pp-cli customers update-custom-attribute-definition` — Updates a customer-related [custom attribute definition](entity:CustomAttributeDefinition) for a Square seller account.
- `square-pp-cli customers update-group` — Updates a customer group as identified by the `group_id` value.

**devices** — Manage devices

- `square-pp-cli devices create-code` — Creates a DeviceCode that can be used to login to a Square Terminal device to enter the connected terminal mode.
- `square-pp-cli devices get` — Retrieves Device with the associated `device_id`.
- `square-pp-cli devices get-code` — Retrieves DeviceCode with the associated ID.
- `square-pp-cli devices list` — List devices associated with the merchant. Currently, only Terminal API devices are supported.
- `square-pp-cli devices list-codes` — Lists all DeviceCodes associated with the merchant.

**disputes** — Manage disputes

- `square-pp-cli disputes list` — Returns a list of disputes associated with a particular account.
- `square-pp-cli disputes retrieve` — Returns details about a specific dispute.

**employees** — Manage employees

- `square-pp-cli employees list` — List employees
- `square-pp-cli employees retrieve` — Retrieve employee

**events** — Manage events

- `square-pp-cli events disable` — Disables events to prevent them from being searchable. All events are disabled by default.
- `square-pp-cli events enable` — Enables events to make them searchable. Only events that occur while in the enabled state are searchable.
- `square-pp-cli events list-types` — Lists all event types that you can subscribe to as webhooks or query using the Events API.
- `square-pp-cli events search` — Search for Square API events that occur within a 28-day timeframe.

**gift-cards** — Manage gift cards

- `square-pp-cli gift-cards create` — Creates a digital gift card or registers a physical (plastic) gift card. The resulting gift card has a `PENDING` state.
- `square-pp-cli gift-cards create-activity` — Creates a gift card activity to manage the balance or state of a [gift card](entity:GiftCard).
- `square-pp-cli gift-cards list` — Lists all gift cards. You can specify optional filters to retrieve a subset of the gift cards.
- `square-pp-cli gift-cards list-activities` — Lists gift card activities. By default, you get gift card activities for all gift cards in the seller's account.
- `square-pp-cli gift-cards retrieve` — Retrieves a gift card using the gift card ID.
- `square-pp-cli gift-cards retrieve-from-gan` — Retrieves a gift card using the gift card account number (GAN).
- `square-pp-cli gift-cards retrieve-from-nonce` — Retrieves a gift card using a secure payment token that represents the gift card.

**inventory** — Manage inventory

- `square-pp-cli inventory batch-change` — Applies adjustments and counts to the provided item quantities.
- `square-pp-cli inventory batch-retrieve-changes` — Returns historical physical counts and adjustments based on the provided filter criteria.
- `square-pp-cli inventory batch-retrieve-counts` — Returns current counts for the provided [CatalogObject](entity:CatalogObject)s at the requested [Location](entity
- `square-pp-cli inventory create-adjustment-reason` — Creates a custom inventory adjustment reason.
- `square-pp-cli inventory delete-adjustment-reason` — Soft deletes a custom inventory adjustment reason.
- `square-pp-cli inventory deprecated-batch-change` — Deprecated version of [BatchChangeInventory](api-endpoint:Inventory-BatchChangeInventory)
- `square-pp-cli inventory deprecated-batch-retrieve-changes` — Deprecated version of [BatchRetrieveInventoryChanges](api-endpoint:Inventory-BatchRetrieveInventoryChanges)
- `square-pp-cli inventory deprecated-batch-retrieve-counts` — Deprecated version of [BatchRetrieveInventoryCounts](api-endpoint:Inventory-BatchRetrieveInventoryCounts)
- `square-pp-cli inventory deprecated-retrieve-adjustment` — Deprecated version of [RetrieveInventoryAdjustment](api-endpoint:Inventory-RetrieveInventoryAdjustment)
- `square-pp-cli inventory deprecated-retrieve-physical-count` — Deprecated version of [RetrieveInventoryPhysicalCount](api-endpoint:Inventory-RetrieveInventoryPhysicalCount)
- `square-pp-cli inventory list-adjustment-reasons` — Returns the standard and custom inventory adjustment reasons available to the seller.
- `square-pp-cli inventory restore-adjustment-reason` — Restores a soft-deleted custom inventory adjustment reason.
- `square-pp-cli inventory retrieve-adjustment` — Returns the [InventoryAdjustment](entity:InventoryAdjustment) object with the provided `adjustment_id`.
- `square-pp-cli inventory retrieve-adjustment-reason` — Returns the inventory adjustment reason identified by the provided `reason_id`.
- `square-pp-cli inventory retrieve-count` — Retrieves the current calculated stock count for a given [CatalogObject](entity:CatalogObject)
- `square-pp-cli inventory retrieve-physical-count` — Returns the [InventoryPhysicalCount](entity:InventoryPhysicalCount) object with the provided `physical_count_id`.
- `square-pp-cli inventory update-adjustment` — Applies an update to the provided adjustment. On success: returns the newly updated adjustment.
- `square-pp-cli inventory update-adjustment-reason` — Updates a custom inventory adjustment reason.

**invoices** — Manage invoices

- `square-pp-cli invoices create` — Creates a draft [invoice](entity:Invoice) for an order created using the Orders API.
- `square-pp-cli invoices delete` — Deletes the specified invoice. When an invoice is deleted, the associated order status changes to CANCELED.
- `square-pp-cli invoices get` — Retrieves an invoice by invoice ID.
- `square-pp-cli invoices list` — Returns a list of invoices for a given location. The response is paginated.
- `square-pp-cli invoices search` — Searches for invoices from a location specified in the filter.
- `square-pp-cli invoices update` — Updates an invoice.

**labor** — Manage labor

- `square-pp-cli labor bulk-publish-scheduled-shifts` — Publishes 1 - 100 scheduled shifts.
- `square-pp-cli labor create-break-type` — Creates a new `BreakType`. A `BreakType` is a template for creating `Break` objects.
- `square-pp-cli labor create-scheduled-shift` — Creates a scheduled shift by providing draft shift details such as job ID, team member assignment
- `square-pp-cli labor create-shift` — Creates a new `Shift`. A `Shift` represents a complete workday for a single team member.
- `square-pp-cli labor create-timecard` — Creates a new `Timecard`. A `Timecard` represents a complete workday for a single team member.
- `square-pp-cli labor delete-break-type` — Deletes an existing `BreakType`. A `BreakType` can be deleted even if it is referenced from a `Shift`.
- `square-pp-cli labor delete-shift` — Deletes a `Shift`.
- `square-pp-cli labor delete-timecard` — Deletes a `Timecard`.
- `square-pp-cli labor get-break-type` — Returns a single `BreakType` specified by `id`.
- `square-pp-cli labor get-employee-wage` — Returns a single `EmployeeWage` specified by `id`.
- `square-pp-cli labor get-shift` — Returns a single `Shift` specified by `id`.
- `square-pp-cli labor get-team-member-wage` — Returns a single `TeamMemberWage` specified by `id`.
- `square-pp-cli labor list-break-types` — Returns a paginated list of `BreakType` instances for a business.
- `square-pp-cli labor list-employee-wages` — Returns a paginated list of `EmployeeWage` instances for a business.
- `square-pp-cli labor list-team-member-wages` — Returns a paginated list of `TeamMemberWage` instances for a business.
- `square-pp-cli labor list-workweek-configs` — Returns a list of `WorkweekConfig` instances for a business.
- `square-pp-cli labor publish-scheduled-shift` — Publishes a scheduled shift.
- `square-pp-cli labor retrieve-scheduled-shift` — Retrieves a scheduled shift by ID.
- `square-pp-cli labor retrieve-timecard` — Returns a single `Timecard` specified by `id`.
- `square-pp-cli labor search-scheduled-shifts` — Returns a paginated list of scheduled shifts, with optional filter and sort settings.
- `square-pp-cli labor search-shifts` — Returns a paginated list of `Shift` records for a business.
- `square-pp-cli labor search-timecards` — Returns a paginated list of `Timecard` records for a business.
- `square-pp-cli labor update-break-type` — Updates an existing `BreakType`.
- `square-pp-cli labor update-scheduled-shift` — Updates the draft shift details for a scheduled shift.
- `square-pp-cli labor update-shift` — Updates an existing `Shift`.
- `square-pp-cli labor update-timecard` — Updates an existing `Timecard`.
- `square-pp-cli labor update-workweek-config` — Updates a `WorkweekConfig`.

**locations** — Manage locations

- `square-pp-cli locations bulk-delete-custom-attributes` — Deletes [custom attributes](entity:CustomAttribute) for locations as a bulk operation.
- `square-pp-cli locations bulk-upsert-custom-attributes` — Creates or updates [custom attributes](entity:CustomAttribute) for locations as a bulk operation.
- `square-pp-cli locations create` — Creates a [location](https://developer.squareup.com/docs/locations-api).
- `square-pp-cli locations create-custom-attribute-definition` — Creates a location-related [custom attribute definition](entity:CustomAttributeDefinition) for a Square seller account.
- `square-pp-cli locations delete-custom-attribute-definition` — Deletes a location-related [custom attribute definition](entity:CustomAttributeDefinition) from a Square seller account.
- `square-pp-cli locations list` — Provides details about all of the seller's [locations](https://developer.squareup.
- `square-pp-cli locations list-custom-attribute-definitions` — Lists the location-related [custom attribute definitions](entity:CustomAttributeDefinition)
- `square-pp-cli locations retrieve` — Retrieves details of a single location.
- `square-pp-cli locations retrieve-custom-attribute-definition` — Retrieves a location-related [custom attribute definition](entity:CustomAttributeDefinition)
- `square-pp-cli locations update` — Updates a [location](https://developer.squareup.com/docs/locations-api).
- `square-pp-cli locations update-custom-attribute-definition` — Updates a location-related [custom attribute definition](entity:CustomAttributeDefinition) for a Square seller account.

**loyalty** — Manage loyalty

- `square-pp-cli loyalty accumulate-points` — Adds points earned from a purchase to a [loyalty account](entity:LoyaltyAccount).
- `square-pp-cli loyalty adjust-points` — Adds points to or subtracts points from a buyer's account.
- `square-pp-cli loyalty calculate-points` — Calculates the number of points a buyer can earn from a purchase.
- `square-pp-cli loyalty cancel-promotion` — Cancels a loyalty promotion.
- `square-pp-cli loyalty create-account` — Creates a loyalty account.
- `square-pp-cli loyalty create-promotion` — Creates a loyalty promotion for a [loyalty program](entity:LoyaltyProgram).
- `square-pp-cli loyalty create-reward` — Creates a loyalty reward.
- `square-pp-cli loyalty delete-reward` — Deletes a loyalty reward by doing the following: - Returns the loyalty points back to the loyalty account.
- `square-pp-cli loyalty list-programs` — Returns a list of loyalty programs in the seller's account.
- `square-pp-cli loyalty list-promotions` — Lists the loyalty promotions associated with a [loyalty program](entity:LoyaltyProgram).
- `square-pp-cli loyalty redeem-reward` — Redeems a loyalty reward. The endpoint sets the reward to the `REDEEMED` terminal state.
- `square-pp-cli loyalty retrieve-account` — Retrieves a loyalty account.
- `square-pp-cli loyalty retrieve-program` — Retrieves the loyalty program in a seller's account, specified by the program ID or the keyword `main`.
- `square-pp-cli loyalty retrieve-promotion` — Retrieves a loyalty promotion.
- `square-pp-cli loyalty retrieve-reward` — Retrieves a loyalty reward.
- `square-pp-cli loyalty search-accounts` — Searches for loyalty accounts in a loyalty program.
- `square-pp-cli loyalty search-events` — Searches for loyalty events.
- `square-pp-cli loyalty search-rewards` — Searches for loyalty rewards.

**merchants** — Manage merchants

- `square-pp-cli merchants bulk-delete-custom-attributes` — Deletes [custom attributes](entity:CustomAttribute) for a merchant as a bulk operation.
- `square-pp-cli merchants bulk-upsert-custom-attributes` — Creates or updates [custom attributes](entity:CustomAttribute) for a merchant as a bulk operation.
- `square-pp-cli merchants create-custom-attribute-definition` — Creates a merchant-related [custom attribute definition](entity:CustomAttributeDefinition) for a Square seller account.
- `square-pp-cli merchants delete-custom-attribute-definition` — Deletes a merchant-related [custom attribute definition](entity:CustomAttributeDefinition) from a Square seller account.
- `square-pp-cli merchants list` — Provides details about the merchant associated with a given access token.
- `square-pp-cli merchants list-custom-attribute-definitions` — Lists the merchant-related [custom attribute definitions](entity:CustomAttributeDefinition)
- `square-pp-cli merchants retrieve` — Retrieves the `Merchant` object for the given `merchant_id`.
- `square-pp-cli merchants retrieve-custom-attribute-definition` — Retrieves a merchant-related [custom attribute definition](entity:CustomAttributeDefinition)
- `square-pp-cli merchants update-custom-attribute-definition` — Updates a merchant-related [custom attribute definition](entity:CustomAttributeDefinition) for a Square seller account.

**oauth2** — Manage oauth2

- `square-pp-cli oauth2 obtain-token` — Returns an OAuth access token and refresh token using the `authorization_code` or `refresh_token` grant type.
- `square-pp-cli oauth2 retrieve-token-status` — Returns information about an [OAuth access token](https://developer.squareup.
- `square-pp-cli oauth2 revoke-token` — Revokes an access token generated with the OAuth flow.

**online-checkout** — Manage online checkout

- `square-pp-cli online-checkout create-payment-link` — Creates a Square-hosted checkout page.
- `square-pp-cli online-checkout delete-payment-link` — Deletes a payment link.
- `square-pp-cli online-checkout list-payment-links` — Lists all payment links.
- `square-pp-cli online-checkout retrieve-location-settings` — Retrieves the location-level settings for a Square-hosted checkout page.
- `square-pp-cli online-checkout retrieve-merchant-settings` — Retrieves the merchant-level settings for a Square-hosted checkout page.
- `square-pp-cli online-checkout retrieve-payment-link` — Retrieves a payment link.
- `square-pp-cli online-checkout update-location-settings` — Updates the location-level settings for a Square-hosted checkout page.
- `square-pp-cli online-checkout update-merchant-settings` — Updates the merchant-level settings for a Square-hosted checkout page.
- `square-pp-cli online-checkout update-payment-link` — Updates a payment link.

**orders** — Manage orders

- `square-pp-cli orders batch-retrieve` — Retrieves a set of [orders](entity:Order) by their IDs.
- `square-pp-cli orders bulk-delete-custom-attributes` — Deletes order [custom attributes](entity:CustomAttribute) as a bulk operation.
- `square-pp-cli orders bulk-upsert-custom-attributes` — Creates or updates order [custom attributes](entity:CustomAttribute) as a bulk operation.
- `square-pp-cli orders calculate` — Enables applications to preview order pricing without creating an order.
- `square-pp-cli orders clone` — Creates a new order, in the `DRAFT` state, by duplicating an existing order.
- `square-pp-cli orders create` — Creates a new [order](entity:Order)
- `square-pp-cli orders create-custom-attribute-definition` — Creates an order-related custom attribute definition.
- `square-pp-cli orders delete-custom-attribute-definition` — Deletes an order-related [custom attribute definition](entity:CustomAttributeDefinition) from a Square seller account.
- `square-pp-cli orders list-custom-attribute-definitions` — Lists the order-related [custom attribute definitions](entity:CustomAttributeDefinition)
- `square-pp-cli orders retrieve` — Retrieves an [Order](entity:Order) by ID.
- `square-pp-cli orders retrieve-custom-attribute-definition` — Retrieves an order-related [custom attribute definition](entity:CustomAttributeDefinition) from a Square seller account.
- `square-pp-cli orders search` — Search all orders for one or more locations.
- `square-pp-cli orders update` — Updates an open [order](entity:Order) by adding, replacing, or deleting fields.
- `square-pp-cli orders update-custom-attribute-definition` — Updates an order-related custom attribute definition for a Square seller account.

**payments** — Manage payments

- `square-pp-cli payments cancel-by-idempotency-key` — Cancels (voids) a payment identified by the idempotency key that is specified in the request.
- `square-pp-cli payments create` — Creates a payment using the provided source.
- `square-pp-cli payments get` — Retrieves details for a specific payment.
- `square-pp-cli payments list` — Retrieves a list of payments taken by the account making the request.
- `square-pp-cli payments update` — Updates a payment with the APPROVED status. You can update the `amount_money` and `tip_money` using this endpoint.

**payouts** — Manage payouts

- `square-pp-cli payouts get` — Retrieves details of a specific payout identified by a payout ID.
- `square-pp-cli payouts list` — Retrieves a list of all payouts for the default location.

**refunds** — Manage refunds

- `square-pp-cli refunds get-payment` — Retrieves a specific refund using the `refund_id`.
- `square-pp-cli refunds list-payment` — Retrieves a list of refunds for the account making the request.
- `square-pp-cli refunds payment` — Refunds a payment. You can refund the entire payment amount or a portion of it.

**reporting** — Manage reporting

- `square-pp-cli reporting get-metadata` — Returns available reporting cubes, measures, and dimensions.
- `square-pp-cli reporting load-query` — Runs a reporting query.

**sites** — Manage sites

- `square-pp-cli sites` — Lists the Square Online sites that belong to a seller. Sites are listed in descending order by the `created_at` date.

**subscriptions** — Manage subscriptions

- `square-pp-cli subscriptions bulk-swap-plan` — Schedules a plan variation change for all active subscriptions under a given plan variation.
- `square-pp-cli subscriptions create` — Enrolls a customer in a subscription.
- `square-pp-cli subscriptions retrieve` — Retrieves a specific subscription.
- `square-pp-cli subscriptions search` — Searches for subscriptions. Results are ordered chronologically by subscription creation date.
- `square-pp-cli subscriptions update` — Updates a subscription by modifying or clearing `subscription` field values. To clear a field, set its value to `null`.

**team-members** — Manage team members

- `square-pp-cli team-members bulk-create` — Creates multiple `TeamMember` objects. The created `TeamMember` objects are returned on successful creates.
- `square-pp-cli team-members bulk-update` — Updates multiple `TeamMember` objects. The updated `TeamMember` objects are returned on successful updates.
- `square-pp-cli team-members create` — Creates a single `TeamMember` object. The `TeamMember` object is returned on successful creates.
- `square-pp-cli team-members create-job` — Creates a job in a seller account. A job defines a title and tip eligibility.
- `square-pp-cli team-members list-jobs` — Lists jobs in a seller account. Results are sorted by title in ascending order.
- `square-pp-cli team-members retrieve` — Retrieves a `TeamMember` object for the given `TeamMember.id`.
- `square-pp-cli team-members retrieve-job` — Retrieves a specified job.
- `square-pp-cli team-members search` — Returns a paginated list of `TeamMember` objects for a business.
- `square-pp-cli team-members update` — Updates a single `TeamMember` object. The `TeamMember` object is returned on successful updates.
- `square-pp-cli team-members update-job` — Updates the title or tip eligibility of a job.

**terminals** — Manage terminals

- `square-pp-cli terminals cancel-action` — Cancels a Terminal action request if the status of the request permits it.
- `square-pp-cli terminals cancel-checkout` — Cancels a Terminal checkout request if the status of the request permits it.
- `square-pp-cli terminals cancel-refund` — Cancels an Interac Terminal refund request by refund request ID if the status of the request permits it.
- `square-pp-cli terminals create-action` — Creates a Terminal action request and sends it to the specified device.
- `square-pp-cli terminals create-checkout` — Creates a Terminal checkout request and sends it to the specified device to take a payment for the requested amount.
- `square-pp-cli terminals create-refund` — Creates a request to refund an Interac payment completed on a Square Terminal.
- `square-pp-cli terminals dismiss-action` — Dismisses a Terminal action request if the status and type of the request permits it.
- `square-pp-cli terminals dismiss-checkout` — Dismisses a Terminal checkout request if the status and type of the request permits it.
- `square-pp-cli terminals dismiss-refund` — Dismisses a Terminal refund request if the status and type of the request permits it.
- `square-pp-cli terminals get-action` — Retrieves a Terminal action request by `action_id`. Terminal action requests are available for 30 days.
- `square-pp-cli terminals get-checkout` — Retrieves a Terminal checkout request by `checkout_id`. Terminal checkout requests are available for 30 days.
- `square-pp-cli terminals get-refund` — Retrieves an Interac Terminal refund object by ID. Terminal refund objects are available for 30 days.
- `square-pp-cli terminals search-actions` — Retrieves a filtered list of Terminal action requests created by the account making the request.
- `square-pp-cli terminals search-checkouts` — Returns a filtered list of Terminal checkout requests created by the application making the request.
- `square-pp-cli terminals search-refunds` — Retrieves a filtered list of Interac Terminal refund requests created by the seller making the request.

**transfer-orders** — Manage transfer orders

- `square-pp-cli transfer-orders create` — Creates a new transfer order in [DRAFT](entity:TransferOrderStatus) status.
- `square-pp-cli transfer-orders delete` — Deletes a transfer order in [DRAFT](entity:TransferOrderStatus) status. Only draft orders can be deleted.
- `square-pp-cli transfer-orders retrieve` — Retrieves a specific [TransferOrder](entity:TransferOrder) by ID.
- `square-pp-cli transfer-orders search` — Searches for transfer orders using filters.
- `square-pp-cli transfer-orders update` — Updates an existing transfer order.

**vendors** — Manage vendors

- `square-pp-cli vendors bulk-create` — Creates one or more [Vendor](entity:Vendor) objects to represent suppliers to a seller.
- `square-pp-cli vendors bulk-retrieve` — Retrieves one or more vendors of specified [Vendor](entity:Vendor) IDs.
- `square-pp-cli vendors bulk-update` — Updates one or more of existing [Vendor](entity:Vendor) objects as suppliers to a seller.
- `square-pp-cli vendors create` — Creates a single [Vendor](entity:Vendor) object to represent a supplier to a seller.
- `square-pp-cli vendors retrieve` — Retrieves the vendor of a specified [Vendor](entity:Vendor) ID.
- `square-pp-cli vendors search` — Searches for vendors using a filter against supported [Vendor](entity:Vendor) properties and a supported sorter.
- `square-pp-cli vendors update` — Updates an existing [Vendor](entity:Vendor) object as a supplier to a seller.

**webhooks** — Manage webhooks

- `square-pp-cli webhooks create-subscription` — Creates a webhook subscription.
- `square-pp-cli webhooks delete-subscription` — Deletes a webhook subscription.
- `square-pp-cli webhooks list-event-types` — Lists all webhook event types that can be subscribed to.
- `square-pp-cli webhooks list-subscriptions` — Lists all webhook subscriptions owned by your application.
- `square-pp-cli webhooks retrieve-subscription` — Retrieves a webhook subscription identified by its ID.
- `square-pp-cli webhooks test-subscription` — Tests a webhook subscription by sending a test event to the notification URL.
- `square-pp-cli webhooks update-subscription` — Updates a webhook subscription.
- `square-pp-cli webhooks update-subscription-signature-key` — Updates a webhook subscription by replacing the existing signature key with a new one.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
square-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Explain yesterday's close

```bash
square-pp-cli reconcile close --since 24h --agent --select locations.location_name,locations.gross_sales,locations.gross_payments,locations.refunds,locations.net_collected,locations.payouts
```

Return only the location and settlement fields an agent needs to explain a discrepancy.

### Review stock drift

```bash
square-pp-cli inventory drift --since 7d --agent
```

Compare retained catalog and inventory snapshots across locations.

### Trace a customer

```bash
square-pp-cli customer timeline CUSTOMER_ABC123 --agent
```

Assemble connected customer activity into one chronological record.

### Check webhook delivery

```bash
square-pp-cli webhook health --since 24h --agent
```

Analyze real webhook receipts previously captured with webhook ingest, plus retained subscription drift.

### Validate a payment request offline

```bash
square-pp-cli request check --method POST --path /v2/payments --body payment-request.json --agent
```

Check the method and path against the shipped API contract, then inspect JSON readability, environment, API version, explicit mutation approval, and idempotency before sending anything.

## Auth Setup

Use a Square sandbox token while learning or testing. Set SQUARE_ACCESS_TOKEN outside your shell history and set SQUARE_BASE_URL=https://connect.squareupsandbox.com for sandbox calls. Inspect mutation help and use --dry-run before explicitly approving any write. Production and sandbox tokens are not interchangeable.

Run `square-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  square-pp-cli bank-accounts list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `SQUARE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SQUARE_CONFIG_DIR`, `SQUARE_DATA_DIR`, `SQUARE_STATE_DIR`, `SQUARE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SQUARE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `square-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "square": {
        "command": "square-pp-mcp",
        "env": {
          "SQUARE_HOME": "/srv/square"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SQUARE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SQUARE_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
square-pp-cli recall "<user's question>" --agent
```

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "square-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `square-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `square-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `square-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
square-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
square-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
square-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
square-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`square-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `SQUARE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
square-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
square-pp-cli feedback --stdin < notes.txt
square-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SQUARE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SQUARE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
square-pp-cli profile save briefing --json
square-pp-cli --profile briefing bank-accounts list
square-pp-cli profile list --json
square-pp-cli profile show briefing
square-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `square-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/square/cmd/square-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add square-pp-mcp -- square-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which square-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   square-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `square-pp-cli <command> --help`.
