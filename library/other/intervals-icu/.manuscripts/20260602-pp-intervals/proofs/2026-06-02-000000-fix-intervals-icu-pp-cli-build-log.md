Manifest transcendence rows: 5 planned, 0 built. Phase 3 will not pass until all 5 ship.

(sync is spec-emits and already generator-built; the 5 hand-code rows are: form, curve compare, wellness trends, since, gear status.)

## Phase 3 complete (5/5 transcendence rows built)
- form: fitness/fatigue/form (CTL/ATL/TSB) from live activities (reads icu_ctl/icu_atl).
- curve compare: best power/pace/HR curves for two date ranges, delta at standard durations.
- wellness trends: Pearson correlation of HRV/restingHR/sleep vs daily training load.
- since: planned/completed/missed classification across events + activities.
- gear status: distance/time rollup vs reminder thresholds.
All read live API (no reimplementation). Real table-driven tests added (pearson, extractCurve, parseWindowDays, gearDue, dayOf).

## Machine fixes applied to printed CLI (retro candidates)
1. AuthHeader built base64("<key>:") (key as username). intervals.icu needs constant
   username "API_KEY": base64("API_KEY:<key>"). Fixed in config.go. SYSTEMIC: generator
   has no spec-driven constant Basic username (scheme description literally says
   "Username is API_KEY"); propose x-auth-basic-username extension.
2. defaultSyncResources only had pace-distances because every other resource is
   athlete-scoped (/athlete/{id}/...). Wired activity/wellness/events/gear/workouts with
   {id} resolved to INTERVALS_ICU_ATHLETE_ID (default "0"). SYSTEMIC: generator should
   support athlete-scoped + date-range (oldest/newest) syncable resources.
