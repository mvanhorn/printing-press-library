---
name: pp-g2
description: "Every G2 Marketplace surface, plus a credit-aware local mirror, full-text review search Trigger phrases: `show me G2 reviews for`, `what does G2 buyer intent say about`, `competitive watch on G2`, `g2 alt-track`, `use g2-pp-cli`, `run g2-pp-cli`."
author: "Rahul Bansal"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - g2-pp-cli
    install:
      - kind: go
        bins: [g2-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/cmd/g2-pp-cli
---

# G2 — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `g2-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install g2 --cli-only
   ```
2. Verify: `g2-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/cmd/g2-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

g2-pp-cli unifies the v2 OpenAPI surface and the legacy syndication API behind one auth layer, mirrors products, reviews, vendors, categories, and buyer-intent into a local SQLite store, and adds workflows the Anthropic G2 MCP and every scraper miss: credit-burn forecasting (`credits forecast`), Alternatives-signal switching threats (`alt-track`), cross-product weekly diffs (`watch products`), and FTS over synced reviews.

## When to Use This CLI

Reach for g2-pp-cli when a PMM, sales-ops, competitive-intel, or buyer-side workflow needs more than a chat-based MCP. The CLI keeps a local SQLite mirror of products, reviews, vendors, categories, and buyer-intent events so cron jobs, dashboards, and `claude`-piped agent workflows can run offline against a fresh corpus. Use it when credit-burn matters, when cross-product diffs are involved, or when full-text search over review bodies is the actual question.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`watch products`** — Diff star ratings, new review counts, and badge changes across a list of products since a given window.

  _Reach for this when a competitive-intel or PMM workflow asks for a head-to-head delta over a window. Replaces tab-by-tab eyeballing of multiple G2 pages._

  ```bash
  g2-pp-cli watch products --product datadog,new-relic,dynatrace --since 7d --json
  ```
- **`search`** — Full-text search across locally synced reviews, scoped by product list and structured filters (rating, segment).

  _Use when a buyer or competitive analyst wants to grep verbatims across many products' review corpora._

  ```bash
  g2-pp-cli search "rate limit" --type reviews --limit 20 --json
  ```
- **`alt-track`** — Rank companies showing Alternatives-signal buyer intent against your product, weighted by visit count and employee size.

  _Pick this when sales needs a ranked list of accounts evaluating switching away from your product._

  ```bash
  g2-pp-cli alt-track my-product --since 30d --csv
  ```
- **`analytics`** — Per category, surface products in the top growth quartile by 30-day review velocity that aren't yet top quartile by absolute review count.

  _Reach for this when a PMM or competitive-intel analyst wants early signals on momentum players in a market._

  ```bash
  g2-pp-cli analytics --type products --group-by category --limit 20 --json
  ```
- **`reviews list`** — Filter synced reviews for the syndication-eligible flag, scoped by product and a recency window.

  _Pick this when wiring a marketing site or landing page to auto-refresh approved review quotes._

  ```bash
  g2-pp-cli reviews list --product my-product --syndication-eligible --since 7d --json
  ```
- **`watch market-signals`** — Diff locally snapshotted market_signals for a category over a window; emit intent-score and visits-count deltas.

  _Reach for this during a Friday competitive-intel report to spot category-level momentum changes._

  ```bash
  g2-pp-cli watch market-signals --category devops-monitoring --since 7d --json
  ```

### Reachability mitigation
- **`credits forecast`** — Project month-end credit spend from trailing 14-day usage and refuse sync runs that would exceed remaining credits.

  _Pick this before a heavy sync to avoid mid-month throttling on credit-metered endpoints._

  ```bash
  g2-pp-cli credits forecast --json
  ```

### Agent-native plumbing
- **`buyer-intent list`** — Filter buyer-intent rows by recency and minimum score and flatten nested firmographics into a sales-ready CSV.

  _Pick this for a daily cron that pushes high-intent companies into the team's CRM or Slack._

  ```bash
  g2-pp-cli buyer-intent list --since 24h --min-score 50 --csv
  ```
- **`analytics`** — For each product in a list, return the lowest-rated reviews and their cons verbatims as JSON for downstream summarization.

  _Use when competitive intel needs to ground a complaint roundup across competitors; pipe the JSON to a summarizer of choice._

  ```bash
  g2-pp-cli analytics --type reviews --group-by product --limit 10 --json
  ```

## Command Reference

**buyer-intent** — Manage buyer intent

- `g2-pp-cli buyer-intent` — **Requires `buyer_intent.read` scope.** Query buyer intent data across one or more products in a single request.

**categories** — Manage categories

- `g2-pp-cli categories get` — **No scope required.**
- `g2-pp-cli categories get-all-category-features` — List all category product features grouped by category
- `g2-pp-cli categories get-category` — Shows a single category

**market-signals** — Manage market signals

- `g2-pp-cli market-signals` — **Requires `market_signals.read` scope.** Retrieve buyer intent signals (interactions) for specific categories.

**organization** — Manage organization


**partners** — Manage partners


**product-mappings** — Manage product mappings

- `g2-pp-cli product-mappings create` — Create a product mapping
- `g2-pp-cli product-mappings get` — List of product mappings
- `g2-pp-cli product-mappings update` — Update a product mapping

**product-ratings** — Manage product ratings

- `g2-pp-cli product-ratings <product_id>` — **Requires `products.ratings.read` scope.**

**products** — Manage products

- `g2-pp-cli products get` — **Requires `products.read` scope.**
- `g2-pp-cli products get-all-features` — List all product features grouped by product
- `g2-pp-cli products get-productid` — **Requires `products.read` scope.**

**questions** — Manage questions

- `g2-pp-cli questions get` — **Requires `questions.read` scope.**
- `g2-pp-cli questions get-id` — **Requires `questions.read` scope.**

**reports** — Manage reports

- `g2-pp-cli reports <product_id>` — **Requires `reports.read` scope.**

**sandbox** — Manage sandbox

- `g2-pp-cli sandbox <subject_product_id>` — **No scope is required to access this sandbox endpoint** **⚠️ Sandbox API**

**screenshots** — Manage screenshots

- `g2-pp-cli screenshots get` — **Requires `screenshots.read` scope.**
- `g2-pp-cli screenshots get-id` — **Requires `screenshots.read` scope.**

**user** — Manage user

- `g2-pp-cli user` — Returns the currently authenticated user.

**users** — Manage users

- `g2-pp-cli users add-product-to-research-board` — Add a product to a research board
- `g2-pp-cli users batch-add-products-to-research-board` — Add multiple products to a research board
- `g2-pp-cli users batch-remove-products-from-research-board` — Remove multiple products from a research board
- `g2-pp-cli users create-research-board` — Create a research board
- `g2-pp-cli users delete-research-board` — Delete a research board
- `g2-pp-cli users get-current-product-review` — **⚠️ G2 is currently restricting access to this endpoint.
- `g2-pp-cli users get-current-products` — **⚠️ G2 is currently restricting access to this endpoint.
- `g2-pp-cli users get-research-board` — Get a research board
- `g2-pp-cli users list-research-board-products` — List products on a research board
- `g2-pp-cli users list-research-boards` — List research boards for current user
- `g2-pp-cli users remove-product-from-research-board` — Remove a product from a research board
- `g2-pp-cli users submit-current-product-review` — **⚠️ G2 is currently restricting access to this endpoint.
- `g2-pp-cli users update-research-board` — Update a research board

**vendors** — Manage vendors

- `g2-pp-cli vendors get` — **Requires `vendors.read` scope.**
- `g2-pp-cli vendors get-id` — **Requires `vendors.read` scope.**


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
g2-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Monday morning competitive snapshot

```bash
g2-pp-cli watch products --product datadog,new-relic,dynatrace --since 7d --json --select "product,star_delta,new_reviews"
```

Joins yesterday's `products` snapshot with new `reviews` rows to produce a flat star-delta-by-competitor view; `--select` keeps the agent context short.

### Daily buyer-intent triage to CSV

```bash
g2-pp-cli buyer-intent list --since 24h --min-score 50 --csv > intent-$(date +%Y%m%d).csv
```

Cron-friendly: emits a flat CSV with company, domain, employees, industry, intent_score, page_type, signal_type.

### Switching-threat list for sales

```bash
g2-pp-cli alt-track my-product --since 30d --csv
```

Surfaces companies showing Alternatives-signal intent against your product, ranked by visit count and employee size. The website paywalls this view.

### Find rate-limit complaints across competitors

```bash
g2-pp-cli search "rate limit" --type reviews --limit 50 --json --select "product_id,title,rating,cons"
```

FTS5 query across all synced reviews; pair with `--select` to keep nested response payloads manageable for downstream agents.

### Pre-sync credit check

```bash
g2-pp-cli credits forecast --json
```

Reads the local `credit_ledger`, projects month-end burn from the trailing 14-day average, and exits non-zero if a planned sync would exceed the remaining quota.

## Auth Setup

G2 ships two authentication surfaces. The v2 API uses OAuth 2.0 (`G2_CLIENT_ID` + `G2_CLIENT_SECRET`) plus an `AccountAPIToken` header (`G2_ACCOUNT_API_TOKEN`). The legacy syndication API (products, reviews, vendors, product_mappings, hashed_users) uses an `api_token` query parameter (`G2_SYNDICATION_TOKEN`). `g2-pp-cli auth login` walks you through both and stores them in `~/.config/g2-pp-cli/auth.json`. A paid G2 Sell-tier subscription is required for production endpoints; the developer portal at `my.g2.com/developers` provides a 1,000-call/month starter quota.

Run `g2-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  g2-pp-cli categories get --agent --select id,name,status
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
g2-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
g2-pp-cli feedback --stdin < notes.txt
g2-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/g2-pp-cli/feedback.jsonl`. They are never POSTed unless `G2_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `G2_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
g2-pp-cli profile save briefing --json
g2-pp-cli --profile briefing categories get
g2-pp-cli profile list --json
g2-pp-cli profile show briefing
g2-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `g2-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/cmd/g2-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add g2-pp-mcp -- g2-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which g2-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   g2-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `g2-pp-cli <command> --help`.
