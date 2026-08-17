# Placeit CLI — Phase 5 Acceptance (Full Dogfood)

Level: Full Dogfood (live, public Algolia catalog — no credentials needed)
Tests: 70/70 passed. Gate: PASS.

## Matrix coverage
Every leaf subcommand: help (Examples present), happy-path, JSON-parse, output-mode fidelity, error paths.

## Fixes applied inline (7 -> 0 failures)
- bookmarks (exit 5 x2): replaced generated promoted command with a hand-built one that auto-resolves user_id from account and degrades gracefully (hint + [] + exit 0) when not signed in. CLI fix.
- industry-map / kit / watch run (error_path): annotated pp:no-error-path-probe — any string is a valid query/style/name, no invalid-argument case. CLI fix.
- watch list / watch remove (help): added Example fields. CLI fix.

## Printing Press issues for retro
- Generated promoted command for an int-typed required param emitted a UUID example (--user-id 550e8400-...) and required the flag with no auto-resolution; auth-required promoted GETs fail the live matrix happy-path without a session-aware graceful path.

## PII
No live response values quoted; account tested anonymously (returns "Visitor").
