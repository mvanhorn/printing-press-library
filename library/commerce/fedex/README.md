# FedEx CLI

**REST-native FedEx CLI for small business shippers, with rate-shopping, bulk CSV labels, an address book, and a local SQLite ledger no other tool has.**

FedEx retires its SOAP APIs on June 1, 2026 and every existing wrapper goes dark. fedex-pp-cli ships the full FedEx OAuth2 REST surface as a single static Go binary. For small-business shippers it adds the four things SaaS competitors charge for: rate-shopping across every service type, bulk-CSV label printing with adaptive rate limits, a local address book, and a SQLite ledger that powers spend reports, archive search, and accounting export.

Learn more at [FedEx](https://developer.fedex.com).

## Install

The recommended path installs both the `fedex-pp-cli` binary and the `pp-fedex` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install fedex
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install fedex --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/fedex/cmd/fedex-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fedex-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

## Authentication

FedEx uses OAuth2 client_credentials. Run `fedex auth login` once with your sandbox or production Client ID and Client Secret; the CLI mints a 1-hour bearer token, caches it on disk, and refreshes proactively before expiry. Production label printing requires per-project Bar Code Analysis Group (BAG) approval from FedEx — `fedex doctor` surfaces approval status before you hit it.

## Quick Start

```bash
# Mint a bearer token from FedEx OAuth2 client_credentials and cache it
fedex auth login --client-id $FEDEX_API_KEY --client-secret $FEDEX_SECRET_KEY --env sandbox


# Verify auth, sandbox/prod routing, and surface any BAG approval gaps before you hit them at ship-time
fedex doctor


# Compare every applicable service type ranked by cost — the most useful one-liner this CLI offers
fedex rate shop --from 90210 --to 10001 --weight 5lb --json --select rates.serviceType,rates.totalNetCharge


# Save a recipient to the local address book so you do not retype it for every shipment
fedex address save --name acme --street '500 Main St' --city Denver --state CO --zip 80202 --country US


# Batch-create labels with adaptive rate limiting; resumable on partial failure
fedex ship bulk --csv orders.csv --service GROUND --resume

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Save money, save time
- **`rate shop`** — Quote rates across every applicable service type in parallel and rank by cost, transit days, or cost-per-day.

  _When picking the cheapest viable service for a shipment, this collapses 6+ API calls into one ranked answer. The headline cost-saving command for SMB shippers._

  ```bash
  fedex rate shop --from 90210 --to 10001 --weight 5lb --json --select rates.serviceType,rates.totalNetCharge,rates.transitTime
  ```
- **`ship bulk`** — Create labels for a CSV of orders with adaptive rate limiting, per-row PASS/FAIL accounting, and resume-from-last-success.

  _The 'I ship 30 packages a day' workflow. Replaces clicking through FedEx Ship Manager one label at a time._

  ```bash
  fedex ship bulk --csv orders.csv --service GROUND --output labels/ --resume
  ```
- **`return create`** — Generate a Ground Call Tag (return label) against an existing tracking number with one command, optionally emailing it to the recipient.

  _E-commerce customer-service workflow. Issuing a return label is the most common post-sale interaction._

  ```bash
  fedex return create --tracking 794633071234 --reason damaged --email customer@example.com
  ```
- **`address validate`** — SHA-256-keyed local cache prevents re-billing the FedEx Address Validation API for repeat lookups.

  _Direct cost savings — the only feature with a quantifiable per-call $ impact._

  ```bash
  fedex address validate --street '1600 Amphitheatre Pkwy' --city 'Mountain View' --state CA --zip 94043 --country US --cache
  ```
- **`ship etd`** — Single-command Electronic Trade Documents shipping: uploads commercial invoice, captures docId, and stitches it into the shipment create call.

  _International shipments require ETD; this collapses an error-prone multi-step into one atomic action._

  ```bash
  fedex ship etd --invoice invoice.pdf --orig CN --dest US --recipient-name 'Acme' --weight 2kg --service FEDEX_INTERNATIONAL_PRIORITY
  ```

### Local state that compounds
- **`address save`** — Save frequently-used recipients to a local address book; reference them by name in ship commands.

  _Ergonomic parity with paid SaaS competitors. Eliminates retyping addresses for repeat customers._

  ```bash
  fedex address save acme --street '500 Main St' --city Denver --state CO --zip 80202 --country US
  ```
- **`track diff`** — Show only the tracking events that have appeared since the last poll for each tracking number in the local store.

  _Replaces tracking-poll dedupe glue agents would otherwise write._

  ```bash
  fedex track diff --since 1h --json
  ```
- **`track watch`** — Continuously poll a set of tracking numbers and emit new events to stdout, a webhook, or a file as they arrive. Polling alternative to FedEx push notifications.

  _Most SMBs don't have provisioned push webhooks. Polling daemon is the universal alternative._

  ```bash
  fedex track watch --tracking 794633071234 --interval 10m --webhook https://example.com/hook
  ```
- **`archive`** — SQLite FTS5 search across every shipment in the local archive — recipient name, address, reference, tracking number, service.

  _'Did we ship to ACME last week?' — a question SMBs ask constantly._

  ```bash
  fedex archive 'warehouse 47' --service GROUND --json
  ```
- **`spend report`** — Sum of net charges per service type, lane, or account from the local rate-quote and shipment ledger.

  _'How much did I spend on FedEx last month?' — the question every SMB owner asks._

  ```bash
  fedex spend report --since 30d --by service --json
  ```
- **`export`** — Dump shipments + charges + tracking events as CSV or JSON for QuickBooks/Xero reconciliation.

  _Closes the loop on accounting reconciliation without manual data entry._

  ```bash
  fedex export --format csv --since 30d --output fedex-april.csv
  ```
- **`manifest`** — Generate a printable PDF/text summary of every shipment created today from the local archive, optionally invoking the Ground EOD close API.

  _Closes the warehouse-day workflow loop in one command._

  ```bash
  fedex manifest --date 2026-05-02 --close --output manifest.md
  ```
- **`sql`** — Direct SQLite SELECT queries against shipments, rate_quotes, tracking_events, address_validations, addresses tables.

  _Escape hatch for arbitrary analytics over the local ledger._

  ```bash
  fedex sql "select serviceType, count(*) as n, sum(net_charge) as spend from shipments where created_at > date('now','-30 days') group by serviceType order by spend desc"
  ```

### Setup smoothness
- **`doctor`** — Verifies OAuth auth, sandbox/prod routing, account-number format, label-format compatibility, and surfaces BAG (Bar Code Analysis Group) approval status.

  _Avoids the most common 'why won't my labels print' failure mode for first-time users._

  ```bash
  fedex doctor
  ```

## Usage

Run `fedex-pp-cli --help` for the full command reference and flag list.

## Commands

### addresses

Address validation

- **`fedex-pp-cli addresses validate`** - Validate one or more addresses (resolved/standardized form, classification, optional resolved coordinates)

### availability

Service availability, special-service options, and transit times

- **`fedex-pp-cli availability services`** - Get available services for an origin/destination pair
- **`fedex-pp-cli availability special_services`** - Get available special-service options (alcohol, dangerous goods, signature requirements, etc.)
- **`fedex-pp-cli availability transit_times`** - Get transit times for an origin/destination pair across services

### consolidations

Consolidate multiple shipments into one fulfillment (advanced; for high-volume 3PLs)

- **`fedex-pp-cli consolidations add_shipment`** - Add shipments to a consolidation
- **`fedex-pp-cli consolidations confirm`** - Confirm a consolidation (locks rates and triggers shipping)
- **`fedex-pp-cli consolidations confirm_results`** - Get results of a consolidation confirmation
- **`fedex-pp-cli consolidations create`** - Create a consolidation
- **`fedex-pp-cli consolidations delete`** - Delete a consolidation
- **`fedex-pp-cli consolidations delete_shipment`** - Remove a shipment from a consolidation
- **`fedex-pp-cli consolidations modify`** - Modify a consolidation
- **`fedex-pp-cli consolidations results`** - Get consolidation results
- **`fedex-pp-cli consolidations retrieve`** - Retrieve a consolidation

### endofday

Submit and modify Ground end-of-day close (daily manifest)

- **`fedex-pp-cli endofday close`** - Submit Ground end-of-day close (manifest packages shipped today)
- **`fedex-pp-cli endofday modify`** - Re-submit or modify a previous end-of-day close

### freight_pickups

Schedule, check, and cancel freight (LTL) pickups

- **`fedex-pp-cli freight_pickups availability`** - Check freight pickup availability
- **`fedex-pp-cli freight_pickups cancel`** - Cancel a freight pickup
- **`fedex-pp-cli freight_pickups create`** - Schedule a freight (LTL) pickup

### freight_shipments

Create freight (LTL) shipments (for industrial shippers)

- **`fedex-pp-cli freight_shipments create`** - Create a freight (LTL) shipment

### globaltrade

Global trade regulatory details (HS codes, restrictions)

- **`fedex-pp-cli globaltrade regulatory`** - Get regulatory details (HS codes, harmonized system, country restrictions)

### locations

Find FedEx Office, dropoff, and pickup locations

- **`fedex-pp-cli locations find`** - Find FedEx locations near an address or postal code

### openship

Build multi-piece shipments progressively before confirming (advanced; not needed for typical SMB workflows)

- **`fedex-pp-cli openship add_package`** - Add a package to an open shipment
- **`fedex-pp-cli openship create`** - Create an open (uncommitted) multi-piece shipment
- **`fedex-pp-cli openship delete`** - Delete an open shipment
- **`fedex-pp-cli openship delete_package`** - Delete a package from an open shipment
- **`fedex-pp-cli openship modify`** - Modify an open shipment
- **`fedex-pp-cli openship modify_package`** - Modify a package in an open shipment
- **`fedex-pp-cli openship results`** - Get results of a confirmed open shipment
- **`fedex-pp-cli openship retrieve`** - Retrieve an open shipment by index
- **`fedex-pp-cli openship retrieve_package`** - Retrieve a specific package from an open shipment

### pickups

Schedule, check availability, and cancel Express/Ground pickups

- **`fedex-pp-cli pickups availability`** - Check whether pickup is available for a postal code on a date
- **`fedex-pp-cli pickups cancel`** - Cancel a previously-scheduled Express/Ground pickup
- **`fedex-pp-cli pickups create`** - Schedule an Express or Ground pickup at a specified address

### postal

Postal code validation and country servicing

- **`fedex-pp-cli postal validate`** - Validate that FedEx services a postal code in a country

### rates

Rate quotes for Express, Ground, Home, Ground Economy, and freight

- **`fedex-pp-cli rates quote`** - Quote rates for a shipment (Express/Ground/Home/Ground Economy)
- **`fedex-pp-cli rates quote_freight`** - Quote rates for freight (LTL) shipments

### shipments

Create, validate, cancel, and retrieve shipments

- **`fedex-pp-cli shipments cancel`** - Cancel a shipment by tracking number
- **`fedex-pp-cli shipments create`** - Create a shipment and generate label(s)
- **`fedex-pp-cli shipments results`** - Retrieve results of an asynchronous shipment job
- **`fedex-pp-cli shipments tag`** - Create a Ground Call Tag (return label) for an existing shipment
- **`fedex-pp-cli shipments tag_cancel`** - Cancel a Ground Call Tag
- **`fedex-pp-cli shipments validate`** - Validate a shipment package without creating a label (catches rejections before billing)

### track

Track shipments by tracking number, reference, TCN, or associated shipment; retrieve documents and configure notifications

- **`fedex-pp-cli track associated`** - Track shipments associated with a master tracking number (multi-piece)
- **`fedex-pp-cli track documents`** - Retrieve tracking documents (signature proof of delivery, etc.)
- **`fedex-pp-cli track notifications`** - Configure tracking notifications (email/SMS) for a shipment
- **`fedex-pp-cli track number`** - Track up to 30 tracking numbers in one call (returns full event timeline per shipment)
- **`fedex-pp-cli track reference`** - Track by reference number (PO/customer ref/RMA)
- **`fedex-pp-cli track tcn`** - Track by Transportation Control Number (military/government use)


## Cookbook

Patterns combining multiple commands to solve real shipping workflows.

### Pick the cheapest service for a single shipment

```bash
fedex-pp-cli rate shop --from 98101 --to 10001 --weight 5lb \
  --json --select 'rates.service_type,rates.net_amount,rates.transit_days' \
  --account "$FEDEX_ACCOUNT_NUMBER"
```

The `selected=1` row is the cheapest viable service. Each quote is also persisted to the local `rate_quotes` ledger so spend reports can attribute follow-on shipments.

### Compare cost vs transit

```bash
fedex-pp-cli rate shop --from 98101 --to 10001 --weight 12 --rank-by transit --json
```

Rank by `--rank-by transit` to surface fastest service first, then inspect `net_amount` to see the cost premium.

### Bulk-ship from a CSV with safe resume

```bash
# First run — adaptive rate limiting, low concurrency, write labels to ./labels/
fedex-pp-cli ship bulk --csv orders.csv --service FEDEX_GROUND \
  --output ./labels --concurrency 3 --account "$FEDEX_ACCOUNT_NUMBER"

# Network blip? Re-run with --resume; rows already in the archive are skipped
fedex-pp-cli ship bulk --csv orders.csv --resume --account "$FEDEX_ACCOUNT_NUMBER"
```

### Capture only-new tracking events for a webhook

```bash
# One-shot diff — ideal for a cron job or CI step
fedex-pp-cli track diff 794633071234 794633071235 --json

# Long-running watch with webhook fan-out
fedex-pp-cli track watch --tracking 794633071234 \
  --interval 5m --webhook https://hooks.example.com/track
```

### Issue a return label and email it to the customer

```bash
fedex-pp-cli return create --tracking 794633071234 \
  --reason damaged --email customer@example.com \
  --account "$FEDEX_ACCOUNT_NUMBER"
```

### Cache address validations to avoid re-billing

```bash
fedex-pp-cli address validate \
  --street '1600 Amphitheatre Pkwy' --city 'Mountain View' \
  --state CA --zip 94043 --country US --cache
```

The SHA-256-keyed local cache returns the same FedEx-validated payload on repeat calls without hitting the API.

### Save a recipient and reuse it

```bash
fedex-pp-cli address save acme \
  --contact-name "ACME Corp" --street "1 Anvil Way" \
  --city Burbank --state CA --zip 91505 --country US

# Look it up later
fedex-pp-cli address list --json --select name,city,state
```

### Search the local shipment archive (FTS5)

```bash
# Plain word search across recipient/reference/tracking
fedex-pp-cli archive "acme corp"

# Filter by service and recency, render as table
fedex-pp-cli archive --service FEDEX_GROUND --since 720h --limit 25
```

### Run ad-hoc SQL against the local store

```bash
# Top spend lanes in the last 30 days
fedex-pp-cli sql "
  SELECT origin_postal, dest_postal, SUM(net_amount) AS spend
  FROM rate_quotes
  WHERE created_at >= datetime('now','-30 days')
  GROUP BY 1,2 ORDER BY 3 DESC LIMIT 10"
```

### Build a daily manifest, optionally close end-of-day

```bash
# Markdown manifest of every shipment created today
fedex-pp-cli manifest --output today.md

# Same thing, then call FedEx end-of-day to formally close it
fedex-pp-cli manifest --close
```

### Compose a spend report grouped by service or lane

```bash
fedex-pp-cli spend report --by service --since 720h
fedex-pp-cli spend report --by lane --since 168h --json
```

### Pre-flight check before a bulk run

```bash
# Verify auth, sandbox/prod routing, and BAG approval status
fedex-pp-cli doctor

# Verify a specific call without sending it
fedex-pp-cli ship bulk --csv orders.csv --dry-run
```

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
fedex-pp-cli addresses

# JSON for scripting and agents
fedex-pp-cli addresses --json

# Filter to specific fields
fedex-pp-cli addresses --json --select id,name,status

# Dry run — show the request without sending
fedex-pp-cli addresses --dry-run

# Agent mode — JSON + compact + no prompts in one flag
fedex-pp-cli addresses --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-fedex -g
```

Then invoke `/pp-fedex <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/fedex/cmd/fedex-pp-mcp@latest
```

Then register it:

```bash
claude mcp add fedex fedex-pp-mcp -e FEDEX_API_KEY=<your-token>
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-fedex --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-fedex --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-fedex skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-fedex. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fedex-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FEDEX_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/fedex/cmd/fedex-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "fedex": {
      "command": "fedex-pp-mcp",
      "env": {
        "FEDEX_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
fedex-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/fedex-pp-cli/config.toml`

Environment variables:
- `FEDEX_API_KEY`
- `FEDEX_SECRET_KEY`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `fedex-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FEDEX_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 403 with no body, then no responses for 10 minutes** — You hit the per-IP throttle (1 req/s sustained for 2 min). Use `fedex ship bulk --concurrency 1` or wait 10 minutes; cliutil.AdaptiveLimiter prevents this when used
- **401 NOT.AUTHORIZED.ERROR on every call** — Token expired or credentials wrong. Run `fedex auth login` again with valid Client ID + Client Secret
- **PDF labels won't print on thermal printer** — Pass --label-format ZPLII or EPL2 to ship create; thermal printers can't render PDF directly
- **Production ship returns BAG-not-approved error** — Production label printing requires per-project FedEx Bar Code Analysis Group approval. Run `fedex doctor` for status; submit your BAG application via the FedEx developer portal
- **track diff returns nothing despite known new events** — Run `fedex track number <num>` first to seed the local store; diff compares against what's already cached

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**karrio (purplship)**](https://github.com/karrioapi/karrio) — Python (2000 stars)
- [**python-fedex**](https://github.com/python-fedex-devs/python-fedex) — Python (155 stars)
- [**happyreturns/fedex**](https://github.com/happyreturns/fedex) — Go (3 stars)
- [**asrx/go-fedex-api-wrapper**](https://github.com/asrx/go-fedex-api-wrapper) — Go (1 stars)
- [**markswendsen-code/mcp-fedex**](https://github.com/markswendsen-code/mcp-fedex) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
