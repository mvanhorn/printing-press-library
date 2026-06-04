---
name: pp-mailchimp
description: "Every Mailchimp endpoint plus the workflow commands the API forces you to compose yourself. Trigger phrases: `subscribe to mailchimp`, `add a contact to mailchimp`, `send a mailchimp campaign`, `mailchimp campaign performance`, `import contacts to mailchimp`, `audit mailchimp segments`, `mailchimp deliverability`, `use mailchimp`, `run mailchimp`."
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - mailchimp-pp-cli
---

# Mailchimp — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `mailchimp-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install mailchimp --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `mailchimp-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/mailchimp/cmd/mailchimp-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI when you need to drive Mailchimp from a terminal or agent and the workflow spans more than one endpoint: subscribing a contact with tags, importing a CSV through the batch endpoint, measuring whether a campaign worked, or auditing segment hygiene. The local SQLite cache makes the audience SQL-queryable, which is impossible through the dashboard or the SDK. The MCP surface uses code orchestration over 291 endpoints in ~1K tokens, so an agent can reach any Mailchimp endpoint without bloating its tool list.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Workflow composition
- **`subscribe`** — Upsert a member by email and apply tags in a single command. Auto-computes the MD5 subscriber hash so you never see it.

  _When an agent needs to add a contact with tags, this is one call instead of three with MD5 hashing in the middle._

  ```bash
  mailchimp-pp-cli subscribe alice@example.com --list a1b2c3d4e5 --tags vip,onboarding --merge FNAME=Alice
  ```
- **`bulk-subscribe`** — Read a CSV, fan out through Mailchimp's /batches endpoint (the official 10-concurrent-connection escape hatch), poll until done, decode the tar.gz of JSONL results within the 10-minute response URL window, and print per-row outcomes.

  _When an agent needs to subscribe more than ~50 contacts at once, this is the only safe path that respects rate limits and gives back actionable per-row results._

  ```bash
  mailchimp-pp-cli bulk-subscribe --csv contacts.csv --list a1b2c3d4e5 --tags newsletter --watch
  ```

### Local joins that compound
- **`digest`** — Single-campaign mode (digest <id>) joins report + email-activity + ecommerce-product-activity into one summary with opens/clicks/bounces/revenue + top-clicked links + top-converted products. Rollup mode (digest --last N or --week) renders a multi-campaign summary table with aggregate stats. --md renders either as paste-ready markdown.

  _Use digest <id> for per-campaign analysis; use digest --last N for a weekly rollup. --md is the paste-into-doc shape both personas need._

  ```bash
  mailchimp-pp-cli digest --last 5 --md
  ```
- **`segments audit`** — Find empty segments, segments that haven't grown in 90 days, and segments not referenced by any recent campaign. Reads the segments endpoint and joins with member counts from the local SQLite store.

  _When cleaning up an audience or auditing segmentation hygiene, this surfaces what's dead in one call._

  ```bash
  mailchimp-pp-cli segments audit --list a1b2c3d4e5 --json
  ```
- **`attribution`** — Join a campaign's e-commerce product activity report with synced store orders to compute attributed revenue, top products by attributed revenue, and conversion rate (orders divided by opens).

  _When measuring whether a campaign drove revenue, this is the single command._

  ```bash
  mailchimp-pp-cli attribution 7f8a9b0c1d --store mystore --json
  ```
- **`deliverability`** — Roll up domain-performance reports across the last N campaigns. Surfaces per-domain (gmail.com, yahoo.com, outlook.com) bounce, spam, open, and click rates. Highlights domains performing below the account average.

  _When investigating a deliverability dip, this shows you which inbox providers are degrading without clicking through 10 separate report pages._

  ```bash
  mailchimp-pp-cli deliverability --last 10 --json
  ```
- **`compare`** — Side-by-side metric diff for two campaigns. Fetches both campaigns' report + email-activity in parallel, renders open rate / CTR / click-to-open / bounces / unsubscribes / revenue with a winner per metric, computes delta, and surfaces top differences (subject line, send time, audience size). --md renders as paste-ready markdown.

  _When you ask 'which campaign won?' or 'did the change work?', this is the single command. The --md mode pastes into a weekly review doc._

  ```bash
  mailchimp-pp-cli compare 7f8a9b0c1d 8c9d0e1f2a --md
  ```

