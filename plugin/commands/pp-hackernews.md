---
description: "Printing Press CLI for Hacker News. Browse, search, and analyze Hacker News — front page, Show HN, Ask HN, Who is Hiring, topic pulse, and pipe-friendly output Trigger phrases: 'install hackernews', 'use hackernews', 'run hackernews', 'Hacker News commands', 'setup hackernews'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-hackernews` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `hackernews-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `hackernews-pp-cli` command and execute.
