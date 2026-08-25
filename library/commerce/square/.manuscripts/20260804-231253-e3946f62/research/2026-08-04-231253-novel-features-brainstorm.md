## Customer model

### Multi-location operations lead

**Today (without this CLI):** They export sales reports, inspect payments and refunds separately, then compare payout totals by hand. When deposits do not match gross sales, they carry dates and location IDs between several Square screens or API calls.

**Weekly ritual:** Every morning they review the prior business day; once a week they compare locations, investigate exceptions, and confirm that refunds, disputes, fees, and deposits reconcile.

**Frustration:** Square's records are individually available, but no ordinary endpoint explains why orders, collected payments, refunds, disputes, and payouts differ across settlement timing and locations.

### Retail and ecommerce operations engineer

**Today (without this CLI):** They retrieve catalog objects and location-specific inventory counts, then build temporary spreadsheets or scripts to identify stale prices, unavailable variations, and stock drift.

**Weekly ritual:** Before stores open or a menu is published, they sync catalog and inventory state, investigate mismatches, and compare the latest state with an earlier known-good snapshot.

**Frustration:** Catalog identity, variation identity, availability, and inventory are spread across connected resources, while the questions they need answered depend on both location and history.

### Square integration developer

**Today (without this CLI):** They move between API reference pages, SDK code, webhook configuration, sandbox requests, and application logs to check versions, idempotency, retries, and event delivery.

**Weekly ritual:** Before each release they inspect request payloads, confirm the pinned Square API version, audit webhook subscriptions, and test failure handling without accidentally making a production mutation.

**Frustration:** The remote API and SDK expose the necessary primitives, but release readiness requires combining schema facts, local configuration, subscription state, and event history.

### Service-business manager

**Today (without this CLI):** They inspect bookings, invoices, customers, payments, staff, and locations separately, often copying customer or booking identifiers between reports.

**Weekly ritual:** They review the previous week's appointments, no-shows, unpaid work, staff utilization, and repeat-customer activity.

**Frustration:** The business question is cross-resource—whether booked work happened, was invoiced, was paid, and used staff capacity—but the source records are organized by service.

## Candidates (pre-cut)

1. **Daily close reconciliation**
   - Command: `square-pp-cli reconcile close --since 24h`
   - Description: Reconciles orders, payments, refunds, disputes, and payouts by location and reports mechanically explainable differences.
   - Persona served: Multi-location operations lead
   - Source: **(c) cross-entity local query**
   - Long Description: none
   - Checks: `// pp:data-source local`; rejects `--live`; uses synced SQLite snapshots with stale/unsynced hints, drains query rows before subsequent joins, and uses no nested writers. No LLM, external service, new auth, fake API response, or persistent process is required.

2. **Inventory drift**
   - Command: `square-pp-cli inventory drift --since 7d`
   - Description: Compares catalog and inventory snapshots to show changed prices, availability, and location counts.
   - Persona served: Retail and ecommerce operations engineer
   - Source: **(c) cross-entity local query**
   - Long Description: none
   - Checks: `// pp:data-source local`; rejects `--live`; requires at least two retained snapshots and emits stale/unsynced hints. SQLite rows are drained before follow-up catalog joins; no nested writers, LLM, external service, or additional auth is needed.

3. **Customer timeline**
   - Command: `square-pp-cli customer timeline CUSTOMER_ID`
   - Description: Produces a chronological local view of a customer's orders, payments, refunds, loyalty activity, invoices, and bookings.
   - Persona served: Service-business manager and customer-service operator
   - Source: **(c) cross-entity local query**
   - Long Description: none
   - Checks: `// pp:data-source local`; rejects `--live`; uses only synced relationships, warns when relevant resource families are unsynced or stale, drains rows before joining, and avoids nested writers. It is mechanical chronology, not LLM summarization.

4. **Webhook health**
   - Command: `square-pp-cli webhook health --since 24h`
   - Description: Measures duplicate event IDs, out-of-order arrivals, delivery lag, event gaps, and subscription-state changes.
   - Persona served: Square integration developer
   - Source: **(b) Square-specific workflow**
   - Long Description: none
   - Checks: `// pp:data-source local`; rejects `--live`; computes metrics from synced event and webhook-subscription history with stale/unsynced hints. It drains event rows before subscription queries and creates no background listener or nested writer.

5. **Weekly service review**
   - Command: `square-pp-cli service review --since 7d`
   - Description: Correlates bookings, invoices, payments, customers, team members, locations, and shifts into completed, unpaid, no-show, and utilization totals.
   - Persona served: Service-business manager
   - Source: **(a) persona-driven**
   - Long Description: none
   - Checks: `// pp:data-source local`; rejects `--live`; uses deterministic status and timestamp joins, reports missing/stale resource families, drains rows before follow-up queries, and avoids nested writers. No NLP or third-party scheduling system is involved.

