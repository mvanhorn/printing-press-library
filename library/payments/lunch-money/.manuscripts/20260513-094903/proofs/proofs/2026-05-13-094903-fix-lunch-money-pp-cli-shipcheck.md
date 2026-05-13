# Lunch Money Shipcheck Report

## Command outputs and scores

### Shipcheck umbrella (6/6 legs PASS)

| Leg | Result | Elapsed |
|-----|--------|---------|
| dogfood | PASS | 2.1s |
| verify | PASS (22/22 commands) | 2.2s |
| workflow-verify | PASS (no manifest needed) | 13ms |
| verify-skill | PASS | 197ms |
| validate-narrative | PASS (11/11 commands) | 192ms |
| scorecard | PASS | 127ms |

### Scorecard 92/100 — Grade A

| Dimension | Score |
|-----------|-------|
| Output Modes | 10/10 |
| Auth | 10/10 |
| Error Handling | 10/10 |
| Terminal UX | 9/10 |
| README | 8/10 |
| Doctor | 10/10 |
| Agent Native | 10/10 |
| MCP Quality | 8/10 |
| MCP Remote Transport | 10/10 |
| MCP Tool Design | 10/10 |
| MCP Surface Strategy | 10/10 |
| Local Cache | 10/10 |
| Cache Freshness | 5/10 |
| Breadth | 10/10 |
| Vision | 9/10 |
| Workflows | 10/10 |
| Insight | 9/10 |
| Agent Workflow | 9/10 |
| Path Validity | 10/10 |
| Auth Protocol | 10/10 |
| Data Pipeline Integrity | 7/10 |
| Sync Correctness | 10/10 |
| Type Fidelity | 4/5 |
| Dead Code | 5/5 |

Omitted from denominator: `mcp_description_quality`, `mcp_token_efficiency`, `live_api_verification` (auth-required deferred to Phase 5).

### Sample Output Probe (live command sample, post-fix)
- **8/8 passed (100%)**

## Top blockers found and fixed (1 iteration only)

| # | Finding | Fix | Source |
|---|---------|-----|--------|
| 1 | `transactions duplicates --window 3d` in narrative — flag is `--window int` | Changed all instances of `--window 3d` → `--window 3` in research.json + README.md + SKILL.md | scorecard Sample Output Probe (1/8 fail) |

### Pre-Phase-4 narrative fixes (caught by validate-narrative before shipcheck)

| Finding | Fix |
|---------|-----|
| `auth set-token` — requires positional `<token>` arg | Added `YOUR_TOKEN_HERE` placeholder |
| `summary --with-occurrences` — actual flag is `--include-occurrences` | Renamed (also `--with-totals`/`--with-rollover` would have failed; not in narrative) |
| `plaid-accounts trigger-fetch` — narrative said `accounts plaid sync` | Replaced |

## Before/after metrics

| Metric | After 1st shipcheck | After fix | Change |
|--------|---------------------|-----------|--------|
| Verify pass rate | 100% (22/22) | 100% (22/22) | unchanged |
| Sample Output Probe | 7/8 (88%) | 8/8 (100%) | +12% |
| Scorecard total | 92/100 | 92/100 | unchanged |
| dogfood verdict | PASS | PASS | unchanged |
| verify-skill | PASS | PASS | unchanged |
| validate-narrative | PASS (11/11) | PASS (11/11) | unchanged |

## Notes on residual dimensions (not blocking)

- **Cache Freshness 5/10** — sync only tracks per-resource cursor on updated_since for transactions; other resources are full-fetch. Spec doesn't expose cursors on categories/tags/recurring/accounts.
- **Data Pipeline Integrity 7/10** — dogfood notes "Search uses generic Search only or direct SQL." This is true — the FTS index is multi-resource; per-table search methods were not generated. Acceptable; agent users still get FTS.
- **README 8/10** / **Vision 9/10** / **Insight 9/10** / **Agent Workflow 9/10** — minor narrative polish opportunities for Phase 5.5.
- **Type Fidelity 4/5** — minor; one operation skipped a complex body field. Spec residue, not a runtime bug.

## Final ship recommendation: `ship`

All ship-threshold conditions met:
- shipcheck exits 0 ✓
- verify PASS (100% pass rate, 0 critical failures) ✓
- dogfood PASS with no spec/binary/example/wiring failures ✓
- workflow-verify PASS ✓
- verify-skill exits 0 ✓
- scorecard 92 (≥ 65 threshold) ✓
- Sample output probe 8/8 (no flagship feature returns wrong/empty output) ✓
- All 8 novel features built and registered ✓

Auth env-var aliases honored: `LUNCHMONEY_ACCESS_TOKEN` (canonical), `LUNCHMONEY_API_KEY`, `LUNCHMONEY_TOKEN`, `LUNCHMONEY_API_TOKEN`, `LUNCH_MONEY_API_KEY`.
