---
name: pp-stripe-pp
description: "Every Stripe feature, plus a local SQLite mirror, FTS search, and compound-analytics commands no other Stripe tool has. Trigger phrases: `stripe payout breakdown`, `stripe failed payments by reason`, `stripe churn report`, `customer 360 in stripe`, `why did this stripe charge fail`, `use stripe-pp-pp-cli`, `run stripe-pp-pp-cli`."
author: "Primiasolutions"
license: "Apache-2.0"
argument-hint: " [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
 openclaw:
 requires:
 bins:
 - stripe-pp-pp-cli
 install:
 - kind: go
 bins: [stripe-pp-pp-cli]
 module: github.com/mvanhorn/printing-press-library/library/payments/stripe-pp/cmd/stripe-pp-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/payments/stripe-pp/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Stripe — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `stripe-pp-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install stripe --cli-only
   ```
2. Verify: `stripe-pp-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/stripe-pp/cmd/stripe-pp-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Mirror your Stripe account into a single-file SQLite database, then ask questions across customers, payments, subscriptions, refunds, and payouts with one command. Designed for agents (JSON, --select, --csv everywhere) and for the finance/RevOps user who lives in the dashboard today.

## When to Use This CLI

Reach for this CLI when you need to answer a money question about your own Stripe account — who churned, why payments failed, what's in this payout, what's a customer's lifetime value. Use stripe-cli (the official one) for webhook forwarding and event triggering; use this CLI for analyst-style questions and agent-driven workflows that need joined, deterministic, offline-fast answers.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`sync`** — Incrementally mirror your Stripe account into a local SQLite database — customers, payment_intents, charges, invoices, subscriptions, refunds, balance_transactions, payouts — with resumable cursors per entity.

  _Run this first in any Stripe analytics session — every other novel command queries the local mirror instead of hitting the API._

  ```bash
  stripe-pp-pp-cli sync --since 30d --resources customers,charges,invoices,subscriptions,balance_transactions --json
  ```
- **`payouts explain`** — Decompose a payout into every contributing balance_transaction (charges, refunds, fees, adjustments, disputes) with running totals and top-customer attribution.

  _Reach for this when a founder or finance lead asks 'is this payout right?' — replaces a 15-minute spreadsheet exercise with one CLI call._

  ```bash
  stripe-pp-pp-cli payouts explain po_1NXabc --json --select amount,arrival_date,breakdown,top_customers
  ```
- **`customers profile`** — Join one customer's record with their active subscriptions, lifetime charges and refunds, open invoices, recent activity, and computed lifetime value.

  _First command on any support ticket — replaces the dashboard scavenger hunt with one CLI call that hands the agent a complete customer record._

  ```bash
  stripe-pp-pp-cli customers profile cus_1234 --json --select id,email,lifetime_value,active_subscriptions,recent_failures,open_invoice_balance
  ```

### Compound analytics agents need
- **`payments failed`** — Group failed PaymentIntents by decline_code, customer, or amount band over a date window, with $ at risk and sample customers.

  _Morning standup query: 'did anything weird happen overnight?' — answers in one shot what's normally a 5-click dashboard journey._

  ```bash
  stripe-pp-pp-cli payments failed --since 2026-05-01 --group-by decline_code --json --select decline_code,count,total_amount,sample_customers
  ```
- **`subscriptions churn`** — List canceled or past_due subscriptions in a window, grouped by cancellation_details.reason, with MRR delta and customer LTV attribution.

  _First-of-the-month query: 'why are we down on MRR?' — turns vague churn into a ranked list of reasons with dollars attached._

  ```bash
  stripe-pp-pp-cli subscriptions churn --since 2026-04-01 --group-by cancellation_reason --mrr-delta --json
  ```

### Frustration eliminators
- **`charges why`** — For a failed charge, return decline_code, network message, that customer's recent successes/failures, and similar failures in the last 7 days.

  _Default agent action when a user pastes a charge ID — gives complete context for refund/retry/dispute decisions without further roundtrips._

  ```bash
  stripe-pp-pp-cli charges why ch_3Nabc --json --select decline_code,network_message,customer_recent_activity,similar_failures_7d
  ```

## Command Reference

**account** — Manage account

- `stripe-pp-pp-cli account` — Retrieves the details of an account.

**account-links** — Manage account links

- `stripe-pp-pp-cli account-links` — Creates an AccountLink object that includes a single-use Stripe URL that the platform can redirect their user to...

**account-sessions** — Manage account sessions

- `stripe-pp-pp-cli account-sessions` — Creates a AccountSession object that includes a single-use token that the platform can use on their front-end to...

**accounts** — Manage accounts

- `stripe-pp-pp-cli accounts delete` — With Connect, you can delete accounts you manage. Test-mode accounts can be deleted...
- `stripe-pp-pp-cli accounts get` — Returns a list of accounts connected to your platform via Connect. If you’re not a...
- `stripe-pp-pp-cli accounts get-account` — Retrieves the details of an account.
- `stripe-pp-pp-cli accounts post` — With Connect, you can create Stripe accounts for your users. To do this, you’ll...
- `stripe-pp-pp-cli accounts post-account` — Updates a connected account by setting the values of the parameters passed. Any...

**apple-pay** — Manage apple pay

- `stripe-pp-pp-cli apple-pay delete-domains-domain` — Delete an apple pay domain.
- `stripe-pp-pp-cli apple-pay get-domains` — List apple pay domains.
- `stripe-pp-pp-cli apple-pay get-domains-domain` — Retrieve an apple pay domain.
- `stripe-pp-pp-cli apple-pay post-domains` — Create an apple pay domain.

**application-fees** — Manage application fees

- `stripe-pp-pp-cli application-fees get` — Returns a list of application fees you’ve previously collected. The application fees are returned in sorted...
- `stripe-pp-pp-cli application-fees get-id` — Retrieves the details of an application fee that your account has collected. The same information is returned...

**apps** — Manage apps

- `stripe-pp-pp-cli apps get-secrets` — List all secrets stored on the given scope.
- `stripe-pp-pp-cli apps get-secrets-find` — Finds a secret in the secret store by name and scope.
- `stripe-pp-pp-cli apps post-secrets` — Create or replace a secret in the secret store.
- `stripe-pp-pp-cli apps post-secrets-delete` — Deletes a secret from the secret store by name and scope.

**balance** — Manage balance

- `stripe-pp-pp-cli balance get` — Retrieves the current account balance, based on the authentication that was used to make the request. For a...
- `stripe-pp-pp-cli balance get-history` — Returns a list of transactions that have contributed to the Stripe account balance (e.g., charges, transfers, and...
- `stripe-pp-pp-cli balance get-history-id` — Retrieves the balance transaction with the given ID. Note that this endpoint previously used the path...

**balance-settings** — Manage balance settings

- `stripe-pp-pp-cli balance-settings get` — Retrieves balance settings for a given connected account. Related guide: Making...
- `stripe-pp-pp-cli balance-settings post` — Updates balance settings for a given connected account. Related guide: Making...

**balance-transactions** — Manage balance transactions

- `stripe-pp-pp-cli balance-transactions get` — Returns a list of transactions that have contributed to the Stripe account balance (e.g., charges, transfers, and...
- `stripe-pp-pp-cli balance-transactions get-id` — Retrieves the balance transaction with the given ID. Note that this endpoint previously used the path...

**billing** — Manage billing

- `stripe-pp-pp-cli billing get-alerts` — Lists billing active and inactive alerts
- `stripe-pp-pp-cli billing get-alerts-id` — Retrieves a billing alert given an ID
- `stripe-pp-pp-cli billing get-credit-balance-summary` — Retrieves the credit balance summary for a customer.
- `stripe-pp-pp-cli billing get-credit-balance-transactions` — Retrieve a list of credit balance transactions.
- `stripe-pp-pp-cli billing get-credit-balance-transactions-id` — Retrieves a credit balance transaction.
- `stripe-pp-pp-cli billing get-credit-grants` — Retrieve a list of credit grants.
- `stripe-pp-pp-cli billing get-credit-grants-id` — Retrieves a credit grant.
- `stripe-pp-pp-cli billing get-meters` — Retrieve a list of billing meters.
- `stripe-pp-pp-cli billing get-meters-id` — Retrieves a billing meter given an ID.
- `stripe-pp-pp-cli billing get-meters-id-event-summaries` — Retrieve a list of billing meter event summaries.
- `stripe-pp-pp-cli billing post-alerts` — Creates a billing alert
- `stripe-pp-pp-cli billing post-alerts-id-activate` — Reactivates this alert, allowing it to trigger again.
- `stripe-pp-pp-cli billing post-alerts-id-archive` — Archives this alert, removing it from the list view and APIs. This is non-reversible.
- `stripe-pp-pp-cli billing post-alerts-id-deactivate` — Deactivates this alert, preventing it from triggering.
- `stripe-pp-pp-cli billing post-credit-grants` — Creates a credit grant.
- `stripe-pp-pp-cli billing post-credit-grants-id` — Updates a credit grant.
- `stripe-pp-pp-cli billing post-credit-grants-id-expire` — Expires a credit grant.
- `stripe-pp-pp-cli billing post-credit-grants-id-void` — Voids a credit grant.
- `stripe-pp-pp-cli billing post-meter-event-adjustments` — Creates a billing meter event adjustment.
- `stripe-pp-pp-cli billing post-meter-events` — Creates a billing meter event.
- `stripe-pp-pp-cli billing post-meters` — Creates a billing meter.
- `stripe-pp-pp-cli billing post-meters-id` — Updates a billing meter.
- `stripe-pp-pp-cli billing post-meters-id-deactivate` — When a meter is deactivated, no more meter events will be accepted for this meter. You can’t attach a...
- `stripe-pp-pp-cli billing post-meters-id-reactivate` — When a meter is reactivated, events for this meter can be accepted and you can attach the meter to a price.

**billing-portal** — Manage billing portal

- `stripe-pp-pp-cli billing-portal get-configurations` — Returns a list of configurations that describe the functionality of the customer portal.
- `stripe-pp-pp-cli billing-portal get-configurations-configuration` — Retrieves a configuration that describes the functionality of the customer portal.
- `stripe-pp-pp-cli billing-portal post-configurations` — Creates a configuration that describes the functionality and behavior of a PortalSession
- `stripe-pp-pp-cli billing-portal post-configurations-configuration` — Updates a configuration that describes the functionality of the customer portal.
- `stripe-pp-pp-cli billing-portal post-sessions` — Creates a session of the customer portal.

**charges** — Manage charges

- `stripe-pp-pp-cli charges get` — Returns a list of charges you’ve previously created. The charges are returned in sorted order, with the most...
- `stripe-pp-pp-cli charges get-charge` — Retrieves the details of a charge that has previously been created. Supply the unique charge ID that was returned...
- `stripe-pp-pp-cli charges get-search` — Search for charges you’ve previously created using Stripe’s This method is no longer recommended—use the Payment Intents API to...
- `stripe-pp-pp-cli charges post-charge` — Updates the specified charge by setting the values of the parameters passed. Any parameters not provided will be...

**checkout** — Manage checkout

- `stripe-pp-pp-cli checkout get-sessions` — Returns a list of Checkout Sessions.
- `stripe-pp-pp-cli checkout get-sessions-session` — Retrieves a Checkout Session object.
- `stripe-pp-pp-cli checkout get-sessions-session-line-items` — When retrieving a Checkout Session, there is an includable line_items property containing the...
- `stripe-pp-pp-cli checkout post-sessions` — Creates a Checkout Session object.
- `stripe-pp-pp-cli checkout post-sessions-session` — Updates a Checkout Session object. Related guide: Dynamically...
- `stripe-pp-pp-cli checkout post-sessions-session-expire` — A Checkout Session can be expired when it is in one of these statuses: open After it...

**climate** — Manage climate

- `stripe-pp-pp-cli climate get-orders` — Lists all Climate order objects. The orders are returned sorted by creation date, with the most recently created...
- `stripe-pp-pp-cli climate get-orders-order` — Retrieves the details of a Climate order object with the given ID.
- `stripe-pp-pp-cli climate get-products` — Lists all available Climate product objects.
- `stripe-pp-pp-cli climate get-products-product` — Retrieves the details of a Climate product with the given ID.
- `stripe-pp-pp-cli climate get-suppliers` — Lists all available Climate supplier objects.
- `stripe-pp-pp-cli climate get-suppliers-supplier` — Retrieves a Climate supplier object.
- `stripe-pp-pp-cli climate post-orders` — Creates a Climate order object for a given Climate product. The order will be processed immediately after...
- `stripe-pp-pp-cli climate post-orders-order` — Updates the specified order by setting the values of the parameters passed.
- `stripe-pp-pp-cli climate post-orders-order-cancel` — Cancels a Climate order. You can cancel an order within 24 hours of creation. Stripe refunds the reservation...

**confirmation-tokens** — Manage confirmation tokens

- `stripe-pp-pp-cli confirmation-tokens ` — Retrieves an existing ConfirmationToken object

**country-specs** — Manage country specs

- `stripe-pp-pp-cli country-specs get` — Lists all Country Spec objects available in the API.
- `stripe-pp-pp-cli country-specs get-country` — Returns a Country Spec for a given Country code.

**coupons** — Manage coupons

- `stripe-pp-pp-cli coupons delete` — You can delete coupons via the coupon management page of the...
- `stripe-pp-pp-cli coupons get` — Returns a list of your coupons.
- `stripe-pp-pp-cli coupons get-coupon` — Retrieves the coupon with the given ID.
- `stripe-pp-pp-cli coupons post` — You can create coupons easily via the coupon management page...
- `stripe-pp-pp-cli coupons post-coupon` — Updates the metadata of a coupon. Other coupon details (currency, duration, amount_off) are, by design, not...

**credit-notes** — Manage credit notes

- `stripe-pp-pp-cli credit-notes get` — Returns a list of credit notes.
- `stripe-pp-pp-cli credit-notes get-id` — Retrieves the credit note object with the given identifier.
- `stripe-pp-pp-cli credit-notes get-preview` — Get a preview of a credit note without creating it.
- `stripe-pp-pp-cli credit-notes get-preview-lines` — When retrieving a credit note preview, you’ll get a lines property containing the first...
- `stripe-pp-pp-cli credit-notes post` — Issue a credit note to adjust the amount of a finalized invoice. A credit note will first reduce the invoice’s...
- `stripe-pp-pp-cli credit-notes post-id` — Updates an existing credit note.

**customer-sessions** — Manage customer sessions

- `stripe-pp-pp-cli customer-sessions` — Creates a Customer Session object that includes a single-use client secret that you can use on your front-end to...

**customers** — Manage customers

- `stripe-pp-pp-cli customers delete` — Permanently deletes a customer. It cannot be undone. Also immediately cancels any active subscriptions on the...
- `stripe-pp-pp-cli customers get` — Returns a list of your customers. The customers are returned sorted by creation date, with the most recent...
- `stripe-pp-pp-cli customers get-customer` — Retrieves a Customer object.
- `stripe-pp-pp-cli customers get-search` — Search for customers you’ve previously created using Stripe’s Creates a new customer object.
- `stripe-pp-pp-cli customers post-customer` — Updates the specified customer by setting the values of the parameters passed. Any parameters not provided are...

**disputes** — Manage disputes

- `stripe-pp-pp-cli disputes get` — Returns a list of your disputes.
- `stripe-pp-pp-cli disputes get-dispute` — Retrieves the dispute with the given ID.
- `stripe-pp-pp-cli disputes post` — When you get a dispute, contacting your customer is always the best first step. If that doesn’t work, you can...

**entitlements** — Manage entitlements

- `stripe-pp-pp-cli entitlements get-active` — Retrieve a list of active entitlements for a customer
- `stripe-pp-pp-cli entitlements get-active-id` — Retrieve an active entitlement
- `stripe-pp-pp-cli entitlements get-features` — Retrieve a list of features
- `stripe-pp-pp-cli entitlements get-features-id` — Retrieves a feature
- `stripe-pp-pp-cli entitlements post-features` — Creates a feature
- `stripe-pp-pp-cli entitlements post-features-id` — Update a feature’s metadata or permanently deactivate it.

**ephemeral-keys** — Manage ephemeral keys

- `stripe-pp-pp-cli ephemeral-keys delete-key` — Invalidates a short-lived API key for a given resource.
- `stripe-pp-pp-cli ephemeral-keys post` — Creates a short-lived API key for a given resource.

**events** — Manage events

- `stripe-pp-pp-cli events get` — List events, going back up to 30 days. Each event data is rendered according to Stripe API version at its...
- `stripe-pp-pp-cli events get-id` — Retrieves the details of an event if it was created in the last 30 days. Supply the unique identifier of the...

**exchange-rates** — Manage exchange rates

- `stripe-pp-pp-cli exchange-rates get` — [Deprecated] The ExchangeRate APIs are deprecated. Please use the [Deprecated] The ExchangeRate APIs are deprecated. Please use the ` — Updates the metadata, account holder name, account holder type of a bank account belonging to a connected account...

