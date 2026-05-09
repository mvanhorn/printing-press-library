---
name: pp-stripe
description: "Every Stripe feature, plus a local SQLite mirror with FTS, cross-entity SQL, and analytics no other Stripe tool ships. Trigger phrases: `stripe customer health`, `stripe dunning queue`, `stripe health`, `stripe payout reconcile`, `subs about to expire`, `use stripe-pp-cli`, `run stripe-pp-cli`."
author: "Chris Rodriguez"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - stripe-pp-cli
    install:
      - kind: go
        bins: [stripe-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/stripe/cmd/stripe-pp-cli
---

# Stripe — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `stripe-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install stripe --cli-only
   ```
2. Verify: `stripe-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/stripe/cmd/stripe-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

stripe-pp-cli matches the official stripe-cli verb-for-verb and adds what it deliberately omits: a local mirror of customers, subscriptions, invoices, charges, payouts, and events with FTS5 search; cross-entity SQL queries (`sql`); and dossier commands (`health`, `dunning-queue`, `customer-360`, `subs-at-risk`, `payout-reconcile`). Built for agents — every command is one-shot, one-shot, and emits structured output.

## When to Use This CLI

Reach for stripe-pp-cli when you need to (1) run analytics across customers/subs/invoices that Stripe's dashboard doesn't expose, (2) script payment ops in a CI/agent loop without daemon processes, (3) reconcile payouts and balance transactions, or (4) feed Stripe state into a non-Stripe system (CRM, accounting, outreach automation).

## When NOT to Use This CLI (anti-triggers)

Do not use stripe-pp-cli for:

- **Live-mode bulk writes you have not audited.** Mutating commands (POST/PUT/PATCH/DELETE) against an `sk_live_...` key are blocked by default with a clear error; pass `--confirm-live` (or set `STRIPE_CONFIRM_LIVE=1`) once you have audited the invocation. Prefer `sk_test_...` for development regardless.
- **In-person Terminal payments.** `terminal` resource commands are present but Stripe Terminal SDK is not in scope. Use the Terminal iOS/Android SDK.
- **Stripe Issuing card management workflows.** `issuing` commands are REST passthrough only; spending controls, dispute evidence packages not built. Use the Stripe Dashboard.
- **Stripe Tax registration.** `tax-rates` CRUD only; jurisdiction registration not in scope. Use Stripe Tax in Dashboard.
- **Connect platform fan-out across many connected accounts.** Single account at a time via `Stripe-Account` header. Multi-account loops via external script.
- **Local mock-server testing.** Use [stripe-mock](https://github.com/stripe/stripe-mock) directly via `STRIPE_BASE_URL=http://localhost:12111`.

## Not Yet Wired (deferred to v0.2)

| Area | Status | Workaround |
|------|--------|-----------|
| Stripe Issuing (full workflow) | endpoint passthrough only | Use Stripe Dashboard |
| Stripe Terminal (in-person SDK) | endpoint passthrough only | Use Terminal SDK |
| Stripe Tax registration | `tax-rates` CRUD only | Use Stripe Tax in Dashboard |
| `localstripe` mock server | out of scope | Use `stripe-mock` via `STRIPE_BASE_URL` |
| Connect account fan-out | single-account at a time | External script loops |

## Unique Capabilities

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

## Command Reference

**account** — Manage account

- `stripe-pp-cli account` — <p>Retrieves the details of an account.</p>

**account-links** — Manage account links

- `stripe-pp-cli account-links` — <p>Creates an AccountLink object that includes a single-use Stripe URL that the platform can redirect their user to...

**account-sessions** — Manage account sessions

- `stripe-pp-cli account-sessions` — <p>Creates a AccountSession object that includes a single-use token that the platform can use on their front-end to...

**accounts** — Manage accounts

- `stripe-pp-cli accounts delete` — <p>With <a href='/connect'>Connect</a>, you can delete accounts you manage.</p> <p>Test-mode accounts can be deleted...
- `stripe-pp-cli accounts get` — <p>Returns a list of accounts connected to your platform via <a href='/docs/connect'>Connect</a>. If you’re not a...
- `stripe-pp-cli accounts get-account` — <p>Retrieves the details of an account.</p>
- `stripe-pp-cli accounts post` — <p>With <a href='/docs/connect'>Connect</a>, you can create Stripe accounts for your users. To do this, you’ll...
- `stripe-pp-cli accounts post-account` — <p>Updates a <a href='/connect/accounts'>connected account</a> by setting the values of the parameters passed. Any...

**apple-pay** — Manage apple pay

- `stripe-pp-cli apple-pay delete-domains-domain` — <p>Delete an apple pay domain.</p>
- `stripe-pp-cli apple-pay get-domains` — <p>List apple pay domains.</p>
- `stripe-pp-cli apple-pay get-domains-domain` — <p>Retrieve an apple pay domain.</p>
- `stripe-pp-cli apple-pay post-domains` — <p>Create an apple pay domain.</p>

**application-fees** — Manage application fees

- `stripe-pp-cli application-fees get` — <p>Returns a list of application fees you’ve previously collected. The application fees are returned in sorted...
- `stripe-pp-cli application-fees get-id` — <p>Retrieves the details of an application fee that your account has collected. The same information is returned...

**apps** — Manage apps

- `stripe-pp-cli apps get-secrets` — <p>List all secrets stored on the given scope.</p>
- `stripe-pp-cli apps get-secrets-find` — <p>Finds a secret in the secret store by name and scope.</p>
- `stripe-pp-cli apps post-secrets` — <p>Create or replace a secret in the secret store.</p>
- `stripe-pp-cli apps post-secrets-delete` — <p>Deletes a secret from the secret store by name and scope.</p>

**balance** — Manage balance

- `stripe-pp-cli balance` — <p>Retrieves the current account balance, based on the authentication that was used to make the request. For a...

**balance-settings** — Manage balance settings

- `stripe-pp-cli balance-settings get` — <p>Retrieves balance settings for a given connected account. Related guide: <a href='/connect/authentication'>Making...
- `stripe-pp-cli balance-settings post` — <p>Updates balance settings for a given connected account. Related guide: <a href='/connect/authentication'>Making...

**balance-transactions** — Manage balance transactions

- `stripe-pp-cli balance-transactions get` — <p>Returns a list of transactions that have contributed to the Stripe account balance (e.g., charges, transfers, and...
- `stripe-pp-cli balance-transactions get-id` — <p>Retrieves the balance transaction with the given ID.</p> <p>Note that this endpoint previously used the path...

**billing** — Manage billing

- `stripe-pp-cli billing get-alerts` — <p>Lists billing active and inactive alerts</p>
- `stripe-pp-cli billing get-alerts-id` — <p>Retrieves a billing alert given an ID</p>
- `stripe-pp-cli billing get-credit-balance-summary` — <p>Retrieves the credit balance summary for a customer.</p>
- `stripe-pp-cli billing get-credit-balance-transactions` — <p>Retrieve a list of credit balance transactions.</p>
- `stripe-pp-cli billing get-credit-balance-transactions-id` — <p>Retrieves a credit balance transaction.</p>
- `stripe-pp-cli billing get-credit-grants` — <p>Retrieve a list of credit grants.</p>
- `stripe-pp-cli billing get-credit-grants-id` — <p>Retrieves a credit grant.</p>
- `stripe-pp-cli billing get-meters` — <p>Retrieve a list of billing meters.</p>
- `stripe-pp-cli billing get-meters-id` — <p>Retrieves a billing meter given an ID.</p>
- `stripe-pp-cli billing get-meters-id-event-summaries` — <p>Retrieve a list of billing meter event summaries.</p>
- `stripe-pp-cli billing post-alerts` — <p>Creates a billing alert</p>
- `stripe-pp-cli billing post-alerts-id-activate` — <p>Reactivates this alert, allowing it to trigger again.</p>
- `stripe-pp-cli billing post-alerts-id-archive` — <p>Archives this alert, removing it from the list view and APIs. This is non-reversible.</p>
- `stripe-pp-cli billing post-alerts-id-deactivate` — <p>Deactivates this alert, preventing it from triggering.</p>
- `stripe-pp-cli billing post-credit-grants` — <p>Creates a credit grant.</p>
- `stripe-pp-cli billing post-credit-grants-id` — <p>Updates a credit grant.</p>
- `stripe-pp-cli billing post-credit-grants-id-expire` — <p>Expires a credit grant.</p>
- `stripe-pp-cli billing post-credit-grants-id-void` — <p>Voids a credit grant.</p>
- `stripe-pp-cli billing post-meter-event-adjustments` — <p>Creates a billing meter event adjustment.</p>
- `stripe-pp-cli billing post-meter-events` — <p>Creates a billing meter event.</p>
- `stripe-pp-cli billing post-meters` — <p>Creates a billing meter.</p>
- `stripe-pp-cli billing post-meters-id` — <p>Updates a billing meter.</p>
- `stripe-pp-cli billing post-meters-id-deactivate` — <p>When a meter is deactivated, no more meter events will be accepted for this meter. You can’t attach a...
- `stripe-pp-cli billing post-meters-id-reactivate` — <p>When a meter is reactivated, events for this meter can be accepted and you can attach the meter to a price.</p>

**billing-portal** — Manage billing portal

- `stripe-pp-cli billing-portal get-configurations` — <p>Returns a list of configurations that describe the functionality of the customer portal.</p>
- `stripe-pp-cli billing-portal get-configurations-configuration` — <p>Retrieves a configuration that describes the functionality of the customer portal.</p>
- `stripe-pp-cli billing-portal post-configurations` — <p>Creates a configuration that describes the functionality and behavior of a PortalSession</p>
- `stripe-pp-cli billing-portal post-configurations-configuration` — <p>Updates a configuration that describes the functionality of the customer portal.</p>
- `stripe-pp-cli billing-portal post-sessions` — <p>Creates a session of the customer portal.</p>

**charges** — Manage charges

- `stripe-pp-cli charges get` — <p>Returns a list of charges you’ve previously created. The charges are returned in sorted order, with the most...
- `stripe-pp-cli charges get-charge` — <p>Retrieves the details of a charge that has previously been created. Supply the unique charge ID that was returned...
- `stripe-pp-cli charges get-search` — <p>Search for charges you’ve previously created using Stripe’s <a...
- `stripe-pp-cli charges post` — <p>This method is no longer recommended—use the <a href='/docs/api/payment_intents'>Payment Intents API</a> to...
- `stripe-pp-cli charges post-charge` — <p>Updates the specified charge by setting the values of the parameters passed. Any parameters not provided will be...

**checkout** — Manage checkout

- `stripe-pp-cli checkout get-sessions` — <p>Returns a list of Checkout Sessions.</p>
- `stripe-pp-cli checkout get-sessions-session` — <p>Retrieves a Checkout Session object.</p>
- `stripe-pp-cli checkout get-sessions-session-line-items` — <p>When retrieving a Checkout Session, there is an includable <strong>line_items</strong> property containing the...
- `stripe-pp-cli checkout post-sessions` — <p>Creates a Checkout Session object.</p>
- `stripe-pp-cli checkout post-sessions-session` — <p>Updates a Checkout Session object.</p> <p>Related guide: <a href='/payments/advanced/dynamic-updates'>Dynamically...
- `stripe-pp-cli checkout post-sessions-session-expire` — <p>A Checkout Session can be expired when it is in one of these statuses: <code>open</code> </p> <p>After it...

**climate** — Manage climate

- `stripe-pp-cli climate get-orders` — <p>Lists all Climate order objects. The orders are returned sorted by creation date, with the most recently created...
- `stripe-pp-cli climate get-orders-order` — <p>Retrieves the details of a Climate order object with the given ID.</p>
- `stripe-pp-cli climate get-products` — <p>Lists all available Climate product objects.</p>
- `stripe-pp-cli climate get-products-product` — <p>Retrieves the details of a Climate product with the given ID.</p>
- `stripe-pp-cli climate get-suppliers` — <p>Lists all available Climate supplier objects.</p>
- `stripe-pp-cli climate get-suppliers-supplier` — <p>Retrieves a Climate supplier object.</p>
- `stripe-pp-cli climate post-orders` — <p>Creates a Climate order object for a given Climate product. The order will be processed immediately after...
- `stripe-pp-cli climate post-orders-order` — <p>Updates the specified order by setting the values of the parameters passed.</p>
- `stripe-pp-cli climate post-orders-order-cancel` — <p>Cancels a Climate order. You can cancel an order within 24 hours of creation. Stripe refunds the reservation...

**confirmation-tokens** — Manage confirmation tokens

- `stripe-pp-cli confirmation-tokens <confirmation_token>` — <p>Retrieves an existing ConfirmationToken object</p>

**country-specs** — Manage country specs

- `stripe-pp-cli country-specs get` — <p>Lists all Country Spec objects available in the API.</p>
- `stripe-pp-cli country-specs get-country` — <p>Returns a Country Spec for a given Country code.</p>

**coupons** — Manage coupons

- `stripe-pp-cli coupons delete` — <p>You can delete coupons via the <a href='https://dashboard.stripe.com/coupons'>coupon management</a> page of the...
- `stripe-pp-cli coupons get` — <p>Returns a list of your coupons.</p>
- `stripe-pp-cli coupons get-coupon` — <p>Retrieves the coupon with the given ID.</p>
- `stripe-pp-cli coupons post` — <p>You can create coupons easily via the <a href='https://dashboard.stripe.com/coupons'>coupon management</a> page...
- `stripe-pp-cli coupons post-coupon` — <p>Updates the metadata of a coupon. Other coupon details (currency, duration, amount_off) are, by design, not...

**credit-notes** — Manage credit notes

- `stripe-pp-cli credit-notes get` — <p>Returns a list of credit notes.</p>
- `stripe-pp-cli credit-notes get-id` — <p>Retrieves the credit note object with the given identifier.</p>
- `stripe-pp-cli credit-notes get-preview` — <p>Get a preview of a credit note without creating it.</p>
- `stripe-pp-cli credit-notes get-preview-lines` — <p>When retrieving a credit note preview, you’ll get a <strong>lines</strong> property containing the first...
- `stripe-pp-cli credit-notes post` — <p>Issue a credit note to adjust the amount of a finalized invoice. A credit note will first reduce the invoice’s...
- `stripe-pp-cli credit-notes post-id` — <p>Updates an existing credit note.</p>

**customer-sessions** — Manage customer sessions

- `stripe-pp-cli customer-sessions` — <p>Creates a Customer Session object that includes a single-use client secret that you can use on your front-end to...

**customers** — Manage customers

- `stripe-pp-cli customers delete` — <p>Permanently deletes a customer. It cannot be undone. Also immediately cancels any active subscriptions on the...
- `stripe-pp-cli customers get` — <p>Returns a list of your customers. The customers are returned sorted by creation date, with the most recent...
- `stripe-pp-cli customers get-customer` — <p>Retrieves a Customer object.</p>
- `stripe-pp-cli customers get-search` — <p>Search for customers you’ve previously created using Stripe’s <a...
- `stripe-pp-cli customers post` — <p>Creates a new customer object.</p>
- `stripe-pp-cli customers post-customer` — <p>Updates the specified customer by setting the values of the parameters passed. Any parameters not provided are...

**disputes** — Manage disputes

- `stripe-pp-cli disputes get` — <p>Returns a list of your disputes.</p>
- `stripe-pp-cli disputes get-dispute` — <p>Retrieves the dispute with the given ID.</p>
- `stripe-pp-cli disputes post` — <p>When you get a dispute, contacting your customer is always the best first step. If that doesn’t work, you can...

**entitlements** — Manage entitlements

- `stripe-pp-cli entitlements get-active` — <p>Retrieve a list of active entitlements for a customer</p>
- `stripe-pp-cli entitlements get-active-id` — <p>Retrieve an active entitlement</p>
- `stripe-pp-cli entitlements get-features` — <p>Retrieve a list of features</p>
- `stripe-pp-cli entitlements get-features-id` — <p>Retrieves a feature</p>
- `stripe-pp-cli entitlements post-features` — <p>Creates a feature</p>
- `stripe-pp-cli entitlements post-features-id` — <p>Update a feature’s metadata or permanently deactivate it.</p>

**ephemeral-keys** — Manage ephemeral keys

- `stripe-pp-cli ephemeral-keys delete-key` — <p>Invalidates a short-lived API key for a given resource.</p>
- `stripe-pp-cli ephemeral-keys post` — <p>Creates a short-lived API key for a given resource.</p>

**events** — Manage events

- `stripe-pp-cli events get` — <p>List events, going back up to 30 days. Each event data is rendered according to Stripe API version at its...
- `stripe-pp-cli events get-id` — <p>Retrieves the details of an event if it was created in the last 30 days. Supply the unique identifier of the...

**exchange-rates** — Manage exchange rates

- `stripe-pp-cli exchange-rates get` — <p>[Deprecated] The <code>ExchangeRate</code> APIs are deprecated. Please use the <a...
- `stripe-pp-cli exchange-rates get-rate-id` — <p>[Deprecated] The <code>ExchangeRate</code> APIs are deprecated. Please use the <a...

**file-links** — Manage file links

- `stripe-pp-cli file-links get` — <p>Returns a list of file links.</p>
- `stripe-pp-cli file-links get-link` — <p>Retrieves the file link with the given ID.</p>
- `stripe-pp-cli file-links post` — <p>Creates a new file link object.</p>
- `stripe-pp-cli file-links post-link` — <p>Updates an existing file link object. Expired links can no longer be updated.</p>

**files** — Manage files

- `stripe-pp-cli files get` — <p>Returns a list of the files that your account has access to. Stripe sorts and returns the files by their creation...
- `stripe-pp-cli files get-file` — <p>Retrieves the details of an existing file object. After you supply a unique file ID, Stripe returns the...
- `stripe-pp-cli files post` — <p>To upload a file to Stripe, you need to send a request of type <code>multipart/form-data</code>. Include the file...

**financial-connections** — Manage financial connections

- `stripe-pp-cli financial-connections get-accounts` — <p>Returns a list of Financial Connections <code>Account</code> objects.</p>
- `stripe-pp-cli financial-connections get-accounts-account` — <p>Retrieves the details of an Financial Connections <code>Account</code>.</p>
- `stripe-pp-cli financial-connections get-accounts-account-owners` — <p>Lists all owners for a given <code>Account</code></p>
- `stripe-pp-cli financial-connections get-sessions-session` — <p>Retrieves the details of a Financial Connections <code>Session</code></p>
- `stripe-pp-cli financial-connections get-transactions` — <p>Returns a list of Financial Connections <code>Transaction</code> objects.</p>
- `stripe-pp-cli financial-connections get-transactions-transaction` — <p>Retrieves the details of a Financial Connections <code>Transaction</code></p>
- `stripe-pp-cli financial-connections post-accounts-account-disconnect` — <p>Disables your access to a Financial Connections <code>Account</code>. You will no longer be able to access data...
- `stripe-pp-cli financial-connections post-accounts-account-refresh` — <p>Refreshes the data associated with a Financial Connections <code>Account</code>.</p>
- `stripe-pp-cli financial-connections post-accounts-account-subscribe` — <p>Subscribes to periodic refreshes of data associated with a Financial Connections <code>Account</code>. When the...
- `stripe-pp-cli financial-connections post-accounts-account-unsubscribe` — <p>Unsubscribes from periodic refreshes of data associated with a Financial Connections <code>Account</code>.</p>
- `stripe-pp-cli financial-connections post-sessions` — <p>To launch the Financial Connections authorization flow, create a <code>Session</code>. The session’s...

**forwarding** — Manage forwarding

- `stripe-pp-cli forwarding get-requests` — <p>Lists all ForwardingRequest objects.</p>
- `stripe-pp-cli forwarding get-requests-id` — <p>Retrieves a ForwardingRequest object.</p>
- `stripe-pp-cli forwarding post-requests` — <p>Creates a ForwardingRequest object.</p>

**identity** — Manage identity

- `stripe-pp-cli identity get-verification-reports` — <p>List all verification reports.</p>
- `stripe-pp-cli identity get-verification-reports-report` — <p>Retrieves an existing VerificationReport</p>
- `stripe-pp-cli identity get-verification-sessions` — <p>Returns a list of VerificationSessions</p>
- `stripe-pp-cli identity get-verification-sessions-session` — <p>Retrieves the details of a VerificationSession that was previously created.</p> <p>When the session status is...
- `stripe-pp-cli identity post-verification-sessions` — <p>Creates a VerificationSession object.</p> <p>After the VerificationSession is created, display a verification...
- `stripe-pp-cli identity post-verification-sessions-session` — <p>Updates a VerificationSession object.</p> <p>When the session status is <code>requires_input</code>, you can use...
- `stripe-pp-cli identity post-verification-sessions-session-cancel` — <p>A VerificationSession object can be canceled when it is in <code>requires_input</code> <a...
- `stripe-pp-cli identity post-verification-sessions-session-redact` — <p>Redact a VerificationSession to remove all collected information from Stripe. This will redact the...

**invoice-payments** — Manage invoice payments

- `stripe-pp-cli invoice-payments get` — <p>When retrieving an invoice, there is an includable payments property containing the first handful of those items....
- `stripe-pp-cli invoice-payments get-invoicepayments` — <p>Retrieves the invoice payment with the given ID.</p>

**invoice-rendering-templates** — Manage invoice rendering templates

- `stripe-pp-cli invoice-rendering-templates get` — <p>List all templates, ordered by creation date, with the most recently created template appearing first.</p>
- `stripe-pp-cli invoice-rendering-templates get-template` — <p>Retrieves an invoice rendering template with the given ID. It by default returns the latest version of the...

**invoiceitems** — Manage invoiceitems

- `stripe-pp-cli invoiceitems delete` — <p>Deletes an invoice item, removing it from an invoice. Deleting invoice items is only possible when they’re not...
- `stripe-pp-cli invoiceitems get` — <p>Returns a list of your invoice items. Invoice items are returned sorted by creation date, with the most recently...
- `stripe-pp-cli invoiceitems get-invoiceitem` — <p>Retrieves the invoice item with the given ID.</p>
- `stripe-pp-cli invoiceitems post` — <p>Creates an item to be added to a draft invoice (up to 250 items per invoice). If no invoice is specified, the...
- `stripe-pp-cli invoiceitems post-invoiceitem` — <p>Updates the amount or description of an invoice item on an upcoming invoice. Updating an invoice item is only...

**invoices** — Manage invoices

- `stripe-pp-cli invoices delete` — <p>Permanently deletes a one-off invoice draft. This cannot be undone. Attempts to delete invoices that are no...
- `stripe-pp-cli invoices get` — <p>You can list all invoices, or list the invoices for a specific customer. The invoices are returned sorted by...
- `stripe-pp-cli invoices get-invoice` — <p>Retrieves the invoice with the given ID.</p>
- `stripe-pp-cli invoices get-search` — <p>Search for invoices you’ve previously created using Stripe’s <a...
- `stripe-pp-cli invoices post` — <p>This endpoint creates a draft invoice for a given customer. The invoice remains a draft until you <a...
- `stripe-pp-cli invoices post-create-preview` — <p>At any time, you can preview the upcoming invoice for a subscription or subscription schedule. This will show you...
- `stripe-pp-cli invoices post-invoice` — <p>Draft invoices are fully editable. Once an invoice is <a href='/docs/billing/invoices/workflow#finalized'>finalize...

**issuing** — Manage issuing

- `stripe-pp-cli issuing get-authorizations` — <p>Returns a list of Issuing <code>Authorization</code> objects. The objects are sorted in descending order by...
- `stripe-pp-cli issuing get-authorizations-authorization` — <p>Retrieves an Issuing <code>Authorization</code> object.</p>
- `stripe-pp-cli issuing get-cardholders` — <p>Returns a list of Issuing <code>Cardholder</code> objects. The objects are sorted in descending order by creation...
- `stripe-pp-cli issuing get-cardholders-cardholder` — <p>Retrieves an Issuing <code>Cardholder</code> object.</p>
- `stripe-pp-cli issuing get-cards` — <p>Returns a list of Issuing <code>Card</code> objects. The objects are sorted in descending order by creation date,...
- `stripe-pp-cli issuing get-cards-card` — <p>Retrieves an Issuing <code>Card</code> object.</p>
- `stripe-pp-cli issuing get-disputes` — <p>Returns a list of Issuing <code>Dispute</code> objects. The objects are sorted in descending order by creation...
- `stripe-pp-cli issuing get-disputes-dispute` — <p>Retrieves an Issuing <code>Dispute</code> object.</p>
- `stripe-pp-cli issuing get-personalization-designs` — <p>Returns a list of personalization design objects. The objects are sorted in descending order by creation date,...
- `stripe-pp-cli issuing get-personalization-designs-personalization-design` — <p>Retrieves a personalization design object.</p>
- `stripe-pp-cli issuing get-physical-bundles` — <p>Returns a list of physical bundle objects. The objects are sorted in descending order by creation date, with the...
- `stripe-pp-cli issuing get-physical-bundles-physical-bundle` — <p>Retrieves a physical bundle object.</p>
- `stripe-pp-cli issuing get-tokens` — <p>Lists all Issuing <code>Token</code> objects for a given card.</p>
- `stripe-pp-cli issuing get-tokens-token` — <p>Retrieves an Issuing <code>Token</code> object.</p>
- `stripe-pp-cli issuing get-transactions` — <p>Returns a list of Issuing <code>Transaction</code> objects. The objects are sorted in descending order by...
- `stripe-pp-cli issuing get-transactions-transaction` — <p>Retrieves an Issuing <code>Transaction</code> object.</p>
- `stripe-pp-cli issuing post-authorizations-authorization` — <p>Updates the specified Issuing <code>Authorization</code> object by setting the values of the parameters passed....
- `stripe-pp-cli issuing post-authorizations-authorization-approve` — <p>[Deprecated] Approves a pending Issuing <code>Authorization</code> object. This request should be made within the...
- `stripe-pp-cli issuing post-authorizations-authorization-decline` — <p>[Deprecated] Declines a pending Issuing <code>Authorization</code> object. This request should be made within the...
- `stripe-pp-cli issuing post-cardholders` — <p>Creates a new Issuing <code>Cardholder</code> object that can be issued cards.</p>
- `stripe-pp-cli issuing post-cardholders-cardholder` — <p>Updates the specified Issuing <code>Cardholder</code> object by setting the values of the parameters passed. Any...
- `stripe-pp-cli issuing post-cards` — <p>Creates an Issuing <code>Card</code> object.</p>
- `stripe-pp-cli issuing post-cards-card` — <p>Updates the specified Issuing <code>Card</code> object by setting the values of the parameters passed. Any...
- `stripe-pp-cli issuing post-disputes` — <p>Creates an Issuing <code>Dispute</code> object. Individual pieces of evidence within the <code>evidence</code>...
- `stripe-pp-cli issuing post-disputes-dispute` — <p>Updates the specified Issuing <code>Dispute</code> object by setting the values of the parameters passed. Any...
- `stripe-pp-cli issuing post-disputes-dispute-submit` — <p>Submits an Issuing <code>Dispute</code> to the card network. Stripe validates that all evidence fields required...
- `stripe-pp-cli issuing post-personalization-designs` — <p>Creates a personalization design object.</p>
- `stripe-pp-cli issuing post-personalization-designs-personalization-design` — <p>Updates a card personalization object.</p>
- `stripe-pp-cli issuing post-tokens-token` — <p>Attempts to update the specified Issuing <code>Token</code> object to the status specified.</p>
- `stripe-pp-cli issuing post-transactions-transaction` — <p>Updates the specified Issuing <code>Transaction</code> object by setting the values of the parameters passed. Any...

**mandates** — Manage mandates

- `stripe-pp-cli mandates` — <p>Retrieves a Mandate object.</p>

**payment-attempt-records** — Manage payment attempt records

- `stripe-pp-cli payment-attempt-records get` — <p>List all the Payment Attempt Records attached to the specified Payment Record.</p>
- `stripe-pp-cli payment-attempt-records get-id` — <p>Retrieves a Payment Attempt Record with the given ID</p>

**payment-intents** — Manage payment intents

- `stripe-pp-cli payment-intents get` — <p>Returns a list of PaymentIntents.</p>
- `stripe-pp-cli payment-intents get-intent` — <p>Retrieves the details of a PaymentIntent that has previously been created. </p> <p>You can retrieve a...
- `stripe-pp-cli payment-intents get-search` — <p>Search for PaymentIntents you’ve previously created using Stripe’s <a...
- `stripe-pp-cli payment-intents post` — <p>Creates a PaymentIntent object.</p> <p>After the PaymentIntent is created, attach a payment method and <a...
- `stripe-pp-cli payment-intents post-intent` — <p>Updates properties on a PaymentIntent object without confirming.</p> <p>Depending on which properties you update,...

**payment-links** — Manage payment links

- `stripe-pp-cli payment-links get` — <p>Returns a list of your payment links.</p>
- `stripe-pp-cli payment-links get-paymentlinks` — <p>Retrieve a payment link.</p>
- `stripe-pp-cli payment-links post` — <p>Creates a payment link.</p>
- `stripe-pp-cli payment-links post-paymentlinks` — <p>Updates a payment link.</p>

**payment-method-configurations** — Manage payment method configurations

- `stripe-pp-cli payment-method-configurations get` — <p>List payment method configurations</p>
- `stripe-pp-cli payment-method-configurations get-configuration` — <p>Retrieve payment method configuration</p>
- `stripe-pp-cli payment-method-configurations post` — <p>Creates a payment method configuration</p>
- `stripe-pp-cli payment-method-configurations post-configuration` — <p>Update payment method configuration</p>

**payment-method-domains** — Manage payment method domains

- `stripe-pp-cli payment-method-domains get` — <p>Lists the details of existing payment method domains.</p>
- `stripe-pp-cli payment-method-domains get-paymentmethoddomains` — <p>Retrieves the details of an existing payment method domain.</p>
- `stripe-pp-cli payment-method-domains post` — <p>Creates a payment method domain.</p>
- `stripe-pp-cli payment-method-domains post-paymentmethoddomains` — <p>Updates an existing payment method domain.</p>

**payment-methods** — Manage payment methods

- `stripe-pp-cli payment-methods get` — <p>Returns a list of all PaymentMethods.</p>
- `stripe-pp-cli payment-methods get-paymentmethods` — <p>Retrieves a PaymentMethod object attached to the StripeAccount. To retrieve a payment method attached to a...
- `stripe-pp-cli payment-methods post` — <p>Creates a PaymentMethod object. Read the <a href='/docs/stripe-js/reference#stripe-create-payment-method'>Stripe.j...
- `stripe-pp-cli payment-methods post-paymentmethods` — <p>Updates a PaymentMethod object. A PaymentMethod must be attached to a customer to be updated.</p>

**payment-records** — Manage payment records

- `stripe-pp-cli payment-records get-id` — <p>Retrieves a Payment Record with the given ID</p>
- `stripe-pp-cli payment-records post-report-payment` — <p>Report a new Payment Record. You may report a Payment Record as it is initialized and later report updates...

**payouts** — Manage payouts

- `stripe-pp-cli payouts get` — <p>Returns a list of existing payouts sent to third-party bank accounts or payouts that Stripe sent to you. The...
- `stripe-pp-cli payouts get-payout` — <p>Retrieves the details of an existing payout. Supply the unique payout ID from either a payout creation request or...
- `stripe-pp-cli payouts post` — <p>To send funds to your own bank account, create a new payout object. Your <a href='#balance'>Stripe balance</a>...
- `stripe-pp-cli payouts post-payout` — <p>Updates the specified payout by setting the values of the parameters you pass. We don’t change parameters that...

**plans** — Manage plans

- `stripe-pp-cli plans delete` — <p>Deleting plans means new subscribers can’t be added. Existing subscribers aren’t affected.</p>
- `stripe-pp-cli plans get` — <p>Returns a list of your plans.</p>
- `stripe-pp-cli plans get-plan` — <p>Retrieves the plan with the given ID.</p>
- `stripe-pp-cli plans post` — <p>You can now model subscriptions more flexibly using the <a href='#prices'>Prices API</a>. It replaces the Plans...
- `stripe-pp-cli plans post-plan` — <p>Updates the specified plan by setting the values of the parameters passed. Any parameters not provided are left...

**prices** — Manage prices

- `stripe-pp-cli prices get` — <p>Returns a list of your active prices, excluding <a href='/docs/products-prices/pricing-models#inline-pricing'>inli...
- `stripe-pp-cli prices get-price` — <p>Retrieves the price with the given ID.</p>
- `stripe-pp-cli prices get-search` — <p>Search for prices you’ve previously created using Stripe’s <a...
- `stripe-pp-cli prices post` — <p>Creates a new <a href='https://docs.stripe.com/api/prices'>Price</a> for an existing <a...
- `stripe-pp-cli prices post-price` — <p>Updates the specified price by setting the values of the parameters passed. Any parameters not provided are left...

**products** — Manage products

- `stripe-pp-cli products delete-id` — <p>Delete a product. Deleting a product is only possible if it has no prices associated with it. Additionally,...
- `stripe-pp-cli products get` — <p>Returns a list of your products. The products are returned sorted by creation date, with the most recently...
- `stripe-pp-cli products get-id` — <p>Retrieves the details of an existing product. Supply the unique product ID from either a product creation request...
- `stripe-pp-cli products get-search` — <p>Search for products you’ve previously created using Stripe’s <a...
- `stripe-pp-cli products post` — <p>Creates a new product object.</p>
- `stripe-pp-cli products post-id` — <p>Updates the specific product by setting the values of the parameters passed. Any parameters not provided will be...

**promotion-codes** — Manage promotion codes

- `stripe-pp-cli promotion-codes get` — <p>Returns a list of your promotion codes.</p>
- `stripe-pp-cli promotion-codes get-promotioncodes` — <p>Retrieves the promotion code with the given ID. In order to retrieve a promotion code by the customer-facing...
- `stripe-pp-cli promotion-codes post` — <p>A promotion code points to an underlying promotion. You can optionally restrict the code to a specific customer,...
- `stripe-pp-cli promotion-codes post-promotioncodes` — <p>Updates the specified promotion code by setting the values of the parameters passed. Most fields are, by design,...

**quotes** — Manage quotes

- `stripe-pp-cli quotes get` — <p>Returns a list of your quotes.</p>
- `stripe-pp-cli quotes get-quote` — <p>Retrieves the quote with the given ID.</p>
- `stripe-pp-cli quotes post` — <p>A quote models prices and services for a customer. Default options for <code>header</code>,...
- `stripe-pp-cli quotes post-quote` — <p>A quote models prices and services for a customer.</p>

**radar** — Manage radar

- `stripe-pp-cli radar delete-value-list-items-item` — <p>Deletes a <code>ValueListItem</code> object, removing it from its parent value list.</p>
- `stripe-pp-cli radar delete-value-lists-value-list` — <p>Deletes a <code>ValueList</code> object, also deleting any items contained within the value list. To be deleted,...
- `stripe-pp-cli radar get-early-fraud-warnings` — <p>Returns a list of early fraud warnings.</p>
- `stripe-pp-cli radar get-early-fraud-warnings-early-fraud-warning` — <p>Retrieves the details of an early fraud warning that has previously been created. </p> <p>Please refer to the <a...
- `stripe-pp-cli radar get-value-list-items` — <p>Returns a list of <code>ValueListItem</code> objects. The objects are sorted in descending order by creation...
- `stripe-pp-cli radar get-value-list-items-item` — <p>Retrieves a <code>ValueListItem</code> object.</p>
- `stripe-pp-cli radar get-value-lists` — <p>Returns a list of <code>ValueList</code> objects. The objects are sorted in descending order by creation date,...
- `stripe-pp-cli radar get-value-lists-value-list` — <p>Retrieves a <code>ValueList</code> object.</p>
- `stripe-pp-cli radar post-payment-evaluations` — <p>Request a Radar API fraud risk score from Stripe for a payment before sending it for external processor...
- `stripe-pp-cli radar post-value-list-items` — <p>Creates a new <code>ValueListItem</code> object, which is added to the specified parent value list.</p>
- `stripe-pp-cli radar post-value-lists` — <p>Creates a new <code>ValueList</code> object, which can then be referenced in rules.</p>
- `stripe-pp-cli radar post-value-lists-value-list` — <p>Updates a <code>ValueList</code> object by setting the values of the parameters passed. Any parameters not...

**refunds** — Manage refunds

- `stripe-pp-cli refunds get` — <p>Returns a list of all refunds you created. We return the refunds in sorted order, with the most recent refunds...
- `stripe-pp-cli refunds get-refund` — <p>Retrieves the details of an existing refund.</p>
- `stripe-pp-cli refunds post` — <p>When you create a new refund, you must specify a Charge or a PaymentIntent object on which to create it.</p>...
- `stripe-pp-cli refunds post-refund` — <p>Updates the refund that you specify by setting the values of the passed parameters. Any parameters that you...

**reporting** — Manage reporting

- `stripe-pp-cli reporting get-report-runs` — <p>Returns a list of Report Runs, with the most recent appearing first.</p>
- `stripe-pp-cli reporting get-report-runs-report-run` — <p>Retrieves the details of an existing Report Run.</p>
- `stripe-pp-cli reporting get-report-types` — <p>Returns a full list of Report Types.</p>
- `stripe-pp-cli reporting get-report-types-report-type` — <p>Retrieves the details of a Report Type. (Certain report types require a <a...
- `stripe-pp-cli reporting post-report-runs` — <p>Creates a new object and begin running the report. (Certain report types require a <a...

**reviews** — Manage reviews

- `stripe-pp-cli reviews get` — <p>Returns a list of <code>Review</code> objects that have <code>open</code> set to <code>true</code>. The objects...
- `stripe-pp-cli reviews get-review` — <p>Retrieves a <code>Review</code> object.</p>

**setup-attempts** — Manage setup attempts

- `stripe-pp-cli setup-attempts` — <p>Returns a list of SetupAttempts that associate with a provided SetupIntent.</p>

**setup-intents** — Manage setup intents

- `stripe-pp-cli setup-intents get` — <p>Returns a list of SetupIntents.</p>
- `stripe-pp-cli setup-intents get-intent` — <p>Retrieves the details of a SetupIntent that has previously been created. </p> <p>Client-side retrieval using a...
- `stripe-pp-cli setup-intents post` — <p>Creates a SetupIntent object.</p> <p>After you create the SetupIntent, attach a payment method and <a...
- `stripe-pp-cli setup-intents post-intent` — <p>Updates a SetupIntent object.</p>

**shipping-rates** — Manage shipping rates

- `stripe-pp-cli shipping-rates get` — <p>Returns a list of your shipping rates.</p>
- `stripe-pp-cli shipping-rates get-token` — <p>Returns the shipping rate object with the given ID.</p>
- `stripe-pp-cli shipping-rates post` — <p>Creates a new shipping rate object.</p>
- `stripe-pp-cli shipping-rates post-token` — <p>Updates an existing shipping rate object.</p>

**sigma** — Manage sigma

- `stripe-pp-cli sigma get-scheduled-query-runs` — <p>Returns a list of scheduled query runs.</p>
- `stripe-pp-cli sigma get-scheduled-query-runs-scheduled-query-run` — <p>Retrieves the details of an scheduled query run.</p>

**sources** — Manage sources

- `stripe-pp-cli sources get` — <p>Retrieves an existing source object. Supply the unique source ID from a source creation request and Stripe will...
- `stripe-pp-cli sources post` — <p>Creates a new source object.</p>
- `stripe-pp-cli sources post-source` — <p>Updates the specified source by setting the values of the parameters passed. Any parameters not provided will be...

**subscription-items** — Manage subscription items

- `stripe-pp-cli subscription-items delete-item` — <p>Deletes an item from the subscription. Removing a subscription item from a subscription will not cancel the...
- `stripe-pp-cli subscription-items get` — <p>Returns a list of your subscription items for a given subscription.</p>
- `stripe-pp-cli subscription-items get-item` — <p>Retrieves the subscription item with the given ID.</p>
- `stripe-pp-cli subscription-items post` — <p>Adds a new item to an existing subscription. No existing items will be changed or replaced.</p>
- `stripe-pp-cli subscription-items post-item` — <p>Updates the plan or quantity of an item on a current subscription.</p>

**subscription-schedules** — Manage subscription schedules

- `stripe-pp-cli subscription-schedules get` — <p>Retrieves the list of your subscription schedules.</p>
- `stripe-pp-cli subscription-schedules get-schedule` — <p>Retrieves the details of an existing subscription schedule. You only need to supply the unique subscription...
- `stripe-pp-cli subscription-schedules post` — <p>Creates a new subscription schedule object. Each customer can have up to 500 active or scheduled subscriptions.</p>
- `stripe-pp-cli subscription-schedules post-schedule` — <p>Updates an existing subscription schedule.</p>

**subscriptions** — Manage subscriptions

- `stripe-pp-cli subscriptions delete-exposed-id` — <p>Cancels a customer’s subscription immediately. The customer won’t be charged again for the subscription....
- `stripe-pp-cli subscriptions get` — <p>By default, returns a list of subscriptions that have not been canceled. In order to list canceled subscriptions,...
- `stripe-pp-cli subscriptions get-exposed-id` — <p>Retrieves the subscription with the given ID.</p>
- `stripe-pp-cli subscriptions get-search` — <p>Search for subscriptions you’ve previously created using Stripe’s <a...
- `stripe-pp-cli subscriptions post` — <p>Creates a new subscription on an existing customer. Each customer can have up to 500 active or scheduled...
- `stripe-pp-cli subscriptions post-exposed-id` — <p>Updates an existing subscription to match the specified parameters. When changing prices or quantities, we...

**tax** — Manage tax

- `stripe-pp-cli tax get-associations-find` — <p>Finds a tax association object by PaymentIntent id.</p>
- `stripe-pp-cli tax get-calculations-calculation` — <p>Retrieves a Tax <code>Calculation</code> object, if the calculation hasn’t expired.</p>
- `stripe-pp-cli tax get-calculations-calculation-line-items` — <p>Retrieves the line items of a tax calculation as a collection, if the calculation hasn’t expired.</p>
- `stripe-pp-cli tax get-registrations` — <p>Returns a list of Tax <code>Registration</code> objects.</p>
- `stripe-pp-cli tax get-registrations-id` — <p>Returns a Tax <code>Registration</code> object.</p>
- `stripe-pp-cli tax get-settings` — <p>Retrieves Tax <code>Settings</code> for a merchant.</p>
- `stripe-pp-cli tax get-transactions-transaction` — <p>Retrieves a Tax <code>Transaction</code> object.</p>
- `stripe-pp-cli tax get-transactions-transaction-line-items` — <p>Retrieves the line items of a committed standalone transaction as a collection.</p>
- `stripe-pp-cli tax post-calculations` — <p>Calculates tax based on the input and returns a Tax <code>Calculation</code> object.</p>
- `stripe-pp-cli tax post-registrations` — <p>Creates a new Tax <code>Registration</code> object.</p>
- `stripe-pp-cli tax post-registrations-id` — <p>Updates an existing Tax <code>Registration</code> object.</p> <p>A registration cannot be deleted after it has...
- `stripe-pp-cli tax post-settings` — <p>Updates Tax <code>Settings</code> parameters used in tax calculations. All parameters are editable but none can...
- `stripe-pp-cli tax post-transactions-create-from-calculation` — <p>Creates a Tax Transaction from a calculation, if that calculation hasn’t expired. Calculations expire after 90...
- `stripe-pp-cli tax post-transactions-create-reversal` — <p>Partially or fully reverses a previously created <code>Transaction</code>.</p>

**tax-codes** — Manage tax codes

- `stripe-pp-cli tax-codes get` — <p>A list of <a href='https://stripe.com/docs/tax/tax-categories'>all tax codes available</a> to add to Products in...
- `stripe-pp-cli tax-codes get-id` — <p>Retrieves the details of an existing tax code. Supply the unique tax code ID and Stripe will return the...

**tax-ids** — Manage tax ids

- `stripe-pp-cli tax-ids delete-id` — <p>Deletes an existing account or customer <code>tax_id</code> object.</p>
- `stripe-pp-cli tax-ids get` — <p>Returns a list of tax IDs.</p>
- `stripe-pp-cli tax-ids get-id` — <p>Retrieves an account or customer <code>tax_id</code> object.</p>
- `stripe-pp-cli tax-ids post` — <p>Creates a new account or customer <code>tax_id</code> object.</p>

**tax-rates** — Manage tax rates

- `stripe-pp-cli tax-rates get` — <p>Returns a list of your tax rates. Tax rates are returned sorted by creation date, with the most recently created...
- `stripe-pp-cli tax-rates get-taxrates` — <p>Retrieves a tax rate with the given ID</p>
- `stripe-pp-cli tax-rates post` — <p>Creates a new tax rate.</p>
- `stripe-pp-cli tax-rates post-taxrates` — <p>Updates an existing tax rate.</p>

**terminal** — Manage terminal

- `stripe-pp-cli terminal delete-configurations-configuration` — <p>Deletes a <code>Configuration</code> object.</p>
- `stripe-pp-cli terminal delete-locations-location` — <p>Deletes a <code>Location</code> object.</p>
- `stripe-pp-cli terminal delete-readers-reader` — <p>Deletes a <code>Reader</code> object.</p>
- `stripe-pp-cli terminal get-configurations` — <p>Returns a list of <code>Configuration</code> objects.</p>
- `stripe-pp-cli terminal get-configurations-configuration` — <p>Retrieves a <code>Configuration</code> object.</p>
- `stripe-pp-cli terminal get-locations` — <p>Returns a list of <code>Location</code> objects.</p>
- `stripe-pp-cli terminal get-locations-location` — <p>Retrieves a <code>Location</code> object.</p>
- `stripe-pp-cli terminal get-readers` — <p>Returns a list of <code>Reader</code> objects.</p>
- `stripe-pp-cli terminal get-readers-reader` — <p>Retrieves a <code>Reader</code> object.</p>
- `stripe-pp-cli terminal post-configurations` — <p>Creates a new <code>Configuration</code> object.</p>
- `stripe-pp-cli terminal post-configurations-configuration` — <p>Updates a new <code>Configuration</code> object.</p>
- `stripe-pp-cli terminal post-connection-tokens` — <p>To connect to a reader the Stripe Terminal SDK needs to retrieve a short-lived connection token from Stripe,...
- `stripe-pp-cli terminal post-locations` — <p>Creates a new <code>Location</code> object. For further details, including which address fields are required in...
- `stripe-pp-cli terminal post-locations-location` — <p>Updates a <code>Location</code> object by setting the values of the parameters passed. Any parameters not...
- `stripe-pp-cli terminal post-onboarding-links` — <p>Creates a new <code>OnboardingLink</code> object that contains a redirect_url used for onboarding onto Tap to Pay...
- `stripe-pp-cli terminal post-readers` — <p>Creates a new <code>Reader</code> object.</p>
- `stripe-pp-cli terminal post-readers-reader` — <p>Updates a <code>Reader</code> object by setting the values of the parameters passed. Any parameters not provided...
- `stripe-pp-cli terminal post-readers-reader-cancel-action` — <p>Cancels the current reader action. See <a href='/docs/terminal/payments/collect-card-payment?terminal-sdk-platform...
- `stripe-pp-cli terminal post-readers-reader-collect-inputs` — <p>Initiates an <a href='/docs/terminal/features/collect-inputs'>input collection flow</a> on a Reader to display...
- `stripe-pp-cli terminal post-readers-reader-collect-payment-method` — <p>Initiates a payment flow on a Reader and updates the PaymentIntent with card details before manual confirmation....
- `stripe-pp-cli terminal post-readers-reader-confirm-payment-intent` — <p>Finalizes a payment on a Reader. See <a href='/docs/terminal/payments/collect-card-payment?terminal-sdk-platform=s...
- `stripe-pp-cli terminal post-readers-reader-process-payment-intent` — <p>Initiates a payment flow on a Reader. See <a href='/docs/terminal/payments/collect-card-payment?terminal-sdk-platf...
- `stripe-pp-cli terminal post-readers-reader-process-setup-intent` — <p>Initiates a SetupIntent flow on a Reader. See <a href='/docs/terminal/features/saving-payment-details/save-directl...
- `stripe-pp-cli terminal post-readers-reader-refund-payment` — <p>Initiates an in-person refund on a Reader. See <a href='/docs/terminal/payments/regional?integration-country=CA#re...
- `stripe-pp-cli terminal post-readers-reader-set-reader-display` — <p>Sets the reader display to show <a href='/docs/terminal/features/display'>cart details</a>.</p>

**test-helpers** — Manage test helpers

- `stripe-pp-cli test-helpers delete-test-clocks-test-clock` — <p>Deletes a test clock.</p>
- `stripe-pp-cli test-helpers get-test-clocks` — <p>Returns a list of your test clocks.</p>
- `stripe-pp-cli test-helpers get-test-clocks-test-clock` — <p>Retrieves a test clock.</p>
- `stripe-pp-cli test-helpers post-confirmation-tokens` — <p>Creates a test mode Confirmation Token server side for your integration tests.</p>
- `stripe-pp-cli test-helpers post-customers-customer-fund-cash-balance` — <p>Create an incoming testmode bank transfer</p>
- `stripe-pp-cli test-helpers post-issuing-authorizations` — <p>Create a test-mode authorization.</p>
- `stripe-pp-cli test-helpers post-issuing-authorizations-authorization-capture` — <p>Capture a test-mode authorization.</p>
- `stripe-pp-cli test-helpers post-issuing-authorizations-authorization-expire` — <p>Expire a test-mode Authorization.</p>
- `stripe-pp-cli test-helpers post-issuing-authorizations-authorization-finalize-amount` — <p>Finalize the amount on an Authorization prior to capture, when the initial authorization was for an estimated...
- `stripe-pp-cli test-helpers post-issuing-authorizations-authorization-fraud-challenges-respond` — <p>Respond to a fraud challenge on a testmode Issuing authorization, simulating either a confirmation of fraud or a...
- `stripe-pp-cli test-helpers post-issuing-authorizations-authorization-increment` — <p>Increment a test-mode Authorization.</p>
- `stripe-pp-cli test-helpers post-issuing-authorizations-authorization-reverse` — <p>Reverse a test-mode Authorization.</p>
- `stripe-pp-cli test-helpers post-issuing-cards-card-shipping-deliver` — <p>Updates the shipping status of the specified Issuing <code>Card</code> object to <code>delivered</code>.</p>
- `stripe-pp-cli test-helpers post-issuing-cards-card-shipping-fail` — <p>Updates the shipping status of the specified Issuing <code>Card</code> object to <code>failure</code>.</p>
- `stripe-pp-cli test-helpers post-issuing-cards-card-shipping-return` — <p>Updates the shipping status of the specified Issuing <code>Card</code> object to <code>returned</code>.</p>
- `stripe-pp-cli test-helpers post-issuing-cards-card-shipping-ship` — <p>Updates the shipping status of the specified Issuing <code>Card</code> object to <code>shipped</code>.</p>
- `stripe-pp-cli test-helpers post-issuing-cards-card-shipping-submit` — <p>Updates the shipping status of the specified Issuing <code>Card</code> object to <code>submitted</code>. This...
- `stripe-pp-cli test-helpers post-issuing-personalization-designs-personalization-design-activate` — <p>Updates the <code>status</code> of the specified testmode personalization design object to <code>active</code>.</p>
- `stripe-pp-cli test-helpers post-issuing-personalization-designs-personalization-design-deactivate` — <p>Updates the <code>status</code> of the specified testmode personalization design object to <code>inactive</code>.</p>
- `stripe-pp-cli test-helpers post-issuing-personalization-designs-personalization-design-reject` — <p>Updates the <code>status</code> of the specified testmode personalization design object to <code>rejected</code>.</p>
- `stripe-pp-cli test-helpers post-issuing-transactions-create-force-capture` — <p>Allows the user to capture an arbitrary amount, also known as a forced capture.</p>
- `stripe-pp-cli test-helpers post-issuing-transactions-create-unlinked-refund` — <p>Allows the user to refund an arbitrary amount, also known as a unlinked refund.</p>
- `stripe-pp-cli test-helpers post-issuing-transactions-transaction-refund` — <p>Refund a test-mode Transaction.</p>
- `stripe-pp-cli test-helpers post-refunds-refund-expire` — <p>Expire a refund with a status of <code>requires_action</code>.</p>
- `stripe-pp-cli test-helpers post-terminal-readers-reader-present-payment-method` — <p>Presents a payment method on a simulated reader. Can be used to simulate accepting a payment, saving a card or...
- `stripe-pp-cli test-helpers post-terminal-readers-reader-succeed-input-collection` — <p>Use this endpoint to trigger a successful input collection on a simulated reader.</p>
- `stripe-pp-cli test-helpers post-terminal-readers-reader-timeout-input-collection` — <p>Use this endpoint to complete an input collection with a timeout error on a simulated reader.</p>
- `stripe-pp-cli test-helpers post-test-clocks` — <p>Creates a new test clock that can be attached to new customers and quotes.</p>
- `stripe-pp-cli test-helpers post-test-clocks-test-clock-advance` — <p>Starts advancing a test clock to a specified time in the future. Advancement is done when status changes to...
- `stripe-pp-cli test-helpers post-treasury-inbound-transfers-id-fail` — <p>Transitions a test mode created InboundTransfer to the <code>failed</code> status. The InboundTransfer must...
- `stripe-pp-cli test-helpers post-treasury-inbound-transfers-id-return` — <p>Marks the test mode InboundTransfer object as returned and links the InboundTransfer to a ReceivedDebit. The...
- `stripe-pp-cli test-helpers post-treasury-inbound-transfers-id-succeed` — <p>Transitions a test mode created InboundTransfer to the <code>succeeded</code> status. The InboundTransfer must...
- `stripe-pp-cli test-helpers post-treasury-outbound-payments-id` — <p>Updates a test mode created OutboundPayment with tracking details. The OutboundPayment must not be cancelable,...
- `stripe-pp-cli test-helpers post-treasury-outbound-payments-id-fail` — <p>Transitions a test mode created OutboundPayment to the <code>failed</code> status. The OutboundPayment must...
- `stripe-pp-cli test-helpers post-treasury-outbound-payments-id-post` — <p>Transitions a test mode created OutboundPayment to the <code>posted</code> status. The OutboundPayment must...
- `stripe-pp-cli test-helpers post-treasury-outbound-payments-id-return` — <p>Transitions a test mode created OutboundPayment to the <code>returned</code> status. The OutboundPayment must...
- `stripe-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer` — <p>Updates a test mode created OutboundTransfer with tracking details. The OutboundTransfer must not be cancelable,...
- `stripe-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer-fail` — <p>Transitions a test mode created OutboundTransfer to the <code>failed</code> status. The OutboundTransfer must...
- `stripe-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer-post` — <p>Transitions a test mode created OutboundTransfer to the <code>posted</code> status. The OutboundTransfer must...
- `stripe-pp-cli test-helpers post-treasury-outbound-transfers-outbound-transfer-return` — <p>Transitions a test mode created OutboundTransfer to the <code>returned</code> status. The OutboundTransfer must...
- `stripe-pp-cli test-helpers post-treasury-received-credits` — <p>Use this endpoint to simulate a test mode ReceivedCredit initiated by a third party. In live mode, you can’t...
- `stripe-pp-cli test-helpers post-treasury-received-debits` — <p>Use this endpoint to simulate a test mode ReceivedDebit initiated by a third party. In live mode, you can’t...

**tokens** — Manage tokens

- `stripe-pp-cli tokens get` — <p>Retrieves the token with the given ID.</p>
- `stripe-pp-cli tokens post` — <p>Creates a single-use token that represents a bank account’s details. You can use this token with any v1 API...

**topups** — Manage topups

- `stripe-pp-cli topups get` — <p>Returns a list of top-ups.</p>
- `stripe-pp-cli topups get-topup` — <p>Retrieves the details of a top-up that has previously been created. Supply the unique top-up ID that was returned...
- `stripe-pp-cli topups post` — <p>Top up the balance of an account</p>
- `stripe-pp-cli topups post-topup` — <p>Updates the metadata of a top-up. Other top-up details are not editable by design.</p>

**transfers** — Manage transfers

- `stripe-pp-cli transfers get` — <p>Returns a list of existing transfers sent to connected accounts. The transfers are returned in sorted order, with...
- `stripe-pp-cli transfers get-transfer` — <p>Retrieves the details of an existing transfer. Supply the unique transfer ID from either a transfer creation...
- `stripe-pp-cli transfers post` — <p>To send funds from your Stripe account to a connected account, you create a new transfer object. Your <a...
- `stripe-pp-cli transfers post-transfer` — <p>Updates the specified transfer by setting the values of the parameters passed. Any parameters not provided will...

**treasury** — Manage treasury

- `stripe-pp-cli treasury get-credit-reversals` — <p>Returns a list of CreditReversals.</p>
- `stripe-pp-cli treasury get-credit-reversals-credit-reversal` — <p>Retrieves the details of an existing CreditReversal by passing the unique CreditReversal ID from either the...
- `stripe-pp-cli treasury get-debit-reversals` — <p>Returns a list of DebitReversals.</p>
- `stripe-pp-cli treasury get-debit-reversals-debit-reversal` — <p>Retrieves a DebitReversal object.</p>
- `stripe-pp-cli treasury get-financial-accounts` — <p>Returns a list of FinancialAccounts.</p>
- `stripe-pp-cli treasury get-financial-accounts-financial-account` — <p>Retrieves the details of a FinancialAccount.</p>
- `stripe-pp-cli treasury get-financial-accounts-financial-account-features` — <p>Retrieves Features information associated with the FinancialAccount.</p>
- `stripe-pp-cli treasury get-inbound-transfers` — <p>Returns a list of InboundTransfers sent from the specified FinancialAccount.</p>
- `stripe-pp-cli treasury get-inbound-transfers-id` — <p>Retrieves the details of an existing InboundTransfer.</p>
- `stripe-pp-cli treasury get-outbound-payments` — <p>Returns a list of OutboundPayments sent from the specified FinancialAccount.</p>
- `stripe-pp-cli treasury get-outbound-payments-id` — <p>Retrieves the details of an existing OutboundPayment by passing the unique OutboundPayment ID from either the...
- `stripe-pp-cli treasury get-outbound-transfers` — <p>Returns a list of OutboundTransfers sent from the specified FinancialAccount.</p>
- `stripe-pp-cli treasury get-outbound-transfers-outbound-transfer` — <p>Retrieves the details of an existing OutboundTransfer by passing the unique OutboundTransfer ID from either the...
- `stripe-pp-cli treasury get-received-credits` — <p>Returns a list of ReceivedCredits.</p>
- `stripe-pp-cli treasury get-received-credits-id` — <p>Retrieves the details of an existing ReceivedCredit by passing the unique ReceivedCredit ID from the...
- `stripe-pp-cli treasury get-received-debits` — <p>Returns a list of ReceivedDebits.</p>
- `stripe-pp-cli treasury get-received-debits-id` — <p>Retrieves the details of an existing ReceivedDebit by passing the unique ReceivedDebit ID from the ReceivedDebit...
- `stripe-pp-cli treasury get-transaction-entries` — <p>Retrieves a list of TransactionEntry objects.</p>
- `stripe-pp-cli treasury get-transaction-entries-id` — <p>Retrieves a TransactionEntry object.</p>
- `stripe-pp-cli treasury get-transactions` — <p>Retrieves a list of Transaction objects.</p>
- `stripe-pp-cli treasury get-transactions-id` — <p>Retrieves the details of an existing Transaction.</p>
- `stripe-pp-cli treasury post-credit-reversals` — <p>Reverses a ReceivedCredit and creates a CreditReversal object.</p>
- `stripe-pp-cli treasury post-debit-reversals` — <p>Reverses a ReceivedDebit and creates a DebitReversal object.</p>
- `stripe-pp-cli treasury post-financial-accounts` — <p>Creates a new FinancialAccount. Each connected account can have up to three FinancialAccounts by default.</p>
- `stripe-pp-cli treasury post-financial-accounts-financial-account` — <p>Updates the details of a FinancialAccount.</p>
- `stripe-pp-cli treasury post-financial-accounts-financial-account-close` — <p>Closes a FinancialAccount. A FinancialAccount can only be closed if it has a zero balance, has no pending...
- `stripe-pp-cli treasury post-financial-accounts-financial-account-features` — <p>Updates the Features associated with a FinancialAccount.</p>
- `stripe-pp-cli treasury post-inbound-transfers` — <p>Creates an InboundTransfer.</p>
- `stripe-pp-cli treasury post-inbound-transfers-inbound-transfer-cancel` — <p>Cancels an InboundTransfer.</p>
- `stripe-pp-cli treasury post-outbound-payments` — <p>Creates an OutboundPayment.</p>
- `stripe-pp-cli treasury post-outbound-payments-id-cancel` — <p>Cancel an OutboundPayment.</p>
- `stripe-pp-cli treasury post-outbound-transfers` — <p>Creates an OutboundTransfer.</p>
- `stripe-pp-cli treasury post-outbound-transfers-outbound-transfer-cancel` — <p>An OutboundTransfer can be canceled if the funds have not yet been paid out.</p>

**webhook-endpoints** — Manage webhook endpoints

- `stripe-pp-cli webhook-endpoints delete` — <p>You can also delete webhook endpoints via the <a href='https://dashboard.stripe.com/account/webhooks'>webhook...
- `stripe-pp-cli webhook-endpoints get` — <p>Returns a list of your webhook endpoints.</p>
- `stripe-pp-cli webhook-endpoints get-webhookendpoints` — <p>Retrieves the webhook endpoint with the given ID.</p>
- `stripe-pp-cli webhook-endpoints post` — <p>A webhook endpoint must have a <code>url</code> and a list of <code>enabled_events</code>. You may optionally...
- `stripe-pp-cli webhook-endpoints post-webhookendpoints` — <p>Updates the webhook endpoint. You may edit the <code>url</code>, the list of <code>enabled_events</code>, and the...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
stripe-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find double-subscribed customers

```bash
stripe-pp-cli sql 'SELECT json_extract(s.data,"$.customer") AS cid, COUNT(*) AS subs FROM resources s WHERE s.resource_type="subscriptions" AND json_extract(s.data,"$.status")="active" GROUP BY cid HAVING subs > 1' --json
```

All Stripe data lives in the generic `resources` table; query JSON fields with json_extract.

### Top 10 customers with at-risk PM

```bash
stripe-pp-cli subs-at-risk --within 30d --limit 10 --json --select customer_email,mrr,card_exp
```

Email these customers proactively before involuntary churn hits.

### Replay missed events since last cron tick

```bash
stripe-pp-cli events-since --type 'invoice' --json --select id,type,created,data.object.id
```

Cursor-driven event tail — one-shot, agent-friendly, persists cursor between runs.

### Reconcile this week's payouts

```bash
stripe-pp-cli payout-reconcile --since 7d --csv
```

Joins payout → balance_transactions → charges → customers; flags missing rows.

### Compact customer dossier for agent context

```bash
stripe-pp-cli customer-360 alice@example.com --json --select customer.email,active_subscriptions,open_disputes,lifetime_spend
```

Use --select to project deeply-nested fields; saves tokens vs full dossier.

## Auth Setup

Authenticate by exporting `STRIPE_SECRET_KEY=sk_test_...` (recommended) or running `stripe-pp-cli auth set-token <key>` to persist it. Test-mode keys (`sk_test_...`) and live-mode keys (`sk_live_...`) are accepted; this v1 does NOT yet enforce a live-mode write guard, so audit any live invocation before running mutating commands.

Run `stripe-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  stripe-pp-cli account --agent --select id,name,status
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
stripe-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
stripe-pp-cli feedback --stdin < notes.txt
stripe-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.stripe-pp-cli/feedback.jsonl`. They are never POSTed unless `STRIPE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `STRIPE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
stripe-pp-cli profile save briefing --json
stripe-pp-cli --profile briefing account
stripe-pp-cli profile list --json
stripe-pp-cli profile show briefing
stripe-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `stripe-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/stripe/cmd/stripe-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add stripe-pp-mcp -- stripe-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which stripe-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   stripe-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `stripe-pp-cli <command> --help`.
