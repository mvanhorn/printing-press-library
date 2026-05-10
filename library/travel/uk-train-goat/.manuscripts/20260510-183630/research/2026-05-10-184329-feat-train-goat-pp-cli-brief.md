# uk-train-goat CLI Brief

**Distilled from the locked design spec at**
`docs/superpowers/specs/2026-05-10-uk-train-goat-cli-design.md`. The spec is the
source of truth for v0.1 scope; this brief reframes it for the absorb manifest
and skips re-research the user already locked.

## API Identity

- **Domain:** UK National Rail live data via the OpenLDBWS SOAP API (Darwin-backed).
- **Data profile:** real-time read-heavy. Departure boards turn over every
  ~30 seconds; service status and platform changes are highly time-sensitive.
  No write surface exists in v0.1 (no booking, no payments).

## Users

Four concrete personas, drawn from the locked spec's scope and the workflows
below. The CLI is an everyday morning-and-disruption tool, not a one-off
travel-planning oracle.

- **Daily-commuter Dani** — London-area commuter on a fixed home → office route.
  Today: opens the National Rail mobile site every morning, taps into the
  station's CRS, scans the next 5 departures for delays, then walks. Frustration:
  three taps and a hostile mobile UI to answer "is the 08:18 to King's Cross on
  time today?" Weekly ritual: 10x identical departure-board lookups per work
  week, plus 3-4 disruption diagnoses ("which platform now?").
- **Field-engineer Frank** — UK field engineer who SSHs from a phone or a small
  Linux box. Today: calls the train info phoneline on disruption days because
  the National Rail mobile site is JS-heavy and breaks on weak 3G. Frustration:
  no terminal-friendly way to check arrivals or service status on the road.
  Weekly ritual: 2-3 long inter-city journeys with mid-trip platform changes.
- **Trip-planner Tara** — Plans a UK trip every 6-8 weeks (visiting family,
  short holiday). Today: bounces between Trainline (slow), Google Maps
  (incomplete), and National Rail. Frustration: "next train from Reading to
  Paddington tomorrow morning" requires picking a date, scrolling, filtering.
  Weekly ritual: less frequent than the others (6-8 weeks); but each session is
  10-20 lookups in one sitting.
- **Agent-in-the-loop Aria** — An LLM assistant running inside an MCP host. A
  non-technical end-user asks "when does the next Eurostar leave St Pancras"
  or "is the 18:32 to Manchester running on time" and Aria must select the
  right tool and resolve CRS codes from station names. Today (without this
  CLI): scrapes National Rail HTML and produces flaky answers. Frustration:
  every UK rail tool exposes either no MCP surface or a verbose REST surface
  that wastes context.

- **Top-line workflow density:** Dani drives `board` and `service` calls
  ~13× per week. Frank drives `arrivals`, `service` and `saved` ~5× per week
  during disruption. Tara drives `journey` (with `--date`) in bursts of
  10-20 per session. Aria drives every command via MCP and benefits most from
  agent-eval-tuned tool descriptions.

## Reachability Risk

- **Low.** OpenLDBWS is the official, free SOAP endpoint operated by Rail
  Delivery Group; it has been stable for years and is documented at
  https://realtime.nationalrail.co.uk/OpenLDBWSRegistration/. Authentication
  is a single GUID-shaped token issued from the legacy registration page.
- **One scrape risk:** fare lookup uses an HTML scrape of nationalrail.co.uk.
  Marked **experimental** in the spec; isolated in `internal/farescrape/` so
  layout drift is contained. No fabrication on failure (per AGENTS.md
  anti-reimplementation rule and locked spec language).

## Top Workflows (locked v0.1)

1. **Live departure board for a UK station** — `uk-train-goat board PAD --num 10`.
   The most-asked question on a UK rail commute.
2. **Journey planning A → B with at most one change** — `uk-train-goat journey PAD RDG`.
   Chained OpenLDBWS calls (filter departures from A by destination = B).
3. **Service status for a specific train** — `uk-train-goat service <serviceID>`.
   Platform, formation, delay reasons. Source of "is the 18:32 to Manchester
   on time" answers.
4. **Live arrivals for a station** — `uk-train-goat arrivals KGX`. Symmetric to
   the departure board; less common but high-leverage during disruption.
5. **Best-effort fare lookup A → B** — `uk-train-goat fare PAD RDG` via
   `nationalrail.co.uk` HTML scrape, **marked experimental**.

## Table Stakes

Existing UK rail terminal/CLI tools surveyed during the locked-spec phase:

- **opentraintimes.com** (web only, no CLI) — gold standard for service
  status and Realtime Trains data; out-of-scope here.
- **No mature Go CLI competitor** for OpenLDBWS exists. The Python
  `nrcc` and JS `openldbws` packages are abandoned.
- **Trainline** rejected during locked-spec phase — Akamai 2026 posture
  (TLS fingerprinting + per-pageload JS) breaks static reverse-engineering.
  Same wall already disabled VRBO support in `pp-airbnb`.

So the table stakes for "the only Go-native UK rail CLI that exists" are
modest: any of the five workflows above shipping reliably is a step change.

## Data Layer

- **Primary entities:**
  - `stations` — CRS code → station name → coordinates (~2,580 entries; static).
  - `saved_routes` — user's named commutes (e.g. "morning" = PAD → RDG at 08:30).
  - `search_history` — recent `journey` queries for fast recall.
