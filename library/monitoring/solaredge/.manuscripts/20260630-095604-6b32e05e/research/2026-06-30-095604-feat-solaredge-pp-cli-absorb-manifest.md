# SolarEdge CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

Every wrapper found (Python `solaredge`/`py-solaredge`/`solaredge-interface`, Node `solaredge`/`solarmon`, Go `clambin/solaredge`, Rust `se_ms_api`) is a 1:1 client over this same 24-endpoint official surface; none expose more, and `solaredge-interface` explicitly claims to implement every documented endpoint. The full surface below is absorbed via the generator's typed endpoint emission.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Site List (search/sort/paginate) | Official API `/sites/list` | (generated endpoint) sites list | Offline search via local FTS after sync, `--json`/`--select` |
| 2 | Site Details | Official API `/site/{id}/details` | (generated endpoint) site details | Local cache, instant repeat lookups |
| 3 | Site Data Period | `/site/{id}/dataPeriod` | (generated endpoint) site data-period | Drives sync cursor automatically |
| 4 | Site Data Period bulk | `/sites/{ids}/dataPeriod` | (generated endpoint) sites data-period | Same, bulk |
| 5 | Site Energy | `/site/{id}/energy` | (generated endpoint) site energy | Persisted history beyond the API's per-call window |
| 6 | Site Energy bulk | `/sites/{ids}/energy` | (generated endpoint) sites energy | Fleet-wide energy in one local query after sync |
| 7 | Site Energy Time-Period | `/site/{id}/timeFrameEnergy` | (generated endpoint) site time-frame-energy | — |
| 8 | Site Energy Time-Period bulk | `/sites/{ids}/timeFrameEnergy` | (generated endpoint) sites time-frame-energy | — |
| 9 | Site Power | `/site/{id}/power` | (generated endpoint) site power | — |
| 10 | Site Power bulk | `/sites/{ids}/power` | (generated endpoint) sites power | — |
| 11 | Site Overview | `/site/{id}/overview` | (generated endpoint) site overview | — |
| 12 | Site Overview bulk | `/sites/{ids}/overview` | (generated endpoint) sites overview | — |
| 13 | Site Power Detailed (meters) | `/site/{id}/powerDetails` | (generated endpoint) site power-details | — |
| 14 | Site Energy Detailed (meters) | `/site/{id}/energyDetails` | (generated endpoint) site energy-details | — |
| 15 | Site Power Flow (live) | `/site/{id}/currentPowerFlow` | (generated endpoint) site current-power-flow | — |
| 16 | Storage Information | `/site/{id}/storageData` | (generated endpoint) site storage-data | Persisted battery history beyond the API's 1-week window |
| 17 | Site Image | `/site/{id}/siteImage/{name}` | (generated endpoint) site site-image | — |
| 18 | Installer Logo Image | `/site/{id}/installerImage/{name}` | (generated endpoint) site installer-image | — |
| 19 | Components List | `/equipment/{id}/list` | (generated endpoint) equipment list | — |
| 20 | Inventory | `/site/{id}/Inventory` | (generated endpoint) site inventory | Drives local equipment health baseline |
| 21 | Inverter Technical Data | `/equipment/{id}/{serial}/data` | (generated endpoint) equipment inverter-data | Persisted beyond the API's 1-week window |
| 22 | Account List | `/accounts/list` | (generated endpoint) accounts list | — |
| 23 | Sensor List | `/equipment/{id}/sensors` | (generated endpoint) equipment sensors | — |
| 24 | Sensor Data | `/site/{id}/sensors` | (generated endpoint) site sensor-data | Persisted beyond the API's 1-week window |
| 25 | API Version Current | `/version/current` | (generated endpoint) version current | Used internally by `doctor` |
| 26 | API Version Supported | `/version/supported` | (generated endpoint) version supported | — |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|---------------|--------------------------|-------------------|
| 1 | System health check | `site health <siteId>` | hand-code | Combines four live calls (details, overview, current-power-flow, inventory) into one healthy/degraded/unknown view — no single endpoint returns this | Use this command to get one combined go/no-go status for a site. Do NOT use it for raw live power numbers; use 'site current-power-flow' instead, or for raw summary stats use 'site overview'. |
| 2 | Underperformance vs baseline | `site underperformance <siteId> --since 30d` | hand-code | Computes each day's energy against the site's own trailing historical average from one 364-day energy call — the raw API has no historical-baseline comparison feature | Use this command to flag days that are statistically low vs this site's own history. Do NOT use it for short-term deltas; use 'site changes' instead. |
| 3 | What changed since X | `site changes <siteId> --since 7d` | hand-code | Diffs current-period energy against the prior period of equal length from one energy call, plus a current (non-delta) equipment-count snapshot from inventory — the API has no diff feature and no equipment-history endpoint | Use this command for a short-term delta digest. Do NOT use it for statistical underperformance analysis; use 'site underperformance' instead. |
| 4 | Rate-limit budget tracker | `budget status` | hand-code | Maintains a local call-count table per siteId against the 300/day cap, written to by the other four novel commands — no endpoint exposes remaining quota | none |
| 5 | Equipment fault digest | `equipment faults <siteId>` | hand-code | Combines inventory (optimizer counts), recent battery telemetry (batteryState), and current power-flow element status to surface only non-nominal equipment — no endpoint pre-filters this | Use this command for a filtered list of equipment in a non-nominal state. Do NOT use it for full inventory; use 'equipment inventory' instead. Do NOT use it for raw per-serial telemetry; use 'equipment inverter-data' instead. |

Minimum 5 transcendence features met (5 of 8 survivors kept per user trim at Phase Gate 1.5; all scored ≥7/10). Dropped at user request, not for buildability or evidence reasons: Fleet underperformer rollup, Battery health trend, Stale site detector — these were installer/fleet-oriented features less relevant to a single residential site. Full brainstorm audit trail (customer model, pre-cut candidates, kill reasons) saved at `research/2026-06-30-095604-novel-features-brainstorm.md`.
