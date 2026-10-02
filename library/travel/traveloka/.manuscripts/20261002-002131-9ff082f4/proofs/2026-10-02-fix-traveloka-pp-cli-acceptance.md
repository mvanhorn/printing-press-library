# Traveloka Phase 18 acceptance

Level: **Full dogfood**. Tests: **156/156 measured checks passed; zero failures; 86 explicitly skipped rows**. Gate: **PASS**. Required polish, promotion and archive remain pending.

The Printing Press-owned runner wrote `phase5-acceptance.json` and `18-dogfood-results-corrected-final.json` without agent edits to markers or counts. It enumerated 52 nodes, including eight help-only groups. Its native `CaptureSourceFingerprint` independently confirms all 219 current source files match fingerprint `f28e476891bcda8f6bf6769932d682d2fc8253f4e9e816cd3717ac61aa4f24d1`. This uses the runner's module/import normalization; raw file SHA equality is not the fingerprint contract. The omitted `coverage_hollow` and `hollow_features` fields decode to false/empty. Earlier hollow and failed evidence remains preserved.

Actual core happy paths contain no dry-run arguments or preview output. The final matrix created five new source snapshots with eight offers: two flight retrievals, two room retrievals and one hotel catalog retrieval. Both grids attempted an explicit cell and returned a populated retrieval snapshot. Local inspection and comparison commands read this real history. Cancellation comparison returned zero comparable pairs and two labelled unpaired source offers. Quote diff compared two exact identities with zero changes/new/missing offers. These sparse outcomes do not invent matching rates or price changes.

| Approved feature | Real evidence |
|---|---|
| 1. Resolve cities/airports/properties | Owned resolve/source lookups; manual city/property resolution |
| 2. One-way flights | Owned flight grid; manual one-way and second-route searches |
| 3. Complete return flights | Owned search with return date, both legs and source-prefetched trip total; manual return workflow |
| 4. Flight offer inspection | Owned inspect and manual source-snapshot inspection |
| 5. Dated hotel catalog | Owned Bangkok family catalog; manual Bangkok family and Singapore solo/USD |
| 6. Room/rate inspection | Owned rooms/grid; manual six source room offers |
| 7. Matching-context comparison/handoff | Owned compare/history and canonical URLs; five exact source price matches |
| 8. Agent help/output/input/errors | Owned help/JSON/preview checks; meaningful validation and labelled empty/invalid evidence |
| 9. Reproducible anonymous setup | Separate actual capture/import, eight scoped profiles, browser closed, subsequent Go HTTP verification |
| 10. Flight date grid | Owned populated cell plus manual two-date grid |
| 11. Hotel stay grid | Owned populated cell plus manual two-stay grid |
| 12. Flight trade-off frontier | Owned shortlist plus manual source-snapshot frontier |
| 13. Same-room cancellation difference | Owned flexibility, honest unpaired rates; simulated branch tests labelled separately |
| 14. Snapshot changes | Owned exact-identity diff plus manual matched-context diff |

`18-manual-live-summary-immediate.jsonl` records 15 richer workflows across two flight routes, one-way/return travel, differing passengers, Bangkok/Singapore hotel destinations, differing stays/occupancy, child ages, and SGD/USD. `18-source-price-crosschecks.json` records five exact offer matches across family hotel, one-way and return comparisons under matching context. These complement the owned matrix. Some bounded flight retrievals retain explicit `incomplete` status when upstream polling has not announced completion; prices remain fresh, indicative quotes, with no full-inventory or global-cheapest claim.

All 86 skipped rows remain excluded from the measured pass count. Actual statuses are 156 pass/86 skip/zero unverified. The runner's legacy `unverified` aggregate also reports 86 for the skipped rows. Auth/setup is excluded by the framework filter and is disclosed as separate real setup evidence. Exact skip reasons are:

| Reason | Rows |
|---|---:|
| No positional argument | 32 |
| Non-ID interface positional | 2 |
| Command does not honor preview | 5 |
| Non-ID text positional | 3 |
| Mutating error probe skipped | 8 |
| Non-ID resource positional | 3 |
| No preview short-circuit | 3 |
| No ID from companion | 3 |
| Non-ID query positional | 7 |
| Mutation preview only | 6 |
| Previously failed list companion | 3 |
| Non-ID profile name positional | 11 |

One CLI fix in this continuation: the eight advanced source previews now return canonical `dry_run:true` with a named POST-path action before data-file/session/client IO. Regression cases for all eight actual commands with missing input/session paths pass, alongside bounded-object/conflict, core preview and invalid-input checks. CLI and MCP builds pass. Both changed files already belong to `.printing-press-patches/full-live-source-fixture-contract.json`.

Printing Press repairs remain isolated in `/private/tmp/traveloka-press-runner-fix`: boolean serialization, truthful confined local-write execution and reuse of identical successful happy/JSON execution. Focused boolean, confinement/symlink, single-execution and hidden/overview safety checks pass. Unrelated legacy shell-stub test failures remain documented, not claimed passing. The global installation was unchanged. `18-dogfood-acceptance-corrected-preview-fail.json` preserves the genuine intermediate 148/156 result with eight preview failures; the earlier 140/140 hollow marker is not used for acceptance.

`18-full-matrix-effects-independent.json` bounds every selected example to eight pinned read-only source operations, seven live commands writing inside the symlink-free disposable fixture home, or scoped local/preview/skipped infrastructure. Optional feedback/delivery/profile/base overrides were unset. Legacy feedback/profile preview-ignoring writes are confined to the runner's temporary HOME. No booking, payment, account change, outbound feedback, global installation, publication or PR occurred. Private values remain in mode-0600 TMP files; eight public fixtures exclude them. The exact-value scan checked 593 run files against 658 private values with zero leaks before this manuscript; the saved-manuscript scan is recorded separately.

Durable evidence: `18-corrected-final-evidence-summary.json`, `18-corrected-final-fingerprint-audit.json`, `18-live-capture-corrected-final.json`, `18-public-fixture-preparation-corrected-final.json`, `18-advanced-preview-fix-tests.txt`, `18-full-matrix-selected-args.json`, and `18-scan-private-values.py`. The parent handles phase receipts, required forked polish and local promotion/archive. Any later Go edit requires fresh owned full acceptance before promotion.

## Final gate after polish

Phase 19 reran the real full matrix after its source changes: 156/156 measured checks passed, zero failed, and 86 disclosed skips. The current untouched runner marker binds all 219 source files to `ee7bed075277eac02d4f0d0c21e0f894104ff95970db43b1d332ea03bda6e7b1`. Both real workflows and all 31 verification checks pass. The original phase-18 evidence above remains historical; the phase-19 polish proof records the final accepted tree.