**file-links** — Manage file links

- `stripe-pp-pp-cli file-links get` — Returns a list of file links.
- `stripe-pp-pp-cli file-links get-link` — Retrieves the file link with the given ID.
- `stripe-pp-pp-cli file-links post` — Creates a new file link object.
- `stripe-pp-pp-cli file-links post-link` — Updates an existing file link object. Expired links can no longer be updated.

**files** — Manage files

- `stripe-pp-pp-cli files get` — Returns a list of the files that your account has access to. Stripe sorts and returns the files by their creation...
- `stripe-pp-pp-cli files get-file` — Retrieves the details of an existing file object. After you supply a unique file ID, Stripe returns the...
- `stripe-pp-pp-cli files post` — To upload a file to Stripe, you need to send a request of type multipart/form-data. Include the file...

**financial-connections** — Manage financial connections

- `stripe-pp-pp-cli financial-connections get-accounts` — Returns a list of Financial Connections Account objects.
- `stripe-pp-pp-cli financial-connections get-accounts-account` — Retrieves the details of an Financial Connections Account.
- `stripe-pp-pp-cli financial-connections get-accounts-account-owners` — Lists all owners for a given Account
- `stripe-pp-pp-cli financial-connections get-sessions-session` — Retrieves the details of a Financial Connections Session
- `stripe-pp-pp-cli financial-connections get-transactions` — Returns a list of Financial Connections Transaction objects.
- `stripe-pp-pp-cli financial-connections get-transactions-transaction` — Retrieves the details of a Financial Connections Transaction
- `stripe-pp-pp-cli financial-connections post-accounts-account-disconnect` — Disables your access to a Financial Connections Account. You will no longer be able to access data...
- `stripe-pp-pp-cli financial-connections post-accounts-account-refresh` — Refreshes the data associated with a Financial Connections Account.
- `stripe-pp-pp-cli financial-connections post-accounts-account-subscribe` — Subscribes to periodic refreshes of data associated with a Financial Connections Account. When the...
- `stripe-pp-pp-cli financial-connections post-accounts-account-unsubscribe` — Unsubscribes from periodic refreshes of data associated with a Financial Connections Account.
- `stripe-pp-pp-cli financial-connections post-sessions` — To launch the Financial Connections authorization flow, create a Session. The session’s...

