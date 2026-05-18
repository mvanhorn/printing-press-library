---
name: pp-servicetitan-pricebook
description: "A focused per-module ServiceTitan CLI for the Pricebook — sync once, then audit markup, costs, vendor parts, and warranties the ST UI cannot. Trigger phrases: `use servicetitan-pricebook`, `run servicetitan-pricebook`, `sync the pricebook`, `check markup drift`, `reconcile a vendor quote`, `update pricing from this invoice`, `audit warranty text`, `find duplicate parts in the pricebook`, `find SKUs missing a 2M part number`, `rewrite this pricebook description`, `find a part for`, `pricebook health check`, `which prices drifted`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - servicetitan-pricebook-pp-cli
---

# ServiceTitan Pricebook — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `servicetitan-pricebook-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install servicetitan-pricebook --cli-only
   ```
2. Verify: `servicetitan-pricebook-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI (servicetitan-pricebook-pp-cli) mirrors every ServiceTitan Pricebook v2 endpoint (categories, client-specific pricing, discounts & fees, equipment, materials, materials markup, bulk operations, images, services, export feeds) and adds twelve novel commands that join across them in a local SQLite store. It snapshots cost and price history on every sync, so markup-audit, cost-drift, and reprice become one-shot. It is part of a per-module ST CLI family (servicetitan-crm, servicetitan-dispatch, servicetitan-inventory, servicetitan-jpm, servicetitan-pricebook) designed to replace the heavy 600+-tool general ServiceTitan MCP with focused, agent-native binaries.

## When to Use This CLI

Use this CLI when an agent or operator needs to inspect or maintain the ServiceTitan Pricebook — costs, prices, markup tiers, vendor part numbers, warranty text, category taxonomy — without loading the 600+-tool general ST MCP. It is the right choice for margin-discipline checks after vendor cost changes, vendor-quote reconciliation, and pricebook hygiene audits. Run sync first; the novel audit commands read the local store.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Margin discipline
- **`markup-audit`** — Find every material and equipment SKU whose actual markup has drifted off the materials-markup tier ladder.

  _Reach for this after any vendor cost change to catch SKUs that are now mispriced for their tier before margin erodes._

  ```bash
  servicetitan-pricebook-pp-cli markup-audit --tolerance 5 --agent
  ```
- **`cost-drift`** — Show every SKU whose cost moved since a given date, with old and new cost, old and new price, and whether the price followed.

  _Reach for this to see whether recent vendor cost increases were actually passed through to price._

  ```bash
  servicetitan-pricebook-pp-cli cost-drift --since 2026-04-01 --agent
  ```
- **`reprice`** — Compute the tier-correct price for markup-drifted SKUs and emit the exact update payloads; preview by default, --apply to write.

  _Reach for this after markup-audit to apply hold-markup pricing in one pass instead of editing each SKU by hand._

  ```bash
  servicetitan-pricebook-pp-cli reprice --tolerance 5 --agent
  ```

### Pricebook hygiene
- **`vendor-part-gaps`** — List materials and equipment whose primary vendor part number is empty — the missing 2M Part # sweep.

  _Reach for this to enforce the 2M Part # discipline before a vendor-quote reconcile, which matches on vendor part._

  ```bash
  servicetitan-pricebook-pp-cli vendor-part-gaps --kind material --agent
  ```
- **`warranty-lint`** — Flag equipment whose warranty text is not prefixed Manufacturer's, or that is missing JKA's 1-year parts & labor line.

  _Reach for this to keep warranty attribution honest — manufacturer warranties clearly labeled, JKA's own offering listed alongside._

  ```bash
  servicetitan-pricebook-pp-cli warranty-lint --agent
  ```
- **`orphan-skus`** — List materials, equipment, and services assigned to inactive or non-existent categories.

  _Reach for this during category taxonomy cleanup to find SKUs that fell out of the visible tree._

  ```bash
  servicetitan-pricebook-pp-cli orphan-skus --agent
  ```
- **`dedupe`** — Cluster near-duplicate materials and equipment so excess pricebook growth can be collapsed.

  _Reach for this to find redundant parts before they multiply — duplicate SKUs scatter estimates and inventory across entries that should be one._

  ```bash
  servicetitan-pricebook-pp-cli dedupe --kind material --min-score 0.8 --agent
  ```
- **`copy-audit`** — Flag SKUs whose display name or description is empty, too short, ALL-CAPS, a bare part number, or otherwise not customer-facing.

  _Reach for this to find pricebook entries that read like internal shorthand instead of customer-facing sales copy._

  ```bash
  servicetitan-pricebook-pp-cli copy-audit --kind equipment --agent
  ```

