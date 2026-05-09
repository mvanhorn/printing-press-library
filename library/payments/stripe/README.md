# Stripe CLI

**Every Stripe feature, plus a local SQLite mirror with FTS, cross-entity SQL, and analytics no other Stripe tool ships.**

stripe-pp-cli matches the official stripe-cli verb-for-verb and adds what it deliberately omits: a local mirror of customers, subscriptions, invoices, charges, payouts, and events with FTS5 search; cross-entity SQL queries (`sql`); and dossier commands (`health`, `dunning-queue`, `customer-360`, `subs-at-risk`, `payout-reconcile`). Built for agents — every command is one-shot, one-shot, and emits structured output.

Learn more at [Stripe](https://stripe.com).

## Install

The recommended path installs both the `stripe-pp-cli` binary and the `pp-stripe` agent skill in one shot:

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
go install github.com/mvanhorn/printing-press-library/library/payments/stripe/cmd/stripe-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/stripe-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-stripe --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-stripe --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-stripe skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-stripe. The skill defines how its required CLI can be installed.
```

## Authentication

Authenticate by exporting `STRIPE_SECRET_KEY=sk_test_...` (recommended) or running `stripe-pp-cli auth set-token <key>` to persist it. Test-mode keys (`sk_test_...`) and live-mode keys (`sk_live_...`) are accepted. Mutating commands against a live key are blocked by default; pass `--confirm-live` (or set `STRIPE_CONFIRM_LIVE=1`) once you have audited the invocation.

## Known Gaps (v1)

These are deferred to v0.2:

- **Stripe Issuing full workflow** — endpoint passthrough only; spending controls / dispute evidence not built
- **Stripe Terminal** — endpoint passthrough only; in-person SDK pairing out of scope
- **Stripe Tax registration** — `tax-rates` CRUD only; jurisdiction registration not built
- **Connect account fan-out** — single-account at a time; multi-account loops via external script
- **localstripe mock server** — out of scope; use [stripe-mock](https://github.com/stripe/stripe-mock) via `STRIPE_BASE_URL`

## Quick Start

```bash
# Persist your test-mode key (or just `export STRIPE_SECRET_KEY=sk_test_...`)
stripe-pp-cli auth set-token sk_test_<your-key>


# Pull customers, subscriptions, invoices, charges, payouts, and events into local SQLite
stripe-pp-cli sync --since 30d


# Score every customer; surface the worst 20 — feed into outreach segments
stripe-pp-cli health --all --limit 20 --json


# Surface the failed-payment retry queue ranked by days-overdue
stripe-pp-cli dunning-queue --owner billing@yourcompany.com --json


# Full customer dossier in one shot — replaces 6+ dashboard clicks
stripe-pp-cli customer-360 alice@example.com --compact

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`health`** — Compute a 0-100 health score for one or many customers from local SQLite, factoring in failed-payment count, dispute history, MRR contribution, subscription status, and account age.

  _Use this to feed customer-health signals into outreach, dunning, and renewal automation without burning hundreds of API calls per customer._

  ```bash
  stripe-pp-cli health cus_xyz --json --select score,failed_payments,mrr_contribution
  ```
- **`dunning-queue`** — List invoices in past_due/uncollectible state ranked by days-overdue, with last failure reason and customer email pulled from the local store.

  _When deciding which customers to follow up with about failed payments, this beats walking the Stripe dashboard or chaining 4+ API calls per row._

  ```bash
  stripe-pp-cli dunning-queue --json --select invoice,customer_email,days_overdue,last_failure_reason
  ```
- **`sql`** — Run read-only SQLite queries against the local Stripe mirror. All synced data lives in a generic 'resources' table — query with json_extract(data, '$.field') and filter by resource_type.

  _When a one-off question doesn't fit the canned commands, drop into SQL instead of writing a script that hits the API a thousand times._

  ```bash
  stripe-pp-cli sql "SELECT json_extract(data,'$.email') AS email, COUNT(*) AS active_subs FROM resources WHERE resource_type='subscriptions' AND json_extract(data,'$.status')='active' GROUP BY json_extract(data,'$.customer') HAVING active_subs > 1" --json
  ```
- **`payout-reconcile`** — Join payouts to balance_transactions to charges to customers from local store; flag missing balance_transactions and mismatches; CSV export.

  _Use this for end-of-period payout reconciliation against bank statements or accounting systems — replaces the manual CSV-pivot ritual._

  ```bash
  stripe-pp-cli payout-reconcile --since 7d --csv
  ```
- **`subs-at-risk`** — List subscriptions whose default payment method's card expires in a window, sorted by current MRR contribution.

  _Use this monthly to proactively email customers whose card is about to expire — prevents involuntary churn._

  ```bash
  stripe-pp-cli subs-at-risk --within 30d --json --select customer_email,mrr,card_exp
  ```
- **`metadata-grep`** — Search every synced resource's metadata bag for a key/value across resource types; returns (resource_type, id, matched_kv) rows.

  _When integrating Stripe with a CRM via metadata tags, this finds every related resource in one query._

  ```bash
  stripe-pp-cli metadata-grep 'crm_id=acme-001' --json
  ```

### Agent-native plumbing
- **`customer-360`** — One-shot dossier: customer profile + active subs + recent invoices + payment methods + recent charges + open disputes + lifetime spend.

  _Use this as the first command when investigating a customer ticket, support escalation, or churn risk — full context in one call._

  ```bash
  stripe-pp-cli customer-360 alice@example.com --json --compact
  ```
- **`events-since`** — Fetch all Events since a cursor, persist new cursor in profile, no daemon. One-shot replacement for stripe-cli's listen daemon.

  _Use this in cron-driven agent loops to replay only new events since the last run — no daemon to babysit, no missed events._

  ```bash
  stripe-pp-cli events-since --type 'invoice.*' --json --select id,type,created,data.object.id
  ```

## Usage

Run `stripe-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Manage account

- **`stripe-pp-cli account get`** - <p>Retrieves the details of an account.</p>

### account-links

Manage account links

- **`stripe-pp-cli account-links post`** - <p>Creates an AccountLink object that includes a single-use Stripe URL that the platform can redirect their user to in order to take them through the Connect Onboarding flow.</p>

### account-sessions

Manage account sessions

- **`stripe-pp-cli account-sessions post`** - <p>Creates a AccountSession object that includes a single-use token that the platform can use on their front-end to grant client-side API access.</p>

### accounts

Manage accounts

- **`stripe-pp-cli accounts delete`** - <p>With <a href="/connect">Connect</a>, you can delete accounts you manage.</p>

<p>Test-mode accounts can be deleted at any time.</p>

<p>Live-mode accounts that have access to the standard dashboard and Stripe is responsible for negative account balances cannot be deleted, which includes Standard accounts. All other Live-mode accounts, can be deleted when all <a href="/api/balance/balance_object">balances</a> are zero.</p>

<p>If you want to delete your own account, use the <a href="https://dashboard.stripe.com/settings/account">account information tab in your account settings</a> instead.</p>
- **`stripe-pp-cli accounts get`** - <p>Returns a list of accounts connected to your platform via <a href="/docs/connect">Connect</a>. If you’re not a platform, the list is empty.</p>
- **`stripe-pp-cli accounts get-account`** - <p>Retrieves the details of an account.</p>
- **`stripe-pp-cli accounts post`** - <p>With <a href="/docs/connect">Connect</a>, you can create Stripe accounts for your users.
To do this, you’ll first need to <a href="https://dashboard.stripe.com/account/applications/settings">register your platform</a>.</p>

<p>If you’ve already collected information for your connected accounts, you <a href="/docs/connect/best-practices#onboarding">can prefill that information</a> when
creating the account. Connect Onboarding won’t ask for the prefilled information during account onboarding.
You can prefill any information on the account.</p>
- **`stripe-pp-cli accounts post-account`** - <p>Updates a <a href="/connect/accounts">connected account</a> by setting the values of the parameters passed. Any parameters not provided are
left unchanged.</p>

<p>For accounts where <a href="/api/accounts/object#account_object-controller-requirement_collection">controller.requirement_collection</a>
is <code>application</code>, which includes Custom accounts, you can update any information on the account.</p>

<p>For accounts where <a href="/api/accounts/object#account_object-controller-requirement_collection">controller.requirement_collection</a>
is <code>stripe</code>, which includes Standard and Express accounts, you can update all information until you create
an <a href="/api/account_links">Account Link</a> or <a href="/api/account_sessions">Account Session</a> to start Connect onboarding,
after which some properties can no longer be updated.</p>

<p>To update your own account, use the <a href="https://dashboard.stripe.com/settings/account">Dashboard</a>. Refer to our
<a href="/docs/connect/updating-accounts">Connect</a> documentation to learn more about updating accounts.</p>

### apple-pay

Manage apple pay

- **`stripe-pp-cli apple-pay delete-domains-domain`** - <p>Delete an apple pay domain.</p>
- **`stripe-pp-cli apple-pay get-domains`** - <p>List apple pay domains.</p>
- **`stripe-pp-cli apple-pay get-domains-domain`** - <p>Retrieve an apple pay domain.</p>
- **`stripe-pp-cli apple-pay post-domains`** - <p>Create an apple pay domain.</p>

### application-fees

Manage application fees

- **`stripe-pp-cli application-fees get`** - <p>Returns a list of application fees you’ve previously collected. The application fees are returned in sorted order, with the most recent fees appearing first.</p>
- **`stripe-pp-cli application-fees get-id`** - <p>Retrieves the details of an application fee that your account has collected. The same information is returned when refunding the application fee.</p>

### apps

Manage apps

- **`stripe-pp-cli apps get-secrets`** - <p>List all secrets stored on the given scope.</p>
- **`stripe-pp-cli apps get-secrets-find`** - <p>Finds a secret in the secret store by name and scope.</p>
- **`stripe-pp-cli apps post-secrets`** - <p>Create or replace a secret in the secret store.</p>
- **`stripe-pp-cli apps post-secrets-delete`** - <p>Deletes a secret from the secret store by name and scope.</p>

### balance

Manage balance

- **`stripe-pp-cli balance get`** - <p>Retrieves the current account balance, based on the authentication that was used to make the request.
 For a sample request, see <a href="/docs/connect/account-balances#accounting-for-negative-balances">Accounting for negative balances</a>.</p>

### balance-settings

Manage balance settings

- **`stripe-pp-cli balance-settings get`** - <p>Retrieves balance settings for a given connected account.
 Related guide: <a href="/connect/authentication">Making API calls for connected accounts</a></p>
- **`stripe-pp-cli balance-settings post`** - <p>Updates balance settings for a given connected account.
 Related guide: <a href="/connect/authentication">Making API calls for connected accounts</a></p>

### balance-transactions

Manage balance transactions

- **`stripe-pp-cli balance-transactions get`** - <p>Returns a list of transactions that have contributed to the Stripe account balance (e.g., charges, transfers, and so forth). The transactions are returned in sorted order, with the most recent transactions appearing first.</p>

<p>Note that this endpoint was previously called “Balance history” and used the path <code>/v1/balance/history</code>.</p>
- **`stripe-pp-cli balance-transactions get-id`** - <p>Retrieves the balance transaction with the given ID.</p>

<p>Note that this endpoint previously used the path <code>/v1/balance/history/:id</code>.</p>

### billing

Manage billing

- **`stripe-pp-cli billing get-alerts`** - <p>Lists billing active and inactive alerts</p>
- **`stripe-pp-cli billing get-alerts-id`** - <p>Retrieves a billing alert given an ID</p>
- **`stripe-pp-cli billing get-credit-balance-summary`** - <p>Retrieves the credit balance summary for a customer.</p>
- **`stripe-pp-cli billing get-credit-balance-transactions`** - <p>Retrieve a list of credit balance transactions.</p>
- **`stripe-pp-cli billing get-credit-balance-transactions-id`** - <p>Retrieves a credit balance transaction.</p>
- **`stripe-pp-cli billing get-credit-grants`** - <p>Retrieve a list of credit grants.</p>
- **`stripe-pp-cli billing get-credit-grants-id`** - <p>Retrieves a credit grant.</p>
- **`stripe-pp-cli billing get-meters`** - <p>Retrieve a list of billing meters.</p>
- **`stripe-pp-cli billing get-meters-id`** - <p>Retrieves a billing meter given an ID.</p>
- **`stripe-pp-cli billing get-meters-id-event-summaries`** - <p>Retrieve a list of billing meter event summaries.</p>
- **`stripe-pp-cli billing post-alerts`** - <p>Creates a billing alert</p>
- **`stripe-pp-cli billing post-alerts-id-activate`** - <p>Reactivates this alert, allowing it to trigger again.</p>
- **`stripe-pp-cli billing post-alerts-id-archive`** - <p>Archives this alert, removing it from the list view and APIs. This is non-reversible.</p>
- **`stripe-pp-cli billing post-alerts-id-deactivate`** - <p>Deactivates this alert, preventing it from triggering.</p>
- **`stripe-pp-cli billing post-credit-grants`** - <p>Creates a credit grant.</p>
- **`stripe-pp-cli billing post-credit-grants-id`** - <p>Updates a credit grant.</p>
- **`stripe-pp-cli billing post-credit-grants-id-expire`** - <p>Expires a credit grant.</p>
- **`stripe-pp-cli billing post-credit-grants-id-void`** - <p>Voids a credit grant.</p>
- **`stripe-pp-cli billing post-meter-event-adjustments`** - <p>Creates a billing meter event adjustment.</p>
- **`stripe-pp-cli billing post-meter-events`** - <p>Creates a billing meter event.</p>
- **`stripe-pp-cli billing post-meters`** - <p>Creates a billing meter.</p>
- **`stripe-pp-cli billing post-meters-id`** - <p>Updates a billing meter.</p>
- **`stripe-pp-cli billing post-meters-id-deactivate`** - <p>When a meter is deactivated, no more meter events will be accepted for this meter. You can’t attach a deactivated meter to a price.</p>
- **`stripe-pp-cli billing post-meters-id-reactivate`** - <p>When a meter is reactivated, events for this meter can be accepted and you can attach the meter to a price.</p>

### billing-portal

Manage billing portal

- **`stripe-pp-cli billing-portal get-configurations`** - <p>Returns a list of configurations that describe the functionality of the customer portal.</p>
- **`stripe-pp-cli billing-portal get-configurations-configuration`** - <p>Retrieves a configuration that describes the functionality of the customer portal.</p>
- **`stripe-pp-cli billing-portal post-configurations`** - <p>Creates a configuration that describes the functionality and behavior of a PortalSession</p>
- **`stripe-pp-cli billing-portal post-configurations-configuration`** - <p>Updates a configuration that describes the functionality of the customer portal.</p>
- **`stripe-pp-cli billing-portal post-sessions`** - <p>Creates a session of the customer portal.</p>

### charges

Manage charges

- **`stripe-pp-cli charges get`** - <p>Returns a list of charges you’ve previously created. The charges are returned in sorted order, with the most recent charges appearing first.</p>
- **`stripe-pp-cli charges get-charge`** - <p>Retrieves the details of a charge that has previously been created. Supply the unique charge ID that was returned from your previous request, and Stripe will return the corresponding charge information. The same information is returned when creating or refunding the charge.</p>
- **`stripe-pp-cli charges get-search`** - <p>Search for charges you’ve previously created using Stripe’s <a href="/docs/search#search-query-language">Search Query Language</a>.
Don’t use search in read-after-write flows where strict consistency is necessary. Under normal operating
conditions, data is searchable in less than a minute. Occasionally, propagation of new or updated data can be up
to an hour behind during outages. Search functionality is not available to merchants in India.</p>
- **`stripe-pp-cli charges post`** - <p>This method is no longer recommended—use the <a href="/docs/api/payment_intents">Payment Intents API</a>
to initiate a new payment instead. Confirmation of the PaymentIntent creates the <code>Charge</code>
object used to request payment.</p>
- **`stripe-pp-cli charges post-charge`** - <p>Updates the specified charge by setting the values of the parameters passed. Any parameters not provided will be left unchanged.</p>

### checkout

Manage checkout

- **`stripe-pp-cli checkout get-sessions`** - <p>Returns a list of Checkout Sessions.</p>
- **`stripe-pp-cli checkout get-sessions-session`** - <p>Retrieves a Checkout Session object.</p>
- **`stripe-pp-cli checkout get-sessions-session-line-items`** - <p>When retrieving a Checkout Session, there is an includable <strong>line_items</strong> property containing the first handful of those items. There is also a URL where you can retrieve the full (paginated) list of line items.</p>
- **`stripe-pp-cli checkout post-sessions`** - <p>Creates a Checkout Session object.</p>
- **`stripe-pp-cli checkout post-sessions-session`** - <p>Updates a Checkout Session object.</p>

<p>Related guide: <a href="/payments/advanced/dynamic-updates">Dynamically update a Checkout Session</a></p>
- **`stripe-pp-cli checkout post-sessions-session-expire`** - <p>A Checkout Session can be expired when it is in one of these statuses: <code>open</code> </p>

<p>After it expires, a customer can’t complete a Checkout Session and customers loading the Checkout Session see a message saying the Checkout Session is expired.</p>

### climate

Manage climate

- **`stripe-pp-cli climate get-orders`** - <p>Lists all Climate order objects. The orders are returned sorted by creation date, with the
most recently created orders appearing first.</p>
- **`stripe-pp-cli climate get-orders-order`** - <p>Retrieves the details of a Climate order object with the given ID.</p>
- **`stripe-pp-cli climate get-products`** - <p>Lists all available Climate product objects.</p>
- **`stripe-pp-cli climate get-products-product`** - <p>Retrieves the details of a Climate product with the given ID.</p>
- **`stripe-pp-cli climate get-suppliers`** - <p>Lists all available Climate supplier objects.</p>
- **`stripe-pp-cli climate get-suppliers-supplier`** - <p>Retrieves a Climate supplier object.</p>
- **`stripe-pp-cli climate post-orders`** - <p>Creates a Climate order object for a given Climate product. The order will be processed immediately
after creation and payment will be deducted your Stripe balance.</p>
- **`stripe-pp-cli climate post-orders-order`** - <p>Updates the specified order by setting the values of the parameters passed.</p>
- **`stripe-pp-cli climate post-orders-order-cancel`** - <p>Cancels a Climate order. You can cancel an order within 24 hours of creation. Stripe refunds the
reservation <code>amount_subtotal</code>, but not the <code>amount_fees</code> for user-triggered cancellations. Frontier
might cancel reservations if suppliers fail to deliver. If Frontier cancels the reservation, Stripe
provides 90 days advance notice and refunds the <code>amount_total</code>.</p>

### confirmation-tokens

Manage confirmation tokens

- **`stripe-pp-cli confirmation-tokens get`** - <p>Retrieves an existing ConfirmationToken object</p>

### country-specs

Manage country specs

- **`stripe-pp-cli country-specs get`** - <p>Lists all Country Spec objects available in the API.</p>
- **`stripe-pp-cli country-specs get-country`** - <p>Returns a Country Spec for a given Country code.</p>

### coupons

Manage coupons

- **`stripe-pp-cli coupons delete`** - <p>You can delete coupons via the <a href="https://dashboard.stripe.com/coupons">coupon management</a> page of the Stripe dashboard. However, deleting a coupon does not affect any customers who have already applied the coupon; it means that new customers can’t redeem the coupon. You can also delete coupons via the API.</p>
- **`stripe-pp-cli coupons get`** - <p>Returns a list of your coupons.</p>
- **`stripe-pp-cli coupons get-coupon`** - <p>Retrieves the coupon with the given ID.</p>
- **`stripe-pp-cli coupons post`** - <p>You can create coupons easily via the <a href="https://dashboard.stripe.com/coupons">coupon management</a> page of the Stripe dashboard. Coupon creation is also accessible via the API if you need to create coupons on the fly.</p>

<p>A coupon has either a <code>percent_off</code> or an <code>amount_off</code> and <code>currency</code>. If you set an <code>amount_off</code>, that amount will be subtracted from any invoice’s subtotal. For example, an invoice with a subtotal of <currency>100</currency> will have a final total of <currency>0</currency> if a coupon with an <code>amount_off</code> of <amount>200</amount> is applied to it and an invoice with a subtotal of <currency>300</currency> will have a final total of <currency>100</currency> if a coupon with an <code>amount_off</code> of <amount>200</amount> is applied to it.</p>
- **`stripe-pp-cli coupons post-coupon`** - <p>Updates the metadata of a coupon. Other coupon details (currency, duration, amount_off) are, by design, not editable.</p>

### credit-notes

Manage credit notes

- **`stripe-pp-cli credit-notes get`** - <p>Returns a list of credit notes.</p>
- **`stripe-pp-cli credit-notes get-id`** - <p>Retrieves the credit note object with the given identifier.</p>
- **`stripe-pp-cli credit-notes get-preview`** - <p>Get a preview of a credit note without creating it.</p>
- **`stripe-pp-cli credit-notes get-preview-lines`** - <p>When retrieving a credit note preview, you’ll get a <strong>lines</strong> property containing the first handful of those items. This URL you can retrieve the full (paginated) list of line items.</p>
- **`stripe-pp-cli credit-notes post`** - <p>Issue a credit note to adjust the amount of a finalized invoice. A credit note will first reduce the invoice’s <code>amount_remaining</code> (and <code>amount_due</code>), but not below zero.
This amount is indicated by the credit note’s <code>pre_payment_amount</code>. The excess amount is indicated by <code>post_payment_amount</code>, and it can result in any combination of the following:</p>

<ul>
<li>Refunds: create a new refund (using <code>refund_amount</code>) or link existing refunds (using <code>refunds</code>).</li>
<li>Customer balance credit: credit the customer’s balance (using <code>credit_amount</code>) which will be automatically applied to their next invoice when it’s finalized.</li>
<li>Outside of Stripe credit: record the amount that is or will be credited outside of Stripe (using <code>out_of_band_amount</code>).</li>
</ul>

<p>The sum of refunds, customer balance credits, and outside of Stripe credits must equal the <code>post_payment_amount</code>.</p>

<p>You may issue multiple credit notes for an invoice. Each credit note may increment the invoice’s <code>pre_payment_credit_notes_amount</code>,
<code>post_payment_credit_notes_amount</code>, or both, depending on the invoice’s <code>amount_remaining</code> at the time of credit note creation.</p>
- **`stripe-pp-cli credit-notes post-id`** - <p>Updates an existing credit note.</p>

### customer-sessions

Manage customer sessions

- **`stripe-pp-cli customer-sessions post`** - <p>Creates a Customer Session object that includes a single-use client secret that you can use on your front-end to grant client-side API access for certain customer resources.</p>

### customers

Manage customers

- **`stripe-pp-cli customers delete`** - <p>Permanently deletes a customer. It cannot be undone. Also immediately cancels any active subscriptions on the customer.</p>
- **`stripe-pp-cli customers get`** - <p>Returns a list of your customers. The customers are returned sorted by creation date, with the most recent customers appearing first.</p>
- **`stripe-pp-cli customers get-customer`** - <p>Retrieves a Customer object.</p>
- **`stripe-pp-cli customers get-search`** - <p>Search for customers you’ve previously created using Stripe’s <a href="/docs/search#search-query-language">Search Query Language</a>.
Don’t use search in read-after-write flows where strict consistency is necessary. Under normal operating
conditions, data is searchable in less than a minute. Occasionally, propagation of new or updated data can be up
to an hour behind during outages. Search functionality is not available to merchants in India.</p>
- **`stripe-pp-cli customers post`** - <p>Creates a new customer object.</p>
- **`stripe-pp-cli customers post-customer`** - <p>Updates the specified customer by setting the values of the parameters passed. Any parameters not provided are left unchanged. For example, if you pass the <strong>source</strong> parameter, that becomes the customer’s active source (such as a card) to be used for all charges in the future. When you update a customer to a new valid card source by passing the <strong>source</strong> parameter: for each of the customer’s current subscriptions, if the subscription bills automatically and is in the <code>past_due</code> state, then the latest open invoice for the subscription with automatic collection enabled is retried. This retry doesn’t count as an automatic retry, and doesn’t affect the next regularly scheduled payment for the invoice. Changing the <strong>default_source</strong> for a customer doesn’t trigger this behavior.</p>

<p>This request accepts mostly the same arguments as the customer creation call.</p>

### disputes

Manage disputes

- **`stripe-pp-cli disputes get`** - <p>Returns a list of your disputes.</p>
- **`stripe-pp-cli disputes get-dispute`** - <p>Retrieves the dispute with the given ID.</p>
- **`stripe-pp-cli disputes post`** - <p>When you get a dispute, contacting your customer is always the best first step. If that doesn’t work, you can submit evidence to help us resolve the dispute in your favor. You can do this in your <a href="https://dashboard.stripe.com/disputes">dashboard</a>, but if you prefer, you can use the API to submit evidence programmatically.</p>

<p>Depending on your dispute type, different evidence fields will give you a better chance of winning your dispute. To figure out which evidence fields to provide, see our <a href="/docs/disputes/categories">guide to dispute types</a>.</p>

### entitlements

Manage entitlements

- **`stripe-pp-cli entitlements get-active`** - <p>Retrieve a list of active entitlements for a customer</p>
- **`stripe-pp-cli entitlements get-active-id`** - <p>Retrieve an active entitlement</p>
- **`stripe-pp-cli entitlements get-features`** - <p>Retrieve a list of features</p>
- **`stripe-pp-cli entitlements get-features-id`** - <p>Retrieves a feature</p>
- **`stripe-pp-cli entitlements post-features`** - <p>Creates a feature</p>
- **`stripe-pp-cli entitlements post-features-id`** - <p>Update a feature’s metadata or permanently deactivate it.</p>

### ephemeral-keys

Manage ephemeral keys

- **`stripe-pp-cli ephemeral-keys delete-key`** - <p>Invalidates a short-lived API key for a given resource.</p>
- **`stripe-pp-cli ephemeral-keys post`** - <p>Creates a short-lived API key for a given resource.</p>

### events

Manage events

- **`stripe-pp-cli events get`** - <p>List events, going back up to 30 days. Each event data is rendered according to Stripe API version at its creation time, specified in <a href="https://docs.stripe.com/api/events/object">event object</a> <code>api_version</code> attribute (not according to your current Stripe API version or <code>Stripe-Version</code> header).</p>
- **`stripe-pp-cli events get-id`** - <p>Retrieves the details of an event if it was created in the last 30 days. Supply the unique identifier of the event, which you might have received in a webhook.</p>

### exchange-rates

Manage exchange rates

- **`stripe-pp-cli exchange-rates get`** - <p>[Deprecated] The <code>ExchangeRate</code> APIs are deprecated. Please use the <a href="https://docs.stripe.com/payments/currencies/localize-prices/fx-quotes-api">FX Quotes API</a> instead.</p>

<p>Returns a list of objects that contain the rates at which foreign currencies are converted to one another. Only shows the currencies for which Stripe supports.</p>
- **`stripe-pp-cli exchange-rates get-rate-id`** - <p>[Deprecated] The <code>ExchangeRate</code> APIs are deprecated. Please use the <a href="https://docs.stripe.com/payments/currencies/localize-prices/fx-quotes-api">FX Quotes API</a> instead.</p>

<p>Retrieves the exchange rates from the given currency to every supported currency.</p>

### file-links

Manage file links

- **`stripe-pp-cli file-links get`** - <p>Returns a list of file links.</p>
- **`stripe-pp-cli file-links get-link`** - <p>Retrieves the file link with the given ID.</p>
- **`stripe-pp-cli file-links post`** - <p>Creates a new file link object.</p>
- **`stripe-pp-cli file-links post-link`** - <p>Updates an existing file link object. Expired links can no longer be updated.</p>

### files

Manage files

- **`stripe-pp-cli files get`** - <p>Returns a list of the files that your account has access to. Stripe sorts and returns the files by their creation dates, placing the most recently created files at the top.</p>
- **`stripe-pp-cli files get-file`** - <p>Retrieves the details of an existing file object. After you supply a unique file ID, Stripe returns the corresponding file object. Learn how to <a href="/docs/file-upload#download-file-contents">access file contents</a>.</p>
- **`stripe-pp-cli files post`** - <p>To upload a file to Stripe, you need to send a request of type <code>multipart/form-data</code>. Include the file you want to upload in the request, and the parameters for creating a file.</p>

<p>All of Stripe’s officially supported Client libraries support sending <code>multipart/form-data</code>.</p>

### financial-connections

Manage financial connections

- **`stripe-pp-cli financial-connections get-accounts`** - <p>Returns a list of Financial Connections <code>Account</code> objects.</p>
- **`stripe-pp-cli financial-connections get-accounts-account`** - <p>Retrieves the details of an Financial Connections <code>Account</code>.</p>
- **`stripe-pp-cli financial-connections get-accounts-account-owners`** - <p>Lists all owners for a given <code>Account</code></p>
- **`stripe-pp-cli financial-connections get-sessions-session`** - <p>Retrieves the details of a Financial Connections <code>Session</code></p>
- **`stripe-pp-cli financial-connections get-transactions`** - <p>Returns a list of Financial Connections <code>Transaction</code> objects.</p>
- **`stripe-pp-cli financial-connections get-transactions-transaction`** - <p>Retrieves the details of a Financial Connections <code>Transaction</code></p>
- **`stripe-pp-cli financial-connections post-accounts-account-disconnect`** - <p>Disables your access to a Financial Connections <code>Account</code>. You will no longer be able to access data associated with the account (e.g. balances, transactions).</p>
- **`stripe-pp-cli financial-connections post-accounts-account-refresh`** - <p>Refreshes the data associated with a Financial Connections <code>Account</code>.</p>
- **`stripe-pp-cli financial-connections post-accounts-account-subscribe`** - <p>Subscribes to periodic refreshes of data associated with a Financial Connections <code>Account</code>. When the account status is active, data is typically refreshed once a day.</p>
- **`stripe-pp-cli financial-connections post-accounts-account-unsubscribe`** - <p>Unsubscribes from periodic refreshes of data associated with a Financial Connections <code>Account</code>.</p>
- **`stripe-pp-cli financial-connections post-sessions`** - <p>To launch the Financial Connections authorization flow, create a <code>Session</code>. The session’s <code>client_secret</code> can be used to launch the flow using Stripe.js.</p>

### forwarding

Manage forwarding

- **`stripe-pp-cli forwarding get-requests`** - <p>Lists all ForwardingRequest objects.</p>
- **`stripe-pp-cli forwarding get-requests-id`** - <p>Retrieves a ForwardingRequest object.</p>
- **`stripe-pp-cli forwarding post-requests`** - <p>Creates a ForwardingRequest object.</p>

### identity

Manage identity

- **`stripe-pp-cli identity get-verification-reports`** - <p>List all verification reports.</p>
- **`stripe-pp-cli identity get-verification-reports-report`** - <p>Retrieves an existing VerificationReport</p>
- **`stripe-pp-cli identity get-verification-sessions`** - <p>Returns a list of VerificationSessions</p>
- **`stripe-pp-cli identity get-verification-sessions-session`** - <p>Retrieves the details of a VerificationSession that was previously created.</p>

<p>When the session status is <code>requires_input</code>, you can use this method to retrieve a valid
<code>client_secret</code> or <code>url</code> to allow re-submission.</p>
- **`stripe-pp-cli identity post-verification-sessions`** - <p>Creates a VerificationSession object.</p>

<p>After the VerificationSession is created, display a verification modal using the session <code>client_secret</code> or send your users to the session’s <code>url</code>.</p>

<p>If your API key is in test mode, verification checks won’t actually process, though everything else will occur as if in live mode.</p>

<p>Related guide: <a href="/docs/identity/verify-identity-documents">Verify your users’ identity documents</a></p>
- **`stripe-pp-cli identity post-verification-sessions-session`** - <p>Updates a VerificationSession object.</p>

<p>When the session status is <code>requires_input</code>, you can use this method to update the
verification check and options.</p>
- **`stripe-pp-cli identity post-verification-sessions-session-cancel`** - <p>A VerificationSession object can be canceled when it is in <code>requires_input</code> <a href="/docs/identity/how-sessions-work">status</a>.</p>

<p>Once canceled, future submission attempts are disabled. This cannot be undone. <a href="/docs/identity/verification-sessions#cancel">Learn more</a>.</p>
- **`stripe-pp-cli identity post-verification-sessions-session-redact`** - <p>Redact a VerificationSession to remove all collected information from Stripe. This will redact
the VerificationSession and all objects related to it, including VerificationReports, Events,
request logs, etc.</p>

<p>A VerificationSession object can be redacted when it is in <code>requires_input</code> or <code>verified</code>
<a href="/docs/identity/how-sessions-work">status</a>. Redacting a VerificationSession in <code>requires_action</code>
state will automatically cancel it.</p>

<p>The redaction process may take up to four days. When the redaction process is in progress, the
VerificationSession’s <code>redaction.status</code> field will be set to <code>processing</code>; when the process is
finished, it will change to <code>redacted</code> and an <code>identity.verification_session.redacted</code> event
will be emitted.</p>

<p>Redaction is irreversible. Redacted objects are still accessible in the Stripe API, but all the
fields that contain personal data will be replaced by the string <code>[redacted]</code> or a similar
placeholder. The <code>metadata</code> field will also be erased. Redacted objects cannot be updated or
used for any purpose.</p>

<p><a href="/docs/identity/verification-sessions#redact">Learn more</a>.</p>

### invoice-payments

Manage invoice payments

- **`stripe-pp-cli invoice-payments get`** - <p>When retrieving an invoice, there is an includable payments property containing the first handful of those items. There is also a URL where you can retrieve the full (paginated) list of payments.</p>
- **`stripe-pp-cli invoice-payments get-invoicepayments`** - <p>Retrieves the invoice payment with the given ID.</p>

### invoice-rendering-templates

Manage invoice rendering templates

- **`stripe-pp-cli invoice-rendering-templates get`** - <p>List all templates, ordered by creation date, with the most recently created template appearing first.</p>
- **`stripe-pp-cli invoice-rendering-templates get-template`** - <p>Retrieves an invoice rendering template with the given ID. It by default returns the latest version of the template. Optionally, specify a version to see previous versions.</p>

### invoiceitems

Manage invoiceitems

- **`stripe-pp-cli invoiceitems delete`** - <p>Deletes an invoice item, removing it from an invoice. Deleting invoice items is only possible when they’re not attached to invoices, or if it’s attached to a draft invoice.</p>
- **`stripe-pp-cli invoiceitems get`** - <p>Returns a list of your invoice items. Invoice items are returned sorted by creation date, with the most recently created invoice items appearing first.</p>
- **`stripe-pp-cli invoiceitems get-invoiceitem`** - <p>Retrieves the invoice item with the given ID.</p>
- **`stripe-pp-cli invoiceitems post`** - <p>Creates an item to be added to a draft invoice (up to 250 items per invoice). If no invoice is specified, the item will be on the next invoice created for the customer specified.</p>
- **`stripe-pp-cli invoiceitems post-invoiceitem`** - <p>Updates the amount or description of an invoice item on an upcoming invoice. Updating an invoice item is only possible before the invoice it’s attached to is closed.</p>

### invoices

Manage invoices

- **`stripe-pp-cli invoices delete`** - <p>Permanently deletes a one-off invoice draft. This cannot be undone. Attempts to delete invoices that are no longer in a draft state will fail; once an invoice has been finalized or if an invoice is for a subscription, it must be <a href="/api/invoices/void">voided</a>.</p>
- **`stripe-pp-cli invoices get`** - <p>You can list all invoices, or list the invoices for a specific customer. The invoices are returned sorted by creation date, with the most recently created invoices appearing first.</p>
- **`stripe-pp-cli invoices get-invoice`** - <p>Retrieves the invoice with the given ID.</p>
- **`stripe-pp-cli invoices get-search`** - <p>Search for invoices you’ve previously created using Stripe’s <a href="/docs/search#search-query-language">Search Query Language</a>.
Don’t use search in read-after-write flows where strict consistency is necessary. Under normal operating
conditions, data is searchable in less than a minute. Occasionally, propagation of new or updated data can be up
to an hour behind during outages. Search functionality is not available to merchants in India.</p>
- **`stripe-pp-cli invoices post`** - <p>This endpoint creates a draft invoice for a given customer. The invoice remains a draft until you <a href="/api/invoices/finalize">finalize</a> the invoice, which allows you to <a href="/api/invoices/pay">pay</a> or <a href="/api/invoices/send">send</a> the invoice to your customers.</p>
- **`stripe-pp-cli invoices post-create-preview`** - <p>At any time, you can preview the upcoming invoice for a subscription or subscription schedule. This will show you all the charges that are pending, including subscription renewal charges, invoice item charges, etc. It will also show you any discounts that are applicable to the invoice.</p>

<p>You can also preview the effects of creating or updating a subscription or subscription schedule, including a preview of any prorations that will take place. To ensure that the actual proration is calculated exactly the same as the previewed proration, you should pass the <code>subscription_details.proration_date</code> parameter when doing the actual subscription update.</p>

<p>The recommended way to get only the prorations being previewed on the invoice is to consider line items where <code>parent.subscription_item_details.proration</code> is <code>true</code>.</p>

<p>Note that when you are viewing an upcoming invoice, you are simply viewing a preview – the invoice has not yet been created. As such, the upcoming invoice will not show up in invoice listing calls, and you cannot use the API to pay or edit the invoice. If you want to change the amount that your customer will be billed, you can add, remove, or update pending invoice items, or update the customer’s discount.</p>

<p>Note: Currency conversion calculations use the latest exchange rates. Exchange rates may vary between the time of the preview and the time of the actual invoice creation. <a href="https://docs.stripe.com/currencies/conversions">Learn more</a></p>
- **`stripe-pp-cli invoices post-invoice`** - <p>Draft invoices are fully editable. Once an invoice is <a href="/docs/billing/invoices/workflow#finalized">finalized</a>,
monetary values, as well as <code>collection_method</code>, become uneditable.</p>

<p>If you would like to stop the Stripe Billing engine from automatically finalizing, reattempting payments on,
sending reminders for, or <a href="/docs/billing/invoices/reconciliation">automatically reconciling</a> invoices, pass
<code>auto_advance=false</code>.</p>

### issuing

Manage issuing

- **`stripe-pp-cli issuing get-authorizations`** - <p>Returns a list of Issuing <code>Authorization</code> objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.</p>
- **`stripe-pp-cli issuing get-authorizations-authorization`** - <p>Retrieves an Issuing <code>Authorization</code> object.</p>
- **`stripe-pp-cli issuing get-cardholders`** - <p>Returns a list of Issuing <code>Cardholder</code> objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.</p>
- **`stripe-pp-cli issuing get-cardholders-cardholder`** - <p>Retrieves an Issuing <code>Cardholder</code> object.</p>
- **`stripe-pp-cli issuing get-cards`** - <p>Returns a list of Issuing <code>Card</code> objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.</p>
- **`stripe-pp-cli issuing get-cards-card`** - <p>Retrieves an Issuing <code>Card</code> object.</p>
- **`stripe-pp-cli issuing get-disputes`** - <p>Returns a list of Issuing <code>Dispute</code> objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.</p>
- **`stripe-pp-cli issuing get-disputes-dispute`** - <p>Retrieves an Issuing <code>Dispute</code> object.</p>
- **`stripe-pp-cli issuing get-personalization-designs`** - <p>Returns a list of personalization design objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.</p>
- **`stripe-pp-cli issuing get-personalization-designs-personalization-design`** - <p>Retrieves a personalization design object.</p>
- **`stripe-pp-cli issuing get-physical-bundles`** - <p>Returns a list of physical bundle objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.</p>
- **`stripe-pp-cli issuing get-physical-bundles-physical-bundle`** - <p>Retrieves a physical bundle object.</p>
- **`stripe-pp-cli issuing get-tokens`** - <p>Lists all Issuing <code>Token</code> objects for a given card.</p>
- **`stripe-pp-cli issuing get-tokens-token`** - <p>Retrieves an Issuing <code>Token</code> object.</p>
- **`stripe-pp-cli issuing get-transactions`** - <p>Returns a list of Issuing <code>Transaction</code> objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.</p>
- **`stripe-pp-cli issuing get-transactions-transaction`** - <p>Retrieves an Issuing <code>Transaction</code> object.</p>
- **`stripe-pp-cli issuing post-authorizations-authorization`** - <p>Updates the specified Issuing <code>Authorization</code> object by setting the values of the parameters passed. Any parameters not provided will be left unchanged.</p>
- **`stripe-pp-cli issuing post-authorizations-authorization-approve`** - <p>[Deprecated] Approves a pending Issuing <code>Authorization</code> object. This request should be made within the timeout window of the <a href="/docs/issuing/controls/real-time-authorizations">real-time authorization</a> flow. 
This method is deprecated. Instead, <a href="/docs/issuing/controls/real-time-authorizations#authorization-handling">respond directly to the webhook request to approve an authorization</a>.</p>
- **`stripe-pp-cli issuing post-authorizations-authorization-decline`** - <p>[Deprecated] Declines a pending Issuing <code>Authorization</code> object. This request should be made within the timeout window of the <a href="/docs/issuing/controls/real-time-authorizations">real time authorization</a> flow.
This method is deprecated. Instead, <a href="/docs/issuing/controls/real-time-authorizations#authorization-handling">respond directly to the webhook request to decline an authorization</a>.</p>
- **`stripe-pp-cli issuing post-cardholders`** - <p>Creates a new Issuing <code>Cardholder</code> object that can be issued cards.</p>
- **`stripe-pp-cli issuing post-cardholders-cardholder`** - <p>Updates the specified Issuing <code>Cardholder</code> object by setting the values of the parameters passed. Any parameters not provided will be left unchanged.</p>
- **`stripe-pp-cli issuing post-cards`** - <p>Creates an Issuing <code>Card</code> object.</p>
- **`stripe-pp-cli issuing post-cards-card`** - <p>Updates the specified Issuing <code>Card</code> object by setting the values of the parameters passed. Any parameters not provided will be left unchanged.</p>
- **`stripe-pp-cli issuing post-disputes`** - <p>Creates an Issuing <code>Dispute</code> object. Individual pieces of evidence within the <code>evidence</code> object are optional at this point. Stripe only validates that required evidence is present during submission. Refer to <a href="/docs/issuing/purchases/disputes#dispute-reasons-and-evidence">Dispute reasons and evidence</a> for more details about evidence requirements.</p>
- **`stripe-pp-cli issuing post-disputes-dispute`** - <p>Updates the specified Issuing <code>Dispute</code> object by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Properties on the <code>evidence</code> object can be unset by passing in an empty string.</p>
- **`stripe-pp-cli issuing post-disputes-dispute-submit`** - <p>Submits an Issuing <code>Dispute</code> to the card network. Stripe validates that all evidence fields required for the dispute’s reason are present. For more details, see <a href="/docs/issuing/purchases/disputes#dispute-reasons-and-evidence">Dispute reasons and evidence</a>.</p>
- **`stripe-pp-cli issuing post-personalization-designs`** - <p>Creates a personalization design object.</p>
- **`stripe-pp-cli issuing post-personalization-designs-personalization-design`** - <p>Updates a card personalization object.</p>
- **`stripe-pp-cli issuing post-tokens-token`** - <p>Attempts to update the specified Issuing <code>Token</code> object to the status specified.</p>
- **`stripe-pp-cli issuing post-transactions-transaction`** - <p>Updates the specified Issuing <code>Transaction</code> object by setting the values of the parameters passed. Any parameters not provided will be left unchanged.</p>

### mandates

Manage mandates

- **`stripe-pp-cli mandates get`** - <p>Retrieves a Mandate object.</p>

### payment-attempt-records

Manage payment attempt records

- **`stripe-pp-cli payment-attempt-records get`** - <p>List all the Payment Attempt Records attached to the specified Payment Record.</p>
- **`stripe-pp-cli payment-attempt-records get-id`** - <p>Retrieves a Payment Attempt Record with the given ID</p>

### payment-intents

Manage payment intents

- **`stripe-pp-cli payment-intents get`** - <p>Returns a list of PaymentIntents.</p>
- **`stripe-pp-cli payment-intents get-intent`** - <p>Retrieves the details of a PaymentIntent that has previously been created. </p>

<p>You can retrieve a PaymentIntent client-side using a publishable key when the <code>client_secret</code> is in the query string. </p>

<p>If you retrieve a PaymentIntent with a publishable key, it only returns a subset of properties. Refer to the <a href="#payment_intent_object">payment intent</a> object reference for more details.</p>
- **`stripe-pp-cli payment-intents get-search`** - <p>Search for PaymentIntents you’ve previously created using Stripe’s <a href="/docs/search#search-query-language">Search Query Language</a>.
Don’t use search in read-after-write flows where strict consistency is necessary. Under normal operating
conditions, data is searchable in less than a minute. Occasionally, propagation of new or updated data can be up
to an hour behind during outages. Search functionality is not available to merchants in India.</p>
- **`stripe-pp-cli payment-intents post`** - <p>Creates a PaymentIntent object.</p>

<p>After the PaymentIntent is created, attach a payment method and <a href="/docs/api/payment_intents/confirm">confirm</a>
to continue the payment. Learn more about <a href="/docs/payments/payment-intents">the available payment flows
with the Payment Intents API</a>.</p>

<p>When you use <code>confirm=true</code> during creation, it’s equivalent to creating
and confirming the PaymentIntent in the same call. You can use any parameters
available in the <a href="/docs/api/payment_intents/confirm">confirm API</a> when you supply
<code>confirm=true</code>.</p>
- **`stripe-pp-cli payment-intents post-intent`** - <p>Updates properties on a PaymentIntent object without confirming.</p>

<p>Depending on which properties you update, you might need to confirm the
PaymentIntent again. For example, updating the <code>payment_method</code>
always requires you to confirm the PaymentIntent again. If you prefer to
update and confirm at the same time, we recommend updating properties through
the <a href="/docs/api/payment_intents/confirm">confirm API</a> instead.</p>

### payment-links

Manage payment links

- **`stripe-pp-cli payment-links get`** - <p>Returns a list of your payment links.</p>
- **`stripe-pp-cli payment-links get-paymentlinks`** - <p>Retrieve a payment link.</p>
- **`stripe-pp-cli payment-links post`** - <p>Creates a payment link.</p>
- **`stripe-pp-cli payment-links post-paymentlinks`** - <p>Updates a payment link.</p>

### payment-method-configurations

Manage payment method configurations

- **`stripe-pp-cli payment-method-configurations get`** - <p>List payment method configurations</p>
- **`stripe-pp-cli payment-method-configurations get-configuration`** - <p>Retrieve payment method configuration</p>
- **`stripe-pp-cli payment-method-configurations post`** - <p>Creates a payment method configuration</p>
- **`stripe-pp-cli payment-method-configurations post-configuration`** - <p>Update payment method configuration</p>

### payment-method-domains

Manage payment method domains

- **`stripe-pp-cli payment-method-domains get`** - <p>Lists the details of existing payment method domains.</p>
- **`stripe-pp-cli payment-method-domains get-paymentmethoddomains`** - <p>Retrieves the details of an existing payment method domain.</p>
- **`stripe-pp-cli payment-method-domains post`** - <p>Creates a payment method domain.</p>
- **`stripe-pp-cli payment-method-domains post-paymentmethoddomains`** - <p>Updates an existing payment method domain.</p>

### payment-methods

Manage payment methods

- **`stripe-pp-cli payment-methods get`** - <p>Returns a list of all PaymentMethods.</p>
- **`stripe-pp-cli payment-methods get-paymentmethods`** - <p>Retrieves a PaymentMethod object attached to the StripeAccount. To retrieve a payment method attached to a Customer, you should use <a href="/docs/api/payment_methods/customer">Retrieve a Customer’s PaymentMethods</a></p>
- **`stripe-pp-cli payment-methods post`** - <p>Creates a PaymentMethod object. Read the <a href="/docs/stripe-js/reference#stripe-create-payment-method">Stripe.js reference</a> to learn how to create PaymentMethods via Stripe.js.</p>

<p>Instead of creating a PaymentMethod directly, we recommend using the <a href="/docs/payments/accept-a-payment">PaymentIntents</a> API to accept a payment immediately or the <a href="/docs/payments/save-and-reuse">SetupIntent</a> API to collect payment method details ahead of a future payment.</p>
- **`stripe-pp-cli payment-methods post-paymentmethods`** - <p>Updates a PaymentMethod object. A PaymentMethod must be attached to a customer to be updated.</p>

### payment-records

Manage payment records

- **`stripe-pp-cli payment-records get-id`** - <p>Retrieves a Payment Record with the given ID</p>
- **`stripe-pp-cli payment-records post-report-payment`** - <p>Report a new Payment Record. You may report a Payment Record as it is
 initialized and later report updates through the other report_* methods, or report Payment
 Records in a terminal state directly, through this method.</p>

### payouts

Manage payouts

- **`stripe-pp-cli payouts get`** - <p>Returns a list of existing payouts sent to third-party bank accounts or payouts that Stripe sent to you. The payouts return in sorted order, with the most recently created payouts appearing first.</p>
- **`stripe-pp-cli payouts get-payout`** - <p>Retrieves the details of an existing payout. Supply the unique payout ID from either a payout creation request or the payout list. Stripe returns the corresponding payout information.</p>
- **`stripe-pp-cli payouts post`** - <p>To send funds to your own bank account, create a new payout object. Your <a href="#balance">Stripe balance</a> must cover the payout amount. If it doesn’t, you receive an “Insufficient Funds” error.</p>

<p>If your API key is in test mode, money won’t actually be sent, though every other action occurs as if you’re in live mode.</p>

<p>If you create a manual payout on a Stripe account that uses multiple payment source types, you need to specify the source type balance that the payout draws from. The <a href="/api/balances/object">balance object</a> details available and pending amounts by source type.</p>
- **`stripe-pp-cli payouts post-payout`** - <p>Updates the specified payout by setting the values of the parameters you pass. We don’t change parameters that you don’t provide. This request only accepts the metadata as arguments.</p>

### plans

Manage plans

- **`stripe-pp-cli plans delete`** - <p>Deleting plans means new subscribers can’t be added. Existing subscribers aren’t affected.</p>
- **`stripe-pp-cli plans get`** - <p>Returns a list of your plans.</p>
- **`stripe-pp-cli plans get-plan`** - <p>Retrieves the plan with the given ID.</p>
- **`stripe-pp-cli plans post`** - <p>You can now model subscriptions more flexibly using the <a href="#prices">Prices API</a>. It replaces the Plans API and is backwards compatible to simplify your migration.</p>
- **`stripe-pp-cli plans post-plan`** - <p>Updates the specified plan by setting the values of the parameters passed. Any parameters not provided are left unchanged. By design, you cannot change a plan’s ID, amount, currency, or billing cycle.</p>

### prices

Manage prices

- **`stripe-pp-cli prices get`** - <p>Returns a list of your active prices, excluding <a href="/docs/products-prices/pricing-models#inline-pricing">inline prices</a>. For the list of inactive prices, set <code>active</code> to false.</p>
- **`stripe-pp-cli prices get-price`** - <p>Retrieves the price with the given ID.</p>
- **`stripe-pp-cli prices get-search`** - <p>Search for prices you’ve previously created using Stripe’s <a href="/docs/search#search-query-language">Search Query Language</a>.
Don’t use search in read-after-write flows where strict consistency is necessary. Under normal operating
conditions, data is searchable in less than a minute. Occasionally, propagation of new or updated data can be up
to an hour behind during outages. Search functionality is not available to merchants in India.</p>
- **`stripe-pp-cli prices post`** - <p>Creates a new <a href="https://docs.stripe.com/api/prices">Price</a> for an existing <a href="https://docs.stripe.com/api/products">Product</a>. The Price can be recurring or one-time.</p>
- **`stripe-pp-cli prices post-price`** - <p>Updates the specified price by setting the values of the parameters passed. Any parameters not provided are left unchanged.</p>

### products

Manage products

- **`stripe-pp-cli products delete-id`** - <p>Delete a product. Deleting a product is only possible if it has no prices associated with it. Additionally, deleting a product with <code>type=good</code> is only possible if it has no SKUs associated with it.</p>
- **`stripe-pp-cli products get`** - <p>Returns a list of your products. The products are returned sorted by creation date, with the most recently created products appearing first.</p>
- **`stripe-pp-cli products get-id`** - <p>Retrieves the details of an existing product. Supply the unique product ID from either a product creation request or the product list, and Stripe will return the corresponding product information.</p>
- **`stripe-pp-cli products get-search`** - <p>Search for products you’ve previously created using Stripe’s <a href="/docs/search#search-query-language">Search Query Language</a>.
Don’t use search in read-after-write flows where strict consistency is necessary. Under normal operating
conditions, data is searchable in less than a minute. Occasionally, propagation of new or updated data can be up
to an hour behind during outages. Search functionality is not available to merchants in India.</p>
- **`stripe-pp-cli products post`** - <p>Creates a new product object.</p>
- **`stripe-pp-cli products post-id`** - <p>Updates the specific product by setting the values of the parameters passed. Any parameters not provided will be left unchanged.</p>

### promotion-codes

Manage promotion codes

- **`stripe-pp-cli promotion-codes get`** - <p>Returns a list of your promotion codes.</p>
- **`stripe-pp-cli promotion-codes get-promotioncodes`** - <p>Retrieves the promotion code with the given ID. In order to retrieve a promotion code by the customer-facing <code>code</code> use <a href="/docs/api/promotion_codes/list">list</a> with the desired <code>code</code>.</p>
- **`stripe-pp-cli promotion-codes post`** - <p>A promotion code points to an underlying promotion. You can optionally restrict the code to a specific customer, redemption limit, and expiration date.</p>
- **`stripe-pp-cli promotion-codes post-promotioncodes`** - <p>Updates the specified promotion code by setting the values of the parameters passed. Most fields are, by design, not editable.</p>

### quotes

Manage quotes

- **`stripe-pp-cli quotes get`** - <p>Returns a list of your quotes.</p>
- **`stripe-pp-cli quotes get-quote`** - <p>Retrieves the quote with the given ID.</p>
- **`stripe-pp-cli quotes post`** - <p>A quote models prices and services for a customer. Default options for <code>header</code>, <code>description</code>, <code>footer</code>, and <code>expires_at</code> can be set in the dashboard via the <a href="https://dashboard.stripe.com/settings/billing/quote">quote template</a>.</p>
- **`stripe-pp-cli quotes post-quote`** - <p>A quote models prices and services for a customer.</p>

### radar

Manage radar

- **`stripe-pp-cli radar delete-value-list-items-item`** - <p>Deletes a <code>ValueListItem</code> object, removing it from its parent value list.</p>
- **`stripe-pp-cli radar delete-value-lists-value-list`** - <p>Deletes a <code>ValueList</code> object, also deleting any items contained within the value list. To be deleted, a value list must not be referenced in any rules.</p>
- **`stripe-pp-cli radar get-early-fraud-warnings`** - <p>Returns a list of early fraud warnings.</p>
- **`stripe-pp-cli radar get-early-fraud-warnings-early-fraud-warning`** - <p>Retrieves the details of an early fraud warning that has previously been created. </p>

<p>Please refer to the <a href="#early_fraud_warning_object">early fraud warning</a> object reference for more details.</p>
- **`stripe-pp-cli radar get-value-list-items`** - <p>Returns a list of <code>ValueListItem</code> objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.</p>
- **`stripe-pp-cli radar get-value-list-items-item`** - <p>Retrieves a <code>ValueListItem</code> object.</p>
- **`stripe-pp-cli radar get-value-lists`** - <p>Returns a list of <code>ValueList</code> objects. The objects are sorted in descending order by creation date, with the most recently created object appearing first.</p>
- **`stripe-pp-cli radar get-value-lists-value-list`** - <p>Retrieves a <code>ValueList</code> object.</p>
- **`stripe-pp-cli radar post-payment-evaluations`** - <p>Request a Radar API fraud risk score from Stripe for a payment before sending it for external processor authorization.</p>
- **`stripe-pp-cli radar post-value-list-items`** - <p>Creates a new <code>ValueListItem</code> object, which is added to the specified parent value list.</p>
- **`stripe-pp-cli radar post-value-lists`** - <p>Creates a new <code>ValueList</code> object, which can then be referenced in rules.</p>
- **`stripe-pp-cli radar post-value-lists-value-list`** - <p>Updates a <code>ValueList</code> object by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Note that <code>item_type</code> is immutable.</p>

### refunds

Manage refunds

- **`stripe-pp-cli refunds get`** - <p>Returns a list of all refunds you created. We return the refunds in sorted order, with the most recent refunds appearing first. The 10 most recent refunds are always available by default on the Charge object.</p>
- **`stripe-pp-cli refunds get-refund`** - <p>Retrieves the details of an existing refund.</p>
- **`stripe-pp-cli refunds post`** - <p>When you create a new refund, you must specify a Charge or a PaymentIntent object on which to create it.</p>

<p>Creating a new refund will refund a charge that has previously been created but not yet refunded.
Funds will be refunded to the credit or debit card that was originally charged.</p>

<p>You can optionally refund only part of a charge.
You can do so multiple times, until the entire charge has been refunded.</p>

<p>Once entirely refunded, a charge can’t be refunded again.
This method will raise an error when called on an already-refunded charge,
or when trying to refund more money than is left on a charge.</p>
- **`stripe-pp-cli refunds post-refund`** - <p>Updates the refund that you specify by setting the values of the passed parameters. Any parameters that you don’t provide remain unchanged.</p>

<p>This request only accepts <code>metadata</code> as an argument.</p>

### reporting

Manage reporting

- **`stripe-pp-cli reporting get-report-runs`** - <p>Returns a list of Report Runs, with the most recent appearing first.</p>
- **`stripe-pp-cli reporting get-report-runs-report-run`** - <p>Retrieves the details of an existing Report Run.</p>
- **`stripe-pp-cli reporting get-report-types`** - <p>Returns a full list of Report Types.</p>
- **`stripe-pp-cli reporting get-report-types-report-type`** - <p>Retrieves the details of a Report Type. (Certain report types require a <a href="https://stripe.com/docs/keys#test-live-modes">live-mode API key</a>.)</p>
- **`stripe-pp-cli reporting post-report-runs`** - <p>Creates a new object and begin running the report. (Certain report types require a <a href="https://stripe.com/docs/keys#test-live-modes">live-mode API key</a>.)</p>

### reviews

Manage reviews

- **`stripe-pp-cli reviews get`** - <p>Returns a list of <code>Review</code> objects that have <code>open</code> set to <code>true</code>. The objects are sorted in descending order by creation date, with the most recently created object appearing first.</p>
- **`stripe-pp-cli reviews get-review`** - <p>Retrieves a <code>Review</code> object.</p>

### setup-attempts

Manage setup attempts

- **`stripe-pp-cli setup-attempts get`** - <p>Returns a list of SetupAttempts that associate with a provided SetupIntent.</p>

### setup-intents

Manage setup intents

- **`stripe-pp-cli setup-intents get`** - <p>Returns a list of SetupIntents.</p>
- **`stripe-pp-cli setup-intents get-intent`** - <p>Retrieves the details of a SetupIntent that has previously been created. </p>

<p>Client-side retrieval using a publishable key is allowed when the <code>client_secret</code> is provided in the query string. </p>

<p>When retrieved with a publishable key, only a subset of properties will be returned. Please refer to the <a href="#setup_intent_object">SetupIntent</a> object reference for more details.</p>
- **`stripe-pp-cli setup-intents post`** - <p>Creates a SetupIntent object.</p>

<p>After you create the SetupIntent, attach a payment method and <a href="/docs/api/setup_intents/confirm">confirm</a>
it to collect any required permissions to charge the payment method later.</p>
- **`stripe-pp-cli setup-intents post-intent`** - <p>Updates a SetupIntent object.</p>

### shipping-rates

Manage shipping rates

- **`stripe-pp-cli shipping-rates get`** - <p>Returns a list of your shipping rates.</p>
- **`stripe-pp-cli shipping-rates get-token`** - <p>Returns the shipping rate object with the given ID.</p>
- **`stripe-pp-cli shipping-rates post`** - <p>Creates a new shipping rate object.</p>
- **`stripe-pp-cli shipping-rates post-token`** - <p>Updates an existing shipping rate object.</p>

### sigma

Manage sigma

- **`stripe-pp-cli sigma get-scheduled-query-runs`** - <p>Returns a list of scheduled query runs.</p>
- **`stripe-pp-cli sigma get-scheduled-query-runs-scheduled-query-run`** - <p>Retrieves the details of an scheduled query run.</p>

### sources

Manage sources

- **`stripe-pp-cli sources get`** - <p>Retrieves an existing source object. Supply the unique source ID from a source creation request and Stripe will return the corresponding up-to-date source object information.</p>
- **`stripe-pp-cli sources post`** - <p>Creates a new source object.</p>
- **`stripe-pp-cli sources post-source`** - <p>Updates the specified source by setting the values of the parameters passed. Any parameters not provided will be left unchanged.</p>

<p>This request accepts the <code>metadata</code> and <code>owner</code> as arguments. It is also possible to update type specific information for selected payment methods. Please refer to our <a href="/docs/sources">payment method guides</a> for more detail.</p>

### subscription-items

Manage subscription items

- **`stripe-pp-cli subscription-items delete-item`** - <p>Deletes an item from the subscription. Removing a subscription item from a subscription will not cancel the subscription.</p>
- **`stripe-pp-cli subscription-items get`** - <p>Returns a list of your subscription items for a given subscription.</p>
- **`stripe-pp-cli subscription-items get-item`** - <p>Retrieves the subscription item with the given ID.</p>
- **`stripe-pp-cli subscription-items post`** - <p>Adds a new item to an existing subscription. No existing items will be changed or replaced.</p>
- **`stripe-pp-cli subscription-items post-item`** - <p>Updates the plan or quantity of an item on a current subscription.</p>

### subscription-schedules

Manage subscription schedules

- **`stripe-pp-cli subscription-schedules get`** - <p>Retrieves the list of your subscription schedules.</p>
- **`stripe-pp-cli subscription-schedules get-schedule`** - <p>Retrieves the details of an existing subscription schedule. You only need to supply the unique subscription schedule identifier that was returned upon subscription schedule creation.</p>
- **`stripe-pp-cli subscription-schedules post`** - <p>Creates a new subscription schedule object. Each customer can have up to 500 active or scheduled subscriptions.</p>
- **`stripe-pp-cli subscription-schedules post-schedule`** - <p>Updates an existing subscription schedule.</p>

### subscriptions

Manage subscriptions

- **`stripe-pp-cli subscriptions delete-exposed-id`** - <p>Cancels a customer’s subscription immediately. The customer won’t be charged again for the subscription. After it’s canceled, you can no longer update the subscription or its <a href="/metadata">metadata</a>.</p>

<p>Any pending invoice items that you’ve created are still charged at the end of the period, unless manually <a href="/api/invoiceitems/delete">deleted</a>. If you’ve set the subscription to cancel at the end of the period, any pending prorations are also left in place and collected at the end of the period. But if the subscription is set to cancel immediately, pending prorations are removed if <code>invoice_now</code> and <code>prorate</code> are both set to true.</p>

<p>By default, upon subscription cancellation, Stripe stops automatic collection of all finalized invoices for the customer. This is intended to prevent unexpected payment attempts after the customer has canceled a subscription. However, you can resume automatic collection of the invoices manually after subscription cancellation to have us proceed. Or, you could check for unpaid invoices before allowing the customer to cancel the subscription at all.</p>
- **`stripe-pp-cli subscriptions get`** - <p>By default, returns a list of subscriptions that have not been canceled. In order to list canceled subscriptions, specify <code>status=canceled</code>.</p>
- **`stripe-pp-cli subscriptions get-exposed-id`** - <p>Retrieves the subscription with the given ID.</p>
- **`stripe-pp-cli subscriptions get-search`** - <p>Search for subscriptions you’ve previously created using Stripe’s <a href="/docs/search#search-query-language">Search Query Language</a>.
Don’t use search in read-after-write flows where strict consistency is necessary. Under normal operating
conditions, data is searchable in less than a minute. Occasionally, propagation of new or updated data can be up
to an hour behind during outages. Search functionality is not available to merchants in India.</p>
- **`stripe-pp-cli subscriptions post`** - <p>Creates a new subscription on an existing customer. Each customer can have up to 500 active or scheduled subscriptions.</p>

<p>When you create a subscription with <code>collection_method=charge_automatically</code>, the first invoice is finalized as part of the request.
The <code>payment_behavior</code> parameter determines the exact behavior of the initial payment.</p>

<p>To start subscriptions where the first invoice always begins in a <code>draft</code> status, use <a href="/docs/billing/subscriptions/subscription-schedules#managing">subscription schedules</a> instead.
Schedules provide the flexibility to model more complex billing configurations that change over time.</p>
- **`stripe-pp-cli subscriptions post-exposed-id`** - <p>Updates an existing subscription to match the specified parameters.
When changing prices or quantities, we optionally prorate the price we charge next month to make up for any price changes.
To preview how the proration is calculated, use the <a href="/docs/api/invoices/create_preview">create preview</a> endpoint.</p>

<p>By default, we prorate subscription changes. For example, if a customer signs up on May 1 for a <currency>100</currency> price, they’ll be billed <currency>100</currency> immediately. If on May 15 they switch to a <currency>200</currency> price, then on June 1 they’ll be billed <currency>250</currency> (<currency>200</currency> for a renewal of her subscription, plus a <currency>50</currency> prorating adjustment for half of the previous month’s <currency>100</currency> difference). Similarly, a downgrade generates a credit that is applied to the next invoice. We also prorate when you make quantity changes.</p>

<p>Switching prices does not normally change the billing date or generate an immediate charge unless:</p>

<ul>
<li>The billing interval is changed (for example, from monthly to yearly).</li>
<li>The subscription moves from free to paid.</li>
<li>A trial starts or ends.</li>
</ul>

<p>In these cases, we apply a credit for the unused time on the previous price, immediately charge the customer using the new price, and reset the billing date. Learn about how <a href="/docs/billing/subscriptions/upgrade-downgrade#immediate-payment">Stripe immediately attempts payment for subscription changes</a>.</p>

<p>If you want to charge for an upgrade immediately, pass <code>proration_behavior</code> as <code>always_invoice</code> to create prorations, automatically invoice the customer for those proration adjustments, and attempt to collect payment. If you pass <code>create_prorations</code>, the prorations are created but not automatically invoiced. If you want to bill the customer for the prorations before the subscription’s renewal date, you need to manually <a href="/docs/api/invoices/create">invoice the customer</a>.</p>

<p>If you don’t want to prorate, set the <code>proration_behavior</code> option to <code>none</code>. With this option, the customer is billed <currency>100</currency> on May 1 and <currency>200</currency> on June 1. Similarly, if you set <code>proration_behavior</code> to <code>none</code> when switching between different billing intervals (for example, from monthly to yearly), we don’t generate any credits for the old subscription’s unused time. We still reset the billing date and bill immediately for the new subscription.</p>

<p>Updating the quantity on a subscription many times in an hour may result in <a href="/docs/rate-limits">rate limiting</a>. If you need to bill for a frequently changing quantity, consider integrating <a href="/docs/billing/subscriptions/usage-based">usage-based billing</a> instead.</p>

### tax

Manage tax

- **`stripe-pp-cli tax get-associations-find`** - <p>Finds a tax association object by PaymentIntent id.</p>
- **`stripe-pp-cli tax get-calculations-calculation`** - <p>Retrieves a Tax <code>Calculation</code> object, if the calculation hasn’t expired.</p>
- **`stripe-pp-cli tax get-calculations-calculation-line-items`** - <p>Retrieves the line items of a tax calculation as a collection, if the calculation hasn’t expired.</p>
- **`stripe-pp-cli tax get-registrations`** - <p>Returns a list of Tax <code>Registration</code> objects.</p>
- **`stripe-pp-cli tax get-registrations-id`** - <p>Returns a Tax <code>Registration</code> object.</p>
- **`stripe-pp-cli tax get-settings`** - <p>Retrieves Tax <code>Settings</code> for a merchant.</p>
- **`stripe-pp-cli tax get-transactions-transaction`** - <p>Retrieves a Tax <code>Transaction</code> object.</p>
- **`stripe-pp-cli tax get-transactions-transaction-line-items`** - <p>Retrieves the line items of a committed standalone transaction as a collection.</p>
- **`stripe-pp-cli tax post-calculations`** - <p>Calculates tax based on the input and returns a Tax <code>Calculation</code> object.</p>
- **`stripe-pp-cli tax post-registrations`** - <p>Creates a new Tax <code>Registration</code> object.</p>
- **`stripe-pp-cli tax post-registrations-id`** - <p>Updates an existing Tax <code>Registration</code> object.</p>

<p>A registration cannot be deleted after it has been created. If you wish to end a registration you may do so by setting <code>expires_at</code>.</p>
- **`stripe-pp-cli tax post-settings`** - <p>Updates Tax <code>Settings</code> parameters used in tax calculations. All parameters are editable but none can be removed once set.</p>
- **`stripe-pp-cli tax post-transactions-create-from-calculation`** - <p>Creates a Tax Transaction from a calculation, if that calculation hasn’t expired. Calculations expire after 90 days.</p>
- **`stripe-pp-cli tax post-transactions-create-reversal`** - <p>Partially or fully reverses a previously created <code>Transaction</code>.</p>

### tax-codes

Manage tax codes

- **`stripe-pp-cli tax-codes get`** - <p>A list of <a href="https://stripe.com/docs/tax/tax-categories">all tax codes available</a> to add to Products in order to allow specific tax calculations.</p>
- **`stripe-pp-cli tax-codes get-id`** - <p>Retrieves the details of an existing tax code. Supply the unique tax code ID and Stripe will return the corresponding tax code information.</p>

### tax-ids

Manage tax ids

- **`stripe-pp-cli tax-ids delete-id`** - <p>Deletes an existing account or customer <code>tax_id</code> object.</p>
- **`stripe-pp-cli tax-ids get`** - <p>Returns a list of tax IDs.</p>
- **`stripe-pp-cli tax-ids get-id`** - <p>Retrieves an account or customer <code>tax_id</code> object.</p>
- **`stripe-pp-cli tax-ids post`** - <p>Creates a new account or customer <code>tax_id</code> object.</p>

### tax-rates

Manage tax rates

- **`stripe-pp-cli tax-rates get`** - <p>Returns a list of your tax rates. Tax rates are returned sorted by creation date, with the most recently created tax rates appearing first.</p>
- **`stripe-pp-cli tax-rates get-taxrates`** - <p>Retrieves a tax rate with the given ID</p>
- **`stripe-pp-cli tax-rates post`** - <p>Creates a new tax rate.</p>
- **`stripe-pp-cli tax-rates post-taxrates`** - <p>Updates an existing tax rate.</p>

### terminal

Manage terminal

- **`stripe-pp-cli terminal delete-configurations-configuration`** - <p>Deletes a <code>Configuration</code> object.</p>
- **`stripe-pp-cli terminal delete-locations-location`** - <p>Deletes a <code>Location</code> object.</p>
- **`stripe-pp-cli terminal delete-readers-reader`** - <p>Deletes a <code>Reader</code> object.</p>
- **`stripe-pp-cli terminal get-configurations`** - <p>Returns a list of <code>Configuration</code> objects.</p>
- **`stripe-pp-cli terminal get-configurations-configuration`** - <p>Retrieves a <code>Configuration</code> object.</p>
- **`stripe-pp-cli terminal get-locations`** - <p>Returns a list of <code>Location</code> objects.</p>
- **`stripe-pp-cli terminal get-locations-location`** - <p>Retrieves a <code>Location</code> object.</p>
- **`stripe-pp-cli terminal get-readers`** - <p>Returns a list of <code>Reader</code> objects.</p>
- **`stripe-pp-cli terminal get-readers-reader`** - <p>Retrieves a <code>Reader</code> object.</p>
- **`stripe-pp-cli terminal post-configurations`** - <p>Creates a new <code>Configuration</code> object.</p>
- **`stripe-pp-cli terminal post-configurations-configuration`** - <p>Updates a new <code>Configuration</code> object.</p>
- **`stripe-pp-cli terminal post-connection-tokens`** - <p>To connect to a reader the Stripe Terminal SDK needs to retrieve a short-lived connection token from Stripe, proxied through your server. On your backend, add an endpoint that creates and returns a connection token.</p>
- **`stripe-pp-cli terminal post-locations`** - <p>Creates a new <code>Location</code> object.
For further details, including which address fields are required in each country, see the <a href="/docs/terminal/fleet/locations">Manage locations</a> guide.</p>
- **`stripe-pp-cli terminal post-locations-location`** - <p>Updates a <code>Location</code> object by setting the values of the parameters passed. Any parameters not provided will be left unchanged.</p>
- **`stripe-pp-cli terminal post-onboarding-links`** - <p>Creates a new <code>OnboardingLink</code> object that contains a redirect_url used for onboarding onto Tap to Pay on iPhone.</p>
- **`stripe-pp-cli terminal post-readers`** - <p>Creates a new <code>Reader</code> object.</p>
- **`stripe-pp-cli terminal post-readers-reader`** - <p>Updates a <code>Reader</code> object by setting the values of the parameters passed. Any parameters not provided will be left unchanged.</p>
- **`stripe-pp-cli terminal post-readers-reader-cancel-action`** - <p>Cancels the current reader action. See <a href="/docs/terminal/payments/collect-card-payment?terminal-sdk-platform=server-driven#programmatic-cancellation">Programmatic Cancellation</a> for more details.</p>
- **`stripe-pp-cli terminal post-readers-reader-collect-inputs`** - <p>Initiates an <a href="/docs/terminal/features/collect-inputs">input collection flow</a> on a Reader to display input forms and collect information from your customers.</p>
- **`stripe-pp-cli terminal post-readers-reader-collect-payment-method`** - <p>Initiates a payment flow on a Reader and updates the PaymentIntent with card details before manual confirmation. See <a href="/docs/terminal/payments/collect-card-payment?terminal-sdk-platform=server-driven&process=inspect#collect-a-paymentmethod">Collecting a Payment method</a> for more details.</p>
- **`stripe-pp-cli terminal post-readers-reader-confirm-payment-intent`** - <p>Finalizes a payment on a Reader. See <a href="/docs/terminal/payments/collect-card-payment?terminal-sdk-platform=server-driven&process=inspect#confirm-the-paymentintent">Confirming a Payment</a> for more details.</p>
- **`stripe-pp-cli terminal post-readers-reader-process-payment-intent`** - <p>Initiates a payment flow on a Reader. See <a href="/docs/terminal/payments/collect-card-payment?terminal-sdk-platform=server-driven&process=immediately#process-payment">process the payment</a> for more details.</p>
- **`stripe-pp-cli terminal post-readers-reader-process-setup-intent`** - <p>Initiates a SetupIntent flow on a Reader. See <a href="/docs/terminal/features/saving-payment-details/save-directly">Save directly without charging</a> for more details.</p>
- **`stripe-pp-cli terminal post-readers-reader-refund-payment`** - <p>Initiates an in-person refund on a Reader. See <a href="/docs/terminal/payments/regional?integration-country=CA#refund-an-interac-payment">Refund an Interac Payment</a> for more details.</p>
- **`stripe-pp-cli terminal post-readers-reader-set-reader-display`** - <p>Sets the reader display to show <a href="/docs/terminal/features/display">cart details</a>.</p>

### test-helpers

Manage test helpers

- **`stripe-pp-cli test-helpers delete-test-clocks-test-clock`** - <p>Deletes a test clock.</p>
- **`stripe-pp-cli test-helpers get-test-clocks`** - <p>Returns a list of your test clocks.</p>
- **`stripe-pp-cli test-helpers get-test-clocks-test-clock`** - <p>Retrieves a test clock.</p>
- **`stripe-pp-cli test-helpers post-confirmation-tokens`** - <p>Creates a test mode Confirmation Token server side for your integration tests.</p>
- **`stripe-pp-cli test-helpers post-customers-customer-fund-cash-balance`** - <p>Create an incoming testmode bank transfer</p>
- **`stripe-pp-cli test-helpers post-issuing-authorizations`** - <p>Create a test-mode authorization.</p>
- **`stripe-pp-cli test-helpers post-issuing-authorizations-authorization-capture`** - <p>Capture a test-mode authorization.</p>
- **`stripe-pp-cli test-helpers post-issuing-authorizations-authorization-expire`** - <p>Expire a test-mode Authorization.</p>
- **`stripe-pp-cli test-helpers post-issuing-authorizations-authorization-finalize-amount`** - <p>Finalize the amount on an Authorization prior to capture, when the initial authorization was for an estimated amount.</p>
- **`stripe-pp-cli test-helpers post-issuing-authorizations-authorization-fraud-challenges-respond`** - <p>Respond to a fraud challenge on a testmode Issuing authorization, simulating either a confirmation of fraud or a correction of legitimacy.</p>
- **`stripe-pp-cli test-helpers post-issuing-authorizations-authorization-increment`** - <p>Increment a test-mode Authorization.</p>
- **`stripe-pp-cli test-helpers post-issuing-authorizations-authorization-reverse`** - <p>Reverse a test-mode Authorization.</p>
- **`stripe-pp-cli test-helpers post-issuing-cards-card-shipping-deliver`** - <p>Updates the shipping status of the specified Issuing <code>Card</code> object to <code>delivered</code>.</p>
- **`stripe-pp-cli test-helpers post-issuing-cards-card-shipping-fail`** - <p>Updates the shipping status of the specified Issuing <code>Card</code> object to <code>failure</code>.</p>
- **`stripe-pp-cli test-helpers post-issuing-cards-card-shipping-return`** - <p>Updates the shipping status of the specified Issuing <code>Card</code> object to <code>returned</code>.</p>
- **`stripe-pp-cli test-helpers post-issuing-cards-card-shipping-ship`** - <p>Updates the shipping status of the specified Issuing <code>Card</code> object to <code>shipped</code>.</p>
- **`stripe-pp-cli test-helpers post-issuing-cards-card-shipping-submit`** - <p>Updates the shipping status of the specified Issuing <code>Card</code> object to <code>submitted</code>. This method requires Stripe Version ‘2024-09-30.acacia’ or later.</p>
- **`stripe-pp-cli test-helpers post-issuing-personalization-designs-personalization-design-activate`** - <p>Updates the <code>status</code> of the specified testmode personalization design object to <code>active</code>.</p>
- **`stripe-pp-cli test-helpers post-issuing-personalization-designs-personalization-design-deactivate`** - <p>Updates the <code>status</code> of the specified testmode personalization design object to <code>inactive</code>.</p>
- **`stripe-pp-cli test-helpers post-issuing-personalization-designs-personalization-design-reject`** - <p>Updates the <code>status</code> of the specified testmode personalization design object to <code>rejected</code>.</p>
- **`stripe-pp-cli test-helpers post-issuing-transactions-create-force-capture`** - <p>Allows the user to capture an arbitrary amount, also known as a forced capture.</p>
- **`stripe-pp-cli test-helpers post-issuing-transactions-create-unlinked-refund`** - <p>Allows the user to refund an arbitrary amount, also known as a unlinked refund.</p>
- **`stripe-pp-cli test-helpers post-issuing-transactions-transaction-refund`** - <p>Refund a test-mode Transaction.</p>
- **`stripe-pp-cli test-helpers post-refunds-refund-expire`** - <p>Expire a refund with a status of <code>requires_action</code>.</p>
- **`stripe-pp-cli test-helpers post-terminal-readers-reader-present-payment-method`** - <p>Presents a payment method on a simulated reader. Can be used to simulate accepting a payment, saving a card or refunding a transaction.</p>
- **`stripe-pp-cli test-helpers post-terminal-readers-reader-succeed-input-collection`** - <p>Use this endpoint to trigger a successful input collection on a simulated reader.</p>
- **`stripe-pp-cli test-helpers post-terminal-readers-reader-timeout-input-collection`** - <p>Use this endpoint to complete an input collection with a timeout error on a simulated reader.</p>
- **`stripe-pp-cli test-helpers post-test-clocks`** - <p>Creates a new test clock that can be attached to new customers and quotes.</p>
- **`stripe-pp-cli test-helpers post-test-clocks-test-clock-advance`** - <p>Starts advancing a test clock to a specified time in the future. Advancement is done when status changes to <code>Ready</code>.</p>
- **`stripe-pp-cli test-helpers post-treasury-inbound-transfers-id-fail`** - <p>Transitions a test mode created InboundTransfer to the <code>failed</code> status. The InboundTransfer must already be in the <code>processing</code> state.</p>
- **`stripe-pp-cli test-helpers post-treasury-inbound-transfers-id-return`** - <p>Marks the test mode InboundTransfer object as returned and links the InboundTransfer to a ReceivedDebit. The InboundTransfer must already be in the <code>succeeded</code> state.</p>
- **`stripe-pp-cli test-helpers post-treasury-inbound-transfers-id-succeed`** - <p>Transitions a test mode created InboundTransfer to the <code>succeeded</code> status. The InboundTransfer must already be in the <code>processing</code> state.</p>
- **`stripe-pp-cli test-helpers post-treasury-outbound-payments-id`** - <p>Updates a test mode created OutboundPayment with tracking details. The OutboundPayment must not be cancelable, and cannot be in the <code>canceled</code> or <code>failed</code> states.</p>
- **`stripe-pp-cli test-helpers post-treasury-outbound-payments-id-fail`** - <p>Transitions a test mode created OutboundPayment to the <code>failed</code> status. The OutboundPayment must already be in the <code>processing</code> state.</p>
- **`stripe-pp-cli test-helpers post-treasury-outbound-payments-id-post`** - <p>Transitions a test mode created OutboundPayment to the <code>posted</code> status. The OutboundPayment must already be in the <code>processing</code> state.</p>
- **`stripe-pp-cli test-helpers post-treasury-outbound-payments-id-return`** - <p>Transitions a test mode created OutboundPayment to the <code>returned</code> status. The OutboundPayment must already be in the <code>processing</code> state.</p>
- **`stripe-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer`** - <p>Updates a test mode created OutboundTransfer with tracking details. The OutboundTransfer must not be cancelable, and cannot be in the <code>canceled</code> or <code>failed</code> states.</p>
- **`stripe-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer-fail`** - <p>Transitions a test mode created OutboundTransfer to the <code>failed</code> status. The OutboundTransfer must already be in the <code>processing</code> state.</p>
- **`stripe-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer-post`** - <p>Transitions a test mode created OutboundTransfer to the <code>posted</code> status. The OutboundTransfer must already be in the <code>processing</code> state.</p>
- **`stripe-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer-return`** - <p>Transitions a test mode created OutboundTransfer to the <code>returned</code> status. The OutboundTransfer must already be in the <code>processing</code> state.</p>
- **`stripe-pp-cli test-helpers post-treasury-received-credits`** - <p>Use this endpoint to simulate a test mode ReceivedCredit initiated by a third party. In live mode, you can’t directly create ReceivedCredits initiated by third parties.</p>
- **`stripe-pp-cli test-helpers post-treasury-received-debits`** - <p>Use this endpoint to simulate a test mode ReceivedDebit initiated by a third party. In live mode, you can’t directly create ReceivedDebits initiated by third parties.</p>

### tokens

Manage tokens

- **`stripe-pp-cli tokens get`** - <p>Retrieves the token with the given ID.</p>
- **`stripe-pp-cli tokens post`** - <p>Creates a single-use token that represents a bank account’s details.
You can use this token with any v1 API method in place of a bank account dictionary. You can only use this token once. To do so, attach it to a <a href="#accounts">connected account</a> where <a href="/api/accounts/object#account_object-controller-requirement_collection">controller.requirement_collection</a> is <code>application</code>, which includes Custom accounts.</p>

### topups

Manage topups

- **`stripe-pp-cli topups get`** - <p>Returns a list of top-ups.</p>
- **`stripe-pp-cli topups get-topup`** - <p>Retrieves the details of a top-up that has previously been created. Supply the unique top-up ID that was returned from your previous request, and Stripe will return the corresponding top-up information.</p>
- **`stripe-pp-cli topups post`** - <p>Top up the balance of an account</p>
- **`stripe-pp-cli topups post-topup`** - <p>Updates the metadata of a top-up. Other top-up details are not editable by design.</p>

### transfers

Manage transfers

- **`stripe-pp-cli transfers get`** - <p>Returns a list of existing transfers sent to connected accounts. The transfers are returned in sorted order, with the most recently created transfers appearing first.</p>
- **`stripe-pp-cli transfers get-transfer`** - <p>Retrieves the details of an existing transfer. Supply the unique transfer ID from either a transfer creation request or the transfer list, and Stripe will return the corresponding transfer information.</p>
- **`stripe-pp-cli transfers post`** - <p>To send funds from your Stripe account to a connected account, you create a new transfer object. Your <a href="#balance">Stripe balance</a> must be able to cover the transfer amount, or you’ll receive an “Insufficient Funds” error.</p>
- **`stripe-pp-cli transfers post-transfer`** - <p>Updates the specified transfer by setting the values of the parameters passed. Any parameters not provided will be left unchanged.</p>

<p>This request accepts only metadata as an argument.</p>

### treasury

Manage treasury

- **`stripe-pp-cli treasury get-credit-reversals`** - <p>Returns a list of CreditReversals.</p>
- **`stripe-pp-cli treasury get-credit-reversals-credit-reversal`** - <p>Retrieves the details of an existing CreditReversal by passing the unique CreditReversal ID from either the CreditReversal creation request or CreditReversal list</p>
- **`stripe-pp-cli treasury get-debit-reversals`** - <p>Returns a list of DebitReversals.</p>
- **`stripe-pp-cli treasury get-debit-reversals-debit-reversal`** - <p>Retrieves a DebitReversal object.</p>
- **`stripe-pp-cli treasury get-financial-accounts`** - <p>Returns a list of FinancialAccounts.</p>
- **`stripe-pp-cli treasury get-financial-accounts-financial-account`** - <p>Retrieves the details of a FinancialAccount.</p>
- **`stripe-pp-cli treasury get-financial-accounts-financial-account-features`** - <p>Retrieves Features information associated with the FinancialAccount.</p>
- **`stripe-pp-cli treasury get-inbound-transfers`** - <p>Returns a list of InboundTransfers sent from the specified FinancialAccount.</p>
- **`stripe-pp-cli treasury get-inbound-transfers-id`** - <p>Retrieves the details of an existing InboundTransfer.</p>
- **`stripe-pp-cli treasury get-outbound-payments`** - <p>Returns a list of OutboundPayments sent from the specified FinancialAccount.</p>
- **`stripe-pp-cli treasury get-outbound-payments-id`** - <p>Retrieves the details of an existing OutboundPayment by passing the unique OutboundPayment ID from either the OutboundPayment creation request or OutboundPayment list.</p>
- **`stripe-pp-cli treasury get-outbound-transfers`** - <p>Returns a list of OutboundTransfers sent from the specified FinancialAccount.</p>
- **`stripe-pp-cli treasury get-outbound-transfers-outbound-transfer`** - <p>Retrieves the details of an existing OutboundTransfer by passing the unique OutboundTransfer ID from either the OutboundTransfer creation request or OutboundTransfer list.</p>
- **`stripe-pp-cli treasury get-received-credits`** - <p>Returns a list of ReceivedCredits.</p>
- **`stripe-pp-cli treasury get-received-credits-id`** - <p>Retrieves the details of an existing ReceivedCredit by passing the unique ReceivedCredit ID from the ReceivedCredit list.</p>
- **`stripe-pp-cli treasury get-received-debits`** - <p>Returns a list of ReceivedDebits.</p>
- **`stripe-pp-cli treasury get-received-debits-id`** - <p>Retrieves the details of an existing ReceivedDebit by passing the unique ReceivedDebit ID from the ReceivedDebit list</p>
- **`stripe-pp-cli treasury get-transaction-entries`** - <p>Retrieves a list of TransactionEntry objects.</p>
- **`stripe-pp-cli treasury get-transaction-entries-id`** - <p>Retrieves a TransactionEntry object.</p>
- **`stripe-pp-cli treasury get-transactions`** - <p>Retrieves a list of Transaction objects.</p>
- **`stripe-pp-cli treasury get-transactions-id`** - <p>Retrieves the details of an existing Transaction.</p>
- **`stripe-pp-cli treasury post-credit-reversals`** - <p>Reverses a ReceivedCredit and creates a CreditReversal object.</p>
- **`stripe-pp-cli treasury post-debit-reversals`** - <p>Reverses a ReceivedDebit and creates a DebitReversal object.</p>
- **`stripe-pp-cli treasury post-financial-accounts`** - <p>Creates a new FinancialAccount. Each connected account can have up to three FinancialAccounts by default.</p>
- **`stripe-pp-cli treasury post-financial-accounts-financial-account`** - <p>Updates the details of a FinancialAccount.</p>
- **`stripe-pp-cli treasury post-financial-accounts-financial-account-close`** - <p>Closes a FinancialAccount. A FinancialAccount can only be closed if it has a zero balance, has no pending InboundTransfers, and has canceled all attached Issuing cards.</p>
- **`stripe-pp-cli treasury post-financial-accounts-financial-account-features`** - <p>Updates the Features associated with a FinancialAccount.</p>
- **`stripe-pp-cli treasury post-inbound-transfers`** - <p>Creates an InboundTransfer.</p>
- **`stripe-pp-cli treasury post-inbound-transfers-inbound-transfer-cancel`** - <p>Cancels an InboundTransfer.</p>
- **`stripe-pp-cli treasury post-outbound-payments`** - <p>Creates an OutboundPayment.</p>
- **`stripe-pp-cli treasury post-outbound-payments-id-cancel`** - <p>Cancel an OutboundPayment.</p>
- **`stripe-pp-cli treasury post-outbound-transfers`** - <p>Creates an OutboundTransfer.</p>
- **`stripe-pp-cli treasury post-outbound-transfers-outbound-transfer-cancel`** - <p>An OutboundTransfer can be canceled if the funds have not yet been paid out.</p>

### webhook-endpoints

Manage webhook endpoints

- **`stripe-pp-cli webhook-endpoints delete`** - <p>You can also delete webhook endpoints via the <a href="https://dashboard.stripe.com/account/webhooks">webhook endpoint management</a> page of the Stripe dashboard.</p>
- **`stripe-pp-cli webhook-endpoints get`** - <p>Returns a list of your webhook endpoints.</p>
- **`stripe-pp-cli webhook-endpoints get-webhookendpoints`** - <p>Retrieves the webhook endpoint with the given ID.</p>
- **`stripe-pp-cli webhook-endpoints post`** - <p>A webhook endpoint must have a <code>url</code> and a list of <code>enabled_events</code>. You may optionally specify the Boolean <code>connect</code> parameter. If set to true, then a Connect webhook endpoint that notifies the specified <code>url</code> about events from all connected accounts is created; otherwise an account webhook endpoint that notifies the specified <code>url</code> only about events from your account is created. You can also create webhook endpoints in the <a href="https://dashboard.stripe.com/account/webhooks">webhooks settings</a> section of the Dashboard.</p>
- **`stripe-pp-cli webhook-endpoints post-webhookendpoints`** - <p>Updates the webhook endpoint. You may edit the <code>url</code>, the list of <code>enabled_events</code>, and the status of your endpoint.</p>


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
stripe-pp-cli account

# JSON for scripting and agents
stripe-pp-cli account --json

# Filter to specific fields
stripe-pp-cli account --json --select id,name,status

# Dry run — show the request without sending
stripe-pp-cli account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
stripe-pp-cli account --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-stripe -g
```

Then invoke `/pp-stripe <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/stripe/cmd/stripe-pp-mcp@latest
```

Then register it:

```bash
claude mcp add stripe stripe-pp-mcp -e STRIPE_SECRET_KEY=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/stripe-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `STRIPE_SECRET_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/stripe/cmd/stripe-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "stripe": {
      "command": "stripe-pp-mcp",
      "env": {
        "STRIPE_SECRET_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
stripe-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/stripe-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `STRIPE_SECRET_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `stripe-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $STRIPE_SECRET_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Auth errors after `auth login` succeeded** — Check active profile with `stripe-pp-cli auth status`; switch with `stripe-pp-cli profile use <name>`
- **Rate limit (429) errors on sync** — Test-mode caps at 25 r/s; sync auto-throttles, but reduce `--page-size` if hitting it persistently
- **`sql` command shows empty tables** — Run `stripe-pp-cli sync --full` — local store starts empty until first sync
- **Live-mode key detected** — v1 of stripe-pp-cli does not enforce a live-mode write guard. Audit every command before running with sk_live_... ; prefer sk_test_... for development.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**stripe-cli**](https://github.com/stripe/stripe-cli) — Go (5800 stars)
- [**stripe-agent-toolkit**](https://github.com/stripe/agent-toolkit) — TypeScript (220 stars)
- [**stripemetrics**](https://github.com/igorbenav/stripemetrics) — Python
- [**stripe-cohort-analysis**](https://github.com/petaldata/stripe-cohort-analysis) — Python
- [**profitable**](https://github.com/rameerez/profitable) — Ruby

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
