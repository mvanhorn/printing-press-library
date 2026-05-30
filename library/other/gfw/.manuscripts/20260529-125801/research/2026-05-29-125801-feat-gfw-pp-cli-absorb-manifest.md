# gfw-pp-cli Absorb Manifest

Sources surveyed: official **gfw-api-python-client** (Vessels/Events/Insights/4Wings), official **gfwr** R package (vessels/events), community **samapriya/gfw** CLI (auth/data-list/file-list/download). No agent-native, offline-caching GFW CLI exists.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Vessel search (name/MMSI/IMO/callsign) | python-client, gfwr, samapriya | `(generated endpoint) vessel search` | offline cache, `--json/--select/--compact`, FTS |
| 2 | Vessel get by ID | python-client | `(generated endpoint) vessel get` | caches identity for offline join |
| 3 | Vessel list (cached browse) | — (we add) | `(behavior in gfw-pp-cli vessel list)` SQLite query | browse accumulated vessels offline |
| 4 | Events list (by vessel/type/date) | python-client, gfwr | `(generated endpoint) events list` | typed filters, cached |
| 5 | Event get by ID | python-client | `(generated endpoint) events get` | typed JSON |
| 6 | Event stats | python-client | `(generated endpoint) events stats` | aggregates |
| 7 | Insights (vessel risk indicators) | python-client | `(generated endpoint) insights vessel` | cached, agent-native |
| 8 | 4Wings fishing-effort report | python-client, gfwr | `(generated endpoint) 4wings report` | region/time effort as data |
| 9 | 4Wings worldwide stats | python-client | `(generated endpoint) 4wings stats` | |
| 10 | 4Wings last-report | api | `(generated endpoint) 4wings last-report` | |
| 11 | Datasets (fishing-effort dataset) | samapriya data-list | `(generated endpoint) datasets ...` | dataset metadata |
| 12 | Bulk report create | samapriya download | `(generated endpoint) bulk-reports create` | async bulk |
| 13 | Bulk report get/status | samapriya | `(generated endpoint) bulk-reports get` | status poll |
| 14 | Bulk report query (JSON) | samapriya | `(generated endpoint) bulk-reports query` | query bulk results |
| 15 | Bulk download file URL | samapriya download | `(generated endpoint) bulk-reports download-file-url` | resolve file URL |
| 16 | Auth / token setup | samapriya auth | `(behavior in gfw-pp-cli auth set-token / doctor)` | `GFW_TOKEN` env + doctor health |

Every absorbed row works offline (cached), with agent-native output, typed exit codes, and `--dry-run` where applicable — none of which the Python/R libraries or the download-only community CLI provide.

## Transcendence (only possible with our local-cache + cross-source approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Vessel dossier | `vessel dossier <id>` | hand-code | One call merges identity + recent events + insights risk — a local join across 3 endpoints no SDK call returns together | Use for a one-shot DD snapshot of a vessel. For just identity use 'vessel get'; for just behavior use 'events list'. |
| 2 | Risk rollup | `vessel risk <id>` | hand-code | Composite signal from Insights indicators + event patterns (encounters, AIS gaps, port visits); requires correlating endpoints locally | Use to triage a vessel's risk. Not a substitute for the raw 'insights vessel' indicators. |
| 3 | Encounter network | `encounters network <id>` | hand-code | Builds the at-sea meeting graph (counterpart vessels) from cached encounter events — a local graph the API never returns | Use to map who a vessel met at sea. For raw events use 'events list --types encounter'. |
| 4 | Port-visit pattern | `vessel ports <id>` | hand-code | Local aggregation of port-visit events into a frequency/recency pattern | Use for a vessel's port history pattern. |
| 5 | Watchlist pin/unpin | `watch pin <id>` / `watch unpin <id>` | hand-code | Local watchlist of vessels under active DD; pairs with refresh | Use to track vessels across sessions. `watch --list` shows the watchlist. |
| 6 | Watchlist refresh | `watch refresh [--pinned]` | hand-code | Re-pull events/insights for watchlisted vessels under throttle | Use to bring the watchlist current. |
| 7 | Recent changes | `watch since <dur>` | hand-code | Time-windowed new events for watchlisted vessels — requires accumulated local history | Use for "what happened to my vessels in the last N days". |
| 8 | Dark-activity flag | `vessel gaps <id>` | hand-code | Filters AIS-gap/loitering events into a dark-activity signal from cached events | Use to surface possible AIS disabling. For raw events use 'events list --types gap,loitering'. |

8 transcendence features, all `hand-code`. They compound with `gisis-pp-cli` (identity/flag-history) for the Phase 3 Vessel-MCP orchestrator: GISIS answers "who is this vessel," GFW answers "what has it done and how risky is it."

## Notes
- No stubs planned. All 8 transcendence rows are shipping scope.
- 4Wings map-tile/PNG/MVT rendering and dataset context-layer tiles intentionally excluded (headless DD tool; user-confirmed scope).
