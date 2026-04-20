---
description: "{{.EnrichedDesc}}"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `{{.SkillName}}` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `{{.CLIBinary}} --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `{{.CLIBinary}}` command and execute.
