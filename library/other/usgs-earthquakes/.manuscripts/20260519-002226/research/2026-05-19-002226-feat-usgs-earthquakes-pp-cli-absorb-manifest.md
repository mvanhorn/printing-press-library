# USGS Earthquakes — Absorb Manifest

This is the binding feature list. Phase 3 builds every row here.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Status |
|---|---------|-------------|--------------------|--------|
| 1 | Find earthquakes by time/magnitude | blake365/usgs-quakes-mcp `find-earthquakes` | `events search` + `recent` with full FDSN param surface | spec-emits |
| 2 | Get single earthquake by ID | blake365/usgs-quakes-mcp `find-earthquake-details` | `events get <id>` | spec-emits |
| 3 | Bbox region search | usgs-earthquake-api (npm) + obspy | `events search --min-latitude --max-latitude --min-longitude --max-longitude` | spec-emits |
| 4 | Circle/radius search | obspy FDSN client | `events search --latitude --longitude --max-radius-km` | spec-emits |
| 5 | Magnitude filter | every FDSN client | `--min-magnitude` / `--max-magnitude` / `--magnitude-type` | spec-emits |
| 6 | Depth filter | obspy | `--min-depth-km` / `--max-depth-km` | spec-emits |
| 7 | Event type filter | FDSN spec | `--event-type` | spec-emits |
| 8 | Alert level filter (PAGER) | FDSN spec — **NOT** in either MCP | `--alert green\|yellow\|orange\|red` | spec-emits |
| 9 | MMI / CDI filter | FDSN spec — **NOT** in either MCP | `--min-mmi`, `--max-mmi`, `--min-cdi`, `--max-cdi` | spec-emits |
| 10 | Product type filter | FDSN spec — **NOT** in either MCP | `--product-type shakemap,losspager,dyfi,focal-mechanism,moment-tensor` | spec-emits |
| 11 | Review status filter | FDSN spec — **NOT** in either MCP | `--review-status reviewed\|automatic\|all` | spec-emits |
| 12 | Catalog filter | FDSN spec | `--catalog us,nc,ci,ak,...` (enum from metadata cache) | spec-emits |
| 13 | Contributor filter | FDSN spec | `--contributor us,nc,...` | spec-emits |
| 14 | Count queries | FDSN /count | `events count` | spec-emits |
| 15 | Catalogs list | FDSN /catalogs | `catalogs list` | spec-emits |
| 16 | Contributors list | FDSN /contributors | `contributors list` | spec-emits |
| 17 | Application metadata (enums) | FDSN application.json | `metadata show` | spec-emits |
| 18 | All 20 GeoJSON summary feeds | aio-geojson-usgs-earthquakes (Python) | `feeds get <feed>` (parameterized) + `feed-list` discovery | spec-emits + hand-code |
| 19 | Feed snapshot caching | aio-geojson-usgs-earthquakes | `--max-age` flag on `feeds get`, written into local cache | hand-code |
| 20 | Time-window parsing | every tool reinvents | `--since 24h` / `--since 7d` / `--start <iso>` / `--end <iso>` | hand-code (on `recent`/`search`/`watch`) |
| 21 | Output formats | FDSN native | `--format geojson\|csv\|kml\|xml\|text\|quakeml` (passthrough) + `--json` default | spec-emits |
| 22 | Ordering | FDSN /query | `--order-by time\|time-asc\|magnitude\|magnitude-asc` | spec-emits |
| 23 | Limit + offset pagination | FDSN /query | `--limit` (default 100, ≤20K), `--offset` | spec-emits |
| 24 | Tsunami filter | FDSN passthrough | `--tsunami-only` flag on `recent` | hand-code |
| 25 | Felt count filter | FDSN spec | `--min-felt` | spec-emits |
| 26 | Significance score filter | FDSN spec | `--min-significance` / `--max-significance` | spec-emits |
| 27 | Updated-after watermark | FDSN spec | `--updated-after` (also drives sync watermark) | spec-emits |
| 28 | Doctor / health check | every CLI | `doctor` covers FDSN /version + summary feed reachability + UA | spec-emits |
| 29 | Sync to local SQLite | **NO** existing tool has this | `sync` (last 30 days, M2.5+, watermark incremental) | hand-code |
| 30 | Offline search / SQL | **NO** existing tool has this | `search` (FTS5) + `sql` (raw SELECT) | hand-code |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|------------------------|
| T1 | Live event watch with dedup + pluggable notifier | `watch --min-magnitude 5 --since-cursor <last-id> --notify "cmd {id}"` | hand-code | Polls summary feed, dedups against local `earthquakes` table by event ID, invokes user shell hook per new event. Short-circuits under `IsDogfoodEnv` / `IsVerifyEnv`. No competing tool offers polling. |
| T2 | Aftershock sequence query | `aftershocks <event-id> --radius-km 100 --days 30 --min-mag 3.0` | hand-code | Local SQLite haversine query on `earthquakes` bounded by mainshock time+location, with FDSN `/query` fallback when uncached. Competing MCP exposes only single-event detail; no CLI composes the sequence. |
| T3 | Spatial-temporal swarm detection | `swarm-detect --bbox W,S,E,N --window 7d --min-events 10 --cluster-radius-km 20` | hand-code | Grid-bucket spatial+temporal clustering over local `earthquakes` table (group by 0.1° cells × T-hour windows, filter cells ≥N, merge contiguous hot cells). Volcano monitoring, fault swarms, induced seismicity. |
| T4 | Region or period comparison | `compare --region-a <bbox> --region-b <bbox> --window 30d` OR `compare --region <bbox> --period-a 2020 --period-b 2024` | hand-code | Two parallel `SELECT COUNT, MAX(mag), SUM(energy)` aggregations on local `earthquakes` with delta; FDSN `/count` fallback. Research-grade comparison no competitor exposes. |
| T5 | Newsroom event briefing | `brief <event-id> --format markdown\|text\|json` | hand-code | Composes FDSN `/query?eventid=X` GeoJSON properties + products manifest into a structured briefing block (mag, place, PAGER, DYFI, tsunami, MMI, product inventory, event-page URL). Competing MCP returns raw JSON; this is agent-native by construction. |
| T6 | Editorial-rank top events | `top --window 24h --limit 10 --score composite` | hand-code | Local `earthquakes` `ORDER BY (sig * alert_weight(alert) * (1+ln(1+felt)) * (1+2*tsunami)) DESC` over windowed slice. FDSN exposes `sig` but no composite ranking endpoint. |
| T7 | Stateful change / revision diff | `changes --since 24h --type new\|revised\|deleted --min-mag-delta 0.3` | hand-code | `revisions` table populated by `sync` comparing pre/post values per event on `(mag, depth, alert, status, updated)`, queried by watermark. Genuinely novel — no tool tracks USGS solution revisions. |
| T8 | USGS event ID decoder | `decode-id us7000abcd` | hand-code | String parse of `<network><sequence>` joined against cached `contributors` dictionary for network display name + operator. Common confusion point for new users; no tool decodes IDs today. |

**Hand-code commitment:** 8 transcendence features + 5 hand-code absorbed (T1-T8 + #19, #20, #24, #29, #30) = **13 hand-code commands** beyond what the generator auto-emits. Each ~50-150 LoC plus `root.go` wiring.

## Killed candidates (audit trail)

| Candidate | Reason |
|-----------|--------|
| `region-history <place>` | Spec-emits via FDSN `/count` loop with year buckets; too thin to be novel |
| `migration-check` | No community-pain evidence; catalogs/contributors already show current state |
| `escalation-history` | Requires multi-sync history first-install users don't have |
| `explain <event-id>` | Subsumed by `event <id> --json` + static reference text |
| `digest` | Subsumed by composing `top` + `brief` |
| `summary --by` | Spec-emits via FDSN `/count`; remainder is `sql` |
| `predict-next` aftershock probability | LLM/ML dependency + verifiability — Omori-law fit not shippable/testable |
| `dashboard` TUI | Scope creep — application not a command; `watch` is the descope |
| `notify-slack` | External service + scope creep — `watch --notify` keeps notifier pluggable |