### Vendor quote workflow
- **`quote-reconcile`** — Match a vendor cost file against synced SKUs by vendor part number and print a no-write diff of proposed cost changes; accepts CSV or Claude-extracted JSON from a quote, order confirmation, or invoice PDF.

  _Reach for this when a vendor quote, order confirmation, or invoice lands; have Claude extract it to JSON, then reconcile against current costs before touching the pricebook._

  ```bash
  servicetitan-pricebook-pp-cli quote-reconcile ./2m-quote-2026-05.csv --agent
  ```
- **`bulk-plan`** — Turn a reviewed cost or copy change file into a single pricebook bulk-update payload instead of N individual update calls.

  _Reach for this to apply a batch of reconciled changes in one rate-limit-friendly call._

  ```bash
  servicetitan-pricebook-pp-cli bulk-plan ./reviewed-changes.csv --agent
  ```

### Agent-native plumbing
- **`health`** — One compact rollup of markup-drift, vendor-part-gap, warranty-lint, cost-drift, orphan-SKU, duplicate, and weak-copy counts.

  _Reach for this first in any pricebook session to see what needs attention before drilling into a specific audit._

  ```bash
  servicetitan-pricebook-pp-cli health --agent
  ```
- **`find`** — Forgiving ranked search over synced SKUs tuned for describing a part you do not know the code for.

  _Reach for this when a tech describes a part in plain words — it returns suggested SKUs with the code, price, vendor part, and category they need to pick one._

  ```bash
  servicetitan-pricebook-pp-cli find "1 hp submersible pump motor" --agent
  ```

## Command Reference

**categories** — Manage categories

- `servicetitan-pricebook-pp-cli categories create` — Post to add a new category to your pricebook
- `servicetitan-pricebook-pp-cli categories delete` — Deletes an existing category from your pricebook
- `servicetitan-pricebook-pp-cli categories get` — Gets category details
- `servicetitan-pricebook-pp-cli categories get-list` — GET the categories in your pricebook
- `servicetitan-pricebook-pp-cli categories update` — Edits an existing category in your pricebook

**clientspecificpricing** — Manage clientspecificpricing

- `servicetitan-pricebook-pp-cli clientspecificpricing client-specific-pricing-get-all-rate-sheets` — Client specific pricing_get all rate sheets
- `servicetitan-pricebook-pp-cli clientspecificpricing client-specific-pricing-update-rate-sheet` — Client specific pricing_update rate sheet

**discounts-and-fees** — Manage discounts and fees

- `servicetitan-pricebook-pp-cli discounts-and-fees discount-and-fees-create` — Post to add a new discount or fee to your pricebook
- `servicetitan-pricebook-pp-cli discounts-and-fees discount-and-fees-delete` — Deletes a discount or fee from your pricebook
- `servicetitan-pricebook-pp-cli discounts-and-fees discount-and-fees-get` — Get details of a discount or fee in the pricebook.
- `servicetitan-pricebook-pp-cli discounts-and-fees discount-and-fees-get-list` — Get data on all of the discounts or fees in the pricebook. Supports optional search filtering.
- `servicetitan-pricebook-pp-cli discounts-and-fees discount-and-fees-update` — Edit an existing item in your pricebook

**equipment** — Manage equipment

- `servicetitan-pricebook-pp-cli equipment create` — Post to add new equipment to your pricebook
- `servicetitan-pricebook-pp-cli equipment delete` — Deletes equipment from your pricebook
- `servicetitan-pricebook-pp-cli equipment get` — Get details of equipment in the pricebook.
- `servicetitan-pricebook-pp-cli equipment get-list` — Get data on all the equipment in the pricebook. Supports optional search filtering.
- `servicetitan-pricebook-pp-cli equipment update` — Edit an existing item in your pricebook

**images** — Manage images

- `servicetitan-pricebook-pp-cli images get` — Downloads a specified pricebook image.
- `servicetitan-pricebook-pp-cli images post` — Uploads a specified image to temporary storage. To associate the image with a pricebook item, send a separate...

**materials** — Manage materials

- `servicetitan-pricebook-pp-cli materials create` — Add a new Materials to your pricebook
- `servicetitan-pricebook-pp-cli materials delete` — Deletes a material from your pricebook
- `servicetitan-pricebook-pp-cli materials get` — Get details on a material in the pricebook.
- `servicetitan-pricebook-pp-cli materials get-cost-types` — Get details on materials in the pricebook.
- `servicetitan-pricebook-pp-cli materials get-list` — Get details on materials in the pricebook. Supports optional search filtering.
- `servicetitan-pricebook-pp-cli materials update` — Edit an existing item in your pricebook

