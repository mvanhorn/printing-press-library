# amazon-orders Absorb Manifest

## Tool landscape

The community has built ~10 distinct tools for buyer-side Amazon order history. I focused on the five with the broadest feature coverage:

| Tool | Type | Lang | Stars | Notes |
|---|---|---|---|---|
| **alexdlaird/amazon-orders** | CLI + library | Python | (active) | OTP/2FA support, `--full-details`, by-year filtering. Closest direct competitor as a CLI. |
| **marcusquinn/amazon-order-history-csv-download-mcp** | MCP server | TypeScript/Playwright | (active) | 5 export tools + 6 query tools. **Most feature-complete competitor.** |
| **philipmulcahy/azad** | Chrome extension | TS | (popular) | Long-running incumbent. Some features paywalled (shipment/tracking). |
| **MaX-Lo/Amazon-Order-History** | Scraper + dashboard | Python | (smaller) | Has a Flask dashboard for visualization. |
| **drewdaemon/amazon_order_history_scraper** | CSV exporter | Python | (smaller) | Minimal, CSV-focused. |

## Absorbed (match-or-beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-----------|-------------------|-------------|--------|
| 1 | List orders | alexdlaird `history`, marcusquinn `get_amazon_orders` | `orders list` w/ time + status filters | Filterable in SQL, agent-native `--json --select` | shipping |
| 2 | Filter by year | alexdlaird `--year`, marcusquinn `year` param | `orders list --year 2025` | Backed by indexed local data; filter is SQL not refetch | shipping |
| 3 | Last 30 days / 3 months | alexdlaird `--last-30-days`/`--last-3-months` | `orders list --window 30d|3m|6m|ytd` | Open-ended window syntax via Go time.Duration | shipping |
| 4 | Order details (items, qty, prices) | alexdlaird `--full-details`, marcusquinn `get_amazon_order_details` | `orders get <id>` (or `order <id>`) | Cached after first fetch; offline thereafter | shipping |
| 5 | Items / ASIN export | marcusquinn `export_amazon_items_csv` | `items list`, `export items` | FTS5 full-text on title; ASIN reverse-index | shipping |
| 6 | Shipment / tracking export | marcusquinn `export_amazon_shipments_csv`, azad (paywalled) | `shipments list`, `track <id>` | Live ETA + carrier; offline historical from store | shipping |
| 7 | Carrier tracking number extraction | marcusquinn `fetch_tracking_numbers` | `track <id> --carrier` | Same data, but written to store so downstream queries can reason about carriers | shipping |
| 8 | Transactions / payments page | marcusquinn `get_amazon_transactions`, `export_amazon_transactions_csv` | `transactions list`, `export transactions` | Reconciled to orders via order ID join | shipping |
| 9 | Gift card balance | marcusquinn `get_amazon_gift_card_balance` | `gift-cards balance` | Integrated, not a separate tool | shipping |
| 10 | Gift card transactions | marcusquinn `export_amazon_gift_cards_csv`, `get_amazon_gift_card_transactions` | `gift-cards activity`, `export gift-cards` | Joined with order IDs and current balance | shipping |
| 11 | Auth status check | marcusquinn `check_amazon_auth_status` | `doctor`, `auth status` | Standard pp-cli idiom; runs before every sync | shipping |
| 12 | Browser session reuse | marcusquinn (Playwright), azad (extension) | `auth login --chrome` (cookie import) | No resident browser; cookies imported once; refreshed automatically | shipping |
| 13 | OTP / 2FA support | alexdlaird `AMAZON_OTP_SECRET_KEY` | Not needed — we use Chrome's session, the user's browser already cleared 2FA | Simpler UX; one-step login | shipping |
| 14 | CSV export | marcusquinn (5 export types), drewdaemon, azad | `export <surface> --csv` | Unified flag pattern; columns documented | shipping |
| 15 | JSON output | (none — competitors are CSV/UI only) | every command supports `--json --select` | Agent-native first-class | shipping |
| 16 | Date filters | alexdlaird (year + time-window), MaX-Lo | `--from`, `--to`, `--window` | Composable filters, plus SQL access for arbitrary windows | shipping |
| 17 | Batch order detail fetch | (none — incumbents serialize 1-at-a-time) | `sync --full-details` w/ AdaptiveLimiter | Honest rate-limit error class, not silent empty | shipping |
| 18 | Multi-region | marcusquinn (16 regions) | `--region us|uk|de|jp|...` (TLD switch) | v1 ships US only; multi-region is a follow-up | (stub — v2) |
| 19 | Dashboard / visualization | MaX-Lo Flask dashboard | Out of scope (CLI is the dashboard via SQL queries) | We provide raw SQL access, not a Flask app | not built |
| 20 | Print invoice (HTML) | (none) | `invoice <id> --html\|--md` | Markdown rendering for AI agent context | shipping |

## Transcendence (only possible because we have a local SQLite store)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|------------------------|-------|
| 1 | Where is my stuff (radar) | `where-is-my-stuff` | Requires JOIN of all open orders × current shipment status × ETA, plus a recency filter on tracking. Existing tools serve one order at a time. | 9 |
| 2 | Delivery slip detector | `delivery-slips --days 3` | Compare initial promised ETA to actual delivery date across history. Needs **historical snapshots** of ETA, which only exist if we sync periodically. | 8 |
| 3 | Spend rollup | `spend --by month\|year\|category\|seller\|payment` | SQL aggregate over orders + items + transactions. Incumbent CSVs are static; no rollup. | 9 |
| 4 | Top-items rollup | `top-items --by count\|total-spend` | Group-by ASIN/title across all years. Reveals what you actually buy. | 8 |
| 5 | Subscribe & Save detector | `subscribe-and-save` | Heuristic: same ASIN ordered on a regular cadence (every N±k days). Detected from history, not the official S&S page. | 7 |
| 6 | Find that thing | `find <query>` | FTS5 across order/item/seller text. "When did I order that USB-C cable" is a one-liner. | 9 |
| 7 | Arriving soon radar | `arriving-soon --days 7` | Cross-order ETA window query. | 7 |
| 8 | Late shipments alert | `late` | Active shipments past their original ETA. Carrier delays surface immediately. | 7 |
| 9 | Returns ledger | `returns` | Cross-reference orders → transactions for negative entries → mark items returned. Nobody does this end-to-end. | 6 |
| 10 | Carrier reliability scorecard | `carriers --rank` | For each carrier (UPS, USPS, Amazon Logistics, FedEx), compute on-time % and average slip. Only possible after months of synced history. | 6 |

Minimum 5 transcendence features (we have 10), all scored ≥ 6.

## Stubbed / out-of-scope

- Multi-region TLD support — v1 ships US only; structure is in place, lit up in v2.
- Cancel / return-request / contact-seller — state-changing endpoints with CSRF tokens, out of scope for a reader CLI.
- Cart / checkout — different surface entirely.
- Reviews / wish lists — adjacent surfaces, not order history.
- Subscribe & Save management UI (cancel, change cadence) — read-only detection only in v1.
- Transactions "Show More" Ajax pagination — first page only in v1.

## Ranking summary

- **Absorbed:** 17 features (build all of them as shipping; 1 stub for multi-region; 2 explicit out-of-scope).
- **Transcendence:** 10 features (build all of them — they are the differentiator).
- **Total shipping:** 27 features.

That is **~3× the surface of marcusquinn's MCP** (the densest competitor) and adds local-store transcendence that nobody else even attempts.
