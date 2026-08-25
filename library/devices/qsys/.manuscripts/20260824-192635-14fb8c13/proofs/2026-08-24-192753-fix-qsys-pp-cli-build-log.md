Manifest transcendence rows: 8 planned, 8 built. Phase 3 gate PASS.

# qsys-pp-cli reprint build log (2026-08-24)

## Built (8/8 approved transcendence rows)
- product get, bom verify, compat check, connect, coverage — carried from prior print, registration hardened with root.Find + addNovelCommandIfAbsent so a regen of the generated parent cannot drop the leaf
- bom risks, fault, qds — NEW, implemented from TODO scaffolds
- All 8 verified: `<binary> <path> --help` exits 0 and Usage ends in `[flags]`
- Deterministic backstop: dogfood novel_features_check planned=8 found=8 missing=none

## Foundation
- Third source added: support.qsys.com (1,906 sitemap URLs across 11 categories) -> qsys_support table + FTS, `harvest --only support`, coverage reports per-category counts
- New shared reader internal/cli/qsys_support.go with pure helpers (normalizeFaultString, faultKey, faultRank, mentionsModel, articleVersions, versionRelevant)

## Removed per approved manifest
- integrations.go/_test.go, compat_deprecated.go/_test.go. Absorbed `compat deprecations` (generated endpoint) verified retained.

## Generator drift fixed
- ftsMatchQuery -> FTSMatchQuery (exported between press 4.30.1 and 4.31.1); carried hand-authored qsys_migrations.go referenced the old name and would not compile.

## The reprint's reason (CORRECTED 2026-08-24 — the DSN change does NOT fix the crash)
- internal/store/store.go DSNs now mmap_size(0) on both read-only and write handles (was mmap_size(268435456)); write DSN also gained _txlock=immediate. The DSN diff is real and verified in the binary.
- **Correction, measured post-build:** the change does NOT close the SIGBUS-on-concurrent-read defect. 20-way concurrent-read stress crashes both the old (2026.8.1) and reprinted binaries at similar rates (OLD 18/20, 12/20; NEW 14/20, 17/20). The crash stack is in modernc.org/sqlite `_walFindFrame` — the WAL-index (`-shm`) mmap, which the `mmap_size` pragma does not govern. Two concurrent readers already crash ~50%; strictly serial access is 100% safe. Filed upstream as cli-printing-press#4349. Use the CLI serially (no parallel MCP tool calls, no `&` fan-out) until a real fix lands.

## Known gaps carried forward (not introduced by this build)
- store.SearchCorpus is unreachable: `search` calls db.Search (generic resources table) only, so `search dante` returns [] even against the full 766-page corpus. Pre-existing on the shipped CLI. Wiring search to SearchCorpus is a behavior change deliberately left out of Phase 3 scope.
- CX-Q has is_product=0 (series landing page publishes no PDF) and four family-index pseudo-models carry is_product=1; both filters documented in code.
