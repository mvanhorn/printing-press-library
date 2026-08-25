# Bambu Absorb Manifest

## Source Tools
- Madcow Bambu core (`twidtwid/madcowbot`)
- Bambu Lab Home Assistant integration (`greghesp/ha-bambulab`)
- Bambuddy (`maziggy/bambuddy`)
- BambuTools Python API (`BambuTools/bambulabs_api`)
- Go `bambu-cli` (`tobiasbischoff/bambu-cli`)
- Bambu Printer MCP (`DMontgomery40/bambu-printer-mcp`)
- Bambu MCP (`schwarztim/bambu-mcp`)
- `bambu-node` (`THE-SIMPLE-MARK/bambu-node`)
- `@versatly/bambu`, `mcp-server-for-bambu`, `beambam`, OpenBambuAPI, and Bambu Lab's official local-integration documentation

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | SSDP printer discovery | Madcow core | `bambu-pp-cli discover` | Serial filtering, private-IP validation, bounded probing, JSON/MCP |
| 2 | Multi-printer profiles and config precedence | Go CLI, Versatly CLI | `bambu-pp-cli profile` | Secret-safe profiles, env override, no credential flags or output |
| 3 | Protocol and identity diagnostics | Madcow core, Go CLI doctor | `bambu-pp-cli doctor` | Bambu CA and serial-CN verification plus MQTT/FTPS freshness checks |
| 4 | Fresh normalized printer status | HA, SDKs, MCPs | `bambu-pp-cli printer status` | `pushall`, sparse-report merge, complete typed output |
| 5 | Complete raw MQTT report | Madcow live inventory | `bambu-pp-cli printer raw` | Redacted escape hatch that preserves unknown firmware fields |
| 6 | Provider-neutral print lifecycle payloads | Madcow core and user workflow | `bambu-pp-cli events watch` | NDJSON `print.started` with job, weight, ETA, and preview attachment reference; terminal event with actual timing/outcome |
| 7 | Live watch and transition detection | Go CLI watch, bambu-node events | `bambu-pp-cli printer watch` | started/paused/resumed/finished/failed/canceled, restart-safe baseline |
| 8 | Temperature and target status | HA, Go CLI, SDKs | `bambu-pp-cli printer temperatures` | Bed, chamber, active/dual nozzles, AMS, current and target |
| 9 | Fan and airflow status | HA, Go CLI, SDKs | `bambu-pp-cli printer fans` | Part, aux, secondary aux, chamber, heatbreak, airduct |
| 10 | Lights, speed, network, door, SD, firmware, nozzle, tool and hotend-rack capabilities | HA entity model | `bambu-pp-cli printer capabilities` | Model/firmware-aware shape without fake unsupported values |
| 11 | AMS unit and tray inventory | HA, MCPs, SDKs | `bambu-pp-cli ams status` | Units, trays, material, color, RFID/spool identity, humidity, temperature, remaining |
| 12 | Active filament and external spool | DeepWiki HA model | `bambu-pp-cli ams active` | Correct `tray_now` 0-15, 254, and 255 semantics |
| 13 | AMS drying and filament-backup status | HA, Bambuddy | `bambu-pp-cli ams services` | Read-only heated-AMS state and backup-pair visibility |
| 14 | HMS and print errors | HA, official Local Server | `bambu-pp-cli printer health` | Severity, module, code, description, wiki URL, history |
| 15 | Current print job detail | HA, Madcow, MCPs | `bambu-pp-cli job current` | IDs, plate, progress, ETA, layers, stages, speed, file, timing |
| 16 | Printable and skipped objects | HA, DMontgomery MCP | `bambu-pp-cli job objects` | Stable object IDs/names and skip state from current 3MF/report |
| 17 | Exact-current-job 3MF metadata | Madcow core, HA | `bambu-pp-cli job metadata` | Exact candidate paths, bounded ZIP/XML, weight, plate and object metadata |
| 18 | Current plate thumbnail | Madcow core, HA image | `bambu-pp-cli job thumbnail` | Validated bounded image written only to explicit output path |
| 19 | Queue, upload, upgrade, camera and timelapse service flags | Madcow live fields | `bambu-pp-cli printer services` | Typed operational summary with raw fallback |
| 20 | Printer file listing and download | Go CLI, MCPs | `bambu-pp-cli files list` and `bambu-pp-cli files download` | Implicit FTPS TLS session reuse, safe paths, bounded downloads |
| 21 | Multi-printer fleet summary and filtering | Bambuddy, official Local Server | `bambu-pp-cli fleet status` | Parallel bounded read polling by model/location/status |
| 22 | Print history and statistics | HA, Bambuddy, maintenance MCP | `bambu-pp-cli history` | Local LAN lifecycle history, outcomes, durations and filters |
| 23 | Maintenance ledger, due state and forecast | maintenance MCP, Bambuddy | `bambu-pp-cli maintenance` | Local usage/event-derived thresholds with explicit completion logging |
| 24 | Offline sync, search, SQL and analytics | Printing Press framework | `bambu-pp-cli sync`, `search`, `sql`, and `analytics` | SQLite persistence and composable agent analysis |
| 25 | JSON, agent, select, compact, CSV and MCP parity | Go CLI, MCP ecosystem | `(behavior in bambu-pp-cli printer status) agent-native output and automatic MCP mirror` | Stable machine output across every read command |
| 26 | Camera snapshot and live image | HA, MCPs, Go CLI | `(stub - standard LAN authorization and model support are unverified) bambu-pp-cli camera snapshot` | Honest capability refusal; never disables TLS verification |
| 27 | Pause, resume and cancel print | HA, CLIs, MCPs, SDKs | `(stub - restricted without Developer Mode or an authorized Bambu path) bambu-pp-cli job pause|resume|cancel` | Never pretends a restricted command succeeded |
| 28 | Start G-code or 3MF print | CLIs, MCPs, Bambuddy | `(stub - restricted without Developer Mode or Bambu Connect) bambu-pp-cli job start` | No authorization bypass |
| 29 | Light, speed, fan, temperature, chamber, airduct and buzzer controls | HA, CLIs, MCPs | `(stub - restricted without Developer Mode or an authorized Bambu path) bambu-pp-cli printer set` | Capability-aware refusal |
| 30 | Calibration, home, movement, reboot and arbitrary G-code | Go/Versatly CLIs, SDKs | `(stub - unsafe and restricted in the requested mode) bambu-pp-cli printer control` | Explicit hardware safety boundary |
| 31 | AMS load/unload, RFID reread, drying and settings | HA, MCPs, SDKs | `(stub - restricted without Developer Mode or an authorized Bambu path) bambu-pp-cli ams control` | Explicit authorization boundary |
| 32 | File upload and delete | Go CLI, MCPs, Bambuddy | `(stub - write surface is outside the approved read-only mode) bambu-pp-cli files upload|delete` | No hidden write path |
| 33 | H2-safe 3MF AMS mapping and print dispatch | DMontgomery MCP | `(stub - print dispatch is restricted) bambu-pp-cli job print-3mf` | No unsafe mapping or signature bypass |
| 34 | STL inspection and mesh transforms | DMontgomery MCP | `(stub - generic CAD scope) bambu-pp-cli model` | Redirect to dedicated mesh/CAD tools |
| 35 | Slicer presets, templates, pipelines and CLI slicing | DMontgomery MCP, Bambuddy | `(stub - generic slicer/application scope) bambu-pp-cli slice` | Redirect to Bambu Studio or OrcaSlicer |
| 36 | Queues, scheduling, load balancing, batch/staggered dispatch | Bambuddy, official Local Server | `(stub - requires an authorized write path and persistent scheduler) bambu-pp-cli fleet queue` | Honest unavailable boundary |
| 37 | Smart-plug power and energy accounting | Bambuddy | `(stub - requires external services not in the brief) bambu-pp-cli energy` | No invented power measurements |
| 38 | REST/MQTT/Prometheus service exports | Bambuddy | `(stub - persistent server is application scope) bambu-pp-cli serve` | CLI and MCP remain bounded runtime surfaces |
| 39 | MakerWorld search/import/download | schwarztim MCP, Bambuddy | `(stub - cloud/website source is outside local-LAN scope) bambu-pp-cli makerworld` | Preserves local-only promise |
| 40 | Bambu Cloud connection, profiles and cloud history | HA, PyPI CLI, Bambuddy | `(stub - user explicitly excluded Bambu Cloud) bambu-pp-cli cloud` | No cloud credentials or network dependency |
| 41 | Archive library, projects, comparisons, tags, photos and timelapse editing | Bambuddy | `(stub - multi-user media library is application scope) bambu-pp-cli archive` | Keeps CLI focused on printer truth |
| 42 | Spool inventory, cost, catalog, labels and Spoolman sync | Bambuddy | `(stub - inventory application and external service are outside the brief) bambu-pp-cli spools` | AMS telemetry remains available read-only |
| 43 | Provider-specific Discord, Telegram, email, Pushover and webhook delivery | Bambuddy, HA | `(behavior in bambu-pp-cli events watch) emit provider-neutral lifecycle payloads` | Consumers choose delivery; CLI owns printer semantics |
| 44 | Virtual printer, proxy and remote relay | Bambuddy | `(stub - persistent proxy and remote access are application scope) bambu-pp-cli proxy` | No certificate impersonation or authorization bypass |
| 45 | Farm-app users, permissions, SSO, backups and activity audit | Bambuddy | `(stub - multi-user server administration is application scope) bambu-pp-cli admin` | Not part of a local printer CLI |
| 46 | Camera wall, external cameras, OBS overlay and plate-empty vision | Bambuddy | `(stub - persistent media/vision application is outside the brief) bambu-pp-cli vision` | Current camera flags and plate preview remain readable |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Calibrated completion forecast | `job eta` | 9/10 | hand-code | Uses persisted remaining-time snapshots and terminal transitions to compute a corrected ETA and historical error band. | Madcow remaining time/transitions; durable-history requirement | none |
| 2 | Fleet attention queue | `fleet attention` | 8/10 | hand-code | Uses bounded fleet snapshots plus recent transitions and AMS/job metadata to rank intervention needs. | Fleet operators; official Local Server; Bambuddy; Madcow fields | Use this command for cross-printer intervention prioritization. Do NOT use this command for quantified filament sufficiency; use `ams runway` instead. |
| 3 | Filament runway | `ams runway` | 9/10 | hand-code | Joins exact current-plate 3MF weights, progress and AMS remaining estimates to compute surplus or shortfall. | Madcow 3MF weight; DeepWiki AMS hierarchy; live AMS data | Use this command for quantified filament sufficiency. Do NOT use this command for cross-printer intervention prioritization; use `fleet attention` instead. |
| 4 | Repeat-job comparison | `job repeats` | 7/10 | hand-code | Groups persisted jobs, transitions, HMS observations and asset metadata to compare repeated runs. | Brief data model; Madcow task/file/plate identity and transitions | Use this command for comparing repeated executions of the same printable job. Do NOT use this command for the stage-by-stage chronology of one execution; use `job timeline` instead. |
| 5 | Firmware field drift | `printer field-diff` | 8/10 | hand-code | Compares persisted redacted raw schemas and timestamps for added, removed, type-changed and stale fields. | Madcow's 99 live fields; BambuTools model/firmware compatibility issues | none |
| 6 | Failure correlation matrix | `history failure-correlations` | 9/10 | hand-code | Joins job outcomes to HMS/error, printer, filament, plate, speed, firmware and temperature observations. | Brief analytics requirement; Madcow/HA errors and transitions | none |
| 7 | Print-stage timeline | `job timeline` | 9/10 | hand-code | Reconstructs ordered stages, pauses, layers, temperature recovery and errors from sparse snapshots and events. | DeepWiki reducers; Madcow transitions; live stage/layer fields | Use this command for the stage-by-stage chronology of one execution. Do NOT use this command for comparing repeated executions of the same printable job; use `job repeats` instead. |

## Killed Novel Candidates
- Connectivity reliability report: missing observations cannot distinguish printer, network and collector downtime without a persistent collector.
- Diagnostic incident bundle: incident-only and mostly repackages existing diagnostics/history.
- Thermal stability baseline: actionable interpretation needs unsupported model/material thresholds.
- Job provenance manifest: mostly combines existing fields and is only occasional.
- Plate-to-spool mismatch detector: standard-LAN mapping can be ambiguous, making definitive claims unsafe.
- Estimated energy consumption: no measured power field and smart-plug integration is external.
- Automation sensor export: thin reshaping of structured status without new leverage.
