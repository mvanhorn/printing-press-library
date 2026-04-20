---
description: "Printing Press CLI for Cal.com. Manage bookings, event types, schedules, and availability via the Cal.com API Trigger phrases: 'install cal-com', 'use cal-com', 'run cal-com', 'Cal.com commands', 'setup cal-com'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-cal-com` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `cal-com-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `cal-com-pp-cli` command and execute.
