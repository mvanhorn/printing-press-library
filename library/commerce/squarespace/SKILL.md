---
name: pp-squarespace
description: "Squarespace Commerce plus browser-backed account domains, DNS, email forwarding, and billing reads."
author: "Zayd"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - squarespace-pp-cli
---

# Squarespace — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `squarespace-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install squarespace --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `squarespace-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/squarespace/cmd/squarespace-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Account Dashboard Auth

Commerce commands use `COMMERCE_AUTHORIZATION`. Account dashboard commands use a Cookie header from an authenticated `account.squarespace.com` browser session:

```bash
export SQUARESPACE_ACCOUNT_COOKIE_FILE="$HOME/.config/squarespace/account-cookie.txt"
```

Prefer `--name example.com` over internal IDs. The CLI resolves `/api/account/1/domains/byName/<domain>` first, then uses the returned domain, website, and contract IDs for follow-on calls. Do not bake account-specific Squarespace IDs into scripts or docs.

## Account Dashboard Commands

Use these for read-only domain, DNS, email forwarding, Google Workspace, and billing metadata:

```bash
squarespace-pp-cli account domain-summaries --page-size 50 --json
squarespace-pp-cli account domain get --name example.com --json
squarespace-pp-cli account domain custom-records --name example.com --json
squarespace-pp-cli account domain email-forwarding --name example.com --json
squarespace-pp-cli account domain email-mx-conflicts --name example.com --json
squarespace-pp-cli account domain billing-eligibility --name example.com --json
squarespace-pp-cli account domain billing-valid-terms --name example.com --json
squarespace-pp-cli account domain google-workspace-pricing --country-code US --json
```



## Command Reference

**1-0** — Manage 1 0

- `squarespace-pp-cli 1-0 adjust-inventory-stock-levels` — Adjusts stock quantities for product variants. Stock quantities can be added or subtracted, set with a given number,...
- `squarespace-pp-cli 1-0 create-order` — Creates an order using information from a third-party sales channel. A successful request creates an Order resource.
- `squarespace-pp-cli 1-0 create-webhook-subscription` — Creates a webhook subscription. A successful request returns the created Webhook Subscription resource.
- `squarespace-pp-cli 1-0 delete-webhook-subscription` — Deletes a webhook subscription.
- `squarespace-pp-cli 1-0 fulfill-order` — Updates the status of a specific order to fulfilled, with options to include shipment information and send a...
- `squarespace-pp-cli 1-0 get-documents-by-id` — Retrieves information for specific transaction Documents. The response contains up to 50 Documents. Multiple...
- `squarespace-pp-cli 1-0 get-documents-by-updated-on` — Retrieves all financial transactions for orders and donations. Per order and donation, the response groups...
- `squarespace-pp-cli 1-0 get-inventory-items` — Retrieves real-time stock information for all product variants. Stock information is stored in an InventoryItem for...
- `squarespace-pp-cli 1-0 get-member-profile` — Retrieves basic details about the Squarespace member who owns the provided OAuth token
- `squarespace-pp-cli 1-0 get-order` — Retrieves information for a specific order. The response contains order information in an Order.
- `squarespace-pp-cli 1-0 get-orders` — Retrieves information about all orders. Orders can be filtered by Customer ID and date ranges. The response contains...
- `squarespace-pp-cli 1-0 get-profiles` — Retrieves all profiles; profiles can be filtered and sorted. The response contains up to 50 Profiles and supports...
- `squarespace-pp-cli 1-0 get-specific-inventory-items` — Retrieves real-time stock information for specific product variants. Stock information is stored in an InventoryItem...
- `squarespace-pp-cli 1-0 get-specific-profiles` — Retrieves information for specific profiles. The response contains a list of up to 50 Profiles. Multiple Profiles...
- `squarespace-pp-cli 1-0 get-store-pages` — Retrieves information for all store pages on the website. The response supports dynamic cursors for pagination.
- `squarespace-pp-cli 1-0 get-webhook-subscription` — Retrieves information for a specific webhook subscription.
- `squarespace-pp-cli 1-0 get-webhook-subscriptions` — Retrieves information for all webhook subscriptions. The response contains up to 25 Webhook Subscriptions.
- `squarespace-pp-cli 1-0 get-website-profile` — Retrieves basic details about the website that owns the provided API key or OAuth token
- `squarespace-pp-cli 1-0 rotate-subscription-secret` — Rotates a webhook subscription's secret. The previous secret for a subscription is no longer valid after a new one...
- `squarespace-pp-cli 1-0 send-test-notification-for-webhook-subscription` — Sends a notification to a subscribed webhook endpoint for testing purposes. This is a one-time notification, and...
- `squarespace-pp-cli 1-0 update-webhook-subscription` — Updates information for a webhook subscription. A successful request returns the updated Webhook Subscription resource.

