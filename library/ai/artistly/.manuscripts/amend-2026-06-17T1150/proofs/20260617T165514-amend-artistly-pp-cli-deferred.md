---
date: 2026-06-17
target_cli: artistly-pp-cli
amend_run_id: amend-2026-06-17T1150
deferred_count: 1
---

## F3 — dogfood harness passes literal `<design-id>` placeholder to resource-fetching commands

- category: test-harness / generator
- classification: machine-level (not a CLI defect)
- rationale: The Phase 5 live dogfood matrix passes the literal string `<design-id>` to `designs download`, `edit upscale`, and `prompt extract`. These commands must hit a real resource and cannot dry-run, so they exit non-zero (6/89 gate failures: 3 commands x 2 kinds). All three were verified working with a real design id (exit 0, real output) during this run.
- evidence: phase5-acceptance.json failure_summary.commands = ["designs download","edit upscale","prompt extract"]; manual reproduction with real id 57786554 returned exit 0 for all three.
- reason-deferred: This is an upstream printing-press harness gap, independent of the artistly code; already documented in the prior patch `.printing-press-patches/artistly-refresh-example-ids.json`. Not fixable at the artistly CLI level.
- still_relevant: yes (gate cannot pass cleanly until the harness resolves a real fixture id for resource-fetching commands)
