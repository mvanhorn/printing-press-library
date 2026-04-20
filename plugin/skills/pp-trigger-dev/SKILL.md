---
name: pp-trigger-dev
description: "Printing Press CLI for Trigger.dev. Monitor runs, trigger tasks, manage schedules, and detect failures via the Trigger.dev API Trigger phrases: 'install trigger-dev', 'use trigger-dev', 'run trigger-dev', 'Trigger.dev commands', 'setup trigger-dev'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["trigger-dev-pp-cli"],"env":["TRIGGER_SECRET_KEY"]},"primaryEnv":"TRIGGER_SECRET_KEY","install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/cmd/trigger-dev-pp-cli@latest","bins":["trigger-dev-pp-cli"],"label":"Install via go install"}]}}'
---

# Trigger.dev — Printing Press CLI

Monitor runs, trigger tasks, manage schedules, and detect failures via the Trigger.dev API

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `trigger-dev-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/cmd/trigger-dev-pp-cli@latest
   ```

   If `@latest` installs a stale build (the Go module proxy cache can lag the repo by hours after a fresh merge), install from main directly:
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/cmd/trigger-dev-pp-cli@main
   ```
3. Verify: `trigger-dev-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Auth setup — set the API key and register it with the CLI:
   ```bash
   export TRIGGER_SECRET_KEY="your-key-here"
   trigger-dev-pp-cli auth set-token
   ```
   Run `trigger-dev-pp-cli doctor` to verify credentials.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/cmd/trigger-dev-pp-mcp@latest
   ```

   If `@latest` installs a stale build, install from main directly:
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/cmd/trigger-dev-pp-mcp@main
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add -e TRIGGER_SECRET_KEY=value trigger-dev-pp-mcp -- trigger-dev-pp-mcp
   ```
   Ask the user for actual values of required API keys before running.
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which trigger-dev-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Discover commands: `trigger-dev-pp-cli --help`
3. Match the user query to the best command. Drill into subcommand help if needed: `trigger-dev-pp-cli <command> --help`
4. Execute with the `--agent` flag:
   ```bash
   trigger-dev-pp-cli <command> [subcommand] [args] --agent
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