**commerce** — Manage commerce

- `squarespace-pp-cli commerce associate-product-variant-image` — Assigns a product image to a product variant. Specifying imageId of null will delete the association.
- `squarespace-pp-cli commerce create-product` — Creates a product with appropriate subresources based on product type.
- `squarespace-pp-cli commerce create-product-variant` — Creates a variant of a product. Creating a variant for a physical or service product requires that the product...
- `squarespace-pp-cli commerce delete-product` — Delete specified product
- `squarespace-pp-cli commerce delete-product-image` — Delete one product image
- `squarespace-pp-cli commerce delete-product-variant` — Delete one product variant
- `squarespace-pp-cli commerce get-product-image-processing-status` — Retrieves the processing status for an uploaded product image. A successful request indicates that a ProductImage is...
- `squarespace-pp-cli commerce get-products` — Retrieves information for products; products can be filtered within a date range and by product type. The response...
- `squarespace-pp-cli commerce get-specific-products` — Retrieves information for specific products, including information for any variants or images. Up to 50 products can...
- `squarespace-pp-cli commerce update-product` — Updates information for a product. The endpoint supports partial updates.
- `squarespace-pp-cli commerce update-product-image` — Updates information for a product image. Currently, the endpoint only supports updates to the alt text for an image.
- `squarespace-pp-cli commerce update-product-image-order` — Updates the ordering of a product image on the product details page.
- `squarespace-pp-cli commerce update-product-variant` — Updates a variant of a product. The endpoint supports partial updates.
- `squarespace-pp-cli commerce upload-product-image` — Uploads an image for a product. Uploading an image doesn't set the product's featured image. A successful request...

**commerce-analytics** — Manage commerce analytics

- `squarespace-pp-cli commerce-analytics` — Retrieve transaction summaries grouped by contact

**contacts** — Manage customer contacts and address book entries for a website: create, read, update, delete, and query contacts; maintain addresses for shipping and fulfillment.

- `squarespace-pp-cli contacts create` — Creates a new contact. Requires OAuth website scope WEBSITE_CONTACTS or WEBSITE_PROFILES (read/write).
- `squarespace-pp-cli contacts delete` — Deletes the contact for the given contact ID. Requires OAuth website scope WEBSITE_CONTACTS or WEBSITE_PROFILES...
- `squarespace-pp-cli contacts get` — Returns a paginated list of contacts for the website. Requires OAuth website scope WEBSITE_CONTACTS_READ,...
- `squarespace-pp-cli contacts get-contactid` — Returns the contact for the given contact ID. Requires OAuth website scope WEBSITE_CONTACTS_READ, WEBSITE_CONTACTS,...
- `squarespace-pp-cli contacts patch` — Updates a contact using JSON merge patch. Requires OAuth website scope WEBSITE_CONTACTS or WEBSITE_PROFILES...
- `squarespace-pp-cli contacts query` — Returns a paginated list of contacts matching filters and sort options in the request body. Requires OAuth website...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
squarespace-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `squarespace-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
squarespace-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `COMMERCE_AUTHORIZATION` as an environment variable.

Run `squarespace-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  squarespace-pp-cli contacts get --pagination-parameters example-value --agent --select id,name,status
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
squarespace-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
squarespace-pp-cli feedback --stdin < notes.txt
squarespace-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.squarespace-pp-cli/feedback.jsonl`. They are never POSTed unless `SQUARESPACE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SQUARESPACE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
squarespace-pp-cli profile save briefing --json
squarespace-pp-cli --profile briefing contacts get --pagination-parameters example-value
squarespace-pp-cli profile list --json
squarespace-pp-cli profile show briefing
squarespace-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `squarespace-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add squarespace-pp-mcp -- squarespace-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which squarespace-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   squarespace-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `squarespace-pp-cli <command> --help`.
