---
description: "Printing Press CLI for flightgoat. Free Google Flights search, Kayak nonstop route explorer, and optional FlightAware live tracking in one CLI. No API key required for search. Capabilities include: aircraft, aircraft-bio, airports, alerts, analytics, cheapest-longhaul, compare, dates, digest, disruption-counts, eta, explore, feedback, flights, foresight, gf-search, heatmap, history, longhaul, monitor, ontime-now, operators, profile, reliability, resolve, schedules, search, tail. Trigger phrases: 'install flightgoat', 'use flightgoat', 'run flightgoat', 'flightgoat commands', 'setup flightgoat'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-flightgoat` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `flightgoat-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `flightgoat-pp-cli` command and execute.
