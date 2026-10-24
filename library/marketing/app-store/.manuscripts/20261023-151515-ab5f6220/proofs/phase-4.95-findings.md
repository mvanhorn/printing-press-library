# Phase 4.95 local code review: app-store-pp-cli

Run 20261023-151515-ab5f6220, 2026-10-24. Scope: every hand-written, non-test file in internal/cli, internal/appstore, internal/store and cmd that changed since the generated baseline (53 files, about 22,000 lines). internal/cliutil and internal/mcp/cobratree were excluded as generator-reserved.

**Autofix summary.** 39 findings were fixed in place over 3 rounds:

- round 1: 31 distinct findings after removing overlaps (36 as reported), from correctness, security and reliability reviewers;
- round 2: 6 defects introduced by the round-1 fixes;
- round 3: 2 items.

Commits: 080a53a, 1ca2bea, 82ead25, c06bba1, 184d919, a7f6625, 095ba8d, 141cdc3, 6e2f8af, edfcff7, 3892cae, d714df0, ee49ebb, 65b9978. Commits cb54e05 (output review) and e2fa1de (headline) were reviewed in round 3. Every behaviour fix has a regression test, and `go build`, `go vet` and `go test ./...` are green.

**Review path chosen.** The reviewers were dispatched directly by a workflow.

- **Round 1.** Four finders: correctness on the client and live commands, correctness on the store and local readers, security, and reliability with maintainability. Each lens had one adversarial verifier. The security verifier confirmed all 4 of its findings. The correctness and reliability verifiers hit the session usage limit, so I verified those findings inline while fixing them; every finding has a scratch-test reproduction from its reviewer.
- **Rounds 2 and 3.** One reviewer read only the fix diff.

**Native timeout-boundary check.** All 16 live command files and the categories `--live` path call `boundCtx(cmd.Context(), flags)` before their first request.

## Template-shape retro candidates

- **The bare-invocation help guard.** The scaffold's verify-friendly pattern, `if len(args)==0 && cmd.Flags().NFlag()==0 { return cmd.Help() }`, was copied into hand-written commands that have no required input: categories list/games, top, featured, changes, movers, breakouts and keyword-share. A bare call printed help instead of running the command's defaults, and the MCP shell-out returned help text when called with no parameters. Fixed locally in the hand-written files, and filed for the press: the guidance in helpers.go should limit the pattern to commands with a required positional or flag.
- **`resource-path:export` and the stale staged binary.** Both are press issues found in phase 12, recorded in the shipcheck proof.

## Out-of-scope retro candidates

None. No finding fell in internal/cliutil or internal/mcp/cobratree.

## Surface-to-user findings

None. No fix needed a scope, dependency or design tradeoff.

## Convergence outcome

Round 3 (a re-review of the round-2 fixes plus the output-review and headline changes) found 2 items:

- N2 partly resolved: the traction cap flag could reset when a later capture saw the same reviews again.
- A low-severity miscount in the changes never-captured note.

Both were fixed with regression tests in 65b9978. The cap is now read from the capture log (a full page 10 of the reviews RSS), and the note counts only resolved apps. The 3-round cap was reached, so no fourth review round ran. What is left is a capture-log query and a note count, both covered by tests and by the full suite.

Post-fix simplification (/simplify) was deferred to phase 19 polish to save weekly usage: the owner's weekly limit was at 74% when the run resumed. The fix output had already passed three review rounds.
