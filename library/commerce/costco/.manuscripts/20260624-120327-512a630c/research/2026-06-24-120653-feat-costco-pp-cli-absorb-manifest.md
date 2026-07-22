# Costco Receipts CLI — Absorb Manifest

Source endpoint: `POST https://ecom-api.costco.com/ebusiness/order/v1/orders/graphql`
Auth: bearer idToken + `costco-x-wcs-clientId` (no cookies). Content-Type: `application/json-patch+json`.
This is a GraphQL-only sniffed API: framework is generated, all data commands are hand-built around a GraphQL client wrapper.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Fetch in-warehouse receipts by date range | TCRDD / Chrome ext `nnalnbom...` | costco-pp-cli receipts --since --until | Arbitrary range (past UI 2yr cap), --json/--csv/--select, exit codes |
| 2 | Fetch max history in one shot | TCRDD (rolling 3yr) | costco-pp-cli receipts --all | Single call returns full set; no 6-month UI chunking |
| 3 | JSON export of receipts | TCRDD / extension | (behavior in costco-pp-cli receipts) --json | Agent-native, --select dotted paths, --compact |
| 4 | CSV export for budgeting/sheets | costco-importer / CostcoWrapped | (behavior in costco-pp-cli receipts) --csv | Flat line-item CSV straight to a spreadsheet |
| 5 | Single receipt detail w/ line items | all tools | costco-pp-cli receipt get <barcode> | Full itemArray/tender/coupon/tax from the local store or live |
| 6 | Dedup receipts by barcode | TCRDD | (behavior in costco-pp-cli sync) | membershipNumber+transactionBarcode composite key in SQLite |
| 7 | Incremental "only new" merge | TCRDD smart merge | (behavior in costco-pp-cli sync) | Cursor = last transactionDate; idempotent re-sync |
| 8 | Gas-station receipts | receipts documentType | (behavior in costco-pp-cli receipts --type gas) | Fuel grade/qty/UOM fields surfaced |
| 9 | Online .com order history | getOnlineOrders query | costco-pp-cli orders --since --until | Paginated; shipment + tracking line items |
| 10 | Category counts (warehouse/gas/carwash) | receiptsWithCounts | costco-pp-cli counts --since --until | One-call summary of receipt counts by channel |
| 11 | Household / multi-member merge | TCRDD multi-member | (behavior in costco-pp-cli sync, grouped by membershipNumber) | Multiple tokens sync into one archive, grouped by member |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | History-depth probe | history-depth | hand-code | Steps startDate backward (binary/year-step), reports where earliest receipt stops moving — finds the true server-side floor the UI hides | Use this to discover how far back YOUR account's receipts actually go. Directly answers "can I get receipts older than 2 years." Do NOT use for routine fetch; use 'receipts' for a known range. |
| 2 | Local SQLite archive + offline search/sql | sync, search, sql | hand-code | A single GraphQL endpoint returns one date-range; only a local store enables cross-receipt FTS, SQL, and compounding history | none |
| 3 | Spend analytics rollups | spend --by month\|warehouse\|department | hand-code | Requires local aggregation across many receipts + line items; no API call returns this | none |
| 4 | Item price history | item-history <upc-or-query> | hand-code | Tracks one item's unit price across every receipt over time — only possible with the local line-item store | none |
| 5 | Instant-savings captured | savings --since --until | hand-code | Sums instantSavings + coupon amounts across receipts; the UI never totals this | none |
| 6 | Warranty/return-window watch | returns-window --days 90 | hand-code | Flags items still inside Costco's return window from local receipt dates; no API/UI equivalent | none |

Minimum-5 transcendence: satisfied (6). Flagship = #1 history-depth (the user's stated goal).

## Stubs
None. Every row ships fully. (Online `orders` enrichment with product links is deferred as a non-shipping nice-to-have, not a stub — it's simply out of scope for v1 and not listed as a command.)

## Notes for generation
- GraphQL-only: generate framework scaffolding; hand-build the `internal/costco` GraphQL client (custom headers + json-patch content-type) and all commands.
- Auth: bearer idToken via COSTCO_ID_TOKEN + COSTCO_CLIENT_ID; doctor decodes JWT exp.
- Read-only CLI: every command is `mcp:read-only`. No mutations exist on this surface.
- PII: redact membership numbers, masked PANs, addresses in any proof/manuscript.
