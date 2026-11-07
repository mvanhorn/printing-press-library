Manifest transcendence rows: 8 planned, 8 built. Phase 3 will not pass until all 8 ship.

## Slice 1: transport and config (done, build and vet green)
Files: internal/client/unifi_transport.go, unifi_audit.go, client.go seams; internal/config/unifi_config.go; internal/store/unifi_console.go, store.go seams; internal/cli/unifi_target.go, sync.go, data_source.go, helpers.go, doctor.go seams. Patches recorded under .printing-press-patches.
Absorption check of prior patches: batch upsert committed count already absorbed (no change). HTML classifier seam absorbed as classifyHTMLPayload (used). Ambiguous writes NOT absorbed (re-applied). TLS skip and mirror cleanup NOT absorbed (re-implemented).
Generator bugs found: dependents parented on Site Manager /v1/sites; Mobility devices_clients parented on Site Manager devices; device uplink only present on GET detail, not list.

## Slice 2: absorbed extras (done, build and vet green)
Files: internal/cli/raw.go, fleet.go, audit.go; internal/mcp/unifi_transport_test.go. Rows 2, 6 and 7 (via raw), 10, 11, 12, 13 built.
Priority 1 review gate: --help, --dry-run and --json exercised by tests for raw, fleet, audit.

## Slice 3: reach, topology, drift, newcomer, rule-predict (done, build and vet green)
Files: internal/cli/reach.go, topology.go, drift.go, newcomer.go, rule_predict.go, unifi_mirror.go; internal/store/unifi_history.go, extras.go. Sync now hydrates device details (uplink only exists on GET detail), syncs Site Manager devices and ISP samples, and takes a console-keyed snapshot after a successful run.

## Slice 4: impact, digest, change apply, change undo (done, build and vet green)
Files: internal/cli/impact.go, digest.go, change_apply.go (apply, undo, and an added list so undo can find an id).

## Completion gate
Per-row Cobra walk: raw, fleet, audit, doctor, sync, search, reach, impact, change apply, change undo, digest, drift, newcomer, rule-predict, topology all resolve to '<leaf> [flags]'.
Backstop: dogfood novel_features_check {"planned":8,"found":8}. Overall dogfood verdict is FAIL on 2 naming violations in generated network_info.go and protect_info.go (verb info->get), which pre-date this phase and were left for Phase 4.
Manifest row 13 starts "unifi-pp-mcp", which the gate's row grammar treats as invalid (not "unifi-pp-cli ..."); checked by hand instead: unifi_search, unifi_get, unifi_read, unifi_execute registered, bearer HTTP present.

## Deferred
Typed controller commands (rows 6, 7) wait for live proof, per the assignment; raw is their home today.
Nothing in this phase was tested against a real console.

## Skipped body fields and generator limitations
No skipped bodies touched. Generator limitations: dependents parented by path parameter name (see unifi-sync-parents); no multi-host routing; resourceIDFieldOverrides picks httpStatusCode for Mobility from the error schema (harmless fallback, left alone).
