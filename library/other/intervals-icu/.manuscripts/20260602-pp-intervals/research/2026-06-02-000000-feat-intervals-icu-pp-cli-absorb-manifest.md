# intervals.icu CLI — Absorb Manifest

## Scope summary
- 115 OpenAPI endpoints across 13 tags become typed Cobra commands automatically (generator-emitted, Priority 1).
- Absorbed-tool surface: hhopke-mcp (58 tools), eddmann-mcp (48), mvilanova, q050cr CLI, py-intervalsicu, tp2intervals. Every feature any of them has is covered by the generated endpoint surface or a novel command below.
- 6 transcendence features hand-coded on top (Priority 2).

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List/get activities | all MCPs, q050cr | (generated endpoint) activity list / activity get | Offline after sync, --json/--select/--csv |
| 2 | Search activities by name/tag | hhopke, eddmann | (generated endpoint) activities search | Plus offline FTS via `search` |
| 3 | Multi-activity fetch | hhopke | (generated endpoint) activities (by ids) | Bounded, agent-native |
| 4 | Power/pace/HR curves | eddmann, mvilanova | (generated endpoint) activity power-curve / pace-curve / hr-curve | CSV + JSON, plus `curve compare` novel |
| 5 | Histograms / best efforts / streams / intervals | hhopke | (generated endpoint) activity histograms/best-efforts/streams/intervals | Full analytics surface |
| 6 | Wellness get/list/update/bulk | all | (generated endpoint) wellness get/list/update | Plus `wellness trends` novel |
| 7 | Events list/get/create/update/delete/bulk | all | (generated endpoint) events ... | Plus `since` calendar window |
| 8 | Apply plan / mark done / duplicate events | hhopke | (generated endpoint) events apply-plan / mark-done / duplicate | Mutations gated under verify |
| 9 | Workout library + folders/plans | hhopke, eddmann | (generated endpoint) workouts / folders ... | Full CRUD |
| 10 | Workout export (.zwo/.erg/.mrc/.fit) | eddmann | (generated endpoint) download-workout | Zwift/trainer export |
| 11 | Athlete profile / settings | all | (generated endpoint) athlete get/profile/settings | |
| 12 | Sport settings / zones / pace distances | hhopke | (generated endpoint) sport-settings ... | |
| 13 | Gear list/CRUD/reminders | eddmann (Gear Mgmt) | (generated endpoint) gear ... | Plus `gear status` rollup novel |
| 14 | Routes | — | (generated endpoint) routes ... | |
| 15 | Weather forecast/config | — | (generated endpoint) weather-forecast/config | |
| 16 | Chats/messages | mvilanova | (generated endpoint) chats ... | |
| 17 | Custom items (charts/fields) | mvilanova | (generated endpoint) custom-item ... | |
| 18 | CSV export of activities | q050cr | (generated endpoint) activities.csv | Plus --csv on every read |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Local training database | sync | spec-emits | Generator emits sync+store from syncable resources; no other intervals.icu tool persists locally | none |
| 2 | Fitness & form trend | form | hand-code | Requires a local load time series to compute CTL/ATL/TSB | Use to judge taper/overreaching. Do NOT use for a single activity's load; use 'activity get'. |
| 3 | Season curve comparison | curve compare | hand-code | Requires joining best-effort curves across two historical ranges held locally | Use for season-over-season change at a duration. |
| 4 | Wellness vs load correlation | wellness trends | hand-code | Requires joining wellness + activity series locally | Use to see if HRV/resting-HR track fatigue. |
| 5 | Calendar catch-up window | since | hand-code | Requires time-windowed aggregation across events + activities | Use for 'what happened / what's coming'. Do NOT use for a single date; use 'events list'. |
| 6 | Gear mileage rollup | gear status | hand-code | Requires aggregating gear usage + reminders locally | Use to flag service/replacement thresholds. |

## Hand-code commitment
- 5 hand-code transcendence commands: `form`, `curve compare`, `wellness trends`, `since`, `gear status` (~50-150 LoC each + root.go wiring).
- 1 spec-emits transcendence: `sync` (generator-owned).
- No stubs planned.

## Risks before approval
- Spec server URL is http:// — must override base_url to https://intervals.icu at generation.
- Athlete-scoped paths use `{id}`; intervals.icu accepts `0` for "self". Config needs an ATHLETE_ID (INTERVALS_ICU_ATHLETE_ID) with `0` default so commands work without manual ID lookup.
- Large activity payloads (streams, curves) — recipes must use --select to bound agent context.