### Operational safety
- **`send-checked`** — Run the official send-checklist before sending. Exits 2 with the failing items if any have type=error or is_ready=false; otherwise sends.

  _When sending campaigns from a pipeline, this is the safe default — broken campaigns don't ship silently._

  ```bash
  mailchimp-pp-cli send-checked 7f8a9b0c1d
  ```

## Command Reference

**account-exports** — Manage account exports

- `mailchimp-pp-cli account-exports get` — Get a list of account exports for a given account.
- `mailchimp-pp-cli account-exports get-id` — Get information about a specific account export.
- `mailchimp-pp-cli account-exports post` — Create a new account export in your Mailchimp account.

**activity-feed** — Manage activity feed

- `mailchimp-pp-cli activity-feed` — Return the Chimp Chatter for this account ordered by most recent.

**audiences** — Manage audiences

- `mailchimp-pp-cli audiences get-contacts` — Get information about all audiences in the account.
- `mailchimp-pp-cli audiences get-id` — Get information about a specific audience.

**authorized-apps** — Manage authorized apps

- `mailchimp-pp-cli authorized-apps get` — Get a list of an account's registered, connected applications.
- `mailchimp-pp-cli authorized-apps get-id` — Get information about a specific authorized application.

**automations** — Manage automations

- `mailchimp-pp-cli automations get` — Get a summary of an account's classic automations.
- `mailchimp-pp-cli automations get-id` — Get a summary of an individual classic automation workflow's settings and content. The `trigger_settings` object...
- `mailchimp-pp-cli automations post` — Create a new classic automation in your Mailchimp account.

**batch-webhooks** — Manage batch webhooks

- `mailchimp-pp-cli batch-webhooks delete-id` — Remove a batch webhook. Webhooks will no longer be sent to the given URL.
- `mailchimp-pp-cli batch-webhooks get` — Get all webhooks that have been configured for batches.
- `mailchimp-pp-cli batch-webhooks get-batchwebhooks` — Get information about a specific batch webhook.
- `mailchimp-pp-cli batch-webhooks patch` — Update a webhook that will fire whenever any batch request completes processing.
- `mailchimp-pp-cli batch-webhooks post` — Configure a webhook that will fire whenever any batch request completes processing. You may only have a maximum of...

**batches** — Manage batches

- `mailchimp-pp-cli batches delete-id` — Stops a batch request from running. Since only one batch request is run at a time, this can be used to cancel a long...
- `mailchimp-pp-cli batches get` — Get a summary of batch requests that have been made.
- `mailchimp-pp-cli batches get-id` — Get the status of a batch request.
- `mailchimp-pp-cli batches post` — Begin processing a batch operations request.

**campaign-folders** — Manage campaign folders

- `mailchimp-pp-cli campaign-folders delete-id` — Delete a specific campaign folder, and mark all the campaigns in the folder as 'unfiled'.
- `mailchimp-pp-cli campaign-folders get` — Get all folders used to organize campaigns.
- `mailchimp-pp-cli campaign-folders get-id` — Get information about a specific folder used to organize campaigns.
- `mailchimp-pp-cli campaign-folders patch-id` — Update a specific folder used to organize campaigns.
- `mailchimp-pp-cli campaign-folders post` — Create a new campaign folder.

**campaigns** — Manage campaigns

- `mailchimp-pp-cli campaigns delete-id` — Remove a campaign from your Mailchimp account.
- `mailchimp-pp-cli campaigns get` — Get all campaigns in an account.
- `mailchimp-pp-cli campaigns get-id` — Get information about a specific campaign.
- `mailchimp-pp-cli campaigns patch-id` — Update some or all of the settings for a specific campaign.
- `mailchimp-pp-cli campaigns post` — Create a new Mailchimp campaign.

**connected-sites** — Manage connected sites

- `mailchimp-pp-cli connected-sites delete-id` — Remove a connected site from your Mailchimp account.
- `mailchimp-pp-cli connected-sites get` — Get all connected sites in an account.
- `mailchimp-pp-cli connected-sites get-id` — Get information about a specific connected site.
- `mailchimp-pp-cli connected-sites post` — Create a new Mailchimp connected site.

