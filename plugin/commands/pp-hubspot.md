---
description: "Printing Press CLI for HubSpot. Manage HubSpot CRM contacts, companies, deals, tickets, engagements, pipelines, and associations with offline search and pipeline analytics Trigger phrases: 'install hubspot', 'use hubspot', 'run hubspot', 'HubSpot commands', 'setup hubspot'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-hubspot` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `hubspot-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `hubspot-pp-cli` command and execute.
