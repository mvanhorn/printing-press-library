# Phase 4.85 output review: japan-theme-parks-pp-cli (2026-10-09)

Procedure: same contract as the printing-press-output-review sub-skill, run inline so the live-check file stays in this run's private scratch dir (the sub-skill writes a fixed /tmp path that two sibling print agents could overwrite at the same time). `scorecard --live-check`: dates pass, typical pass, waits pass, snapshot skip (mutating example). Reviewer: general-purpose agent.

Result: WARN, 2 findings. Both fixed in-session.

| # | Check | Finding | Fix |
|---|---|---|---|
| 1 | Aggregation shows all sources | Under `--agent`, multi-row `dates` output lost `tickets_unknown_reason` for USJ/Fuji-Q rows (list compaction keeps only keys present in >=80% of rows). Same cause dropped `delta_vs_typical_minutes` from `waits` rows. | `dates`, `waits` and `typical` now pass explicit keep-fields to `printJSONFilteredKeep`, so reasons, sale_opens_at, typical blocks and deltas survive `--agent`. |
| 2 | Format | `typical` `filters.hour` was a string; zero-sample cells had `first_date`/`last_date` = "". | Filters now `hour_from`/`hour_to` integers (null when unset); first/last date are null when a cell has no open samples. |

No relevance, mojibake, URL or ordering issues found (waits sort longest first, closed rides last with null wait).
