Acceptance Report: github-contents
  Level: Full Dogfood
  Tests: 99/99 passed (final binary; matrix auto-derived from the command tree, 66 additional cells skipped by annotation/None-applicable rules)
  Failures: none (final run)
  Fix history within Phase 5:
    - Run 1 (100 cells): 9 failures — 8 were matrix-fixture synthesis on generated learning-loop commands (Example line-continuation `\` scraped as argv; placeholders passed literally), 1 was search's error-path probe misclassifying valid empty results. Fixed with pp:happy-args fixtures (pre-validated against the live binary) + pp:no-error-path-probe on search + making `teach --json` always emit its envelope (generated-template divergence, filed for retro).
    - Run 2 (99 cells): 12 environmental failures — Little Snitch dropped the rebuilt binary's outbound (context deadline vs curl 200 from same shell). Resolved by operator approving the binary in Little Snitch (path-scoped rule; survived subsequent rebuilds).
    - Real-target acceptance (the run's user goal): fetch of mjwoon/AI-readings/books surfaced a genuine bug — the global --timeout default (1m) bound the whole download phase; 64/118 files landed before the deadline killed the rest. Fixed properly: download phase now runs unbounded-but-cancellable unless --timeout is explicitly set; streaming HTTP client uses ResponseHeaderTimeout(30s) with no whole-body cap (fetch, sync-dir, tarball, releases download). Unit-tested (TestDeadlineForTransfer) + behaviorally re-verified.
  Real-workflow proof (the reason this CLI exists):
    - plan mjwoon/AI-readings/books → 118 files, 1.92 GB, api_cost 2, no LFS, not truncated
    - fetch → 118/118 (64 skip-identical + 54 downloaded post-fix), 0 failures, structure preserved incl. paths with spaces
    - verify ~/Documents/AI-readings-books → ok:true, matched 118, changed/missing/extra all empty (git blob SHA, zero re-download)
  Printing Press issues for retro: 7 (see phase-4.95-findings.md retro list + dogfood matrix Example-scraper `\` bug + teach --quiet/--json envelope gating + happy-args bool-flag two-argv tokenization + generated client per-call timeout parity note)
  Gate: PASS
