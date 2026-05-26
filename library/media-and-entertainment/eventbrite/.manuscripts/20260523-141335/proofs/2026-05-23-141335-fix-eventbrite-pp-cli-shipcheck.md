# Eventbrite CLI Shipcheck Report

## Shipcheck umbrella: PASS (6/6 legs)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS (no manifest) |
| verify-skill | PASS |
| scorecard | PASS — 95/100 Grade A |

Scorecard dims: Path Validity 3/3, Auth Protocol 10/10 (Bearer match), Data Pipeline 10/10, Sync Correctness 10/10, Type Fidelity 3/5, Dead Code 5/5. Omitted: mcp_description_quality, mcp_token_efficiency, live_api_verification.

## Blockers found + fixed (2 shipcheck loops)
1. **scorecard FAIL** — spec referenced undefined `oauth2` scheme on 95 operations (I'd replaced the global scheme + definitions but not per-op `security`). Fixed: renamed all per-op `oauth2` refs → `privateToken`; refreshed embedded `spec.json`. Scorecard → 95/A.
2. **validate-narrative FAIL** — quickstart `sync --resources events,orders,attendees` failed under verify+dry-run because Eventbrite list endpoints are org-scoped (parent context required). Fixed: quickstart → bare `sync` (parent-scoped resources skip cleanly; flat resources dry-run). Re-rendered README/SKILL.

## Phase 4.7 Sync Param-Drop Gate: SKIP
No `traffic-analysis.json` (vendor-spec / converted-Blueprint CLI, no browser-sniff phase).

## Phase 4.8 / 4.9 docs review: findings fixed
- discount-performance description overstated ("discounted gross revenue, share of orders") → corrected to actual output (redemptions, type, redemption rate) in research.json + README + SKILL.
- fan-export "marketing opt-in flagged" not in output → corrected description (email/name/events/check-in).
- README troubleshoot tips used invalid `sync --resources events,...` → bare `sync`.
- cross-platform DICE recipe snippet was a duplicate sales-velocity call → changed to `fan-export --json` (the export step an agent joins).
- Verified clean: all 9 novel commands resolve; auth narrative accurate (EVENTBRITE_API_KEY Bearer, no OAuth-flow claims); no working public event-search command presented; brand "Eventbrite".

## Phase 4.85 Agentic Output Review: SKIP (no live data)
No token ⇒ Phase 5 live dogfood skipped; novel-command output is structurally `[]` (no synced rows). Empty-safety validated by reader unit tests + dry-run/JSON checks. Warnings-only phase; no actionable findings without data.

## Phase 4.95 Local code review: findings fixed
- MEDIUM: sales-velocity mixed windowed/all-time denominators (tickets_per_day inflated under --since). Fixed: denominator now event lifetime (created→now), independent of the --since order window; --since scopes orders_in_window/gross only.
- LOW: ebRound2 truncated negatives toward zero → switched to math.Round (symmetric).
- LOW (left as documented intent): readers `continue` on scan error — deliberate NULL-safe row handling, explained in eventbrite_store.go header; COALESCE makes scan errors near-impossible.
- PASS: SQL SELECT-only + parameterized/Go-side filters (no injection), empty-slice init (make([]T,0)), divide-by-zero guarded, no resource leaks, no panics.

## Verify pass-rate / scorecard
- verify: PASS (before/after stable)
- scorecard: 95/100 Grade A (after oauth2 spec fix; was failing-to-run before)

## Final ship recommendation: SHIP
All 6 legs PASS, scorecard 95/A, no known functional bugs in shipping-scope features. 9 novel commands resolve, dry-run/JSON safe, reader tests pass. Live dogfood deferred (no token).