**forwarding** — Manage forwarding

- `stripe-pp-pp-cli forwarding get-requests` — Lists all ForwardingRequest objects.
- `stripe-pp-pp-cli forwarding get-requests-id` — Retrieves a ForwardingRequest object.
- `stripe-pp-pp-cli forwarding post-requests` — Creates a ForwardingRequest object.

**identity** — Manage identity

- `stripe-pp-pp-cli identity get-verification-reports` — List all verification reports.
- `stripe-pp-pp-cli identity get-verification-reports-report` — Retrieves an existing VerificationReport
- `stripe-pp-pp-cli identity get-verification-sessions` — Returns a list of VerificationSessions
- `stripe-pp-pp-cli identity get-verification-sessions-session` — Retrieves the details of a VerificationSession that was previously created. When the session status is...
- `stripe-pp-pp-cli identity post-verification-sessions` — Creates a VerificationSession object. After the VerificationSession is created, display a verification...
- `stripe-pp-pp-cli identity post-verification-sessions-session` — Updates a VerificationSession object. When the session status is requires_input, you can use...
- `stripe-pp-pp-cli identity post-verification-sessions-session-cancel` — A VerificationSession object can be canceled when it is in requires_input Redact a VerificationSession to remove all collected information from Stripe. This will redact the...

**invoice-payments** — Manage invoice payments

- `stripe-pp-pp-cli invoice-payments get` — When retrieving an invoice, there is an includable payments property containing the first handful of those items....
- `stripe-pp-pp-cli invoice-payments get-invoicepayments` — Retrieves the invoice payment with the given ID.

**invoice-rendering-templates** — Manage invoice rendering templates

- `stripe-pp-pp-cli invoice-rendering-templates get` — List all templates, ordered by creation date, with the most recently created template appearing first.
- `stripe-pp-pp-cli invoice-rendering-templates get-template` — Retrieves an invoice rendering template with the given ID. It by default returns the latest version of the...

**invoiceitems** — Manage invoiceitems

- `stripe-pp-pp-cli invoiceitems delete` — Deletes an invoice item, removing it from an invoice. Deleting invoice items is only possible when they’re not...
- `stripe-pp-pp-cli invoiceitems get` — Returns a list of your invoice items. Invoice items are returned sorted by creation date, with the most recently...
- `stripe-pp-pp-cli invoiceitems get-invoiceitem` — Retrieves the invoice item with the given ID.
- `stripe-pp-pp-cli invoiceitems post` — Creates an item to be added to a draft invoice (up to 250 items per invoice). If no invoice is specified, the...
- `stripe-pp-pp-cli invoiceitems post-invoiceitem` — Updates the amount or description of an invoice item on an upcoming invoice. Updating an invoice item is only...

**invoices** — Manage invoices

- `stripe-pp-pp-cli invoices delete` — Permanently deletes a one-off invoice draft. This cannot be undone. Attempts to delete invoices that are no...
- `stripe-pp-pp-cli invoices get` — You can list all invoices, or list the invoices for a specific customer. The invoices are returned sorted by...
- `stripe-pp-pp-cli invoices get-invoice` — Retrieves the invoice with the given ID.
- `stripe-pp-pp-cli invoices get-search` — Search for invoices you’ve previously created using Stripe’s This endpoint creates a draft invoice for a given customer. The invoice remains a draft until you At any time, you can preview the upcoming invoice for a subscription or subscription schedule. This will show you...
- `stripe-pp-pp-cli invoices post-invoice` — Draft invoices are fully editable. Once an invoice is finalize...

**issuing** — Manage issuing

