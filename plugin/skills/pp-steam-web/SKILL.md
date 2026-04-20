---
name: pp-steam-web
description: "Printing Press CLI for Steam Web. Look up Steam players, games, achievements, friends, and stats from the command line Trigger phrases: 'install steam-web', 'use steam-web', 'run steam-web', 'Steam Web commands', 'setup steam-web'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["steam-web-pp-cli"],"env":["STEAM_WEB_API_KEY"]},"primaryEnv":"STEAM_WEB_API_KEY","install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/cmd/steam-web-pp-cli@latest","bins":["steam-web-pp-cli"],"label":"Install via go install"}]}}'
---

# Steam Web — Printing Press CLI

Look up Steam players, games, achievements, friends, and stats from the command line

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `steam-web-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/cmd/steam-web-pp-cli@latest
   ```

   If `@latest` installs a stale build (the Go module proxy cache can lag the repo by hours after a fresh merge), install from main directly:
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/cmd/steam-web-pp-cli@main
   ```
3. Verify: `steam-web-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Auth setup — set the API key and register it with the CLI:
   ```bash
   export STEAM_WEB_API_KEY="your-key-here"
   steam-web-pp-cli auth set-token
   ```
   Run `steam-web-pp-cli doctor` to verify credentials.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/cmd/steam-web-pp-mcp@latest
   ```

   If `@latest` installs a stale build, install from main directly:
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/cmd/steam-web-pp-mcp@main
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add -e STEAM_WEB_API_KEY=value steam-web-pp-mcp -- steam-web-pp-mcp
   ```
   Ask the user for actual values of required API keys before running.
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which steam-web-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Discover commands: `steam-web-pp-cli --help`
3. Match the user query to the best command. Drill into subcommand help if needed: `steam-web-pp-cli <command> --help`
4. Execute with the `--agent` flag:
   ```bash
   steam-web-pp-cli <command> [subcommand] [args] --agent
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
