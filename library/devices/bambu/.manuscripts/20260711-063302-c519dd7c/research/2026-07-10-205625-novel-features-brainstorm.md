## Customer model

### Maya, the local-only maker supervising weekly prints

**Today (without this CLI):** Maya checks the Bambu app, Madcow status, or a Home Assistant entity while a print runs, then separately inspects AMS values and the current 3MF when filament use or ETA looks wrong. Those views answer what is happening now, but not whether this printer's ETA is habitually optimistic or how this execution compares with prior copies of the same job.

**Weekly ritual:** She starts several prints each week, checks progress and filament sufficiency, and reviews completed jobs before rerunning a model.

**Frustration:** Each run is an isolated snapshot, so repeatability, ETA accuracy, and stage-level delays require manual notes and comparison.

### Rafael, the small print-farm operator triaging several LAN printers

**Today (without this CLI):** Rafael opens one status view per printer and mentally combines progress, ETA, AMS inventory, errors, and upcoming completions. A fleet summary can list every machine, but it does not identify which printer needs attention first or whether the loaded filament can finish the current plates.

**Weekly ritual:** Every workday he scans the fleet, replenishes likely-to-run-out spools, responds to HMS or print failures, and sequences hands-on visits around expected completion times.

**Frustration:** Raw fleet state is abundant but prioritization is manual, especially when material risk, errors, and completion timing compete for attention.

### Priya, the home-automation maintainer protecting local integrations

**Today (without this CLI):** Priya consumes normalized MQTT state in automations and falls back to redacted raw reports when a firmware update changes behavior. She can inspect a current report, but determining which fields appeared, disappeared, changed type, or stopped updating requires saving reports and writing one-off diff scripts.

**Weekly ritual:** She reviews printer-derived automations, investigates missing or stale entities, and validates that firmware or model differences have not broken her local schemas.

**Frustration:** Sparse MQTT reports and firmware-specific field drift make ordinary text diffs misleading and integration regressions difficult to isolate.

## Candidates (pre-cut)

