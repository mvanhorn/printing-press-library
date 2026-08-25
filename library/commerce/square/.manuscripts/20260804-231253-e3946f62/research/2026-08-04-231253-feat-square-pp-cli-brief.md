# Square CLI Brief

## API Identity
- Domain: Square's REST platform for payments, commerce, customers, staff, appointments, merchant operations, and developer integrations.
- Users:
  - A multi-location Square operator who reviews yesterday's sales, refunds, fees, payouts, and location anomalies every morning before opening.
  - An ecommerce or retail operations engineer who repeatedly syncs catalog items and inventory, then traces mismatches across orders, payments, customers, and locations.
  - A Square integration developer who tests sandbox workflows, debugs webhook delivery, checks idempotency and API-version behavior, and validates production-readiness before releases.
  - A service-business manager who reviews bookings, invoices, customer histories, no-shows, and staff/location utilization each week.
- Data profile: 334 REST operations across 255 paths in the current OpenAPI 3 contract, with cursor pagination, OAuth 2 bearer tokens, date-versioned requests, idempotency keys on important mutations, and webhook events that can be duplicated or delivered out of order.

## Reachability Risk
- Low. Square publishes the API, current SDKs, API reference, sandbox, and an official MCP server. Searches of the official MCP and Node SDK issue trackers found no cluster of current 403, blocking, or rate-limit failures.
- Authentication: `Authorization: Bearer <token>`; canonical CLI environment variable will be `SQUARE_ACCESS_TOKEN`.
- Environments: production `https://connect.squareup.com`; sandbox `https://connect.squareupsandbox.com`.
- API version: the fetched specification defaults the `Square-Version` header to `2026-07-15`.
- Spec provenance: upstream commit `f9ff8da9443ade43f823f9100f390b9797c1dcd6` (Square release 2026-07-15); fetched file SHA-256 `101fca4d5346611a487542085c91bc66eb953e10224e442b275becbba5c10d6b`.
- No Square token was detected or approved for this run, so live authenticated smoke testing will be skipped unless the user adds a sandbox token later.

## Top Workflows
1. Daily close and payout reconciliation: sync orders, payments, refunds, disputes, and payouts; explain why gross sales, net receipts, and deposits differ by date and location.
2. Catalog and inventory operations: snapshot items, variations, modifiers, prices, and location counts; find drift, low stock, stale items, and mismatched availability before opening or publishing a menu.
3. Customer and order service: search a customer once, then trace their orders, payments, refunds, cards, loyalty activity, invoices, and bookings without copying IDs between tools.
4. Integration release checks: exercise sandbox-safe reads and dry-run mutations, inspect request requirements, verify API versions, audit webhook subscriptions, and detect retry/idempotency mistakes before deployment.
5. Weekly service-business review: correlate bookings, customers, staff, locations, invoices, and payments to find no-shows, unpaid work, utilization gaps, and repeat-customer patterns.

## Table Stakes
- Complete typed access to every operation in Square's current contract, not a hand-picked payments subset.
- Production/sandbox switching, bearer-token auth, `Square-Version` pinning, cursor pagination, structured errors, retry/backoff for 408/429/retryable 5xx, and generated idempotency keys for safe mutations.
- Human-readable tables plus JSON, agent-compact output, field selection, CSV, quiet/plain modes, and stable exit codes.
- Read-only safety mode, dry-run support, confirmation around destructive or money-moving operations, and no secret values in logs or command history.
- Webhook signature verification and webhook-subscription inspection.
- Local SQLite sync, full-text search, analytics, and repeatable scripts.

