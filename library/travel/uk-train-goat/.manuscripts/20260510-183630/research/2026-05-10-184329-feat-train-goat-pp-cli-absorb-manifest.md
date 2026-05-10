# uk-train-goat Absorb Manifest

The GOAT CLI does not "find gaps." It absorbs every feature from every existing
UK National Rail tool, beats them all with offline + agent-native + SQLite, and
then transcends with compound use cases nobody else ships.

This manifest is the v0.1 build contract.

## Tools surveyed (Step 1.5a / 1.5a.5 / 1.5a.6)

| Tool | Type | Source | Notes |
|------|------|--------|-------|
| `caminad/ldb-cli` | Node CLI (npm) | https://github.com/caminad/ldb-cli | Direct CLI competitor; richest command surface for OpenLDBWS-backed boards |
| `lucygoodchild/mcp-national-rail` | MCP server | https://github.com/lucygoodchild/mcp-national-rail | Closest MCP competitor; uses Realtime Trains API (different data source) |
| `mattsalt/national-rail-darwin` | JS SOAP wrapper + CLI | https://github.com/mattsalt/national-rail-darwin | Generic Darwin SOAP exposure |
| `ChrisThoung/national-rail` | Python timetable script | https://github.com/ChrisThoung/national-rail | Script-based timetable queries |
| `martinsirbe/go-national-rail-client` | Go OpenLDBWS wrapper | https://github.com/martinsirbe/go-national-rail-client | The wrapper this CLI uses; not a competitor |
| `crismc/homeassistant_nationalrailtimes_integration` | Home Assistant plugin | https://github.com/crismc/homeassistant_nationalrailtimes_integration | Home automation integration; out of scope |
| `jajsilver/UK-Train-Departure-Display-NRE` | Pi display | https://github.com/jajsilver/UK-Train-Departure-Display-NRE | Hardware display; out of scope |
| `nre-darwin-py` (PyPI) | Python wrapper | https://pypi.org/project/nre-darwin-py/ | Library, not a CLI |
| `railalefan/phpOpenLDBWS` | PHP wrapper | https://github.com/railalefan/phpOpenLDBWS | Library, not a CLI |
| `markbush/openLDBWS-json` | Apps Script | https://github.com/markbush/openLDBWS-json | XML→JSON bridge |
| `jackdevey/LDBWS-Client` | Library | https://github.com/jackdevey/LDBWS-Client | Generic client |
| Trainline | Web/mobile | https://www.thetrainline.com/ | Rejected during locked-spec phase: Akamai 2026 wall |

The closest direct CLI competitor (`caminad/ldb-cli`) covers OpenLDBWS reads;
the closest MCP competitor (`lucygoodchild/mcp-national-rail`) uses RTT API
(separate credentials). No one ships a Go-native, MCP-exposed, SQLite-backed
UK rail CLI.

## Absorbed (match or beat everything that exists)

Every row below is a feature that MUST ship in v0.1.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|---------------------|-------------|
| 1 | Live departures by CRS | caminad/ldb-cli `departures`; mattsalt `departures` | `uk-train-goat board <crs>` via OpenLDBWS | `--json --select` dotted-path filter, `--csv`, MCP-exposed read-only, offline CRS lookup, typed exit codes, agent-shaped errors |
| 2 | Live arrivals by CRS | caminad/ldb-cli `arrivals` | `uk-train-goat arrivals <crs>` via OpenLDBWS | Same as #1 |
| 3 | Filter departures by destination | caminad/ldb-cli `--filter-list.crs --filter-type to` | `uk-train-goat board <crs> --dest <crs>` | Offline-resolved CRS from station name (FTS5 lookup) |
| 4 | Service status / platform / delay | caminad/ldb-cli `service`; lucygoodchild MCP `get_*_by_date` (live only) | `uk-train-goat service <serviceID>` | Real-time platform, formation, delay reason; combined with operator alerts |
| 5 | Time-offset / time-window filter | caminad/ldb-cli `--time-offset --time-window` | `uk-train-goat board <crs> --in 30m --within 30m` | Human-readable durations, accepts `--at HH:MM` for absolute |
| 6 | Next-N departures / fastest filter | caminad/ldb-cli `next departures`, `fastest departures` | Folded into `board --in/--within --num` | Simpler surface, single command |
| 7 | JSON pipe output | caminad/ldb-cli (auto-detect on pipe) | `--json --select <dotted>` everywhere | Explicit (no surprise auto-mode), dotted-path selection, `--csv` |
| 8 | Live departures via MCP | lucygoodchild/mcp-national-rail `get_live_departures` | MCP server auto-mirrors `board` from Cobra tree | OpenLDBWS source — no extra RTT credentials; one token covers everything |
| 9 | Live arrivals via MCP | lucygoodchild/mcp-national-rail `get_live_arrivals` | MCP server auto-mirrors `arrivals` | Same as #8 |
| 10 | Date-windowed planning | lucygoodchild/mcp-national-rail `get_*_by_date` | `uk-train-goat journey --date YYYY-MM-DD` | OpenLDBWS-backed (live + future-dated chained calls) |
| 11 | A→B journey search | mattsalt/national-rail-darwin (composite) | `uk-train-goat journey <from> <to>` | Chained OpenLDBWS calls + at-most-one-change filter; agent-resolvable from station names |
| 12 | Fare lookup A→B | (none — Trainline blocked) | `uk-train-goat fare <from> <to>` via NR website scrape | Experimental flag, isolated `internal/farescrape` package, swappable |
| 13 | Saved route management | None (gap in every competitor) | `uk-train-goat saved add|list|rm` | Local SQLite plumbing for saved-commute features below |
| 14 | Health / reachability | (generator surface) | `uk-train-goat doctor` | OpenLDBWS reachability + token validity + DB schema check |

