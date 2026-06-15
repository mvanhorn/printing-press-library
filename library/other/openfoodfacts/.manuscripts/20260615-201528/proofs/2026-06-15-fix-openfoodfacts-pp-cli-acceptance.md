# Open Food Facts CLI — Phase 5 Acceptance

  Level: Full Dogfood (binary-owned live matrix, no-auth API)
  Tests: 102/102 passed (64 skipped — no-positional error paths), 0 failed
  Gate: PASS

## Fixes applied this phase (4 dogfood failures → 0)
1. **`product <invalid-barcode>`** exited 0 on OFF's `{"status":0}` envelope → now returns typed not-found (exit 3). Valid barcodes still exit 0. (`isProductNotFound` + test.)
2. **`tail <invalid-resource>`** swallowed a 401 and exited 0 → now validates the resource against the known set and returns a usage error (exit 2). (`tailResourceKnown`.)
3. **`sync` (default)** pulled the 1199-record `find` search corpus → tripped the OFF search rate limit (10/min) mid-matrix. `find` removed from `defaultSyncResources()`; default sync now = attribute-groups + cgi + preferences (75 records, ~180ms). Aligns with OFF's "don't crawl search, use the dump" guidance. `find` is opt-in via `sync --resources find` (the `rank` command and the offline-ranking recipe point there).
4. **`workflow archive`** hardcoded `find` → 35s sync → dogfood timeout (exit -1). Now uses `defaultSyncResources()`, consistent with `sync`.

## Behavioral spot-checks (live)
- compare / diary add+today / allergens check (exit 3) / recipe (per-serving=total/4) / swap (10 healthier alts) / budget / rank (1199 synced rows) — all verified earlier in the run.
- product valid barcode → exit 0; product invalid → exit 3; tail invalid → exit 2; sync default → 75 records clean.

## Printing Press issues for retro
- Endpoint-mirror commands don't treat API-level "not found" envelopes (HTTP 200 + status:0) as failures — generic across APIs that signal errors in the body. (Worked around per-CLI in product.)
- `workflow archive` hardcodes its resource list instead of reusing `defaultSyncResources()`, so excluding a heavy/rate-limited resource from default sync doesn't propagate to archive.

## Gate marker
`proofs/phase5-acceptance.json` — status: pass, level: full, 102 passed, 0 failed.