## Data Layer
- Primary entities: locations, merchants, catalog objects and variations, inventory counts and changes, customers, orders, payments, refunds, disputes, payouts, invoices, subscriptions, bookings, team members, labor shifts, loyalty accounts/events, gift cards, and webhook events.
- Relationships: payments reference customers, cards, orders, and locations; inventory joins catalog variations to locations; bookings join customers, team members, and locations; payouts reconcile payment/refund activity; invoices and subscriptions connect customers to orders and payments.
- Sync cursor: persist per-resource API cursor plus the newest observed `updated_at`/`created_at`; use full refresh where Square exposes no stable time filter.
- FTS/search: customer names/email/phone, catalog names/SKUs/descriptions, order references/notes, invoice numbers/titles, booking notes, and dispute reason/status.
- History: retain timestamped snapshots for balances, inventory, catalog prices, payment/refund status, payouts, bookings, and webhook-subscription state so drift and reconciliation commands have evidence.

## Codebase Intelligence
- Source: official `square/square-mcp-server` source plus DeepWiki analysis of `square/square-nodejs-sdk` (DeepWiki snapshot dated May 2025; current vendor spec and docs take precedence).
- Auth: OAuth 2 bearer token in `Authorization`; official SDK convention includes `SQUARE_TOKEN`, official examples use `SQUARE_ACCESS_TOKEN`, and the local MCP uses `ACCESS_TOKEN`. This CLI standardizes on `SQUARE_ACCESS_TOKEN` while documenting aliases if generated auth supports them.
- Data model: resource clients are modular but heavily connected; payments connect customers/cards/orders, inventory connects catalog/locations, and bookings connect customers/team/locations.
- Rate limiting: Square reports `429` / `RATE_LIMITED`; official guidance is exponential backoff with jitter. SDKs also retry timeouts and retryable server failures.
- Architecture: Square's official MCP compresses the full platform into `get_service_info`, `get_type_info`, and `make_api_request`; the SDKs expose dedicated typed resource clients with automatic pagination and standardized errors.

## User Pain Points
- The platform is broad enough that routine investigation requires finding IDs in one service and carrying them into several others.
- Money reconciliation spans orders, payments, refunds, disputes, fees, and payouts; no single ordinary endpoint explains the full difference between sales and deposits.
- Catalog/inventory state is location-dependent and changes over time, making drift and “what changed?” questions difficult without a local history.
- Webhooks can be retried for up to 24 hours and are not guaranteed to arrive in order, so integrations need deduplication and lag visibility.
- Official SDK major-version migration and generated surface changes create release-check work even when the underlying business workflow has not changed.

## Product Thesis
- Name: Square Press CLI (`square-pp-cli`)
- Thesis: Every current Square API operation, plus a local operational memory that turns disconnected commerce records into explainable daily-close, inventory, customer, and integration workflows.
- Why install it instead of the incumbent: Square's MCP and SDKs provide comprehensive remote calls; this CLI adds a normal scriptable command surface, offline history, cross-resource joins, cautious mutation controls, compact agent output, and repeatable operator workflows.

## Build Priorities
1. Generate the complete 334-operation typed endpoint surface from Square's current OpenAPI contract.
2. Make authentication, production/sandbox selection, API versioning, retries, pagination, errors, idempotency, and mutation safety trustworthy.
3. Sync high-gravity resources into SQLite and provide exact/FTS search plus field-selectable agent output.
4. Add daily reconciliation, inventory drift, customer timeline, webhook health, and service-operations compound commands selected by the adversarial feature gate.
5. Verify all non-auth behavior with mocks; reserve any payment/refund/write dogfood for an explicitly approved Square sandbox plan.

## Sources
- Square API reference: https://developer.squareup.com/reference/square
- Current machine-readable contract: https://raw.githubusercontent.com/square/connect-api-specification/master/api.json
- Authentication: https://developer.squareup.com/docs/auth
- Access tokens: https://developer.squareup.com/docs/build-basics/access-tokens
- Errors and rate limits: https://developer.squareup.com/docs/build-basics/general-considerations/handling-errors
- Webhooks: https://developer.squareup.com/docs/webhooks/overview
- Official MCP: https://developer.squareup.com/docs/mcp and https://github.com/square/square-mcp-server
- Official TypeScript SDK: https://github.com/square/square-nodejs-sdk
- Official Python SDK: https://pypi.org/project/squareup/
