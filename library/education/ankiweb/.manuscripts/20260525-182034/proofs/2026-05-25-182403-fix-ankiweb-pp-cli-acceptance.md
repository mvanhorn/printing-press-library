# AnkiWeb CLI — Acceptance Report

Level: Full (privacy-constrained: HttpOnly session cookie not extracted into artifacts; authenticated paths validated via the user's logged-in browser session instead).

## Binary-owned dogfood (printing-press dogfood --live --level quick)
Gate: PASS — matrix 4/4 passed. The search 429 throttle is correctly surfaced as a typed RateLimitError (exit 7), not a crash.

## Manual + decoder verification
- list-decks protobuf: decoded live (200, 1039 decks for "spanish"); field map validated (upvotes+downvotes == site "Ratings" column across 3 decks).
- item-info: live PASS (id 241428882 → "Spanish Top 5000 Vocabulary", description, review_count). Field map corrected by Phase 3 against live data.
- deck-list-info (authenticated): validated via logged-in browser — 200, real synced-deck data. Real wire shape is nested (top #1 → repeated #3 decks); DecodeDeckList was FIXED to walk the nesting recursively + unit-tested (TestDecodeDeckListNested).
- authenticated search: 200, NO throttle — confirms the 429 is an anonymous-only access policy; with ANKIWEB_COOKIES search is unthrottled.
- compare: live PASS (2 rows).
- shared download: honest stub (token gap) — --dry-run prints URL (exit 0); real run returns actionable token-gap message (no crash).
- decks (no cookie): actionable auth error pointing to ANKIWEB_COOKIES / auth login --chrome.
- go test ./internal/svc/... : PASS (protobuf + decode + nested-deck tests).

## Documented gaps (carried to README Known Gaps)
1. shared download requires a client-minted signed token (op=sdd) — stub.
2. Anonymous search is rate-limited by AnkiWeb; set ANKIWEB_COOKIES for unthrottled search.
3. drift: owned-shared-deck download data not exposed by available endpoints — honest empty result.

## Printing Press issues (for retro)
- v4.16.0 cookie-auth template imports github.com/mvanhorn/agentcookie@v0.14.0-beta.1 which is not publicly resolvable ("Repository not found") — blocks any cookie-auth CLI build; required manual decoupling.
- Generator emitted unregistered generic boilerplate (channel_workflow.go, data_source.go, deliver.go) unrelated to the spec domain.

Gate: PASS
