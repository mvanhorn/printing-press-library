# Sumble CLI — Polish (Phase 5.5)

## Decision: autonomous polish skipped to protect credits
The printing-press-polish skill runs `cli-printing-press scorecard --live-check`
unconditionally as part of its diagnostic loop. On Sumble (usage-based billing),
live sampling would invoke novel features against the real API — notably stack-diff,
which falls back to organizations/enrich (5 credits per technology found, easily
hundreds of credits for a large company). The user explicitly required the CLI to
minimize credit spend and approved only a small frugal live-test budget (11 credits
spent in Phase 5). Running polish's live sampling would violate that constraint, so
the autonomous polish pass was skipped deliberately.

## State at skip
- Shipcheck: PASS (5/5 legs). Scorecard 86/100, Grade A.
- Weak dims are mostly structural / generator-bound and not cheaply addressable:
  - MCP Tool Design 5/10: nested request bodies emit as JSON-string flags
    (--filters/--organization) — a generator limitation for nested objects, not a
    printed-CLI fix (retro candidate).
  - Vision 7, Insight 6, Cache Freshness 5, README 8: minor; would need more features
    or live re-sampling to move.
- verify-skill PASS, validate-narrative PASS, go vet clean, dead code 5/5.

## ship_recommendation: ship
No hold trigger. The CLI is Grade A and behaviorally verified live for its flagship
credit-economy features. Polish can be re-run later with explicit credit consent
(`/printing-press-polish sumble`) if the user wants to chase the remaining points.
