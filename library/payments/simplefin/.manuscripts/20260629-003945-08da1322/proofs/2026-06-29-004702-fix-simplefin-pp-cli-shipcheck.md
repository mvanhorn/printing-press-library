# SimpleFIN CLI Shipcheck

## Verdict: ship
- shipcheck umbrella: PASS (7/7 legs) — verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard all green.
- Scorecard: 85/100 Grade A. Sample Output Probe: 8/8 (100%).

## Notable scores
- Output Modes 10, Auth 10, Error Handling 8, Terminal UX 10, README 10, Doctor 10, Agent Native 10.
- Local Cache 10, Workflows 10, Insight 10, Agent Workflow 9.
- Cache Freshness 5 — INTENTIONAL: cache auto-refresh disabled because SimpleFIN has a hard ~24 req/day limit that disables tokens on abuse. Manual sync + quota-aware doctor instead.
- MCP Token Efficiency 7, MCP Tool Design 7, Breadth 7, Vision 7 — minor.

## Blockers found and fixed (dogfood, fix-before-ship)
1. `accounts` HTTP 400 — the generated live accounts command passed relative --start-date raw; SimpleFIN requires unix epoch. Replaced with hand-authored store-backed `accounts` (offline-first, --live fetches balances-only, no date param -> no 400).
2. `categorize --json` — "categorized N" status line polluted stdout JSON. Moved status lines to stderr.
3. `export --json` — global --json ignored. Now --json/--agent forces JSON format.

## Code review (Phase 4.95): 0 errors, 4 warnings (3 fixed, 1 accepted). See phase-4.95-findings.md.

## Known gap (routed to retro, NOT patched)
- internal/cliutil/credentials_test.go: 4 failing tests — generator test-template bug (substring-asserts a base64-encoded Basic-auth header). Credential loading is correct; live auth verified working. Generator-reserved package; route to /printing-press-retro.

## Live verification (public demo Access URL)
- doctor: base_url correctly derived to beta-bridge.simplefin.org from the credential; API reachable.
- sync: 3 accounts, 342 txns, 1 holding, 1 connection; 90-day cap surfaced from errlist.
- networth/cashflow/recurring/portfolio/categorize/export/since/reconcile/transactions/holdings/stale all return correct data.
- Full live dogfood: 69/69 pass, 0 failures.

## Before/after
- verify pass: PASS both runs.
- scorecard: 86 -> 85 (Grade A) after replacing promoted accounts with store-backed accounts.
