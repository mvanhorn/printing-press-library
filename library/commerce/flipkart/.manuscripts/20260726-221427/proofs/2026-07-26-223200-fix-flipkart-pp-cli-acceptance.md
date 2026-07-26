# Flipkart CLI — Phase 5 Acceptance Report

**Level:** Full Dogfood
**Tests:** 64/65 passed (live mechanical matrix via `printing-press dogfood --live --level full`)
**Gate:** See verdict below

## Failure

`sync` reported an error syncing the `feed` resource (Affiliate API) because no `FLIPKART_AFFILIATE_ID`/`FLIPKART_AFFILIATE_TOKEN` were configured — no Flipkart affiliate account was available to test the credentialed path in this session (disclosed to the user at the start of the run).

**Verified by hand this is not a defect:**
```
$ flipkart-pp-cli sync
{"event":"sync_error","resource":"feed","status":401,...,"error":"GET /1.0/search.json returned HTTP 401: Invalid Headers"}
{"event":"sync_summary","total_records":1,"resources":2,"success":1,"warned":0,"errored":1,...}
{"event":"sync_warning","reason":"exit_policy_default_changed","errored":1,"message":"1 resource(s) failed but exit code is 0 because the new default treats non-critical failures as warnings..."}
EXIT_CODE=0
```
`sync` exits 0 and treats this as a non-critical warning by design (the `feed` resource was never marked `critical`, matching its optional/credential-gated nature). The dogfood harness's test classification flags any per-resource sync error as a matrix failure regardless of exit code — stricter than the CLI's actual, correct runtime contract.

**Retro candidate (Printing Press generator, not this CLI):** `sync` has no mechanism to skip a resource up front when its tier's required auth env vars are unconfigured — it always attempts the fetch, gets a 401, and reports it as a per-resource error (even though exit code correctly stays 0). A cleaner UX would pre-check tier auth availability and print a one-line "skipped: feed (affiliate credentials not configured)" instead of surfacing a raw 401. Filed as a retro candidate; not fixable in this session without hand-editing a generated (DO NOT EDIT) file.

## Independently hand-verified live (beyond the automated matrix)

- `catalog "wireless earbuds"` — live search, correct JSON-LD results
- `product get <real-url>` — live fetch, correct title/brand/price/rating/images/description
- `compare <url1> <url2>` — live fetch of 2 real products, correct side-by-side JSON
- `offers best-card <url1> <url2>` — live fetch + bank-offer aggregation, correct ranked totals
- `watch add` / `watch check` / `watch digest` — smoke-tested during Phase 3 build
- `catalog diff` — smoke-tested during Phase 3 build (baseline-then-diff behavior confirmed)
- `feed digest` / `deals category` — correctly return an actionable error naming `FLIPKART_AFFILIATE_ID`/`FLIPKART_AFFILIATE_TOKEN` when credentials are absent; cannot be tested against the real credentialed path without an affiliate account

## Fixes applied during this run (from the shipcheck loop, folded into this report for completeness)
- Fixed positional-arg / dry-run-ordering bugs in `deals category` and `watch add`
- Fixed dry-run short-circuit ordering in `feed digest` (and audited `deals category` for the same class of bug)
- Fixed 2 real extraction bugs found during Phase 3 smoke testing: bank-offers now parsed from the real tracking-JSON shape (the naive DOM-text shape isn't actually rendered server-side); `catalog diff`'s snapshot-matching fixed (was comparing `time.Time` values that don't round-trip equal through SQLite)
- Corrected research.json quickstart/recipe examples (removed a nonexistent `--limit` flag, replaced fake placeholder product URLs with real, verified ones)

## Gate

**Recommendation: ship**, with the `feed`/`sync` interaction documented as a Known Gap. All primary (no-auth) flagship functionality — the CLI's actual differentiators — is verified working against live data. The one dogfood-flagged item is a credential-gated optional feature behaving exactly as designed (graceful non-fatal degradation), not a functional defect.
