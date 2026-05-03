---
name: pp-fedex
description: "REST-native FedEx CLI for small business shippers, with rate-shopping, bulk CSV labels, an address book, and a local SQLite ledger no other tool has. Trigger phrases: `ship a package via FedEx`, `rate-shop FedEx services`, `bulk-print FedEx labels from CSV`, `save a FedEx recipient`, `issue a FedEx return label`, `FedEx spend this month`, `track a FedEx shipment`, `use fedex-pp-cli`, `run fedex`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["fedex-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/commerce/fedex/cmd/fedex-pp-cli@latest","bins":["fedex-pp-cli"],"label":"Install via go install"}]}}'
---

# FedEx — Printing Press CLI

FedEx retires its SOAP APIs on June 1, 2026 and every existing wrapper goes dark. fedex-pp-cli ships the full FedEx OAuth2 REST surface as a single static Go binary. For small-business shippers it adds the four things SaaS competitors charge for: rate-shopping across every service type, bulk-CSV label printing with adaptive rate limits, a local address book, and a SQLite ledger that powers spend reports, archive search, and accounting export.

## When to Use This CLI

Use fedex-pp-cli when you run a small-to-medium shipping operation and want to programmatically interact with the FedEx REST API. Right fit for: e-commerce stores creating labels from order CSVs, customer-service teams generating return labels, ops teams tracking in-flight shipments, and SMB owners who want a local ledger of every shipment and rate quote for accounting. The local SQLite archive makes it the right tool for analytical queries over historical shipments. Choose this CLI over multi-carrier SaaS (ShipStation, Shippo) when you only need FedEx, want to own your data, and prefer terminal/agent automation over web UIs.

## Unique Capabilities

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

## Command Reference

**addresses** — Address validation

- `fedex-pp-cli addresses` — Validate one or more addresses (resolved/standardized form, classification, optional resolved coordinates)

**availability** — Service availability, special-service options, and transit times

- `fedex-pp-cli availability services` — Get available services for an origin/destination pair
- `fedex-pp-cli availability special_services` — Get available special-service options (alcohol, dangerous goods, signature requirements, etc.)
- `fedex-pp-cli availability transit_times` — Get transit times for an origin/destination pair across services

**consolidations** — Consolidate multiple shipments into one fulfillment (advanced; for high-volume 3PLs)

- `fedex-pp-cli consolidations add_shipment` — Add shipments to a consolidation
- `fedex-pp-cli consolidations confirm` — Confirm a consolidation (locks rates and triggers shipping)
- `fedex-pp-cli consolidations confirm_results` — Get results of a consolidation confirmation
- `fedex-pp-cli consolidations create` — Create a consolidation
- `fedex-pp-cli consolidations delete` — Delete a consolidation
- `fedex-pp-cli consolidations delete_shipment` — Remove a shipment from a consolidation
- `fedex-pp-cli consolidations modify` — Modify a consolidation
- `fedex-pp-cli consolidations results` — Get consolidation results
- `fedex-pp-cli consolidations retrieve` — Retrieve a consolidation

**endofday** — Submit and modify Ground end-of-day close (daily manifest)

- `fedex-pp-cli endofday close` — Submit Ground end-of-day close (manifest packages shipped today)
- `fedex-pp-cli endofday modify` — Re-submit or modify a previous end-of-day close

**freight_pickups** — Schedule, check, and cancel freight (LTL) pickups

- `fedex-pp-cli freight_pickups availability` — Check freight pickup availability
- `fedex-pp-cli freight_pickups cancel` — Cancel a freight pickup
- `fedex-pp-cli freight_pickups create` — Schedule a freight (LTL) pickup

**freight_shipments** — Create freight (LTL) shipments (for industrial shippers)

- `fedex-pp-cli freight_shipments` — Create a freight (LTL) shipment

**globaltrade** — Global trade regulatory details (HS codes, restrictions)

