# gmail-pp-cli Shipcheck Report

## Verdict: PASS (7/7 legs), scorecard 93/100 Grade A — recommendation: ship (pending Phase 5 live dogfood)

## Leg results
verify PASS (13.7s) | validate-narrative PASS | dogfood PASS | workflow-verify PASS | apify-audit PASS | verify-skill PASS | scorecard PASS
Sample output probe: 6/6 novel features passed (100%).

## Fixes applied during build/shipcheck loops
1. Generated-test failure: hasCompleteCredentialFields treated RefreshToken-only config as complete, so credentials-file AccessToken never merged (template bug — RETRO). Fixed: only AuthHeaderVal/AccessToken count as complete.
2. userId TemplateVar defaulted to "userId_placeholder" outside verify mode; Gmail needs "me" (RETRO: generator should honor spec default).
3. Novel command name collision: promoted users.watch claimed `watch`; hand streamer renamed to `stream` (manifest row 7 updated).
4. Dead helper hasChangedLocalFlags: adopted in all hand commands' help-only checks (replacing NFlag()).
5. schedule depth mismatch: research.json novel_features[0].command "schedule" -> "schedule send".
6. pull cursor robustness: HTTP 400 (invalid historyId) treated like 404 (expired) -> windowed resync.

## Known weak scorecard dims (accepted)
- Cache Freshness 5/10: intentionally disabled — Gmail is quota-metered; pre-read auto-refresh would burn quota units. Manual pull + doctor report is the designed shape.
- Data Pipeline Integrity 7/10 / Insight 6/10: framework sync is a runtime no-op for Gmail because messages.list returns ids only (RETRO: id-only list APIs need an enrichment hook). The hand-built `pull` command is the real population path and is advertised in README/SKILL/hints.

## Before/after
- verify: PASS from first umbrella run (auto-fix enabled, no criticals)
- scorecard: 93/100 first full run

## Phase 4.7 sync-param-drop gate: SKIPPED (vendor-spec run, no traffic-analysis.json)
