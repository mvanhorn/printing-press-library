# Acceptance Report: thunderbird
  Level: Full Dogfood (runner-owned live matrix, real local Thunderbird profile via THUNDERBIRD_ROOT)
  Tests: 153/153 passed (117 skipped by runner rules: no positional / non-id positionals / declared exit 3 / side-effect commands dry-run only)
  First run: 151/153 — `feedback list` and `profile list` (generated framework commands) ignored --dry-run and printed `[]`, which the dry_run_json contract reports as "invalid JSON".
  Fixes applied: 1
    - CLI fix: internal/cli/tb_dry_run_lists.go hook makes both commands honour --dry-run (writeDryRun envelope); test TestTBListCommandsHonourDryRun; mutation (branch disabled) caught.
  Printing Press issues: 2
    - Generated `feedback list` / `profile list` templates do not short-circuit --dry-run.
    - live_dogfood liveDogfoodDryRunJSONContract reports a valid JSON array as "invalid JSON" instead of "does not honour --dry-run".
  Privacy: non-help output samples in the saved results were redacted (real mailbox content); remaining addresses are fictional example.com fixtures from help text.
  Gate: PASS
