# Acceptance Report: grubhub

Level: Full Dogfood (live, against api-gtm.grubhub.com)
Auth: anonymous bearer auto-minted (no credentials; read-only)

## Result
Tests: 58/58 passed (0 failed). Gate: PASS.

## Coverage
- doctor, geocode, near, menu, item, compare, dish, deals, pick — help, happy-path, JSON-fidelity, error-path.
- Raw restaurants search/get/menu-item — pass with pp:happy-args fixtures (restaurant 1414955, POINT(-73.9857 40.7484), item 278441811233).
- Auth login (credential-free mint) verified.

## Behavioral spot-checks (live, NYC test address)
- near --sort fee -> cheapest-delivery restaurants returned.
- compare --sort eta -> sorted board with fee/min/eta/rating/deals columns.
- deals -> 12 restaurants ranked by offer value (top "$20 off $50").
- pick --weight-deal 2 -> top pick with transparent score breakdown.
- menu 1414955 -> full menu with correct prices (build-your-own price-variation fallback applied).
- dish "bowl" -> actual bowls ranked above description-only matches (relevance fix verified).

## Fixes applied during Phase 4-5
- CLI fix: replaced generated `geo` command (broken array-response envelope extraction returned empty) with hand-written `geocode`.
- CLI fix: menu/dish price now falls back to minimum_price_variation for build-your-own items (was rendering $0.00).
- CLI fix: dish relevance — name matches rank above description-only matches.
- CLI fix: root PersistentPreRun mints the anonymous token for the raw `restaurants` subtree so it works without a separate `auth login`.
- CLI fix: pp:happy-args fixtures on raw restaurants commands so live dogfood probes with valid inputs.
- Code review: added explanatory Note to dish local-mode empty-mirror JSON; removed unused parse field.

## Printing Press issues (retro candidates)
- Promoted single-endpoint command for an ARRAY response (`geo geocode`) wrapped the body and extracted `results: []` despite a populated array; object-response endpoints wrap correctly.

## PII
No PII in this report. Test fixture is a public NYC street address; no account, email, or org used (anonymous auth).
