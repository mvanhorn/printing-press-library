# seats-aero reprint — Phase 5.5 polish (2026-09-06)

Invoked via the Skill tool (forked, mid-pipeline, no `--standalone`), pointed at the working dir, with the run's XDG isolation + `SEATS_AERO_NO_AUTO_REFRESH=1`; the operator's store was never opened. No generate/promote/publish; nothing committed by polish (the orchestrator committed afterwards).

```
Polish pass:
  Scorecard:   93 → 93  (Grade A)
  Verify:      100% (43/43) → 100%
  gosec (hand-authored, post-triage): 8 → 0
  tools-audit: 2 pending → 0 pending (2 accepted, generated files)
  pii-audit:   0 → 0      verify-skill: 0 → 0      workflow-verify: pass
  go vet 0; go test ./... ok
  ship_recommendation: ship   further_polish_recommended: no
```

## Fixes applied (13 files, +109/−31)
1. gosec G104 ×8 in `recheck.go`, `new_since.go`, `direct_scan.go`, `calendar.go` — `rows.Close()` on error paths → `_ = rows.Close()`.
2. `promoted_awards.go` Short/Long: the generator's sentence splitter cut the spec description at "Seats.aero" → `"Search Seats."`; replaced with the real first sentence / full description (hand-patch on a generated file — retro candidate).
3. `promoted_trips.go` Short/Long: restored the clause the generator cut mid-sentence.
4. `SKILL.md` command list: same two descriptions fixed.
5. Output-review finding (verified): `direct-scan --sources bogus,united` silently dropped the unknown source → `warnUnknownSources` (stderr, store-grounded; exit code/JSON unchanged; silent on an unsynced store), wired into `direct-scan` and `recheck`, with `TestDirectScanWarnsOnUnknownSource`.
6. Output-review finding (verified as upstream data via one live `/destinations` call): `reach` shows FRA at 88 economy miles — the vendor's cross-program floor passed through unmodified; documented in `reach`'s Long with a pointer to `--confirm-live`.
7. tools-audit ledger: 2 `thin-short` findings in generated files accepted with DO-NOT-EDIT rationales.
8. Dogfood resync of README/SKILL Unique Features + Recipes, `root.go` Highlights, `which.go`, `.printing-press.json` from `research.json::novel_features_built` — partially re-rendered T7's hand edits in rendered sections (no information lost; durable home is `research.json`).

## Skipped (deliberate)
- 33 gosec findings in generator-emitted files (G304/G302/G204/G201/G202/G101/G117/G112) → retro candidates.
- Dead helper `handleBinaryResponseDelivery` → generator-owned (retro).
- Verify `execute:false` entries (`availability`, `awards`, `live`, `refresh`, `reach`, grouper commands) → required-input/live-only/parents; the live gate covers them.
- Scorecard 7/10 dims (Data Pipeline Integrity — generated `search.go`; MCP Token Efficiency / Tool Design — spec-level intents) → would need a regenerate; not chased for the score.

## Divergence check
Public clone at `~/Projects/printing-press-library/library/travel/seats-aero` (3.10.0 / release 2026.8.1) is divergent in the reprint-shaped way; all three public patches accounted for (two Go-floor sweeps superseded by `go 1.26.6`; `set-token-writes-api-key-field` generator-native via `cfg.SaveCredential`).

## Retro candidates surfaced by polish
(a) description→Short sentence split on "Seats.aero"; (b) Short length-cap truncation without ellipsis; (c) `internal/platform/migration.go` emitted without the generator header; (d) MCP HTTP server lacks `ReadHeaderTimeout` (G112); (e) thin `list` Short defaults; (f) dead `handleBinaryResponseDelivery`; (g) dogfood resync clobbers hand edits in rendered README/SKILL sections.

Quota spent by polish: 1 live `/destinations` call (+ the output-review sub-agent's `reach` samples).

## Post-polish
Source edits invalidate the phase-5 marker fingerprint → live dogfood re-run after committing polish (see the acceptance report addendum / `phase5-acceptance.json`).
