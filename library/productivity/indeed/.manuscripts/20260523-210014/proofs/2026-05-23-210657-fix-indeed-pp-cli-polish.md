# Indeed CLI — Polish Pass

Scorecard 82 → 84. Verify 100%, Dogfood PASS, go vet 0, tools-audit 0 pending, PII 0.
ship_recommendation: ship. further_polish_recommended: no (insight is a structural domain ceiling).

Fixes:
- Removed dead `--allow-partial-failure` flag + 3 dead partial-failure helpers (Dead Code 3/5 → 5/5).
- `search --min-salary` now filters on the annualized salary floor (SalaryMin), not the range max.
- Multi-location fan-out emits a stderr note when a location leg returns zero jobs (was silent).
- Enriched `saved list` description.
