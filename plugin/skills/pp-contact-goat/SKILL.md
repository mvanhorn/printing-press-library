---
name: pp-contact-goat
description: "Printing Press CLI for contact-goat. Super LinkedIn for the terminal - search, enrich, and map warm-intro paths across LinkedIn (stickerdaniel/linkedin-mcp-server subprocess), Happenstance (Chrome cookie auth with Clerk JWT refresh), and Deepline (paid enrichment, hybrid subprocess+HTTP). Unified SQLite store powers warm-intro, coverage, and cross-source prospect commands no single tool has. Trigger phrases: 'install contact-goat', 'use contact-goat', 'run contact-goat', 'contact-goat commands', 'setup contact-goat'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["contact-goat-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/cmd/contact-goat-pp-cli@latest","bins":["contact-goat-pp-cli"],"label":"Install via go install"}]}}'
---

# contact-goat — Printing Press CLI

Super LinkedIn for the terminal - search, enrich, and map warm-intro paths across LinkedIn (stickerdaniel/linkedin-mcp-server subprocess), Happenstance (Chrome cookie auth with Clerk JWT refresh), and Deepline (paid enrichment, hybrid subprocess+HTTP). Unified SQLite store powers warm-intro, coverage, and cross-source prospect commands no single tool has.

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `contact-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/cmd/contact-goat-pp-cli@latest
   ```
3. Verify: `contact-goat-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Auth setup — log in via browser:
   ```bash
   contact-goat-pp-cli auth login
   ```
   Run `contact-goat-pp-cli doctor` to verify credentials.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/cmd/contact-goat-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add -e DEEPLINE_API_KEY=value contact-goat-pp-mcp -- contact-goat-pp-mcp
   ```
   Ask the user for actual values of required API keys before running.
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which contact-goat-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Discover commands: `contact-goat-pp-cli --help`
3. Match the user query to the best command. Drill into subcommand help if needed: `contact-goat-pp-cli <command> --help`
4. Execute with the `--agent` flag:
   ```bash
   contact-goat-pp-cli <command> [subcommand] [args] --agent
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
