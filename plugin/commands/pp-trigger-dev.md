---
description: "Printing Press CLI for Trigger.dev. Monitor runs, trigger tasks, manage schedules, and detect failures via the Trigger.dev API Capabilities include: batches, costs, deployments, envvars, failures, feedback, health, profile, queues, runs, schedules, search, waitpoints, watch. Trigger phrases: 'install trigger-dev', 'use trigger-dev', 'run trigger-dev', 'Trigger.dev commands', 'setup trigger-dev'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-trigger-dev` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `trigger-dev-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `trigger-dev-pp-cli` command and execute.
