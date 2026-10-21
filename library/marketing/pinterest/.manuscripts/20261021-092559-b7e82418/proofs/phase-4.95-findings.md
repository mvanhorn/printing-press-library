# Phase 4.95 — local code review

Review path: direct subagent dispatch (correctness + security + maintainability personas, one combined reviewer per round).

Autofix summary: 11 findings autofixed in-place across 2 rounds; see commits afd7d8f 7baf8a1 84e747e .

Convergence: round 2 returned only 2 low findings (ads report resume UX), fixed in the last commit; the diff was 12 lines and was verified by build/vet/tests. A third reviewer round was not dispatched.

Post-fix simplification: shared helpers were consolidated during round 1 (pinterest_local.go, slices.Contains). /simplify was not run separately.

## Template-shape retro candidates
- dogfood collectRegisteredCommands ignores addNovelCommandIfAbsent, which the generator itself emits for novel parents. This produced a false depth mismatch ("ads report" vs "ad-accounts ads") and forced a direct AddCommand in root.go/ads.go.
- The OpenAPI pagination detector does not recognize the Pinterest cursor name "bookmark". Every v5 list resource is emitted as offset/no-cursor, so sync and --all stop after one page. Patched in the printed CLI via the determinePaginationDefaults wrapper.
- The novel-command scaffold rule "len(args)==0 && NFlag()==0 → Help" stops env-only invocations from running (ads report with only PINTEREST_AD_ACCOUNT_ID set).
- auto_refresh.go readCommandResources plus the README/SKILL Freshness template list ~75 command paths that do not exist in the cobra tree (resource-name keys rendered as commands).
- The readme.md.tmpl agentcookie section names a non-existent auth-status command.
- The generator's v4.33.8 go.mod template pinned go1.26.9 (GO-2026-6809); fixed on main by #5131 but not yet released.
- After a spec change, a regen leaves a .preserve snapshot that blocks the next regen. Same-spec regen of an unchanged overlay was classified as cross-spec (novel-only preservation) and dropped templated edits.

## Out-of-scope retro candidates
None.

## Surface-to-user findings
None.
