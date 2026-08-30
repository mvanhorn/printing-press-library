# SRAM AXS — Absorb Manifest

SRAM AXS is a closed ecosystem: the only existing tools are SRAM's own **web app** (axs.sram.com) and **AXS mobile app**. There are no third-party CLIs, MCP servers, or SDK wrappers (the API is private/undocumented). So "absorb" = match every capability of SRAM's own clients, then transcend with offline + agent-native features they don't offer.

## Absorbed (match the official web/mobile app)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | View registered bikes | AXS web app | `axs-pp-cli bikes list` | `--json`, offline store, scriptable |
| 2 | View bike detail | AXS web app | `axs-pp-cli bikes get <id>` | typed output |
| 3 | View AXS components | AXS web app | `axs-pp-cli components list` | `--json`, `--select`, offline |
| 4 | Component detail (firmware, battery) | AXS web app | `axs-pp-cli components get <id>` | typed output |
| 5 | Register a component by serial | AXS web app onboarding | `axs-pp-cli registrations create --serial ...` | `--dry-run`, scriptable batch |
| 6 | Device-type catalog | AXS web app (public) | `(generated endpoint) devicetypes list` | works with no login |
| 7 | Model / firmware catalog | AXS web app | `(generated endpoint) models list` | offline store |
| 8 | Product detail | AXS web app | `(generated endpoint) products get <id>` | typed |
| 9 | Notifications inbox | AXS web app | `(generated endpoint) notifications list` | `--json` |
| 10 | Linked accounts (Strava etc.) | AXS web app | `axs-pp-cli linkedids list` / `unlink <id>` | scriptable |
| 11 | Account profile | AXS web app | `(generated endpoint) account profile` | typed |
| 12 | Data export request | AXS web app (GDPR) | `(generated endpoint) account export` | one command |
| 13 | Ride activities | AXS / Quarq | `axs-pp-cli activities list` | offline, paginated |
| 14 | Activity summaries | Quarq | `(generated endpoint) summaries activities` | `--json` |
| 15 | Component usage summaries (wear) | Quarq | `axs-pp-cli summaries components` | offline rollup |
| 16 | Aggregate riding stats | Quarq | `(generated endpoint) stats get` | typed |
| 17 | Auth (SRAM account login) | Auth0 web flow | `(behavior in axs-pp-cli auth login)` Auth0 password-realm + token paste | terminal login, no browser |

## Transcendence (only possible with our offline + cross-resource approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Firmware staleness check | firmware-check | hand-code | Local join of each component's installed firmware against the `models` catalog's latest — the app shows a badge per-component, never a fleet-wide list | Use to find every AXS component that has a firmware update available. Reads local store + models catalog. |
| 2 | Component wear rollup | wear | hand-code | Aggregates `componentsummaries` (distance, shift counts, battery changes) across ALL bikes into one ranked table — the app only shows per-component | none |
| 3 | Battery dashboard | battery | hand-code | Cross-component battery-level view sorted lowest-first, so you know what to charge before a ride — the app buries battery per-component | none |
| 4 | Unified garage view | garage | hand-code | One tree: each bike → its components → firmware + battery, joined locally from bikes+components — no single API call returns this | none |
| 5 | What changed since last sync | since | hand-code | Time-windowed diff of new activities and notifications from the local store — impossible without a persistent local mirror | Use for recent ride/notification changes since your last sync. |

All five transcendence rows are hand-code (require local SQLite joins or cross-resource aggregation). Auth login is a behavior-row hand-code. No stubs.