- `fedex-pp-cli globaltrade` — Get regulatory details (HS codes, harmonized system, country restrictions)

**locations** — Find FedEx Office, dropoff, and pickup locations

- `fedex-pp-cli locations` — Find FedEx locations near an address or postal code

**openship** — Build multi-piece shipments progressively before confirming (advanced; not needed for typical SMB workflows)

- `fedex-pp-cli openship add_package` — Add a package to an open shipment
- `fedex-pp-cli openship create` — Create an open (uncommitted) multi-piece shipment
- `fedex-pp-cli openship delete` — Delete an open shipment
- `fedex-pp-cli openship delete_package` — Delete a package from an open shipment
- `fedex-pp-cli openship modify` — Modify an open shipment
- `fedex-pp-cli openship modify_package` — Modify a package in an open shipment
- `fedex-pp-cli openship results` — Get results of a confirmed open shipment
- `fedex-pp-cli openship retrieve` — Retrieve an open shipment by index
- `fedex-pp-cli openship retrieve_package` — Retrieve a specific package from an open shipment

**pickups** — Schedule, check availability, and cancel Express/Ground pickups

- `fedex-pp-cli pickups availability` — Check whether pickup is available for a postal code on a date
- `fedex-pp-cli pickups cancel` — Cancel a previously-scheduled Express/Ground pickup
- `fedex-pp-cli pickups create` — Schedule an Express or Ground pickup at a specified address

**postal** — Postal code validation and country servicing

- `fedex-pp-cli postal` — Validate that FedEx services a postal code in a country

**rates** — Rate quotes for Express, Ground, Home, Ground Economy, and freight

- `fedex-pp-cli rates quote` — Quote rates for a shipment (Express/Ground/Home/Ground Economy)
- `fedex-pp-cli rates quote_freight` — Quote rates for freight (LTL) shipments

**shipments** — Create, validate, cancel, and retrieve shipments

- `fedex-pp-cli shipments cancel` — Cancel a shipment by tracking number
- `fedex-pp-cli shipments create` — Create a shipment and generate label(s)
- `fedex-pp-cli shipments results` — Retrieve results of an asynchronous shipment job
- `fedex-pp-cli shipments tag` — Create a Ground Call Tag (return label) for an existing shipment
- `fedex-pp-cli shipments tag_cancel` — Cancel a Ground Call Tag
- `fedex-pp-cli shipments validate` — Validate a shipment package without creating a label (catches rejections before billing)

**track** — Track shipments by tracking number, reference, TCN, or associated shipment; retrieve documents and configure notifications

- `fedex-pp-cli track associated` — Track shipments associated with a master tracking number (multi-piece)
- `fedex-pp-cli track documents` — Retrieve tracking documents (signature proof of delivery, etc.)
- `fedex-pp-cli track notifications` — Configure tracking notifications (email/SMS) for a shipment
- `fedex-pp-cli track number` — Track up to 30 tracking numbers in one call (returns full event timeline per shipment)
- `fedex-pp-cli track reference` — Track by reference number (PO/customer ref/RMA)
- `fedex-pp-cli track tcn` — Track by Transportation Control Number (military/government use)


**Hand-written commands**