- `stripe-pp-pp-cli issuing get-authorizations` — Returns a list of Issuing Authorization objects. The objects are sorted in descending order by...
- `stripe-pp-pp-cli issuing get-authorizations-authorization` — Retrieves an Issuing Authorization object.
- `stripe-pp-pp-cli issuing get-cardholders` — Returns a list of Issuing Cardholder objects. The objects are sorted in descending order by creation...
- `stripe-pp-pp-cli issuing get-cardholders-cardholder` — Retrieves an Issuing Cardholder object.
- `stripe-pp-pp-cli issuing get-cards` — Returns a list of Issuing Card objects. The objects are sorted in descending order by creation date,...
- `stripe-pp-pp-cli issuing get-cards-card` — Retrieves an Issuing Card object.
- `stripe-pp-pp-cli issuing get-disputes` — Returns a list of Issuing Dispute objects. The objects are sorted in descending order by creation...
- `stripe-pp-pp-cli issuing get-disputes-dispute` — Retrieves an Issuing Dispute object.
- `stripe-pp-pp-cli issuing get-personalization-designs` — Returns a list of personalization design objects. The objects are sorted in descending order by creation date,...
- `stripe-pp-pp-cli issuing get-personalization-designs-personalization-design` — Retrieves a personalization design object.
- `stripe-pp-pp-cli issuing get-physical-bundles` — Returns a list of physical bundle objects. The objects are sorted in descending order by creation date, with the...
- `stripe-pp-pp-cli issuing get-physical-bundles-physical-bundle` — Retrieves a physical bundle object.
- `stripe-pp-pp-cli issuing get-settlements-settlement` — Retrieves an Issuing Settlement object.
- `stripe-pp-pp-cli issuing get-tokens` — Lists all Issuing Token objects for a given card.
- `stripe-pp-pp-cli issuing get-tokens-token` — Retrieves an Issuing Token object.
- `stripe-pp-pp-cli issuing get-transactions` — Returns a list of Issuing Transaction objects. The objects are sorted in descending order by...
- `stripe-pp-pp-cli issuing get-transactions-transaction` — Retrieves an Issuing Transaction object.
- `stripe-pp-pp-cli issuing post-authorizations-authorization` — Updates the specified Issuing Authorization object by setting the values of the parameters passed....
- `stripe-pp-pp-cli issuing post-authorizations-authorization-approve` — [Deprecated] Approves a pending Issuing Authorization object. This request should be made within the...
- `stripe-pp-pp-cli issuing post-authorizations-authorization-decline` — [Deprecated] Declines a pending Issuing Authorization object. This request should be made within the...
- `stripe-pp-pp-cli issuing post-cardholders` — Creates a new Issuing Cardholder object that can be issued cards.
- `stripe-pp-pp-cli issuing post-cardholders-cardholder` — Updates the specified Issuing Cardholder object by setting the values of the parameters passed. Any...
- `stripe-pp-pp-cli issuing post-cards` — Creates an Issuing Card object.
- `stripe-pp-pp-cli issuing post-cards-card` — Updates the specified Issuing Card object by setting the values of the parameters passed. Any...
- `stripe-pp-pp-cli issuing post-disputes` — Creates an Issuing Dispute object. Individual pieces of evidence within the evidence...
- `stripe-pp-pp-cli issuing post-disputes-dispute` — Updates the specified Issuing Dispute object by setting the values of the parameters passed. Any...
- `stripe-pp-pp-cli issuing post-disputes-dispute-submit` — Submits an Issuing Dispute to the card network. Stripe validates that all evidence fields required...
- `stripe-pp-pp-cli issuing post-personalization-designs` — Creates a personalization design object.
- `stripe-pp-pp-cli issuing post-personalization-designs-personalization-design` — Updates a card personalization object.
- `stripe-pp-pp-cli issuing post-settlements-settlement` — Updates the specified Issuing Settlement object by setting the values of the parameters passed. Any...
- `stripe-pp-pp-cli issuing post-tokens-token` — Attempts to update the specified Issuing Token object to the status specified.
- `stripe-pp-pp-cli issuing post-transactions-transaction` — Updates the specified Issuing Transaction object by setting the values of the parameters passed. Any...

**link-account-sessions** — Manage link account sessions

- `stripe-pp-pp-cli link-account-sessions get-session` — Retrieves the details of a Financial Connections Session
- `stripe-pp-pp-cli link-account-sessions post` — To launch the Financial Connections authorization flow, create a Session. The session’s...

**linked-accounts** — Manage linked accounts

- `stripe-pp-pp-cli linked-accounts get` — Returns a list of Financial Connections Account objects.
- `stripe-pp-pp-cli linked-accounts get-account` — Retrieves the details of an Financial Connections Account.

**mandates** — Manage mandates

- `stripe-pp-pp-cli mandates` — Retrieves a Mandate object.

**payment-attempt-records** — Manage payment attempt records

- `stripe-pp-pp-cli payment-attempt-records get` — List all the Payment Attempt Records attached to the specified Payment Record.
- `stripe-pp-pp-cli payment-attempt-records get-id` — Retrieves a Payment Attempt Record with the given ID

**payment-intents** — Manage payment intents

- `stripe-pp-pp-cli payment-intents get` — Returns a list of PaymentIntents.
- `stripe-pp-pp-cli payment-intents get-intent` — Retrieves the details of a PaymentIntent that has previously been created. You can retrieve a...
- `stripe-pp-pp-cli payment-intents get-search` — Search for PaymentIntents you’ve previously created using Stripe’s Creates a PaymentIntent object. After the PaymentIntent is created, attach a payment method and Updates properties on a PaymentIntent object without confirming. Depending on which properties you update,...

**payment-links** — Manage payment links

- `stripe-pp-pp-cli payment-links get` — Returns a list of your payment links.
- `stripe-pp-pp-cli payment-links get-paymentlinks` — Retrieve a payment link.
- `stripe-pp-pp-cli payment-links post` — Creates a payment link.
- `stripe-pp-pp-cli payment-links post-paymentlinks` — Updates a payment link.

**payment-method-configurations** — Manage payment method configurations

- `stripe-pp-pp-cli payment-method-configurations get` — List payment method configurations
- `stripe-pp-pp-cli payment-method-configurations get-configuration` — Retrieve payment method configuration
- `stripe-pp-pp-cli payment-method-configurations post` — Creates a payment method configuration
- `stripe-pp-pp-cli payment-method-configurations post-configuration` — Update payment method configuration

**payment-method-domains** — Manage payment method domains

- `stripe-pp-pp-cli payment-method-domains get` — Lists the details of existing payment method domains.
- `stripe-pp-pp-cli payment-method-domains get-paymentmethoddomains` — Retrieves the details of an existing payment method domain.
- `stripe-pp-pp-cli payment-method-domains post` — Creates a payment method domain.
- `stripe-pp-pp-cli payment-method-domains post-paymentmethoddomains` — Updates an existing payment method domain.

**payment-methods** — Manage payment methods

- `stripe-pp-pp-cli payment-methods get` — Returns a list of all PaymentMethods.
- `stripe-pp-pp-cli payment-methods get-paymentmethods` — Retrieves a PaymentMethod object attached to the StripeAccount. To retrieve a payment method attached to a...
- `stripe-pp-pp-cli payment-methods post` — Creates a PaymentMethod object. Read the Stripe.j...
- `stripe-pp-pp-cli payment-methods post-paymentmethods` — Updates a PaymentMethod object. A PaymentMethod must be attached to a customer to be updated.

**payment-records** — Manage payment records

- `stripe-pp-pp-cli payment-records get-id` — Retrieves a Payment Record with the given ID
- `stripe-pp-pp-cli payment-records post-report-payment` — Report a new Payment Record. You may report a Payment Record as it is initialized and later report updates...

**payouts** — Manage payouts

