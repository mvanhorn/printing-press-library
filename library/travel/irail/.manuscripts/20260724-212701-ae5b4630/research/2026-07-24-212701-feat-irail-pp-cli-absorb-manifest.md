# iRail CLI — Absorb Manifest

Run: `20260724-212701-ae5b4630` · API: iRail (`https://api.irail.be`) · Slug: `irail`

## Tools surveyed (complete ecosystem)

| Tool | Type | Reach | Source read |
|---|---|---|---|
| `Kaya-Sem/commandtrein` | Go/Cobra CLI | 21★, active 2025-12 | ✅ all `cmd/*.go` |
| `clirail` (framagit Midgard) | Python CLI | PyPI 1.7.3 | ✅ full package metadata + README |
| `HansF/irail-mcp` | Python MCP server | active 2026-02, on LobeHub | ✅ `server.py` tool definitions |
| Raycast "NMBS Planner" | Raycast extension | store listing | listing only (GUI, not scriptable) |
| `sncb-nmbs-train-search` | npm package | Snyk listing | listing only (thin search wrapper) |
| `dedene-irail` | OpenClaw skill | clawskills.sh | listing only (skill, no binary) |
| Apify `nmbs-scraper` | hosted scraper | Apify store | listing only (paid platform) |
| iRail API itself | 8 endpoints | live-verified 2026-07-24 | ✅ every endpoint probed |

No Claude Code plugin and no Go/agent-native CLI exists for this API.

