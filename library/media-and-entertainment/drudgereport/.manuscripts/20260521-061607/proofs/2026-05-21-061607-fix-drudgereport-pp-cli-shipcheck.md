# Drudge Report Shipcheck

## Final results (after 1 fix loop)

```
Shipcheck Summary
=================
  LEG                  RESULT  EXIT  ELAPSED
  verify               PASS    0     1.664s
  validate-narrative   PASS    0     154ms
  dogfood              PASS    0     832ms
  workflow-verify      PASS    0     12ms
  verify-skill         PASS    0     80ms
  scorecard            PASS    0     138ms

Verdict: PASS (6/6 legs passed)

Scorecard: 84/100 Grade A
```

## Fixes applied during shipcheck loop

1. **Codex regex bug** (caught after T1, before T2): `internal/drudge/parser.go` used a Perl-style `(?=...)` lookahead in `rssImageRE`. Go's RE2 engine doesn't support lookahead. Replaced with two-branch alternation covering class-before-src and src-before-class orderings, fixed submatch indexing.
2. **Duration syntax in narrative examples**: `--window 7d` in `research.json` examples failed `time.ParseDuration` (Go supports h/m/s but not d). Updated `novel_features[*].example`, `novel_features_built[*].example`, and `narrative.recipes[*].command` to use `168h` consistently. After fix, all 10 examples resolved and `validate-narrative --strict --full-examples` flipped from FAIL to PASS.
3. **Stale staged binary**: `build/stage/bin/drudgereport-pp-cli` was the original generate-time build without the 10 novel commands wired. Scorecard's live-check probe picked it up and reported 0/10. Rebuilt the staged binary; probe now reports 7/10 pass.

## Live Sample Output Probe (after rebuild)

```
Passed: 7/10  (70% pass rate)
Failures:
  - Splash now:        exit 1: database is locked (5) (SQLITE_BUSY)
  - Ranked headlines:  exit 1: database is locked (5) (SQLITE_BUSY)
  - On-this-date:      exit 4: no snapshots in local store
```

All three failures are scorecard-concurrency artifacts:

- **SQLITE_BUSY on splash/headlines:** The probe runs the 10 commands in parallel. Two write commands hitting the same SQLite file collide. Verified manually: running each command standalone returns exit 0 with the expected JSON envelope.
- **on-date "no snapshots":** The probe queried `2026-04-15T08:30` while a parallel splash/headlines probe was still writing the first snapshot. The on-date query saw an empty `drudge_snapshot` table at that exact moment. Verified manually: `drudgereport-pp-cli on-date 2026-04-15T08:30 --json` returns the nearest available snapshot (today's) with `exit 0`.

These are scorecard parallelism artifacts, not real correctness bugs. The commands themselves behave correctly under normal use (one user, one process at a time). Fix-by-tightening-the-sqlite-busy-handling is a printing-press generator concern (the store's open/close path is generator-emitted); not patching it in the printed CLI per the AGENTS.md "machine vs printed CLI" rule.

## Dogfood warnings (informational, not blocking)

```
- 1 dead flag found: allowPartialFailure
- 2 dead helper functions found: detectPartialFailure, partialFailureErr
- defaultSyncResources empty: sync command is a runtime no-op
- 1 source client file(s) without rate-limit handling: internal/drudge/fetch.go
- pure-logic packages with no tests: drudge
```

- The dead flag/funcs are generator-emitted partial-failure scaffolding for APIs that return partial-success envelopes. Drudge's HTML scrape has no such envelope; these are unused but harmless. Retro candidate for the generator (skip the partialFailure scaffolding when `auth.type: none` and no JSON response types).
- `defaultSyncResources empty` is expected: Drudge has no list endpoints to sync (the data layer is populated as a side effect of `splash`/`headlines`/`breaking` calls, intentionally). Not a fix-here issue.
- Rate-limit gap in `internal/drudge/fetch.go` and no tests in `internal/drudge` are real polish opportunities. Both can be added in Phase 5.5 polish pass.

## Top blockers (none)

No blockers. The CLI builds, vets clean, runs end-to-end against the live drudgereport.com and the unofficial RSS mirror, and produces correct ranked/red-aware/slot-aware output. SQLite contention is the only operational artifact and is bounded to scorecard parallel probes.

## Before/after

- Verify pass rate: not run pre-fix vs post-fix (single shipcheck loop).
- Validate-narrative: FAIL (1 example) → PASS (0 failures).
- Scorecard total: 83/100 → 84/100.
- Live sample probe: 0/10 → 7/10.

## Final ship recommendation

**ship**

All 6 shipcheck legs PASS. The 3 remaining live-probe failures are scorecard concurrency artifacts (manually reproducible only when two commands write to the same SQLite file in parallel); they do not represent broken behavior under normal single-process use. No flagship feature returns wrong or empty output. Known gaps are confined to generator-emitted dead code and a rate-limit/test polish opportunity in the `internal/drudge` package — both candidates for Phase 5.5 polish, neither a ship blocker.
