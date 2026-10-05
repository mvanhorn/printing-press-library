# Phase 4.9 README/SKILL/AGENTS correctness audit (2026-10-05)

Auditor: one Opus general-purpose agent; it ran --help only, made no edits, sent no traffic. Result: 3 errors and 15 warnings, ALL FIXED (none carried as warnings).

## Code fixes found by the audit (with regression tests, mutation-proven)
- Warning 4 was a real bug, not only a doc gap: after a site refusal, postings/screen with --work-pattern or a description filter fell back to Oracle rows that carry neither field, so the result was silently empty (or, for --description-not-contains, unfiltered) with exit 0. readLive now refuses the fallback for --work-pattern and the description filters, as it already did for --team/--sub-team/--contract-type (TestUJCReadLiveRefusesFallbackForUnservableFilters).
- Warning 8: stats' local note now always carries the last sync date, plus the history start date during the first 30 days.

## Doc fixes
- Errors: README Output Formats used bare careers search (bare prints help) -> careers search --countries Germany; Quick Start doctor --dry-run said it checks config -> "Confirm the binary runs, without touching the network" (research.json quickstart[0]); troubleshoot for exit 7 rewritten (no envelope on exit 7, the 00:00 UTC latch, --data-source local) (research.json troubleshoots[0]).
- Warnings: fallback limits gain --work-pattern and the description filters (value_prop); troubleshoot for baseline_advanced false rewritten; exit code 1 listed in both tables and README gains the latch clause; sync lines explain that a fallback sync marks no closures and is not a complete sync; config headers apply only to careers and doctor; state contents listed accurately; learnings forget example gains --all; quickstart new notes that the first run takes the baseline; SKILL drops the false "sync progress events" claim, the --since feedback example and the off-domain sports examples; AGENTS no longer implies remote mutation, has no unsubstituted <command> placeholders, and defines auto per command.
- Checks after the fixes: verify-skill all PASS; validate-narrative OK (12); dogfood exit 0 (WARN: dead generated helpers only); go test ./... 14/14 ok.

## Retro candidates noted
- The generated import --help describes `import careers` as "create/upsert calls", but it POSTs to the read-only batch lookup (resourceWritePaths treats a mutation:false POST as writable); kept out of live legs.
- Template text in AGENTS/SKILL/README (persisted queries and jobs, credentials/auth sidecars, sports alias examples, a --since feedback example, the Discovery Signals block from noise hosts) needed per-CLI hand edits.
