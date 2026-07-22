---
name: pp-amazon-seller
description: "Printing Press CLI for Amazon Seller. Read FBA inventory, orders, sales reports, listings, and catalog data for an Amazon seller account."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - amazon-seller-pp-cli
    install:
      - kind: go
        bins: [amazon-seller-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/amazon-seller/cmd/amazon-seller-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/commerce/amazon-seller/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Amazon Seller — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `amazon-seller-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install amazon-seller --cli-only
   ```
2. Verify: `amazon-seller-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/amazon-seller/cmd/amazon-seller-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Command Reference

**catalog** — Read Catalog Items API item data.

- `amazon-seller-pp-cli catalog get` — Get one catalog item by ASIN.
- `amazon-seller-pp-cli catalog search` — Search catalog items. Provide marketplaceIds plus one valid search mode such as keywords or identifiers with...

**fba-inventory** — Inspect Fulfillment by Amazon inventory summaries.

- `amazon-seller-pp-cli fba-inventory` — List FBA inventory summaries. For North America marketplace-level inventory, pass granularityType=Marketplace,...

**listings** — Read Listings Items API data for seller SKUs.

- `amazon-seller-pp-cli listings get` — Get one listing item by seller ID and SKU.
- `amazon-seller-pp-cli listings search` — Search listing items for a seller.

**orders** — Search and inspect Orders API v2026-01-01 order records.

- `amazon-seller-pp-cli orders get` — Get one Orders API v2026-01-01 order.
- `amazon-seller-pp-cli orders search` — Search orders. Provide exactly one of createdAfter or lastUpdatedAfter; Amazon returns 400 for invalid combinations.

**reports** — Create reports, poll report status, and inspect report document metadata.

- `amazon-seller-pp-cli reports create` — Create a report request. Prefer --stdin for JSON bodies so marketplaceIds remains a JSON array and reportOptions...
- `amazon-seller-pp-cli reports document` — Get report document metadata and the presigned download URL. This command does not download or open the document.
- `amazon-seller-pp-cli reports get` — Get one report by report ID. This is the manual polling endpoint for report processing status.
- `amazon-seller-pp-cli reports list` — List reports. If nextToken is set, Amazon requires it to be the only query parameter; pass no other filters with...

**inbound-plans** — Create and manage Fulfillment Inbound v2024-03-20 inbound plans.

- `amazon-seller-pp-cli inbound-plans create --marketplace-id ATVPDKIKX0DER --source-address address.json --items items.csv --name "June FBA"` — Preview a createInboundPlan body from CSV/JSON input. **Write command:** pass `--yes` to send, `--dry-run` to render the HTTP request, and `--wait` to poll the returned operation.
- `amazon-seller-pp-cli inbound-plans create --stdin` — Read Amazon's exact createInboundPlan JSON request body from stdin. **Write command:** pass `--yes` to send.
- `amazon-seller-pp-cli inbound-plans status --operation-id <operationId> --wait` — Poll getInboundOperationStatus for asynchronous Fulfillment Inbound POST/PUT operations.
- `amazon-seller-pp-cli inbound-plans get <inboundPlanId>` — Get one inbound plan.
- `amazon-seller-pp-cli inbound-plans list --status ACTIVE --page-size 10` — List inbound plans.
- `amazon-seller-pp-cli inbound-plans cancel <inboundPlanId> --yes` — Cancel an inbound plan. **Write command:** requires `--yes` unless `--dry-run` is set.
- `amazon-seller-pp-cli inbound-plans packing generate --inbound-plan-id <id>` — Generate packing options. **Write command:** use `--dry-run` for request preview.
- `amazon-seller-pp-cli inbound-plans packing list --inbound-plan-id <id>` — List packing options.
- `amazon-seller-pp-cli inbound-plans packing confirm --inbound-plan-id <id> --option-id <packingOptionId> --yes` — Confirm a packing option. **Write command:** requires `--yes` unless `--dry-run` is set.
- `amazon-seller-pp-cli inbound-plans packing set --inbound-plan-id <id> --body cartons.json --yes` — Set carton-level packing information. **Write command:** requires `--yes` unless `--dry-run` is set.
- `amazon-seller-pp-cli inbound-plans placement generate --inbound-plan-id <id> --body placement.json` — Generate placement options, optionally with customPlacement JSON. **Write command:** pass `--yes` when providing a body to send instead of previewing.
- `amazon-seller-pp-cli inbound-plans placement list --inbound-plan-id <id>` — List placement options.
- `amazon-seller-pp-cli inbound-plans placement confirm --inbound-plan-id <id> --option-id <placementOptionId> --yes` — Confirm a placement option. **Write command:** requires `--yes` unless `--dry-run` is set.
- `amazon-seller-pp-cli inbound-plans transportation generate --inbound-plan-id <id> --body transportation.json` — Generate transportation options from placement/shipment JSON. **Write command:** pass `--yes` when providing a body to send instead of previewing.
- `amazon-seller-pp-cli inbound-plans transportation list --inbound-plan-id <id> --placement-option-id <placementOptionId>` — List transportation options.
- `amazon-seller-pp-cli inbound-plans transportation confirm --inbound-plan-id <id> --body selections.json --yes` — Confirm transportation selections. **Write command:** requires `--yes` unless `--dry-run` is set.

For CSV item input, use columns `msku,quantity,prepOwner,labelOwner,expiration,manufacturingLotCode`. For the US marketplace `ATVPDKIKX0DER`, the CLI warns when `prepOwner=AMAZON` or `labelOwner=AMAZON`, because Amazon says US FBA prep and item label services are no longer available starting January 1, 2026.

**profitability** — Compute estimated SKU profitability from Amazon reports.

- `amazon-seller-pp-cli profitability sku-pnl --marketplace-id ATVPDKIKX0DER --days 30` — Estimate per-SKU revenue, fees, storage cost, margin, and profit.
- `amazon-seller-pp-cli profitability fee-breakdown --marketplace-id ATVPDKIKX0DER` — Show itemized estimated referral, FBA, closing, and total fee percentages.
- `amazon-seller-pp-cli profitability settlement-reconciliation --marketplace-id ATVPDKIKX0DER --days 90` — Compare order revenue to completed settlement rows and flag discrepancies.
- `amazon-seller-pp-cli profitability reimbursements --marketplace-id ATVPDKIKX0DER --days 90` — Aggregate reimbursements by SKU and reason.

**inventory-intel** — Compute FBA inventory health, restock, aging, and fulfillment recommendations.

- `amazon-seller-pp-cli inventory-intel health-score --marketplace-id ATVPDKIKX0DER` — Score inventory health per SKU.
- `amazon-seller-pp-cli inventory-intel restock --marketplace-id ATVPDKIKX0DER --lead-time-days 14` — Estimate stockout timing and reorder quantities.
- `amazon-seller-pp-cli inventory-intel stranded --marketplace-id ATVPDKIKX0DER` — Surface stranded inventory rows and recommended actions.
- `amazon-seller-pp-cli inventory-intel aging --marketplace-id ATVPDKIKX0DER --warn-days 150` — Forecast long-term storage fee risk.
- `amazon-seller-pp-cli inventory-intel fba-vs-fbm --marketplace-id ATVPDKIKX0DER --estimated-shipping-cost 5` — Compare estimated FBA vs FBM per-unit profit.

**sales-intel** — Compute sales, traffic, conversion, velocity, and returns analytics.

- `amazon-seller-pp-cli sales-intel dashboard --marketplace-id ATVPDKIKX0DER --days 30 --group-by date` — Summarize Sales and Traffic report metrics.
- `amazon-seller-pp-cli sales-intel velocity --marketplace-id ATVPDKIKX0DER --days 60` — Detect SKU velocity trends and anomalies.
- `amazon-seller-pp-cli sales-intel returns --marketplace-id ATVPDKIKX0DER --days 30` — Analyze return rate, reasons, and customer comments.
- `amazon-seller-pp-cli sales-intel conversion-funnel --marketplace-id ATVPDKIKX0DER --days 30` — Diagnose traffic, buy-box, and conversion bottlenecks.

**brand-analytics** — Compute Brand Registry search-term and basket insights.

- `amazon-seller-pp-cli brand-analytics search-terms --marketplace-id ATVPDKIKX0DER --period WEEK` — Show search terms where tracked ASINs appear in top click positions.
- `amazon-seller-pp-cli brand-analytics market-basket --marketplace-id ATVPDKIKX0DER --period WEEK` — Show products frequently purchased together with your ASINs.

**listing-intel** — Audit listing defects and completeness from reports plus local listing data.

- `amazon-seller-pp-cli listing-intel health-audit --marketplace-id ATVPDKIKX0DER` — Aggregate listing defect alerts and suppression signals.
- `amazon-seller-pp-cli listing-intel catalog-completeness --marketplace-id ATVPDKIKX0DER --seller-id <seller>` — Score image, bullet, title, and A+ content completeness from local listing records.

**account-health** — Summarize account-level listing, inventory, returns, and reimbursement health.

- `amazon-seller-pp-cli account-health dashboard --marketplace-id ATVPDKIKX0DER` — Produce one dashboard with suppressed listings, stranded units, return rate, reimbursements, defect counts, and inventory grade summary.

**sellers** — Verify seller authorization and list marketplace participations.

- `amazon-seller-pp-cli sellers` — List marketplace participations for the authorized seller account.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
amazon-seller-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Self-authorize your private application in the provider console, export the OAuth client ID, OAuth client secret, and refresh token, then run doctor:

```bash
export SP_API_LWA_CLIENT_ID="<client-id>"
export SP_API_LWA_CLIENT_SECRET="<client-secret>"
export SP_API_REFRESH_TOKEN="<refresh-token>"
amazon-seller-pp-cli doctor
```

The CLI exchanges the refresh token for an access token on first live use and caches the access token locally.

Run `amazon-seller-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  amazon-seller-pp-cli catalog get <asin> --marketplace-ids ATVPDKIKX0DER --agent --select asin,attributes,summaries
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
amazon-seller-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
amazon-seller-pp-cli feedback --stdin < notes.txt
amazon-seller-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.amazon-seller-pp-cli/feedback.jsonl`. They are never POSTed unless `AMAZON_SELLER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AMAZON_SELLER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
amazon-seller-pp-cli profile save briefing --json
amazon-seller-pp-cli --profile briefing sellers marketplaces
amazon-seller-pp-cli profile list --json
amazon-seller-pp-cli profile show briefing
amazon-seller-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `amazon-seller-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)
## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/amazon-seller/cmd/amazon-seller-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add amazon-seller-pp-mcp -- amazon-seller-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which amazon-seller-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   amazon-seller-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `amazon-seller-pp-cli <command> --help`.
