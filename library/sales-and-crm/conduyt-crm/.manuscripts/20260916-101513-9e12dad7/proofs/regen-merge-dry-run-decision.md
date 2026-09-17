# Phase 20 promote path decision (2026-09-17 11:15 AM ET)

Dry run: regen-merge-dry-run-report.json (library run 20260711-202005, generator 4.28.0 → fresh tree, generator 4.32.1).
Verdicts: NOVEL 3, TEMPLATED-WITH-ADDITIONS 8, TEMPLATED-BODY-DRIFT 543, TEMPLATED-VALUE-DRIFT 208, TEMPLATED-CLEAN 255, NEW-TEMPLATE-EMISSION 175, PUBLISHED-ONLY-TEMPLATED 26; lost registrations with missing referents 13.

- The 3 NOVEL files (internal/cli/drips.go, drips_audit.go, drips_audit_test.go) are the July "Drip ledger audit" novel. The approved Phase 1.5 manifest (research/2026-09-16-102758-feat-conduyt-crm-pp-cli-absorb-manifest.md, "Reprint verdicts") DROPS it: "no weekly caller; server-side ledger + sms-delivery/sms-drip-conversion reports cover it". The other two July novels not carried as hand code (analytics, tail) are press 4.32 framework commands in the fresh tree. The four kept July novels (send-check, imports blame, imports watch, verify-line-type --estimate) are rebuilt in the fresh tree and dogfooded live (1757/1757).
- The 13 missing referents are constructor names the 4.28 generator emitted (newCallFlowsPublishCallFlowCmd, newReportsGetBdaCmd, newConduytAuthGetGoogleCmd, ...). The fresh 4.32 tree names the same call-flows routes newCallFlowsPostIdPublishCmd-style, and the remaining routes (reports/bda, dialer conference events, Slack OAuth, Microsoft mailbox callbacks, Google/Microsoft auth) are no longer in the live OpenAPI (572 paths) that this print was built from. Nothing the published wiring references is a feature the fresh tree lacks.
- Path B (regen-merge --apply) would preserve the dropped drips audit and halt on 759 review verdicts of generator drift.

Decision: Path A (from-scratch reprint, every kept prior novel rebuilt, drops documented). The July library remains recoverable from the public registry (run 20260711-202005) via /printing-press-import.
