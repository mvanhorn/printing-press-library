# KDP Niche Finder CLI — Shipcheck

## Verdict: PASS (6/6 legs)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative (--strict --full-examples) | PASS |
| dogfood | PASS (novel_features_check 7/7) |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS — 87/100 Grade A |

Sample Output Probe: 7/7 (100%).

## Fixes applied this pass
1. verify-skill FAIL → PASS: reworded `auth login --chrome` from single-quote-wrapped prose (scanner extracted `--chrome'` and couldn't match the real flag) to backtick-wrapped in README/SKILL/research.json. `--chrome` is a registered flag (internal/cli/auth.go:192).
2. Sample probe "export: empty output" → 7/7: `export` now always emits the CSV header row, and a missing mirror yields a header-only CSV (+ stderr hint) instead of empty/`[]`.
3. Narrative flag fixes: research.json + README + SKILL updated to real flags (`--sort` not `--select`; `niches <type>` not `niches browse`; `refresh` not `sync --resources niches`; `export --csv`).

## Known non-blocking gaps
- auth_protocol 2/10: scorer caution for cookie auth (correctly modeled: cookie session + composed X-XSRF-TOKEN for writes). Candidate for Phase 5.5 polish.
- MCP 0 public / 7 auth-required (readiness: partial): correct — every endpoint needs the user's session.

## Outstanding (Phase 5 live dogfood)
The Inertia HTML data-page niche fetch, the 2 CSRF-bearing writes, and cookie auth need a logged-in session to validate end-to-end. Verified so far against mock/dry-run + fixture-DB behavioral tests for all 7 novel commands.

## Ship recommendation: ship (pending Phase 5 live validation against a logged-in session)
