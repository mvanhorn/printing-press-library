# Absorb Manifest — Delta Trip Management CLI
# Run: 20260520-210157 | Stage: Phase 1.5 Ecosystem Absorb

## Sources Surveyed

76+ tools catalogued across: Delta-specific scrapers, airline CLI tools, flight MCP servers,
boarding pass parsers, check-in automation bots, email confirmation parsers, and the existing
pp-flight-goat Printing Press CLI.

---

## Feature Inventory (All Existing Tools)

### Category 1: Trip Lookup & Display

| Feature | Source | Priority |
|---|---|---|
| Lookup trip by confirmation + name (no login) | Scrape-Delta-Flights, Delta web surface | P0 |
| Full itinerary display: all flights under one confirmation | flight-utils, flight-reservation-emails | P0 |
| Flight number, origin, destination, departure/arrival times | All flight tools | P0 |
| Aircraft type and operator (marketed vs. operated-by) | Scrape-Delta-Flights | P0 |
| Seat assignment per passenger per flight | Scrape-Delta-Flights | P0 |
| Cabin class / fare brand (Main Classic, Premium Select, etc.) | Delta web surface | P0 |
| Boarding group / zone | auto-southwest-check-in, Delta web surface | P0 |
| Terminal and gate info (when available) | flight-utils | P1 |
| Layover duration between legs | Delta web surface | P1 |
| On-time status per flight | Delta web surface | P1 |
| Trip total duration | flight-utils | P1 |
| eTicket numbers per passenger | Delta web surface | P1 |
| SkyMiles membership indicator | Delta web surface | P2 |

### Category 2: Seat Management

