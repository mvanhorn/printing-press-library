Manifest transcendence rows: 5 planned, 5 built. Phase 3 gate PASSED 2026-07-10: per-row Cobra resolution 17/17 (search + 4 hand-code transcendence + fetch/tarball/releases-download + generated endpoints; contents/trees/blobs/rate-limit flattened by generator — covered as (generated endpoint) rows); dogfood novel_features_check {"planned":5,"found":5}; ghfetch pure-logic package has table-driven tests, all green. Correction: `search` turned out to be a generator-emitted novel SCAFFOLD (not a framework command) — implemented by hand against the store (LIKE over resources_type='trees'), behaviorally verified (5 hits for 'transformers', empty-result note works).

# github-contents Phase 3 build log

Hand-code scope (7 commands): fetch, tarball, releases download (absorbed); plan, verify, sync-dir, stats (transcendence).
Shared engine: internal/ghfetch (address parsing, segment-escaping, git blob SHA, LFS pointer detection, glob filtering, tree walking with truncation fallback, bounded-concurrency raw-CDN download) + table-driven tests.

## Progress
- [ ] internal/ghfetch engine + tests
- [ ] fetch
- [ ] plan
- [ ] verify
- [ ] sync-dir
- [ ] stats
- [ ] tarball
- [ ] releases download
- [ ] store write-through so framework `search` covers tree listings