---

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Liveboard: trains departing a station | commandtrein `timetable`, clirail, irail-mcp `get_liveboard` | `(generated endpoint) board` | `--json/--agent/--select` narrowing; `observe`/`changes` re-emit the same rows typed (iRail sends every scalar as a string) |
| 2 | Arrivals instead of departures | commandtrein `-a`, irail-mcp `arrdep` | `(behavior in irail-pp-cli board) --arrivals maps to arrdep=arrival` | Same board shape either direction |
| 3 | Route planning A→B with transfers | commandtrein root cmd, clirail, irail-mcp `find_connections` | `(generated endpoint) route` | Per-leg typed delay, via/transfer detail, intermediate stops |
| 4 | Arrive-by planning | irail-mcp `timesel` | `(behavior in irail-pp-cli route) --arrive-by maps to timesel=arrival` | Pairs with `leave-by` transcendence cmd |
| 5 | Transport-type filter | iRail `typeOfTransport` | `(behavior in irail-pp-cli route) --transport automatic\|trains\|nointernationaltrains\|all` | Enum-validated instead of raw passthrough |
| 6 | Fuzzy station search | commandtrein `search`, clirail fuzzy, irail-mcp `search_stations` | `irail-pp-cli stations search` | Offline FTS over name + fr/nl/de/en aliases + telegraph + TAF/TAP codes — widest resolver of any tool |
| 7 | Telegraphic station codes (FR, FBMZ, MWL) | clirail | `(behavior in irail-pp-cli stations search) resolves and reverse-resolves 566 telegraph codes` | clirail accepts them; we also display and reverse them |
| 8 | Full station list | iRail `/stations`, irail-mcp bundled `stations.json` | `(generated endpoint) stations` | Works offline after `sync`, enriched beyond the API's 6 fields |
| 9 | Disturbances / network issues | commandtrein `issues`, irail-mcp `get_disturbances` | `(generated endpoint) disruptions` | Splits `disturbance` (real) from `planned` (works) — live sample was 4 vs 28 |
| 10 | Train info: stops + live delays | irail-mcp `get_train_info` | `(generated endpoint) train get` | Typed stop trace; documents that upstream ignores `date` |
| 11 | Train composition (carriages) | **no competitor implements this** | `(generated endpoint) train composition` | First tool to expose `/composition`, incl. `data=all` raw mode |
| 12 | Saved route shortcuts | commandtrein `shortcut add/list` | `irail-pp-cli saved` | Persisted in SQLite, consumable by every other command |
| 13 | Natural-language dates | commandtrein `-d` (NL: morgen, overmorgen, weekdays) | `(behavior in irail-pp-cli route) --date accepts tomorrow/morgen/demain/mon/2026-07-25/+2d` | 4 languages + ISO + relative, vs commandtrein's Dutch-only |
| 14 | Time selection | commandtrein `-t`, clirail `<moment>` | `(behavior in irail-pp-cli route) --time accepts 08:12, 0812, +30m` | **Fixes clirail's documented bug**: a bare past time rolls to tomorrow instead of silently meaning today |
| 15 | Timeliness snapshot across stations | clirail bare invocation | `irail-pp-cli punctuality` | clirail computes it live and throws it away; ours persists (see transcendence #2) |
| 16 | Occupancy / crowding display | iRail `occupancy` field | `(behavior in irail-pp-cli board) occupancy shown as low\|medium\|high\|unknown` | No competitor surfaces it |
| 17 | Submit occupancy feedback | iRail `POST /feedback/occupancy` | `irail-pp-cli occupancy report` | Only tool exposing the write endpoint; `--dry-run` by default |
| 18 | Alerts on trains/routes | iRail `alerts` param | `(behavior in irail-pp-cli board) --alerts includes railworks notices` | Typed alert array |
| 19 | Plain/simple output mode | commandtrein `--simple` | `(behavior in irail-pp-cli board) --quiet and auto table/JSON by TTY` | Framework-standard |
| 20 | Multi-language output | iRail `lang` | `(behavior in irail-pp-cli board) --lang nl\|fr\|en\|de` | Enum-validated |
| 21 | Shell completions | commandtrein bash completions | `(generated endpoint) completion` | bash/zsh/fish/powershell, not just bash |
| 22 | Raw request log feed | iRail `/logs` | `(generated endpoint) logs` | Covered for completeness; documented as returning `[]` in practice |
| 23 | Offline station bundle | irail-mcp bundles `stations.json` + CI refresh | `(behavior in irail-pp-cli sync) syncs stations + facilities into SQLite` | Refreshed on demand with ETag, not frozen at package build |
| 24 | MCP agent surface | irail-mcp (5 tools) | `(behavior in irail-pp-cli mcp) whole Cobra tree mirrored as MCP tools` | Every command is a tool, not a hand-picked 5 |

**No stubs.** Every row above ships fully implemented.

---

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Transfer-risk under live delay | `transfer-risk` | 10/10 | hand-code | Joins `/v1/connections` per-leg live `delay` + `vias[].timebetween` against `official_transfer_time` from `iRail/stations` `stations.csv` (618 stations populated) to flag transfers that no longer hold | `official_transfer_time` exists in the open dataset but is unused by every tool surveyed; live connections carry per-leg delay; API returns itineraries as if delays never threaten a transfer | Use this to check whether a planned transfer survives today's delays. Do NOT use it to find routes; use 'route' for planning. |
| 2 | Route punctuality history | `punctuality` | 10/10 | hand-code | Aggregates delay/cancel observations accumulated locally by `observe` into per-train and per-route stats in SQLite | Every one of the 6 competitors is stateless request/response; iRail itself publishes nightly log archives, showing the project values accumulated data; clirail computes timeliness live then discards it | Use this for historical reliability of a train or route. Do NOT use it for today's live delay; use 'board' or 'route'. |
| 3 | Disruptions scoped to my route | `disruptions route` | 10/10 | hand-code | Resolves the stations on an A→B route, then string-matches them against `/v1/disturbances` titles/descriptions, splitting real disruptions from planned works | Live `/v1/disturbances` returned 32 national entries (4 real, 28 planned) with station names embedded in titles; competitors show only the flat national list | Use this to see disruptions affecting one journey. Do NOT use it for the whole network; use 'disruptions'. |
| 4 | Live observation capture | `observe` | 9/10 | hand-code | Writes timestamped liveboard/route observations (delay, canceled, platform, occupancy) into SQLite, building the history that powers `punctuality` and `changes` | irail-mcp bundling offline stations shows appetite for local data; no tool records live observations, making "is the 08:12 always late" unanswerable today | Use this to record what the board says right now. Do NOT use it to read data back; use 'punctuality' or 'changes'. |
| 5 | Latest safe departure | `leave-by` | 9/10 | hand-code | Queries `/v1/connections` with `timesel=arrival`, applies live per-leg delay and a safety margin, returns the last departure that still lands before the deadline | clirail's own README documents a time-reasoning bug ("11 PM planning a 7 AM route" silently means today); `timesel=arrival` exists but no tool frames it as a leave-by answer | Use this when the arrival deadline is fixed. Do NOT use it for open-ended browsing; use 'route'. |
| 6 | Station facilities & accessibility | `stations facilities` | 8/10 | hand-code | Serves `facilities.csv` (691 rows) from SQLite: wheelchair, ramp, elevator, escalators, audio induction loop, lockers, Blue-bike, bus/tram/metro links, plus ticket-desk hours for all 7 weekdays | `facilities.csv` is published open data yet used by zero tools surveyed; accessibility info is absent from the API's own `/stations` response | Use this for station amenities and step-free access. Do NOT use it to find a station by name; use 'stations search'. |
| 7 | What changed since last check | `changes` | 7/10 | hand-code | Diffs the newest `observe` snapshot against the prior one for saved routes, reporting new delays, cancellations and **platform changes** via the undocumented `platforminfo.normal == 0` flag | `platforminfo.normal` is present in live responses but absent from the 2020 docs and surfaced by no tool; platform changes were an explicit user requirement | Use this for deltas since your last check. Do NOT use it for a full current board; use 'board'. |

**All 7 are `hand-code`.** Each is ~80–150 lines of Go plus `root.go` wiring.

### Killed candidates (audit trail)

| Candidate | Why cut |
|---|---|
| Long-running `watch` daemon | Scope creep — needs a background process; descoped to single-pass `changes` |
| Crowding-trend analytics | Occupancy is crowd-sourced and sparse; folded into `observe`/`punctuality` rather than its own command |
| `compare` two routes head-to-head | Depends on history depth; useless on a fresh install with an empty store. Folded into `punctuality` |
| Standalone telegraph-code command | Real but too thin for its own command; folded into `stations search` as a resolver behavior |
| `sql` / `search` | Framework-provided by the generator, not transcendence |
| `next` one-line lookup | Thin wrapper over `route`; folded in as a limit/format mode |
| LLM summarization of disruptions | Fails the LLM-dependency kill check — replaced by mechanical station matching in `disruptions route`, output pipeable to an LLM |

---

## Scope summary

- **24 absorbed** features across 8 tools + the raw API.
- **7 transcendence** features, all hand-coded.
- Best existing tool (`commandtrein`) ships roughly 6 user-facing commands with no JSON, no persistence, and no agent surface.
