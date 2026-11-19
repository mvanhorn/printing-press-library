# etsy-pp-cli — Phase 4 shipcheck proof

Run: 20261116-133635-274efd3c · press v4.35.6 · tree 99dde69 · 2026-11-16T20:51:08Z

## Umbrella (final run: proofs/shipcheck-run5.log)
| Leg | Result |
|---|---|
| verify | PASS — 159/159 (100%), 0 critical |
| validate-narrative | PASS — 12 ok, 0 failed examples, 1 unsupported (side-effectful `auth login --scopes` troubleshoot, expected) |
| dogfood | PASS — OAuth scope coverage 76/76, novel features 15/15, 0 dead flags/functions, MCP surface mirrors Cobra tree |
| workflow-verify | workflow-pass |
| apify-audit | PASS |
| verify-skill | PASS (flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command, canonical-sections) |
| scorecard | 96/100 — Grade A |
Verdict: PASS (7/7 legs).

## Fix history across runs
- run1: dogfood FAIL oauth_scope_coverage 0/76 (scope list moved out of auth.go; least-privilege default). Fixed d00977d: `auth login --all-scopes` (honest opt-in; read-only default kept).
- run2–3: validate-narrative FAIL — `market snapshot --terms terms.txt --dry-run` read the file before the dry-run short-circuit (68fb7bf). Run3 still red because shipcheck ran validate-narrative against a stale staged binary (logged machine issue).
- run4: PASS 7/7. Behavioral sampling then found real bugs the legs did not:
  - market snapshot planned 26 calls for a page that needed 79 (shop estimate 25 vs 100 fetched) → d17855f (--per-page, true upper bound).
  - BLOCKER: sync `listings` = GLOBAL /listings/active in default sync (447 of 5000 daily calls burned on one test sync) → 3fd3de3 (shop path; missing ETSY_SHOP_ID = one hint, zero requests; mutation-checked).
  - dead-weight reported zero sales with no transaction data (absent≠0) → 967d79b; false "store not synced" hints → f7705d9; listings synced active-only → 99dde69 (all 5 states, non-active 401/403 → one warning).
- run5: PASS 7/7 on the final tree.

## Behavioral sample of every novel command (live, API-key only; outputs kept private in proofs/novel-sample, proofs/shop-sample*.db)
- market snapshot: real search "ceramic mug" 1 page → 100 listings, 78 shops, budget cap → partial → --resume → complete (79 calls). Tags/views/original_creation_timestamp stored; review text never stored.
- market shops: 78 shops; crowd-out new 30% / established 70% of top-10. market gaps: "6 listings from 6 shops found in 1 pages of 1 terms" and "no listing found in 1 pages of 1 terms". market tags: 20-tag contrast (basis=position, honest). market price: medians per taxonomy; rho null because demand absent ("absent demand is not zero"). market demand: state absent (no reviews collected). market trend: [] + note (one snapshot).
- Shop commands on a real public shop (11 active listings, 49 reviews; key-only data): listings audit → real findings (no materials, short titles) + GPSR manufacturer/responsible-person missing; images rules honestly skipped. listings expiring → listing 2 days out, auto-renew true. reviews digest → 12 reviews/365d, histogram 5★×12. dead-weight → [] + transactions-not-synced note. inventory audit / money fees / money reconcile / shop attention → empty/unknown with accurate notes (OAuth-only data).

## Verdict: ship-with-gaps
Rules check (phases/12-shipcheck.md): all ship-threshold conditions hold; every known bug fixable in 1–3 files was fixed in-session. Remaining gaps need a sync-engine refactor (state-aware prune + checkpoints) or OAuth access to a real shop that this operator does not have; all are documented in README `## Known Gaps` and here:
1. `sync --full` cannot prune listings deleted on Etsy (per-state walks never prune, by design, to protect drafts).
2. A `--max-pages`-capped listings sync restarts instead of resuming (cursor is not state-scoped).
3. Shop-data features (orders, money, inventory, non-active listings) are verified offline against synthetic fixtures only; no shop/OAuth live run possible for this operator.
4. `shop attention` reports 0 + an "unknown, not zero" note for never-synced families (JSON shape unchanged).
