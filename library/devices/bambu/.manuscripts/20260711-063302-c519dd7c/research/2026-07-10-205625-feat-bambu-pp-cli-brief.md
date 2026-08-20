# Bambu CLI Brief

## API Identity
- Domain: local Bambu Lab printer monitoring over SSDP, MQTT/TLS, and implicit FTPS.
- Users: makers, print-farm operators, home automation users, and agents that need structured printer state without Bambu Cloud.
- Data profile: partial MQTT reports plus on-demand 3MF ZIP/XML/image metadata. There is no OpenAPI spec or REST API for this surface.
- Primary sources: Bambu Lab's third-party integration and access-code documentation; OpenBambuAPI protocol notes; BambuTools `bambulabs_api`; the live Madcow H2D implementation.

## Reachability Risk
- **Medium for read-only H2D monitoring; high for portable control across models.** Bambu's current authorization documentation explicitly leaves MQTT status pushes unaffected in normal LAN mode, but restricts print start, movement, temperatures, fans, AMS settings, calibration, and camera access outside authorized Bambu paths or Developer Mode.
- Madcow has live H2D evidence for SSDP discovery, MQTT `pushall`, partial report accumulation, Bambu-CA/CN peer verification, and exact-current-job 3MF retrieval over FTPS with Developer Mode and LAN Only both disabled.
- `BambuTools/bambulabs_api` has open firmware/auth compatibility issue #95, H2D AMS mapping issue #153, X1 FTPS session-reuse issue #158, and X1E missing-value issue #177. Its README says H2D is untested.
- `tobiasbischoff/bambu-cli` has open P2S protocol issue #1 and FTPS `522 ... session reuse required` issue #2. Its MQTT, FTP, and camera clients disable TLS verification.
- Probe-safe operation: MQTT `pushing.pushall` followed by the device report topic. It is read-only but should be used sparingly because large update volume can lag the printer.

## Top Workflows
1. Emit provider-neutral lifecycle payloads: at print start include job name, plate weight, estimated finish time, and a preview attachment reference; at completion include outcome and actual timing.
2. Discover the configured printer and get one fresh, normalized status snapshot: job, progress, ETA, layer, speed, temperatures, fans, active filament, network signal, light, errors, and HMS.
3. Inspect AMS units and trays: material, color, spool identity, humidity, temperature, remaining estimate, and active tray.
4. Inspect the current job's exact 3MF metadata: selected plate, predicted/actual filament weight, objects, and thumbnail without choosing an arbitrary newest file.
5. Watch state transitions, persist queryable history, and diagnose reachability, certificate identity, MQTT freshness, FTPS availability, raw-field coverage, and secret-redacted evidence.

## Table Stakes
- SSDP discovery with explicit-host fallback; serial filtering; bounded timeouts and reconnects.
- Local access-code auth via environment/config without exposing values in flags, output, logs, MCP descriptions, or artifacts.
- MQTT/TLS on port 8883, subscription to `device/{serial}/report`, publish to `device/{serial}/request`, sparse-report merge, and fresh `pushall` snapshots.
- Human, JSON, compact, CSV/select, and watch output suitable for scripts and agents.
- Status, temperatures, fans, speed, AMS, health/HMS, camera/timelapse flags, queue/upload/upgrade state, and a raw redacted report.
- Implicit FTPS on port 990 with TLS session reuse; exact file listing/download; bounded 3MF parsing and optional thumbnail output.
- Camera capability/status reporting. Image capture is model/authorization-dependent and is not initial shipping scope.
- Multi-printer profiles and a `doctor` command with protocol-specific diagnostics.
- MCP parity for every read operation.

## Data Layer
- Primary entities: printers, printer snapshots, print jobs, state transitions, AMS units/trays, HMS/error observations, and job assets.
- Sync cursor: observation timestamp plus MQTT `sequence_id`; merge sparse reports into the latest per-printer snapshot.
- Search: job/subtask/file names and normalized HMS/error text. SQL should support duration, material use, failure rate, temperature, and per-printer history analysis.
- Retention: store normalized scalar/report JSON and asset metadata; do not store access codes or duplicate thumbnail/archive blobs by default.

## Codebase Intelligence
- Source: DeepWiki analysis of `greghesp/ha-bambulab`, indexed 2025-04-21, plus current MCP source inspection.
- Auth: local MQTT/TLS uses IP, serial, username `bblp`, and access code; implicit FTPS 990 and supported camera transports reuse the access code. Madcow additionally validates Bambu's CA and the peer certificate CN against the configured serial.
- Data model: a central Device owns specialized temperature, lights, fans, print-job, info, AMS, external-spool, HMS, and print-error components. AMS is hierarchical: up to four units, four trays per unit, active tray values 0-15, external spool 254, none 255.
- Rate limiting: no HTTP rate limit applies. `pushall` must be used sparingly because full MQTT snapshots can lag the printer; long-lived subscriptions should merge partial reports.
- Architecture: MQTT updates flow through component-specific reducers that compare previous/current state and emit meaningful events. FTP enriches the job model with 3MF/model metadata. Model, hardware, and firmware capability checks prevent exposing unsupported features.

## User Vision
- Build a Printing Press CLI for Bambu 3D printers using local LAN APIs, not Bambu Cloud and not Developer Mode.
- Start from Madcow's proven SSDP, MQTT status, FTPS current-3MF, plate-weight, thumbnail, and transition implementation.
- Expose the richer live report: progress, ETA, temperatures, layers, speed, AMS trays/colors, active filament, errors/HMS, camera/timelapse flags, queue, upload, upgrade, and raw fields.
- Reproduce Madcow's real workflow without coupling the CLI to Discord: a polished start event with print information, plate preview, weight, and estimated finish time, followed by a finished event. Structured payloads must be directly consumable by Discord, Pushover, webhook, or automation adapters.

## Product Thesis
- Name: Bambu LAN CLI (`bambu-pp-cli`).
- Why it should exist: existing tools optimize for broad control but often weaken TLS, mishandle FTPS session reuse, assume Developer Mode, flatten sparse reports, or omit durable history. This CLI provides the secure, read-only, agent-native observability surface that current standard LAN mode actually permits.

## Build Priorities
1. Secure discovery/auth/TLS plus a complete normalized fresh-status command and redacted raw escape hatch.
2. AMS, HMS/health, camera flags, queue/upload/upgrade, and current-job 3MF metadata/thumbnail.
3. Watch/sync with transition history, offline SQL/search, and full MCP parity.
4. Replay fixtures across H2D and community report shapes, followed by live read-only H2D dogfood.

## Evidence Sources
- Bambu Lab third-party integration: https://wiki.bambulab.com/en/software/third-party-integration
- Bambu Lab access-code guide: https://wiki.bambulab.com/en/knowledge-sharing/access-code-connect
- BambuTools API docs: https://bambutools.github.io/bambulabs_api/api.html
- OpenBambuAPI MQTT protocol: https://github.com/Doridian/OpenBambuAPI/blob/main/mqtt.md
- Madcow Bambu core: https://github.com/twidtwid/madcowbot/blob/main/core/bambu.py
- Independent Go CLI: https://github.com/tobiasbischoff/bambu-cli
- Bambu printer MCP: https://github.com/DMontgomery40/bambu-printer-mcp
- Node SDK: https://github.com/THE-SIMPLE-MARK/bambu-node

## Reachability Gate
- Decision: PASS (carve-out)
- Reason: lan-only-no-global-url
- Evidence: the target is discovered by Bambu SSDP on the user's private LAN and connects directly to the discovered appliance over MQTT/TLS 8883 and implicit FTPS 990; there is no stable global HTTP origin to probe.
