## Customer model

### Maya — renter who delegates evening routines to an agent

**Today (without this CLI):** Maya opens the Home Assistant dashboard, hunts for the right scene, checks whether a window is open, and manually adjusts lights and media devices. She cannot safely ask an agent to “start movie night” because the agent may resolve a friendly name incorrectly or call several services without showing what will change.

**Weekly ritual:** Several evenings each week, Maya switches the apartment between work, dinner, movie, sleep, and away modes.

**Frustration:** Home Assistant exposes all the primitives, but there is no single previewable, verifiable command for executing a household mode from an imprecise human request.

### Theo — enthusiast who maintains automations every weekend

**Today (without this CLI):** Theo moves among automation traces, history charts, logbook entries, YAML, entity registries, and system logs. When an automation misfires, he manually aligns timestamps and entity IDs across those views.

**Weekly ritual:** Every weekend, Theo reviews failed routines, edits automations, and checks that renamed or replaced entities did not break references.

**Frustration:** Diagnosing one incident or determining an automation’s blast radius requires manually joining several Home Assistant data surfaces.

### Priya — household operator responsible for safe bulk changes

**Today (without this CLI):** Priya filters entities in the UI, writes down current values, performs repetitive edits, and checks rooms individually afterward. She avoids delegating reorganizations because ambiguous names and partial failures are hard to review.

**Weekly ritual:** Priya checks doors, windows, lights, batteries, and unavailable devices, then organizes or repairs anything that drifted.

**Frustration:** She lacks concise exception reports and a deterministic plan/apply/verify workflow that rejects ambiguity.

### Sam — self-hoster keeping the installation healthy

**Today (without this CLI):** Sam separately checks integrations, system health, updates, backups, unavailable entities, battery sensors, and energy charts. A failing integration may appear as dozens of entity problems without an obvious common cause.

**Weekly ritual:** Sam performs a weekend health review and looks for reliability, storage, radio, and energy problems before they become household complaints.

**Frustration:** Home Assistant reports individual symptoms well, but does not group them into actionable device, area, or integration-level hotspots.

## Candidates (pre-cut)

