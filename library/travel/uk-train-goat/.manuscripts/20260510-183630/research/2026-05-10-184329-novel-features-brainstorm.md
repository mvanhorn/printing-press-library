# Novel Features Brainstorm — uk-train-goat (audit trail)

> Persisted from the Phase 1.5 Step 1.5c.5 subagent invocation. The Customer
> model and Killed candidates are NOT rendered into the absorb manifest —
> only the Survivors table is — but they are persisted here for retro / dogfood
> debugging per the subagent contract.

## Customer model

**Daily-commuter Dani** — London-area commuter on a fixed home → office route
(e.g., PAD → RDG, or KGX → home).

- **Today (without this CLI):** Pulls out a phone, opens the National Rail
  mobile site, types "PAD" or taps through a station picker, scans the next 5
  departures, eyeballs the row that says "08:18 to King's Cross" for
  delay/platform, then walks. On disruption mornings, also taps into individual
  service rows to read why a train is late.
- **Weekly ritual:** ~10 identical departure-board lookups per work week
  (one per outbound + return leg, weekday). Plus 3-4 disruption diagnoses per
  week ("which platform now?" / "is mine cancelled?"). Always the same 1-2
  origin stations and same 1-2 destination stations.
- **Frustration:** Three taps and a hostile JS-heavy mobile UI to answer the
  *same single question* every day: "is the 08:18 to KGX on time today, and
  from which platform?" The UI does not remember the route. Every morning
  starts from zero.

**Field-engineer Frank** — UK field engineer, SSHs from a phone or a small
Linux box to a server, often on flaky 3G in the middle of an inter-city
journey.

- **Today:** On disruption days, calls the National Rail telephone info line
  because the mobile site is JS-heavy and breaks on weak 3G. On a good signal,
  opens the mobile site and waits for it to load. Has no terminal-friendly way
  to check arrivals or service status mid-trip.
- **Weekly ritual:** 2-3 long inter-city journeys per week. Each journey has
  1-2 mid-trip platform changes or replacement-bus events that demand a
  real-time arrivals lookup at the next interchange and a service-status check
  on his current train.
- **Frustration:** No terminal-native way to do the lookup. He is sitting in a
  tmux session with full SSH access to a Linux box but cannot answer "which
  platform does the 14:32 to Manchester arrive on at Crewe" without leaving
  the terminal.

**Trip-planner Tara** — Plans a UK trip every 6-8 weeks (visiting family,
short holiday). Bursts of high-density planning sessions.

- **Today:** Bounces between Trainline (slow, ad-heavy), Google Maps
  (incomplete UK rail data, no platform info), and the National Rail site.
  Picks a date, scrolls through departure options, filters by direct/changes,
  all while juggling 2-3 candidate days for the trip.
- **Weekly ritual:** Less frequent than the others (one planning session every
  6-8 weeks), but each session is **10-20 lookups in one sitting**: try date X
  for route A→B, try date X+1 for same, try return leg, swap origin to a
  nearby station, etc.
- **Frustration:** Three sites do not agree, none is fast, and "next train
  from Reading to Paddington tomorrow morning" requires picking a date in a
  calendar widget, scrolling a list, and filtering, before she can even
  compare options. The friction kills iterative planning.

**Agent-in-the-loop Aria** — An LLM assistant running inside an MCP host
(Claude Desktop, Cursor, etc.) where a non-technical end-user asks ambient
natural-language questions like "when does the next Eurostar leave St Pancras"
or "is the 18:32 to Manchester running on time".

- **Today:** Scrapes National Rail HTML and produces flaky answers, or hits a
  verbose REST tool that wastes context window on prose, or fails to resolve
  "Paddington" → `PAD` because no offline station table is exposed.
- **Weekly ritual:** Every UK rail question routed to Aria — easily 50+/week
  per host across all users. Tool selection is a constant cost: which of a
  dozen near-identical tools does she pick?
- **Frustration:** No UK rail tool exposes a clean MCP surface with offline
  CRS resolution and tightly-scoped tool descriptions tuned for an agent eval.
  She wastes context tokens, picks wrong tools, or hallucinates CRS codes.

## Candidates (pre-cut)

(See subagent output: 16 candidates labeled (a)-(f) with inline kill/keep
verdicts. Survivors C1-C7, C16 carried into Pass 3. Plumbing C15 retained as
supporting commands; absorbed C13/C14 dropped from novel list. C8/C9/C10/C11/C12
killed.)

## Survivors and kills

### Survivors (rendered into absorb manifest's transcendence table)

8 features scoring >= 7/10. Highest: `go <name>` and `stations --search` at
10/10. See manifest for the table.

### Killed candidates (audit trail)

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C8 — Coach formation diagram (`formation <serviceID>`) | Verifiability low (formation data sparsely populated by operators); cosmetic single-endpoint render with weak weekly-use signal. | C4 — `why <serviceID>` |
| C9 — Live board watch (`board PAD --watch`) | Scope creep / thin wrapper: just a polling loop around an existing endpoint, no transcendence beyond `sleep + redraw`. | C4 — `journey --rank` |
| C10 — Return-window query (`return <a> <b> --return-after`) | Two `journey` calls glued together; no novel join or content pattern. | C5 — `recent` |
| C11 — Free-text NL parse (`ask "next train from..."`) | LLM-dependency: robust NL parsing is an LLM job. The mechanical version *is* the eval grader; doing both is duplication. | C7 — eval grader |
| C12 — Operator on-time stats (`operator GWR --stats`) | Reimplementation: `search_history` is not a statistical sample; computing operator-level on-time locally fabricates numbers. | C2 — `why <serviceID>` |
| C13 — CRS context briefing (`context stations`) | Already covered by the generator-emitted `context` framework command; not novel. | C6 — `stations --search` |
| C14 — Stations sync staleness (`stale`) | Already in the generator's framework-commands set (per AGENTS.md); not novel. | n/a — framework command |
| C15 — Saved CRUD (`saved add/list/rm`) | Plumbing, not transcendence — required to make C1/C3 work but not a leverage feature on its own. | C1 — `go <name>` |

## Reprint verdicts

N/A — first print, no prior research.json.
