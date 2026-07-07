# Withings CLI — Novel Features Brainstorm (audit trail)

## Customer model

**Dana — QS body-recomposition tracker (cutting on a lifting program).** Weighs daily, exports CSV by hand, eyeballs fat-vs-lean in a spreadsheet. Weekly: Sunday 7-day weight/fat/lean review vs cut target. Frustration: water-weight noise makes a good cut look like a stall; app has no rolling average and no fat-down/muscle-up readout.

**Marcus — Masters endurance athlete syncing to a training stack.** ScanWatch, 5-6x/week, pushes to Strava/TrainerRoad. Weekly: Monday load check — workout HR-zone minutes vs morning resting HR/HRV/sleep. Frustration: training load and recovery markers live in separate silos; nothing joins "how hard I trained" to "how recovered I am."

**Priya — hypertensive patient sharing BP/AFib with a cardiologist.** Morning BPM cuff readings, ScanWatch AFib history, screenshots before appointments. Weekly: morning BP log + pre-visit scramble. Frustration: no clean dated export of BP + AFib events she can annotate ("started new med on the 3rd").

**Sam — agent/dashboard builder piping health data into an LLM.** Hand-rolled script dumps recent metrics into a prompt; re-implements pagination/mantissa/date math each time. Weekly: scheduled agent health digest. Frustration: every pull is bespoke ETL; no "since <time>, everything across all metrics as structured output" command.

## Survivors (transcendence rows)

| # | Feature | Command | Score | Buildability | Why only we can do this | Long Description |
|---|---------|---------|-------|--------------|------------------------|-----------------|
| 1 | Body-recomposition tracker | `recomp --since 90d` | 8/10 | hand-code | Local join across weight + fat-mass + lean-mass measures with rolling average + fat-down/muscle-held verdict; no single getmeas call produces it | Use for fat-vs-lean recomposition + de-noised weight trend. Do NOT use for raw single-day body-comp fields (hydration/bone/visceral) — that's `measure get --select`. |
| 2 | Training load vs recovery | `recovery --since 14d` | 8/10 | hand-code | Three-table join: workouts HR-zone load × measures (resting HR/HRV) × sleep_summaries; no endpoint spans it | Use to weigh training load vs recovery markers. For sleep-deficit use `sleep debt`; for arbitrary pairs use `correlate`. |
| 3 | Clinician BP/AFib report | `bp-report --since 90d --note DATE=TEXT` | 7/10 | hand-code | Joins BP measures × heart_records AFib classifications + a local annotations table into one dated clinician table | Use for a dated BP+AFib history with annotations for a clinician. Includes AFib inline; don't use `heart list` (raw dump) for the report. |
| 4 | Sleep-debt rolling window | `sleep debt --window 14d` | 6/10 | hand-code | Cumulative actual-vs-target sleep over a rolling window from sleep_summaries; per-night getsummary never returns it | Use for cumulative sleep-deficit accounting. For one night/stages use `sleep summary`; for load context use `recovery`. |
| 5 | Since-time health digest | `digest --since 24h` | 7/10 | hand-code | Reads every entity in the SQLite mirror since a timestamp → one structured multi-metric snapshot for an agent/clinician | Use for "what changed since X across all metrics" (pair with `--json`). For one metric use its command; for clinician BP/AFib use `bp-report`. |
| 6 | Multi-metric correlation | `correlate METRIC_A METRIC_B --since 90d` | 6/10 | hand-code | Pearson + best-lag correlation between any two daily series from the mirror; no endpoint offers it | Use for ad-hoc correlation of two arbitrary metrics. Prefer `recomp`/`recovery` for curated readouts; this is the general fallback. |

## Killed candidates
weight-trend (→ folded into recomp) · resting-hr-drift (→ folded into recovery) · weigh-in streaks (gamification) · sleep-stage breakdown (thin wrapper on sleep summary) · afib-timeline (→ folded into bp-report) · vo2max trend (single field, no join) · bodycomp export (overlaps measure get + --csv) · apnea/snoring flag (--select filter) · activity goal-streak (low ambition) · web-tier pull (out of scope for novel features; lives in P3 web tier).