- **Sync cursor:** none for live boards (always live). Stations sync once
  per N days from the National Rail Knowledge Base CSV (or a one-shot
  scrape of the OpenLDBWS station enum if the CSV proves unwieldy — open
  question deferred to plan).
- **FTS/search:** SQLite FTS5 over station names so `uk-train-goat stations
  --search "kings"` returns KGX. Powers natural-language disambiguation
  in the eval grader.

## Codebase Intelligence

- **Source:** `martinsirbe/go-national-rail-client` GitHub README (read at
  Phase 1.5a.6 if more depth needed).
- **Auth:** wrapper accepts `nrc.AccessTokenOpt(token)` constructor option;
  the CLI passes `LDBWS_API_TOKEN` explicitly so the wrapper's own
  `NR_ACCESS_TOKEN` env-pickup is bypassed and irrelevant.
- **Data model:** wraps the four core OpenLDBWS SOAP operations
  (`GetDepartureBoard`, `GetArrivalBoard`, `GetServiceDetails`,
  `GetDepartureBoardWithDetails`).
- **Rate limiting:** OpenLDBWS publishes no documented rate limit; the
  wrapper does not impose one. Spec adds exponential backoff with cap and
  surfaces limit-related headers under `--debug`.
- **Architecture:** thin Go-only wrapper; ~1.5K LoC; MIT licensed; no
  transitive deps beyond `encoding/xml`. Native integration mode in the
  catalog entry is the right call.

## User Vision

User's locked vision (from spec + prior session): a UK-only, free-API,
agent-native train CLI that ships with a programmatic eval grader (no
LLM-judge in v0.1, 80% pass rate threshold gated behind `EVAL_AGENT_MODEL`).
The CLI must beat "open the National Rail website on a phone" for the
five Top Workflows above.

## Source Priority

Single-source CLI. OpenLDBWS is primary; nationalrail.co.uk fare scrape is
a secondary best-effort surface marked experimental. No combo CLI gating
required (no `source-priority.json` needed).

## Product Thesis

- **Name:** `uk-train-goat`. Matches the established `*-goat` family
  (`flight-goat`, `movie-goat`, `recipe-goat`, etc.).
- **Why it should exist:**
  1. The only Go-native, agent-native, MCP-exposed UK rail CLI.
  2. Replaces "open the National Rail website" for the five highest-traffic
     workflows.
  3. Local SQLite store gives offline station lookup and saved-commute
     recall — features the official site does not surface in any usable form.
  4. Agent-native by design — eval grader pins the MCP tool descriptions so
     LLM agents reliably resolve `journey from Paddington to Reading` to the
     right tool with the right args.

## Build Priorities

(Sized against the hybrid hand-build plan: the generator emits the standard
surface from a synthetic seed spec; novel handlers are hand-authored against
the wrapper.)

1. **Priority 0 — Foundation:** SQLite store with `stations`, `saved_routes`,
   `search_history`. Synthetic seed spec for the generator. Standard
   generator surface (root, doctor, version, auth, config, sql, search,
   MCP server scaffolding).
2. **Priority 1 — Absorbed wrapper surface:** the four OpenLDBWS-backed
   commands that match every existing UK rail tool feature: `board`,
   `arrivals`, `journey`, `service`. All annotated `// pp:client-call`.
3. **Priority 2 — Transcendence:** the local-store-driven commands that
   only work because everything is in SQLite — `stations --search`,
   `saved` (add/list/rm), `sync`. Plus the **agent-eval grader** as the
   9th quality gate at 80% threshold, gated behind `EVAL_AGENT_MODEL`.
4. **Priority 3 — Polish:** `fare` (experimental), terse-flag-description
   enrichment, README/SKILL prose, golden update for catalog list (already
   done in Phase A).

## Anti-reimplementation compliance

Per AGENTS.md and locked spec:

| Command | Backing | Annotation |
|---|---|---|
| `board`, `arrivals`, `journey`, `service` | OpenLDBWS via `martinsirbe/go-national-rail-client` | `// pp:client-call` |
| `fare` | `nationalrail.co.uk` HTML scrape (`internal/farescrape`) | `// pp:client-call` |
| `stations`, `saved`, `sync` | Local SQLite read/write | (no annotation needed; reads from `internal/store`) |

No hand-rolled response builders. No fabricated data on API unreachability —
surface the failure with a clean exit code.

## MCP exposure

Default-expose. All read commands annotated `mcp:read-only=true`. Mutating
commands (`saved add`, `saved rm`, `sync`) are exposed but unannotated.
`auth login` annotated `mcp:hidden=true` (needs human input).

No side-effect commands in v0.1, so no `cliutil.IsVerifyEnv()`
short-circuits required.

## What this brief explicitly does NOT investigate

Per the advisor: distill, don't re-investigate. Items already settled
upstream:

- Wrapper choice (martinsirbe/go-national-rail-client, locked).
- Auth env var (`LDBWS_API_TOKEN`, locked; catalog Notes is source of truth).
- Eval grader approach (programmatic, 80% threshold, behind `EVAL_AGENT_MODEL`,
  9th quality gate, locked).
- Trainline bypass (rejected, locked).
- Streaming Darwin Kafka feed (rejected for v0.1, locked).
