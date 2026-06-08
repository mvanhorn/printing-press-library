# NinjaOne CLI Build Log

Manifest transcendence rows: 8 planned, 0 built. Phase 3 will not pass until all 8 ship.

Planned (all hand-code):
1. patch-gaps        — cross-org patch gap report (local join)
2. patch-sweep       — throttled cohort scan+apply (mutating, --dry-run)
3. alert-storms      — alert clustering group-by
4. patch-stuck       — KBs failing across syncs (needs history table)
5. alert-clear       — bulk alert reset by predicate (mutating, --dry-run)
6. stale-devices     — cross-org offline device sweep (optional reboot)
7. alert-flappers    — fire/resolve cycle counter (needs history table)
8. cf-hygiene        — required custom-field gap finder

## Phase 3 result
All 8 transcendence rows built (8 planned, 8 built). 306 endpoint commands generator-emitted.
- build/vet/test: green
- novel_features_check: planned 8 / found 8
- structural_issues: 0; dead_flags: 2 (deferred to shipcheck/polish)
- New: internal/store/ninjaone_history.go (history table for patch-stuck + alert-flappers); internal/cli/ninjaone_novel_common.go (shared shapes/paging/helpers)
- Mutating commands (patch-sweep, alert-clear, stale-devices --reboot) default to dry preview, require --apply, short-circuit under verify env.
