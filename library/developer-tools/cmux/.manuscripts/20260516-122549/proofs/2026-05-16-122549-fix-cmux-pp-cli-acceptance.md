# cmux-pp-cli Phase 5 Acceptance Report

## Level: Quick (gate marker) + Full (deep)

## Quick (gate marker, written to phase5-acceptance.json)
- **6/6 mandatory tests passed**
- 3 tests skipped (no positional args / dry-run-only mutators)
- Auth context: `type: none` (cmux uses local socket auth handled by the cmux binary, not by this CLI)
- Gate: **PASS**

## Full (118 tests)
- **117 passed, 1 failed** (99.2%)
- The single failure: `workflow archive --json → invalid JSON`. **Generator-level quirk, not novel-feature code.** `workflow archive` is emitted by the Printing Press generator and streams JSONL events (`{"event":"sync_start","resource":"buffers"}` ...) which the dogfood matrix's `json_fidelity` check tries to parse as a single JSON document. Affects every printed CLI; not cmux-specific. Retro candidate.
- All auth checks: PASS
- All sync checks: PASS
- All flagship features (search, watch, alert, status awaiting/stuck/timeline/changes, workspaces card, snapshot): PASS
- All absorbed commands (workspaces/windows/panes/surfaces/status/logs/notifications/hooks/buffers/capabilities/doctor): PASS

## Fixes applied during the dogfood loop
1. **alert add help-only invocation:** When neither `--on` nor `--sink` is supplied (bare `alert add`), print help and exit 0 instead of erroring. Matches the verify-friendly RunE pattern in AGENTS.md.
2. **alert add Cobra `Example` field:** Removed shell-quoted apostrophes around the slack sink in the example so dogfood's matrix parser (which feeds the example back as args) gets a clean unquoted value.
3. **alert add/remove dry-run JSON envelope:** Under `--dry-run --json`, emit a structured envelope describing what would be written, rather than returning nil. Keeps the dogfood `json_fidelity` test green.
4. **panes sample graceful degrade:** When `cmux read-screen` returns `invalid_params: Surface is not a terminal` (e.g., the example surface happens to be a browser tab), return success with `skipped: true` instead of propagating the error.
5. **watch JSON drain envelope:** When `--source notifications --sink stdout --json` drains zero pending events in `--one-shot` mode, emit a `{"event":"drain_complete","emitted":0,...}` envelope so JSON-fidelity probes don't choke on empty output.

## Printing Press issues for retro
- `workflow archive --json` emits JSONL, not single JSON. Dogfood's `json_fidelity` check should either tolerate JSONL on generator-emitted streaming workflows or skip them. Affects every printed CLI; one of the 1/118 dogfood-full failures.
- Generator's `id` extractor doesn't know about ref-keyed resources (panes use `ref` not `id`, capabilities use `method`). Synced rows landed empty for these. Retro candidate to support `id_field` overrides in the spec.

## Gate: PASS
