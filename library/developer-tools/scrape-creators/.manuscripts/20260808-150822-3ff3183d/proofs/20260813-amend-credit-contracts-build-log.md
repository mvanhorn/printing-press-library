# Amend build log — amend-2026-08-13T1200 (credit-spend contracts) — updated after Greptile round 1

Base: upstream/main post-#1680 merge (b20261cf27). Branch: amend/scrape-creators-20260813.
Round 2 adds: cursor-cycle detection, honest page-failure diagnosis, consolidated patch record.

## go build / vet
```
go build ./...  -> OK
go vet ./...    -> OK
```

## unit suite (full package list)
```
?   	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-cli	[no test files]
?   	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-mcp	[no test files]
?   	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cache	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cli	2.204s
?   	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cli/playbooks	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/client	2.002s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cliutil	11.158s
?   	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cliutil/testenv	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/config	1.200s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/learn	2.068s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/learn/entities	0.312s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/learn/lookups	2.630s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/learn/patterns	3.010s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/mcp	3.781s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/mcp/bound	2.781s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/mcp/cobratree	5.481s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/platform	2.578s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/store	3.248s
?   	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/types	[no test files]
```

## focused credit-contract tests
```
--- PASS: TestAccountEstimate_OverBudgetBoundary (0.00s)
--- PASS: TestAccountEstimate_BalanceShapes (0.00s)
--- PASS: TestAccountEstimate_PerCommentUsesFlatUpperBound (0.00s)
--- PASS: TestThreadRouting_AutoThresholds (0.00s)
--- PASS: TestThreadForcedFlat_SkipsProbe (0.00s)
--- PASS: TestThreadDefaultKeepsSinglePageAndReportsTruncation (0.00s)
--- PASS: TestThreadTraversal_CompletesWithinBudget (0.00s)
--- PASS: TestThreadTraversal_BudgetStopKeepsTruncatedTrue (0.00s)
--- PASS: TestThreadTraversal_BreachHaltsImmediately (0.00s)
--- PASS: TestThreadTraversal_CyclicCursorStopsWithoutRebuying (0.00s)
--- PASS: TestThreadTraversal_PageFailureIsDiagnosedHonestly (0.00s)
--- PASS: TestThreadTraversal_PerCommentReplyFetchesAreGated (0.00s)
--- PASS: TestSweep_BudgetGatesCommentFetches (0.01s)
--- PASS: TestSweep_BreachHaltsImmediately (0.01s)
--- PASS: TestSweep_MaxPostsStops (0.00s)
--- PASS: TestSweepBudget_FreshBudgetAlwaysAdmitsTheFirstFetch (0.00s)
--- PASS: TestSweepBudget_EstimateNeverGoesStale (0.00s)
--- PASS: TestSweepBudget_AdmitsOnlyWhenWorstCaseFits (0.00s)
--- PASS: TestSweepBudget_HaltsWhenAFetchExceedsItsEstimate (0.00s)
--- PASS: TestSweepBudget_NoBudgetNeverBlocks (0.00s)
--- PASS: TestSweepBudget_StopNoteNamesFetchKindAndEstimate (0.00s)
PASS
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cli	0.328s
```

## publish validate (round 2)
```
manifest: pass
transcendence: pass
phase5: pass
go mod tidy: pass
module path: pass
govulncheck: pass
go vet: pass
go build: pass
--help: pass
--version: pass
verify-skill: pass
manuscripts: pass
```

## phase5 live dogfood (quick) — marker re-mint (round 2)
```
{
  "status": "pass",
  "level": "quick",
  "matrix_size": 15,
  "tests_passed": 15,
  "tests_skipped": 9,
  "tests_unverified": 9
}
```

## library verify_skill.py
```
=== scrape-creators ===
  ✓ All checks passed (flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command)
```
