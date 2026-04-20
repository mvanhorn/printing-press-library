---
name: pp-pagliacci-pizza
description: "Printing Press CLI for Pagliacci Pizza. Order pizza, browse menus, manage rewards, and track deliveries from Pagliacci Pizza Trigger phrases: 'install pagliacci-pizza', 'use pagliacci-pizza', 'run pagliacci-pizza', 'Pagliacci Pizza commands', 'setup pagliacci-pizza'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["pagliacci-pizza-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci-pizza/cmd/pagliacci-pizza-pp-cli@latest","bins":["pagliacci-pizza-pp-cli"],"label":"Install via go install"}]}}'
---

# Pagliacci Pizza — Printing Press CLI

Order pizza, browse menus, manage rewards, and track deliveries from Pagliacci Pizza

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `pagliacci-pizza-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci-pizza/cmd/pagliacci-pizza-pp-cli@latest
   ```

   If `@latest` installs a stale build (the Go module proxy cache can lag the repo by hours after a fresh merge), install from main directly:
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci-pizza/cmd/pagliacci-pizza-pp-cli@main
   ```
3. Verify: `pagliacci-pizza-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Auth setup — log in via browser:
   ```bash
   pagliacci-pizza-pp-cli auth login
   ```
   Run `pagliacci-pizza-pp-cli doctor` to verify credentials.

## MCP Server Installation

> **Note:** Not all tools are available via MCP (21 of 38 tools exposed).

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci-pizza/cmd/pagliacci-pizza-pp-mcp@latest
   ```

   If `@latest` installs a stale build, install from main directly:
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci-pizza/cmd/pagliacci-pizza-pp-mcp@main
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add pagliacci-pizza-pp-mcp -- pagliacci-pizza-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pagliacci-pizza-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Discover commands: `pagliacci-pizza-pp-cli --help`
3. Match the user query to the best command. Drill into subcommand help if needed: `pagliacci-pizza-pp-cli <command> --help`
4. Execute with the `--agent` flag:
   ```bash
   pagliacci-pizza-pp-cli <command> [subcommand] [args] --agent
   ```
5. The `--agent` flag sets `--json --compact --no-input --no-color --yes` for structured, token-efficient output.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
