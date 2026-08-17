# Withings CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|--------------------|-------------|
| 1 | OAuth2 login + persisted auto-rotating refresh token | withings-sync | withings-pp-cli auth login | Single-use refresh rotation handled, local token store, 401→auto-refresh |
| 2 | Body measures (weight, fat ratio, fat/lean mass, hydration, bone, visceral, BP, SpO2, temp, ECG intervals) | all wrappers (getmeas) | (generated endpoint) measure get | Offline SQLite, --json/--select/--csv, type-code filters, mantissa→float scaling |
| 3 | Daily activity (steps, distance, calories, HR zones) | wrappers/MCP (getactivity) | (generated endpoint) activity get | Offline, date-range + lastupdate sync, HR-zone fields |
| 4 | Intraday activity (high-res series) | MCP (getintradayactivity) | (generated endpoint) activity intraday | Timestamp-keyed series typed correctly |
| 5 | Workouts (category names, HR zones, distance) | wrappers/MCP (getworkouts) | (generated endpoint) workouts list | Category code→name, HR-zone minutes |
| 6 | Sleep stage series (light/deep/REM/awake) | wrappers/MCP (sleep get) | (generated endpoint) sleep series | Phase states decoded |
| 7 | Per-night sleep summary (durations, HR, RR, snoring, AHI, score) | wrappers/MCP (getsummary) | (generated endpoint) sleep summary | Full summary field set, offline |
| 8 | Heart records (ECG signalid, AFib class, BP) | wrappers/MCP (heart list) | (generated endpoint) heart list | AFib enum decoded, BP + HR |
| 9 | Raw ECG signal export | MCP (heart get) | (generated endpoint) heart ecg | µV signal array + sampling freq + wear position |
| 10 | Devices (model, battery, last sync) | wrappers/MCP (getdevice) | (generated endpoint) devices | Battery + last-session, offline |
| 11 | Goals (steps/sleep/weight) | wrappers/MCP (getgoals) | (generated endpoint) goals | Mantissa-scaled weight goal |
| 12 | Webhook subscription mgmt | wrappers (notify) | (generated endpoint) notify subscribe/list/get/update/revoke | appli code→category names |
| 13 | Incremental sync (lastupdate cursor per resource) | withings-sync/aiowithings | withings-pp-cli sync | Per-resource cursor, --since/--full/--resources |
| 14 | CSV/JSON/select export everywhere | withings-sync | (behavior in withings-pp-cli measure get) global --json/--select/--csv/--compact | Agent-native on every command |
| 15 | Type-code + appli-code + category resolution, mantissa scaling | wrappers | (behavior in withings-pp-cli measure get) built-in decoders | Human names; real-value floats |
| 16 | Offline full-text + SQL over all metrics | (none — our differentiator) | (behavior in withings-pp-cli sync) search / sql | Composable local queries no SDK offers |

## Transcendence (only possible with our approach) — all hand-code, all approved as shipping scope

| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|------------------------|-----------------|
| 1 | Body-recomposition tracker | recomp | hand-code | 8/10 | Local join across weight+fat-mass+lean-mass measures, rolling avg, fat-down/muscle-held verdict | Use for fat-vs-lean recomposition + de-noised weight trend. Do NOT use for raw single-day body-comp fields; use `measure get --select`. |
| 2 | Training load vs recovery | recovery | hand-code | 8/10 | Three-table join: workouts HR-zone load × measures (resting HR/HRV) × sleep_summaries | Use to weigh training load vs recovery. For sleep deficit use `sleep debt`; for arbitrary pairs use `correlate`. |
| 3 | Clinician BP/AFib report | bp-report | hand-code | 7/10 | Joins BP measures × heart_records AFib + local annotations into a dated clinician table | Use for dated BP+AFib history with annotations. Includes AFib inline; don't use `heart list` for the report. |
| 4 | Sleep-debt rolling window | sleep debt | hand-code | 6/10 | Cumulative actual-vs-target sleep over a window from sleep_summaries | Use for cumulative sleep deficit. For one night/stages use `sleep summary`; for context use `recovery`. |
| 5 | Since-time health digest (agent-native) | digest | hand-code | 7/10 | Reads every entity in the mirror since a timestamp → one structured multi-metric snapshot | Use for "what changed since X across all metrics" (pair --json). For clinician BP/AFib use `bp-report`. |
| 6 | Multi-metric correlation | correlate | hand-code | 6/10 | Pearson + best-lag correlation between any two daily series from the mirror | Use for ad-hoc two-metric correlation. Prefer `recomp`/`recovery` for curated readouts. |

## Secondary tier (web extras — fragile, cookie-import; P3, NOT a stub)

| # | Feature | Source | Our Implementation | Notes |
|---|---------|--------|--------------------|-------|
| W1 | Web timeline feed / goals / targets / plans / feature flags | scalews.withings.com (cookie) | withings-pp-cli web timeline / web target / web plan ... | Cookie-import auth; requires a DevTools HAR to finalize the exact action/param contract. Clearly labeled fragile in help + SKILL. |

## Stubs
None. Every absorbed and transcendence row ships fully. The web tier (W1) is a real secondary source pending the HAR, not a stub — if the HAR isn't provided, web commands ship with honest "import your Chrome session_token cookie first" guidance and a documented `--har`-derived contract.
