# Mailchimp CLI

**Every Mailchimp endpoint plus the workflow commands the API forces you to compose yourself.**

Mailchimp's REST API has 291 endpoints and an SDK in every language, but composes nothing — subscribing a contact with tags takes three calls and an MD5 hash; checking whether a campaign worked takes four endpoints joined by hand; bulk imports require decoding tar.gz JSONL from a 10-minute expiring URL. This CLI ships every endpoint as a typed command plus eight novel workflow commands that compose the API the way humans and agents actually use it: subscribe-with-tags in one call, CSV bulk subscribe with batch decode, single and multi-campaign digests with --md for pasting into weekly review docs, head-to-head campaign comparison, segment health audit, send-checklist CI gate, e-commerce attribution, and per-domain deliverability rollup. A local SQLite cache makes every audience SQL-queryable, and the MCP surface uses code orchestration so an agent loads the whole API in ~1K tokens.

Learn more at [Mailchimp](https://mailchimp.com/developer/marketing/).

Created by [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `mailchimp-pp-cli` binary and the `pp-mailchimp` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install mailchimp
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install mailchimp --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install mailchimp --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install mailchimp --agent claude-code
npx -y @mvanhorn/printing-press-library install mailchimp --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/mailchimp/cmd/mailchimp-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mailchimp-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install mailchimp --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-mailchimp --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-mailchimp --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install mailchimp --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mailchimp-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `MAILCHIMP_API_KEY` (format: `key-dc`, e.g., `abc...-us6`) when Claude Desktop prompts you. The CLI parses the `-dc` suffix to route requests to the correct datacenter.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "mailchimp": {
      "command": "mailchimp-pp-mcp",
      "env": {
        "MAILCHIMP_USERNAME": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Mailchimp encodes the datacenter in the API key suffix. The key 'abc...xyz-us6' tells the CLI to route requests to 'us6.api.mailchimp.com.' Set MAILCHIMP_API_KEY and the CLI parses the suffix at startup. For OAuth tokens (no embedded suffix), set MAILCHIMP_DC=us1 explicitly or let 'mailchimp auth dc-lookup' resolve it from the metadata endpoint.

## Quick Start

```bash
# datacenter encoded in the suffix; the CLI parses it
export MAILCHIMP_API_KEY=your-key-us6

# verify auth, dc routing, and API reach
mailchimp-pp-cli doctor

# pull a local copy so search and sql work offline
mailchimp-pp-cli sync --resources lists,members,campaigns,reports

# one-shot upsert with tags, no MD5 dance
mailchimp-pp-cli subscribe alice@example.com --list LIST_ID --tags vip,newsletter

# the four-endpoint join the dashboard buries
mailchimp-pp-cli digest CAMPAIGN_ID --json --select campaign_id,open_rate,click_rate,top_links

# offline FTS over synced audiences, members, campaigns
mailchimp-pp-cli search 'newsletter' --json

```

## Unique Features

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

## Usage

Run `mailchimp-pp-cli --help` for the full command reference and flag list.

## Commands

### account-exports

Manage account exports

- **`mailchimp-pp-cli account-exports get`** - Get a list of account exports for a given account.
- **`mailchimp-pp-cli account-exports get-id`** - Get information about a specific account export.
- **`mailchimp-pp-cli account-exports post`** - Create a new account export in your Mailchimp account.

### activity-feed

Manage activity feed

- **`mailchimp-pp-cli activity-feed`** - Return the Chimp Chatter for this account ordered by most recent.

### audiences

Manage audiences

- **`mailchimp-pp-cli audiences get-contacts`** - Get information about all audiences in the account.
- **`mailchimp-pp-cli audiences get-id`** - Get information about a specific audience.

### authorized-apps

Manage authorized apps

- **`mailchimp-pp-cli authorized-apps get`** - Get a list of an account's registered, connected applications.
- **`mailchimp-pp-cli authorized-apps get-id`** - Get information about a specific authorized application.

### automations

Manage automations

- **`mailchimp-pp-cli automations get`** - Get a summary of an account's classic automations.
- **`mailchimp-pp-cli automations get-id`** - Get a summary of an individual classic automation workflow's settings and content. The `trigger_settings` object returns information for the first email in the workflow.
- **`mailchimp-pp-cli automations post`** - Create a new classic automation in your Mailchimp account.

### batch-webhooks

Manage batch webhooks

- **`mailchimp-pp-cli batch-webhooks delete-id`** - Remove a batch webhook. Webhooks will no longer be sent to the given URL.
- **`mailchimp-pp-cli batch-webhooks get`** - Get all webhooks that have been configured for batches.
- **`mailchimp-pp-cli batch-webhooks get-batchwebhooks`** - Get information about a specific batch webhook.
- **`mailchimp-pp-cli batch-webhooks patch`** - Update a webhook that will fire whenever any batch request completes processing.
- **`mailchimp-pp-cli batch-webhooks post`** - Configure a webhook that will fire whenever any batch request completes processing.  You may only have a maximum of 20 batch webhooks.

### batches

Manage batches

- **`mailchimp-pp-cli batches delete-id`** - Stops a batch request from running. Since only one batch request is run at a time, this can be used to cancel a long running request. The results of any completed operations will not be available after this call.
- **`mailchimp-pp-cli batches get`** - Get a summary of batch requests that have been made.
- **`mailchimp-pp-cli batches get-id`** - Get the status of a batch request.
- **`mailchimp-pp-cli batches post`** - Begin processing a batch operations request.

### campaign-folders

Manage campaign folders

- **`mailchimp-pp-cli campaign-folders delete-id`** - Delete a specific campaign folder, and mark all the campaigns in the folder as 'unfiled'.
- **`mailchimp-pp-cli campaign-folders get`** - Get all folders used to organize campaigns.
- **`mailchimp-pp-cli campaign-folders get-id`** - Get information about a specific folder used to organize campaigns.
- **`mailchimp-pp-cli campaign-folders patch-id`** - Update a specific folder used to organize campaigns.
- **`mailchimp-pp-cli campaign-folders post`** - Create a new campaign folder.

### campaigns

Manage campaigns

- **`mailchimp-pp-cli campaigns delete-id`** - Remove a campaign from your Mailchimp account.
- **`mailchimp-pp-cli campaigns get`** - Get all campaigns in an account.
- **`mailchimp-pp-cli campaigns get-id`** - Get information about a specific campaign.
- **`mailchimp-pp-cli campaigns patch-id`** - Update some or all of the settings for a specific campaign.
- **`mailchimp-pp-cli campaigns post`** - Create a new Mailchimp campaign.

### connected-sites

Manage connected sites

- **`mailchimp-pp-cli connected-sites delete-id`** - Remove a connected site from your Mailchimp account.
- **`mailchimp-pp-cli connected-sites get`** - Get all connected sites in an account.
- **`mailchimp-pp-cli connected-sites get-id`** - Get information about a specific connected site.
- **`mailchimp-pp-cli connected-sites post`** - Create a new Mailchimp connected site.

### conversations

Manage conversations

- **`mailchimp-pp-cli conversations get`** - Get a list of conversations for the account. Conversations has been deprecated in favor of Inbox and these endpoints don't include Inbox data. Past Conversations are still available via this endpoint, but new campaign replies and other Inbox messages aren’t available using this endpoint.
- **`mailchimp-pp-cli conversations get-id`** - Get details about an individual conversation. Conversations has been deprecated in favor of Inbox and these endpoints don't include Inbox data. Past Conversations are still available via this endpoint, but new campaign replies and other Inbox messages aren’t available using this endpoint.

### customer-journeys

Manage customer journeys

- **`mailchimp-pp-cli customer-journeys <journey_id> <step_id>`** - A step trigger in an Automation flow. To use it, create a starting point or step from the Automation flow builder in the app using the Customer Journey API condition. We’ll provide a url during the process that includes the {journey_id} and {step_id}. You’ll then be able to use this endpoint to trigger the condition for the posted contact.

### ecommerce

Manage ecommerce

- **`mailchimp-pp-cli ecommerce delete-stores-id`** - Delete a store. Deleting a store will also delete any associated subresources, including Customers, Orders, Products, and Carts.
- **`mailchimp-pp-cli ecommerce delete-stores-id-carts-id`** - Delete a cart.
- **`mailchimp-pp-cli ecommerce delete-stores-id-carts-lines-id`** - Delete a specific cart line item.
- **`mailchimp-pp-cli ecommerce delete-stores-id-customers-id`** - Delete a customer from a store.
- **`mailchimp-pp-cli ecommerce delete-stores-id-orders-id`** - Delete an order.
- **`mailchimp-pp-cli ecommerce delete-stores-id-orders-id-lines-id`** - Delete a specific order line item.
- **`mailchimp-pp-cli ecommerce delete-stores-id-products-id`** - Delete a product.
- **`mailchimp-pp-cli ecommerce delete-stores-id-products-id-images-id`** - Delete a product image.
- **`mailchimp-pp-cli ecommerce delete-stores-id-products-id-variants-id`** - Delete a product variant.
- **`mailchimp-pp-cli ecommerce delete-stores-id-promocodes-id`** - Delete a promo code from a store.
- **`mailchimp-pp-cli ecommerce delete-stores-id-promorules-id`** - Delete a promo rule from a store.
- **`mailchimp-pp-cli ecommerce get-orders`** - Get information about an account's orders.
- **`mailchimp-pp-cli ecommerce get-stores`** - Get information about all stores in the account.
- **`mailchimp-pp-cli ecommerce get-stores-id`** - Get information about a specific store.
- **`mailchimp-pp-cli ecommerce get-stores-id-carts`** - Get information about a store's carts.
- **`mailchimp-pp-cli ecommerce get-stores-id-carts-id`** - Get information about a specific cart.
- **`mailchimp-pp-cli ecommerce get-stores-id-carts-id-lines`** - Get information about a cart's line items.
- **`mailchimp-pp-cli ecommerce get-stores-id-carts-id-lines-id`** - Get information about a specific cart line item.
- **`mailchimp-pp-cli ecommerce get-stores-id-customers`** - Get information about a store's customers.
- **`mailchimp-pp-cli ecommerce get-stores-id-customers-id`** - Get information about a specific customer.
- **`mailchimp-pp-cli ecommerce get-stores-id-orders`** - Get information about a store's orders.
- **`mailchimp-pp-cli ecommerce get-stores-id-orders-id`** - Get information about a specific order.
- **`mailchimp-pp-cli ecommerce get-stores-id-orders-id-lines`** - Get information about an order's line items.
- **`mailchimp-pp-cli ecommerce get-stores-id-orders-id-lines-id`** - Get information about a specific order line item.
- **`mailchimp-pp-cli ecommerce get-stores-id-products`** - Get information about a store's products.
- **`mailchimp-pp-cli ecommerce get-stores-id-products-id`** - Get information about a specific product.
- **`mailchimp-pp-cli ecommerce get-stores-id-products-id-images`** - Get information about a product's images.
- **`mailchimp-pp-cli ecommerce get-stores-id-products-id-images-id`** - Get information about a specific product image.
- **`mailchimp-pp-cli ecommerce get-stores-id-products-id-variants`** - Get information about a product's variants.
- **`mailchimp-pp-cli ecommerce get-stores-id-products-id-variants-id`** - Get information about a specific product variant.
- **`mailchimp-pp-cli ecommerce get-stores-id-promocodes`** - Get information about a store's promo codes.
- **`mailchimp-pp-cli ecommerce get-stores-id-promocodes-id`** - Get information about a specific promo code.
- **`mailchimp-pp-cli ecommerce get-stores-id-promorules`** - Get information about a store's promo rules.
- **`mailchimp-pp-cli ecommerce get-stores-id-promorules-id`** - Get information about a specific promo rule.
- **`mailchimp-pp-cli ecommerce patch-stores-id`** - Update a store.
- **`mailchimp-pp-cli ecommerce patch-stores-id-carts-id`** - Update a specific cart.
- **`mailchimp-pp-cli ecommerce patch-stores-id-carts-id-lines-id`** - Update a specific cart line item.
- **`mailchimp-pp-cli ecommerce patch-stores-id-customers-id`** - Update a customer.
- **`mailchimp-pp-cli ecommerce patch-stores-id-orders-id`** - Update a specific order.
- **`mailchimp-pp-cli ecommerce patch-stores-id-orders-id-lines-id`** - Update a specific order line item.
- **`mailchimp-pp-cli ecommerce patch-stores-id-products-id`** - Update a specific product.
- **`mailchimp-pp-cli ecommerce patch-stores-id-products-id-images-id`** - Update a product image.
- **`mailchimp-pp-cli ecommerce patch-stores-id-products-id-variants-id`** - Update a product variant.
- **`mailchimp-pp-cli ecommerce patch-stores-id-promocodes-id`** - Update a promo code.
- **`mailchimp-pp-cli ecommerce patch-stores-id-promorules-id`** - Update a promo rule.
- **`mailchimp-pp-cli ecommerce post-stores`** - Add a new store to your Mailchimp account.
- **`mailchimp-pp-cli ecommerce post-stores-id-carts`** - Add a new cart to a store.
- **`mailchimp-pp-cli ecommerce post-stores-id-carts-id-lines`** - Add a new line item to an existing cart.
- **`mailchimp-pp-cli ecommerce post-stores-id-customers`** - Add a new customer to a store.
- **`mailchimp-pp-cli ecommerce post-stores-id-orders`** - Add a new order to a store.
- **`mailchimp-pp-cli ecommerce post-stores-id-orders-id-lines`** - Add a new line item to an existing order.
- **`mailchimp-pp-cli ecommerce post-stores-id-products`** - Add a new product to a store.
- **`mailchimp-pp-cli ecommerce post-stores-id-products-id-images`** - Add a new image to the product.
- **`mailchimp-pp-cli ecommerce post-stores-id-products-id-variants`** - Add a new variant to the product.
- **`mailchimp-pp-cli ecommerce post-stores-id-promocodes`** - Add a new promo code to a store.
- **`mailchimp-pp-cli ecommerce post-stores-id-promorules`** - Add a new promo rule to a store.
- **`mailchimp-pp-cli ecommerce put-stores-id-customers-id`** - Add or update a customer.
- **`mailchimp-pp-cli ecommerce put-stores-id-orders-id`** - Add or update an order.
- **`mailchimp-pp-cli ecommerce put-stores-id-products-id`** - Update a specific product.
- **`mailchimp-pp-cli ecommerce put-stores-id-products-id-variants-id`** - Add or update a product variant.

### facebook-ads

Manage facebook ads

- **`mailchimp-pp-cli facebook-ads get-all`** - Get list of Facebook ads.
- **`mailchimp-pp-cli facebook-ads get-id`** - Get details of a Facebook ad.

### file-manager

Manage file manager

- **`mailchimp-pp-cli file-manager delete-files-id`** - Remove a specific file from the File Manager.
- **`mailchimp-pp-cli file-manager delete-folders-id`** - Delete a specific folder in the File Manager.
- **`mailchimp-pp-cli file-manager get-files`** - Get a list of available images and files stored in the File Manager for the account.
- **`mailchimp-pp-cli file-manager get-files-id`** - Get information about a specific file in the File Manager.
- **`mailchimp-pp-cli file-manager get-folders`** - Get a list of all folders in the File Manager.
- **`mailchimp-pp-cli file-manager get-folders-files`** - Get a list of available images and files stored in this folder.
- **`mailchimp-pp-cli file-manager get-folders-id`** - Get information about a specific folder in the File Manager.
- **`mailchimp-pp-cli file-manager patch-files-id`** - Update a file in the File Manager.
- **`mailchimp-pp-cli file-manager patch-folders-id`** - Update a specific File Manager folder.
- **`mailchimp-pp-cli file-manager post-files`** - Upload a new image or file to the File Manager.
- **`mailchimp-pp-cli file-manager post-folders`** - Create a new folder in the File Manager.

### landing-pages

Manage landing pages

- **`mailchimp-pp-cli landing-pages delete-id`** - Delete a landing page.
- **`mailchimp-pp-cli landing-pages get-all`** - Get all landing pages.
- **`mailchimp-pp-cli landing-pages get-id`** - Get information about a specific page.
- **`mailchimp-pp-cli landing-pages patch-id`** - Update a landing page.
- **`mailchimp-pp-cli landing-pages post-all`** - Create an unpublished and contentless Mailchimp landing page.

### lists

Manage lists

- **`mailchimp-pp-cli lists delete-id`** - Delete a list from your Mailchimp account. If you delete a list, you'll lose the list history—including subscriber activity, unsubscribes, complaints, and bounces. You’ll also lose subscribers’ email addresses, unless you exported and backed up your list.
- **`mailchimp-pp-cli lists get`** - Get information about all lists in the account.
- **`mailchimp-pp-cli lists get-id`** - Get information about a specific list in your Mailchimp account. Results include list members who have signed up but haven't confirmed their subscription yet and unsubscribed or cleaned.
- **`mailchimp-pp-cli lists patch-id`** - Update the settings for a specific list.
- **`mailchimp-pp-cli lists post`** - Create a new list in your Mailchimp account.
- **`mailchimp-pp-cli lists post-id`** - Batch subscribe or unsubscribe list members.

### ping

Manage ping

- **`mailchimp-pp-cli ping`** - A health check for the API that won't return any account-specific information.

### reporting

Manage reporting

- **`mailchimp-pp-cli reporting get-facebook-ads`** - Get reports of Facebook ads.
- **`mailchimp-pp-cli reporting get-facebook-ads-id`** - Get report of a Facebook ad.
- **`mailchimp-pp-cli reporting get-facebook-ads-id-ecommerce-product-activity`** - Get breakdown of product activity for an outreach.
- **`mailchimp-pp-cli reporting get-landing-pages`** - Get reports of landing pages.
- **`mailchimp-pp-cli reporting get-landing-pages-id`** - Get report of a landing page.
- **`mailchimp-pp-cli reporting get-surveys`** - Get reports for surveys.
- **`mailchimp-pp-cli reporting get-surveys-id`** - Get report for a survey.
- **`mailchimp-pp-cli reporting get-surveys-id-questions`** - Get reports for survey questions.
- **`mailchimp-pp-cli reporting get-surveys-id-questions-id`** - Get report for a survey question.
- **`mailchimp-pp-cli reporting get-surveys-id-questions-id-answers`** - Get answers for a survey question.
- **`mailchimp-pp-cli reporting get-surveys-id-responses`** - Get responses to a survey.
- **`mailchimp-pp-cli reporting get-surveys-id-responses-id`** - Get a single survey response.

### reports

Manage reports

- **`mailchimp-pp-cli reports get`** - Get campaign reports.
- **`mailchimp-pp-cli reports get-id`** - Get report details for a specific sent campaign.

### search-campaigns

Manage search campaigns

- **`mailchimp-pp-cli search-campaigns`** - Search all campaigns for the specified query terms.

### search-members

Manage search members

- **`mailchimp-pp-cli search-members`** - Search for list members. This search can be restricted to a specific list, or can be used to search across all lists in an account.

### sms-campaigns

Manage sms campaigns

- **`mailchimp-pp-cli sms-campaigns delete-id`** - Remove an SMS campaign from your Mailchimp account.
- **`mailchimp-pp-cli sms-campaigns get`** - Get all SMS campaigns in an account.
- **`mailchimp-pp-cli sms-campaigns get-id`** - Get information about a specific SMS campaign.
- **`mailchimp-pp-cli sms-campaigns patch-id`** - Update some or all of the settings for a specific SMS campaign.
- **`mailchimp-pp-cli sms-campaigns post`** - Create a new SMS campaign.

### template-folders

Manage template folders

- **`mailchimp-pp-cli template-folders delete-id`** - Delete a specific template folder, and mark all the templates in the folder as 'unfiled'.
- **`mailchimp-pp-cli template-folders get`** - Get all folders used to organize templates.
- **`mailchimp-pp-cli template-folders get-id`** - Get information about a specific folder used to organize templates.
- **`mailchimp-pp-cli template-folders patch-id`** - Update a specific folder used to organize templates.
- **`mailchimp-pp-cli template-folders post`** - Create a new template folder.

### templates

Manage templates

- **`mailchimp-pp-cli templates delete-id`** - Delete a specific template.
- **`mailchimp-pp-cli templates get`** - Get a list of an account's available templates.
- **`mailchimp-pp-cli templates get-id`** - Get information about a specific template.
- **`mailchimp-pp-cli templates patch-id`** - Update the name, HTML, or `folder_id` of an existing template.
- **`mailchimp-pp-cli templates post`** - Create a new template for the account. Only Classic templates are supported.

### verified-domains

Manage verified domains

- **`mailchimp-pp-cli verified-domains create`** - Add a domain to the account.
- **`mailchimp-pp-cli verified-domains delete`** - Delete a verified domain from the account.
- **`mailchimp-pp-cli verified-domains get`** - Get all of the sending domains on the account.
- **`mailchimp-pp-cli verified-domains get-verifieddomains`** - Get the details for a single domain on the account.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
mailchimp-pp-cli account-exports get

# JSON for scripting and agents
mailchimp-pp-cli account-exports get --json

# Filter to specific fields
mailchimp-pp-cli account-exports get --json --select id,name,status

# Dry run — show the request without sending
mailchimp-pp-cli account-exports get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
mailchimp-pp-cli account-exports get --agent
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

## Health Check

```bash
mailchimp-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/mailchimp-marketing-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MAILCHIMP_API_KEY` | per_call | Yes | Full Mailchimp API key with `-dc` suffix (e.g., `abc...-us6`). The CLI parses the suffix to route requests and sets HTTP Basic auth automatically. |
| `MAILCHIMP_DC` | per_call | No | Override datacenter (e.g., `us6`). Useful for OAuth tokens that lack the suffix. Takes precedence over the key's embedded `-dc`. |
| `MAILCHIMP_USERNAME` | per_call | No | Legacy: explicit Basic-auth username. Ignored when `MAILCHIMP_API_KEY` is set. |
| `MAILCHIMP_PASSWORD` | per_call | No | Legacy: explicit Basic-auth password. Ignored when `MAILCHIMP_API_KEY` is set. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `mailchimp-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo "${MAILCHIMP_API_KEY:0:8}…${MAILCHIMP_API_KEY: -4}"` (avoid printing the full key)
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports 'no datacenter suffix in API key'** — Mailchimp keys must end with '-us1' through '-us21'. Re-copy the key from https://us1.admin.mailchimp.com/account/api/; the dashboard truncates it visually.
- **429 'rate limit' on bulk operations** — Mailchimp caps at 10 concurrent connections per key. For more than ~50 mutations, use 'mailchimp-pp-cli bulk-subscribe' which posts to /batches.
- **'Member Exists' error on creating a subscriber** — Mailchimp uses PUT for upsert, not POST. The 'mailchimp-pp-cli subscribe' command handles this automatically; for raw API calls use the 'lists members upsert' subcommand (PUT) instead of 'create' (POST).
- **Batch finished but result URL returns 404** — Mailchimp's response_body_url is valid only 10 minutes after batch completion. 'mailchimp-pp-cli bulk-subscribe --watch' polls and downloads within the window.
- **doctor passes but every API call returns 401** — Mailchimp's HTTP Basic auth uses 'anystring:{KEY}'. If you exported MAILCHIMP_API_KEY with a leading space or quoted wrapper, strip them. The key starts immediately after '='.

## Known Limitations

A few documented upstream behaviors that surprise CLI users:

- **`file-manager get-folders-id` and `get-folders-files` return 200 with default content for invalid folder IDs.** The Mailchimp API treats an unknown folder ID as "the unfiled bucket" and returns the empty default rather than 404. Check the `id` field in the response: an `id: 0` with `name: "Unfiled"` indicates the folder did not match anything.
- **`sms-campaigns content get-sms-campaigns-id` returns 200 with empty `message_body` for invalid campaign IDs** rather than 404. Most other Mailchimp endpoints (e.g., `campaigns get-id`, `lists get-id`, `account-exports get-id`) return clean 404s.
- **`workflow archive --json` emits NDJSON (newline-delimited JSON event stream), not a single JSON document.** This is intentional for streaming sync events; pipe through `jq -c` or read line-by-line.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**mailchimp-marketing-node**](https://github.com/mailchimp/mailchimp-marketing-node) — JavaScript (166 stars)
- [**mailchimp-marketing-python**](https://github.com/mailchimp/mailchimp-marketing-python) — Python (120 stars)
- [**AgentX-ai/mailchimp-mcp**](https://github.com/AgentX-ai/mailchimp-mcp) — TypeScript (11 stars)
- [**robinsloan/mailchimp-cli**](https://github.com/robinsloan/mailchimp-cli) — Ruby (10 stars)
- [**damientilman/mailchimp-mcp-server**](https://github.com/damientilman/mailchimp-mcp-server) — Python (9 stars)
- [**cyanheads/mailchimp-mcp-server**](https://github.com/cyanheads/mailchimp-mcp-server) — TypeScript (1 stars)
- [**richardpowellus/mailchimp-mcp-server**](https://github.com/richardpowellus/mailchimp-mcp-server) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
