---
description: "Printing Press CLI for Cross-site recipe aggregator (15 trusted sites) + USDA FoodData Central. Find the best version of any recipe across 15 trusted sites — rank by trust × rating × reviews, build a local SQLite cookbook with pantry match, meal plans, cook log, substitutions, and USDA-backed nutrition backfill Trigger phrases: 'install recipe-goat', 'use recipe-goat', 'run recipe-goat', 'Cross-site recipe aggregator (15 trusted sites) + USDA FoodData Central commands', 'setup recipe-goat'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-recipe-goat` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `recipe-goat-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `recipe-goat-pp-cli` command and execute.