- `stripe-pp-pp-cli payouts get` — Returns a list of existing payouts sent to third-party bank accounts or payouts that Stripe sent to you. The...
- `stripe-pp-pp-cli payouts get-payout` — Retrieves the details of an existing payout. Supply the unique payout ID from either a payout creation request or...
- `stripe-pp-pp-cli payouts post` — To send funds to your own bank account, create a new payout object. Your Stripe balance...
- `stripe-pp-pp-cli payouts post-payout` — Updates the specified payout by setting the values of the parameters you pass. We don’t change parameters that...

**plans** — Manage plans

- `stripe-pp-pp-cli plans delete` — Deleting plans means new subscribers can’t be added. Existing subscribers aren’t affected.
- `stripe-pp-pp-cli plans get` — Returns a list of your plans.
- `stripe-pp-pp-cli plans get-plan` — Retrieves the plan with the given ID.
- `stripe-pp-pp-cli plans post` — You can now model subscriptions more flexibly using the Prices API. It replaces the Plans...
- `stripe-pp-pp-cli plans post-plan` — Updates the specified plan by setting the values of the parameters passed. Any parameters not provided are left...

**prices** — Manage prices

- `stripe-pp-pp-cli prices get` — Returns a list of your active prices, excluding inli...
- `stripe-pp-pp-cli prices get-price` — Retrieves the price with the given ID.
- `stripe-pp-pp-cli prices get-search` — Search for prices you’ve previously created using Stripe’s Creates a new Price for an existing Updates the specified price by setting the values of the parameters passed. Any parameters not provided are left...

**products** — Manage products

- `stripe-pp-pp-cli products delete-id` — Delete a product. Deleting a product is only possible if it has no prices associated with it. Additionally,...
- `stripe-pp-pp-cli products get` — Returns a list of your products. The products are returned sorted by creation date, with the most recently...
- `stripe-pp-pp-cli products get-id` — Retrieves the details of an existing product. Supply the unique product ID from either a product creation request...
- `stripe-pp-pp-cli products get-search` — Search for products you’ve previously created using Stripe’s Creates a new product object.
- `stripe-pp-pp-cli products post-id` — Updates the specific product by setting the values of the parameters passed. Any parameters not provided will be...

**promotion-codes** — Manage promotion codes

- `stripe-pp-pp-cli promotion-codes get` — Returns a list of your promotion codes.
- `stripe-pp-pp-cli promotion-codes get-promotioncodes` — Retrieves the promotion code with the given ID. In order to retrieve a promotion code by the customer-facing...
- `stripe-pp-pp-cli promotion-codes post` — A promotion code points to an underlying promotion. You can optionally restrict the code to a specific customer,...
- `stripe-pp-pp-cli promotion-codes post-promotioncodes` — Updates the specified promotion code by setting the values of the parameters passed. Most fields are, by design,...

**quotes** — Manage quotes

- `stripe-pp-pp-cli quotes get` — Returns a list of your quotes.
- `stripe-pp-pp-cli quotes get-quote` — Retrieves the quote with the given ID.
- `stripe-pp-pp-cli quotes post` — A quote models prices and services for a customer. Default options for header,...
- `stripe-pp-pp-cli quotes post-quote` — A quote models prices and services for a customer.

**radar** — Manage radar

- `stripe-pp-pp-cli radar delete-value-list-items-item` — Deletes a ValueListItem object, removing it from its parent value list.
- `stripe-pp-pp-cli radar delete-value-lists-value-list` — Deletes a ValueList object, also deleting any items contained within the value list. To be deleted,...
- `stripe-pp-pp-cli radar get-early-fraud-warnings` — Returns a list of early fraud warnings.
- `stripe-pp-pp-cli radar get-early-fraud-warnings-early-fraud-warning` — Retrieves the details of an early fraud warning that has previously been created. Please refer to the Returns a list of ValueListItem objects. The objects are sorted in descending order by creation...
- `stripe-pp-pp-cli radar get-value-list-items-item` — Retrieves a ValueListItem object.
- `stripe-pp-pp-cli radar get-value-lists` — Returns a list of ValueList objects. The objects are sorted in descending order by creation date,...
- `stripe-pp-pp-cli radar get-value-lists-value-list` — Retrieves a ValueList object.
- `stripe-pp-pp-cli radar post-payment-evaluations` — Request a Radar API fraud risk score from Stripe for a payment before sending it for external processor...
- `stripe-pp-pp-cli radar post-value-list-items` — Creates a new ValueListItem object, which is added to the specified parent value list.
- `stripe-pp-pp-cli radar post-value-lists` — Creates a new ValueList object, which can then be referenced in rules.
- `stripe-pp-pp-cli radar post-value-lists-value-list` — Updates a ValueList object by setting the values of the parameters passed. Any parameters not...

**refunds** — Manage refunds

- `stripe-pp-pp-cli refunds get` — Returns a list of all refunds you created. We return the refunds in sorted order, with the most recent refunds...
- `stripe-pp-pp-cli refunds get-refund` — Retrieves the details of an existing refund.
- `stripe-pp-pp-cli refunds post` — When you create a new refund, you must specify a Charge or a PaymentIntent object on which to create it....
- `stripe-pp-pp-cli refunds post-refund` — Updates the refund that you specify by setting the values of the passed parameters. Any parameters that you...

**reporting** — Manage reporting

