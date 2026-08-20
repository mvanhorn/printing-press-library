# Phase 4.95 Local Code Review — Findings & Fixes

Reviewer: superpowers:code-reviewer on the hand-authored files
(internal/cookunity/client.go, internal/cli/{sync,promoted_meals,cookunity_meals,
cookunity_sql,plan,value,drift,favorites,compare,chefs,channel_workflow}.go,
internal/store/cookunity_migrations.go). All findings autofixed in-place; build + vet green;
fixes re-verified against the live API.

## Autofixed (5)
1. **client.go — cluster 401/403 silently swallowed (WARNING, correctness).** A cluster
   returning 401/403 was `continue`d, so an expired-mid-sync token yielded an empty menu with
   nil error; sync would then wipe the catalog and write a bogus empty snapshot week. Fix:
   401/403 on a cluster is now a hard error, and clusters-found-but-zero-meals returns an error.
2. **cookunity_sql.go — SELECT-only substring blocklist false-positives (WARNING).** Rejected
   legitimate reads whose literals contained "update "/"create "/etc. Fix: dropped the blocklist;
   the read-only (mode=ro) handle is the real write barrier. Kept the SELECT/WITH prefix check.
   Verified: `sql "... LIKE '%Grill%'"` now returns rows.
3. **sync.go — catalog wipe not atomic with repopulate (WARNING, data-loss window).** DELETE-then-
   UpsertBatch left the table empty if the batch failed/crashed. Fix: upsert the fetched week
   first, then `DELETE FROM meals WHERE delivery_date != <date>`; the table is never empty.
   Verified: two consecutive syncs keep the count at one week (221), not doubled.
4. **cookunity_sql.go — user SQL ignored the bound context (LOW).** Fix: run via
   `db.DB().QueryContext(ctx, query)` so --timeout/SIGINT interrupt long queries.
5. **chefs.go — cuisines aggregated as one joined string (LOW).** Fix: split m.Cuisines on ","
   so each cuisine is an individual entry. Verified: top chef shows
   ["latin american","american","african","asian"].

## Reviewed and found sound (no change)
- SDUI JSON walking: all accesses are comma-ok/type-switch; no nil-deref or unchecked assertions.
- toFloat/toInt/toStr/toBool: tolerant of string-vs-number JSON; zero-value on parse failure.
- Rows/handle lifecycle: every Query has a matching Close; no write inside an open rows iteration.
- Read-only vs read-write open selection is correct (snapshot paths open RW for CREATE IF NOT EXISTS).
- meal_snapshots upsert: correct composite PK + ON CONFLICT; writeMu held; no nested transactions.

## Not from review — verified separately
- `search`: works (returns {meta, results}; "Chicken & Dumplings" top hit for "chicken"). The
  "no_search_endpoint" string is a provenance label, not an error.
