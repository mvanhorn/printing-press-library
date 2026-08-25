# amazon-jobs-pp-cli Polish

Verdict: ship (further_polish_recommended: no)

## Deltas
- Scorecard: 84 -> 84
- Verify: 100% -> 100% (PASS, 0 critical)
- Live matrix: 4/5 -> 5/5 exercised
- tools-audit: 1 pending -> 0 pending (1 accepted: learnings-list thin-short, generated)
- go vet: 0; gosec hand-authored: 0

## Real fix (hand-authored)
Output-provenance mislabel: find/get/new (pp:data-source live) emitted meta.source="local"
in --agent JSON because they routed through the shared emitResult default. Polish added
emitLiveResult() in amazonjobs_common.go and switched the three commands to it; they now
report source:"live" (matching the generated postings command). Verified live; non-agent
--json output byte-identical (no regression).

## Retro candidates (generator defects, NOT hand-edited per DO-NOT-EDIT rule)
1. HEADLINE: sync-hint subsystem disabled (const syncHintsEnabled=false, correct for this
   manual-sync CLI) but the generator still emits (a) a dead --max-age persistent flag,
   (b) an uncalled hasChangedLocalFlags helper, and (c) sync_hint_test.go with 5 tests that
   assume the subsystem is ENABLED -> `go test ./internal/cli` is RED on 5 tests. Zero runtime
   impact; durable fix is a generator change to suppress the flag/helper/test when disabled.
2. 19 gosec findings, all in generated files (0 in hand-authored code) -> not ship-blocking.
3. learnings-list thin-short (generated learn-loop template).
4. Low scorecard dims (Cache Freshness 3, MCP Token Efficiency 7, Vision 7, Data Pipeline 7,
   Sync 7, Type Fidelity 4/5) -- structural, pipeline-accepted (manual-sync, single-endpoint API).

## Publish heads-up
If a downstream publish CI runs `go test ./...`, it is red on the 5 generated sync_hint tests
until the generator is fixed. Runtime and shipcheck (verify PASS, dogfood 81/81, scorecard 84/A)
are unaffected.
