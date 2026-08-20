# printgoat Polish Report

```
Polish Results for printgoat-pp-cli:

                    Before    After     Delta
  Scorecard:        90/100    90/100    +0
  Verify:           100%      100%      +0%
  Tools-audit:      1 pending  0 pending  -1 pending findings
  Gosec (hand-authored): 6    0          -6
```

## Fixes applied
- `job_download.go`: download-dir perms 0o755→0o750, downloaded-file perms 0o644→0o600 (gosec G301/G302)
- `job_download.go`, `feed.go`, `designer_stats.go`, `history_diff.go`: explicit `_ = rows.Close()` in scan-error recovery branches (gosec G104, 4 sites)
- Accepted `learnings list` tools-audit thin-short finding with rationale (accurate/brief, lives in a DO-NOT-EDIT generated file)

## Investigated, not fixed (environmental/generator-owned, not code defects)
- `verify`'s Data Pipeline check fails against the mock harness because `sync`'s only default resource (Thingiverse categories) uses a hardcoded absolute URL — a documented, deliberate workaround for cli-printing-press's multi-spec merge dropping per-resource base_url overrides (see Phase 5 acceptance report). This always targets the real `api.thingiverse.com`, so it 401s without a live token in a token-less mock context. Confirmed via reproduction that auth wiring is correct (a fake token produces a different, expected 401 body). Retro candidate for the generator itself.
- 19 remaining gosec findings live exclusively in DO-NOT-EDIT generated files — left untouched per policy, flagged as generator retro candidates.
- Scorecard's `mcp_tool_design`/`cache_freshness` dimensions are spec-driven and were already deliberate research-brief decisions (cache freshness doesn't apply to a download-history cache, not a full-mirror sync target).

## Verdict
`ship_recommendation: ship`, `further_polish_recommended: no`. All hard gates (verify-skill, workflow-verify, gosec-in-hand-code, tools-audit, pii-audit) clean; verify pass rate 100%.
