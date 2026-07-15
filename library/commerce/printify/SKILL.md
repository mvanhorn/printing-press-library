---
name: pp-printify
description: "Create and audit Printify products with manifest-driven image uploads, personalization checks, and local proofing. Trigger phrases: `create Printify products`, `upload Printify artwork`, `audit Printify personalization`, `check Printify product drift`, `run Printify`, `use Printify`."
author: "horknfbr"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - printify-pp-cli
    install:
      - kind: go
        bins: [printify-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/printify/cmd/printify-pp-cli
---

# Printify — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `printify-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install printify --cli-only
   ```
2. Verify: `printify-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/printify/cmd/printify-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI for Printify product automation that needs more than raw endpoint calls: image uploads, manifest-driven drafts, personalization field checks, placement proofing, and operational audits. It is especially useful for agents building or verifying POD product batches from local artwork and CSV inputs.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Product proofing
- **`personalization-audit`** — Audit a product for documented personalization placeholder fields, missing text/image setup, and unsupported browser-only gaps.

  _Use this before publishing a personalized product or before handing a manifest to another agent._

  ```bash
  printify-pp-cli personalization-audit --product-file ./examples/sample-product.json --agent
  ```
- **`placement-matrix`** — Show variant, print-area, placeholder, uploaded image, x/y/scale/angle, and missing-placement rows for a product.

  _Use this when an agent must prove artwork placement is consistent across variants._

  ```bash
  printify-pp-cli placement-matrix --product-file ./examples/sample-product.json --uploads-file ./examples/sample-uploads.json --agent
  ```
- **`product-drift`** — Compare an intended product manifest against the current Printify product payload after create or update.

  _Use this after automation creates a product and before trusting the resulting listing._

  ```bash
  printify-pp-cli product-drift --product-file ./examples/current-product.json --manifest ./examples/sample-product.json --agent
  ```

### Catalog decisions
- **`catalog-margin-matrix`** — Join catalog variants and shipping data to estimate per-variant cost and margin at a target retail price.

  _Use this before creating a batch when product economics matter more than raw catalog browsing._

  ```bash
  printify-pp-cli catalog-margin-matrix --variants-file ./examples/sample-variants.json --shipping-file ./examples/sample-shipping.json --target-price 24.99 --agent
  ```

### Batch automation
- **`personalization-batch`** — Expand a reusable product manifest and CSV rows into per-product manifests using documented image and text placeholder fields.

  _Use this when an agent needs repeatable personalized product drafts before making API writes._

  ```bash
  printify-pp-cli personalization-batch --template ./examples/template-product.json --csv ./examples/personalization.csv --out ./examples/generated-manifests --agent
  ```

### Operational audits
- **`asset-reuse`** — List uploaded images, where each is used, unused uploads, and products sharing the same artwork.

  _Use this to clean upload libraries or verify that a product batch reuses the expected artwork._

  ```bash
  printify-pp-cli asset-reuse --products-file ./examples/sample-products.json --uploads-file ./examples/sample-uploads.json --agent
  ```
- **`fulfillment-risk`** — Flag open orders tied to risky product, variant, publish, or shipment states.

  _Use this when an agent needs to find fulfillment problems before customer-service work starts._

  ```bash
  printify-pp-cli fulfillment-risk --orders-file ./examples/sample-orders.json --products-file ./examples/sample-products.json --agent
  ```

## Command Reference

**catalog** — Browse the Printify catalog including blueprints, print providers, product variants, and shipping information. Explore available products and their customization options.

- `printify-pp-cli catalog retrieve-alist-of-all-print-providers-that-fulfill-orders-for-aspecific-blueprint` — Retrieve a list of all print providers that fulfill orders for a specific blueprint
- `printify-pp-cli catalog retrieve-alist-of-available-print-providers` — Retrieves the list of blueprints in the catalog to explore from
- `printify-pp-cli catalog retrieve-alist-of-variants-of-ablueprint-from-aspecific-print-provider` — Retrieves the list of of variants options for the Print Provider and Blueprint.
- `printify-pp-cli catalog retrieve-aspecific-blueprint` — Retrieves the list of blueprints in the catalog to explore from
- `printify-pp-cli catalog retrieve-aspecific-print-provider` — Retrieves the list of blueprints in the catalog to explore from
- `printify-pp-cli catalog retrieve-available-shipping-list-information` — Retrieves the list of print providers avilable for the Blueprint
- `printify-pp-cli catalog retrieve-economy-shipping-method-information` — Retrieves the list of print providers available for the Blueprint
- `printify-pp-cli catalog retrieve-express-shipping-method-information` — Retrieves the list of print providers available for the Blueprint
- `printify-pp-cli catalog retrieve-priority-shipping-method-information` — Retrieves the list of print providers available for the Blueprint
- `printify-pp-cli catalog retrieve-shipping-information` — Retrieves the list of print providers avilable for the Blueprint
- `printify-pp-cli catalog retrieve-specific-shipping-method-information` — Retrieves the list of print providers avilable for the Blueprint
- `printify-pp-cli catalog retrieves-list-of-blueprints-in-the` — Retrieves the list of blueprints in the catalog to explore from

**shops** — Manage Printify shops and shop connections. Retrieve shop information and disconnect shops from your account.


**shops-json** — Manage shops json

- `printify-pp-cli shops-json` — This will return the list of available merchant shops (IDs and titles)

**uploads** — Upload and manage images and assets. Upload images from URLs or base64-encoded content, retrieve upload information, and archive uploaded images.

- `printify-pp-cli uploads an-image` — Upload an image
- `printify-pp-cli uploads retrieve-an-uploaded-image-by-id` — Retrieve an uploaded image by id

**uploads-json** — Manage uploads json

- `printify-pp-cli uploads-json` — Retrieve a list of uploaded images


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
printify-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find a shop with narrow output

```bash
printify-pp-cli shops-json --agent --select id,title
```

Use this first so follow-up product and order commands target the right shop without dumping the full shop payload.

### Compile personalized product manifests

```bash
printify-pp-cli personalization-batch --template ./examples/template-product.json --csv ./examples/personalization.csv --out ./examples/generated-manifests --agent
```

Turn a reusable template and row data into deterministic product manifests before any API writes.

### Proof artwork placement

```bash
printify-pp-cli placement-matrix --product-file ./examples/sample-product.json --uploads-file ./examples/sample-uploads.json --agent --select variant_id,print_area,image_id,x,y,scale,angle
```

Inspect only the placement columns an agent needs before publishing.

### Check product drift after create

```bash
printify-pp-cli product-drift --product-file ./examples/current-product.json --manifest ./examples/sample-product.json --agent
```

Confirm the remote product still matches the manifest the automation intended.

### Scan fulfillment risk

```bash
printify-pp-cli fulfillment-risk --orders-file ./examples/sample-orders.json --products-file ./examples/sample-products.json --agent
```

Join orders, variants, product state, and shipment state into a single operational risk view.

## Auth Setup

Printify uses a personal access token as a bearer token. Set `PRINTIFY_API_TOKEN` in your environment or `.env`; do not pass the token as a command argument.

Run `printify-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  printify-pp-cli catalog retrieve-alist-of-all-print-providers-that-fulfill-orders-for-aspecific-blueprint mock-value --agent --select id,name,status
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
printify-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
printify-pp-cli feedback --stdin < notes.txt
printify-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/printify-pp-cli/feedback.jsonl`. They are never POSTed unless `PRINTIFY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PRINTIFY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
printify-pp-cli profile save briefing --json
printify-pp-cli --profile briefing catalog retrieve-alist-of-all-print-providers-that-fulfill-orders-for-aspecific-blueprint mock-value
printify-pp-cli profile list --json
printify-pp-cli profile show briefing
printify-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `printify-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/printify/cmd/printify-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add printify-pp-mcp -- printify-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which printify-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   printify-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `printify-pp-cli <command> --help`.
