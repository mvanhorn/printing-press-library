# Novel Features Brainstorm — dreaming (audit trail)

## Customer model

**Marisol — the 600-hour climber (intermediate, pace-obsessed).** Following the roadmap toward L5 (600h) by a self-imposed travel deadline.
- Today: reads total hours, then mental/spreadsheet arithmetic to check pace.
- Weekly ritual: Sunday "am I on pace?" check; re-derives required min/day by hand.
- Frustration: app shows a number, not a plan; no "hit 600h by date → N min/day"; no unified L1–L7 ladder with speaking/reading advisories; Romance-halving is folklore applied by hand.

**Ken — the multi-source logger.** Most input is Netflix/podcasts/YouTube, not on-platform.
- Today: logs external time one entry at a time after the fact.
- Weekly ritual: reconcile a personal spreadsheet vs what he logged; weeks of backlog.
- Frustration: no bulk/CSV path; backlog entry is dozens of identical clicks, so cumulative hours under-count reality.

**Priya — the next-video hunter.** Wants exactly one right-level, unwatched video each sitting.
- Today: scrolls catalog, manually hides watched, eyeballs difficulty.
- Weekly ritual: hunt by accent (prepping for Argentina) and favorite guides at comprehension edge.
- Frustration: no cross-field search, no sort by 1–100 difficulty, no "given your hours, here's the next unwatched video at your level."

## Candidates (pre-cut)
(12 candidates: next, search, external-import, plan, roadmap, diet, sql, since/tail, doctor, gaps, guide-follow, catchup. Sources a/b/c/e. Kill-checks applied inline.)

## Survivors and kills

### Survivors
| # | Feature | Command | Why only we | Score | Persona |
|---|---------|---------|-------------|-------|---------|
| 1 | Roadmap-aware next-video picker | `dreaming next --level auto --limit 10` | join user(level)→videos−playlist, sort by 1–100 rating, offline | 9 | Priya |
| 2 | FTS5 offline catalog search | `dreaming search "cocina argentina" --unwatched` | FTS over title/guide/dialect/topic/series, offline | 8 | Priya/Diego |
| 3 | CSV bulk external-hours import | `dreaming external import backlog.csv` | CSV→batched POST /externalTime; app has no bulk path | 9 | Ken |
| 4 | Difficulty-progression / readiness | `dreaming diet --window 90d` | playlist×videos.difficulty×daily series trend | 7 | Marisol/Priya |
| 5 | Unified fluency-ladder roadmap | `dreaming roadmap` | full L1–L7 ladder + CI advisories + per-level ETAs | 6 | Marisol |
| 6 | Raw offline SQL over local store | `dreaming sql "..."` | agent-shaped queryable offline mirror | 5 | power/agents |

### Killed candidates
| Feature | Kill reason | Sibling |
|---------|-------------|---------|
| plan | required-daily-avg is absorbed #12; ETAs live in roadmap | roadmap |
| since/tail | sibling to absorbed rolling averages #15 | diet |
| doctor | infra every fragile-API CLI ships; not a domain differentiator | (base CLI) |
| gaps | sibling to absorbed streak #17 | roadmap/diet |
| guide-follow | sibling to absorbed catalog filter #9 + watched #11 | next/search |
| catchup | sibling to catalog list #9; mostly sync plumbing | next/search |
