---
description: "Printing Press CLI for Slack. Send messages, search conversations, monitor channels, and manage your Slack workspace from the terminal Capabilities include: activity, analytics, bots, conversations, digest, dnd, emoji, feedback, files, funny, health, messages, pins, profile, quiet, reactions, reminders, response-times, search, stars, tail, team, threads-stale, trends, usergroups, users. Trigger phrases: 'install slack', 'use slack', 'run slack', 'Slack commands', 'setup slack'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-slack` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `slack-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `slack-pp-cli` command and execute.
