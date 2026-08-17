# 3CX XAPI CLI — Shipcheck

## Final shipcheck (with live credentials)
| Leg | Result | Notes |
|---|---|---|
| verify | PASS | mock runtime + live data-pipeline (sync succeeds with creds) |
| validate-narrative | PASS | 10 narrative commands resolved; full examples pass |
| dogfood (structural) | PASS | 7/7 path validity; wiring, novel_features_check 8/8 |
| workflow-verify | workflow-pass | no workflow manifest (skipped) |
| apify-audit | pass | n/a |
| verify-skill | PASS | flag-names, flag-commands, positional-args, canonical-sections all pass |
| scorecard | PASS | **96/100 — Grade A** |

**Umbrella verdict: PASS (7/7 legs).**

## Scorecard 96/100 (Grade A)
Output Modes 10, Auth 10, Error Handling 10, Terminal UX 10, README 10, Doctor 10, Agent Native 10,
MCP Quality 8, MCP Remote Transport 10, MCP Tool Design 10, MCP Surface Strategy 10, Local Cache 10,
Cache Freshness 5 (intentionally disabled — snapshot/diff model wants stable local data), Breadth 10,
Vision 10, Workflows 10, Insight 10, Agent Workflow 9. Domain: Path 10, Auth Protocol 10, Data Pipeline 10,
Sync 10, Type Fidelity 4/5, Dead Code 5/5.

## Fixes applied (all hand-edits to generated files documented for retro)
1. OAuth token URL derives from base URL host (`internal/client/tokenurl_3cx.go`) — instance-portable; base-URL override redirects the token mint.
2. `doctor` recognizes client-credentials env vars as configured auth.
3. Store ID extraction: `Number` added to `genericIDFieldFallbacks` (store + cli) so DN-keyed entities cache.
4. Novel commands open the store read-write so an empty/unschema'd DB returns `[]` gracefully.
5. `$expand` hint + audit JSON note when ring-group/queue memberships aren't expanded.
6. `trace` rejects non-numeric extensions (usage error).

## Behavioral verification (live, production instance)
All flagship/core/novel features verified against your-3cx-fqdn.example.com — see acceptance.md.

## Known gaps (ship-with-gaps, user-accepted)
26 of 1471 live-dogfood checks fail, all on peripheral generated endpoints (file-download/export → CSV not JSON;
mode-gated status endpoints → HTTP 400; oversized responses → harness capture cap). None are flagship/core/novel.
Documented in README `## Known Gaps`. Generator-level fix (wire existing `guardLiveJSON=false` lever + raw output
for non-JSON `response_format` endpoints) filed for retro.

## Verdict: ship-with-gaps
All real, documented features work live; remaining failures are documented niche generated-endpoint limitations
requiring a generator refactor. User explicitly accepted ship-with-gaps.
