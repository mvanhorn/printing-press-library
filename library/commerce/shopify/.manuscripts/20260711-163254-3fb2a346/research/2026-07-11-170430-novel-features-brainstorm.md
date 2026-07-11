# Shopify Novel-Features Brainstorm (Reprint, subagent output)

## Customer model

**Priya — ops lead at a 6-person DTC agency, running Codex CLI every Monday morning across 3 client stores before her team's standup.**
*Today (without this CLI):* Priya has three Shopify admin tabs open, one per client store. She clicks into Orders, filters by "unfulfilled," then separately checks the Inventory tab for anything backordered, then repeats for the next store. When a client asks "what changed since Friday," she has no answer beyond memory — there's no diff, just the current state of the dashboard. For anything bigger she has ad hoc curl scripts against Admin GraphQL that she re-runs and greps.
*Weekly ritual:* Monday morning "state of the stores" pass across all three clients before the weekly check-in call.
*Frustration:* No fast, agent-consumable way to see "what changed since last time I looked" per store.

**Dev — support engineer wiring Claude Desktop's MCP config so frontline reps can ask "what's the status of order #1234" in plain English.**
*Today:* Reps open the Shopify admin order search, click into the order, flip to a separate fulfillment/tracking tab, then check a third tab for whether backordered line items are in stock elsewhere.
*Weekly ritual:* Fields ~30-50 "where's my order" tickets per week via Claude Desktop chat.
*Frustration:* The agent has to make three separate tool calls and stitch them together itself.

**Marcus — merchandising analyst who kicks off a full-catalog bulk-operations export every Friday afternoon before weekend price changes.**
*Today:* Manually calls `bulkOperationRunQuery`, then repeatedly re-invokes `currentBulkOperation` in a loop, then downloads/reshapes JSONL.
*Weekly ritual:* Friday-afternoon full-catalog export.
*Frustration:* Babysitting the async job by hand-polling in a loop.

**Alicia — platform engineer responsible for keeping the shopify-pp-cli MCP deployment alive during heavy agent sessions.**
*Today:* Finds out about GraphQL throttling only when a call mid-task fails with THROTTLED.
*Weekly ritual:* Pre-flight check before any batch job or heavy agent session.
*Frustration:* Zero visibility into remaining GraphQL cost budget until it's too late.

## Candidates (pre-cut)

1. orders brief <id> — (a) persona-driven, passes checks. Long Description: yes.
2. store diff --since <sync-run> — (a)/(c), passes checks. Long Description: yes.
3. bulk-operations wait — (a) persona-driven, passes checks. Long Description: yes.
4. doctor throttle — (a)/(e), passes checks. Long Description: yes.
5. checkouts trace <checkout-id> — (b)/(c), passes mechanical checks, flagged low-frequency.
6. Inventory-risk join — CUT, duplicate of `ops fulfillment-risk`.
7. Customer order timeline — CUT, duplicate of `report customer-lifecycle`.
8. New-vs-returning revenue split — CUT, duplicate of `report channel-mix`/`report revenue-daily`.
9. Draft orders / order-edit mutations — CUT, auth gap (read-only scopes only), out of modeled resources.
10. Metafields/metaobjects support — CUT, scope creep, not a modeled resource.
11. Bulk-operation status via generic `tail --resource` — CUT, not buildable (bulk ops aren't a synced resource type).
12. Rate-limit live monitor/dashboard — CUT, scope creep; descoped into `doctor throttle`.

## Survivors and kills

### Survivors
| # | Feature | Command | Score | Persona | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|---------|--------------|--------------|----------|-------------------|
| 1 | Order support brief | `orders brief <id>` | 10/10 | Dev | hand-code | Calls `orders get`, `fulfillment-orders list` (filtered), `inventory-items get` per line item, stitches into one agent-facing summary. | Brief Top Workflow #5 + User Vision (agent-native depth) | Use for a single support-ready order summary incl. fulfillment/inventory. Do NOT use for the raw order object; use 'orders get' instead. |
| 2 | Sync content diff | `store diff --since <sync-run>` | 6/10 | Priya | hand-code | Reads local SQLite filtered by `updated_at` against the previous sync watermark, pure local-data query. | Brief Data Layer (per-entity updatedAt watermark) | Use to see which rows changed since last sync. Do NOT use for sync freshness/row counts; use 'workflow status' instead. |
| 3 | Bulk operation blocking wait | `bulk-operations wait` | 9/10 | Marcus | hand-code | Repeatedly calls existing `currentBulkOperation` query until terminal state, prints result once. | Brief Top Workflow #3 | Use to block until the current bulk operation finishes. Do NOT use for a non-blocking snapshot; use 'bulk-operations current' instead. |
| 4 | GraphQL cost/throttle check | `doctor throttle` | 7/10 | Alicia | hand-code | Issues minimal GraphQL query, reads `extensions.cost.throttleStatus`. | Brief User Vision (agent-native depth top axis) | Use to check remaining GraphQL query-cost budget. Do NOT use for general auth/connectivity health; use 'doctor' instead. |

### Killed candidates
| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| Checkouts trace | Fails weekly-use test (occasional, not recurring). | orders brief |
| Inventory-risk join | Duplicate of `ops fulfillment-risk`. | orders brief |
| Customer order timeline | Duplicate of `report customer-lifecycle`. | store diff |
| New-vs-returning revenue split | Duplicate of `report channel-mix`/`report revenue-daily`. | store diff |
| Draft orders / order-edit mutations | Auth gap, out of modeled resources. | orders brief |
| Metafields/metaobjects support | Scope creep, not modeled. | store diff |
| Bulk-op status via generic tail --resource | Not buildable. | bulk-operations wait |
| Rate-limit live monitor/dashboard | Scope creep, descoped into doctor throttle. | doctor throttle |

## Reprint verdicts

- Prior research.json novel_features/novel_features_built are empty (stale) — reconciled against absorb manifest rows 1-13 and the ~35 already-shipped post-publish commands instead.
- Rows 1-13 (endpoint mirrors, bulk-ops, local store, MCP server): **Keep**, all still core/foundational.
- report / shopifyql / bulk-operations / store / ops / merchandising themes: **Keep**.
- growth theme (campaign-brief, vip-segments): Keep with frequency caveat (more monthly than weekly).
- orders tag / customers tag (write mutations): Keep with caveat — confirm write_orders/write_customers scopes are actually granted; the brief's auth_narrative only confirms read scopes.
- **OAuth client_credentials auth-audit command: flagged as a likely mismatch.** Shopify custom-app Admin API auth is a static `X-Shopify-Access-Token` header, not an OAuth client_credentials grant. This command may have been copied from a different API template and should be reviewed/fixed to match Shopify's real auth model.
