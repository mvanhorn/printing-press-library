---
description: "Printing Press CLI for Open-Meteo + NWS. Weather forecasts, severe weather alerts, air quality, and GO/CAUTION/STOP activity verdicts for walk, bike, hike, commute, and drive Trigger phrases: 'install weather-goat', 'use weather-goat', 'run weather-goat', 'Open-Meteo + NWS commands', 'setup weather-goat'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-weather-goat` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `weather-goat-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `weather-goat-pp-cli` command and execute.
