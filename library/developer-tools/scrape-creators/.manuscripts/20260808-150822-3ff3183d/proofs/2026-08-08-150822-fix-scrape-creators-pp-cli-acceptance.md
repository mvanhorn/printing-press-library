# scrape-creators-pp-cli Acceptance Report (Phase 5) — reprint run 20260808-150822-3ff3183d

```
Acceptance Report: scrape-creators
  Level: Quick Check (live, real credentials) + supplementary full 635-probe live matrix
  Tests: 15/15 passed (quick gate; 9 skipped-by-design)
  Auth: api_key via SCRAPECREATORS_API_KEY (user-provided key)
  Gate: PASS
```

## What was tested live
- doctor, list commands across platforms, sync to the local store, offline search over synced data, JSON/select/csv output modes, novel-feature sample (quick matrix).
- Supplementary full matrix: every leaf command (help, happy-path, JSON fidelity, error path) — 496/635 pass; the 139 failures are fixture-bound (synthesized mock handles/urls against live scraping endpoints that require real public targets). ScrapeCreators does not charge failed requests (verified: error envelopes carry credits_charged=0).
- The comment-thread flagship was verified against a real public reel during development of the routing logic; unit tests pin the routing boundary (>=15 flat, tie prefers flat for completeness).

## Credits
- Full matrix + quick matrix + live verify spent ~1,300 credits total (balance moved ~15,400 → ~14,100). The full-matrix spend exceeded the pre-run estimate (50-200) because successful probes across 28 platforms charged 1-2 credits each; recorded here for honesty. Failed probes were free.

## Fixes applied during Phase 5
- pp:no-error-path-probe on comments coverage (empty local result for unknown handle is valid, not an input error).
- pp:happy-args fixture for apple-music list (endpoint requires id|url).

## Printing Press issues (retro)
- Full live dogfood on many-platform scraping APIs is fixture-bound by construction; a per-platform fixture registry (real public handles) would make level=full attainable.