Every absorbed row maps to a real `// pp:client-call` (rows 1-12) or local
SQLite read/write (rows 13-14). No hand-rolled response builders. No fabricated
data on API unreachability — surface the failure with a clean exit code.

## Transcendence (only possible with our approach)

8 novel features from the Phase 1.5 Step 1.5c.5 subagent. All score ≥7/10 on
the absorb-scoring rubric. Each cites the persona served and the specific
mechanism that makes it impossible for a thin wrapper to deliver.

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Saved-commute one-shot | `uk-train-goat go <name>` | 10/10 | Joins local `saved_routes` row with `GetDepartureBoard` + `--dest` filter, plus `--in/--within` from the saved time-window. | Daily-commuter Dani: 10 identical home→office lookups/week; brief locks `saved_routes` table; "site doesn't remember the route" frustration. |
| 2 | Offline station search | `uk-train-goat stations --search <q>` | 10/10 | Pure SQLite FTS5 over the local `stations` table (~2,580 rows); zero network. | Agent-in-the-loop Aria: every query needs CRS resolution; brief locks FTS5 spine for the eval grader; Trip-planner Tara iterative planning. |
| 3 | Saved-routes morning status | `uk-train-goat saved status` | 9/10 | Fan-out: for each row in `saved_routes`, parallel-call `GetDepartureBoard --dest`; merge into one ranked status table. | Dani persona: 3-4 disruption diagnoses/week; brief locks `saved_routes` table; cross-entity join (saved × live boards) no single API call returns. |
| 4 | Service delay-reason briefing | `uk-train-goat why <serviceID>` | 9/10 | Composes `GetServiceDetails` (delay/cancel reasons) with adjacent `Nrcc.Messages` operator alerts into a one-screen explanation. | Field-engineer Frank: mid-trip platform/delay diagnosis on flaky 3G; brief calls out `delayReason`/`cancelReason` as `service` payload; OpenLDBWS spec exposes `Nrcc.Messages`. |
| 5 | Eval-grader 9th quality gate | `uk-train-goat eval` (CI) | 8/10 | Fixture suite of NL→tool mappings; mechanical scorer; 80% pass threshold gated behind `EVAL_AGENT_MODEL`. | Brief's locked User Vision; Aria persona's tool-selection cost; AGENTS.md framework supports custom gates. |
| 6 | Disruption-aware journey ranking | `uk-train-goat journey A B --rank` | 7/10 | Runs `journey` then per-result `GetServiceDetails`; ranks by (scheduled-time, current-delay, platform-known) so on-time-but-later beats earlier-but-late. | Tara persona: 10-20 iterative lookups/session; brief lists `journey` as Workflow #2; service-specific delay content. |
| 7 | Recent-journey replay | `uk-train-goat recent` | 7/10 | Reads last 5 rows from `search_history` and re-fires each as a live `journey`; renders side-by-side. | Tara persona iterative pattern (try X, try X+1); brief locks `search_history` table; cross-entity local × live join. |
| 8 | Multi-origin board fan-out | `uk-train-goat board PAD,KGX,EUS` | 7/10 | Parallel `GetDepartureBoard` per CRS; merge by absolute departure time into one ranked list, dedup by serviceID. | Frank persona: rotates between London terminals on disruption days; cross-source merge no single OpenLDBWS call provides. |

