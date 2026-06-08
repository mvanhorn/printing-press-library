# NinjaOne CLI — Absorb Manifest

## Scope
- **306 operations / 246 paths** from the official OpenAPI 3.0.1 spec are emitted as typed endpoint commands by the generator (every feature every competitor exposes is reachable).
- High-value families promoted to ergonomic named commands + the framework (sync/search/analytics/sql/tail) + a thin MCP (`ninjaone_search` + `ninjaone_execute`).
- **8 hand-code transcendence features** layered on top — fleet-wide, cross-org, local-store-powered commands no competitor ships.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Device list/get/search | Lungshot MCP, homotechsual PS | (generated endpoint) devices list/get | offline FTS, `--json`/`--select`, `df` filter flag |
| 2 | Reboot device | Lungshot MCP | (generated endpoint) devices reboot | `--dry-run`, typed exits |
| 3 | Set/clear maintenance mode | Lungshot, fredriksknese MCP | (generated endpoint) devices maintenance | dry-run, scriptable |
| 4 | Run script on device | Lungshot MCP, ninjapy | (generated endpoint) devices run-script | dry-run, `management` scope guard |
| 5 | Device system/HW queries (AV, health, OS, disks, volumes, NICs, processors, RAID, services, software, warranty, last-user) | Lungshot MCP | (generated endpoint) device sub-resources + `/queries/*` | offline cache, group-by analytics |
| 6 | OS patch query/scan/apply | Lungshot MCP, StackJack | (generated endpoint) os-patches + management endpoints | adaptive limiter on bulk apply |
| 7 | Software patch query/scan/apply | Lungshot MCP | (generated endpoint) software-patches | same |
| 8 | Org list/get/create/update | wyre MCP, ninjapy, homotechsual | (generated endpoint) organizations | offline store, cross-org joins |
| 9 | Org locations | wyre MCP, ninjapy | (generated endpoint) organization locations | persisted locally |
| 10 | Contacts / end-users | StackJack, ninjapy | (generated endpoint) contacts/end-users | CRUD + offline search |
| 11 | Policies + overrides | homotechsual, Lungshot | (generated endpoint) policies + policy-overrides | local persistence |
| 12 | Alert list/get/reset (single) | every MCP | (generated endpoint) alerts list/get/reset | offline store powers cohort commands below |
| 13 | Conditions | fredriksknese MCP | (generated endpoint) conditions | — |
| 14 | Ticket CRUD + comments + boards/forms/statuses | Lungshot, wyre MCP, StackJack | (generated endpoint) ticketing/* | offline search, `--json` |
| 15 | Custom fields query/update (device/org/location/end-user) | Lungshot, homotechsual | (generated endpoint) custom-fields/* | powers `cf-hygiene` below |
| 16 | Automation scripts / jobs / scripting options | Lungshot, fredriksknese | (generated endpoint) automation/* | dry-run, scope guard |
| 17 | Activities feed | ninjapy, jasondsmith72 | (generated endpoint) activities | local store, `tail` |
| 18 | Vulnerability management | jasondsmith72 gateway | (generated endpoint) vulnerability/* | offline search |
| 19 | Backup usage/jobs | ninjapy (partial) | (generated endpoint) backup/* | — |
| 20 | Software licenses | jasondsmith72 | (generated endpoint) software-licenses | — |
| 21 | Asset tags / relationships | ninjapy (batch), jasondsmith72 | (generated endpoint) asset-tags / asset-relationships | — |
| 22 | Billing (accounts/agreements/invoices/products) | jasondsmith72 gateway | (generated endpoint) billing/* | — |
| 23 | KB articles | jasondsmith72 | (generated endpoint) knowledge-base/* | offline search |
| 24 | Org documents / templates / checklists | jasondsmith72 | (generated endpoint) documents/* | — |
| 25 | Groups, node roles, custom tabs, unmanaged devices | jasondsmith72 gateway | (generated endpoint) groups/node-roles/custom-tabs/unmanaged-devices | full coverage |
| 26 | Webhooks config | Lungshot, ninjapy | (generated endpoint) webhooks | — |
| 27 | Offline full-text search | NONE (no competitor) | (behavior in ninjaone-pp-cli search) FTS over synced entities | unique vs all competitors |
| 28 | Group-by analytics / raw SQL / live tail | NONE | (behavior in ninjaone-pp-cli analytics / sql / tail) | unique |
| 29 | Cursor pagination auto-follow + `df` device-filter | partial (ninjapy) | (behavior in ninjaone-pp-cli sync) | first-class `df`, auto-`nextCursor` |
| 30 | Adaptive Retry-After rate limiter | NONE (all do it naively) | (behavior across all live commands) | bulk ops survive 429 throttling |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Cross-org patch-gap report | `patch-gaps [--severity] [--org]` | hand-code | Local join across os_patches+software_patches+devices+organizations; the console + API are org-scoped, no single call is fleet-wide | none |
| 2 | Patch-stuck detector | `patch-stuck [--cycles N]` | hand-code | Needs a local time-series of patch state across syncs; the API has no history | Use this command for KBs failing repeatedly over time. For a point-in-time list of all current gaps use `patch-gaps`; to fix them use `patch-sweep`. |
| 3 | Throttled cross-org patch sweep | `patch-sweep --df <filter> [--dry-run]` | hand-code | Feeds a fleet-wide gap cohort into scan+apply serialized through the adaptive Retry-After limiter; competitors apply naively and 429 out | Use this command to scan+apply patches across a cohort. This mutates devices; for a read-only report use `patch-gaps`. Post-apply reboots are handled here. |
| 4 | Alert-storm clustering | `alert-storms [--window]` | hand-code | Mechanical group-by over local alerts (org, location, condition, time-bucket) collapses a storm into ranked incidents — no API endpoint does this | none |
| 5 | Bulk alert reset by query | `alert-reset --where <cond\|org\|age> [--dry-run]` | hand-code | Resolves alert IDs from a local-store predicate then resets serialized through the limiter; single-reset exists, cohort-reset-by-predicate does not | Use this command to reset many alerts by predicate. To understand the storm first use `alert-storms`; this command mutates. |
| 6 | Flapping-alert detector | `alert-flappers [--window]` | hand-code | Counts fire/auto-resolve cycles per (device, condition) from retained local alert history; the API exposes no flap history | none |
| 7 | Stale/offline device sweep | `stale [--offline-days N] [--reboot]` | hand-code | Cross-org local query for devices with no contact in N days, grouped by org; optional throttled cohort reboot | Use this command for devices gone quiet fleet-wide. For entities missing required field values use `cf-hygiene`. |
| 8 | Custom-field hygiene | `cf-hygiene --require <field...>` | hand-code | Joins local custom_field_definitions with device/org CF values to list entities missing required values, fleet-wide | none |

## Buildability notes
- **6 features** (patch-gaps, patch-sweep, alert-storms, alert-reset, stale, cf-hygiene) read current synced state from the local store — standard hand-code transcendence rows.
- **2 features** (patch-stuck, alert-flappers) additionally require a **patch/alert history table** retained across syncs (the generated store upserts current state only). This is a small extra data-model addition (one history table + a snapshot write in the sync path). Flagged for the user at the gate.
- **2 features mutate** state (patch-sweep, alert-reset) — both ship with `--dry-run` defaulting to a count/preview and require the `management` scope.
