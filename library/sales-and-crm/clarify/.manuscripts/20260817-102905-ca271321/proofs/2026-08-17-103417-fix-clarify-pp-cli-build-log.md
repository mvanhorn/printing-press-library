Manifest transcendence rows: 7 planned, 0 built. Phase 3 will not pass until all 7 ship.

Update 2026-08-17: 7 planned, 7 built. All approved transcendence rows resolve as Cobra leaves
(prep, brief, followup, stale, dossier, velocity, dupes) and dogfood novel_features_check reports 7/7.

Built:
- AuthHeader api-key scheme prefix normalization (config.go body edit, regen-merge preserved)
- internal/store/clarify_extras.go: clarify_stage_history + clarify_transcripts side tables
- internal/cli/clarify_common.go: JSON:API mirror parsing, key-variant attr access, mirror guard
- 7 novel commands, all mirror-driven, drain-first SQLite, hintIfUnsynced/hintIfStale, dryRunOK
- 10 behavioral acceptance tests incl. negative/absence cases (all pass)

Deferred/notes:
- Attribute keys are schema-driven per workspace; commands use key-variant candidate lists and
  degrade with honest notes. Live verification pending workspace slug.
- Sync is one object type per run (--path-context object=X); documented in README/SKILL.