**materialsmarkup** — Manage materialsmarkup

- `servicetitan-pricebook-pp-cli materialsmarkup materials-markup-create` — Create materials markup item
- `servicetitan-pricebook-pp-cli materialsmarkup materials-markup-get` — Get materials markup item
- `servicetitan-pricebook-pp-cli materialsmarkup materials-markup-get-list` — Get materials markup collection
- `servicetitan-pricebook-pp-cli materialsmarkup materials-markup-update` — Update materials markup item

**pricebook** — Manage pricebook

- `servicetitan-pricebook-pp-cli pricebook bulk-create` — Pricebook bulk_create
- `servicetitan-pricebook-pp-cli pricebook bulk-update` — Pricebook bulk_update

**pricebook-export** — Manage pricebook export

- `servicetitan-pricebook-pp-cli pricebook-export categories` — Provides export feed for categories
- `servicetitan-pricebook-pp-cli pricebook-export equipment` — Provides export feed for equipment
- `servicetitan-pricebook-pp-cli pricebook-export materials` — Provides export feed for materials
- `servicetitan-pricebook-pp-cli pricebook-export services` — Provides export feed for services

**services** — Manage services

- `servicetitan-pricebook-pp-cli services create` — Post to add a new service to your pricebook
- `servicetitan-pricebook-pp-cli services delete` — Deletes a service from your pricebook
- `servicetitan-pricebook-pp-cli services get` — Get details a service in the pricebook.
- `servicetitan-pricebook-pp-cli services get-list` — Get data on all of the services in the pricebook. Supports optional search filtering.
- `servicetitan-pricebook-pp-cli services update` — Edit an existing item in your pricebook


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
servicetitan-pricebook-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Post-vendor-quote margin check

```bash
servicetitan-pricebook-pp-cli markup-audit --tolerance 5 --agent
```

Sync the latest pricebook, then list every SKU whose markup drifted more than 5% off its tier.

### Reconcile a 2M vendor quote

```bash
servicetitan-pricebook-pp-cli quote-reconcile ./2m-quote-2026-05.csv --agent
```

Match the quote's vendor part numbers against synced SKUs and print the proposed cost diff without writing.

### Narrow a large material list for an agent

```bash
servicetitan-pricebook-pp-cli materials get-list 42 --active True --agent --select results.data.code,results.data.cost,results.data.primaryVendor.vendorPart
```

Materials responses are large and deeply nested; --select with dotted paths returns only the fields an agent needs to reason about cost and vendor part.

### Pre-session pricebook triage

```bash
servicetitan-pricebook-pp-cli health --agent
```

One rollup of every audit count so an agent knows where to look before drilling in.

### Find SKUs missing a 2M Part #

```bash
servicetitan-pricebook-pp-cli vendor-part-gaps --kind material --agent
```

List materials whose primary vendor part number is empty so they can be filled before the next reconcile.

## Auth Setup

ServiceTitan uses composed auth: a static ST-App-Key header (set ST_APP_KEY) plus an OAuth2 client-credentials bearer token (set ST_CLIENT_ID and ST_CLIENT_SECRET). Run servicetitan-pricebook-pp-cli auth login to walk the credential setup, then set the three credentials plus ST_TENANT_ID (your numeric tenant ID) in your environment. The client mints and refreshes the bearer token automatically and attaches both headers to every call. Whitespace is stripped defensively from credentials — a known JKA gotcha where a trailing newline in an env file produced opaque invalid_client 400s.

Run `servicetitan-pricebook-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  servicetitan-pricebook-pp-cli categories get mock-value mock-value --agent --select id,name,status
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
servicetitan-pricebook-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
servicetitan-pricebook-pp-cli feedback --stdin < notes.txt
servicetitan-pricebook-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.servicetitan-pricebook-pp-cli/feedback.jsonl`. They are never POSTed unless `SERVICETITAN_PRICEBOOK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SERVICETITAN_PRICEBOOK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
servicetitan-pricebook-pp-cli profile save briefing --json
servicetitan-pricebook-pp-cli --profile briefing categories get mock-value mock-value
servicetitan-pricebook-pp-cli profile list --json
servicetitan-pricebook-pp-cli profile show briefing
servicetitan-pricebook-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `servicetitan-pricebook-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add servicetitan-pricebook-pp-mcp -- servicetitan-pricebook-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which servicetitan-pricebook-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   servicetitan-pricebook-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `servicetitan-pricebook-pp-cli <command> --help`.
