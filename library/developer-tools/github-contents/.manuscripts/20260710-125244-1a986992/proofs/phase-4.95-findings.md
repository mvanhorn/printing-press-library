# Phase 4.95 local code review findings — github-contents

**Review path chosen:** direct reviewer-subagent dispatch via the Agent tool — compound-engineering ce-correctness-reviewer, ce-security-reviewer, ce-maintainability-reviewer over the hand-written scope (internal/ghfetch/, internal/cli/{ghfetch_common,fetch,plan,verify,sync_dir,stats,tarball,releases_download,search}.go), plus two general-purpose docs auditors (Phase 4.8 SKILL semantic review, Phase 4.9 README/SKILL/AGENTS audit).

**Autofix summary:** 24 findings routed to a single autofix batch (S1-S4, C1-C11, M1-M5, D1-D6) executed by the original implementation agent; fix round 1 of max 3. Convergence outcome appended below after re-review.

**Skipped (with reasons):**
- Constructor naming consistency (newFetchCmd vs newNovel*Cmd) — `newNovel*` may be a generator scaffold contract for regen body-preservation; renaming is risk without evidence of benefit.
- README troubleshooting omitting the exact BFS request cap (500) — truncated flag is disclosed; numeric cap is internal detail.
- persistTreeEntries stale-ref accumulation (residual risk) — mild contract drift on `search` help text; noted for v2, not user-visible enough to churn now.

**Template-shape / out-of-scope retro candidates (file with /printing-press-retro, do NOT patch in place):**
1. Framework `--select` silently strips all fields from nested array objects on unknown selector names instead of erroring (generator-emitted filterFields in helpers.go; caught live by Phase 4.85 — the CLI's own generated example used a wrong selector and produced arrays of empty objects).
2. `auth set-token` is fully implemented in generated auth.go but never registered on the auth parent (auth.go only adds setup/status/logout) — while the generated SKILL.md documents set-token as a step. Generator wiring/docs mismatch.
3. Generated SKILL.md learning-loop boilerplate instructs "run `<cli> sync`" even when the spec has no syncable resources and no `sync` command is emitted.
4. Root `--help` omits the `repos` and `releases` command blocks in the Available Commands listing even though both resolve (help-rendering quirk observed during Phase 4.9 audit).
5. Generated README env-var table marks auth env vars Required: Yes for auth-optional (public-API) CLIs.

**Surface-to-user findings:** none — no finding required a real tradeoff (no scope shrink, no competing fixes, no research miss, no long-phase re-run).

## Convergence outcome
Findings cleared at round 3. Round 1: 24-finding consolidated batch (5 reviewers + output review) — all applied. Round 2: adversarial re-review found 2 fix-introduced errors (orphan-temp filter mismatch; ':'-rejection bricking POSIX fetches) + 6 warnings — ADV-8 no_change_needed (dry-run ordering is the Printing Press template contract), rest fixed in round 3 with named regression tests (TestPlanJobsDivertsUnsafePaths, TestCheckBlobSHA, TestListLocalFilesSkipsPartialTemps, TestListLocalFilesOrphanFromRealStreamToFileFailure) and live probes. Round-3 gates: gofmt/build/vet/test all green; staged binary rebuilt; fetch/verify round-trip ok:true; plan still 118 files / 1.92 GB.
