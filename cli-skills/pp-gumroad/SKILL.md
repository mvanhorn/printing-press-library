---
name: pp-gumroad
description: "Gumroad's seller API as an agent-ready CLI and MCP server. Trigger phrases: `gumroad products`, `gumroad sales`, `gumroad payouts`, `gumroad subscribers`, `verify gumroad license`, `gumroad offer codes`."
author: "Bheem Reddy"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - gumroad-pp-cli
---

# Gumroad — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `gumroad-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install gumroad --cli-only
   ```
2. Verify: `gumroad-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This tool wraps Gumroad's documented OAuth API for products, files, covers, variants, offer codes, custom fields, resource subscriptions, sales, subscribers, licenses, payouts, tax forms, and earnings. It also adds the Printing Press local sync/search/analytics layer so agents can answer seller questions from a consistent local snapshot instead of repeatedly walking paginated endpoints.

## When to Use This CLI

Use this skill when the user asks to inspect or operate a Gumroad seller account, especially products, sales, subscribers, payouts, licenses, offer codes, or tax data. Prefer sync/search/analytics for broad questions and direct endpoint commands for a specific object or mutation.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local seller intelligence

- **`sync --resources products,sales,subscribers,payouts --latest-only --json`** — Refresh a local SQLite snapshot of the seller-facing Gumroad resources that agents inspect most often.

  _Use this before research or reporting tasks so later searches and analytics are grounded in one consistent seller snapshot._

  ```bash
  gumroad-pp-cli sync --resources products,sales,subscribers,payouts --latest-only --json
  ```
- **`search`** — Search locally synced products, sales, subscribers, payouts, and tax records through one command.

  _Use this when the user remembers a buyer, product, license, or payout clue but not the exact Gumroad object ID._

  ```bash
  gumroad-pp-cli search "annual plan" --data-source local --json --limit 20
  ```
- **`analytics`** — Group and count synced Gumroad records locally for fast summaries without additional API traffic.

  _Use this for lightweight revenue, subscriber, product, and payout triage after a sync._

  ```bash
  gumroad-pp-cli analytics --type sales --group-by product_id --limit 10 --json
  ```

### Operational monitoring

- **`tail`** — Poll selected Gumroad resources and emit NDJSON changes for scripts or agent monitors.

  _Use this for one-off checks after a launch, refund, payout, or product update._

  ```bash
  gumroad-pp-cli tail --resource sales --interval 30s --json
  ```

## Command Reference

**earnings** — Manage earnings

- `gumroad-pp-cli earnings` — Retrieve an annual earnings breakdown for the authenticated user. Requires view_tax_data scope.

**files** — Manage files

- `gumroad-pp-cli files abort` — Cancel a multipart upload started by /files/presign. Requires edit_products scope.
- `gumroad-pp-cli files complete` — Finalize a multipart upload started by /files/presign. Requires edit_products scope.
- `gumroad-pp-cli files presign` — Start a multipart upload and return presigned URLs for each part. Requires edit_products scope.

**licenses** — Manage licenses

- `gumroad-pp-cli licenses decrement-uses-count` — Decrement the uses count of a license. Requires edit_products scope.
- `gumroad-pp-cli licenses disable` — Disable a license. Requires edit_products scope.
- `gumroad-pp-cli licenses enable` — Enable a license. Requires edit_products scope.
- `gumroad-pp-cli licenses rotate` — Rotate a license key. The old key will no longer be valid. Requires edit_products scope.
- `gumroad-pp-cli licenses verify` — Verify a license key.

**payouts** — Manage payouts

- `gumroad-pp-cli payouts get` — Retrieve details of a payout. Requires view_payouts scope.
- `gumroad-pp-cli payouts get-upcoming` — Retrieve upcoming payouts. Requires view_payouts scope.
- `gumroad-pp-cli payouts list` — Retrieve payouts for the authenticated user. Requires view_payouts scope.

**products** — Manage products

- `gumroad-pp-cli products create` — Create a new product as a draft. Requires edit_products or account scope.
- `gumroad-pp-cli products delete` — Permanently delete a product.
- `gumroad-pp-cli products get` — Retrieve details of a product.
- `gumroad-pp-cli products list` — Retrieve all existing products for the authenticated user.
- `gumroad-pp-cli products update` — Update an existing product. Collection fields such as files, tags, and rich_content replace the full collection.

**resource-subscriptions** — Manage resource subscriptions

- `gumroad-pp-cli resource-subscriptions create` — Subscribe to a resource webhook. Requires view_sales scope.
- `gumroad-pp-cli resource-subscriptions delete` — Unsubscribe from a resource.
- `gumroad-pp-cli resource-subscriptions list` — Show active subscriptions for the input resource. Requires view_sales scope.

**sales** — Manage sales

- `gumroad-pp-cli sales get` — Retrieve details of a sale. Requires view_sales scope.
- `gumroad-pp-cli sales list` — Retrieve successful sales by the authenticated user. Requires view_sales scope.

**subscribers** — Manage subscribers

- `gumroad-pp-cli subscribers <id>` — Retrieve details of a subscriber. Requires view_sales scope.

**tax-forms** — Manage tax forms

- `gumroad-pp-cli tax-forms` — Retrieve 1099 tax forms for the authenticated user. Requires view_tax_data scope.

**user** — Manage user

- `gumroad-pp-cli user` — Retrieve the authenticated user's data.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
gumroad-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### List live products

```bash
gumroad-pp-cli products list --agent
```

Returns compact JSON for the authenticated seller's products.

### Find a buyer or license locally

```bash
gumroad-pp-cli search "customer query" --data-source local --json
```

Searches previously synced seller data without making additional API calls.

### Review payout history

```bash
gumroad-pp-cli payouts list --after 2026-01-01 --agent
```

Reads payout records with pagination metadata when the token has view_payouts scope.

## Auth Setup

Create a Gumroad OAuth application, authorize it with the scopes needed for your workflow, and provide the resulting access token as GUMROAD_ACCESS_TOKEN. The MCP bundle exposes this as a sensitive user configuration value.

Run `gumroad-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  gumroad-pp-cli earnings --year 42 --agent --select id,name,status
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
gumroad-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
gumroad-pp-cli feedback --stdin < notes.txt
gumroad-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.gumroad-pp-cli/feedback.jsonl`. They are never POSTed unless `GUMROAD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GUMROAD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
gumroad-pp-cli profile save briefing --json
gumroad-pp-cli --profile briefing earnings --year 42
gumroad-pp-cli profile list --json
gumroad-pp-cli profile show briefing
gumroad-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `gumroad-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add gumroad-pp-mcp -- gumroad-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which gumroad-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   gumroad-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `gumroad-pp-cli <command> --help`.
