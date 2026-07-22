---
name: pp-judge-me
description: "Printing Press CLI for Judge Me. Our REST API lets you access to resources and perform actions on behalf of a store."
author: "cathrynlavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - judge-me-pp-cli
    install:
      - kind: go
        bins: [judge-me-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/judge-me/cmd/judge-me-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/marketing/judge-me/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Judge Me — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `judge-me-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install judge-me --cli-only
   ```
2. Verify: `judge-me-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/judge-me/cmd/judge-me-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

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

## Unique Capabilities

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

## Command Reference

**private-replies** — Manage private replies

- `judge-me-pp-cli private-replies` — Create a private email reply to a [Judge.me](http://judge.me/) review privately via your app interface.

**replies** — Manage replies

- `judge-me-pp-cli replies` — Create a reply to a Judge.me review on the public [Judge.me](http://judge.me/) review widget via your app interface.

**reviewers** — Manage reviewers

- `judge-me-pp-cli reviewers data-request` — Data Request
- `judge-me-pp-cli reviewers get` — Get information of the reviewers such as name and email.
- `judge-me-pp-cli reviewers update` — Create or update a reviewer via your app interface.

**reviews** — You can use the reviews endpoints to access review information. Common use cases include:
- Synchronize and display reviews in your admin dashboard.
- Get event of new reviews (via webhook) to perform an action on your side.
- Let users manage reviews (publish/hide) on your side.
*Note: these endpoints respond **raw** review information, which may include **unpublished reviews**, or review content that is **not sanitized** yet (so risks of XSS).
To render review content on storefront, please use widget endpoints instead.

- `judge-me-pp-cli reviews create` — Create a web review in background
- `judge-me-pp-cli reviews get` — Get info of a specific review.
- `judge-me-pp-cli reviews index` — Get info of reviews of a product. If `product_id` is not provided, return all product and store reviews of that store.
- `judge-me-pp-cli reviews reviewers-count` — Get count of reviews for a specific product or reviewer.
- `judge-me-pp-cli reviews update` — Publish or hide a [Judge.me](http://judge.me/) review via your app interface.

**settings** — Manage settings

- `judge-me-pp-cli settings` — Get multiple settings values of the store in [Judge.me](http://judge.

**shops** — Manage shops

- `judge-me-pp-cli shops comments-create` — Create a checkout comment. Available in Checkout Comments app only.
- `judge-me-pp-cli shops destroy` — Uninstall the store from Judge.me
- `judge-me-pp-cli shops info` — Get the basic information of the store such as [Judge.me](http://judge.
- `judge-me-pp-cli shops update` — Update store information

**webhooks** — Subscribe to an event happens in Judge.me. Judge.me will send a POST request to the registered URL containing relevant information for each event.

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

- `judge-me-pp-cli webhooks bulk-create` — Bulk Create
- `judge-me-pp-cli webhooks create` — Create a webhook in Judge.me with a `key` and a `url`. When an event associated with `key` happens, Judge.
- `judge-me-pp-cli webhooks destroy` — Delete
- `judge-me-pp-cli webhooks get` — Get
- `judge-me-pp-cli webhooks index` — Index
- `judge-me-pp-cli webhooks update` — Update

**widgets** — Manage widgets

- `judge-me-pp-cli widgets all-reviews-count` — Return a single total number of product and store reviews.
- `judge-me-pp-cli widgets all-reviews-page` — All Reviews Page is a dedicated page to showcase all product and store reviews all in one place.
- `judge-me-pp-cli widgets all-reviews-rating` — Return a single number of average rating of product reviews and store reviews.
- `judge-me-pp-cli widgets checkout-comments` — Return Checkout Comments widget for a product (for Checkout Comments app only).
- `judge-me-pp-cli widgets featured-carousel` — Reviews Carousel is usually placed on the homepage to showcase specific reviews featured by the store.
- `judge-me-pp-cli widgets html-miracle` — Return special HTML that helps show essential parts of widgets before the JS and CSS files are loaded.
- `judge-me-pp-cli widgets preview-badge` — Preview Badge is usually placed below product titles on product pages or inside product thumbnails on collection pages.
- `judge-me-pp-cli widgets product-review` — Review Widget is usually placed at the bottom of each product page, displaying all reviews of a product.
- `judge-me-pp-cli widgets reviews-tab` — Floating Reviews Tab display all product and store reviews via a floating button on any pages.
- `judge-me-pp-cli widgets settings` — Return widget settings of the shop, under HTML format, containing a `<script>` tag and a `<style>` tag.
- `judge-me-pp-cli widgets shop-reviews-count` — Return a single total number of store reviews.
- `judge-me-pp-cli widgets shop-reviews-rating` — Return a single number of average rating of store reviews.
- `judge-me-pp-cli widgets verified-badge` — Verified Reviews Count Badge displays the number of verified published reviews.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
judge-me-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `judge-me-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export JUDGE_ME_API_TOKEN="<your-key>"
export JUDGE_ME_SHOP_DOMAIN="your-store.myshopify.com"
```

`JUDGE_ME_PRIVATE_APIKEY` is also accepted as the generated-token alias.

Run `judge-me-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  judge-me-pp-cli reviewers get mock-value --agent --select id,name,status
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
judge-me-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
judge-me-pp-cli feedback --stdin < notes.txt
judge-me-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/judge-me-pp-cli/feedback.jsonl`. They are never POSTed unless `JUDGE_ME_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `JUDGE_ME_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
judge-me-pp-cli profile save briefing --json
judge-me-pp-cli --profile briefing reviewers get mock-value
judge-me-pp-cli profile list --json
judge-me-pp-cli profile show briefing
judge-me-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `judge-me-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/judge-me/cmd/judge-me-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add judge-me-pp-mcp -- judge-me-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which judge-me-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   judge-me-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `judge-me-pp-cli <command> --help`.
