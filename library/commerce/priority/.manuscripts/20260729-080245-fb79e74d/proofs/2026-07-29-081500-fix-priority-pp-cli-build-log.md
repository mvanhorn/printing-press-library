# Build Log — priority-pp-cli

Manifest transcendence rows: 7 planned, 0 built. Phase 3 will not pass until all 7 ship.

Absorbed hand-code scope: entity create/update, entity subform-add/subform-update/subform-delete, query, files attach/get, text get/set, forms list/describe/refresh, aging, debtors, stock alerts, batch load (journaled).

## Phase 3 outcome
Manifest transcendence rows: 7 planned, 7 built (dogfood novel_features_check: planned=7 found=7 missing=[]).

Built (hand-code): forms search/diff/licensed, batch load+resume (journaled $batch), reconcile, shortage, customer summary,
entity create/update + subform-add/update/delete (raw JSON write surface), query (raw OData), files attach/get, text get/set,
forms list/describe/refresh, aging, debtors, stock alerts. Plus internal/priorityx (EDMX parser, schema diff, HTML strip,
age buckets — all table-tested) and pp_* store tables via the extras.go hook.

Deviations from generation defaults (live-verified against the official sandbox):
- orderitems resource dropped from spec: flat ORDERITEMS is unstable on the reference tenant (intermittent 404/400);
  order lines come from orders + $expand=ORDERITEMS_SUBFORM instead.
- id_field added per list endpoint (ORDNAME/CUSTNAME/IV/PARTNAME/SUPNAME/WARHSNAME) — without it sync stored 0 rows.
- store.go LookupFieldValue patched to probe ALL-UPPERCASE field names (Priority style). RETRO CANDIDATE: generator
  template only probes snake/camel/Pascal; ERP OData APIs use uppercase.
- cliutil credentials_test patched for two-var Basic pair auth. RETRO CANDIDATE: generated test template assumes
  single-token auth; Basic pair specs get empty AuthHeader in tests as generated.
- Generated "batch" promoted command has no "run" child (single-endpoint collapse); raw batch = `batch --requests`,
  journaled = `batch load`, recovery = `batch resume`.
- Regen re-apply script for hand wiring + patches: $RUN/reapply-hand-wiring.py

Behavioral verification against live sandbox (apidemo): doctor OK; entity list/get OK; query OK; forms refresh cached
3,868 forms / 50,200 fields in ~2s; forms search/describe/diff/licensed OK; sync 1,200 records 0 errors; aging buckets
real totals; debtors ranked; customer summary joined; reconcile in-sync/drifted verdicts correct; shortage computed
482 embedded order lines; text get returns clean plain text; write commands dry-run clean.
