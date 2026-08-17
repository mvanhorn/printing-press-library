# 3CX XAPI CLI — Phase 3 Build Log

Manifest transcendence rows: 7 planned (hand-code), 7 built. Phase 3 gate target met.

Planned/built hand-code rows: audit, diff (+snapshot via `diff --save`), provision, qrollup, posture, trace, changed.
spec-emits row (not hand-code): search (framework FTS5) — verified resolves.

## What was built
- `internal/cli/novel_common.go` — shared local-mirror helpers (kebab resource_type constants, defensive PascalCase JSON extraction, DN-number-set builder, dangling-ref + timestamp cores). Confirmed resource_type wiring matches sync via integration test.
- `audit` (local) — dangling-reference integrity across ring groups, queue agents, inbound-rule destinations vs the valid DN set. External transfer numbers excluded via looksInternalExtension heuristic.
- `trace <ext>` (local) — every ring group / queue / inbound rule / DID route reaching one extension.
- `posture` (local) — blocklist + blacklist + firewall + security-keyword event signals + admin audit aggregate.
- `changed --since` (local) — time-windowed merge of activity-log/event-logs/active-calls/system-status.
- `qrollup --since` (local) — per-queue agent counts + ring strategy + merged queue-stat reports when synced.
- `diff` (local) — `diff --save <name>` snapshots the config graph to JSON; `diff <a> <b>` compares; `diff --list`.
- `provision --file` (live) — bulk-create extensions from CSV, idempotent (--idempotent on 409), --dry-run plan; guarded under verify/dogfood so testing never writes to a real PBX.

## Tests
- novel_behavior_test.go — findDanglingRefs (incl. external-number exclusion), findRoutesToExtension (positive + negative), diffSnapshots (added/removed/changed + identical), looksInternalExtension table.
- novel_store_test.go — full store-integration (Upsert under kebab resource_type → dnNumberSet/listObjects/findDanglingRefs/buildQueueRows) proving the local-mirror contract.

## Notes / deferred
- All 7 are local-mirror readers (except provision/live). Live parameterized OData report-function shapes (queue stats) are merged opportunistically by qrollup when synced; not hard-required.
- Generator gap: workflow template `comm_health.go.tmpl` missing (warning, skipped) — retro candidate.
- Empty git author/printer config — only matters at publish.