| # | Candidate | Command | Description | Persona served | Source | Inline verdict | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Calibrated completion forecast | `job eta` | Compare the printer's remaining-time estimates with locally observed completions for matching prior jobs and report a corrected ETA plus historical error band. | Maya | Cross-entity local queries | Keep: mechanical SQLite analysis, same local auth, weekly value, and verifiable after completed jobs exist. | none |
| 2 | Fleet attention queue | `fleet attention` | Rank printers needing human attention from live errors, completion proximity, AMS sufficiency, and stale telemetry. | Rafael | Persona-driven | Keep after reframing from a dashboard to a bounded one-command read-only ranking. | Use this command for cross-printer intervention prioritization. Do NOT use this command for quantified filament sufficiency; use `ams runway` instead. |
| 3 | Filament runway | `ams runway` | Join current-job 3MF material demand, progress, active-tray mapping, and AMS remaining estimates to report likely surplus or shortfall. | Rafael | DeepWiki | Keep: service-specific AMS hierarchy plus FTPS metadata creates leverage unavailable from either source alone. | Use this command for quantified filament sufficiency. Do NOT use this command for cross-printer intervention prioritization; use `fleet attention` instead. |
| 4 | Repeat-job comparison | `job repeats` | Group repeat executions by stable job/file/plate identity and compare duration, material estimate, pauses, and outcome. | Maya | Service-specific content patterns | Keep: local history turns otherwise isolated print executions into a repeatability comparison. | Use this command for comparing repeated executions of the same printable job. Do NOT use this command for the stage-by-stage chronology of one execution; use `job timeline` instead. |
| 5 | Firmware field drift | `printer field-diff` | Compare accumulated redacted report schemas across two observations or time windows, including added, removed, type-changed, and stale fields. | Priya | User briefing | Keep: sparse-report-aware structural comparison is agent-shaped output rather than a raw-report wrapper. | none |
| 6 | Failure correlation matrix | `history failure-correlations` | Join failed outcomes and HMS/error observations with printer, filament, plate, speed, firmware, and temperature context to produce grouped counts and rates. | Rafael | Cross-entity local queries | Keep: deterministic local aggregation directly supports recurring farm diagnosis without an LLM. | none |
| 7 | Print-stage timeline | `job timeline` | Reconstruct stage dwell times, pauses, layer progress, temperature recovery, and error events for one execution. | Maya | DeepWiki | Keep: component reducers and persisted sparse snapshots support a chronology no single live report contains. | Use this command for the stage-by-stage chronology of one execution. Do NOT use this command for comparing repeated executions of the same printable job; use `job repeats` instead. |
| 8 | Connectivity reliability report | `printer reliability` | Calculate observation gaps, reconnect periods, duplicate sequences, and pushall recovery frequency. | Priya | Cross-entity local queries | Cut: absence of reports cannot reliably distinguish printer downtime, CLI downtime, and network loss without a persistent collector. | none |
| 9 | Diagnostic incident bundle | `doctor bundle` | Write a redacted package containing diagnostics, recent reports, transitions, HMS events, and metadata references. | Priya | Persona-driven | Cut: useful only during incidents, so it fails the weekly-use test and writes a user-visible artifact. | none |
| 10 | Thermal stability baseline | `printer thermal-baseline` | Compare heat-up and temperature variance with historical jobs using the same printer and material. | Maya | Cross-entity local queries | Cut: interpreting harmless versus actionable variance needs unsupported model/material thresholds and manual domain judgment. | none |
| 11 | Job provenance manifest | `job provenance` | Combine task IDs, exact 3MF path and hash, plate metadata, firmware, profile, and spool identifiers into one reproducibility record. | Maya | Service-specific content patterns | Cut: most inputs are already exposed by current-job metadata, status, and AMS commands, while the combined record is only occasional. | none |
| 12 | Plate-to-spool mismatch detector | `job material-check` | Compare 3MF filament declarations with loaded AMS tray material and color. | Rafael | User briefing | Cut: profile-to-tray mapping can be incomplete or ambiguous in read-only standard LAN mode, making a definitive mismatch claim unreliable. | none |
| 13 | Estimated energy consumption | `analytics energy-estimate` | Infer electrical use from heater, fan, and duration observations. | Rafael | Persona-driven | Cut: the protocol exposes no power measurement, and adding a smart-plug source violates the external-service boundary. | none |
| 14 | Automation sensor export | `printer sensors` | Flatten current state into stable key-value sensor names for home-automation ingestion. | Priya | Persona-driven | Cut: this is a thin reshaping of the already structured `printer status` output and does not add local or cross-source leverage. | none |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | Persona served | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|---|
| 1 | Calibrated completion forecast | `job eta` | 9/10 | hand-code | Maya | This uses persisted `mc_remaining_time`/`remain_time` snapshots and terminal job transitions to compute a corrected completion estimate and historical error band with no external dependencies. | Madcow live reports expose remaining time and transitions; the brief requires durable transition history and identifies ETA as a top status field. | none |
| 2 | Fleet attention queue | `fleet attention` | 8/10 | hand-code | Rafael | This uses bounded live fleet snapshots plus local recent transitions and AMS/job metadata to compute a deterministic cross-printer attention ranking with no external dependencies. | The brief names print-farm operators and fleet monitoring; Madcow exposes errors, ETA, queue, AMS, and freshness fields; the official Local Server and Bambuddy research establish multi-printer operation. | Use this command for cross-printer intervention prioritization. Do NOT use this command for quantified filament sufficiency; use `ams runway` instead. |
| 3 | Filament runway | `ams runway` | 9/10 | hand-code | Rafael | This uses exact current-plate 3MF filament weights, print progress, and MQTT AMS tray remaining estimates to compute per-material surplus or shortfall with no external dependencies. | Madcow retrieves exact-current-job 3MF plate weight; DeepWiki documents the hierarchical AMS and active-tray mapping; live MQTT exposes AMS remaining estimates. | Use this command for quantified filament sufficiency. Do NOT use this command for cross-printer intervention prioritization; use `fleet attention` instead. |
| 4 | Repeat-job comparison | `job repeats` | 7/10 | hand-code | Maya | This uses locally persisted jobs, transitions, HMS observations, and 3MF asset metadata to compute repeat-run duration, pause, material, and outcome comparisons with no external dependencies. | The brief defines print jobs, transitions, HMS observations, and job assets as primary stored entities; Madcow supplies task/file/plate identity and terminal transitions. | Use this command for comparing repeated executions of the same printable job. Do NOT use this command for the stage-by-stage chronology of one execution; use `job timeline` instead. |
| 5 | Firmware field drift | `printer field-diff` | 8/10 | hand-code | Priya | This uses locally persisted redacted raw reports and merged snapshot timestamps to compute added, removed, type-changed, and stale MQTT fields with no external dependencies. | Madcow observed 99 top-level fields; the brief requires a raw-field escape hatch and notes firmware/model missing-value compatibility issues in BambuTools. | none |
| 6 | Failure correlation matrix | `history failure-correlations` | 9/10 | hand-code | Rafael | This uses local job outcomes joined to HMS/error, printer, filament, plate, speed, firmware, and temperature observations to compute grouped failure counts and rates with no external dependencies. | The brief explicitly requires SQL support for failure rate, material, temperature, and per-printer history; Madcow and Home Assistant expose terminal transitions plus HMS and print-error context. | none |
| 7 | Print-stage timeline | `job timeline` | 9/10 | hand-code | Maya | This uses persisted sparse MQTT snapshots, component state changes, and transition events to compute ordered stage dwell times, pauses, layer movement, temperature recovery, and errors with no external dependencies. | DeepWiki describes component reducers that compare previous/current state and emit meaningful events; the brief requires sparse-report merging, layer/stage metadata, and transition persistence. | Use this command for the stage-by-stage chronology of one execution. Do NOT use this command for comparing repeated executions of the same printable job; use `job repeats` instead. |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| Connectivity reliability report | Missing observations cannot distinguish printer, network, and collector downtime without a persistent collector, so its central metric is ambiguous. | Firmware field drift |
| Diagnostic incident bundle | It is incident-only rather than weekly and primarily repackages already available diagnostics and history into a file. | Fleet attention queue |
| Thermal stability baseline | The data is available, but actionable interpretation requires unsupported model/material thresholds and manual expertise, weakening verifiability. | Print-stage timeline |
| Job provenance manifest | It mostly combines fields already exposed by current-job metadata, status, and AMS, and the reproducibility ritual is only occasional. | Repeat-job comparison |
| Plate-to-spool mismatch detector | Standard-LAN read data does not guarantee an unambiguous profile-to-physical-tray mapping, so a definitive mismatch verdict could be false. | Filament runway |
| Estimated energy consumption | No measured power field exists, and an external smart-plug integration is outside the brief. | Failure correlation matrix |
| Automation sensor export | Flattening `printer status` into sensor keys is a thin output wrapper without new local, cross-source, or service-specific leverage. | Firmware field drift |
