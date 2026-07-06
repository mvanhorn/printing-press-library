# Agilix Dawn CLI — Build Log

Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

## Built
- Foundation: generated client (raw-token Authorization auth), SQLite store, sync, offline search, doctor.
- Absorbed (generated endpoints): concept list/get, user list/me, organization list, purchase list,
  progress list, conversation list, config get.
- Transcendence (all hand-coded, all verified live):
  1. course tree <id>    — renders section→instruction→interaction hierarchy (verified on 34-section course)
  2. course stats <id>   — 34 sections / 392 instr / 1223 inter / 617 pts / 47.0h computed correctly
  3. course outline <id> --format md|csv — curriculum export
  4. catalog diff        — snapshot-based catalog drift (baseline + change detection)
  5. roster export --format csv|json — user roster export
  6. purchase reconcile  — purchase↔user local join

## Notes
- `search={JSON}` wrapper is mandatory; generated list commands default --search to {"limit":100}.
- progress/conversation are real endpoints but return little for admin tokens (learner-scoped).
- All commands read-only; no mutation shipped in v1.
- Shared parse helpers in internal/cli/dawn_structure.go; behavioral tests in dawn_structure_test.go.
