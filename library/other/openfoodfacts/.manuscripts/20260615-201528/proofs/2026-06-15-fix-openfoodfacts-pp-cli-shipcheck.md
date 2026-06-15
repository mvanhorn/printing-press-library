# Open Food Facts CLI — Shipcheck

## Verdict: ship

Shipcheck umbrella exit 0 — all 6 legs PASS.

| Leg | Result | Notes |
|-----|--------|-------|
| verify | PASS | runtime + dry-run across command tree |
| validate-narrative | PASS | 10 narrative commands resolved, full examples pass under VERIFY |
| dogfood | PASS | no dead flags/paths, wiring + novel-feature presence OK |
| workflow-verify | PASS | primary workflow |
| verify-skill | PASS | SKILL flags/commands/positional all resolve; canonical sections present |
| scorecard | PASS | 93/100 Grade A |

## Scorecard 93/100 (Grade A)
Most dimensions 10/10. Weak spots:
- `mcp_token_efficiency` 4/10 — candidate for Phase 5.5 polish (large endpoint-mirror surface).
- `cache_freshness` 5/10, `insight` 7/10 — minor.
- Breadth 9/10, Vision 10/10, Workflows 10/10.

## Fixes applied this phase
1. **Narrative command names corrected** — CLI uses `product` (not `lookup`); online search is `find` with `--categories-tags` (no free-text positional in OFF v2). Fixed all quickstart/recipe/troubleshoot examples in research.json + SKILL + README.
2. **Allergen example** — `lookup --check-allergens` (nonexistent) → `allergens check <code>`.
3. **Offline-rank recipe** — `sync --category` (nonexistent flag) → `sync --resources find` (verified: populates 1199-row `find` table; rank sorts correctly).
4. **SKILL/README recovery** — a scripted edit truncated both docs to empty; regenerated from corrected research.json into a temp dir and copied SKILL.md/README.md back (working novel code untouched). Re-verified clean.

## Behavioral correctness (live, no-auth)
All 8 novel features verified against the live API earlier this run:
- compare: Nutella 539/E vs Prince 465/D ✓
- diary add/today: 30g Nutella = 161.7 kcal, remaining 1838/2000 ✓
- allergens check: Nutella vs milk,gluten → HIT milk, exit 3 ✓
- recipe: per_serving == total/4 ✓
- swap: 10 alts, all NOVA≤3, all nutriscore better than Nutella's 30 ✓
- budget: 20 results, 0 over remaining budget ✓
- rank: 1199 synced rows, sorted by sugar desc ✓

## Known non-blocking
- Sample probe flags `budget "snacks"` exit 10 when no goal is set — this is **correct by design** (budget requires a daily goal); verified working with a goal present. Not a ship blocker.
- MCP labels 13 tools "auth-required" though OFF reads need no auth — cosmetic readiness label; endpoints work unauthenticated.

## Before/after
- verify pass: PASS throughout.
- scorecard: 93/100 (stable).
- Blocking leg failures: 2 (validate-narrative, verify-skill) → 0 after fixes.
