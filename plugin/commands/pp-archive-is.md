---
description: "Printing Press CLI for Archive.today. Bypass paywalls and look up web archives via archive.today. Hero command: find or create an archive for any URL with lookup-before-submit, Wayback Machine fallback, and agent-hints on stderr when called non-interactively. Capabilities include: bulk, captures, feedback, feeds, get, history, profile, read, request, save, snapshots, tldr. Trigger phrases: 'install archive-is', 'use archive-is', 'run archive-is', 'Archive.today commands', 'setup archive-is'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-archive-is` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `archive-is-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `archive-is-pp-cli` command and execute.