**conversations** — Manage conversations

- `mailchimp-pp-cli conversations get` — Get a list of conversations for the account. Conversations has been deprecated in favor of Inbox and these endpoints...
- `mailchimp-pp-cli conversations get-id` — Get details about an individual conversation. Conversations has been deprecated in favor of Inbox and these...

**customer-journeys** — Manage customer journeys

- `mailchimp-pp-cli customer-journeys <journey_id> <step_id>` — A step trigger in an Automation flow. To use it, create a starting point or step from the Automation flow builder in...

**ecommerce** — Manage ecommerce

- `mailchimp-pp-cli ecommerce delete-stores-id` — Delete a store. Deleting a store will also delete any associated subresources, including Customers, Orders,...
- `mailchimp-pp-cli ecommerce delete-stores-id-carts-id` — Delete a cart.
- `mailchimp-pp-cli ecommerce delete-stores-id-carts-lines-id` — Delete a specific cart line item.
- `mailchimp-pp-cli ecommerce delete-stores-id-customers-id` — Delete a customer from a store.
- `mailchimp-pp-cli ecommerce delete-stores-id-orders-id` — Delete an order.
- `mailchimp-pp-cli ecommerce delete-stores-id-orders-id-lines-id` — Delete a specific order line item.
- `mailchimp-pp-cli ecommerce delete-stores-id-products-id` — Delete a product.
- `mailchimp-pp-cli ecommerce delete-stores-id-products-id-images-id` — Delete a product image.
- `mailchimp-pp-cli ecommerce delete-stores-id-products-id-variants-id` — Delete a product variant.
- `mailchimp-pp-cli ecommerce delete-stores-id-promocodes-id` — Delete a promo code from a store.
- `mailchimp-pp-cli ecommerce delete-stores-id-promorules-id` — Delete a promo rule from a store.
- `mailchimp-pp-cli ecommerce get-orders` — Get information about an account's orders.
- `mailchimp-pp-cli ecommerce get-stores` — Get information about all stores in the account.
- `mailchimp-pp-cli ecommerce get-stores-id` — Get information about a specific store.
- `mailchimp-pp-cli ecommerce get-stores-id-carts` — Get information about a store's carts.
- `mailchimp-pp-cli ecommerce get-stores-id-carts-id` — Get information about a specific cart.
- `mailchimp-pp-cli ecommerce get-stores-id-carts-id-lines` — Get information about a cart's line items.
- `mailchimp-pp-cli ecommerce get-stores-id-carts-id-lines-id` — Get information about a specific cart line item.
- `mailchimp-pp-cli ecommerce get-stores-id-customers` — Get information about a store's customers.
- `mailchimp-pp-cli ecommerce get-stores-id-customers-id` — Get information about a specific customer.
- `mailchimp-pp-cli ecommerce get-stores-id-orders` — Get information about a store's orders.
- `mailchimp-pp-cli ecommerce get-stores-id-orders-id` — Get information about a specific order.
- `mailchimp-pp-cli ecommerce get-stores-id-orders-id-lines` — Get information about an order's line items.
- `mailchimp-pp-cli ecommerce get-stores-id-orders-id-lines-id` — Get information about a specific order line item.
- `mailchimp-pp-cli ecommerce get-stores-id-products` — Get information about a store's products.
- `mailchimp-pp-cli ecommerce get-stores-id-products-id` — Get information about a specific product.
- `mailchimp-pp-cli ecommerce get-stores-id-products-id-images` — Get information about a product's images.
- `mailchimp-pp-cli ecommerce get-stores-id-products-id-images-id` — Get information about a specific product image.
- `mailchimp-pp-cli ecommerce get-stores-id-products-id-variants` — Get information about a product's variants.
- `mailchimp-pp-cli ecommerce get-stores-id-products-id-variants-id` — Get information about a specific product variant.
- `mailchimp-pp-cli ecommerce get-stores-id-promocodes` — Get information about a store's promo codes.
- `mailchimp-pp-cli ecommerce get-stores-id-promocodes-id` — Get information about a specific promo code.
- `mailchimp-pp-cli ecommerce get-stores-id-promorules` — Get information about a store's promo rules.
- `mailchimp-pp-cli ecommerce get-stores-id-promorules-id` — Get information about a specific promo rule.
- `mailchimp-pp-cli ecommerce patch-stores-id` — Update a store.
- `mailchimp-pp-cli ecommerce patch-stores-id-carts-id` — Update a specific cart.
- `mailchimp-pp-cli ecommerce patch-stores-id-carts-id-lines-id` — Update a specific cart line item.
- `mailchimp-pp-cli ecommerce patch-stores-id-customers-id` — Update a customer.
- `mailchimp-pp-cli ecommerce patch-stores-id-orders-id` — Update a specific order.
- `mailchimp-pp-cli ecommerce patch-stores-id-orders-id-lines-id` — Update a specific order line item.
- `mailchimp-pp-cli ecommerce patch-stores-id-products-id` — Update a specific product.
- `mailchimp-pp-cli ecommerce patch-stores-id-products-id-images-id` — Update a product image.
- `mailchimp-pp-cli ecommerce patch-stores-id-products-id-variants-id` — Update a product variant.
- `mailchimp-pp-cli ecommerce patch-stores-id-promocodes-id` — Update a promo code.
- `mailchimp-pp-cli ecommerce patch-stores-id-promorules-id` — Update a promo rule.
- `mailchimp-pp-cli ecommerce post-stores` — Add a new store to your Mailchimp account.
- `mailchimp-pp-cli ecommerce post-stores-id-carts` — Add a new cart to a store.
- `mailchimp-pp-cli ecommerce post-stores-id-carts-id-lines` — Add a new line item to an existing cart.
- `mailchimp-pp-cli ecommerce post-stores-id-customers` — Add a new customer to a store.
- `mailchimp-pp-cli ecommerce post-stores-id-orders` — Add a new order to a store.
- `mailchimp-pp-cli ecommerce post-stores-id-orders-id-lines` — Add a new line item to an existing order.
- `mailchimp-pp-cli ecommerce post-stores-id-products` — Add a new product to a store.
- `mailchimp-pp-cli ecommerce post-stores-id-products-id-images` — Add a new image to the product.
- `mailchimp-pp-cli ecommerce post-stores-id-products-id-variants` — Add a new variant to the product.
- `mailchimp-pp-cli ecommerce post-stores-id-promocodes` — Add a new promo code to a store.
- `mailchimp-pp-cli ecommerce post-stores-id-promorules` — Add a new promo rule to a store.
- `mailchimp-pp-cli ecommerce put-stores-id-customers-id` — Add or update a customer.
- `mailchimp-pp-cli ecommerce put-stores-id-orders-id` — Add or update an order.
- `mailchimp-pp-cli ecommerce put-stores-id-products-id` — Update a specific product.
- `mailchimp-pp-cli ecommerce put-stores-id-products-id-variants-id` — Add or update a product variant.

