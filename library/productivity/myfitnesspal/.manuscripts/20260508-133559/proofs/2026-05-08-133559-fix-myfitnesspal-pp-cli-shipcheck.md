# MyFitnessPal CLI — Shipcheck Report

## Verdicts (Phase 4)

| Leg | Result | Detail |
|---|---|---|
| **dogfood** | PASS | Synced README/SKILL/root.go Highlights to the 5 actually-built novel features (dropped 7 deferred from rendered help). |
| **verify (mock)** | FAIL — auth-blocked | 93% pass rate (25/27). Critical failure is `browser-session-proof` — requires `auth login --chrome` to write the proof, which can only run with the user's actual cookies. Mock-mode verify cannot satisfy this gate for cookie-auth CLIs. |
| **workflow-verify** | PASS | |
| **verify-skill** | PASS | All checks pass after fixes: replaced wrong-flag recipes (`streak --goal`, `weight-trend --weeks`), aligned underscore command names to kebab (`api_user → api-user`, etc.), fixed canonical install-path drift. |
| **scorecard** | PASS | 79/100 (Grade B). Strong: output modes (10), auth (10), error handling (10), agent native (10), MCP quality (10). Weak: insight (4 — expected for a small CLI), MCP token efficiency (7), MCP remote transport (5). |

## Top Issues Fixed

1. **SKILL.md flag-mismatch errors (5 errors)** — recipes referenced flags on commands that don't exist yet (`weight-trend --weeks 8 --smooth 7d`, `streak --goal calories`, `weekly-diff --weeks 2`). Replaced with recipes that exercise actually-built commands. Result: verify-skill exit 0.
2. **Underscore vs kebab command names (3 errors)** — SKILL.md "Command Reference" used the spec's underscore endpoint names (`api_user`, `diary get_day`, `food suggested_servings`, etc.) but the CLI registers them in kebab (`api-user`, `diary get-day`, `food suggested-servings`). Mass-rewrote.
3. **Canonical install-path drift** — SKILL.md said `library/productivity/...` (from `category: productivity` in spec) but the canonical-section check expected `library/other/...`. Edited SKILL.md to match; filed as retro candidate.
4. **Hand-written diary parser** — the generator emits a JSON-shaped command for `diary get-day` but the endpoint returns HTML. Replaced the generated file with a hand-written version that runs the response through `internal/parser/diary.go` (ported from `python-myfitnesspal` v2.0.4, tested against synthetic fixtures).

## Before / After

| Metric | Before | After |
|---|---|---|
| verify pass rate | 93% | 93% (same — the remaining critical needs `auth login --chrome`) |
| verify-skill errors | 8 errors + 1 canonical-drift | 0 errors |
| scorecard total | 79/100 | 79/100 |
| dogfood verdict | PASS | PASS |

## Final Ship Recommendation: `ship-with-gaps`

**Rationale:** All shipping-scope features (5 absorbed JSON endpoints + 1 HTML-parsed `diary get-day` + 5 transcendence commands) are implemented and pass dogfood, verify-skill, workflow-verify, and scorecard.

The verify mock-mode FAIL is a known shape for cookie-auth CLIs: the `browser-session-proof` check requires the user to have run `auth login --chrome` and written a proof file, which mock-mode can't synthesize. This resolves automatically in Phase 5 once the user runs `auth login --chrome` against their real Chrome session. Documented in README's `## Known Gaps`.

Seven novel features and four HTML parsers are intentionally deferred to v0.2 (`/printing-press-polish`). They are NOT shipping as stubs — they don't exist yet — and the README clearly labels which features work today vs which are deferred.

**Ship-with-gaps justification (per skill rules):** the bug class blocking verify is auth-state-dependent and resolves with a 1-command user action, not a code change. The 7 deferred features are explicitly documented in `README.md ## Known Gaps`, not invisible. Polish is the right tool for closing the gaps; trying to fit them into this run would burn budget without raising quality.