| Feature | Source | Priority |
|---|---|---|
| View current seat assignment | Scrape-Delta-Flights | P0 |
| View upgrade eligibility (Medallion status, miles, cash) | Delta web surface | P1 |
| Seat map browsing (available/unavailable per class) | United upgrade monitor | P1 |
| Fare brand feature comparison (what's included) | Delta web surface | P1 |
| Upgrade cost in miles/USD | united-upgrade-monitor | P2 |
| Accessible services add/view | Delta web surface | P2 |

### Category 3: Check-in Automation

| Feature | Source | Priority |
|---|---|---|
| Check-in window status (24-hour awareness) | auto-southwest-check-in | P0 |
| Automated check-in at T-24h | auto-southwest-check-in | P1 |
| Boarding pass download / display | boarding-pass-scanner, iata_bcbp | P1 |
| IATA BCBP barcode parsing (M1/M2 format) | boarding-pass-scanner (Python), iata_bcbp (Rust) | P2 |
| Push notification when check-in opens | auto-southwest-check-in | P2 |

### Category 4: Baggage

| Feature | Source | Priority |
|---|---|---|
| Baggage allowance per fare class | Delta web surface | P1 |
| Checked bag count | Delta web surface | P1 |
| Bag tracking by tag or confirmation | Delta web surface | P2 |
| Overweight/oversized fee lookup | Delta web surface | P2 |

### Category 5: Offline Cache & Data Layer

| Feature | Source | Priority |
|---|---|---|
| SQLite local cache of looked-up trips | pp-flight-goat pattern | P0 |
| Cache TTL / stale-while-revalidate | pp-flight-goat | P0 |
| FTS search: query cached trips by name, route, date | pp-flight-goat, flight-goat | P1 |
| Multiple trips stored (not just last) | pp-flight-goat | P1 |
| Export cached trip to JSON/CSV | flight-utils | P2 |
| Import from email confirmation (iCalendar / PDF) | AwardWallet, flight-reservation-emails | P2 |

### Category 6: Agent-Native & Output

| Feature | Source | Priority |
|---|---|---|
| `--json` flag on all commands | pp-flight-goat, Duffel MCP | P0 |
| Machine-readable error codes | pp-flight-goat | P0 |
| Pipe-friendly output (tabular by default, JSON with flag) | All Printing Press CLIs | P0 |
| MCP server mode (expose trip data as MCP tools) | Duffel MCP, Amadeus MCP | P2 |

### Category 7: Monitoring & Alerting

| Feature | Source | Priority |
|---|---|---|
| Watch mode for flight status changes | united-upgrade-monitor | P2 |
| Webhook / cron hook for check-in timer | auto-southwest-check-in | P2 |
| Price drop alerts (fare class changes) | AwardWallet | P2 |

---

## Gap Analysis (What No Existing Tool Does)

1. **Multi-airline itinerary in one command** — HPASD7 has DL + KL flights; no tool unifies Delta + partner flights from one confirmation
2. **Per-passenger-per-flight breakdown** — no tool shows "passenger A is in 15C, passenger B is in 15D on leg 1; A in 21C, B in 20C on leg 2" in a single view
3. **Fare features diff** — no tool diffs what's included in Main Classic vs. Premium Select for the same trip
4. **Offline-first with reachability awareness** — tools either always hit the web or are fully offline; none combine both with graceful fallback
5. **Structured layover analysis** — no tool flags risky connections (<60 min) with MCT awareness per airport
6. **Codeshare clarity** — KL0571 operated by KLM on a Delta ticket; no tool decodes and labels this in plain language
7. **Trip document summary** — eTickets, SkyMiles, and per-passenger passport document requirements in one `trip show` output

---

## Transcendence Features (Novel — Not in Any Existing Tool)

1. **`delta-trip watch CONF FIRST LAST`** — Live polling daemon: monitors gate assignments, on-time status, and upgrade eligibility changes; prints diffs as they happen (background process with ctrl-c to stop)
2. **`delta-trip checkin schedule CONF FIRST LAST`** — Installs a system cron job that auto-checks in at exactly T-24h per passenger; outputs boarding pass PDF URL on completion
3. **`delta-trip codeshare explain CONF`** — Explains codeshare/interline segments in plain language: "Flight 3 is ticketed by Delta (DL) but operated by KLM (KL). Your luggage transfers automatically. Delta Sky Club access does NOT apply on KL metal."
4. **`delta-trip layover risk CONF`** — Scores each connection: MCT for the airport, current on-time probability from historical data, and a plain-language risk rating (LOW / MEDIUM / HIGH) with recommended action
5. **`delta-trip compare CONF --flight N`** — Side-by-side fare feature table for your current fare class vs. next class up: cost to upgrade, what you gain, seat pitch/width delta
6. **`delta-trip export CONF --format ics`** — Exports the full itinerary as an iCalendar file with per-flight events, gate/terminal info when available, and reminders (check-in T-24h, departure T-2h)
7. **`delta-trip serve`** — MCP server mode: exposes `get_trip`, `get_flight_details`, `get_seat_info`, `get_checkin_status` as MCP tools so Claude or other agents can query trip data directly

---

## Shipping Scope

### P0 (must ship)
- Trip lookup by confirmation + first + last name
- All-flight display with per-leg: number, route, aircraft, times, seats per passenger, fare class
- Flight details expansion (gate/terminal, boarding group, layover duration, on-time status)
- eTicket numbers per passenger
- Codeshare clarity (operator vs. marketed carrier)
- JSON output on all commands
- SQLite offline cache with TTL

### P1 (ship if clean)
- Check-in window status (open/closed + opens-at timestamp)
- Baggage allowance display
- FTS search of cached trips
- Layover duration + risk rating
- Fare feature list per flight
- Upgrade eligibility indicator

### P2 (v0.2 stretch)
- Watch mode (live status polling)
- Auto check-in scheduler (cron)
- iCal export
- Boarding pass BCBP parsing
- Full bag tracking
- MCP server mode
- Fare class upgrade comparison table

---

## Absorb Manifest Signature

Sources: 76+ tools
P0 features committed: 12
P1 features committed: 10
Transcendence features proposed: 7
Generated: 2026-05-20