6. **Request readiness check**
   - Command: `square-pp-cli request check --method POST --path /v2/payments --body request.json`
   - Description: Validates a planned request against the bundled schema and checks environment, API version, mutation policy, and idempotency requirements without sending it.
   - Persona served: Square integration developer
   - Source: **(f) DeepWiki**
   - Long Description: none
   - Checks: `// pp:data-source computed`; performs offline schema and policy validation and never pretends to return an API response. It needs no token, external service, write scope, or LLM and is directly testable with fixtures.

7. **Payout explainer** — `square-pp-cli payout explain PAYOUT_ID`; local payout/payment activity is buildable, but intent substantially overlaps daily close reconciliation.
8. **Catalog publish preflight** — `square-pp-cli catalog preflight --location LOCATION_ID`; deterministic in principle, but several proposed checks are merchant-policy judgments not established by Square's contract.
9. **Location anomaly report** — `square-pp-cli location anomalies --since 7d`; needs thresholds not grounded in research and risks opaque pseudo-analytics.
10. **Webhook replay simulator** — `square-pp-cli webhook replay EVENT_ID --to URL`; fails external-service and mutation-safety checks because it sends data to arbitrary endpoints.
11. **Retry and idempotency audit** — `square-pp-cli integration audit --log app.log`; lacks a stable application-log input contract and cannot be verified reliably.
12. **Loyalty retention cohorts** — `square-pp-cli loyalty cohorts --since 90d`; feasible, but weekly pain and cohort semantics are not strongly enough supported to outrank operational workflows.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | Persona served | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|---|
| 1 | Daily close reconciliation | `square-pp-cli reconcile close --since 24h` | 10/10 | hand-code | Multi-location operations lead | Uses synced orders, payments, refunds, disputes, payout entries, and locations in local SQLite to compute gross-to-net differences by date and location with no external dependencies. | The brief names reconciliation as the top workflow and says no single endpoint explains sales-to-deposit differences. Closest killed sibling: payout explainer. | none |
| 2 | Inventory drift | `square-pp-cli inventory drift --since 7d` | 9/10 | hand-code | Retail and ecommerce operations engineer | Uses retained catalog-object and inventory-count snapshots in local SQLite to compute price, availability, and per-location quantity changes with no external dependencies. | The brief identifies location-dependent historical drift as a pain point and defines retained snapshots. Closest killed sibling: catalog publish preflight. | none |
| 3 | Customer timeline | `square-pp-cli customer timeline CUSTOMER_ID` | 8/10 | hand-code | Service-business manager and customer-service operator | Uses synced customers, orders, payments, refunds, loyalty events, invoices, and bookings in local SQLite to compute one ordered customer history with no external dependencies. | The brief identifies carrying IDs across services as user pain. Closest killed sibling: loyalty retention cohorts. | none |
| 4 | Webhook health | `square-pp-cli webhook health --since 24h` | 8/10 | hand-code | Square integration developer | Uses synced Square event records and webhook-subscription snapshots in local SQLite to compute duplicate IDs, ordering inversions, lag, gaps, and configuration drift with no external dependencies. | Square webhook research states events can duplicate, arrive out of order, and retry for up to 24 hours. Closest killed sibling: webhook replay simulator. | none |
| 5 | Weekly service review | `square-pp-cli service review --since 7d` | 8/10 | hand-code | Service-business manager | Uses synced bookings, invoices, payments, customers, team members, locations, and labor shifts in local SQLite to compute completed, unpaid, no-show, and utilization totals with no external dependencies. | The brief names this weekly ritual and the official resource model supplies each input. Closest killed sibling: loyalty retention cohorts. | none |
| 6 | Request readiness check | `square-pp-cli request check --method POST --path /v2/payments --body request.json` | 7/10 | hand-code | Square integration developer | Uses the bundled OpenAPI schema, command configuration, mutation policy, environment selection, Square-Version rules, and idempotency metadata to compute offline validation findings with no external dependencies. | The brief names API-version, idempotency, dry-run, and production-readiness checks. Closest killed sibling: retry and idempotency audit. | none |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| Payout explainer | Narrow overlap with the broader reconciliation workflow. | Daily close reconciliation |
| Catalog publish preflight | Proposed readiness rules contain merchant-policy judgments not established by Square's specification. | Inventory drift |
| Location anomaly report | Requires unsupported baselines or thresholds and risks opaque results. | Daily close reconciliation |
| Webhook replay simulator | Adds arbitrary outbound mutation, authorization, and secret-handling risk. | Webhook health |
| Retry and idempotency audit | No stable application-log schema exists, so correct parsing is unverifiable. | Request readiness check |
| Loyalty retention cohorts | Weaker evidence and less urgent weekly value than customer and service operations. | Customer timeline |
