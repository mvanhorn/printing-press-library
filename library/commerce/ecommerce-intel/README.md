# Ecommerce Intel

Private Printing Press CLI for local-first ecommerce intelligence across Shopify-style stores. It follows the Traffic Intel pattern but expands the model for Shopify products, Klaviyo/email flows, GA4 ecommerce metrics, Google Search Console, Ahrefs, inventory risk, and GEO/answer-engine readiness.

## Binary

```bash
ecommerce-intel-pp-cli --agent agent-context
ecommerce-intel-pp-cli sync
ecommerce-intel-pp-cli dashboard
ecommerce-intel-pp-cli opportunities
ecommerce-intel-pp-cli geo-audit
```

MVP/default behavior is local-first: fixture and JSON import modes make no external API calls. `sources doctor` documents optional child CLI adapters without printing secrets.

## Sources and adapters

- Shopify: `shopify-pp-cli products/export --agent --shop <shop>`
- Klaviyo: `klaviyo-pp-cli reporting flows --agent --account <account>`
- GA4: `google-analytics-pp-cli ecommerce products --agent --property <property>`
- GSC: `google-search-console-pp-cli webmasters query-search-analytics <site> --agent --dimensions ["page"]`
- Ahrefs: `ahrefs-pp-cli site-explorer top-pages --agent --target <target>`
- Local JSON: `ecommerce-intel-pp-cli sync --import dataset.json`
- Fixture: `ecommerce-intel-pp-cli sync`

Environment variables are presence-checked only: `ECOMMERCE_INTEL_HOME`, `SHOPIFY_SHOP`, `SHOPIFY_ACCESS_TOKEN`, `KLAVIYO_API_KEY`, `KLAVIYO_ACCOUNT`, `GA4_PROPERTY_ID`, `GSC_SITE_URL`, `AHREFS_TARGET`, `AHREFS_PROJECT`.

## Commands

- `agent-context` — machine-readable context for agents
- `doctor` — local readiness checks
- `sources doctor` — source adapter status and planned commands
- `profile save/list/show/delete` — local profile metadata
- `sync` — fixture/import/planned source sync
- `dashboard` — KPI overview
- `opportunities` — prioritized revenue/SEO/email/inventory/GEO opportunities
- `action-plan` — 7-day action plan
- `money-pages` — rank pages by revenue
- `money-products` — rank products by revenue and margin
- `query-revenue` — revenue lookup by product/page/category query
- `explain-drop` — root-cause hints for revenue/search/session drops
- `product-actions` — PDP action recommendations
- `category-actions` — collection/category actions
- `email-actions` — Klaviyo/email actions
- `inventory-risk` — stockout/revenue protection queue
- `digest weekly` — weekly executive digest
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
