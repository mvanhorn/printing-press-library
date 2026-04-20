---
description: "Printing Press CLI for Instacart. Natural-language Instacart CLI that talks directly to the web GraphQL API. Add items to your cart, search products, and manage carts across retailers without browser automation. Capabilities include: add, capture, cart, carts, history, retailers, search. Trigger phrases: 'install instacart', 'use instacart', 'run instacart', 'Instacart commands', 'setup instacart'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-instacart` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `instacart-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `instacart-pp-cli` command and execute.
