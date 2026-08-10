# Q-SYS CLI — Live Dogfood Acceptance

```
Acceptance Report: qsys
  Level: Full Dogfood
  Tests: 114/114 passed (100%, 0 failed, 0 skipped in matrix)
  Failures: none after 1 fix loop
  Fixes applied: 5
    - pp:no-error-path-probe on connect, product get, integrations
      (unknown models are honest empty/fallback results, not errors)
    - page index / product index: guardLiveJSON=false for the XML sitemap
      binary response; pp:typed-exit-codes "0,1" declaring the designed
      exit-1 for --json on a binary endpoint
  Printing Press issues: 2 (retro candidates)
    - generated binary-response endpoints call resolveReadWithStrategy...
      which hardcodes guardLiveJSON=true, misclassifying XML bodies as auth
      failures; the guard-exempt variant exists but is not emitted
    - generated binary endpoints refuse --json but do not declare the exit
      code, so live-dogfood json_fidelity fails until annotated
  Gate: PASS
```

## Behavioral verification (beyond the mechanical matrix)
The full dogfood matrix ran against a sandboxed HOME (empty corpus), so
corpus-driven commands were additionally verified against real harvested data:

- `harvest` (live, bounded): compat matrix + pages + products fetched; rc=0.
- `search "q-sys"` over a freshly harvested corpus: returned ranked page
  results with titles and URLs from real help.qsys.com content (FTS populated
  by the harvest rebuild).
- `compat check CX-Q TSC-70-G3 --qds 9.4`: both `supported` against the real
  59-row matrix.
- `page get Networking/Dante_Audio.htm` + `--version 9.4`: live fetches from
  the current and versioned doc trees (URLs resolve to /q-sys_9.4/...).
- `page index` / `product index`: real sitemap XML output, rc=0.
- `coverage`: reports real stored_pages / stored_products / stored_compat_rows.
- `product get`, `bom verify`, `integrations` with a partial corpus returned
  honest empty/partial results with explanatory notes (not errors).

No PII surfaced: both sources are public static documentation/product sites;
no accounts, workspaces, or personal data were touched.

## Phase 5 gate marker
`phase5-acceptance.json`: `{"status": "pass", "level": "full", "matrix_size":
114, "tests_passed": 114, "auth_context": {"type": "none"}}`