- `stripe-pp-pp-cli reporting get-report-runs` — Returns a list of Report Runs, with the most recent appearing first.
- `stripe-pp-pp-cli reporting get-report-runs-report-run` — Retrieves the details of an existing Report Run.
- `stripe-pp-pp-cli reporting get-report-types` — Returns a full list of Report Types.
- `stripe-pp-pp-cli reporting get-report-types-report-type` — Retrieves the details of a Report Type. (Certain report types require a Creates a new object and begin running the report. (Certain report types require a Returns a list of Review objects that have open set to true. The objects...
- `stripe-pp-pp-cli reviews get-review` — Retrieves a Review object.

**setup-attempts** — Manage setup attempts

- `stripe-pp-pp-cli setup-attempts` — Returns a list of SetupAttempts that associate with a provided SetupIntent.

**setup-intents** — Manage setup intents

- `stripe-pp-pp-cli setup-intents get` — Returns a list of SetupIntents.
- `stripe-pp-pp-cli setup-intents get-intent` — Retrieves the details of a SetupIntent that has previously been created. Client-side retrieval using a...
- `stripe-pp-pp-cli setup-intents post` — Creates a SetupIntent object. After you create the SetupIntent, attach a payment method and Updates a SetupIntent object.

**shipping-rates** — Manage shipping rates

- `stripe-pp-pp-cli shipping-rates get` — Returns a list of your shipping rates.
- `stripe-pp-pp-cli shipping-rates get-token` — Returns the shipping rate object with the given ID.
- `stripe-pp-pp-cli shipping-rates post` — Creates a new shipping rate object.
- `stripe-pp-pp-cli shipping-rates post-token` — Updates an existing shipping rate object.

**sigma** — Manage sigma

- `stripe-pp-pp-cli sigma get-scheduled-query-runs` — Returns a list of scheduled query runs.
- `stripe-pp-pp-cli sigma get-scheduled-query-runs-scheduled-query-run` — Retrieves the details of an scheduled query run.
- `stripe-pp-pp-cli sigma post-saved-queries-id` — Update an existing Sigma query that previously exists

**sources** — Manage sources

- `stripe-pp-pp-cli sources get` — Retrieves an existing source object. Supply the unique source ID from a source creation request and Stripe will...
- `stripe-pp-pp-cli sources post` — Creates a new source object.
- `stripe-pp-pp-cli sources post-source` — Updates the specified source by setting the values of the parameters passed. Any parameters not provided will be...

**subscription-items** — Manage subscription items

- `stripe-pp-pp-cli subscription-items delete-item` — Deletes an item from the subscription. Removing a subscription item from a subscription will not cancel the...
- `stripe-pp-pp-cli subscription-items get` — Returns a list of your subscription items for a given subscription.
- `stripe-pp-pp-cli subscription-items get-item` — Retrieves the subscription item with the given ID.
- `stripe-pp-pp-cli subscription-items post` — Adds a new item to an existing subscription. No existing items will be changed or replaced.
- `stripe-pp-pp-cli subscription-items post-item` — Updates the plan or quantity of an item on a current subscription.

**subscription-schedules** — Manage subscription schedules

- `stripe-pp-pp-cli subscription-schedules get` — Retrieves the list of your subscription schedules.
- `stripe-pp-pp-cli subscription-schedules get-schedule` — Retrieves the details of an existing subscription schedule. You only need to supply the unique subscription...
- `stripe-pp-pp-cli subscription-schedules post` — Creates a new subscription schedule object. Each customer can have up to 500 active or scheduled subscriptions.
- `stripe-pp-pp-cli subscription-schedules post-schedule` — Updates an existing subscription schedule.

**subscriptions** — Manage subscriptions

- `stripe-pp-pp-cli subscriptions delete-exposed-id` — Cancels a customer’s subscription immediately. The customer won’t be charged again for the subscription....
- `stripe-pp-pp-cli subscriptions get` — By default, returns a list of subscriptions that have not been canceled. In order to list canceled subscriptions,...
- `stripe-pp-pp-cli subscriptions get-exposed-id` — Retrieves the subscription with the given ID.
- `stripe-pp-pp-cli subscriptions get-search` — Search for subscriptions you’ve previously created using Stripe’s Creates a new subscription on an existing customer. Each customer can have up to 500 active or scheduled...
- `stripe-pp-pp-cli subscriptions post-exposed-id` — Updates an existing subscription to match the specified parameters. When changing prices or quantities, we...

**tax** — Manage tax

- `stripe-pp-pp-cli tax get-associations-find` — Finds a tax association object by PaymentIntent id.
- `stripe-pp-pp-cli tax get-calculations-calculation` — Retrieves a Tax Calculation object, if the calculation hasn’t expired.
- `stripe-pp-pp-cli tax get-calculations-calculation-line-items` — Retrieves the line items of a tax calculation as a collection, if the calculation hasn’t expired.
- `stripe-pp-pp-cli tax get-registrations` — Returns a list of Tax Registration objects.
- `stripe-pp-pp-cli tax get-registrations-id` — Returns a Tax Registration object.
- `stripe-pp-pp-cli tax get-settings` — Retrieves Tax Settings for a merchant.
- `stripe-pp-pp-cli tax get-transactions-transaction` — Retrieves a Tax Transaction object.
- `stripe-pp-pp-cli tax get-transactions-transaction-line-items` — Retrieves the line items of a committed standalone transaction as a collection.
- `stripe-pp-pp-cli tax post-calculations` — Calculates tax based on the input and returns a Tax Calculation object.
- `stripe-pp-pp-cli tax post-registrations` — Creates a new Tax Registration object.
- `stripe-pp-pp-cli tax post-registrations-id` — Updates an existing Tax Registration object. A registration cannot be deleted after it has...
- `stripe-pp-pp-cli tax post-settings` — Updates Tax Settings parameters used in tax calculations. All parameters are editable but none can...
- `stripe-pp-pp-cli tax post-transactions-create-from-calculation` — Creates a Tax Transaction from a calculation, if that calculation hasn’t expired. Calculations expire after 90...
- `stripe-pp-pp-cli tax post-transactions-create-reversal` — Partially or fully reverses a previously created Transaction.

**tax-codes** — Manage tax codes

- `stripe-pp-pp-cli tax-codes get` — A list of all tax codes available to add to Products in...
- `stripe-pp-pp-cli tax-codes get-id` — Retrieves the details of an existing tax code. Supply the unique tax code ID and Stripe will return the...

**tax-ids** — Manage tax ids

- `stripe-pp-pp-cli tax-ids delete-id` — Deletes an existing account or customer tax_id object.
- `stripe-pp-pp-cli tax-ids get` — Returns a list of tax IDs.
- `stripe-pp-pp-cli tax-ids get-id` — Retrieves an account or customer tax_id object.
- `stripe-pp-pp-cli tax-ids post` — Creates a new account or customer tax_id object.

**tax-rates** — Manage tax rates

- `stripe-pp-pp-cli tax-rates get` — Returns a list of your tax rates. Tax rates are returned sorted by creation date, with the most recently created...
- `stripe-pp-pp-cli tax-rates get-taxrates` — Retrieves a tax rate with the given ID
- `stripe-pp-pp-cli tax-rates post` — Creates a new tax rate.
- `stripe-pp-pp-cli tax-rates post-taxrates` — Updates an existing tax rate.

**terminal** — Manage terminal

- `stripe-pp-pp-cli terminal delete-configurations-configuration` — Deletes a Configuration object.
- `stripe-pp-pp-cli terminal delete-locations-location` — Deletes a Location object.
- `stripe-pp-pp-cli terminal delete-readers-reader` — Deletes a Reader object.
- `stripe-pp-pp-cli terminal get-configurations` — Returns a list of Configuration objects.
- `stripe-pp-pp-cli terminal get-configurations-configuration` — Retrieves a Configuration object.
- `stripe-pp-pp-cli terminal get-locations` — Returns a list of Location objects.
- `stripe-pp-pp-cli terminal get-locations-location` — Retrieves a Location object.
- `stripe-pp-pp-cli terminal get-readers` — Returns a list of Reader objects.
- `stripe-pp-pp-cli terminal get-readers-reader` — Retrieves a Reader object.
- `stripe-pp-pp-cli terminal post-configurations` — Creates a new Configuration object.
- `stripe-pp-pp-cli terminal post-configurations-configuration` — Updates a new Configuration object.
- `stripe-pp-pp-cli terminal post-connection-tokens` — To connect to a reader the Stripe Terminal SDK needs to retrieve a short-lived connection token from Stripe,...
- `stripe-pp-pp-cli terminal post-locations` — Creates a new Location object. For further details, including which address fields are required in...
- `stripe-pp-pp-cli terminal post-locations-location` — Updates a Location object by setting the values of the parameters passed. Any parameters not...
- `stripe-pp-pp-cli terminal post-onboarding-links` — Creates a new OnboardingLink object that contains a redirect_url used for onboarding onto Tap to Pay...
- `stripe-pp-pp-cli terminal post-readers` — Creates a new Reader object.
- `stripe-pp-pp-cli terminal post-readers-reader` — Updates a Reader object by setting the values of the parameters passed. Any parameters not provided...
- `stripe-pp-pp-cli terminal post-readers-reader-cancel-action` — Cancels the current reader action. See Initiates an input collection flow on a Reader to display...
- `stripe-pp-pp-cli terminal post-readers-reader-collect-payment-method` — Initiates a payment flow on a Reader and updates the PaymentIntent with card details before manual confirmation....
- `stripe-pp-pp-cli terminal post-readers-reader-confirm-payment-intent` — Finalizes a payment on a Reader. See Initiates a payment flow on a Reader. See Initiates a SetupIntent flow on a Reader. See Initiates an in-person refund on a Reader. See Sets the reader display to show cart details.
- `stripe-pp-pp-cli terminal post-refunds` — Internal endpoint for terminal use to create a refund for a card_present charge. This endpoint only supports...

**test-helpers** — Manage test helpers

- `stripe-pp-pp-cli test-helpers delete-test-clocks-test-clock` — Deletes a test clock.
- `stripe-pp-pp-cli test-helpers get-test-clocks` — Returns a list of your test clocks.
- `stripe-pp-pp-cli test-helpers get-test-clocks-test-clock` — Retrieves a test clock.
- `stripe-pp-pp-cli test-helpers post-confirmation-tokens` — Creates a test mode Confirmation Token server side for your integration tests.
- `stripe-pp-pp-cli test-helpers post-customers-customer-fund-cash-balance` — Create an incoming testmode bank transfer
- `stripe-pp-pp-cli test-helpers post-issuing-authorizations` — Create a test-mode authorization.
- `stripe-pp-pp-cli test-helpers post-issuing-authorizations-authorization-capture` — Capture a test-mode authorization.
- `stripe-pp-pp-cli test-helpers post-issuing-authorizations-authorization-expire` — Expire a test-mode Authorization.
- `stripe-pp-pp-cli test-helpers post-issuing-authorizations-authorization-finalize-amount` — Finalize the amount on an Authorization prior to capture, when the initial authorization was for an estimated...
- `stripe-pp-pp-cli test-helpers post-issuing-authorizations-authorization-fraud-challenges-respond` — Respond to a fraud challenge on a testmode Issuing authorization, simulating either a confirmation of fraud or a...
- `stripe-pp-pp-cli test-helpers post-issuing-authorizations-authorization-increment` — Increment a test-mode Authorization.
- `stripe-pp-pp-cli test-helpers post-issuing-authorizations-authorization-reverse` — Reverse a test-mode Authorization.
- `stripe-pp-pp-cli test-helpers post-issuing-cards-card-shipping-deliver` — Updates the shipping status of the specified Issuing Card object to delivered.
- `stripe-pp-pp-cli test-helpers post-issuing-cards-card-shipping-fail` — Updates the shipping status of the specified Issuing Card object to failure.
- `stripe-pp-pp-cli test-helpers post-issuing-cards-card-shipping-return` — Updates the shipping status of the specified Issuing Card object to returned.
- `stripe-pp-pp-cli test-helpers post-issuing-cards-card-shipping-ship` — Updates the shipping status of the specified Issuing Card object to shipped.
- `stripe-pp-pp-cli test-helpers post-issuing-cards-card-shipping-submit` — Updates the shipping status of the specified Issuing Card object to submitted. This...
- `stripe-pp-pp-cli test-helpers post-issuing-personalization-designs-personalization-design-activate` — Updates the status of the specified testmode personalization design object to active.
- `stripe-pp-pp-cli test-helpers post-issuing-personalization-designs-personalization-design-deactivate` — Updates the status of the specified testmode personalization design object to inactive.
- `stripe-pp-pp-cli test-helpers post-issuing-personalization-designs-personalization-design-reject` — Updates the status of the specified testmode personalization design object to rejected.
- `stripe-pp-pp-cli test-helpers post-issuing-settlements` — Allows the user to create an Issuing settlement.
- `stripe-pp-pp-cli test-helpers post-issuing-settlements-settlement-complete` — Allows the user to mark an Issuing settlement as complete.
- `stripe-pp-pp-cli test-helpers post-issuing-transactions-create-force-capture` — Allows the user to capture an arbitrary amount, also known as a forced capture.
- `stripe-pp-pp-cli test-helpers post-issuing-transactions-create-unlinked-refund` — Allows the user to refund an arbitrary amount, also known as a unlinked refund.
- `stripe-pp-pp-cli test-helpers post-issuing-transactions-transaction-refund` — Refund a test-mode Transaction.
- `stripe-pp-pp-cli test-helpers post-refunds-refund-expire` — Expire a refund with a status of requires_action.
- `stripe-pp-pp-cli test-helpers post-terminal-readers-reader-present-payment-method` — Presents a payment method on a simulated reader. Can be used to simulate accepting a payment, saving a card or...
- `stripe-pp-pp-cli test-helpers post-terminal-readers-reader-succeed-input-collection` — Use this endpoint to trigger a successful input collection on a simulated reader.
- `stripe-pp-pp-cli test-helpers post-terminal-readers-reader-timeout-input-collection` — Use this endpoint to complete an input collection with a timeout error on a simulated reader.
- `stripe-pp-pp-cli test-helpers post-test-clocks` — Creates a new test clock that can be attached to new customers and quotes.
- `stripe-pp-pp-cli test-helpers post-test-clocks-test-clock-advance` — Starts advancing a test clock to a specified time in the future. Advancement is done when status changes to...
- `stripe-pp-pp-cli test-helpers post-treasury-inbound-transfers-id-fail` — Transitions a test mode created InboundTransfer to the failed status. The InboundTransfer must...
- `stripe-pp-pp-cli test-helpers post-treasury-inbound-transfers-id-return` — Marks the test mode InboundTransfer object as returned and links the InboundTransfer to a ReceivedDebit. The...
- `stripe-pp-pp-cli test-helpers post-treasury-inbound-transfers-id-succeed` — Transitions a test mode created InboundTransfer to the succeeded status. The InboundTransfer must...
- `stripe-pp-pp-cli test-helpers post-treasury-outbound-payments-id` — Updates a test mode created OutboundPayment with tracking details. The OutboundPayment must not be cancelable,...
- `stripe-pp-pp-cli test-helpers post-treasury-outbound-payments-id-fail` — Transitions a test mode created OutboundPayment to the failed status. The OutboundPayment must...
- `stripe-pp-pp-cli test-helpers post-treasury-outbound-payments-id-post` — Transitions a test mode created OutboundPayment to the posted status. The OutboundPayment must...
- `stripe-pp-pp-cli test-helpers post-treasury-outbound-payments-id-return` — Transitions a test mode created OutboundPayment to the returned status. The OutboundPayment must...
- `stripe-pp-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer` — Updates a test mode created OutboundTransfer with tracking details. The OutboundTransfer must not be cancelable,...
- `stripe-pp-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer-fail` — Transitions a test mode created OutboundTransfer to the failed status. The OutboundTransfer must...
- `stripe-pp-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer-post` — Transitions a test mode created OutboundTransfer to the posted status. The OutboundTransfer must...
- `stripe-pp-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer-return` — Transitions a test mode created OutboundTransfer to the returned status. The OutboundTransfer must...
- `stripe-pp-pp-cli test-helpers post-treasury-received-credits` — Use this endpoint to simulate a test mode ReceivedCredit initiated by a third party. In live mode, you can’t...
- `stripe-pp-pp-cli test-helpers post-treasury-received-debits` — Use this endpoint to simulate a test mode ReceivedDebit initiated by a third party. In live mode, you can’t...

**tokens** — Manage tokens

- `stripe-pp-pp-cli tokens get` — Retrieves the token with the given ID.
- `stripe-pp-pp-cli tokens post` — Creates a single-use token that represents a bank account’s details. You can use this token with any v1 API...

**topups** — Manage topups

- `stripe-pp-pp-cli topups get` — Returns a list of top-ups.
- `stripe-pp-pp-cli topups get-topup` — Retrieves the details of a top-up that has previously been created. Supply the unique top-up ID that was returned...
- `stripe-pp-pp-cli topups post` — Top up the balance of an account
- `stripe-pp-pp-cli topups post-topup` — Updates the metadata of a top-up. Other top-up details are not editable by design.

**transfers** — Manage transfers

- `stripe-pp-pp-cli transfers get` — Returns a list of existing transfers sent to connected accounts. The transfers are returned in sorted order, with...
- `stripe-pp-pp-cli transfers get-transfer` — Retrieves the details of an existing transfer. Supply the unique transfer ID from either a transfer creation...
- `stripe-pp-pp-cli transfers post` — To send funds from your Stripe account to a connected account, you create a new transfer object. Your Updates the specified transfer by setting the values of the parameters passed. Any parameters not provided will...

**treasury** — Manage treasury

- `stripe-pp-pp-cli treasury get-credit-reversals` — Returns a list of CreditReversals.
- `stripe-pp-pp-cli treasury get-credit-reversals-credit-reversal` — Retrieves the details of an existing CreditReversal by passing the unique CreditReversal ID from either the...
- `stripe-pp-pp-cli treasury get-debit-reversals` — Returns a list of DebitReversals.
- `stripe-pp-pp-cli treasury get-debit-reversals-debit-reversal` — Retrieves a DebitReversal object.
- `stripe-pp-pp-cli treasury get-financial-accounts` — Returns a list of FinancialAccounts.
- `stripe-pp-pp-cli treasury get-financial-accounts-financial-account` — Retrieves the details of a FinancialAccount.
- `stripe-pp-pp-cli treasury get-financial-accounts-financial-account-features` — Retrieves Features information associated with the FinancialAccount.
- `stripe-pp-pp-cli treasury get-inbound-transfers` — Returns a list of InboundTransfers sent from the specified FinancialAccount.
- `stripe-pp-pp-cli treasury get-inbound-transfers-id` — Retrieves the details of an existing InboundTransfer.
- `stripe-pp-pp-cli treasury get-outbound-payments` — Returns a list of OutboundPayments sent from the specified FinancialAccount.
- `stripe-pp-pp-cli treasury get-outbound-payments-id` — Retrieves the details of an existing OutboundPayment by passing the unique OutboundPayment ID from either the...
- `stripe-pp-pp-cli treasury get-outbound-transfers` — Returns a list of OutboundTransfers sent from the specified FinancialAccount.
- `stripe-pp-pp-cli treasury get-outbound-transfers-outbound-transfer` — Retrieves the details of an existing OutboundTransfer by passing the unique OutboundTransfer ID from either the...
- `stripe-pp-pp-cli treasury get-received-credits` — Returns a list of ReceivedCredits.
- `stripe-pp-pp-cli treasury get-received-credits-id` — Retrieves the details of an existing ReceivedCredit by passing the unique ReceivedCredit ID from the...
- `stripe-pp-pp-cli treasury get-received-debits` — Returns a list of ReceivedDebits.
- `stripe-pp-pp-cli treasury get-received-debits-id` — Retrieves the details of an existing ReceivedDebit by passing the unique ReceivedDebit ID from the ReceivedDebit...
- `stripe-pp-pp-cli treasury get-transaction-entries` — Retrieves a list of TransactionEntry objects.
- `stripe-pp-pp-cli treasury get-transaction-entries-id` — Retrieves a TransactionEntry object.
- `stripe-pp-pp-cli treasury get-transactions` — Retrieves a list of Transaction objects.
- `stripe-pp-pp-cli treasury get-transactions-id` — Retrieves the details of an existing Transaction.
- `stripe-pp-pp-cli treasury post-credit-reversals` — Reverses a ReceivedCredit and creates a CreditReversal object.
- `stripe-pp-pp-cli treasury post-debit-reversals` — Reverses a ReceivedDebit and creates a DebitReversal object.
- `stripe-pp-pp-cli treasury post-financial-accounts` — Creates a new FinancialAccount. Each connected account can have up to three FinancialAccounts by default.
- `stripe-pp-pp-cli treasury post-financial-accounts-financial-account` — Updates the details of a FinancialAccount.
- `stripe-pp-pp-cli treasury post-financial-accounts-financial-account-close` — Closes a FinancialAccount. A FinancialAccount can only be closed if it has a zero balance, has no pending...
- `stripe-pp-pp-cli treasury post-financial-accounts-financial-account-features` — Updates the Features associated with a FinancialAccount.
- `stripe-pp-pp-cli treasury post-inbound-transfers` — Creates an InboundTransfer.
- `stripe-pp-pp-cli treasury post-inbound-transfers-inbound-transfer-cancel` — Cancels an InboundTransfer.
- `stripe-pp-pp-cli treasury post-outbound-payments` — Creates an OutboundPayment.
- `stripe-pp-pp-cli treasury post-outbound-payments-id-cancel` — Cancel an OutboundPayment.
- `stripe-pp-pp-cli treasury post-outbound-transfers` — Creates an OutboundTransfer.
- `stripe-pp-pp-cli treasury post-outbound-transfers-outbound-transfer-cancel` — An OutboundTransfer can be canceled if the funds have not yet been paid out.

**webhook-endpoints** — Manage webhook endpoints

- `stripe-pp-pp-cli webhook-endpoints delete` — You can also delete webhook endpoints via the webhook...
- `stripe-pp-pp-cli webhook-endpoints get` — Returns a list of your webhook endpoints.
- `stripe-pp-pp-cli webhook-endpoints get-webhookendpoints` — Retrieves the webhook endpoint with the given ID.
- `stripe-pp-pp-cli webhook-endpoints post` — A webhook endpoint must have a url and a list of enabled_events. You may optionally...
- `stripe-pp-pp-cli webhook-endpoints post-webhookendpoints` — Updates the webhook endpoint. You may edit the url, the list of enabled_events, and the...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
stripe-pp-pp-cli which ""
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily failures triage

```bash
stripe-pp-pp-cli payments failed --since 24h --group-by decline_code --json --select decline_code,count,total_amount,sample_customers
```

Run every morning to see what broke overnight, grouped by reason with dollars at risk.

### Churn report for the week

```bash
stripe-pp-pp-cli subscriptions churn --since 7d --group-by cancellation_reason --mrr-delta --json
```

Group canceled subscriptions by reason with MRR impact attached — the report Stripe Sigma charges extra for.

### Payout reconciliation

```bash
stripe-pp-pp-cli payouts explain po_1NXabc --json --select amount,arrival_date,breakdown,top_customers
```

Decompose a payout into balance_transactions with per-customer attribution. Replaces the CSV-export-to-spreadsheet workflow.

### Agent-narrow JSON for deeply nested customer record

```bash
stripe-pp-pp-cli customers profile cus_1234 --json --select id,email,lifetime_value,active_subscriptions.id,active_subscriptions.status,open_invoices.amount_due
```

Customer 360 returns a deeply nested object; dotted `--select` paths narrow to exactly the fields the agent needs so it doesn't burn context on full responses.

### Local FTS across entities

```bash
stripe-pp-pp-cli search "acme.co" --type customers --json --select entity,id,snippet,score
```

Find every customer, invoice, or charge mentioning a domain or company name — instantly, with no Search API lag.

## Auth Setup

Set STRIPE_SECRET_KEY in your environment. Test-mode keys (sk_test_*) are recommended — `doctor` warns loudly on sk_live_* so you can't accidentally hit production. Auth is Bearer, idempotency keys are auto-injected on every POST.

Run `stripe-pp-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

 ```bash
 stripe-pp-pp-cli account --agent --select id,name,status
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
 "results": 
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
stripe-pp-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
stripe-pp-pp-cli feedback --stdin `. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:` | Atomically write output to `` (tmp + rename) |
| `webhook:` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
stripe-pp-pp-cli profile save briefing --json
stripe-pp-pp-cli --profile briefing account
stripe-pp-pp-cli profile list --json
stripe-pp-pp-cli profile show briefing
stripe-pp-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `stripe-pp-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
 ```bash
 go install github.com/mvanhorn/printing-press-library/library/payments/stripe-pp/cmd/stripe-pp-pp-mcp@latest
 ```
2. Register with Claude Code:
 ```bash
 claude mcp add stripe-pp-pp-mcp -- stripe-pp-pp-mcp
 ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which stripe-pp-pp-cli`
 If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
 ```bash
 stripe-pp-pp-cli [subcommand] [args] --agent
 ```
4. If ambiguous, drill into subcommand help: `stripe-pp-pp-cli --help`.