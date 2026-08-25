# CookUnity CLI Polish (Phase 5.5)

ship_recommendation: ship | further_polish_recommended: no

|                | Before | After |
|----------------|--------|-------|
| Scorecard      | 77     | 77    |
| Verify         | 81.5%  | 81.5% |
| gosec (hand)   | 1      | 0     |
| go vet         | 0      | 0     |
| Tools-audit    | 1 pending | 0 |
| cookunity tests| none   | passing |

## Fixes applied
- drift.go: handled dropped db.Close() error (gosec G104, hard gate) — the only hand-authored finding.
- client.go: typed 429 handling at the transport boundary (throttle aborts sync with retry guidance).
- client.go: wired cliutil.AdaptiveLimiter (2 rps auto-ramp) to pace the many per-sync cluster fetches.
- client_test.go (new): unit tests for toStr/toFloat/toInt/toBool, joinStrings/joinNamed, flattenMeal
  (nested chef + nutrition fallback), collectClusters, collectMealProperties — all pass.

## Skipped (scorer/structural, not defects)
- verify 5 low scorers (compare + framework parents) — verify execute-probe can't run Cobra
  groups / arg-requiring leaves; each prints help and exits 0.
- Path Validity 0/10 — internal-yaml spec (paths validated at parse; dogfood SKIPs).
- MCP Token Efficiency / Tool Design 7/10 — structural for a 3-tool reverse-engineered surface.

## Remaining (generator retro candidates)
- 8 dead helper functions in generated internal/cli/helpers.go (over-emission of the sync-helper
  library); removal needs editing a DO-NOT-EDIT file regen would clobber. → /printing-press-retro.
- 19 gosec findings in generated DO-NOT-EDIT files (validated-identifier SQL, inherent CLI file reads).
- Windows generated-test isolation (HOME/USERPROFILE) + NTFS-DACL credential-perm tests.
