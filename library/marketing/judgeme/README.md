# Judge.me CLI

**A completeness-verified Judge.me review corpus with explicit populations and syndication-aware exports.**

Wrap the full Judge.me API while defending review workflows against the silent 10,000-row pagination loop. Sync once into a documented SQLite mirror, then export, count, and deduplicate published or internal review populations with provenance on every agent response.

## Install

The recommended path installs both the `judgeme-pp-cli` binary and the `pp-judgeme` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install judgeme
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install judgeme --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install judgeme --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install judgeme --agent claude-code
npx -y @mvanhorn/printing-press-library install judgeme --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/judgeme/cmd/judgeme-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download the matching binary and `checksums.txt` from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/judgeme-current) into the same directory. Verify the binary before running it (`shasum -a 256 -c checksums.txt --ignore-missing` on macOS or `sha256sum -c checksums.txt --ignore-missing` on Linux). After verification, macOS users can clear the Gatekeeper quarantine with `xattr -d com.apple.quarantine <binary>`; on Unix, mark the binary executable with `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install judgeme --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-judgeme --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-judgeme --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install judgeme --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/judgeme-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `JUDGEME_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/judgeme/cmd/judgeme-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "judgeme": {
      "command": "judgeme-pp-mcp",
      "env": {
        "JUDGEME_API_KEY": "<your-key>",
        "JUDGEME_SHOP_DOMAIN": "<shop>.myshopify.com"
      }
    }
  }
}
```

</details>

## Authentication

Set JUDGEME_API_KEY to the private token and JUDGEME_SHOP_DOMAIN to the store's myshopify.com domain. The private token is sent only in the X-Api-Token header and is redacted from logs; it is never placed in a request URL.

## Quick Start

```bash
# Check configuration without contacting Judge.me.
judgeme-pp-cli doctor --dry-run

# Build the verified local review mirror before analysis.
judgeme-pp-cli sync --resources reviews --full --agent

# Confirm which population each count represents.
judgeme-pp-cli reviews populations --agent

# Export a storefront-visible analysis slice.
judgeme-pp-cli reviews export --population published --rating 5 --csv

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Verified local corpus
- **`sync`** — Mirror the complete review corpus and refuse success when unique IDs do not equal Judge.me's live count.

  _Use this before any analysis that depends on corpus completeness._

  ```bash
  judgeme-pp-cli sync --resources reviews --full --max-pages 1 --agent
  ```
- **`reviews export`** — Export explicit published, hidden, pending, or all populations as JSON or CSV.

  _Use this to hand a bounded, provenance-labeled review slice to downstream analysis tools._

  ```bash
  judgeme-pp-cli reviews export --population published --rating 5 --csv --dry-run
  ```

### Population integrity
- **`reviews populations`** — Count storefront-visible and internal moderation populations with unambiguous labels.

  _Use this whenever published-vs-hidden population differences affect a report or decision._

  ```bash
  judgeme-pp-cli reviews populations --agent --dry-run
  ```
- **`reviews syndication`** — Find identical normalized review bodies attached to multiple products.

  _Use this before attributing customer language to a specific SKU or product._

  ```bash
  judgeme-pp-cli reviews syndication --population all --min-products 2 --agent --dry-run
  ```
- **`reviews unique-bodies`** — Return one deterministic representative per normalized review body while retaining source row counts.

  _Use this when independent customer voices matter more than API row count._

  ```bash
  judgeme-pp-cli reviews unique-bodies --population published --agent --dry-run
  ```

## Recipes

### Narrow a published corpus for an agent

```bash
judgeme-pp-cli reviews export --population published --agent --select meta.source,meta.synced_at,results.id,results.rating,results.body
```

Return only storefront-visible review fields needed for downstream analysis.

### Measure population differences

```bash
judgeme-pp-cli reviews populations --agent
```

Compare all, published, pending, and hidden review populations.

### Find bundle syndication

```bash
judgeme-pp-cli reviews syndication --min-products 2 --agent
```

Identify one body hash associated with multiple products.

### Export independent voices

```bash
judgeme-pp-cli reviews unique-bodies --population published --csv
```

Produce one representative row per normalized review body.

## Usage

Run `judgeme-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `JUDGEME_CONFIG_DIR`, `JUDGEME_DATA_DIR`, `JUDGEME_STATE_DIR`, or `JUDGEME_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `JUDGEME_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export JUDGEME_HOME=/srv/judgeme
judgeme-pp-cli doctor
```

