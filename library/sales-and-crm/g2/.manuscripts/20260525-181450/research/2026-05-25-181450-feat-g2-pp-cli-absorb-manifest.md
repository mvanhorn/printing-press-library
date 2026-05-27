# G2 CLI Absorb Manifest

## Sources Surveyed
- Anthropic G2 MCP server (hosted; 14 visible tools) — closed source, the strongest public signal of the OAuth surface
- G2 Postman workspace — OAuth setup + smoke tests
- G2 official docs (documentation.g2.com) — v2 API + syndication API + developer-portal
- G2 v2 OpenAPI spec (data.g2.com/openapi/v2.yaml) — narrow surface: buyer_intent, screenshots, categories, market_signals, credit_account, credit_deductions, product_features
- G2 syndication API (data.g2.com/api/2018-01-01/syndication/) — products, reviews, vendors, product_mappings, hashed_users
- 3rd-party scrapers (RapidAPI G2 Scraper, biegehydra/Advanced-G2-Scraper, deprecated Apify actors) — surveyed, NOT absorbed (ToS-violating, brittle)
- Adjacent platforms referenced (NOT absorbed): Trustpilot, Capterra (now G2-owned), TrustRadius, Software Advice, Gartner Peer Insights

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List products | Anthropic G2 MCP `list_products` | `g2-pp-cli products list` | Local mirror, `--json`, `--select`, `--csv`, paginated |
| 2 | Show product | Anthropic G2 MCP `show_product` | `g2-pp-cli products show <slug>` | Offline-first, `--json`, fresh-cache fallback |
| 3 | List my products | Anthropic G2 MCP `list_my_products` | `g2-pp-cli my-products list` | Offline-first, `--json` |
| 4 | List categories | Anthropic G2 MCP `list_categories` | (generated endpoint) categories list | Tree view, recursive |
| 5 | Show category | Anthropic G2 MCP `show_category` | (generated endpoint) categories show | With nested product list |
| 6 | List vendors | Anthropic G2 MCP `list_vendors` | (generated endpoint) vendors list | `--json`, `--select` |
| 7 | Show vendor | Anthropic G2 MCP `show_vendor` | (generated endpoint) vendors show | With all products |
| 8 | List standard product reviews | Anthropic G2 MCP `list_standard_product_reviews` + syndication API | `g2-pp-cli reviews list --product <slug>` | Local FTS, paginated, `--json`, `--since` |
| 9 | List market-intel reviews | Anthropic G2 MCP `list_market_intelligence_product_reviews` | `g2-pp-cli reviews market-intel --product <slug>` | Local cache, `--json` |
| 10 | Browse buyer intent | Anthropic G2 MCP `browse_buyer_intent` | (generated endpoint) buyer-intent list | `--since`, `--min-score`, `--json`, `--csv` |
| 11 | Browse product buyer intent | Anthropic G2 MCP `browse_product_buyer_intent` | (behavior in g2-pp-cli buyer-intent list) `--product <slug>` filter | Same flags as #10 |
| 12 | Buyer-intent dashboard view | Anthropic G2 MCP `show_buyer_intent_dashboard` | `g2-pp-cli buyer-intent dashboard` | Aggregated view, `--period`, `--json` |
| 13 | Competitive intel browse | Anthropic G2 MCP `browse_competitive_intelligence` | (generated endpoint) competitive-intel list | `--product`, `--json` |
| 14 | List screenshots | v2 OpenAPI `/screenshots` | (generated endpoint) screenshots list | -- |
| 15 | List market signals | v2 OpenAPI `/market_signals` | (generated endpoint) market-signals list | -- |
| 16 | Credit balance | v2 OpenAPI `/credit_account` | `g2-pp-cli credits balance` | Surfaces remaining credits, agent-native JSON |
| 17 | Credit deductions log | v2 OpenAPI `/credit_deductions` | `g2-pp-cli credits log --since` | Historical deduction list |
| 18 | List product features | v2 OpenAPI `/product_features` | (generated endpoint) product-features list | -- |
| 19 | List product mappings | Syndication API `/product_mappings` | (generated endpoint) product-mappings list | Syndication ID bootstrap |
| 20 | List hashed users | Syndication API `/hashed_users` | (generated endpoint) hashed-users list | -- |
| 21 | OAuth + AccountAPIToken auth | G2 dev portal + Postman | `g2-pp-cli auth login` | Stores both `client_id/secret` (OAuth) and `api_token` (syndication) |
| 22 | Health check / scope discovery | (none — gap) | `g2-pp-cli doctor` | Validates auth, lists granted scopes, prints credit balance, runs smoke calls |
| 23 | Local sync (delta) | (none — gap) | `g2-pp-cli sync --resources products,reviews,categories,vendors,buyer_intent,market_signals --since 24h` | Incremental SQLite mirror; nobody else does this |
| 24 | Local search | (none — gap) | `g2-pp-cli search "<query>" --type reviews` | FTS5 over reviews; nobody else does this |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This |
|---|---------|---------|-------|--------------|------------------------|
| 1 | Cross-product weekly diff | `g2-pp-cli watch products --product <s,s,s> --since 7d` | 10/10 | hand-code | Joins snapshots of `products` + new `reviews` rows in local SQLite to emit star delta, new-review count, and badge changes across N products |
| 2 | Credit-burn forecast | `g2-pp-cli credits forecast` | 10/10 | hand-code | Projects month-end spend from trailing 14-day local `credit_ledger`; refuses sync runs that would exceed remaining credits under `--budget-check` |
| 3 | Multi-product review FTS | `g2-pp-cli search --type reviews "rate limit" --product <s,s,s>` | 10/10 | spec-emits | Framework `search` over SQLite FTS5 on `reviews.title/body/pros/cons`; product list narrows row set |
| 4 | Alternatives-signal switching threats | `g2-pp-cli alt-track <my-product> --since 30d` | 10/10 | hand-code | Filters local `buyer_intent_events` where `subject_product_id = <my-product>` AND `signal_type=Alternatives`, ranks companies by visit count × employee size |
| 5 | Category rising-challenger detector | `g2-pp-cli analytics --type products --group-by category --metric review-velocity-30d` | 9/10 | hand-code | Joins `products` × `categories` × `reviews`; flags products in top-quartile growth that aren't top-quartile by absolute count |
| 6 | Buyer-intent triage CSV | `g2-pp-cli buyer-intent list --since 24h --min-score 50 --csv` | 10/10 | hand-code | Flattens nested firmographics (company, domain, employees, industry, country, page_type, signal_type) into a sales-ready CSV |
| 7 | Reviews × competitors top-cons | `g2-pp-cli analytics --type reviews --group-by product --filter "rating<=3" --product <s,s,s>` | 9/10 | hand-code | Local aggregation: per product, the lowest-rated reviews' `cons` verbatims as JSON for downstream `claude` summarization |
| 8 | Syndication-eligible review filter | `g2-pp-cli reviews list --product <slug> --syndication-eligible --since 7d` | 8/10 | hand-code | Filters synced reviews for syndication-eligible flag, scoped by product and `--since` |
| 9 | Market-signal weekly diff | `g2-pp-cli watch market-signals --category <cat> --since 7d` | 7/10 | hand-code | Diffs locally synced `market_signals` snapshots; emits intent-score and visits-count deltas per category |

**Hand-code count:** 8 (#1, #2, #4, #5, #6, #7, #8, #9). Spec-emits: 1 (#3). Each hand-code feature is ~50-150 LoC plus `root.go` wiring.

## Stubs

None. Every shipping-scope feature is fully implemented; no stub disclosure required.

## Scope summary

- Absorbed: 24 features matching or beating every existing G2 tool
- Transcendence: 9 features that no existing G2 tool offers
- Total shipping commands: 33 (including generator-emitted endpoint commands)
- Hand-code commitment: 8 transcendence commands

The combined feature set beats the Anthropic G2 MCP (14 chat-only tools, no local mirror, no FTS, no credit telemetry, no cron) and every public scraper (ToS-violating, brittle, no buyer-intent surface).
