# Shipcheck Report — sensortower-pp-cli

## Verdict: SHIP

## Shipcheck umbrella (final run)
7/7 legs PASS: verify, validate-narrative, dogfood, workflow-verify, apify-audit,
verify-skill, scorecard.

- Scorecard: 85/100 — Grade A
- verify: PASS
- verify-skill: exit 0 (SKILL matches source)
- validate-narrative --strict --full-examples: PASS
- workflow-verify: workflow-pass
- Live full dogfood: 133/133, 0 failures (session-injected; cookie-gated commands exercised)

## Scorecard dimension notes
- Auth Protocol 2/10 — cookie auth with 9/11 endpoints anonymous scores low on a rubric that
  rewards a single mandatory-auth scheme. This is an honest reflection of the surface, not a
  defect: the CLI works with zero credentials and correctly gates only unified/*. Left to
  Phase 5.5 polish to confirm nothing actionable remains.
- MCP: 12 tools (10 public, 2 auth-required) — readiness: partial (the 2 cookie-gated tools).

## Blockers found and fixed (across Phases 3–5)
1. `date` required on category_rankings (upstream 422) — spec marked optional. Fixed spec.
2. `find`/`categories` are leaf commands, not parents — fixed manifest + all narrative examples.
3. `sync --resources rankings` fails (only `categories` syncable) — rewrote quickstart/troubleshooting.
4. `find --select data.entities.*` matched no fields (find unwraps to the array) — fixed to `--select app_id,name`.
5. `watch digest` cross-ladder delta bug (rank in cat A vs cat B) — fixed + regression test (Phase 4.95).
6. GetSyncState NULL-scan silent re-sync — fixed, generated-tree patch recorded (Phase 4.95).
7. Full-dogfood fixture/behavior gaps (rankings date, idempotent rm, dry-run JSON, no-error-path) — fixed.

## Before/after
- verify pass rate: PASS throughout
- scorecard: 85/100 both shipcheck runs (stable)
- dogfood: 129/134 → 132/134 → 133/133 (0 failures)

## Known Gaps
None blocking. Documented honestly in README/SKILL: bucketed money (never precise), ~12-req
rate limit, unified/* needs a cookie, and this is the free dashboard surface (not the paid API).

## Final ship recommendation: ship
