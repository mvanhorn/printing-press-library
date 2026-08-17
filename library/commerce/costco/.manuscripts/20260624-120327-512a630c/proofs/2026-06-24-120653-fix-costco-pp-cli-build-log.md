# Costco CLI Build Log

Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.
(history-depth, sync/search/sql, spend, item-history, savings, returns-window)

## Generation
- Framework generated from internal spec (single raw GraphQL passthrough resource).
- Static headers baked into client via required_headers: client-identifier, costco-x-wcs-clientId (static app id), costco.env, costco.service, Content-Type: application/json-patch+json, Origin, Referer.
- Generator scaffolded novel stubs for history-depth, item-history, returns-window, savings, spend (+ tests); promoted `raw`.

## Hand-build plan (Phase 3)
- internal/cli/costco_graphql.go: shared GraphQL helpers, receipt types, JWT exp decode, date helpers. (reuses framework client.PostQueryWithParams; no custom HTTP client)
- internal/cli/receipts.go: receipts (range fetch), receipt get <barcode>, counts.
- internal/cli/orders.go: orders (getOnlineOrders, paginated).
- internal/archive + sync.go + sql.go + search.go: SQLite archive layer.
- Fill novel stubs: history-depth, spend, item-history, savings, returns-window.
- doctor: decode JWT exp; auth set-token stores id_token.
