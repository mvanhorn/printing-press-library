Manifest transcendence rows: 8 planned, 8 built. Phase 3 complete — all 8 ship.

# BMW CarData CLI — Build Log

## Built (P0 foundation)
- Custom SQLite schema (`internal/store/extras.go`): `cardata_vehicles`, `cardata_telematic_snapshots` (append-only, deduped by vin/descriptor/ts), `cardata_charging_sessions`, `cardata_descriptor_catalogue`, `cardata_api_calls`.
- Persistence + reader helpers (`internal/cli/cardata_persist.go`): write-through writers (create the store), time-series readers, quota counter, descriptor search.
- Descriptor catalogue seed (`internal/cli/cardata_catalogue.go`, ~45 VSS descriptors).
- Write-through wired into 4 live read commands (`customers get-telematic-data`, `get-basic-data`, `get-charging-history`, `get-mappings`) — every live fetch appends to the local store + records an API call for quota.

## Built (P1 absorb — hand-code)
- `auth login` — OAuth2 Device Authorization Grant + PKCE S256 (device/code → poll token → SaveTokens + GCID/id_token sidecar). Handles authorization_pending/slow_down/access_denied/expired_token. Verify-env + --launch side-effect convention.
- `stream <vin>` — MQTT v5 over TLS (paho.golang) to customer.streaming-cardata.bmwgroup.com:9000; auth username=GCID, password=id_token; subscribes {GCID}/{VIN} QoS1; persists incoming snapshots. --for / --follow.
- `archive read <zip>` — parses Customer Archive .zip; imports telematic + charging JSON shapes into the store; --vin required; --inspect.
- `customers create-container --template hv-battery` — curated HV-battery descriptor bundle.

## Built (P2 transcendence — 8/8, all hand-code, all `// pp:data-source local|computed`)
1. `soc-trends <vin> --window` — SoC + range time-series from snapshots.
2. `fleet status` — multi-VIN current snapshot (SoC/range/charging/location).
3. `vehicles diff <vin> --since` — field-level diff between snapshots.
4. `descriptors search <pattern>` — FTS-like search over seeded VSS catalogue.
5. `charging-cost <vin> --tariff` — energy × tariff + DC efficiency.
6. `quota` — used vs ~50/day cap + midnight-UTC reset.
7. `battery-health <vin>` — degradation vs nameplate hvsMaxEnergyAbsolute.
8. `trips <vin> --since` — trip segmentation from location breadcrumbs (haversine).

## Generator-emitted (12 typed endpoint commands under `customers`)
get-mappings, get-basic-data, get-telematic-data, get-image, get-charging-history, get-smart-maintenance-tyre-diagnosis, get-location-based-charging-settings, list-containers, create-container, get-container-details, delete-container (+ framework: sync/search/analytics/doctor/…). `x-version: v1` header applied globally in client.

## Intentionally deferred / notes
- `stream` MQTT v5 handshake (GCID/id_token) follows the proven whi-tw pattern but is UNVERIFIED in-session — requires Phase 5 live test with the user's client_id + streaming scope. Fails clearly on auth/handshake error, never silent wrong data.
- Endpoint descriptions under `customers` carry some placeholder spec text ("Please note that some keys…") — cosmetic; Phase 4 polish.
- Absorbed endpoint rows use generator-emitted `customers ...` command paths (manifest reconciled).
