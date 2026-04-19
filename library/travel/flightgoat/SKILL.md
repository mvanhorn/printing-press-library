---
name: pp-flightgoat
description: "Use this skill whenever the user asks about flight prices, cheap-dates discovery, nonstop routes from an airport, long-haul flights, flight tracking, airport info, or wants to search Google Flights / Kayak / FlightAware from the terminal. Three-source flight CLI: free Google Flights + Kayak nonstop (no API key) plus optional FlightAware tracking. Triggers on phrasings like 'cheapest dates to Tokyo in June', 'nonstop flights from Seattle', 'track flight UA123', 'longest nonstop from SFO', 'where can I fly direct from Denver', 'compare flights SEA to LHR next Tuesday'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["flightgoat-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/travel/flightgoat/cmd/flightgoat-pp-cli@latest","bins":["flightgoat-pp-cli"],"label":"Install via go install"}]}}'
---

# Flightgoat — Printing Press CLI

Free Google Flights search + Kayak nonstop route explorer + optional FlightAware tracking in one CLI. The headline features work with **no API key**. FlightAware AeroAPI integration is optional for live tracking, alerts, historical, and aircraft details.

## When to Use This CLI

Reach for this when a user wants:

- Flight prices for a specific route/date (Google Flights, no key)
- Cheapest-dates discovery across a month (Google Flights)
- Nonstop routes from a hub airport — where CAN I fly direct from X (Kayak `/direct`, no key)
- Longest-nonstop flights from an airport (Kayak filter)
- Live flight tracking, alerts, history, or aircraft info (FlightAware, needs API key)

Don't reach for this if the user wants to actually book a flight (this is a research/discovery tool; bookings happen on the airline or OTA site).

## Unique Capabilities

The three-source architecture gives this CLI a unique niche: free broad search + deep tracking when you need it.

### Free — no API key

