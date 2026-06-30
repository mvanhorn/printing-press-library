# BMW CarData CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|------------|
| 1 | OAuth2 device-code login + token refresh | kvanbiesen/bmw-cardata-ha, whi-tw, tjamet/bmw-cardata | `bmw-cardata-pp-cli auth login` (hand-code) | One-command device flow, PKCE S256, auto-refresh, stored session |
| 2 | List mapped vehicles | spec getMappings; tjamet | `(generated endpoint) customers get-mappings` | Offline, --json, --select |
| 3 | Get basic vehicle data | spec getBasicData; tjamet | `(generated endpoint) customers get-basic-data` | Offline store, typed |
| 4 | Get telematic data (per container) | spec getTelematicData; kvanbiesen, tjamet | `(generated endpoint) customers get-telematic-data` | --select descriptors, --compact; write-through to store |
| 5 | Get vehicle image | spec getImage; tjamet | `(generated endpoint) customers get-image` | --out file |
| 6 | Get charging history (paginated) | spec getChargingHistory; tjamet | `(generated endpoint) customers get-charging-history` | Full pagination into SQLite, --since |
| 7 | Get smart maintenance tyre diagnosis | spec getSmartMaintenanceTyreDiagnosis; kvanbiesen | `(generated endpoint) customers get-smart-maintenance-tyre-diagnosis` | Typed tyre set, offline |
| 8 | Get location-based charging settings | spec getLocationBasedChargingSettings; tjamet | `(generated endpoint) customers get-location-based-charging-settings` | Offline |
| 9 | List containers | spec listContainers; kvanbiesen, tjamet | `(generated endpoint) customers list-containers` | Offline |
| 10 | Create container | spec createContainer; kvanbiesen | `(generated endpoint) customers create-container` | --template, --technical-descriptors, --dry-run |
| 11 | Get container details | spec getContainerDetails; kvanbiesen, tjamet | `(generated endpoint) customers get-container-details` | Offline |
| 12 | Delete container | spec deleteContainer; kvanbiesen, tjamet | `(generated endpoint) customers delete-container` | --dry-run, confirm |
| 13 | MQTT stream of live telematic data | kvanbiesen, whi-tw, dj0abr/bmw-mqtt-bridge, bausi2k | `bmw-cardata-pp-cli stream <vin>` (hand-code) | Stream-to-SQLite, --follow, QoS1 |
| 14 | Customer Archive .zip reader (XML KeyList) | tjamet/bmw-cardata read-archive | `bmw-cardata-pp-cli archive read <zip>` (hand-code) | Import to SQLite, structured, historical |
| 15 | Auto-create curated "HV Battery" container | kvanbiesen/bmw-cardata-ha | `(behavior in bmw-cardata-pp-cli customers create-container) --template hv-battery` | One-flag curated descriptor bundle |

## Transcendence (only possible with our approach)
All 8 are hand-code (require local SQLite joins / time-series / static catalogue / computed counters that no API call or existing tool provides).

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | SoC/range trends | `soc-trends <vin> [--window 7d\|30d]` | hand-code | Joins telematic snapshots (SoC descriptors) over time in SQLite; derived range as column | Use for SoC and derived range as a numeric series over time. For capacity degradation vs nameplate use `battery-health`; for monetary charging cost use `charging-cost`. |
| 2 | Fleet status | `fleet status` | hand-code | Cross-join vehicles × latest telematic snapshot per VIN into one multi-VIN table | Multi-VIN current snapshot across all mapped vehicles. For a single VIN's history over time use `soc-trends`. |
| 3 | Snapshot diff | `vehicles diff <vin> [--since <dur>]` | hand-code | Field-by-field diff between two retained snapshots — only possible with stored history | Discrete state changes between two snapshots. For a numeric level series over time use `soc-trends`. |
| 4 | Descriptor search | `descriptors search "<pattern>"` | hand-code | FTS over seeded VSS descriptor catalogue (path, unit, domain) — service-specific reference data | none |
| 5 | Charging cost & efficiency | `charging-cost <vin> --tariff <per-kwh>` | hand-code | Joins charging sessions (kWh/duration) × energy × user tariff; DC efficiency per session | Monetary cost and DC efficiency of charging sessions (requires --tariff). For SoC level trends use `soc-trends`. |
| 6 | Quota tracker | `quota` | hand-code | Local computed counter from sync metadata: used vs ~50/day cap + midnight-UTC reset | Shows the daily API budget remaining and UTC-reset time. To actually spend budget fetching data use `sync`. |
| 7 | Battery health | `battery-health <vin>` | hand-code | Joins basicData nameplate capacity × max observed energy in history → degradation % | Capacity degradation (observed vs nameplate). For SoC level over time use `soc-trends`. |
| 8 | Trip reconstruction | `trips <vin>` | hand-code | Segments navigation.currentLocation breadcrumb time-series into trips (start/end/elapsed/distance) | none |

## Notes
- **Hand-code commitment:** 8 transcendence (all hand-code) + 3 absorbed hand-code (`auth login`, `stream`, `archive read`) = **11 hand-written Go commands** beyond the generator's emitted endpoint surface. The remaining 12 absorbed features are generator-emitted typed endpoint commands.
- **Stubs:** none. All features are shipping scope.
- **Rate limit awareness:** every live-fetching command must respect the ~50 calls/day cap; `quota` + sync metadata + `--max-pages`/conservative polling are the mitigation. Transcendence commands are local-only (no quota cost).
