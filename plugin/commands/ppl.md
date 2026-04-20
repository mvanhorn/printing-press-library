---
description: "Discover, install, and use Printing Press CLI tools and MCP servers for any API. Use when the user wants to browse available CLIs, install a CLI or MCP server, run a Printing Press CLI command, or search for a tool by topic."
argument-hint: "<query> | <cli-name> <query> | install <name> cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `ppl` skill with the user's arguments: $ARGUMENTS

The skill handles discovery (find a CLI by topic), installation (CLI or MCP server), and routing to specific pp-* CLIs. If the user passes no arguments, show the catalog summary; if they pass a CLI name, delegate to that CLI; otherwise treat the input as a discovery query.