- `fedex-pp-cli auth login [--client-id <id>] [--client-secret <secret>] [--env sandbox|prod]` — Mint an OAuth2 bearer token via client_credentials and cache it on disk
- `fedex-pp-cli auth logout` — Clear the cached bearer token
- `fedex-pp-cli auth status` — Show the cached token's expiry and which env vars were detected
- `fedex-pp-cli rate shop --from <zip> --to <zip> --weight <n>[lb|kg]` — Quote rates across every applicable service type in parallel and rank by cost or transit
- `fedex-pp-cli ship bulk --csv <path> --service <type> [--output <dir>] [--resume] [--concurrency N]` — Create labels for a CSV of orders with adaptive rate limiting and resumable on partial failure
- `fedex-pp-cli ship etd --invoice <pdf> --orig <country> --dest <country> ...` — Single-command Electronic Trade Documents shipping (upload + reference + ship in one call)
- `fedex-pp-cli address save <name> --street <s> --city <c> --state <s> --zip <z> --country <c>` — Save a recipient to the local address book
- `fedex-pp-cli address list` — List saved recipients in the local address book
- `fedex-pp-cli address delete <name>` — Delete a saved recipient from the local address book
- `fedex-pp-cli track diff [--since <duration>] [--tracking <num>...]` — Show only tracking events that have appeared since the last poll
- `fedex-pp-cli track watch --tracking <num> [--interval <duration>] [--webhook <url>]` — Long-poll daemon: continuously poll tracking and emit new events
- `fedex-pp-cli return create --tracking <orig-tracking> [--reason <reason>] [--email <addr>]` — Generate a return label (Ground Call Tag) for an existing shipment
- `fedex-pp-cli spend report [--since <duration>] [--by service|account|lane]` — Sum net charges per service type from the local shipment ledger
- `fedex-pp-cli archive <query>` — FTS5 search across shipments in the local archive
- `fedex-pp-cli export [--format csv|json] [--since <duration>] [--output <file>]` — Export shipment history for accounting reconciliation
- `fedex-pp-cli manifest [--date YYYY-MM-DD] [--close] [--output <file>]` — Generate end-of-day manifest report from local archive; optionally fire Ground EOD close
- `fedex-pp-cli doctor` — Verify auth, sandbox/prod routing, account-number format, label-format compatibility, BAG approval status


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
fedex-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Rate-shop and select cheapest service

```bash
fedex rate shop --from 90210 --to 10001 --weight 5lb --json --select rates.serviceType,rates.totalNetCharge,rates.transitTime
```

Returns every applicable service type with cost and transit time so the agent (or human) picks the right balance.

### Bulk ship with resume

```bash
fedex ship bulk --csv orders.csv --service GROUND --output labels/ --resume --concurrency 2
```

Creates labels for every row in orders.csv, respects the FedEx 1 req/s burst limit via cliutil.AdaptiveLimiter, and resumes from the last successful row if interrupted.

### Save a recipient and ship to them

```bash
fedex address save acme --street '500 Main St' --city Denver --state CO --zip 80202 --country US
```

Address book lookup avoids retyping recipient details across every shipment.

### Issue a return label

```bash
fedex return create --tracking 794633071234 --reason damaged --json
```

Generates a Ground Call Tag against the original tracking; PDF/ZPL label is emitted for the customer.

### Monthly spend report by service

```bash
fedex spend report --since 30d --by service --json --select service,total_net_charge,shipment_count
```

Joins shipments + rate quotes from the local ledger; answers 'where did I spend money on FedEx this month'.

## Auth Setup

FedEx uses OAuth2 client_credentials. Run `fedex auth login` once with your sandbox or production Client ID and Client Secret; the CLI mints a 1-hour bearer token, caches it on disk, and refreshes proactively before expiry. Production label printing requires per-project Bar Code Analysis Group (BAG) approval from FedEx — `fedex doctor` surfaces approval status before you hit it.

Run `fedex-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  fedex-pp-cli addresses --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
fedex-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
fedex-pp-cli feedback --stdin < notes.txt
fedex-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.fedex-pp-cli/feedback.jsonl`. They are never POSTed unless `FEDEX_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FEDEX_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
fedex-pp-cli profile save briefing --json
fedex-pp-cli --profile briefing addresses
fedex-pp-cli profile list --json
fedex-pp-cli profile show briefing
fedex-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `fedex-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/fedex/cmd/fedex-pp-cli@latest
   ```
3. Verify: `fedex-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/fedex/cmd/fedex-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add fedex-pp-mcp -- fedex-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which fedex-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   fedex-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `fedex-pp-cli <command> --help`.
