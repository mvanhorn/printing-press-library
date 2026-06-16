# ts-pp-cli Shipcheck

Verdict: **ship**

## Legs (all PASS, 6/6)
verify, validate-narrative, dogfood, workflow-verify, verify-skill, scorecard.

## Scorecard: 93/100 — Grade A
Strong across output modes, auth, error handling, README, doctor, agent-native, MCP, local cache, vision, workflows.
Soft spots (non-blocking): MCP Token Efficiency 7/10, Cache Freshness 5/10, Type Fidelity 4/5.

## Behavioral verification (fixture DB, no live key)
- concentration --by obligor --limit 10%: correct consolidated shares (BARC 85.7%, HSBC 14.3%), redeemed excluded, obligor names joined, breach flags correct.
- book --group-by currency: total 3.5M, weighted-avg yield 4.143% (hand-verified), USD/EUR split, redeemed excluded.
- ladder --by week: weekend maturities (Sat/Sun) settlement-adjusted to Monday, maturity_value summed per bucket.
- auth login: dry-run previews; missing creds exit 2; OAuth2 client-credentials Basic-auth exchange against /oauth/token.
- subscribe / maturity-action (PUT): refuse without --yes (exit 2); --dry-run previews. Capital-write guard.

## Scope
Marquee 3 transcendence commands (ladder, concentration, book) implemented per user-approved scope change; 4 others (changed, reinvest, screen, wall) dropped from manifest. Full read+write API surface generated. 3 wrong-domain templated commands (load/orphans/stale) removed.

## Live smoke (Phase 5): SKIPPED — OAuth2 auth required, no credential provided. Verified against mock + fixture only.