**facebook-ads** — Manage facebook ads

- `mailchimp-pp-cli facebook-ads get-all` — Get list of Facebook ads.
- `mailchimp-pp-cli facebook-ads get-id` — Get details of a Facebook ad.

**file-manager** — Manage file manager

- `mailchimp-pp-cli file-manager delete-files-id` — Remove a specific file from the File Manager.
- `mailchimp-pp-cli file-manager delete-folders-id` — Delete a specific folder in the File Manager.
- `mailchimp-pp-cli file-manager get-files` — Get a list of available images and files stored in the File Manager for the account.
- `mailchimp-pp-cli file-manager get-files-id` — Get information about a specific file in the File Manager.
- `mailchimp-pp-cli file-manager get-folders` — Get a list of all folders in the File Manager.
- `mailchimp-pp-cli file-manager get-folders-files` — Get a list of available images and files stored in this folder.
- `mailchimp-pp-cli file-manager get-folders-id` — Get information about a specific folder in the File Manager.
- `mailchimp-pp-cli file-manager patch-files-id` — Update a file in the File Manager.
- `mailchimp-pp-cli file-manager patch-folders-id` — Update a specific File Manager folder.
- `mailchimp-pp-cli file-manager post-files` — Upload a new image or file to the File Manager.
- `mailchimp-pp-cli file-manager post-folders` — Create a new folder in the File Manager.