## Scope deltas vs the locked design spec

The locked design spec (`docs/superpowers/specs/2026-05-10-uk-train-goat-cli-design.md`)
listed: `board`, `arrivals`, `journey`, `service`, `fare`, `stations`,
`saved add|list|rm`, `sync`, `auth login`, plus the standard generator surface.

The transcendence list adds **6 features not in the locked spec**, plus a
flag-level extension and a comma-list extension:

| New | Type | Risk |
|-----|------|------|
| `go <name>` | New top-level command (saved-route one-shot) | Low — pure local-store + existing `board` reuse |
| `saved status` | New `saved` subcommand | Low — fan-out over existing `board` |
| `why <serviceID>` | New top-level command (delay briefer) | Low — composes existing `service` payload + `Nrcc.Messages` |
| `recent` | New top-level command (history replay) | Low — reads `search_history` + reuses `journey` |
| `eval` | New top-level command (LOCKED in spec) | Locked — already approved |
| `--rank` | New flag on existing `journey` | Low |
| `<crs1>,<crs2>,...` | Comma-list arg on existing `board` | Low |

These are user-facing scope additions; per the advisor's guidance, surface
them at the Phase Gate so the user can approve, trim, or modify. None of these
violate the locked decisions (no Trainline scrape, no Darwin streaming feed,
no booking, no LLM-judge eval) — they are additive to v0.1 if approved.

## Anti-reimplementation compliance

| Command | Backing | Annotation |
|---|---|---|
| `board`, `arrivals`, `journey`, `service` | OpenLDBWS via `martinsirbe/go-national-rail-client` | `// pp:client-call` |
| `fare` | `nationalrail.co.uk` HTML scrape (`internal/farescrape`) | `// pp:client-call` |
| `go`, `saved status`, `recent`, `journey --rank`, `board CRS1,CRS2` | Composes existing client calls + local store reads | `// pp:client-call` (the API call sites it composes) |
| `why <serviceID>` | `GetServiceDetails` + `Nrcc.Messages` composition | `// pp:client-call` |
| `stations --search`, `saved` (CRUD), `sync` | Local SQLite (FTS5 read; CRUD; bulk import) | (no annotation — reads from `internal/store`) |
| `eval` | Drives in-process MCP client; reads fixtures from `internal/evals/dataset/` | (no annotation — test infrastructure) |

No hand-rolled response builders. No fabricated "next train" data on API
unreachability — surface the failure verbatim under `--debug`, clean human
message otherwise.

## MCP exposure

Default-expose all user-facing commands. Per-command annotations:

| Command | `mcp:read-only` | `mcp:hidden` | Notes |
|---|---|---|---|
| `board`, `arrivals`, `journey`, `service`, `why`, `go`, `recent`, `saved status` | `true` | — | OpenLDBWS reads + local-store joins, open-world |
| `stations`, `saved list` | `true` | — | Local store reads |
| `fare` | `true` | — | External read, no mutations |
| `sync`, `saved add`, `saved rm` | — | — | Local store writes |
| `auth login` | — | `true` | Needs human input |
| `eval` | — | `true` | CI-only; not an agent tool |

No side-effect commands (no browser launches, no booking, no notifications) —
no `cliutil.IsVerifyEnv()` short-circuits required.

## Build sequence (informs Phase 3 Priority order)

- **Priority 0 (foundation):** synthetic seed spec → generator scaffold (root,
  doctor, version, auth, config, sql, search, MCP server, store skeleton).
  SQLite `stations`, `saved_routes`, `search_history` tables.
- **Priority 1 (absorbed, rows 1-14):** every absorbed row above. Each is a
  hand-authored handler in `internal/cli/` calling `internal/openldbws/` (the
  thin adapter over martinsirbe) or `internal/farescrape/`.
- **Priority 2 (transcendence, all 8 survivors):** the local-store-driven
  novel features. Eval grader (`internal/evals/`) ships here as the 9th
  quality gate.
- **Priority 3 (polish):** flag descriptions, README, SKILL.md, golden update.
