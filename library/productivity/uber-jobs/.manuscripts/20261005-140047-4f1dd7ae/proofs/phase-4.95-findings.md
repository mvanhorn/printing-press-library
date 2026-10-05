# Phase 4.95 local code review: uber-jobs-pp-cli

Run 20261005-140047-4f1dd7ae, written 2026-10-05T18:23Z.

## Autofix summary

14 findings were autofixed in place across 3 rounds. 3 more were fixed after owner decisions (see the surface-to-user section). The working dir has no git, so there are no commit hashes. The record is the regression tests in `internal/uberjobs/uj_review_regressions_test.go` and `internal/cli/uberjobs_cmd_review_test.go`. Each test was mutation-checked: 24 mutations, each of which makes its test fail, plus 3 re-runs after the Gate.Do simplification.

## Template-shape retro candidates

- **`internal/cli/root.go:207`** (medium; template: the root `Execute` deliver block). `Execute` delivers `--deliver` output only when `err == nil`. A command that prints a meaningful partial result and then returns an error (here, `new --all` when one saved search fails) never reaches a file or webhook sink.
  - **Filed rather than fixed:** the file is generated. The CLI side was handled in hand-written code: `new.go` delivers its partial output by hand when it returns an error, and the owner's all-or-nothing rule means nothing is lost meanwhile.
  - **Proposed template fix:** deliver whatever the command already wrote, then return the error.
- **`internal/mcp` intent handlers** (medium; template: `RunCLICommand` callers such as `handleWeeklyNewSinceCheck` in `intents.go`). On a non-zero exit the handler returns `NewToolResultError(err.Error())`, which holds only stderr, so the stdout envelope is dropped.
  - **Filed rather than fixed:** generated code.
  - **Mitigation:** the owner's all-or-nothing rule for `new --all` means no baseline advances on a failure, so MCP callers lose nothing. They see the changes on the next successful run.
  - **Proposed template fix:** return stdout as content with `IsError=true`.

## Out-of-scope retro candidates

- **`internal/cliutil/filelock.go:51`** (low; generator-reserved `cliutil`). `WithFileLock` takes `sync.Mutex.Lock` and a blocking `flock(LOCK_EX)`, and neither honours a context. A process queued behind the request gate can overrun its `--timeout` inside `flock`.
  - **Filed rather than fixed:** `cliutil` is reserved.
  - **Mitigation:** since round 2, `Gate.Do` returns `ctx.Err()` as soon as it holds the lock, so an expired request sends nothing and writes no stamp.
  - **Proposed template fix:** a `WithFileLockContext` that polls `LOCK_EX|LOCK_NB` with backoff and uses a context-aware in-process semaphore.

## Surface-to-user findings

- **Round 2: `internal/cli/new.go:228`** (medium; category: two materially different valid fixes). When one saved search fails in `new --all`, the other searches have already advanced their baselines, but `--deliver` and MCP drop the output.
  - **Owner decision (2026-10-05):** "All-or-nothing". Every search is read and diffed before anything is written. Any failure holds every baseline, the partial envelope still goes to stdout and `--deliver`, and the first error's exit code is returned.
  - **Tests:** `TestUJCNewAllFailureHoldsEveryBaseline`.
- **Round 3: `internal/cli/new.go:204`** (medium) and **`internal/cli/new.go:251`** (low); category: findings persisting after the 3-round cap. Both were in the new all-or-nothing commit phase:
  - (a) Commits ran on the command context, so a read timeout made every held search's check write fail, and the rows' listings were wiped.
  - (b) A store-write failure on one search left the others advanced while the note said none had.
  - **Owner decision (2026-10-05):** "Fix both, no 4th round". All writes of a run now share one transaction (`AdvanceMembershipTx`, and `MarkChecked` on a `*sql.Tx`) and run on their own 15 s deadline detached from the read deadline (`context.WithoutCancel`). A write failure holds every row and keeps its listing.
  - **Tests:** `TestUJCNewAllWriteFailureAdvancesNothing` and `TestUJCCommitPlansSurvivesExpiredReadDeadline`.

## Convergence outcome

The review stopped at round 3 with 2 in-scope findings outstanding. They were surfaced to the owner, who chose to fix both in place without a 4th review round. Both are fixed with mutation-checked tests.

## Review path

Direct subagent dispatch through the Workflow tool, with 3 parallel Opus reviewers per round: correctness, security, and reliability with maintainability. Your rules cap reviewer subagents at 3. Round 1 reviewed every hand-written path (15 raw findings). Round 2 verified the fixes and looked for regressions (10 raw findings). Round 3 was limited to the round-2 changes (6 raw findings). The `/simplify` step ran in the main loop instead of as `/simplify` subagents, because of the same 3-reviewer cap. It merged Gate.Do's duplicated check, stamp and send sequence into `runGated`/`waitGap`/`stamp`.
