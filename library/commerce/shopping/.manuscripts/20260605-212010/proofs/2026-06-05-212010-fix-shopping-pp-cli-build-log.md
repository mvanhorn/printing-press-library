# Shopping CLI Build Log

Manifest transcendence rows: 7 planned, 7 built. All shipping-scope (no stubs).

## Built
- Foundation: hand-authored `price_points` + `watches` tables (internal/store/shopping_extra.go); shared store helpers (internal/cli/shop_store.go).
- index: real API (GET /retailers, paged GET /retailers/{id}/products|/shopping/products), injects retailers_id/parent_id, optional /price-history capture; dogfood-curtailed.
- compare, deals, arbitrage, leaderboard: SQL over the products table (json_extract, NULL-safe scans, allowlisted ORDER BY/identifier paths).
- price-drops, watch add/status: SQL over price_points/watches (joined to products.id).

## Generator-emitted (absorbed)
All 9 endpoints as typed commands (retailers/products/shopping/categories/amazon/status), plus framework sync/search/sql/analytics/export/import.

## Review fixes applied (Phase 4.95)
- README config path lemmebuyit-pp-cli -> shopping-pp-cli.
- README/SKILL value-prop: analytics -> price-drops/leaderboard (correct attribution).
- Dropped false mcp:read-only on index + watch add (they write the local store).
- price_points keyed by products.id (was unique_merchant_sku) so watch-status/price-drops joins match.
- deals --sort allowlist validation.
