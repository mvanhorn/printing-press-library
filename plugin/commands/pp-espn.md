---
description: "Printing Press CLI for ESPN. Live scores, standings, news, and game history across 17 sports from ESPN Capabilities include: boxscore, compare, dashboard, feedback, h2h, injuries, leaders, news, odds, plays, profile, rankings, recap, rivals, scoreboard, scores, search, sos, standings, streak, summary, teams, today, transactions, trending, watch. Trigger phrases: 'install espn', 'use espn', 'run espn', 'ESPN commands', 'setup espn'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-espn` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `espn-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `espn-pp-cli` command and execute.
