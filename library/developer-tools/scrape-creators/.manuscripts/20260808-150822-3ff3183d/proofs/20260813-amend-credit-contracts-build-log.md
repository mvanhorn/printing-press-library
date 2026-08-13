# Amend build log — amend-2026-08-13T1200 (credit-spend contracts)

Base: upstream/main post-#1680 merge (b20261cf27). Branch: amend/scrape-creators-20260813.

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
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cli	1.918s
?   	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cli/playbooks	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/client	1.887s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cliutil	11.598s
?   	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cliutil/testenv	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/config	1.138s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/learn	2.593s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/learn/entities	1.747s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/learn/lookups	2.859s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/learn/patterns	2.857s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/mcp	5.336s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/mcp/bound	3.843s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/mcp/cobratree	7.442s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/platform	3.674s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/store	3.848s
?   	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/types	[no test files]
```

## focused credit-contract tests
```
=== RUN   TestAccountEstimate_OverBudgetBoundary
--- PASS: TestAccountEstimate_OverBudgetBoundary (0.00s)
=== RUN   TestAccountEstimate_BalanceShapes
--- PASS: TestAccountEstimate_BalanceShapes (0.00s)
=== RUN   TestAccountEstimate_PerCommentUsesFlatUpperBound
--- PASS: TestAccountEstimate_PerCommentUsesFlatUpperBound (0.00s)
=== RUN   TestThreadRouting_AutoThresholds
--- PASS: TestThreadRouting_AutoThresholds (0.00s)
=== RUN   TestThreadForcedFlat_SkipsProbe
--- PASS: TestThreadForcedFlat_SkipsProbe (0.00s)
=== RUN   TestThreadDefaultKeepsSinglePageAndReportsTruncation
--- PASS: TestThreadDefaultKeepsSinglePageAndReportsTruncation (0.00s)
=== RUN   TestThreadTraversal_CompletesWithinBudget
--- PASS: TestThreadTraversal_CompletesWithinBudget (0.00s)
=== RUN   TestThreadTraversal_BudgetStopKeepsTruncatedTrue
--- PASS: TestThreadTraversal_BudgetStopKeepsTruncatedTrue (0.00s)
=== RUN   TestThreadTraversal_BreachHaltsImmediately
--- PASS: TestThreadTraversal_BreachHaltsImmediately (0.00s)
=== RUN   TestThreadTraversal_PerCommentReplyFetchesAreGated
--- PASS: TestThreadTraversal_PerCommentReplyFetchesAreGated (0.00s)
=== RUN   TestSweep_BudgetGatesCommentFetches
--- PASS: TestSweep_BudgetGatesCommentFetches (0.01s)
=== RUN   TestSweep_BreachHaltsImmediately
--- PASS: TestSweep_BreachHaltsImmediately (0.00s)
=== RUN   TestSweep_MaxPostsStops
--- PASS: TestSweep_MaxPostsStops (0.00s)
=== RUN   TestSweepBudget_FreshBudgetAlwaysAdmitsTheFirstFetch
--- PASS: TestSweepBudget_FreshBudgetAlwaysAdmitsTheFirstFetch (0.00s)
=== RUN   TestSweepBudget_EstimateNeverGoesStale
--- PASS: TestSweepBudget_EstimateNeverGoesStale (0.00s)
=== RUN   TestSweepBudget_AdmitsOnlyWhenWorstCaseFits
--- PASS: TestSweepBudget_AdmitsOnlyWhenWorstCaseFits (0.00s)
=== RUN   TestSweepBudget_HaltsWhenAFetchExceedsItsEstimate
--- PASS: TestSweepBudget_HaltsWhenAFetchExceedsItsEstimate (0.00s)
=== RUN   TestSweepBudget_NoBudgetNeverBlocks
--- PASS: TestSweepBudget_NoBudgetNeverBlocks (0.00s)
=== RUN   TestSweepBudget_StopNoteNamesFetchKindAndEstimate
--- PASS: TestSweepBudget_StopNoteNamesFetchKindAndEstimate (0.00s)
PASS
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cli	0.338s
```

## publish validate
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

## phase5 live dogfood (quick) — marker re-mint
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
