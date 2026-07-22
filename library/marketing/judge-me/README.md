# Judge Me CLI

Our REST API lets you access to resources and perform actions on behalf of a store.
For more information, read Judge.me's [integration guidelines](https://help.judge.me/en/articles/8278390-integrating-with-judge-me) and [FAQs](https://help.judge.me/en/articles/8278477-faqs-for-integration-partners).

# Authentication
Pass your API token in the `X-Api-Token` HTTP header (recommended):
```
curl -H "X-Api-Token: YOUR_API_TOKEN" https://api.judge.me/api/v1/widgets/product_review?shop_domain=example.myshopify.com&id=123
```

## OAuth
[Judge.me](http://judge.me/) uses OAuth2 to grant App Developers access to [Judge.me](http://judge.me/) API. You need to use the OAuth api_token generated from the Judge.me app [following this guide](https://help.judge.me/en/articles/8283047-setting-up-the-oauth-flow-for-your-app-in-judge-me).
Example of how to authenticate with OAuth:
```
GET https://app.judge.me/oauth/authorize?client_id=[your_client_id]&redirect_uri=[your_redirect_uri]&response_type=code&scope=[list_of_permissions_you_are_asking]&state=[state]
```

Created by [@cathrynlavery](https://github.com/cathrynlavery).

## Install

The recommended path installs both the `judge-me-pp-cli` binary and the `pp-judge-me` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install judge-me
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install judge-me --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install judge-me --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install judge-me --agent claude-code
npx -y @mvanhorn/printing-press-library install judge-me --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/judge-me/cmd/judge-me-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/judge-me-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install judge-me --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-judge-me --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-judge-me --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install judge-me --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/judge-me-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `JUDGE_ME_API_TOKEN` and `JUDGE_ME_SHOP_DOMAIN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/judge-me/cmd/judge-me-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "judge-me": {
      "command": "judge-me-pp-mcp",
      "env": {
        "JUDGE_ME_API_TOKEN": "<your-key>",
        "JUDGE_ME_SHOP_DOMAIN": "your-store.myshopify.com"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export JUDGE_ME_API_TOKEN="<paste-your-key>"
export JUDGE_ME_SHOP_DOMAIN="your-store.myshopify.com"
```

`JUDGE_ME_PRIVATE_APIKEY` is also accepted as the generated-token env var, but `JUDGE_ME_API_TOKEN` is the clearer alias for Judge.me's `X-Api-Token` header. You can also persist this in your config file at `~/.config/judge-me-pp-cli/config.toml`.

### 3. Verify Setup

```bash
judge-me-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
judge-me-pp-cli reviewers get mock-value
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### reputation
- **`reputation summary`** — One read-only dashboard for store-level review count, rating, shop metadata, settings, and local review stats.

  _Use before campaigns or launch reviews to spot trust-presentation problems quickly._

  ```bash
  judge-me-pp-cli reputation summary --agent
  ```
- **`reputation products`** — Ranks synced products by low-rating rate, average rating, verified rate, and newest review recency.

  _Use weekly to find products dragging down customer trust._

  ```bash
  judge-me-pp-cli sync --resources reviews --max-pages 1 && judge-me-pp-cli reputation products --agent
  ```

### moderation
- **`reputation moderation-queue`** — Surfaces low-rated, hidden, spam-marked, or not-yet-curated synced reviews for human attention.

  _Use during support triage to prioritize reputation-impacting reviews._

  ```bash
  judge-me-pp-cli reputation moderation-queue --agent
  ```

### settings
- **`reputation settings-audit`** — Audits Judge.me settings that affect trust presentation, including autopublish, picture reviews, star color, and admin email presence when returned.

  _Use after setup or theme changes to confirm trust display configuration._

  ```bash
  judge-me-pp-cli reputation settings-audit --agent
  ```

### product
- **`reputation product`** — Fetches product-level count/histogram evidence when product ID is known and widget availability/byte evidence for handles or external IDs.

  _Use before merchandising or ad pushes to verify product social proof is present._

  ```bash
  judge-me-pp-cli reputation product --handle example-product --agent
  ```

## Usage

Run `judge-me-pp-cli --help` for the full command reference and flag list.

## Commands

### private-replies

Manage private replies

- **`judge-me-pp-cli private-replies`** - Create a private email reply to a [Judge.me](http://judge.me/) review privately via your app interface.

### replies

Manage replies

- **`judge-me-pp-cli replies`** - Create a reply to a Judge.me review on the public [Judge.me](http://judge.me/) review widget via your app interface.

### reviewers

Manage reviewers

- **`judge-me-pp-cli reviewers data-request`** - Data Request
- **`judge-me-pp-cli reviewers get`** - Get information of the reviewers such as name and email. This is useful when you are building automated flow for users to follow up with reviewers.
- **`judge-me-pp-cli reviewers update`** - Create or update a reviewer via your app interface.

### reviews

You can use the reviews endpoints to access review information. Common use cases include:
- Synchronize and display reviews in your admin dashboard.
- Get event of new reviews (via webhook) to perform an action on your side.
- Let users manage reviews (publish/hide) on your side.
*Note: these endpoints respond **raw** review information, which may include **unpublished reviews**, or review content that is **not sanitized** yet (so risks of XSS).
To render review content on storefront, please use widget endpoints instead.

- **`judge-me-pp-cli reviews create`** - Create a web review in background, similar to submitting a review via the public form on product pages (no authorization required).
This endpoint doesn't create any review if the store [disables web reviews](https://help.judge.me/en/articles/8375147-restricting-web-reviews).
- **`judge-me-pp-cli reviews get`** - Get info of a specific review.
- **`judge-me-pp-cli reviews index`** - Get info of reviews of a product. If `product_id` is not provided, return all product and store reviews of that store.
- **`judge-me-pp-cli reviews reviewers-count`** - Get count of reviews for a specific product or reviewer. If product_id is not provided, return the count of all product and store reviews of that store.
- **`judge-me-pp-cli reviews update`** - Publish or hide a [Judge.me](http://judge.me/) review via your app interface. For authenticity reason, we don’t support editing reviews via API.

### settings

Manage settings

- **`judge-me-pp-cli settings`** - Get multiple settings values of the store in [Judge.me](http://judge.me/), which can serve as conditions for your app integrations.

### shops

Manage shops

- **`judge-me-pp-cli shops comments-create`** - Create a checkout comment. Available in Checkout Comments app only.
- **`judge-me-pp-cli shops destroy`** - Uninstall the store from Judge.me
- **`judge-me-pp-cli shops info`** - Get the basic information of the store such as [Judge.me](http://judge.me/) plan, owner name, email, e-commerce platform, etc. This is helpful when you are developing your app for specific segment of users (e.g. your integration is only available to Judge.me Awesome users).
- **`judge-me-pp-cli shops update`** - Update store information

### webhooks

Subscribe to an event happens in Judge.me. Judge.me will send a POST request to the registered URL containing relevant information for each event.

Common webhook keys:
1. **review/created** or **review/created_fail**: to know when a review is created, or not.
2. **review/updated**: to know when a review is updated in Judge.me. In particular, when a review is:
- curated or mass curated
- pinned/featured in carousel
- moved to another product
- edited from admin or user profile
- verified review via request emails
- added/hidden/shown review photos from admin or user profile
3. Widgets update webhooks (e.g. **widget/settings/updated**): to know when Judge.me updates a widget.
***Note**: You can learn how to verify webhooks from Judge.me following this [guide](https://help.judge.me/en/articles/8299679-verifying-webhooks-from-judge-me).

- **`judge-me-pp-cli webhooks bulk-create`** - Bulk Create
- **`judge-me-pp-cli webhooks create`** - Create a webhook in Judge.me with a `key` and a `url`. When an event associated with `key` happens,
Judge.me will send a POST request to the webhook's `url`.
- **`judge-me-pp-cli webhooks destroy`** - Delete
- **`judge-me-pp-cli webhooks get`** - Get
- **`judge-me-pp-cli webhooks index`** - Index
- **`judge-me-pp-cli webhooks update`** - Update

### widgets

Manage widgets

- **`judge-me-pp-cli widgets all-reviews-count`** - Return a single total number of product and store reviews.
- **`judge-me-pp-cli widgets all-reviews-page`** - All Reviews Page is a dedicated page to showcase all product and store reviews all in one place.
- **`judge-me-pp-cli widgets all-reviews-rating`** - Return a single number of average rating of product reviews and store reviews.
- **`judge-me-pp-cli widgets checkout-comments`** - Return Checkout Comments widget for a product (for Checkout Comments app only). You can use product handle, external ID, or [Judge.me](http://judge.me/) internal ID to specify the product.
- **`judge-me-pp-cli widgets featured-carousel`** - Reviews Carousel is usually placed on the homepage to showcase specific reviews featured by the store.
- **`judge-me-pp-cli widgets html-miracle`** - Return special HTML that helps show essential parts of widgets before the JS and CSS files are loaded.
- **`judge-me-pp-cli widgets preview-badge`** - Preview Badge is usually placed below product titles on product pages or inside product thumbnails on collection pages. This widget display the average star rating and review count of each product.
You can use product handle, external ID, or [Judge.me](http://judge.me/) internal ID to specify the product.
- **`judge-me-pp-cli widgets product-review`** - Review Widget is usually placed at the bottom of each product page, displaying all reviews of a product.
You can use product handle, external ID, or [Judge.me](http://judge.me/) internal ID to specify the product.
- **`judge-me-pp-cli widgets reviews-tab`** - Floating Reviews Tab display all product and store reviews via a floating button on any pages.
- **`judge-me-pp-cli widgets settings`** - Return widget settings of the shop, under HTML format, containing a `<script>` tag and a `<style>` tag.
This contains values of widget customization in [Judge.me](http://judge.me/) such as text and color, which helps you display the widget correctly.
- **`judge-me-pp-cli widgets shop-reviews-count`** - Return a single total number of store reviews.
- **`judge-me-pp-cli widgets shop-reviews-rating`** - Return a single number of average rating of store reviews.
- **`judge-me-pp-cli widgets verified-badge`** - Verified Reviews Count Badge displays the number of verified published reviews. Stores need at least 20 verified reviews to use this widget.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
judge-me-pp-cli reviewers get mock-value

# JSON for scripting and agents
judge-me-pp-cli reviewers get mock-value --json

# Filter to specific fields
judge-me-pp-cli reviewers get mock-value --json --select id,name,status

# Dry run — show the request without sending
judge-me-pp-cli reviewers get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
judge-me-pp-cli reviewers get mock-value --agent
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
judge-me-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/api-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `JUDGE_ME_API_TOKEN` | per_call | Yes | Judge.me API credential sent as the `X-Api-Token` header. |
| `JUDGE_ME_PRIVATE_APIKEY` | per_call | Yes | Generated-name alias for `JUDGE_ME_API_TOKEN`. |
| `JUDGE_ME_SHOP_DOMAIN` | per_call | Yes | Store domain sent as Judge.me's required `shop_domain` query parameter. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `judge-me-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `judge-me-pp-cli doctor` to check credentials
- Verify the environment variables are set without printing secrets: `test -n "$JUDGE_ME_API_TOKEN" && test -n "$JUDGE_ME_SHOP_DOMAIN"`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
