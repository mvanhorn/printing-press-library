# Forkable CLI Shipcheck

## Result: PASS (7/7 legs)

| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | PASS |

## Scorecard: 83/100 — Grade A

Notable dims:
- Output Modes, Auth (config), Error Handling, Terminal UX, README, Doctor, Agent Native, Local Cache, Workflows, Insight: 10/10
- Breadth 9/10, Vision 8/10, Agent Workflow 9/10
- MCP Quality 7/10, MCP Token Efficiency 7/10, MCP Remote Transport 10/10
- Path Validity 10/10, Sync Correctness 10/10, Type Fidelity 4/5, Dead Code 5/5

Low dims (structural, not bugs):
- **auth_protocol 2/10** — cookie/session_handshake browser-session auth. The scorer favors verifiable API-key/OAuth flows; browser-session auth is the correct and only auth Forkable supports. Not degraded to chase the score.
- **cache_freshness 3/10** — intentionally no local cache/sync. Forkable's reads are POST GraphQL with no GET-list syncable path; per pre-generation cache guidance, cache is left disabled. Novel features are live-fetch.

## Fixes applied
1. Removed `sync`/`search`/`sql` references from research.json quickstart+troubleshoots — the generator (correctly) emits none of these because there are no syncable GET-list resources; all reads are live GraphQL POSTs. Regenerated to refresh README/SKILL. This cleared the validate-narrative MISSING and verify-skill unknown-command failures.

## Novel features (7/7 built, live GraphQL)
served-history, preference-drift, why-picked, spend-trend, allowance-burn, upcoming-digest, venue-rotation — all resolve as Cobra commands, dry-run cleanly, emit --json/--agent, typed exit codes.

## Auth / CSRF
- auth.type: cookie (session_handshake). Runtime: `auth login --chrome` imports Chrome session cookie.
- CSRF: fetched from /api/v2/csrf_token, injected as x-csrf-token on GraphQL POSTs via internal/client/forkable_csrf.go + a one-line pp:hand-edit in client.go request loop. Survived regen-merge.

## Ship recommendation: ship (pending live dogfood)
- Structural gates all green. Behavioral correctness against the live API is UNTESTED in this shell (no Forkable session). Phase 5 live dogfood requires the user's logged-in session to confirm the GraphQL reads and novel-feature joins return correct data.
