---
description: "Printing Press CLI for Steam Web. Look up Steam players, games, achievements, friends, and stats from the command line Capabilities include: achievements, analytics, badges, bans, feedback, friends, games, level, news, players, profile, recent, resolve, schema, search, stats, tail. Trigger phrases: 'install steam-web', 'use steam-web', 'run steam-web', 'Steam Web commands', 'setup steam-web'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-steam-web` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `steam-web-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `steam-web-pp-cli` command and execute.
