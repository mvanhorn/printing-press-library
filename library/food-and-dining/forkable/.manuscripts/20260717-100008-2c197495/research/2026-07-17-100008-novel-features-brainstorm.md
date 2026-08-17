# Forkable Novel-Features Brainstorm (audit trail)

## Customer model

**Persona 1 — Dana, the Office Lunch Admin (club manager)**
- Today: manages 2-3 meal clubs in the web SPA; to answer "what did we spend last quarter?" or "which venues do we use?" she clicks delivery-by-delivery and hand-tallies in a spreadsheet. SPA shows one delivery at a time, never aggregates.
- Weekly ritual: Monday reviews upcoming week's deliveries/allowances per team; Friday reconciles receipts against budget.
- Frustration: no spend trend, no venue-usage rollup, no roster/allowance export.

**Persona 2 — Sam, the Individual Member (recurring diner)**
- Today: eats team lunch 3-4 days/week; sets dietary prefs once and trusts auto-selection; occasionally browses to override.
- Weekly ritual: glances at "what's coming"; sometimes checks why a meal was auto-picked.
- Frustration: can't see whether served meals match stated preferences; opaque `mealGenerationScores`; no "what have I eaten" history.

**Persona 3 — Priya, the Finance/Ops Analyst**
- Today: owns lunch budget across offices; pulls receipts one delivery at a time.
- Weekly ritual: weekly spend check per club; monthly close needing per-venue/per-team breakdowns.
- Frustration: no CSV export, no cross-team comparison, no per-venue cost rollup.

## Survivors (transcendence rows)

| # | Feature | Command | Score | Buildability |
|---|---------|---------|-------|--------------|
| 1 | Served-meal history | `served-history --since 90d --json` | 8/10 | hand-code |
| 2 | Preference-vs-served drift | `preference-drift --since 60d` | 8/10 | hand-code |
| 3 | Auto-selection explainer | `why-picked --delivery <id>` | 7/10 | hand-code |
| 4 | Spend trend over time | `spend-trend --since 6mo --by month --csv` | 7/10 | hand-code |
| 5 | Allowance utilization | `allowance-burn --by club --csv` | 7/10 | hand-code |
| 6 | Week-ahead digest | `upcoming-digest --agent` | 7/10 | hand-code |
| 7 | Venue rotation | `venue-rotation --since 120d` | 6/10 | hand-code |

## Killed candidates
- roster-diff — sync stores latest state not history; needs retained snapshots. → allowance-burn
- rating-leaderboard — thin single-field sort; covered by search+sql. → served-history
- diet-coverage — speculative, niche; overlaps drift. → preference-drift
- team-spend-compare — merged into allowance-burn `--by club`. → allowance-burn
- meal-recap — near-dup of spec-emitted deliveries get. → upcoming-digest
- notifications-triage — thin filter wrapper. → upcoming-digest
- cadence — speculative; schema may not cleanly support attendance reconciliation. → served-history