**landing-pages** — Manage landing pages

- `mailchimp-pp-cli landing-pages delete-id` — Delete a landing page.
- `mailchimp-pp-cli landing-pages get-all` — Get all landing pages.
- `mailchimp-pp-cli landing-pages get-id` — Get information about a specific page.
- `mailchimp-pp-cli landing-pages patch-id` — Update a landing page.
- `mailchimp-pp-cli landing-pages post-all` — Create an unpublished and contentless Mailchimp landing page.

**lists** — Manage lists

- `mailchimp-pp-cli lists delete-id` — Delete a list from your Mailchimp account. If you delete a list, you'll lose the list history—including subscriber...
- `mailchimp-pp-cli lists get` — Get information about all lists in the account.
- `mailchimp-pp-cli lists get-id` — Get information about a specific list in your Mailchimp account. Results include list members who have signed up but...
- `mailchimp-pp-cli lists patch-id` — Update the settings for a specific list.
- `mailchimp-pp-cli lists post` — Create a new list in your Mailchimp account.
- `mailchimp-pp-cli lists post-id` — Batch subscribe or unsubscribe list members.

**ping** — Manage ping

- `mailchimp-pp-cli ping` — A health check for the API that won't return any account-specific information.

**reporting** — Manage reporting

- `mailchimp-pp-cli reporting get-facebook-ads` — Get reports of Facebook ads.
- `mailchimp-pp-cli reporting get-facebook-ads-id` — Get report of a Facebook ad.
- `mailchimp-pp-cli reporting get-facebook-ads-id-ecommerce-product-activity` — Get breakdown of product activity for an outreach.
- `mailchimp-pp-cli reporting get-landing-pages` — Get reports of landing pages.
- `mailchimp-pp-cli reporting get-landing-pages-id` — Get report of a landing page.
- `mailchimp-pp-cli reporting get-surveys` — Get reports for surveys.
- `mailchimp-pp-cli reporting get-surveys-id` — Get report for a survey.
- `mailchimp-pp-cli reporting get-surveys-id-questions` — Get reports for survey questions.
- `mailchimp-pp-cli reporting get-surveys-id-questions-id` — Get report for a survey question.
- `mailchimp-pp-cli reporting get-surveys-id-questions-id-answers` — Get answers for a survey question.
- `mailchimp-pp-cli reporting get-surveys-id-responses` — Get responses to a survey.
- `mailchimp-pp-cli reporting get-surveys-id-responses-id` — Get a single survey response.

**reports** — Manage reports

- `mailchimp-pp-cli reports get` — Get campaign reports.
- `mailchimp-pp-cli reports get-id` — Get report details for a specific sent campaign.

**search-campaigns** — Manage search campaigns

- `mailchimp-pp-cli search-campaigns` — Search all campaigns for the specified query terms.

**search-members** — Manage search members

- `mailchimp-pp-cli search-members` — Search for list members. This search can be restricted to a specific list, or can be used to search across all lists...

**sms-campaigns** — Manage sms campaigns

- `mailchimp-pp-cli sms-campaigns delete-id` — Remove an SMS campaign from your Mailchimp account.
- `mailchimp-pp-cli sms-campaigns get` — Get all SMS campaigns in an account.
- `mailchimp-pp-cli sms-campaigns get-id` — Get information about a specific SMS campaign.
- `mailchimp-pp-cli sms-campaigns patch-id` — Update some or all of the settings for a specific SMS campaign.
- `mailchimp-pp-cli sms-campaigns post` — Create a new SMS campaign.

**template-folders** — Manage template folders

- `mailchimp-pp-cli template-folders delete-id` — Delete a specific template folder, and mark all the templates in the folder as 'unfiled'.
- `mailchimp-pp-cli template-folders get` — Get all folders used to organize templates.
- `mailchimp-pp-cli template-folders get-id` — Get information about a specific folder used to organize templates.
- `mailchimp-pp-cli template-folders patch-id` — Update a specific folder used to organize templates.
- `mailchimp-pp-cli template-folders post` — Create a new template folder.

