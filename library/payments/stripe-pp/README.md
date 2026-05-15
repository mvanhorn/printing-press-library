# Stripe CLI

**Every Stripe feature, plus a local SQLite mirror, FTS search, and compound-analytics commands no other Stripe tool has.**

Mirror your Stripe account into a single-file SQLite database, then ask questions across customers, payments, subscriptions, refunds, and payouts with one command. Designed for agents (JSON, --select, --csv everywhere) and for the finance/RevOps user who lives in the dashboard today.

Learn more at [Stripe](https://stripe.com).

## Install

The recommended path installs both the `stripe-pp-pp-cli` binary and the `pp-stripe-pp` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install stripe
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install stripe --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/stripe-pp/cmd/stripe-pp-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/stripe-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine `. On Unix, mark it executable: `chmod +x `.


## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-stripe-pp --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-stripe-pp --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-stripe-pp skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-stripe-pp. The skill defines how its required CLI can be installed.
```

## Authentication

Set STRIPE_SECRET_KEY in your environment. Test-mode keys (sk_test_*) are recommended — `doctor` warns loudly on sk_live_* so you can't accidentally hit production. Auth is Bearer, idempotency keys are auto-injected on every POST.

## Quick Start

```bash
# Confirm key works, API reachable, rate-limit headroom available — and surface a warning if you're on a live-mode key.
stripe-pp-pp-cli doctor


# Mirror your account into local SQLite. Resumable per-entity cursors mean you can stop and restart.
stripe-pp-pp-cli sync --since 30d --resources customers,charges,invoices,subscriptions,balance_transactions,payouts


# Returns the full customer record — subscriptions, lifetime charges/refunds, open invoices, LTV — in one query.
stripe-pp-pp-cli customers profile cus_1234 --json


# Morning triage: what's failing and why, grouped instantly with no Search API lag.
stripe-pp-pp-cli payments failed --since 2026-05-01 --group-by decline_code --json --select decline_code,count,total_amount


# Decompose a payout into every underlying balance_transaction with per-customer attribution.
stripe-pp-pp-cli payouts explain po_1NXabc --json

```

## Unique Features

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

## Usage

Run `stripe-pp-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Manage account

- **`stripe-pp-pp-cli account get`** - Retrieves the details of an account.

### account-links

Manage account links

- **`stripe-pp-pp-cli account-links post`** - Creates an AccountLink object that includes a single-use Stripe URL that the platform can redirect their user to in order to take them through the Connect Onboarding flow.

### account-sessions

Manage account sessions

- **`stripe-pp-pp-cli account-sessions post`** - Creates a AccountSession object that includes a single-use token that the platform can use on their front-end to grant client-side API access.

### accounts

Manage accounts

- **`stripe-pp-pp-cli accounts delete`** - With Connect, you can delete accounts you manage.

Test-mode accounts can be deleted at any time.

Live-mode accounts that have access to the standard dashboard and Stripe is responsible for negative account balances cannot be deleted, which includes Standard accounts. All other Live-mode accounts, can be deleted when all balances are zero.

If you want to delete your own account, use the account information tab in your account settings instead.
- **`stripe-pp-pp-cli accounts get`** - Returns a list of accounts connected to your platform via Connect. If you’re not a platform, the list is empty.
- **`stripe-pp-pp-cli accounts get-account`** - Retrieves the details of an account.
- **`stripe-pp-pp-cli accounts post`** - With Connect, you can create Stripe accounts for your users.
To do this, you’ll first need to register your platform.

If you’ve already collected information for your connected accounts, you can prefill that information when
creating the account. Connect Onboarding won’t ask for the prefilled information during account onboarding.
You can prefill any information on the account.
- **`stripe-pp-pp-cli accounts post-account`** - Updates a connected account by setting the values of the parameters passed. Any parameters not provided are
left unchanged.

For accounts where controller.requirement_collection
is application, which includes Custom accounts, you can update any information on the account.

For accounts where controller.requirement_collection
is stripe, which includes Standard and Express accounts, you can update all information until you create
an Account Link or Account Session to start Connect onboarding,
after which some properties can no longer be updated.

To update your own account, use the Dashboard. Refer to our
Connect documentation to learn more about updating accounts.

### apple-pay

Manage apple pay

- **`stripe-pp-pp-cli apple-pay delete-domains-domain`** - Delete an apple pay domain.
- **`stripe-pp-pp-cli apple-pay get-domains`** - List apple pay domains.
- **`stripe-pp-pp-cli apple-pay get-domains-domain`** - Retrieve an apple pay domain.
- **`stripe-pp-pp-cli apple-pay post-domains`** - Create an apple pay domain.

### application-fees

Manage application fees

- **`stripe-pp-pp-cli application-fees get`** - Returns a list of application fees you’ve previously collected. The application fees are returned in sorted order, with the most recent fees appearing first.
- **`stripe-pp-pp-cli application-fees get-id`** - Retrieves the details of an application fee that your account has collected. The same information is returned when refunding the application fee.

### apps

Manage apps

- **`stripe-pp-pp-cli apps get-secrets`** - List all secrets stored on the given scope.
- **`stripe-pp-pp-cli apps get-secrets-find`** - Finds a secret in the secret store by name and scope.
- **`stripe-pp-pp-cli apps post-secrets`** - Create or replace a secret in the secret store.
- **`stripe-pp-pp-cli apps post-secrets-delete`** - Deletes a secret from the secret store by name and scope.

### balance

Manage balance

- **`stripe-pp-pp-cli balance get`** - Retrieves the current account balance, based on the authentication that was used to make the request.
 For a sample request, see Accounting for negative balances.
- **`stripe-pp-pp-cli balance get-history`** - Returns a list of transactions that have contributed to the Stripe account balance (e.g., charges, transfers, and so forth). The transactions are returned in sorted order, with the most recent transactions appearing first.

Note that this endpoint was previously called “Balance history” and used the path /v1/balance/history.
- **`stripe-pp-pp-cli balance get-history-id`** - Retrieves the balance transaction with the given ID.

Note that this endpoint previously used the path /v1/balance/history/:id.

### balance-settings

Manage balance settings

- **`stripe-pp-pp-cli balance-settings get`** - Retrieves balance settings for a given connected account.
 Related guide: Making API calls for connected accounts
- **`stripe-pp-pp-cli balance-settings post`** - Updates balance settings for a given connected account.
 Related guide: Making API calls for connected accounts

### balance-transactions

Manage balance transactions

- **`stripe-pp-pp-cli balance-transactions get`** - Returns a list of transactions that have contributed to the Stripe account balance (e.g., charges, transfers, and so forth). The transactions are returned in sorted order, with the most recent transactions appearing first.

Note that this endpoint was previously called “Balance history” and used the path /v1/balance/history.
- **`stripe-pp-pp-cli balance-transactions get-id`** - Retrieves the balance transaction with the given ID.

Note that this endpoint previously used the path /v1/balance/history/:id.

### billing

Manage billing

- **`stripe-pp-pp-cli billing get-alerts`** - Lists billing active and inactive alerts
- **`stripe-pp-pp-cli billing get-alerts-id`** - Retrieves a billing alert given an ID
- **`stripe-pp-pp-cli billing get-credit-balance-summary`** - Retrieves the credit balance summary for a customer.
- **`stripe-pp-pp-cli billing get-credit-balance-transactions`** - Retrieve a list of credit balance transactions.
- **`stripe-pp-pp-cli billing get-credit-balance-transactions-id`** - Retrieves a credit balance transaction.
- **`stripe-pp-pp-cli billing get-credit-grants`** - Retrieve a list of credit grants.
- **`stripe-pp-pp-cli billing get-credit-grants-id`** - Retrieves a credit grant.
- **`stripe-pp-pp-cli billing get-meters`** - Retrieve a list of billing meters.
- **`stripe-pp-pp-cli billing get-meters-id`** - Retrieves a billing meter given an ID.
- **`stripe-pp-pp-cli billing get-meters-id-event-summaries`** - Retrieve a list of billing meter event summaries.
- **`stripe-pp-pp-cli billing post-alerts`** - Creates a billing alert
- **`stripe-pp-pp-cli billing post-alerts-id-activate`** - Reactivates this alert, allowing it to trigger again.
- **`stripe-pp-pp-cli billing post-alerts-id-archive`** - Archives this alert, removing it from the list view and APIs. This is non-reversible.
- **`stripe-pp-pp-cli billing post-alerts-id-deactivate`** - Deactivates this alert, preventing it from triggering.
- **`stripe-pp-pp-cli billing post-credit-grants`** - Creates a credit grant.
- **`stripe-pp-pp-cli billing post-credit-grants-id`** - Updates a credit grant.
- **`stripe-pp-pp-cli billing post-credit-grants-id-expire`** - Expires a credit grant.
- **`stripe-pp-pp-cli billing post-credit-grants-id-void`** - Voids a credit grant.
- **`stripe-pp-pp-cli billing post-meter-event-adjustments`** - Creates a billing meter event adjustment.
- **`stripe-pp-pp-cli billing post-meter-events`** - Creates a billing meter event.
- **`stripe-pp-pp-cli billing post-meters`** - Creates a billing meter.
- **`stripe-pp-pp-cli billing post-meters-id`** - Updates a billing meter.
- **`stripe-pp-pp-cli billing post-meters-id-deactivate`** - When a meter is deactivated, no more meter events will be accepted for this meter. You can’t attach a deactivated meter to a price.
- **`stripe-pp-pp-cli billing post-meters-id-reactivate`** - When a meter is reactivated, events for this meter can be accepted and you can attach the meter to a price.

### billing-portal

Manage billing portal

- **`stripe-pp-pp-cli billing-portal get-configurations`** - Returns a list of configurations that describe the functionality of the customer portal.
- **`stripe-pp-pp-cli billing-portal get-configurations-configuration`** - Retrieves a configuration that describes the functionality of the customer portal.
- **`stripe-pp-pp-cli billing-portal post-configurations`** - Creates a configuration that describes the functionality and behavior of a PortalSession
- **`stripe-pp-pp-cli billing-portal post-configurations-configuration`** - Updates a configuration that describes the functionality of the customer portal.
- **`stripe-pp-pp-cli billing-portal post-sessions`** - Creates a session of the customer portal.

### charges

Manage charges

- **`stripe-pp-pp-cli charges get`** - Returns a list of charges you’ve previously created. The charges are returned in sorted order, with the most recent charges appearing first.
- **`stripe-pp-pp-cli charges get-charge`** - Retrieves the details of a charge that has previously been created. Supply the unique charge ID that was returned from your previous request, and Stripe will return the corresponding charge information. The same information is returned when creating or refunding the charge.
- **`stripe-pp-pp-cli charges get-search`** - Search for charges you’ve previously created using Stripe’s Search Query Language.
Don’t use search in read-after-write flows where strict consistency is necessary. Under normal operating
conditions, data is searchable in less than a minute. Occasionally, propagation of new or updated data can be up
to an hour behind during outages. Search functionality is not available to merchants in India.
- **`stripe-pp-pp-cli charges post`** - This method is no longer recommended—use the Payment Intents API
to initiate a new payment instead. Confirmation of the PaymentIntent creates the Charge
object used to request payment.
- **`stripe-pp-pp-cli charges post-charge`** - Updates the specified charge by setting the values of the parameters passed. Any parameters not provided will be left unchanged.

### checkout

Manage checkout

- **`stripe-pp-pp-cli checkout get-sessions`** - Returns a list of Checkout Sessions.
- **`stripe-pp-pp-cli checkout get-sessions-session`** - Retrieves a Checkout Session object.
- **`stripe-pp-pp-cli checkout get-sessions-session-line-items`** - When retrieving a Checkout Session, there is an includable line_items property containing the first handful of those items. There is also a URL where you can retrieve the full (paginated) list of line items.
- **`stripe-pp-pp-cli checkout post-sessions`** - Creates a Checkout Session object.
- **`stripe-pp-pp-cli checkout post-sessions-session`** - Updates a Checkout Session object.

Related guide: Dynamically update a Checkout Session
- **`stripe-pp-pp-cli checkout post-sessions-session-expire`** - A Checkout Session can be expired when it is in one of these statuses: open 

After it expires, a customer can’t complete a Checkout Session and customers loading the Checkout Session see a message saying the Checkout Session is expired.

### climate

Manage climate

- **`stripe-pp-pp-cli climate get-orders`** - Lists all Climate order objects. The orders are returned sorted by creation date, with the
most recently created orders appearing first.
- **`stripe-pp-pp-cli climate get-orders-order`** - Retrieves the details of a Climate order object with the given ID.
- **`stripe-pp-pp-cli climate get-products`** - Lists all available Climate product objects.
- **`stripe-pp-pp-cli climate get-products-product`** - Retrieves the details of a Climate product with the given ID.
- **`stripe-pp-pp-cli climate get-suppliers`** - Lists all available Climate supplier objects.
- **`stripe-pp-pp-cli climate get-suppliers-supplier`** - Retrieves a Climate supplier object.
- **`stripe-pp-pp-cli climate post-orders`** - Creates a Climate order object for a given Climate product. The order will be processed immediately
after creation and payment will be deducted your Stripe balance.
- **`stripe-pp-pp-cli climate post-orders-order`** - Updates the specified order by setting the values of the parameters passed.
- **`stripe-pp-pp-cli climate post-orders-order-cancel`** - Cancels a Climate order. You can cancel an order within 24 hours of creation. Stripe refunds the
reservation amount_subtotal, but not the amount_fees for user-triggered cancellations. Frontier
might cancel reservations if suppliers fail to deliver. If Frontier cancels the reservation, Stripe
provides 90 days advance notice and refunds the amount_total.

### confirmation-tokens

Manage confirmation tokens

- **`stripe-pp-pp-cli confirmation-tokens get`** - Retrieves an existing ConfirmationToken object

### country-specs

Manage country specs

- **`stripe-pp-pp-cli country-specs get`** - Lists all Country Spec objects available in the API.
- **`stripe-pp-pp-cli country-specs get-country`** - Returns a Country Spec for a given Country code.

### coupons

Manage coupons

- **`stripe-pp-pp-cli coupons delete`** - You can delete coupons via the coupon management page of the Stripe dashboard. However, deleting a coupon does not affect any customers who have already applied the coupon; it means that new customers can’t redeem the coupon. You can also delete coupons via the API.
- **`stripe-pp-pp-cli coupons get`** - Returns a list of your coupons.
- **`stripe-pp-pp-cli coupons get-coupon`** - Retrieves the coupon with the given ID.
- **`stripe-pp-pp-cli coupons post`** - You can create coupons easily via the coupon management page of the Stripe dashboard. Coupon creation is also accessible via the API if you need to create coupons on the fly.

A coupon has either a percent_off or an amount_off and currency. If you set an amount_off, that amount will be subtracted from any invoice’s subtotal. For example, an invoice with a subtotal of 100 will have a final total of 0 if a coupon with an amount_off of 200 is applied to it and an invoice with a subtotal of 300 will have a final total of 100 if a coupon with an amount_off of 200 is applied to it.
- **`stripe-pp-pp-cli coupons post-coupon`** - Updates the metadata of a coupon. Other coupon details (currency, duration, amount_off) are, by design, not editable.

### credit-notes

Manage credit notes

- **`stripe-pp-pp-cli credit-notes get`** - Returns a list of credit notes.
- **`stripe-pp-pp-cli credit-notes get-id`** - Retrieves the credit note object with the given identifier.
- **`stripe-pp-pp-cli credit-notes get-preview`** - Get a preview of a credit note without creating it.
- **`stripe-pp-pp-cli credit-notes get-preview-lines`** - When retrieving a credit note preview, you’ll get a lines property containing the first handful of those items. This URL you can retrieve the full (paginated) list of line items.
- **`stripe-pp-pp-cli credit-notes post`** - Issue a credit note to adjust the amount of a finalized invoice. A credit note will first reduce the invoice’s amount_remaining (and amount_due), but not below zero.
This amount is indicated by the credit note’s pre_payment_amount. The excess amount is indicated by post_payment_amount, and it can result in any combination of the following:


Refunds: create a new refund (using refund_amount) or link existing refunds (using refunds).
Customer balance credit: credit the customer’s balance (using credit_amount) which will be automatically applied to their next invoice when it’s finalized.
Outside of Stripe credit: record the amount that is or will be credited outside of Stripe (using out_of_band_amount).


The sum of refunds, customer balance credits, and outside of Stripe credits must equal the post_payment_amount.

You may issue multiple credit notes for an invoice. Each credit note may increment the invoice’s pre_payment_credit_notes_amount,
post_payment_credit_notes_amount, or both, depending on the invoice’s amount_remaining at the time of credit note creation.
- **`stripe-pp-pp-cli credit-notes post-id`** - Updates an existing credit note.

### customer-sessions

Manage customer sessions

- **`stripe-pp-pp-cli customer-sessions post`** - Creates a Customer Session object that includes a single-use client secret that you can use on your front-end to grant client-side API access for certain customer resources.

### customers

Manage customers

- **`stripe-pp-pp-cli customers delete`** - Permanently deletes a customer. It cannot be undone. Also immediately cancels any active subscriptions on the customer.
- **`stripe-pp-pp-cli customers get`** - Returns a list of your customers. The customers are returned sorted by creation date, with the most recent customers appearing first.
- **`stripe-pp-pp-cli customers get-customer`** - Retrieves a Customer object.
- **`stripe-pp-pp-cli customers get-search`** - Search for customers you’ve previously created using Stripe’s Search Query Language.
Don’t use search in read-after-write flows where strict consistency is necessary. Under normal operating
conditions, data is searchable in less than a minute. Occasionally, propagation of new or updated data can be up
to an hour behind during outages. Search functionality is not available to merchants in India.
- **`stripe-pp-pp-cli customers post`** - Creates a new customer object.
- **`stripe-pp-pp-cli customers post-customer`** - Updates the specified customer by setting the values of the parameters passed. Any parameters not provided are left unchanged. For example, if you pass the source parameter, that becomes the customer’s active source (such as a card) to be used for all charges in the future. When you update a customer to a new valid card source by passing the source parameter: for each of the customer’s current subscriptions, if the subscription bills automatically and is in the past_due state, then the latest open invoice for the subscription with automatic collection enabled is retried. This retry doesn’t count as an automatic retry, and doesn’t affect the next regularly scheduled payment for the invoice. Changing the default_source for a customer doesn’t trigger this behavior.

This request accepts mostly the same arguments as the customer creation call.

### disputes

Manage disputes

- **`stripe-pp-pp-cli disputes get`** - Returns a list of your disputes.
- **`stripe-pp-pp-cli disputes get-dispute`** - Retrieves the dispute with the given ID.
- **`stripe-pp-pp-cli disputes post`** - When you get a dispute, contacting your customer is always the best first step. If that doesn’t work, you can submit evidence to help us resolve the dispute in your favor. You can do this in your dashboard, but if you prefer, you can use the API to submit evidence programmatically.

Depending on your dispute type, different evidence fields will give you a better chance of winning your dispute. To figure out which evidence fields to provide, see our guide to dispute types.

### entitlements

Manage entitlements

- **`stripe-pp-pp-cli entitlements get-active`** - Retrieve a list of active entitlements for a customer
- **`stripe-pp-pp-cli entitlements get-active-id`** - Retrieve an active entitlement
- **`stripe-pp-pp-cli entitlements get-features`** - Retrieve a list of features
- **`stripe-pp-pp-cli entitlements get-features-id`** - Retrieves a feature
- **`stripe-pp-pp-cli entitlements post-features`** - Creates a feature
- **`stripe-pp-pp-cli entitlements post-features-id`** - Update a feature’s metadata or permanently deactivate it.

### ephemeral-keys

Manage ephemeral keys

- **`stripe-pp-pp-cli ephemeral-keys delete-key`** - Invalidates a short-lived API key for a given resource.
- **`stripe-pp-pp-cli ephemeral-keys post`** - Creates a short-lived API key for a given resource.

### events

Manage events

- **`stripe-pp-pp-cli events get`** - List events, going back up to 30 days. Each event data is rendered according to Stripe API version at its creation time, specified in event object api_version attribute (not according to your current Stripe API version or Stripe-Version header).
- **`stripe-pp-pp-cli events get-id`** - Retrieves the details of an event if it was created in the last 30 days. Supply the unique identifier of the event, which you might have received in a webhook.

### exchange-rates

Manage exchange rates

- **`stripe-pp-pp-cli exchange-rates get`** - [Deprecated] The ExchangeRate APIs are deprecated. Please use the FX Quotes API instead.

Returns a list of objects that contain the rates at which foreign currencies are converted to one another. Only shows the currencies for which Stripe supports.
- **`stripe-pp-pp-cli exchange-rates get-rate-id`** - [Deprecated] The ExchangeRate APIs are deprecated. Please use the FX Quotes API instead.

Retrieves the exchange rates from the given currency to every supported currency.

### external-accounts

Manage external accounts

- **`stripe-pp-pp-cli external-accounts post-id`** - Updates the metadata, account holder name, account holder type of a bank account belonging to
a connected account and optionally sets it as the default for its currency. Other bank account
details are not editable by design.

You can only update bank accounts when account.controller.requirement_collection is application, which includes Custom accounts.

You can re-enable a disabled bank account by performing an update call without providing any
arguments or changes.

### file-links

Manage file links

- **`stripe-pp-pp-cli file-links get`** - Returns a list of file links.
- **`stripe-pp-pp-cli file-links get-link`** - Retrieves the file link with the given ID.
- **`stripe-pp-pp-cli file-links post`** - Creates a new file link object.
- **`stripe-pp-pp-cli file-links post-link`** - Updates an existing file link object. Expired links can no longer be updated.

### files

Manage files

- **`stripe-pp-pp-cli files get`** - Returns a list of the files that your account has access to. Stripe sorts and returns the files by their creation dates, placing the most recently created files at the top.
- **`stripe-pp-pp-cli files get-file`** - Retrieves the details of an existing file object. After you supply a unique file ID, Stripe returns the corresponding file object. Learn how to access file contents.
- **`stripe-pp-pp-cli files post`** - To upload a file to Stripe, you need to send a request of type multipart/form-data. Include the file you want to upload in the request, and the parameters for creating a file.

All of Stripe’s officially supported Client libraries support sending multipart/form-data.

### financial-connections

Manage financial connections

- **`stripe-pp-pp-cli financial-connections get-accounts`** - Returns a list of Financial Connections Account objects.
- **`stripe-pp-pp-cli financial-connections get-accounts-account`** - Retrieves the details of an Financial Connections Account.
- **`stripe-pp-pp-cli financial-connections get-accounts-account-owners`** - Lists all owners for a given Account
- **`stripe-pp-pp-cli financial-connections get-sessions-session`** - Retrieves the details of a Financial Connections Session
- **`stripe-pp-pp-cli financial-connections get-transactions`** - Returns a list of Financial Connections Transaction objects.
- **`stripe-pp-pp-cli financial-connections get-transactions-transaction`** - Retrieves the details of a Financial Connections Transaction
- **`stripe-pp-pp-cli financial-connections post-accounts-account-disconnect`** - Disables your access to a Financial Connections Account. You will no longer be able to access data associated with the account (e.g. balances, transactions).
- **`stripe-pp-pp-cli financial-connections post-accounts-account-refresh`** - Refreshes the data associated with a Financial Connections Account.
- **`stripe-pp-pp-cli financial-connections post-accounts-account-subscribe`** - Subscribes to periodic refreshes of data associated with a Financial Connections Account. When the account status is active, data is typically refreshed once a day.
- **`stripe-pp-pp-cli financial-connections post-accounts-account-unsubscribe`** - Unsubscribes from periodic refreshes of data associated with a Financial Connections Account.
- **`stripe-pp-pp-cli financial-connections post-sessions`** - To launch the Financial Connections authorization flow, create a Session. The session’s client_secret can be used to launch the flow using Stripe.js.

### forwarding

Manage forwarding

- **`stripe-pp-pp-cli forwarding get-requests`** - Lists all ForwardingRequest objects.
- **`stripe-pp-pp-cli forwarding get-requests-id`** - Retrieves a ForwardingRequest object.
- **`stripe-pp-pp-cli forwarding post-requests`** - Creates a ForwardingRequest object.

### identity

Manage identity

- **`stripe-pp-pp-cli identity get-verification-reports`** - List all verification reports.
- **`stripe-pp-pp-cli identity get-verification-reports-report`** - Retrieves an existing VerificationReport
- **`stripe-pp-pp-cli identity get-verification-sessions`** - Returns a list of VerificationSessions
- **`stripe-pp-pp-cli identity get-verification-sessions-session`** - Retrieves the details of a VerificationSession that was previously created.

When the session status is requires_input, you can use this method to retrieve a valid
client_secret or url to allow re-submission.
- **`stripe-pp-pp-cli identity post-verification-sessions`** - Creates a VerificationSession object.

After the VerificationSession is created, display a verification modal using the session client_secret or send your users to the session’s url.

If your API key is in test mode, verification checks won’t actually process, though everything else will occur as if in live mode.

Related guide: Verify your users’ identity documents
- **`stripe-pp-pp-cli identity post-verification-sessions-session`** - Updates a VerificationSession object.

When the session status is requires_input, you can use this method to update the
verification check and options.
- **`stripe-pp-pp-cli identity post-verification-sessions-session-cancel`** - A VerificationSession object can be canceled when it is in requires_input status.

Once canceled, future submission attempts are disabled. This cannot be undone. Learn more.
- **`stripe-pp-pp-cli identity post-verification-sessions-session-redact`** - Redact a VerificationSession to remove all collected information from Stripe. This will redact
the VerificationSession and all objects related to it, including VerificationReports, Events,
request logs, etc.

A VerificationSession object can be redacted when it is in requires_input or verified
status. Redacting a VerificationSession in requires_action
state will automatically cancel it.

The redaction process may take up to four days. When the redaction process is in progress, the
VerificationSession’s redaction.status field will be set to processing; when the process is
finished, it will change to redacted and an identity.verification_session.redacted event
will be emitted.

Redaction is irreversible. Redacted objects are still accessible in the Stripe API, but all the
fields that contain personal data will be replaced by the string [redacted] or a similar
placeholder. The metadata field will also be erased. Redacted objects cannot be updated or
used for any purpose.

Learn more.

### invoice-payments

Manage invoice payments

- **`stripe-pp-pp-cli invoice-payments get`** - When retrieving an invoice, there is an includable payments property containing the first handful of those items. There is also a URL where you can retrieve the full (paginated) list of payments.
- **`stripe-pp-pp-cli invoice-payments get-invoicepayments`** - Retrieves the invoice payment with the given ID.

### invoice-rendering-templates

Manage invoice rendering templates

- **`stripe-pp-pp-cli invoice-rendering-templates get`** - List all templates, ordered by creation date, with the most recently created template appearing first.
- **`stripe-pp-pp-cli invoice-rendering-templates get-template`** - Retrieves an invoice rendering template with the given ID. It by default returns the latest version of the template. Optionally, specify a version to see previous versions.

### invoiceitems

Manage invoiceitems

- **`stripe-pp-pp-cli invoiceitems delete`** - Deletes an invoice item, removing it from an invoice. Deleting invoice items is only possible when they’re not attached to invoices, or if it’s attached to a draft invoice.
- **`stripe-pp-pp-cli invoiceitems get`** - Returns a list of your invoice items. Invoice items are returned sorted by creation date, with the most recently created invoice items appearing first.
- **`stripe-pp-pp-cli invoiceitems get-invoiceitem`** - Retrieves the invoice item with the given ID.
- **`stripe-pp-pp-cli invoiceitems post`** - Creates an item to be added to a draft invoice (up to 250 items per invoice). If no invoice is specified, the item will be on the next invoice created for the customer specified.
- **`stripe-pp-pp-cli invoiceitems post-invoiceitem`** - Updates the amount or description of an invoice item on an upcoming invoice. Updating an invoice item is only possible before the invoice it’s attached to is closed.

### invoices

Manage invoices

- **`stripe-pp-pp-cli invoices delete`** - Permanently deletes a one-off invoice draft. This cannot be undone. Attempts to delete invoices that are no longer in a draft state will fail; once an invoice has been finalized or if an invoice is for a subscription, it must be voided.
- **`stripe-pp-pp-cli invoices get`** - You can list all invoices, or list the invoices for a specific customer. The invoices are returned sorted by creation date, with the most recently created invoices appearing first.
- **`stripe-pp-pp-cli invoices get-invoice`** - Retrieves the invoice with the given ID.
- **`stripe-pp-pp-cli invoices get-search`** - Search for invoices you’ve previously created using Stripe’s Search Query Language.
Don’t use search in read-after-write flows where strict consistency is necessary. Under normal operating
conditions, data is searchable in less than a minute. Occasionally, propagation of new or updated data can be up
to an hour behind during outages. Search functionality is not available to merchants in India.
- **`stripe-pp-pp-cli invoices post`** - This endpoint creates a draft invoice for a given customer. The invoice remains a draft until you finalize the invoice, which allows you to pay or send the invoice to your customers.
- **`stripe-pp-pp-cli invoices post-create-preview`** - At any time, you can preview the upcoming invoice for a subscription or subscription schedule. This will show you all the charges that are pending, including subscription renewal charges, invoice item charges, etc. It will also show you any discounts that are applicable to the invoice.

You can also preview the effects of creating or updating a subscription or subscription schedule, including a preview of any prorations that will take place. To ensure that the actual proration is calculated exactly the same as the previewed proration, you should pass the subscription_details.proration_date parameter when doing the actual subscription update.

The recommended way to get only the prorations being previewed on the invoice is to consider line items where parent.subscription_item_details.proration is true.

Note that when you are viewing an upcoming invoice, you are simply viewing a preview – the invoice has not yet been created. As such, the upcoming invoice will not show up in invoice listing calls, and you cannot use the API to pay or edit the invoice. If you want to change the amount that your customer will be billed, you can add, remove, or update pending invoice items, or update the customer’s discount.

Note: Currency conversion calculations use the latest exchange rates. Exchange rates may vary between the time of the preview and the time of the actual invoice creation. Learn more
- **`stripe-pp-pp-cli invoices post-invoice`** - Draft invoices are fully editable. Once an invoice is finalized,
monetary values, as well as collection_method, become uneditable.

If you would like to stop the Stripe Billing engine from automatically finalizing, reattempting payments on,
sending reminders for, or automatically reconciling invoices, pass
auto_advance=false.

### issuing

Manage issuing

- **`stripe-pp-pp-cli issuing get-authorizations`** - Returns a list of Issuing Authorization objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.
- **`stripe-pp-pp-cli issuing get-authorizations-authorization`** - Retrieves an Issuing Authorization object.
- **`stripe-pp-pp-cli issuing get-cardholders`** - Returns a list of Issuing Cardholder objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.
- **`stripe-pp-pp-cli issuing get-cardholders-cardholder`** - Retrieves an Issuing Cardholder object.
- **`stripe-pp-pp-cli issuing get-cards`** - Returns a list of Issuing Card objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.
- **`stripe-pp-pp-cli issuing get-cards-card`** - Retrieves an Issuing Card object.
- **`stripe-pp-pp-cli issuing get-disputes`** - Returns a list of Issuing Dispute objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.
- **`stripe-pp-pp-cli issuing get-disputes-dispute`** - Retrieves an Issuing Dispute object.
- **`stripe-pp-pp-cli issuing get-personalization-designs`** - Returns a list of personalization design objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.
- **`stripe-pp-pp-cli issuing get-personalization-designs-personalization-design`** - Retrieves a personalization design object.
- **`stripe-pp-pp-cli issuing get-physical-bundles`** - Returns a list of physical bundle objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.
- **`stripe-pp-pp-cli issuing get-physical-bundles-physical-bundle`** - Retrieves a physical bundle object.
- **`stripe-pp-pp-cli issuing get-settlements-settlement`** - Retrieves an Issuing Settlement object.
- **`stripe-pp-pp-cli issuing get-tokens`** - Lists all Issuing Token objects for a given card.
- **`stripe-pp-pp-cli issuing get-tokens-token`** - Retrieves an Issuing Token object.
- **`stripe-pp-pp-cli issuing get-transactions`** - Returns a list of Issuing Transaction objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.
- **`stripe-pp-pp-cli issuing get-transactions-transaction`** - Retrieves an Issuing Transaction object.
- **`stripe-pp-pp-cli issuing post-authorizations-authorization`** - Updates the specified Issuing Authorization object by setting the values of the parameters passed. Any parameters not provided will be left unchanged.
- **`stripe-pp-pp-cli issuing post-authorizations-authorization-approve`** - [Deprecated] Approves a pending Issuing Authorization object. This request should be made within the timeout window of the real-time authorization flow. 
This method is deprecated. Instead, respond directly to the webhook request to approve an authorization.
- **`stripe-pp-pp-cli issuing post-authorizations-authorization-decline`** - [Deprecated] Declines a pending Issuing Authorization object. This request should be made within the timeout window of the real time authorization flow.
This method is deprecated. Instead, respond directly to the webhook request to decline an authorization.
- **`stripe-pp-pp-cli issuing post-cardholders`** - Creates a new Issuing Cardholder object that can be issued cards.
- **`stripe-pp-pp-cli issuing post-cardholders-cardholder`** - Updates the specified Issuing Cardholder object by setting the values of the parameters passed. Any parameters not provided will be left unchanged.
- **`stripe-pp-pp-cli issuing post-cards`** - Creates an Issuing Card object.
- **`stripe-pp-pp-cli issuing post-cards-card`** - Updates the specified Issuing Card object by setting the values of the parameters passed. Any parameters not provided will be left unchanged.
- **`stripe-pp-pp-cli issuing post-disputes`** - Creates an Issuing Dispute object. Individual pieces of evidence within the evidence object are optional at this point. Stripe only validates that required evidence is present during submission. Refer to Dispute reasons and evidence for more details about evidence requirements.
- **`stripe-pp-pp-cli issuing post-disputes-dispute`** - Updates the specified Issuing Dispute object by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Properties on the evidence object can be unset by passing in an empty string.
- **`stripe-pp-pp-cli issuing post-disputes-dispute-submit`** - Submits an Issuing Dispute to the card network. Stripe validates that all evidence fields required for the dispute’s reason are present. For more details, see Dispute reasons and evidence.
- **`stripe-pp-pp-cli issuing post-personalization-designs`** - Creates a personalization design object.
- **`stripe-pp-pp-cli issuing post-personalization-designs-personalization-design`** - Updates a card personalization object.
- **`stripe-pp-pp-cli issuing post-settlements-settlement`** - Updates the specified Issuing Settlement object by setting the values of the parameters passed. Any parameters not provided will be left unchanged.
- **`stripe-pp-pp-cli issuing post-tokens-token`** - Attempts to update the specified Issuing Token object to the status specified.
- **`stripe-pp-pp-cli issuing post-transactions-transaction`** - Updates the specified Issuing Transaction object by setting the values of the parameters passed. Any parameters not provided will be left unchanged.

### link-account-sessions

Manage link account sessions

- **`stripe-pp-pp-cli link-account-sessions get-session`** - Retrieves the details of a Financial Connections Session
- **`stripe-pp-pp-cli link-account-sessions post`** - To launch the Financial Connections authorization flow, create a Session. The session’s client_secret can be used to launch the flow using Stripe.js.

### linked-accounts

Manage linked accounts

- **`stripe-pp-pp-cli linked-accounts get`** - Returns a list of Financial Connections Account objects.
- **`stripe-pp-pp-cli linked-accounts get-account`** - Retrieves the details of an Financial Connections Account.

### mandates

Manage mandates

- **`stripe-pp-pp-cli mandates get`** - Retrieves a Mandate object.

### payment-attempt-records

Manage payment attempt records

- **`stripe-pp-pp-cli payment-attempt-records get`** - List all the Payment Attempt Records attached to the specified Payment Record.
- **`stripe-pp-pp-cli payment-attempt-records get-id`** - Retrieves a Payment Attempt Record with the given ID

### payment-intents

Manage payment intents

- **`stripe-pp-pp-cli payment-intents get`** - Returns a list of PaymentIntents.
- **`stripe-pp-pp-cli payment-intents get-intent`** - Retrieves the details of a PaymentIntent that has previously been created. 

You can retrieve a PaymentIntent client-side using a publishable key when the client_secret is in the query string. 

If you retrieve a PaymentIntent with a publishable key, it only returns a subset of properties. Refer to the payment intent object reference for more details.
- **`stripe-pp-pp-cli payment-intents get-search`** - Search for PaymentIntents you’ve previously created using Stripe’s Search Query Language.
Don’t use search in read-after-write flows where strict consistency is necessary. Under normal operating
conditions, data is searchable in less than a minute. Occasionally, propagation of new or updated data can be up
to an hour behind during outages. Search functionality is not available to merchants in India.
- **`stripe-pp-pp-cli payment-intents post`** - Creates a PaymentIntent object.

After the PaymentIntent is created, attach a payment method and confirm
to continue the payment. Learn more about the available payment flows
with the Payment Intents API.

When you use confirm=true during creation, it’s equivalent to creating
and confirming the PaymentIntent in the same call. You can use any parameters
available in the confirm API when you supply
confirm=true.
- **`stripe-pp-pp-cli payment-intents post-intent`** - Updates properties on a PaymentIntent object without confirming.

Depending on which properties you update, you might need to confirm the
PaymentIntent again. For example, updating the payment_method
always requires you to confirm the PaymentIntent again. If you prefer to
update and confirm at the same time, we recommend updating properties through
the confirm API instead.

### payment-links

Manage payment links

- **`stripe-pp-pp-cli payment-links get`** - Returns a list of your payment links.
- **`stripe-pp-pp-cli payment-links get-paymentlinks`** - Retrieve a payment link.
- **`stripe-pp-pp-cli payment-links post`** - Creates a payment link.
- **`stripe-pp-pp-cli payment-links post-paymentlinks`** - Updates a payment link.

### payment-method-configurations

Manage payment method configurations

- **`stripe-pp-pp-cli payment-method-configurations get`** - List payment method configurations
- **`stripe-pp-pp-cli payment-method-configurations get-configuration`** - Retrieve payment method configuration
- **`stripe-pp-pp-cli payment-method-configurations post`** - Creates a payment method configuration
- **`stripe-pp-pp-cli payment-method-configurations post-configuration`** - Update payment method configuration

### payment-method-domains

Manage payment method domains

- **`stripe-pp-pp-cli payment-method-domains get`** - Lists the details of existing payment method domains.
- **`stripe-pp-pp-cli payment-method-domains get-paymentmethoddomains`** - Retrieves the details of an existing payment method domain.
- **`stripe-pp-pp-cli payment-method-domains post`** - Creates a payment method domain.
- **`stripe-pp-pp-cli payment-method-domains post-paymentmethoddomains`** - Updates an existing payment method domain.

### payment-methods

Manage payment methods

- **`stripe-pp-pp-cli payment-methods get`** - Returns a list of all PaymentMethods.
- **`stripe-pp-pp-cli payment-methods get-paymentmethods`** - Retrieves a PaymentMethod object attached to the StripeAccount. To retrieve a payment method attached to a Customer, you should use Retrieve a Customer’s PaymentMethods
- **`stripe-pp-pp-cli payment-methods post`** - Creates a PaymentMethod object. Read the Stripe.js reference to learn how to create PaymentMethods via Stripe.js.

Instead of creating a PaymentMethod directly, we recommend using the PaymentIntents API to accept a payment immediately or the SetupIntent API to collect payment method details ahead of a future payment.
- **`stripe-pp-pp-cli payment-methods post-paymentmethods`** - Updates a PaymentMethod object. A PaymentMethod must be attached to a customer to be updated.

### payment-records

Manage payment records

- **`stripe-pp-pp-cli payment-records get-id`** - Retrieves a Payment Record with the given ID
- **`stripe-pp-pp-cli payment-records post-report-payment`** - Report a new Payment Record. You may report a Payment Record as it is
 initialized and later report updates through the other report_* methods, or report Payment
 Records in a terminal state directly, through this method.

### payouts

Manage payouts

- **`stripe-pp-pp-cli payouts get`** - Returns a list of existing payouts sent to third-party bank accounts or payouts that Stripe sent to you. The payouts return in sorted order, with the most recently created payouts appearing first.
- **`stripe-pp-pp-cli payouts get-payout`** - Retrieves the details of an existing payout. Supply the unique payout ID from either a payout creation request or the payout list. Stripe returns the corresponding payout information.
- **`stripe-pp-pp-cli payouts post`** - To send funds to your own bank account, create a new payout object. Your Stripe balance must cover the payout amount. If it doesn’t, you receive an “Insufficient Funds” error.

If your API key is in test mode, money won’t actually be sent, though every other action occurs as if you’re in live mode.

If you create a manual payout on a Stripe account that uses multiple payment source types, you need to specify the source type balance that the payout draws from. The balance object details available and pending amounts by source type.
- **`stripe-pp-pp-cli payouts post-payout`** - Updates the specified payout by setting the values of the parameters you pass. We don’t change parameters that you don’t provide. This request only accepts the metadata as arguments.

### plans

Manage plans

- **`stripe-pp-pp-cli plans delete`** - Deleting plans means new subscribers can’t be added. Existing subscribers aren’t affected.
- **`stripe-pp-pp-cli plans get`** - Returns a list of your plans.
- **`stripe-pp-pp-cli plans get-plan`** - Retrieves the plan with the given ID.
- **`stripe-pp-pp-cli plans post`** - You can now model subscriptions more flexibly using the Prices API. It replaces the Plans API and is backwards compatible to simplify your migration.
- **`stripe-pp-pp-cli plans post-plan`** - Updates the specified plan by setting the values of the parameters passed. Any parameters not provided are left unchanged. By design, you cannot change a plan’s ID, amount, currency, or billing cycle.

### prices

Manage prices

- **`stripe-pp-pp-cli prices get`** - Returns a list of your active prices, excluding inline prices. For the list of inactive prices, set active to false.
- **`stripe-pp-pp-cli prices get-price`** - Retrieves the price with the given ID.
- **`stripe-pp-pp-cli prices get-search`** - Search for prices you’ve previously created using Stripe’s Search Query Language.
Don’t use search in read-after-write flows where strict consistency is necessary. Under normal operating
conditions, data is searchable in less than a minute. Occasionally, propagation of new or updated data can be up
to an hour behind during outages. Search functionality is not available to merchants in India.
- **`stripe-pp-pp-cli prices post`** - Creates a new Price for an existing Product. The Price can be recurring or one-time.
- **`stripe-pp-pp-cli prices post-price`** - Updates the specified price by setting the values of the parameters passed. Any parameters not provided are left unchanged.

### products

Manage products

- **`stripe-pp-pp-cli products delete-id`** - Delete a product. Deleting a product is only possible if it has no prices associated with it. Additionally, deleting a product with type=good is only possible if it has no SKUs associated with it.
- **`stripe-pp-pp-cli products get`** - Returns a list of your products. The products are returned sorted by creation date, with the most recently created products appearing first.
- **`stripe-pp-pp-cli products get-id`** - Retrieves the details of an existing product. Supply the unique product ID from either a product creation request or the product list, and Stripe will return the corresponding product information.
- **`stripe-pp-pp-cli products get-search`** - Search for products you’ve previously created using Stripe’s Search Query Language.
Don’t use search in read-after-write flows where strict consistency is necessary. Under normal operating
conditions, data is searchable in less than a minute. Occasionally, propagation of new or updated data can be up
to an hour behind during outages. Search functionality is not available to merchants in India.
- **`stripe-pp-pp-cli products post`** - Creates a new product object.
- **`stripe-pp-pp-cli products post-id`** - Updates the specific product by setting the values of the parameters passed. Any parameters not provided will be left unchanged.

### promotion-codes

Manage promotion codes

- **`stripe-pp-pp-cli promotion-codes get`** - Returns a list of your promotion codes.
- **`stripe-pp-pp-cli promotion-codes get-promotioncodes`** - Retrieves the promotion code with the given ID. In order to retrieve a promotion code by the customer-facing code use list with the desired code.
- **`stripe-pp-pp-cli promotion-codes post`** - A promotion code points to an underlying promotion. You can optionally restrict the code to a specific customer, redemption limit, and expiration date.
- **`stripe-pp-pp-cli promotion-codes post-promotioncodes`** - Updates the specified promotion code by setting the values of the parameters passed. Most fields are, by design, not editable.

### quotes

Manage quotes

- **`stripe-pp-pp-cli quotes get`** - Returns a list of your quotes.
- **`stripe-pp-pp-cli quotes get-quote`** - Retrieves the quote with the given ID.
- **`stripe-pp-pp-cli quotes post`** - A quote models prices and services for a customer. Default options for header, description, footer, and expires_at can be set in the dashboard via the quote template.
- **`stripe-pp-pp-cli quotes post-quote`** - A quote models prices and services for a customer.

### radar

Manage radar

- **`stripe-pp-pp-cli radar delete-value-list-items-item`** - Deletes a ValueListItem object, removing it from its parent value list.
- **`stripe-pp-pp-cli radar delete-value-lists-value-list`** - Deletes a ValueList object, also deleting any items contained within the value list. To be deleted, a value list must not be referenced in any rules.
- **`stripe-pp-pp-cli radar get-early-fraud-warnings`** - Returns a list of early fraud warnings.
- **`stripe-pp-pp-cli radar get-early-fraud-warnings-early-fraud-warning`** - Retrieves the details of an early fraud warning that has previously been created. 

Please refer to the early fraud warning object reference for more details.
- **`stripe-pp-pp-cli radar get-value-list-items`** - Returns a list of ValueListItem objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.
- **`stripe-pp-pp-cli radar get-value-list-items-item`** - Retrieves a ValueListItem object.
- **`stripe-pp-pp-cli radar get-value-lists`** - Returns a list of ValueList objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.
- **`stripe-pp-pp-cli radar get-value-lists-value-list`** - Retrieves a ValueList object.
- **`stripe-pp-pp-cli radar post-payment-evaluations`** - Request a Radar API fraud risk score from Stripe for a payment before sending it for external processor authorization.
- **`stripe-pp-pp-cli radar post-value-list-items`** - Creates a new ValueListItem object, which is added to the specified parent value list.
- **`stripe-pp-pp-cli radar post-value-lists`** - Creates a new ValueList object, which can then be referenced in rules.
- **`stripe-pp-pp-cli radar post-value-lists-value-list`** - Updates a ValueList object by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Note that item_type is immutable.

### refunds

Manage refunds

- **`stripe-pp-pp-cli refunds get`** - Returns a list of all refunds you created. We return the refunds in sorted order, with the most recent refunds appearing first. The 10 most recent refunds are always available by default on the Charge object.
- **`stripe-pp-pp-cli refunds get-refund`** - Retrieves the details of an existing refund.
- **`stripe-pp-pp-cli refunds post`** - When you create a new refund, you must specify a Charge or a PaymentIntent object on which to create it.

Creating a new refund will refund a charge that has previously been created but not yet refunded.
Funds will be refunded to the credit or debit card that was originally charged.

You can optionally refund only part of a charge.
You can do so multiple times, until the entire charge has been refunded.

Once entirely refunded, a charge can’t be refunded again.
This method will raise an error when called on an already-refunded charge,
or when trying to refund more money than is left on a charge.
- **`stripe-pp-pp-cli refunds post-refund`** - Updates the refund that you specify by setting the values of the passed parameters. Any parameters that you don’t provide remain unchanged.

This request only accepts metadata as an argument.

### reporting

Manage reporting

- **`stripe-pp-pp-cli reporting get-report-runs`** - Returns a list of Report Runs, with the most recent appearing first.
- **`stripe-pp-pp-cli reporting get-report-runs-report-run`** - Retrieves the details of an existing Report Run.
- **`stripe-pp-pp-cli reporting get-report-types`** - Returns a full list of Report Types.
- **`stripe-pp-pp-cli reporting get-report-types-report-type`** - Retrieves the details of a Report Type. (Certain report types require a live-mode API key.)
- **`stripe-pp-pp-cli reporting post-report-runs`** - Creates a new object and begin running the report. (Certain report types require a live-mode API key.)

### reviews

Manage reviews

- **`stripe-pp-pp-cli reviews get`** - Returns a list of Review objects that have open set to true. The objects are sorted in descending order by creation date, with the most recently created object appearing first.
- **`stripe-pp-pp-cli reviews get-review`** - Retrieves a Review object.

### setup-attempts

Manage setup attempts

- **`stripe-pp-pp-cli setup-attempts get`** - Returns a list of SetupAttempts that associate with a provided SetupIntent.

### setup-intents

Manage setup intents

- **`stripe-pp-pp-cli setup-intents get`** - Returns a list of SetupIntents.
- **`stripe-pp-pp-cli setup-intents get-intent`** - Retrieves the details of a SetupIntent that has previously been created. 

Client-side retrieval using a publishable key is allowed when the client_secret is provided in the query string. 

When retrieved with a publishable key, only a subset of properties will be returned. Please refer to the SetupIntent object reference for more details.
- **`stripe-pp-pp-cli setup-intents post`** - Creates a SetupIntent object.

After you create the SetupIntent, attach a payment method and confirm
it to collect any required permissions to charge the payment method later.
- **`stripe-pp-pp-cli setup-intents post-intent`** - Updates a SetupIntent object.

### shipping-rates

Manage shipping rates

- **`stripe-pp-pp-cli shipping-rates get`** - Returns a list of your shipping rates.
- **`stripe-pp-pp-cli shipping-rates get-token`** - Returns the shipping rate object with the given ID.
- **`stripe-pp-pp-cli shipping-rates post`** - Creates a new shipping rate object.
- **`stripe-pp-pp-cli shipping-rates post-token`** - Updates an existing shipping rate object.

### sigma

Manage sigma

- **`stripe-pp-pp-cli sigma get-scheduled-query-runs`** - Returns a list of scheduled query runs.
- **`stripe-pp-pp-cli sigma get-scheduled-query-runs-scheduled-query-run`** - Retrieves the details of an scheduled query run.
- **`stripe-pp-pp-cli sigma post-saved-queries-id`** - Update an existing Sigma query that previously exists

### sources

Manage sources

- **`stripe-pp-pp-cli sources get`** - Retrieves an existing source object. Supply the unique source ID from a source creation request and Stripe will return the corresponding up-to-date source object information.
- **`stripe-pp-pp-cli sources post`** - Creates a new source object.
- **`stripe-pp-pp-cli sources post-source`** - Updates the specified source by setting the values of the parameters passed. Any parameters not provided will be left unchanged.

This request accepts the metadata and owner as arguments. It is also possible to update type specific information for selected payment methods. Please refer to our payment method guides for more detail.

### subscription-items

Manage subscription items

- **`stripe-pp-pp-cli subscription-items delete-item`** - Deletes an item from the subscription. Removing a subscription item from a subscription will not cancel the subscription.
- **`stripe-pp-pp-cli subscription-items get`** - Returns a list of your subscription items for a given subscription.
- **`stripe-pp-pp-cli subscription-items get-item`** - Retrieves the subscription item with the given ID.
- **`stripe-pp-pp-cli subscription-items post`** - Adds a new item to an existing subscription. No existing items will be changed or replaced.
- **`stripe-pp-pp-cli subscription-items post-item`** - Updates the plan or quantity of an item on a current subscription.

### subscription-schedules

Manage subscription schedules

- **`stripe-pp-pp-cli subscription-schedules get`** - Retrieves the list of your subscription schedules.
- **`stripe-pp-pp-cli subscription-schedules get-schedule`** - Retrieves the details of an existing subscription schedule. You only need to supply the unique subscription schedule identifier that was returned upon subscription schedule creation.
- **`stripe-pp-pp-cli subscription-schedules post`** - Creates a new subscription schedule object. Each customer can have up to 500 active or scheduled subscriptions.
- **`stripe-pp-pp-cli subscription-schedules post-schedule`** - Updates an existing subscription schedule.

### subscriptions

Manage subscriptions

- **`stripe-pp-pp-cli subscriptions delete-exposed-id`** - Cancels a customer’s subscription immediately. The customer won’t be charged again for the subscription. After it’s canceled, you can no longer update the subscription or its metadata.

Any pending invoice items that you’ve created are still charged at the end of the period, unless manually deleted. If you’ve set the subscription to cancel at the end of the period, any pending prorations are also left in place and collected at the end of the period. But if the subscription is set to cancel immediately, pending prorations are removed if invoice_now and prorate are both set to true.

By default, upon subscription cancellation, Stripe stops automatic collection of all finalized invoices for the customer. This is intended to prevent unexpected payment attempts after the customer has canceled a subscription. However, you can resume automatic collection of the invoices manually after subscription cancellation to have us proceed. Or, you could check for unpaid invoices before allowing the customer to cancel the subscription at all.
- **`stripe-pp-pp-cli subscriptions get`** - By default, returns a list of subscriptions that have not been canceled. In order to list canceled subscriptions, specify status=canceled.
- **`stripe-pp-pp-cli subscriptions get-exposed-id`** - Retrieves the subscription with the given ID.
- **`stripe-pp-pp-cli subscriptions get-search`** - Search for subscriptions you’ve previously created using Stripe’s Search Query Language.
Don’t use search in read-after-write flows where strict consistency is necessary. Under normal operating
conditions, data is searchable in less than a minute. Occasionally, propagation of new or updated data can be up
to an hour behind during outages. Search functionality is not available to merchants in India.
- **`stripe-pp-pp-cli subscriptions post`** - Creates a new subscription on an existing customer. Each customer can have up to 500 active or scheduled subscriptions.

When you create a subscription with collection_method=charge_automatically, the first invoice is finalized as part of the request.
The payment_behavior parameter determines the exact behavior of the initial payment.

To start subscriptions where the first invoice always begins in a draft status, use subscription schedules instead.
Schedules provide the flexibility to model more complex billing configurations that change over time.
- **`stripe-pp-pp-cli subscriptions post-exposed-id`** - Updates an existing subscription to match the specified parameters.
When changing prices or quantities, we optionally prorate the price we charge next month to make up for any price changes.
To preview how the proration is calculated, use the create preview endpoint.

By default, we prorate subscription changes. For example, if a customer signs up on May 1 for a 100 price, they’ll be billed 100 immediately. If on May 15 they switch to a 200 price, then on June 1 they’ll be billed 250 (200 for a renewal of her subscription, plus a 50 prorating adjustment for half of the previous month’s 100 difference). Similarly, a downgrade generates a credit that is applied to the next invoice. We also prorate when you make quantity changes.

Switching prices does not normally change the billing date or generate an immediate charge unless:


The billing interval is changed (for example, from monthly to yearly).
The subscription moves from free to paid.
A trial starts or ends.


In these cases, we apply a credit for the unused time on the previous price, immediately charge the customer using the new price, and reset the billing date. Learn about how Stripe immediately attempts payment for subscription changes.

If you want to charge for an upgrade immediately, pass proration_behavior as always_invoice to create prorations, automatically invoice the customer for those proration adjustments, and attempt to collect payment. If you pass create_prorations, the prorations are created but not automatically invoiced. If you want to bill the customer for the prorations before the subscription’s renewal date, you need to manually invoice the customer.

If you don’t want to prorate, set the proration_behavior option to none. With this option, the customer is billed 100 on May 1 and 200 on June 1. Similarly, if you set proration_behavior to none when switching between different billing intervals (for example, from monthly to yearly), we don’t generate any credits for the old subscription’s unused time. We still reset the billing date and bill immediately for the new subscription.

Updating the quantity on a subscription many times in an hour may result in rate limiting. If you need to bill for a frequently changing quantity, consider integrating usage-based billing instead.

### tax

Manage tax

- **`stripe-pp-pp-cli tax get-associations-find`** - Finds a tax association object by PaymentIntent id.
- **`stripe-pp-pp-cli tax get-calculations-calculation`** - Retrieves a Tax Calculation object, if the calculation hasn’t expired.
- **`stripe-pp-pp-cli tax get-calculations-calculation-line-items`** - Retrieves the line items of a tax calculation as a collection, if the calculation hasn’t expired.
- **`stripe-pp-pp-cli tax get-registrations`** - Returns a list of Tax Registration objects.
- **`stripe-pp-pp-cli tax get-registrations-id`** - Returns a Tax Registration object.
- **`stripe-pp-pp-cli tax get-settings`** - Retrieves Tax Settings for a merchant.
- **`stripe-pp-pp-cli tax get-transactions-transaction`** - Retrieves a Tax Transaction object.
- **`stripe-pp-pp-cli tax get-transactions-transaction-line-items`** - Retrieves the line items of a committed standalone transaction as a collection.
- **`stripe-pp-pp-cli tax post-calculations`** - Calculates tax based on the input and returns a Tax Calculation object.
- **`stripe-pp-pp-cli tax post-registrations`** - Creates a new Tax Registration object.
- **`stripe-pp-pp-cli tax post-registrations-id`** - Updates an existing Tax Registration object.

A registration cannot be deleted after it has been created. If you wish to end a registration you may do so by setting expires_at.
- **`stripe-pp-pp-cli tax post-settings`** - Updates Tax Settings parameters used in tax calculations. All parameters are editable but none can be removed once set.
- **`stripe-pp-pp-cli tax post-transactions-create-from-calculation`** - Creates a Tax Transaction from a calculation, if that calculation hasn’t expired. Calculations expire after 90 days.
- **`stripe-pp-pp-cli tax post-transactions-create-reversal`** - Partially or fully reverses a previously created Transaction.

### tax-codes

Manage tax codes

- **`stripe-pp-pp-cli tax-codes get`** - A list of all tax codes available to add to Products in order to allow specific tax calculations.
- **`stripe-pp-pp-cli tax-codes get-id`** - Retrieves the details of an existing tax code. Supply the unique tax code ID and Stripe will return the corresponding tax code information.

### tax-ids

Manage tax ids

- **`stripe-pp-pp-cli tax-ids delete-id`** - Deletes an existing account or customer tax_id object.
- **`stripe-pp-pp-cli tax-ids get`** - Returns a list of tax IDs.
- **`stripe-pp-pp-cli tax-ids get-id`** - Retrieves an account or customer tax_id object.
- **`stripe-pp-pp-cli tax-ids post`** - Creates a new account or customer tax_id object.

### tax-rates

Manage tax rates

- **`stripe-pp-pp-cli tax-rates get`** - Returns a list of your tax rates. Tax rates are returned sorted by creation date, with the most recently created tax rates appearing first.
- **`stripe-pp-pp-cli tax-rates get-taxrates`** - Retrieves a tax rate with the given ID
- **`stripe-pp-pp-cli tax-rates post`** - Creates a new tax rate.
- **`stripe-pp-pp-cli tax-rates post-taxrates`** - Updates an existing tax rate.

### terminal

Manage terminal

- **`stripe-pp-pp-cli terminal delete-configurations-configuration`** - Deletes a Configuration object.
- **`stripe-pp-pp-cli terminal delete-locations-location`** - Deletes a Location object.
- **`stripe-pp-pp-cli terminal delete-readers-reader`** - Deletes a Reader object.
- **`stripe-pp-pp-cli terminal get-configurations`** - Returns a list of Configuration objects.
- **`stripe-pp-pp-cli terminal get-configurations-configuration`** - Retrieves a Configuration object.
- **`stripe-pp-pp-cli terminal get-locations`** - Returns a list of Location objects.
- **`stripe-pp-pp-cli terminal get-locations-location`** - Retrieves a Location object.
- **`stripe-pp-pp-cli terminal get-readers`** - Returns a list of Reader objects.
- **`stripe-pp-pp-cli terminal get-readers-reader`** - Retrieves a Reader object.
- **`stripe-pp-pp-cli terminal post-configurations`** - Creates a new Configuration object.
- **`stripe-pp-pp-cli terminal post-configurations-configuration`** - Updates a new Configuration object.
- **`stripe-pp-pp-cli terminal post-connection-tokens`** - To connect to a reader the Stripe Terminal SDK needs to retrieve a short-lived connection token from Stripe, proxied through your server. On your backend, add an endpoint that creates and returns a connection token.
- **`stripe-pp-pp-cli terminal post-locations`** - Creates a new Location object.
For further details, including which address fields are required in each country, see the Manage locations guide.
- **`stripe-pp-pp-cli terminal post-locations-location`** - Updates a Location object by setting the values of the parameters passed. Any parameters not provided will be left unchanged.
- **`stripe-pp-pp-cli terminal post-onboarding-links`** - Creates a new OnboardingLink object that contains a redirect_url used for onboarding onto Tap to Pay on iPhone.
- **`stripe-pp-pp-cli terminal post-readers`** - Creates a new Reader object.
- **`stripe-pp-pp-cli terminal post-readers-reader`** - Updates a Reader object by setting the values of the parameters passed. Any parameters not provided will be left unchanged.
- **`stripe-pp-pp-cli terminal post-readers-reader-cancel-action`** - Cancels the current reader action. See Programmatic Cancellation for more details.
- **`stripe-pp-pp-cli terminal post-readers-reader-collect-inputs`** - Initiates an input collection flow on a Reader to display input forms and collect information from your customers.
- **`stripe-pp-pp-cli terminal post-readers-reader-collect-payment-method`** - Initiates a payment flow on a Reader and updates the PaymentIntent with card details before manual confirmation. See Collecting a Payment method for more details.
- **`stripe-pp-pp-cli terminal post-readers-reader-confirm-payment-intent`** - Finalizes a payment on a Reader. See Confirming a Payment for more details.
- **`stripe-pp-pp-cli terminal post-readers-reader-process-payment-intent`** - Initiates a payment flow on a Reader. See process the payment for more details.
- **`stripe-pp-pp-cli terminal post-readers-reader-process-setup-intent`** - Initiates a SetupIntent flow on a Reader. See Save directly without charging for more details.
- **`stripe-pp-pp-cli terminal post-readers-reader-refund-payment`** - Initiates an in-person refund on a Reader. See Refund an Interac Payment for more details.
- **`stripe-pp-pp-cli terminal post-readers-reader-set-reader-display`** - Sets the reader display to show cart details.
- **`stripe-pp-pp-cli terminal post-refunds`** - Internal endpoint for terminal use to create a refund for a card_present charge.
This endpoint only supports card_present payment method types (excludes interac_present).

You can optionally refund only part of a charge.

### test-helpers

Manage test helpers

- **`stripe-pp-pp-cli test-helpers delete-test-clocks-test-clock`** - Deletes a test clock.
- **`stripe-pp-pp-cli test-helpers get-test-clocks`** - Returns a list of your test clocks.
- **`stripe-pp-pp-cli test-helpers get-test-clocks-test-clock`** - Retrieves a test clock.
- **`stripe-pp-pp-cli test-helpers post-confirmation-tokens`** - Creates a test mode Confirmation Token server side for your integration tests.
- **`stripe-pp-pp-cli test-helpers post-customers-customer-fund-cash-balance`** - Create an incoming testmode bank transfer
- **`stripe-pp-pp-cli test-helpers post-issuing-authorizations`** - Create a test-mode authorization.
- **`stripe-pp-pp-cli test-helpers post-issuing-authorizations-authorization-capture`** - Capture a test-mode authorization.
- **`stripe-pp-pp-cli test-helpers post-issuing-authorizations-authorization-expire`** - Expire a test-mode Authorization.
- **`stripe-pp-pp-cli test-helpers post-issuing-authorizations-authorization-finalize-amount`** - Finalize the amount on an Authorization prior to capture, when the initial authorization was for an estimated amount.
- **`stripe-pp-pp-cli test-helpers post-issuing-authorizations-authorization-fraud-challenges-respond`** - Respond to a fraud challenge on a testmode Issuing authorization, simulating either a confirmation of fraud or a correction of legitimacy.
- **`stripe-pp-pp-cli test-helpers post-issuing-authorizations-authorization-increment`** - Increment a test-mode Authorization.
- **`stripe-pp-pp-cli test-helpers post-issuing-authorizations-authorization-reverse`** - Reverse a test-mode Authorization.
- **`stripe-pp-pp-cli test-helpers post-issuing-cards-card-shipping-deliver`** - Updates the shipping status of the specified Issuing Card object to delivered.
- **`stripe-pp-pp-cli test-helpers post-issuing-cards-card-shipping-fail`** - Updates the shipping status of the specified Issuing Card object to failure.
- **`stripe-pp-pp-cli test-helpers post-issuing-cards-card-shipping-return`** - Updates the shipping status of the specified Issuing Card object to returned.
- **`stripe-pp-pp-cli test-helpers post-issuing-cards-card-shipping-ship`** - Updates the shipping status of the specified Issuing Card object to shipped.
- **`stripe-pp-pp-cli test-helpers post-issuing-cards-card-shipping-submit`** - Updates the shipping status of the specified Issuing Card object to submitted. This method requires Stripe Version ‘2024-09-30.acacia’ or later.
- **`stripe-pp-pp-cli test-helpers post-issuing-personalization-designs-personalization-design-activate`** - Updates the status of the specified testmode personalization design object to active.
- **`stripe-pp-pp-cli test-helpers post-issuing-personalization-designs-personalization-design-deactivate`** - Updates the status of the specified testmode personalization design object to inactive.
- **`stripe-pp-pp-cli test-helpers post-issuing-personalization-designs-personalization-design-reject`** - Updates the status of the specified testmode personalization design object to rejected.
- **`stripe-pp-pp-cli test-helpers post-issuing-settlements`** - Allows the user to create an Issuing settlement.
- **`stripe-pp-pp-cli test-helpers post-issuing-settlements-settlement-complete`** - Allows the user to mark an Issuing settlement as complete.
- **`stripe-pp-pp-cli test-helpers post-issuing-transactions-create-force-capture`** - Allows the user to capture an arbitrary amount, also known as a forced capture.
- **`stripe-pp-pp-cli test-helpers post-issuing-transactions-create-unlinked-refund`** - Allows the user to refund an arbitrary amount, also known as a unlinked refund.
- **`stripe-pp-pp-cli test-helpers post-issuing-transactions-transaction-refund`** - Refund a test-mode Transaction.
- **`stripe-pp-pp-cli test-helpers post-refunds-refund-expire`** - Expire a refund with a status of requires_action.
- **`stripe-pp-pp-cli test-helpers post-terminal-readers-reader-present-payment-method`** - Presents a payment method on a simulated reader. Can be used to simulate accepting a payment, saving a card or refunding a transaction.
- **`stripe-pp-pp-cli test-helpers post-terminal-readers-reader-succeed-input-collection`** - Use this endpoint to trigger a successful input collection on a simulated reader.
- **`stripe-pp-pp-cli test-helpers post-terminal-readers-reader-timeout-input-collection`** - Use this endpoint to complete an input collection with a timeout error on a simulated reader.
- **`stripe-pp-pp-cli test-helpers post-test-clocks`** - Creates a new test clock that can be attached to new customers and quotes.
- **`stripe-pp-pp-cli test-helpers post-test-clocks-test-clock-advance`** - Starts advancing a test clock to a specified time in the future. Advancement is done when status changes to Ready.
- **`stripe-pp-pp-cli test-helpers post-treasury-inbound-transfers-id-fail`** - Transitions a test mode created InboundTransfer to the failed status. The InboundTransfer must already be in the processing state.
- **`stripe-pp-pp-cli test-helpers post-treasury-inbound-transfers-id-return`** - Marks the test mode InboundTransfer object as returned and links the InboundTransfer to a ReceivedDebit. The InboundTransfer must already be in the succeeded state.
- **`stripe-pp-pp-cli test-helpers post-treasury-inbound-transfers-id-succeed`** - Transitions a test mode created InboundTransfer to the succeeded status. The InboundTransfer must already be in the processing state.
- **`stripe-pp-pp-cli test-helpers post-treasury-outbound-payments-id`** - Updates a test mode created OutboundPayment with tracking details. The OutboundPayment must not be cancelable, and cannot be in the canceled or failed states.
- **`stripe-pp-pp-cli test-helpers post-treasury-outbound-payments-id-fail`** - Transitions a test mode created OutboundPayment to the failed status. The OutboundPayment must already be in the processing state.
- **`stripe-pp-pp-cli test-helpers post-treasury-outbound-payments-id-post`** - Transitions a test mode created OutboundPayment to the posted status. The OutboundPayment must already be in the processing state.
- **`stripe-pp-pp-cli test-helpers post-treasury-outbound-payments-id-return`** - Transitions a test mode created OutboundPayment to the returned status. The OutboundPayment must already be in the processing state.
- **`stripe-pp-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer`** - Updates a test mode created OutboundTransfer with tracking details. The OutboundTransfer must not be cancelable, and cannot be in the canceled or failed states.
- **`stripe-pp-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer-fail`** - Transitions a test mode created OutboundTransfer to the failed status. The OutboundTransfer must already be in the processing state.
- **`stripe-pp-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer-post`** - Transitions a test mode created OutboundTransfer to the posted status. The OutboundTransfer must already be in the processing state.
- **`stripe-pp-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer-return`** - Transitions a test mode created OutboundTransfer to the returned status. The OutboundTransfer must already be in the processing state.
- **`stripe-pp-pp-cli test-helpers post-treasury-received-credits`** - Use this endpoint to simulate a test mode ReceivedCredit initiated by a third party. In live mode, you can’t directly create ReceivedCredits initiated by third parties.
- **`stripe-pp-pp-cli test-helpers post-treasury-received-debits`** - Use this endpoint to simulate a test mode ReceivedDebit initiated by a third party. In live mode, you can’t directly create ReceivedDebits initiated by third parties.

### tokens

Manage tokens

- **`stripe-pp-pp-cli tokens get`** - Retrieves the token with the given ID.
- **`stripe-pp-pp-cli tokens post`** - Creates a single-use token that represents a bank account’s details.
You can use this token with any v1 API method in place of a bank account dictionary. You can only use this token once. To do so, attach it to a connected account where controller.requirement_collection is application, which includes Custom accounts.

### topups

Manage topups

- **`stripe-pp-pp-cli topups get`** - Returns a list of top-ups.
- **`stripe-pp-pp-cli topups get-topup`** - Retrieves the details of a top-up that has previously been created. Supply the unique top-up ID that was returned from your previous request, and Stripe will return the corresponding top-up information.
- **`stripe-pp-pp-cli topups post`** - Top up the balance of an account
- **`stripe-pp-pp-cli topups post-topup`** - Updates the metadata of a top-up. Other top-up details are not editable by design.

### transfers

Manage transfers

- **`stripe-pp-pp-cli transfers get`** - Returns a list of existing transfers sent to connected accounts. The transfers are returned in sorted order, with the most recently created transfers appearing first.
- **`stripe-pp-pp-cli transfers get-transfer`** - Retrieves the details of an existing transfer. Supply the unique transfer ID from either a transfer creation request or the transfer list, and Stripe will return the corresponding transfer information.
- **`stripe-pp-pp-cli transfers post`** - To send funds from your Stripe account to a connected account, you create a new transfer object. Your Stripe balance must be able to cover the transfer amount, or you’ll receive an “Insufficient Funds” error.
- **`stripe-pp-pp-cli transfers post-transfer`** - Updates the specified transfer by setting the values of the parameters passed. Any parameters not provided will be left unchanged.

This request accepts only metadata as an argument.

### treasury

Manage treasury

- **`stripe-pp-pp-cli treasury get-credit-reversals`** - Returns a list of CreditReversals.
- **`stripe-pp-pp-cli treasury get-credit-reversals-credit-reversal`** - Retrieves the details of an existing CreditReversal by passing the unique CreditReversal ID from either the CreditReversal creation request or CreditReversal list
- **`stripe-pp-pp-cli treasury get-debit-reversals`** - Returns a list of DebitReversals.
- **`stripe-pp-pp-cli treasury get-debit-reversals-debit-reversal`** - Retrieves a DebitReversal object.
- **`stripe-pp-pp-cli treasury get-financial-accounts`** - Returns a list of FinancialAccounts.
- **`stripe-pp-pp-cli treasury get-financial-accounts-financial-account`** - Retrieves the details of a FinancialAccount.
- **`stripe-pp-pp-cli treasury get-financial-accounts-financial-account-features`** - Retrieves Features information associated with the FinancialAccount.
- **`stripe-pp-pp-cli treasury get-inbound-transfers`** - Returns a list of InboundTransfers sent from the specified FinancialAccount.
- **`stripe-pp-pp-cli treasury get-inbound-transfers-id`** - Retrieves the details of an existing InboundTransfer.
- **`stripe-pp-pp-cli treasury get-outbound-payments`** - Returns a list of OutboundPayments sent from the specified FinancialAccount.
- **`stripe-pp-pp-cli treasury get-outbound-payments-id`** - Retrieves the details of an existing OutboundPayment by passing the unique OutboundPayment ID from either the OutboundPayment creation request or OutboundPayment list.
- **`stripe-pp-pp-cli treasury get-outbound-transfers`** - Returns a list of OutboundTransfers sent from the specified FinancialAccount.
- **`stripe-pp-pp-cli treasury get-outbound-transfers-outbound-transfer`** - Retrieves the details of an existing OutboundTransfer by passing the unique OutboundTransfer ID from either the OutboundTransfer creation request or OutboundTransfer list.
- **`stripe-pp-pp-cli treasury get-received-credits`** - Returns a list of ReceivedCredits.
- **`stripe-pp-pp-cli treasury get-received-credits-id`** - Retrieves the details of an existing ReceivedCredit by passing the unique ReceivedCredit ID from the ReceivedCredit list.
- **`stripe-pp-pp-cli treasury get-received-debits`** - Returns a list of ReceivedDebits.
- **`stripe-pp-pp-cli treasury get-received-debits-id`** - Retrieves the details of an existing ReceivedDebit by passing the unique ReceivedDebit ID from the ReceivedDebit list
- **`stripe-pp-pp-cli treasury get-transaction-entries`** - Retrieves a list of TransactionEntry objects.
- **`stripe-pp-pp-cli treasury get-transaction-entries-id`** - Retrieves a TransactionEntry object.
- **`stripe-pp-pp-cli treasury get-transactions`** - Retrieves a list of Transaction objects.
- **`stripe-pp-pp-cli treasury get-transactions-id`** - Retrieves the details of an existing Transaction.
- **`stripe-pp-pp-cli treasury post-credit-reversals`** - Reverses a ReceivedCredit and creates a CreditReversal object.
- **`stripe-pp-pp-cli treasury post-debit-reversals`** - Reverses a ReceivedDebit and creates a DebitReversal object.
- **`stripe-pp-pp-cli treasury post-financial-accounts`** - Creates a new FinancialAccount. Each connected account can have up to three FinancialAccounts by default.
- **`stripe-pp-pp-cli treasury post-financial-accounts-financial-account`** - Updates the details of a FinancialAccount.
- **`stripe-pp-pp-cli treasury post-financial-accounts-financial-account-close`** - Closes a FinancialAccount. A FinancialAccount can only be closed if it has a zero balance, has no pending InboundTransfers, and has canceled all attached Issuing cards.
- **`stripe-pp-pp-cli treasury post-financial-accounts-financial-account-features`** - Updates the Features associated with a FinancialAccount.
- **`stripe-pp-pp-cli treasury post-inbound-transfers`** - Creates an InboundTransfer.
- **`stripe-pp-pp-cli treasury post-inbound-transfers-inbound-transfer-cancel`** - Cancels an InboundTransfer.
- **`stripe-pp-pp-cli treasury post-outbound-payments`** - Creates an OutboundPayment.
- **`stripe-pp-pp-cli treasury post-outbound-payments-id-cancel`** - Cancel an OutboundPayment.
- **`stripe-pp-pp-cli treasury post-outbound-transfers`** - Creates an OutboundTransfer.
- **`stripe-pp-pp-cli treasury post-outbound-transfers-outbound-transfer-cancel`** - An OutboundTransfer can be canceled if the funds have not yet been paid out.

### webhook-endpoints

Manage webhook endpoints

- **`stripe-pp-pp-cli webhook-endpoints delete`** - You can also delete webhook endpoints via the webhook endpoint management page of the Stripe dashboard.
- **`stripe-pp-pp-cli webhook-endpoints get`** - Returns a list of your webhook endpoints.
- **`stripe-pp-pp-cli webhook-endpoints get-webhookendpoints`** - Retrieves the webhook endpoint with the given ID.
- **`stripe-pp-pp-cli webhook-endpoints post`** - A webhook endpoint must have a url and a list of enabled_events. You may optionally specify the Boolean connect parameter. If set to true, then a Connect webhook endpoint that notifies the specified url about events from all connected accounts is created; otherwise an account webhook endpoint that notifies the specified url only about events from your account is created. You can also create webhook endpoints in the webhooks settings section of the Dashboard.
- **`stripe-pp-pp-cli webhook-endpoints post-webhookendpoints`** - Updates the webhook endpoint. You may edit the url, the list of enabled_events, and the status of your endpoint.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
stripe-pp-pp-cli account

# JSON for scripting and agents
stripe-pp-pp-cli account --json

# Filter to specific fields
stripe-pp-pp-cli account --json --select id,name,status

# Dry run — show the request without sending
stripe-pp-pp-cli account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
stripe-pp-pp-cli account --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-stripe-pp -g
```

Then invoke `/pp-stripe-pp ` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.


Use as an MCP server in Claude Code (advanced)

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/stripe-pp/cmd/stripe-pp-pp-mcp@latest
```

Then register it:

```bash
claude mcp add stripe stripe-pp-pp-mcp -e STRIPE_BEARER_AUTH=
```



## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/stripe-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `STRIPE_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.


Manual JSON config (advanced)

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/stripe-pp/cmd/stripe-pp-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
 "mcpServers": {
 "stripe": {
 "command": "stripe-pp-pp-mcp",
 "env": {
 "STRIPE_BEARER_AUTH": ""
 }
 }
 }
}
```



## Health Check

```bash
stripe-pp-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/stripe-pp-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `STRIPE_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `stripe-pp-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $STRIPE_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor says 'sk_live_ detected'** — Set STRIPE_SECRET_KEY to a test key (sk_test_*) from https://dashboard.stripe.com/test/apikeys. Live keys still work but every write prints a confirmation.
- **429 Too Many Requests** — Built-in exponential backoff handles spikes; for sync runs add `--rate-limit 10` to slow the request rate (default is 25/s test, 100/s live).
- **sync says 'cursor exhausted'** — Re-run sync with the same --resources; the cursor resumes from the last successful page. Use --reset to start over.
- **customers profile returns 'no local mirror'** — Run `stripe-pp-pp-cli sync --resources customers,charges,subscriptions,invoices` first — novel analytics commands require the mirror.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**stripe-node**](https://github.com/stripe/stripe-node) — JavaScript (3800 stars)
- [**stripe-cli**](https://github.com/stripe/stripe-cli) — Go (3500 stars)
- [**stripe-go**](https://github.com/stripe/stripe-go) — Go (2400 stars)
- [**stripe-python**](https://github.com/stripe/stripe-python) — Python (1700 stars)
- [**dj-stripe**](https://github.com/dj-stripe/dj-stripe) — Python (1700 stars)
- [**agent-toolkit (@stripe/mcp)**](https://github.com/stripe/agent-toolkit) — TypeScript (600 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)