| # | Candidate | Command | Description | Persona | Source | Long Description | Inline verdict |
|---|-----------|---------|-------------|---------|--------|------------------|----------------|
| 1 | Verified household mode | `mode run <scene-or-script>` | Resolves a friendly mode deterministically, previews affected entities, executes it, and verifies resulting states over WebSocket. | Maya, Priya | persona, service pattern, user vision, codebase | none | Keep: more than scene invocation or generic bulk control. |
| 2 | Household exception check | `house check` | Reports open openings, unlocked locks, lights left on, unusual climate states, low batteries, and unavailable safety devices. | Maya, Priya | persona, service pattern, local joins, user vision | Redirect immediate exceptions away from `health hotspots`. | Keep: mechanical domain/device-class classification without NLP. |
| 3 | Routine incident explanation | `routine why <automation-or-script>` | Correlates one trace with surrounding history, logbook entries, state transitions, and errors. | Theo | persona, local joins, codebase | Redirect broad summaries to `house recap`. | Keep: bounded, verifiable time-window join. |
| 4 | Routine reference lint | `routine lint` | Finds missing entities, unavailable services, ambiguous names, disabled dependencies, and stale routine references. | Theo, Priya | persona, local joins, codebase | Redirect valid dependency mapping to `routine impact`. | Keep: directly addresses incumbent lint and missing-entity pain. |
| 5 | Routine impact graph | `routine impact <entity-or-routine>` | Shows which automations, scripts, scenes, helpers, and service targets depend on a target before a change. | Theo, Priya | persona, local joins, codebase | Redirect broken-reference detection to `routine lint`. | Keep: reverse-reference join unavailable from one endpoint. |
| 6 | Mechanical household recap | `house recap --since <duration>` | Factual report of mode usage, presence transitions, openings, automation failures, unavailable time, and state counts. | Maya, Sam | service pattern, local joins, user vision | Redirect one failed routine to `routine why`. | Keep: local SQLite aggregation, not LLM summarization. |
| 7 | Health hotspot grouping | `health hotspots` | Groups stale, unavailable, low-battery, and repeatedly disconnecting entities by device, area, integration, and radio. | Priya, Sam | persona, local joins, codebase | Redirect immediate safety/comfort exceptions to `house check`. | Keep: groups symptoms into evidence-backed operational causes. |
| 8 | Standby energy finder | `energy standby` | Finds persistent power draw during explicit inactive periods derived from related household states. | Sam | service pattern, local joins, user vision | none | Keep: mechanical time-series correlation with visible thresholds. |
| 9 | Scene delta | `scene diff <scene>` | Compares current states with expected scene changes. | Maya | service pattern, user vision | none | Reframe into the `mode run` preview. |
| 10 | Restorable checkpoint | `checkpoint create/restore` | Captures selected states and attempts later restoration. | Priya, Theo | persona, user vision | none | Cut: domain restoration semantics vary and cannot be promised honestly. |
| 11 | Playback ambience binding | `ambience bind` | Creates an automation that dims lights when media starts. | Maya | service pattern, user vision | none | Cut: one-time configuration that overlaps automation CRUD. |
| 12 | Guest-ready check | `guest ready` | Checks guest-area locks, climate, lighting, and connectivity. | Maya, Priya | persona, service pattern | none | Cut: preset over `house check` and `mode run`. |
| 13 | Naming cleanup planner | `registry naming-plan` | Suggests bulk renames from area/device/integration patterns. | Priya | persona, local joins | none | Cut: subjective policy/NLP; deterministic checks belong in lint. |
| 14 | Room pulse | `room pulse <area>` | Shows occupancy, climate, lighting, media, and openings for one room. | Maya | service pattern, local joins | none | Cut: narrow projection of overview/search and `house check`. |
| 15 | Continuous anomaly watcher | `house watch-anomalies` | Runs continuously and detects unusual state patterns. | Sam | local joins, user vision | none | Cut: persistent process and undefined anomaly model violate scope/verifiability. |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona Served | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|----------------|--------------|--------------|----------|------------------|
| 1 | Verified household mode | `mode run <scene-or-script>` | 9/10 | Maya, Priya | hand-code | Uses scene/script configuration, registries, current states, service calls, and WebSocket subscriptions for preview and verified outcomes. | User Vision, Top Workflow 1, `ha-mcp`, official API. | none |
| 2 | Household exception check | `house check` | 9/10 | Maya, Priya | hand-code | Uses states plus entity/device/area/label metadata for deterministic exceptions. | User Vision, Top Workflow 2, both incumbents. | Use this command for immediate household safety and comfort exceptions. Do NOT use this command for maintenance root-cause grouping; use `health hotspots` instead. |
| 3 | Routine incident explanation | `routine why <automation-or-script>` | 9/10 | Theo | hand-code | Joins traces, history, logbook, state transitions, and errors into a bounded timeline. | Top Workflow 3, `ha-mcp`, official REST. | Use this command to investigate one automation or script execution. Do NOT use this command for a broad historical household summary; use `house recap` instead. |
| 4 | Routine reference lint | `routine lint` | 8/10 | Theo, Priya | hand-code | Cross-validates routine configurations against entity/device/service/integration registries. | Top Workflow 5, `hass-cli` issues #96/#341/#468, WebSocket registries. | Use this command to find invalid or risky routine references. Do NOT use this command to map valid downstream dependencies; use `routine impact` instead. |
| 5 | Routine impact graph | `routine impact <entity-or-routine>` | 8/10 | Theo, Priya | hand-code | Builds forward/reverse dependency edges from synced config and registry IDs using drain-first SQLite queries. | Top Workflow 5, brief data model, `hass-cli` issue #407. | Use this command to map valid dependencies and change impact. Do NOT use this command to detect broken references; use `routine lint` instead. |
| 6 | Mechanical household recap | `house recap --since <duration>` | 7/10 | Maya, Sam | hand-code | Aggregates local history, logbook, traces, states, and registries into factual counts and timelines. | Data Layer, User Vision, official API and `ha-mcp`. | Use this command for an aggregate historical household report. Do NOT use this command to diagnose one failed routine; use `routine why` instead. |
| 7 | Health hotspot grouping | `health hotspots` | 8/10 | Priya, Sam | hand-code | Groups state symptoms by device, integration, area, radio, and historical recurrence. | Top Workflow 4, `ha-mcp` diagnostics, relational data model. | Use this command to identify maintenance root causes across devices and integrations. Do NOT use this command for immediate household safety and comfort exceptions; use `house check` instead. |
| 8 | Standby energy finder | `energy standby` | 7/10 | Sam | hand-code | Correlates energy/power histories with occupancy and operational inactivity windows. | `ha-mcp` energy, brief data model, personal-use vision. | none |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| Scene delta | Useful preview behavior is part of the more complete preview/apply/verify mode workflow. | Verified household mode |
| Restorable checkpoint | Generic restoration semantics would overpromise reversibility. | Verified household mode |
| Playback ambience binding | One-time configuration and mostly automation creation. | Verified household mode |
| Guest-ready check | Branded preset without a distinct join or data model. | Household exception check |
| Naming cleanup planner | Subjective suggestions need policy/NLP; deterministic checks belong in lint. | Routine reference lint |
| Room pulse | Narrow current-state projection without distinct leverage. | Household exception check |
| Continuous anomaly watcher | Persistent process and unverifiable anomaly definition exceed scope. | Health hotspot grouping |
