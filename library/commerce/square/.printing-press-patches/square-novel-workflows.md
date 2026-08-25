# Preserve Square novel workflows

## Reprint guard

Keep the six approved workflows fully implemented and wired:

- `reconcile close --since 24h`: local payment, refund, dispute, and payout reconciliation by location.
- `inventory drift --since 7d`: honest local changed-record analysis that reports when a historical baseline is unavailable.
- `customer timeline <customer-id>`: chronological local orders, payments, refunds, loyalty, invoices, and bookings.
- `webhook health --since 24h`: local duplicate, ordering, lag, event-type, and subscription analysis with de-duplication limitations stated.
- `service review --since 7d`: local booking outcomes grouped by staff and location.
- `request check --method ... --path ... [--body ...]`: computed-only request, environment, API-version, JSON, mutation, and idempotency validation; it must never send a request.

The five analytics commands must retain the exact `// pp:data-source local` annotation and reject `--data-source live`. Request check must retain `// pp:data-source computed` and `mcp:read-only=true`. Local SQL rows must be fully drained and closed before follow-up queries.

## Verification

Keep the generated help smoke tests plus `internal/cli/novel_square_behavior_test.go`. No novel source file may contain an unimplemented scaffold marker.

Also preserve the endpoint-safe command name `catalog get` for `GET /v2/catalog/info`; `catalog info` violates Printing Press's command-verb naming gate.

Keep the root `payment-request.json` fixture. It is deliberately fake, is read only by `request check`, and lets the documented offline example pass without credentials or a network request.

Preserve schema v12's bounded, precision-safe `resource_history` snapshots for catalog, inventory, and webhook-subscription resources. They are required for field-level inventory drift and subscription-change comparisons. Preserve tombstones, indexed timestamps, the 365-day/100-version bounds, fully convergent bounded open-time cleanup, and the pre-v12 scrub that removes legacy subscription secrets from resources, history, and FTS before history backfill. Preserve the append-only `webhook_deliveries` receipt log, its 90-day/100,000-row bound, fully convergent open/health cleanup, and `webhook ingest --body FILE [--received-at RFC3339] [--receipt-id ID]`; health metrics must read actual ingested receipts, never Square event-type metadata. Preserve operational sync's method-aware endpoints: Catalog Search POST with deleted objects, SearchOrders, inventory counts/changes, and SearchTeamMembers. Preserve reconciliation's order/payment/refund/dispute/payout joins, resource-specific timestamps, tip-inclusive payments, completed-only refunds, service-review name/payment/order enrichment and per-segment staff allocation, and request check's generated-operation lookup plus explicit `--approve-mutation` gate. Because runtime body-schema validation is unavailable, `safe_to_send` must remain false; `ready_for_manual_review` is the narrower positive result.
