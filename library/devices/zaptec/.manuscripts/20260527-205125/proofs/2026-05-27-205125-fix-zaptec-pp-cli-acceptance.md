# Zaptec CLI — Phase 5 Acceptance (live read-only)

Level: Live read-only dogfood (user opted out of live control commands; those were dry-run-verified only).
Auth: OAuth2 password grant against the real Zaptec API. Login succeeded; token cached.

## Tests (live, against the authenticated viewer's real account)
| # | Command | Result | Evidence |
|---|---------|--------|----------|
| 1 | `auth login` (OAuth2 ROPC) | PASS | masked-password prompt → token cached; this validated the Priority-0 auth fix end-to-end |
| 2 | `doctor` | PASS | Config ok, Auth configured (oauth2), API reachable, Cache fresh |
| 3 | `chargers list --json` | PASS | returned the home charger with full attributes |
| 4 | `live` | PASS | decoded operation mode = "Charging" (after OperatingMode field fix) |
| 5 | `state <charger>` (decoded) | PASS | Operation mode=Charging, power≈3.5 kW, session≈6.3 kWh, phase-1≈15.7 A, ~225 V, internal temp≈42 °C — all decoded from numeric IDs |
| 6 | `sync` | PASS | chargers + installation stored as typed rows (after id-extraction + PascalCase fixes) |
| 7 | `chargers stale` | PASS | correctly empty (charger online + charging) |
| 8 | `cost --by month` | PASS (honest empty) | no completed sessions yet → `total_kwh:0`; rows `[]` |
| 9 | `sessions anomalies` | PASS (honest empty) | `[]` (no sessions) |
| 10 | `current headroom <installation>` | PASS | max 16 A, available 16 A, headroom 0 A |
| 11 | control commands (`pause`/`resume`/`start`/`stop`/`restart`/`unlock`/`firmware upgrade`) | PASS (dry-run) | correct sendCommand command-IDs (e.g. pause→502); NOT sent live per read-only choice |

Quick-check core (doctor, list, sync, search/cost, json+select, transcendence relevance): all pass. Auth and sync both pass (hard requirements).

## Bugs found and fixed in-session (all shipping-scope, fix-before-ship)
1. **`live` showed mode "Unknown".** Read field `OperationMode`; the chargers-list payload uses `OperatingMode`. Fixed (with defensive fallback).
2. **`sync` stored 0 chargers/installations** (`all_items_failed_id_extraction`). The id-field override was `"id"` but Zaptec uses `"Id"`. Fixed override → `"Id"` and added `"Id"` to fallbacks (sync.go + store.go).
3. **Typed columns (name, operating_mode, is_online, installation_name) stored empty/null.** `LookupFieldValue` only tried the literal key and lowerCamelCase; Zaptec serializes PascalCase. Added a PascalCase resolution pass. This fixed `chargers stale` (was mis-flagging a healthy charger) and the offline/store-backed queries.
4. **Mock data pollution** in the real cache DB from verify runs — cleared and re-synced with live data.

## Known gaps (non-blocking, generator-level)
- Two generated `internal/store` unit tests fail (`priority`, `user_groups_messaging_connection_details`): generic batch typed-upsert inserts a flat `{"id":...}` item into a typed table whose path-param column (e.g. `session_id`) is `NOT NULL`. Pre-existing generator behavior for path-nested sub-resources; these endpoints are not in the default sync set and not part of the home-charger workflow. The live `sessions priority` command (a POST) is unaffected. Filed as retro material.
- `mcp_token_efficiency`/`mcp_tool_design`/`mcp_remote_transport` scored mid (MCP endpoint-mirror surface) — polish-phase candidate.

## Gate: PASS
Auth + sync + all live read-only flagship reads correct. No known functional bugs in shipping-scope features used by the home-charger workflow.
