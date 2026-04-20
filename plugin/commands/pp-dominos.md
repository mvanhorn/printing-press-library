---
description: "Printing Press CLI for Dominos Pizza. Order pizza, browse menus, track deliveries, and manage rewards from the terminal Capabilities include: address, analytics, cart, checkout, compare-prices, deals, feedback, graphql, menu, nutrition, orders, profile, quickstart, rewards, stores, tail, template, track, tracking. Trigger phrases: 'install dominos', 'use dominos', 'run dominos', 'Dominos Pizza commands', 'setup dominos'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-dominos` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `dominos-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `dominos-pp-cli` command and execute.