**templates** — Manage templates

- `mailchimp-pp-cli templates delete-id` — Delete a specific template.
- `mailchimp-pp-cli templates get` — Get a list of an account's available templates.
- `mailchimp-pp-cli templates get-id` — Get information about a specific template.
- `mailchimp-pp-cli templates patch-id` — Update the name, HTML, or `folder_id` of an existing template.
- `mailchimp-pp-cli templates post` — Create a new template for the account. Only Classic templates are supported.

**verified-domains** — Manage verified domains

- `mailchimp-pp-cli verified-domains create` — Add a domain to the account.
- `mailchimp-pp-cli verified-domains delete` — Delete a verified domain from the account.
- `mailchimp-pp-cli verified-domains get` — Get all of the sending domains on the account.
- `mailchimp-pp-cli verified-domains get-verifieddomains` — Get the details for a single domain on the account.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
mailchimp-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Add a subscriber with tags in one call

```bash
mailchimp-pp-cli subscribe alice@example.com --list LIST_ID --tags vip,onboarding --merge FNAME=Alice --merge LNAME=Smith
```

Composes PUT /lists/{id}/members/{md5} + POST /lists/{id}/members/{md5}/tags with the MD5 hash computed for you.

### Import 5,000 contacts from CSV via the batch endpoint

```bash
mailchimp-pp-cli bulk-subscribe --csv contacts.csv --list LIST_ID --tags newsletter --watch
```

Posts to /batches, polls until done, decompresses the tar.gz JSONL within the 10-minute expiring URL window, returns per-row outcomes.

### Check whether a campaign succeeded

```bash
mailchimp-pp-cli digest CAMPAIGN_ID --json --select campaign_id,open_rate,click_rate,top_links,top_products
```

Joins four report endpoints and lets --select dotted paths narrow the response to the fields an agent actually needs (Mailchimp's reports run >10KB each).

### Find stale segments to clean up

```bash
mailchimp-pp-cli segments audit --list LIST_ID --json
```

Surfaces empty segments, segments with no member growth in 90 days, and segments not used by any recent campaign.

### Gate a campaign send on the official checklist

```bash
mailchimp-pp-cli send-checked CAMPAIGN_ID
```

Runs /send-checklist first; if any item is type=error, exits 2 with the failing items printed and does not send. Use in CI to prevent broken campaigns shipping silently.

### Compare two campaigns head-to-head for a Monday review doc

```bash
mailchimp-pp-cli compare CAMPAIGN_A CAMPAIGN_B --md
```

Renders a side-by-side diff of open rate, CTR, revenue, and unsubscribes with the winner per metric. The --md output pastes into Notion or Slack.

### Weekly campaign rollup for a founder review

```bash
mailchimp-pp-cli digest --last 5 --md
```

Renders a multi-campaign summary table with aggregate stats. Marcus's Monday founder doc shape.

## Auth Setup

Mailchimp encodes the datacenter in the API key suffix. The key 'abc...xyz-us6' tells the CLI to route requests to 'us6.api.mailchimp.com.' Set MAILCHIMP_API_KEY and the CLI parses the suffix at startup. For OAuth tokens (no embedded suffix), set MAILCHIMP_DC=us1 explicitly or let 'mailchimp auth dc-lookup' resolve it from the metadata endpoint.

Run `mailchimp-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  mailchimp-pp-cli account-exports get --agent --select id,name,status
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
mailchimp-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
mailchimp-pp-cli feedback --stdin < notes.txt
mailchimp-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.mailchimp-pp-cli/feedback.jsonl`. They are never POSTed unless `MAILCHIMP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MAILCHIMP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
mailchimp-pp-cli profile save briefing --json
mailchimp-pp-cli --profile briefing account-exports get
mailchimp-pp-cli profile list --json
mailchimp-pp-cli profile show briefing
mailchimp-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `mailchimp-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add mailchimp-pp-mcp -- mailchimp-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which mailchimp-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   mailchimp-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `mailchimp-pp-cli <command> --help`.
