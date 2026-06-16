# Ecommerce Intel

Private Printing Press CLI for local-first ecommerce intelligence across Shopify-style stores. It follows the Traffic Intel pattern but expands the model for Shopify products, Klaviyo/email flows, GA4 ecommerce metrics, Google Search Console, Ahrefs, inventory risk, and GEO/answer-engine readiness.

## Binary

```bash
ecommerce-intel-pp-cli --agent agent-context
ecommerce-intel-pp-cli sync
ecommerce-intel-pp-cli movers
ecommerce-intel-pp-cli confidence
ecommerce-intel-pp-cli dashboard
ecommerce-intel-pp-cli opportunities
ecommerce-intel-pp-cli geo-audit
```

Default behavior is local-first: fixture and JSON import modes make no external API calls. Live source sync is explicit through `sync --source <source>`, `sync --source all`, `--live`, or `--real`; credentials stay in the child CLIs and `sources doctor` reports only binary/env presence without printing secrets.

## Sources and adapters

- Shopify: `shopify-pp-cli products list --agent --all --data-source live`
- Klaviyo: `klaviyo-pp-cli report flow-comparison --agent --data-source live`
- GA4: `google-analytics-pp-cli top-pages --agent --property <property> --start <start> --end <end>`
- GSC: `google-search-console-pp-cli webmasters query-search-analytics <site> --agent --dimensions ["page"] --start-date <start> --end-date <end>`
- Ahrefs: `ahrefs-pp-cli site-explorer top-pages --agent --target <target> --date <date>`
- Local JSON: `ecommerce-intel-pp-cli sync --import dataset.json`
- Fixture: `ecommerce-intel-pp-cli sync`

`sync --source all` requires all five source configurations before it runs. A single source, such as `sync --source gsc --site sc-domain:example.com`, is allowed and merges into the existing local dataset while preserving unmatched pages/products as orphan evidence instead of deleting the rest of the store model.

Child CLI output must include a supported `schema_version` before it can feed confidence-gated snapshots and movers. Unknown or missing child schemas fail closed with an actionable error.

Every sync still updates `~/.ecommerce-intel-pp-cli/<profile>-data.json`, and also appends a dated snapshot under `snapshots/<profile>/`. Snapshots include schema version, source command versions, date range, and input hashes. Retention keeps daily snapshots for 30 days and weekly snapshots after that. The CLI also keeps `learnings/<profile>.md` for mover and outcome notes.

Environment variables are presence-checked only: `ECOMMERCE_INTEL_HOME`, `SHOPIFY_SHOP`, `SHOPIFY_ACCESS_TOKEN`, `KLAVIYO_API_KEY`, `KLAVIYO_ACCOUNT`, `GA4_PROPERTY_ID`, `GSC_SITE_URL`, `AHREFS_TARGET`, `AHREFS_PROJECT`.

## Commands

- `agent-context` — machine-readable context for agents
- `doctor` — local readiness checks
- `sources doctor` — source adapter status and planned commands
- `profile save/list/show/delete` — local profile metadata
- `sync` — fixture/import or opt-in child CLI live sync
- `movers` — snapshot diff for commerce climbers, droppers, new Strike Zone entrants, and new revenue-at-risk products/pages/categories
- `confidence` — High/Medium/Low/Broken trust score with source coverage, freshness, tracking, and schema checks
- `dashboard` — KPI overview
- `opportunities` — prioritized revenue/SEO/email/inventory/GEO opportunities tiered as Fix-first, Quick-win, Strategic, or Refinement with dependencies
- `action-plan` — 7-day action plan with profile/date-range/source-coverage/confidence/mode status header and tiered dependencies
- `money-pages` — rank pages by revenue
- `money-products` — rank products by revenue and margin
- `query-revenue` — revenue lookup by product/page/category query
- `explain-drop` — root-cause hints for revenue/search/session drops
- `product-actions` — PDP action recommendations
- `category-actions` — collection/category actions
- `email-actions` — Klaviyo/email actions
- `inventory-risk` — stockout/revenue protection queue
- `source-coverage` — product/page/category/email evidence coverage across Shopify, Klaviyo, GA4, GSC, and Ahrefs
- `merchandising-link-plan` — collection-to-PDP and PDP-to-PDP internal link recommendations
- `experiment-plan` — PDP offer/content/schema/measurement experiments for one product
- `forecast-impact` — confidence-gated estimated upside from conversion, CTR, and inventory gaps
- `restock-winners` — high-margin, high-velocity products to protect before stockout or decay
- `cannibalization` — substitute/duplicate products competing for the same query or category
- `category-clusters` — collection-level revenue, sessions, clicks, backlinks, and decay rollups
- `digest weekly` — mover-led weekly executive digest with profile/date-range/source-coverage/confidence/mode status header

`dashboard`, `action-plan`, and `digest weekly` include a status header with profile, date range used, source coverage, confidence, and mode. Reports start from a 30-day window, widen to 90 days or 12 months when the local sample is thin, and announce the range used.
- `geo-audit` — llms.txt, structured data, product facts, buying guides, ChatGPT, Perplexity, and Google AI Overviews readiness

## GEO / answer-engine readiness

`geo-audit` checks:

- `llms.txt` availability
- Product/Offer, ItemList, Breadcrumb, and page structured data coverage
- concise product facts/specifications
- FAQ/answer blocks and buying-guide support
- collection guide gaps for ChatGPT, Perplexity, and Google AI Overviews answerability

## Development

```bash
gofmt -w ./cmd ./internal
go test ./...
go build ./cmd/ecommerce-intel-pp-cli
go run ./cmd/ecommerce-intel-pp-cli --agent agent-context
go run ./cmd/ecommerce-intel-pp-cli --agent sources doctor
```