- **`flights <origin> <dest> <date>`** — Google Flights search with real prices, durations, legs. Uses [krisukox/google-flights-api](https://github.com/krisukox/google-flights-api) with [punitarani/fli](https://github.com/punitarani/fli) as a fallback when Google's abuse-detection demands a cookie.

  _Real prices from Google's actual search results. Works without auth. A huge win over paid flight APIs for casual trip planning._

- **`dates <origin> <dest> --from YYYY-MM-DD --to YYYY-MM-DD [--sort]`** — Cheapest-dates across a window. Exposes the calendar-grid search from Google Flights.

- **`longhaul <airport> --min-hours N`** — Every nonstop flight from an airport of at least N hours. Sourced from Kayak `/direct` parsing.

  _Verified output: `longhaul SEA --min-hours 10` returned Singapore 16h50m SQ, Dubai 15h55m EK, Doha 15h25m QR, Chongqing 14h25m HU, plus 10 others. All real routes._

- **`explore <airport> [--country <c>] [--airline <a>]`** — Every nonstop destination from an airport, filterable by country or airline. The "where can I fly direct" command.

- **`cheapest-longhaul <airport>`** — Cheapest long-haul options from a hub (combines Kayak routes + Google prices).

### Optional — FlightAware AeroAPI key

- **`flights track get-flight <id>`** — Live flight tracking by callsign or registration.

- **`alerts`** — Configure flight status alerts.

- **`history`** — Historical flight data.

- **`aircraft <type>`** — Aircraft type info.

- **`aircraft-bio <registration>`** — Individual tail number history.

- **`operators`** / **`schedules`** / **`disruptions`** / **`foresight`** — Full AeroAPI surface.

### Cross-source

- **`compare <origin> <dest> <date>`** — Side-by-side Google + Kayak + (optional) FlightAware view for one route on one date.

- **`digest <airport>`** — Morning briefing for a hub: notable routes, cheapest options, any tracked flights.

## Command Reference

### Free (Google Flights + Kayak — no key)

- `flightgoat-pp-cli flights <origin> <dest> <date>` — Priced flight search
- `flightgoat-pp-cli dates <origin> <dest>` — Cheapest-dates window
- `flightgoat-pp-cli longhaul <airport>` — Long-haul nonstops from airport
- `flightgoat-pp-cli explore <airport>` — All nonstop destinations
- `flightgoat-pp-cli cheapest-longhaul <airport>` — Combo search
- `flightgoat-pp-cli compare <origin> <dest> <date>` — Cross-source view
- `flightgoat-pp-cli digest <airport>` — Airport-centric briefing

### FlightAware (needs `FLIGHTGOAT_API_KEY_AUTH`)

- `flightgoat-pp-cli airports` — Airport info
- `flightgoat-pp-cli aircraft <type>` / `aircraft-bio <registration>` — Aircraft
- `flightgoat-pp-cli alerts` — Alert configuration
- `flightgoat-pp-cli flights track get-flight <id>` — Live tracking (via FA endpoints)

### Utility

- `flightgoat-pp-cli sync` / `export` / `import` / `archive` — Local store
- `flightgoat-pp-cli auth set-token <FLIGHTAWARE_KEY>` — Save API key
- `flightgoat-pp-cli doctor` — Verify connectivity

## Recipes

### Cheapest dates to Tokyo in June

```bash
flightgoat-pp-cli dates SEA HND --from 2026-06-01 --to 2026-06-30 --sort --agent
```

Returns a sorted list of dates + prices; one call, no per-date loop.

### Where can I fly direct from SEA?

```bash
flightgoat-pp-cli explore SEA --agent                      # all destinations
flightgoat-pp-cli longhaul SEA --min-hours 10 --agent      # long-haul only
flightgoat-pp-cli explore SEA --country JP --agent         # specific country
```

`explore` lists every nonstop; `longhaul` filters to 10h+ flights; country filter finds all Japan-bound nonstops.

### Research a specific trip

```bash
flightgoat-pp-cli flights SEA LHR 2026-06-15 --stops non_stop --agent
flightgoat-pp-cli compare SEA LHR 2026-06-15 --agent
```

The first call gets Google's priced nonstops; `compare` adds Kayak and (if configured) FlightAware context for the same route/date.

### Track a flight with FlightAware key

```bash
export FLIGHTGOAT_API_KEY_AUTH="your-aeroapi-key"
flightgoat-pp-cli flights track get-flight UA123 --agent
flightgoat-pp-cli aircraft-bio N12345 --agent     # specific tail number history
```

Requires an AeroAPI subscription. Prints live position, altitude, ETA, plus historical data on the aircraft itself.

## Auth Setup

**Headline commands need no auth.** `flights`, `dates`, `longhaul`, `explore` use Google Flights and Kayak scraping — no API key, no signup.

**FlightAware tracking optional:** Get a key at [flightaware.com/aeroapi](https://flightaware.com/aeroapi/portal/). Free tier is generous for individual use.

```bash
export FLIGHTGOAT_API_KEY_AUTH="your-aeroapi-key"
flightgoat-pp-cli auth set-token "$FLIGHTGOAT_API_KEY_AUTH"
flightgoat-pp-cli doctor
```

Optional:
- `FLIGHTGOAT_BASE_URL` — override AeroAPI base
- Google Flights fallback kicks in automatically when abuse-detection requires a cookie; no user config needed

## Agent Mode

Add `--agent` to any command. Expands to `--json --compact --no-input --no-color --yes`. Useful flags:

- `--stops non_stop | one_stop | two_plus` — filter results
- `--class economy | premium_economy | business | first` (short: `-c`) — cabin class
- `--sort price | duration | stops` — result ordering
- `--min-hours N` — minimum duration (for `longhaul`)
- `--country <CC>` — country filter (for `explore`)

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error |
| 3 | Not found (airport, flight, aircraft) |
| 4 | Auth required (for FlightAware commands) |
| 5 | Upstream error (Google blocking, Kayak unreachable, AeroAPI down) |
| 7 | Rate limited |

## Installation

### CLI

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/flightgoat/cmd/flightgoat-pp-cli@latest
flightgoat-pp-cli doctor
```


<!-- pr-218-features -->
## Agent Workflow Features

This CLI exposes three shared agent-workflow capabilities patched in from cli-printing-press PR #218.

### Named profiles

Persist a set of flags under a name and reuse them across invocations.

```bash
# Save the current non-default flags as a named profile
flightgoat-pp-cli profile save <name>

# Use a profile — overlays its values onto any flag you don't set explicitly
flightgoat-pp-cli --profile <name> <command>

# List / inspect / remove
flightgoat-pp-cli profile list
flightgoat-pp-cli profile show <name>
flightgoat-pp-cli profile delete <name> --yes
```

Flag precedence: explicit flag > env var > profile > default.

### --deliver

Route command output to a sink other than stdout. Useful when an agent needs to hand a result to a file, a webhook, or another process without plumbing.

```bash
flightgoat-pp-cli <command> --deliver file:/path/to/out.json
flightgoat-pp-cli <command> --deliver webhook:https://hooks.example/in
```

File sinks write atomically (tmp + rename). Webhook sinks POST `application/json` (or `application/x-ndjson` when `--compact` is set). Unknown schemes produce a structured refusal listing the supported set.

### feedback

Record in-band feedback about this CLI from the agent side of the loop. Local-only by default; safe to call without configuration.

```bash
flightgoat-pp-cli feedback "what surprised you or tripped you up"
flightgoat-pp-cli feedback list         # show local entries
flightgoat-pp-cli feedback clear --yes  # wipe
```

Entries append to `~/.flightgoat-pp-cli/feedback.jsonl` as JSON lines. When `FLIGHTGOAT_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `FLIGHTGOAT_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream (non-blocking — local write always succeeds).

