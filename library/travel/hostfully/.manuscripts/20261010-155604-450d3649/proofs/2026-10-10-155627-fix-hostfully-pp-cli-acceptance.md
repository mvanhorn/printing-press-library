Acceptance Report: hostfully
  Level: Full Dogfood (no --allow-destructive; `inbox reply` exercised via --help only)
  Tests: 147/147 passed (131 skipped by the runner: positional fixtures, mutating commands, dry-run-only)
  Failures (first pass, all fixed):
    - 11 hand-written commands returned empty stdout for --dry-run --json
    - fetch --json mixed progress lines into stdout
    - whois returned exit 0 for a non-matching human-mode query
    - 7 commands lacked Examples in --help (inbox threads/messages/message, jobs list, quote, leads, properties)
  Fixes applied: 4 (CLI fixes): writeDryRun envelopes; fetch progress to stderr under --json/--agent; whois human-mode no-match exits 3 (JSON mode still prints []); examples added (spec examples for leads/properties).
  Printing Press issues: see phase-4.95 retro candidates.
  Gate: PASS
