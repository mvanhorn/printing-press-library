# TicketData CLI Shipcheck

## Result: SHIP
All 7 shipcheck legs PASS (verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard).

## Scorecard: 90/100 — Grade A
Strong: Output Modes 10, Auth 10, Terminal UX 10, README 10, Doctor 10, Agent Native 10,
MCP Remote Transport 10, Local Cache 10, Workflows 10, Insight 10, Path Validity 10, Sync Correctness 10.
Lower (all expected/minor):
- Cache Freshness 3/10 — intentional: store is watchlist-driven (no bulk list endpoint upstream), cache disabled per pre-gen guidance.
- MCP Token Efficiency 7, MCP Tool Design 7, Data Pipeline Integrity 7, Error Handling 8 — minor.

## Live sample probe: 7/7 novel commands (100%)

## Fixes applied (before/after)
- Live dogfood: 79/95 -> 101/101 (status pass). Fixes:
  - watch add failed: numeric ids parsed as json.Number (were string)
  - zones showed 0 prices: zone points use field `zone_get_in_price` (not `get_in_price`)
  - stats/zones: reject non-numeric event id (exit 2) for error_path
  - sync + watch add/list/rm: added Examples sections
  - sync empty watchlist: emit valid JSON envelope under --json
  - performers/venues get: pp:no-error-path-probe (API returns 200 found:false for unknown slugs)
  - framework learn commands (teach/teach-pattern/teach-playbook/playbook amend): pp:happy-args + flattened teach Example; teach --json honors explicit JSON over silent --quiet default
- Code review (subagent): drift flat-direction + zero-price target guard.

## Known gaps / retro candidates (machine-level)
- Live dogfood matrix synthesizes incomplete happy-path args for the generated learn-loop
  commands (missing required flags, literal `\` from multi-line Examples) and runs json_fidelity
  against quiet-by-default `teach`. Worked around per-CLI via pp:happy-args + a teach --json tweak
  (recorded in .printing-press-patches/). Systemic fix belongs in the generator/matrix.

## Verdict: ship
