# Shipcheck — priority-pp-cli

Umbrella verdict: **PASS (7/7 legs)** — verify PASS, validate-narrative PASS (10 commands, full examples),
dogfood PASS (novel_features_check planned=7 found=7), workflow-verify PASS, apify-audit PASS,
verify-skill PASS, scorecard PASS.

Scorecard: **92/100 — Grade A** (Path Validity 10/10, Auth Protocol 10/10, Sync Correctness 10/10,
Breadth/Vision/Workflows 10/10, Dead Code 5/5; Cache Freshness 5/10 is the deliberate opt-out for a
rate-metered API; Data Pipeline 7/10, Type Fidelity 4/5, Insight 7/10).

Live sample probe: 5/7. The two misses are environmental, not functional:
- `forms search ORGT` — the official demo tenant has no ORGT_ custom fields; the command correctly reports
  zero hits with an honest note. Examples updated to a universally-present term (WARHS).
- `reconcile` — probe subprocess ran without PRIORITY_API_USERNAME/PASSWORD env; command correctly exits 4
  (auth) with an actionable hint. With credentials it returns in-sync/drifted verdicts (verified manually).

Fixes applied during Phase 3/4 (before/after: first shipcheck run passed after these):
- id_field per resource (sync stored 0 rows → 1,200 rows clean)
- LookupFieldValue uppercase probe (typed columns empty → populated; aging/debtors went from empty to real)
- batch load --dry-run file-read ordering (validate-narrative failure → clean)
- text get HTML stripping via priorityx.StripHTML
- credentials_test Basic-pair patch (generated test template gap)

Behavioral correctness of every novel command sampled against the live sandbox — see build log.

Ship recommendation: **ship**
