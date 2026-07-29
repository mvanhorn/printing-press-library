# Absorb Manifest — priority-pp-cli

Sources absorbed: assafch/Priority-Mcp (72 MCP tools), HirezRa/n8n-nodes-priority (n8n community node), th1-ai/optima-pms-pp-cli (vertical Priority CLI), victor-rioba/priority-sdk (priority-tsdk, dead), MordiSacks/priority-api (PHP), MedatechUK.APY (PyPI EDI loader), aviranbenmoshe/priority-mcp (LobeHub listing, repo gone), Zapier Priority integration, official docs + Postman collection.

DeepWiki (Step 1.5a.6): skipped — the only candidate repos are 0-2 star projects not indexed by DeepWiki; API architecture fully covered by official raw doc captures.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List any entity with OData query options ($filter/$select/$orderby/$top/$skip/$expand/$since) | Priority-Mcp execute_odata_query; n8n-nodes-priority | (generated endpoint) entity list | Typed flags, --json/--select/--csv, offline-composable |
| 2 | Get single record by key (incl. composite keys) | Priority-Mcp get_customer et al.; priority-tsdk findOne | (generated endpoint) entity get | Composite key syntax, --expand |
| 3 | Create record on any form incl. deep insert (subform arrays) | n8n-nodes-priority; MedatechUK.APY toPri | priority-pp-cli entity create | --data/--stdin raw JSON, --dry-run, mandatory-field pre-check from cached metadata |
| 4 | Update record (PATCH, incl. subform line) | n8n-nodes-priority | priority-pp-cli entity update | Composite keys, --dry-run |
| 5 | Delete record | n8n-nodes-priority | (generated endpoint) entity delete | --dry-run, typed exit codes |
| 6 | Subform navigation read | priority-tsdk subform() | (generated endpoint) entity subform | Line addressing, nested expand |
| 7 | Subform line add/update/delete | n8n-nodes-priority | priority-pp-cli entity subform-add | Also subform-update / subform-delete siblings; line-number addressing |
| 8 | Raw OData escape hatch (any path + query) | Priority-Mcp execute_odata_query | priority-pp-cli query | Free-form path+query with auth/throttle handled |
| 9 | $batch with dependencies/atomicity groups | n8n-nodes-priority; optima-pms batch | (behavior in priority-pp-cli batch load) JSONL/stdin builder, auto-chunk at 100 ops, per-op journal; raw single request via generated batch --requests | Throttle-aware; journal enables resume (novel #4) |
| 10 | Changed-records $since feed | n8n-nodes-priority | (behavior in priority-pp-cli entity list) --since flag with UTC-Z normalization; persisted cursors via sync | BPM-entity awareness |
| 11 | Attachment upload (base64 data-URI + SUFFIX) | Priority-Mcp upload_attachment; n8n | priority-pp-cli files attach | Local file → data-URI automatic, MIME suffix |
| 12 | Attachment download | Priority-Mcp get_attachment_url | priority-pp-cli files get | Decodes data-URI to disk |
| 13 | Text subform read | n8n-nodes-priority | priority-pp-cli text get | HTML → plain text view |
| 14 | Text subform write (TEXT/APPEND/SIGNATURE) | n8n-nodes-priority | priority-pp-cli text set | --append; plain or HTML |
| 15 | List all entities/forms on tenant | Priority-Mcp list_entities | priority-pp-cli forms list | Offline from cached parsed $metadata |
| 16 | Per-entity schema (fields, keys, subforms, mandatory) | Priority-Mcp get_entity_schema | priority-pp-cli forms describe | Offline; v25.1 mandatory-flag annotations |
| 17 | Version probes (GetPriorityVersion/GetODataVersion) | n8n-nodes-priority | (generated endpoint) meta version | Also wired into doctor |
| 18 | Metadata refresh (ClearEntityMetadata) | official docs | priority-pp-cli forms refresh | Clear server cache + re-fetch + re-parse local cache |
| 19 | Built-in rate limiting (100/min) with 429 retry | aviranbenmoshe MCP; optima-pms --rate-limit | (behavior in priority-pp-cli doctor) client-wide adaptive limiter; typed RateLimitError; exit code 7 | Cross-cutting on every command |
| 20 | Customer search (name/phone/email fragments) | Priority-Mcp search_customers | (behavior in priority-pp-cli search) FTS5 over synced customers via search --type customers | Offline, instant, no rate-limit burn |
| 21 | Customer balance & AR aging buckets (0-30/31-60/61-90/90+) | Priority-Mcp get_customer_aging / get_aging_report | priority-pp-cli aging | Offline from synced AINVOICES; book-wide + per-customer |
| 22 | Open debts / top debtors | Priority-Mcp get_top_debtors / get_open_debts | priority-pp-cli debtors | Offline ranking, --limit |
| 23 | Unpaid invoices per customer | Priority-Mcp get_unpaid_invoices_for_customer | (behavior in priority-pp-cli customer summary) unpaid list per customer; ad-hoc via generated invoices list --filter | |
| 24 | Orders list / pending orders | Priority-Mcp get_pending_orders / list_orders | (behavior in priority-pp-cli orders list) --filter "ORDSTATUSDES eq '...'" | Typed resource + local sync |
| 25 | Order drill-down: lines + status log | priority-tsdk modifyRelated | (behavior in priority-pp-cli orders get) --expand ORDERITEMS_SUBFORM,ORDISTATUSLOG_SUBFORM | Nested expand options |
| 26 | Stock levels per warehouse | Priority-Mcp get_stock_levels | (behavior in priority-pp-cli entity list) WARHSBAL via the generic entity surface (form availability varies per tenant; absent on the official sandbox) | Warehouse list via (generated endpoint) warehouses list |
| 27 | Low stock alerts | Priority-Mcp get_low_stock_alerts | priority-pp-cli stock alerts | Offline join parts + warehouse balances vs minimums |
| 28 | Top customers / top products | Priority-Mcp get_top_customers / get_top_products | (behavior in priority-pp-cli analytics) analytics --type orders --group-by CUSTNAME | Offline aggregation |
| 29 | PAT auth handled transparently | optima-pms-pp-cli PRIORITY_PAT | (behavior in priority-pp-cli doctor) PRIORITY_API_USERNAME/PASSWORD Basic; PAT = username token + password "PAT" | Documented PAT convention |
| 30 | Doctor / connectivity + license health | optima-pms doctor | priority-pp-cli doctor | Diagnoses "API Cannot Be Run for This Form" licensing 400s, 429 fair-use state |
| 31 | Offline sync to SQLite | optima-pms (vertical-only) | priority-pp-cli sync | Generic: orders, customers, invoices, parts, porders, suppliers, warehouses on any tenant |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Schema grep | forms search <term> | 9/10 | hand-code | Queries the parsed-$metadata SQLite cache (forms, fields, subforms, mandatory flags) with FTS — no API call | Brief workflow #5 "schema archaeology"; >10MB EDMX per tenant; Priority-Mcp list_entities shows demand but has no search | Use this command to find forms and fields in the cached tenant schema. Do NOT use it to search business data (customers, parts, orders); use 'search' instead. |
| 2 | Schema snapshot diff | forms diff --baseline <name> | 8/10 | hand-code | Fetches $metadata live, stores a named snapshot in SQLite, diffs added/removed/changed forms and fields locally | optima-pms ships vertical-only fingerprint/diff; brief workflow #5 "what changed after an upgrade" | Use this command to compare tenant schema snapshots over time or across tenants. Do NOT use it to look up current field names; use 'forms search' instead. |
| 3 | License probe | forms licensed --forms <csv> | 8/10 | hand-code | Issues throttled GET {FORM}?$top=1 probes, classifies 200 vs "API Cannot Be Run for This Form" 400, caches per-tenant verdicts in SQLite | Real-world failure #2 in ecosystem research (Power BI thread); workflow #5 | none |
| 4 | Batch journal + resume | batch resume <journal-id> | 8/10 | hand-code | batch load journals per-op status (id, payload, response) to SQLite; resume re-submits only failed ops through the same throttled $batch endpoint | Official docs: $batch max 100 ops, no rollback; pain point #1 (everyone hand-rolls throttling) | Use this command to re-run failed operations from a journaled batch. Do NOT use it to submit a new batch; use 'batch load' instead. |
| 5 | Sync reconcile | reconcile --resource ORDERS | 8/10 | hand-code | Compares local row counts and max-modified per date window against live windowed $top/$select probes (API has no $count) and reports drift | Brief: "No $count"; workflow #4 nightly sync verification; $since limited to BPM entities | Use this command to verify the local database matches the live tenant. Do NOT use it to pull data; use 'sync' instead. |
| 6 | Shortage (ATP) | shortage --include-inbound | 8/10 | hand-code | Local SQLite join: open ORDERITEMS_SUBFORM demand minus part on-hand stock, optionally netting open PORDERS inbound — no API call | Brief workflows #2/#3; no competitor and no single OData call can produce this join | Use this command for open-order demand vs on-hand shortfall. Do NOT use it for static minimum-level breaches; use 'stock alerts' instead. |
| 7 | Customer 360 | customer summary <CUSTNAME> | 8/10 | hand-code | Single local join over synced CUSTOMERS + AINVOICES + ORDERS: balance, aging buckets, open orders, unpaid invoices, last activity | Brief workflow #1; assafch MCP dedicates 7 of 72 tools to AR but needs four live calls for this picture | Use this command for one customer's full picture before a call. Do NOT use it for ranked debtor lists across customers; use 'debtors' instead. Do NOT use it for book-wide bucket totals; use 'aging' instead. |

Hand-code count: 7 of 7 transcendence rows are hand-code; 0 spec-emits.

Killed candidates and full audit trail: see 2026-07-29-081500-novel-features-brainstorm.md.