Under `JUDGEME_HOME=/srv/judgeme`, the four dirs resolve to `/srv/judgeme/config`, `/srv/judgeme/data`, `/srv/judgeme/state`, and `/srv/judgeme/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "judgeme": {
      "command": "judgeme-pp-mcp",
      "env": {
        "JUDGEME_HOME": "/srv/judgeme"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `JUDGEME_DATA_DIR` overrides an explicit `--home` for that kind. Use `JUDGEME_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `JUDGEME_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `judgeme-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### private-replies

Manage private replies

- **`judgeme-pp-cli private-replies`** - Create a private email reply to a [Judge.me](http://judge.me/) review privately via your app interface.

### replies

Manage replies

- **`judgeme-pp-cli replies`** - Create a reply to a Judge.me review on the public [Judge.me](http://judge.me/) review widget via your app interface.

### reviewers

Manage reviewers

- **`judgeme-pp-cli reviewers data-request`** - Data Request
- **`judgeme-pp-cli reviewers get`** - Get information of the reviewers such as name and email. This is useful when you are building automated flow for users to follow up with reviewers.
- **`judgeme-pp-cli reviewers update`** - Create or update a reviewer via your app interface.

### reviews

You can use the reviews endpoints to access review information. Common use cases include:
- Synchronize and display reviews in your admin dashboard.
- Get event of new reviews (via webhook) to perform an action on your side.
- Let users manage reviews (publish/hide) on your side.
*Note: these endpoints respond **raw** review information, which may include **unpublished reviews**, or review content that is **not sanitized** yet (so risks of XSS).
To render review content on storefront, please use widget endpoints instead.

- **`judgeme-pp-cli reviews create`** - Create a web review in background, similar to submitting a review via the public form on product pages (no authorization required).
This endpoint doesn't create any review if the store [disables web reviews](https://help.judge.me/en/articles/8375147-restricting-web-reviews).
- **`judgeme-pp-cli reviews get`** - Get info of a specific review.
- **`judgeme-pp-cli reviews index`** - Get info of reviews of a product. If `product_id` is not provided, return all product and store reviews of that store.
- **`judgeme-pp-cli reviews reviewers-count`** - Get count of reviews for a specific product or reviewer. If product_id is not provided, return the count of all product and store reviews of that store.
- **`judgeme-pp-cli reviews update`** - Publish or hide a Judge.me review via your app interface. For authenticity reasons, the API does not support editing review content.

### settings

Manage settings

- **`judgeme-pp-cli settings`** - Get multiple settings values of the store in [Judge.me](http://judge.me/), which can serve as conditions for your app integrations.

### shops

Manage shops

- **`judgeme-pp-cli shops comments-create`** - Create a checkout comment. Available in Checkout Comments app only.
- **`judgeme-pp-cli shops destroy`** - Uninstall the store from Judge.me
- **`judgeme-pp-cli shops get`** - Get the basic information of the store such as [Judge.me](http://judge.me/) plan, owner name, email, e-commerce platform, etc. This is helpful when you are developing your app for specific segment of users (e.g. your integration is only available to Judge.me Awesome users).
- **`judgeme-pp-cli shops update`** - Update store information

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

- **`judgeme-pp-cli webhooks bulk-create`** - Bulk Create
- **`judgeme-pp-cli webhooks create`** - Create a webhook in Judge.me with a `key` and a `url`. When an event associated with `key` happens,
Judge.me will send a POST request to the webhook's `url`.
- **`judgeme-pp-cli webhooks destroy`** - Delete
- **`judgeme-pp-cli webhooks get`** - Get
- **`judgeme-pp-cli webhooks index`** - Index
- **`judgeme-pp-cli webhooks update`** - Update

### widgets

Manage widgets

- **`judgeme-pp-cli widgets all-reviews-count`** - Return a single total number of product and store reviews.
- **`judgeme-pp-cli widgets all-reviews-page`** - All Reviews Page is a dedicated page to showcase all product and store reviews all in one place.
- **`judgeme-pp-cli widgets all-reviews-rating`** - Return a single number of average rating of product reviews and store reviews.
- **`judgeme-pp-cli widgets checkout-comments`** - Return Checkout Comments widget for a product (for Checkout Comments app only). You can use product handle, external ID, or [Judge.me](http://judge.me/) internal ID to specify the product.
- **`judgeme-pp-cli widgets featured-carousel`** - Reviews Carousel is usually placed on the homepage to showcase specific reviews featured by the store.
- **`judgeme-pp-cli widgets html-miracle`** - Return special HTML that helps show essential parts of widgets before the JS and CSS files are loaded.
- **`judgeme-pp-cli widgets preview-badge`** - Preview Badge is usually placed below product titles on product pages or inside product thumbnails on collection pages. This widget display the average star rating and review count of each product.
You can use product handle, external ID, or [Judge.me](http://judge.me/) internal ID to specify the product.
- **`judgeme-pp-cli widgets product-review`** - Review Widget is usually placed at the bottom of each product page, displaying all reviews of a product.
You can use product handle, external ID, or [Judge.me](http://judge.me/) internal ID to specify the product.
- **`judgeme-pp-cli widgets reviews-tab`** - Floating Reviews Tab display all product and store reviews via a floating button on any pages.
- **`judgeme-pp-cli widgets settings`** - Return widget settings of the shop, under HTML format, containing a `<script>` tag and a `<style>` tag.
This contains values of widget customization in [Judge.me](http://judge.me/) such as text and color, which helps you display the widget correctly.
- **`judgeme-pp-cli widgets shop-reviews-count`** - Return a single total number of store reviews.
- **`judgeme-pp-cli widgets shop-reviews-rating`** - Return a single number of average rating of store reviews.
- **`judgeme-pp-cli widgets verified-badge`** - Verified Reviews Count Badge displays the number of verified published reviews. Stores need at least 20 verified reviews to use this widget.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`judgeme-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`judgeme-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`judgeme-pp-cli learnings list`** - Inspect taught rows
- **`judgeme-pp-cli learnings forget <query>`** - Undo a teach
- **`judgeme-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`judgeme-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`judgeme-pp-cli teach-pattern`** - Install a query/resource template up front
- **`judgeme-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `JUDGEME_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `judgeme-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
judgeme-pp-cli reviewers get mock-value

# JSON for scripting and agents
judgeme-pp-cli reviewers get mock-value --json

# Filter to specific fields
judgeme-pp-cli reviewers get mock-value --json --select id,name,status

# Dry run — show the request without sending
judgeme-pp-cli reviewers get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
judgeme-pp-cli reviewers get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `JUDGEME_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `judgeme-pp-cli reviews`
- `judgeme-pp-cli reviews get`
- `judgeme-pp-cli reviews list`
- `judgeme-pp-cli reviews search`
- `judgeme-pp-cli settings`
- `judgeme-pp-cli settings get`
- `judgeme-pp-cli settings list`
- `judgeme-pp-cli settings search`
- `judgeme-pp-cli webhooks`
- `judgeme-pp-cli webhooks get`
- `judgeme-pp-cli webhooks list`
- `judgeme-pp-cli webhooks search`
- `judgeme-pp-cli widgets`
- `judgeme-pp-cli widgets get`
- `judgeme-pp-cli widgets list`
- `judgeme-pp-cli widgets search`
- `judgeme-pp-cli widgets-all-reviews-count`
- `judgeme-pp-cli widgets-all-reviews-count get`
- `judgeme-pp-cli widgets-all-reviews-count list`
- `judgeme-pp-cli widgets-all-reviews-count search`
- `judgeme-pp-cli widgets-all-reviews-page`
- `judgeme-pp-cli widgets-all-reviews-page get`
- `judgeme-pp-cli widgets-all-reviews-page list`
- `judgeme-pp-cli widgets-all-reviews-page search`
- `judgeme-pp-cli widgets-all-reviews-rating`
- `judgeme-pp-cli widgets-all-reviews-rating get`
- `judgeme-pp-cli widgets-all-reviews-rating list`
- `judgeme-pp-cli widgets-all-reviews-rating search`
- `judgeme-pp-cli widgets-product-review`
- `judgeme-pp-cli widgets-product-review get`
- `judgeme-pp-cli widgets-product-review list`
- `judgeme-pp-cli widgets-product-review search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
judgeme-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `judgeme-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/api-pp-cli/config.toml`; `--home`, `JUDGEME_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `JUDGEME_API_KEY` | per_call | Yes | Set to your API credential. |
| `JUDGEME_SHOP_DOMAIN` | per_call | Yes | Shopify shop domain in myshopify.com form. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `judgeme-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `judgeme-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $JUDGEME_API_KEY`
- Verify the shop domain is set: `echo $JUDGEME_SHOP_DOMAIN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Sync stops after detecting a repeated page.** — Rerun `judgeme-pp-cli sync --resources reviews --full`; the safe paginator will partition and must match `/reviews/count` before committing.
- **A report count is higher than the storefront count.** — Pass `--population published`; the default `all` population includes reviews Judge.me does not show on the storefront.
- **Product-level VoC contains identical language across bundle SKUs.** — Run `judgeme-pp-cli reviews syndication --agent`, then use `judgeme-pp-cli reviews unique-bodies --population published`.
