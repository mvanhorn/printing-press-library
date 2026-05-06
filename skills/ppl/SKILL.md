---
name: ppl
description: "Discover, install, and use Printing Press CLI tools and MCP servers for any API. This skill should be used when the user wants to browse available CLIs, install a CLI or MCP server, run a Printing Press CLI command, or search for a tool by topic — including sports scores, project management, scheduling, link analytics, CRM, pizza ordering, API exploration, background jobs, and more. Trigger phrases: 'find a cli', 'install espn', 'browse tools', 'what CLIs are available', 'run espn scores', 'check the lakers score', 'look up my Linear tickets', 'shorten a link'."
argument-hint: "<query> | <cli-name> <query> | install <name> cli|mcp"
allowed-tools: "Read Bash"
---

# Printing Press Library — Unified Router

Unified router for the Printing Press CLI library. Discover CLIs, install them, configure MCP servers, and run CLI commands directly.

## Registry

The registry is at `${CLAUDE_SKILL_DIR}/references/registry.json`. Read it only when the task requires it — do not read preemptively. Fallback: read `registry.json` from the repository root.

The registry contains entries with these fields:
- `name` — short identifier (e.g., `espn`, `linear`, `dominos-pp-cli`)
- `category` — grouping (e.g., `media-and-entertainment`, `developer-tools`)
- `api` — human-readable API name (e.g., `ESPN`, `Linear`)
- `description` — what the CLI does
- `path` — repo-relative path to the CLI source (e.g., `library/media-and-entertainment/espn`)
- `mcp` (optional) — MCP server metadata: `binary`, `transports` (e.g., `["stdio"]` or `["stdio", "http"]`), `tool_count`, `auth_type`, `env_vars`

## Argument Parsing

Parse `$ARGUMENTS` to determine intent:

1. **Empty, `help`, or `--help`** → Discovery mode (show full catalog)
2. **Starts with `install`** → Install mode
   - If no CLI name follows `install`, show available CLIs and ask which to install
   - Ends with `cli` → CLI installation
   - Ends with `mcp` → MCP server installation
   - Neither → default to CLI installation
3. **Starts with `uninstall`** → Explain that uninstall is not built in; advise removing the binary manually from `$GOPATH/bin` or `$HOME/go/bin`
4. **First word exactly matches a registry entry `name`** → Explicit Use mode
5. **Anything else** → Semantic Use mode (match query against registry descriptions)

## Mode 1: Discovery

Read `${CLAUDE_SKILL_DIR}/references/registry.json`.

**No arguments:** Show a summary table of all CLIs grouped by category. For each entry, show name, API, description, auth type (from `mcp.auth_type` or "CLI only"), and MCP tool count if available.

Example output:

| Name | API | Description | Auth | MCP Tools |
|------|-----|-------------|------|-----------|
| espn | ESPN | Live scores, standings, news across 17 sports | none | 7 |
| steam-web | Steam Web | Look up players, games, achievements | api_key | 161 |
| agent-capture | agent-capture | Record and screenshot macOS windows | CLI only | — |

**Search query** (arguments that don't match a CLI name or `install`): Match the query against `name`, `api`, `description`, and `category` fields. Show matching entries in the same table format.

After showing results, suggest next steps:
- "To install: `/ppl install <name> cli`"
- "To use directly: `/ppl <name> <your question>`"

## Mode 2: CLI Installation

Triggered by: `install <name> cli` or `install <name>` (without `mcp`).

1. Read the registry and find the entry by `name`.
2. If no match, report the error and show the catalog.
3. Check Go is installed:
   ```bash
   go version
   ```
   If missing, instruct the user to install Go 1.23+.
4. Derive the CLI binary name:
   - If `name` already ends in `-pp-cli`, use it as-is (e.g., `dominos-pp-cli`)
   - Otherwise, use `<name>-pp-cli` (e.g., `espn` → `espn-pp-cli`)
5. Install via `go install`:
   ```bash
   go install github.com/mvanhorn/printing-press-library/<path>/cmd/<binary>@latest
   ```
   If the install fails with a "cannot find package" or "no Go files" error and the derived binary used the `-pp-cli` suffix, retry with the bare name:
   ```bash
   go install github.com/mvanhorn/printing-press-library/<path>/cmd/<name>@latest
   ```
6. Verify installation:
   ```bash
   <binary> --version
   ```
7. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`. If the binary is not found after install, advise adding the Go bin directory to PATH.
8. Show auth guidance:
   - If the entry has an `mcp` block with `auth_type` other than `none`, show the required environment variables from `mcp.env_vars` and explain how to set them.
   - If the entry has no `mcp` block, the registry does not contain auth metadata for this CLI. Run `<binary> auth --help` to discover auth requirements, and suggest `<binary> doctor` to verify credentials are configured correctly.

## Mode 3: MCP Server Installation

Triggered by: `install <name> mcp`.

1. Read the registry and find the entry by `name`.
2. Check the entry has an `mcp` block. If not, explain this CLI has no MCP server available and suggest CLI installation instead.
3. Check `mcp.mcp_ready` — if `partial`, inform the user that not all tools are available via MCP (show `mcp.public_tool_count` vs `mcp.tool_count`).
4. Install the MCP binary using the explicit `mcp.binary` field:
   ```bash
   go install github.com/mvanhorn/printing-press-library/<path>/cmd/<mcp.binary>@latest
   ```
5. Register with Claude Code using `claude mcp add`:
   ```bash
   claude mcp add <mcp.binary> -- <mcp.binary>
   ```
   If `mcp.env_vars` is non-empty, add each as `-e VAR=value`:
   ```bash
   claude mcp add -e VAR1=value1 -e VAR2=value2 <mcp.binary> -- <mcp.binary>
   ```
   Ask the user for the actual values of any required API keys before running the command.
6. Verify registration:
   ```bash
   claude mcp list
   ```

## Mode 4: Explicit Use

Triggered when the first word of `$ARGUMENTS` exactly matches a registry entry `name`.

1. Read the registry to confirm the entry exists and get its `path`.
2. Derive the CLI binary name (same rule as Mode 2 step 4).
3. Check if installed:
   ```bash
   which <binary>
   ```
   If the first form is not found and the `-pp-cli` suffix was appended, try the bare name:
   ```bash
   which <name>
   ```
   If neither is found, offer to install it first.
4. Discover available commands:
   ```bash
   <binary> --help
   ```
5. Match the user's remaining arguments to the best command or subcommand. Drill into subcommand help if needed:
   ```bash
   <binary> <command> --help
   ```
6. Construct and execute the command with the `--agent` flag:
   ```bash
   <binary> <command> [subcommand] [args] --agent
   ```
7. Present the results clearly. If the command fails, check the exit code and advise:
   - `0` — success
   - `2` — usage error (wrong arguments)
   - `3` — resource not found
   - `4` — authentication required (show env vars from registry)
   - `5` — API error (upstream issue)
   - `7` — rate limited (wait and retry)
   - Other codes — show stderr to the user

## Mode 5: Semantic Use

Triggered when arguments don't match a CLI name and don't start with `install`.

1. Read `${CLAUDE_SKILL_DIR}/references/registry.json`.
2. Scan each entry for relevance to the user's query. Prefer matches on `description` and `api` over `name` and `category` — a query like "track my short links" should match Dub (description: "Create short links, track analytics...") even though "short links" doesn't appear in the name or category. Use natural language understanding — e.g., "lakers score" matches ESPN ("Live scores, standings, news, and game history across 17 sports").
3. **Single strong match:** Proceed as Explicit Use (Mode 4) with that CLI.
4. **Multiple plausible matches:** Present the options with name, API, and description, and ask which the user wants.
5. **No match:** Report "No relevant CLI found in the library" and show the full catalog.

## Important Notes

- **Always use `--agent` flag** when executing CLIs. It sets `--json --compact --no-input --no-color --yes` for structured, token-efficient output.
- **Binary name derivation** is a two-step heuristic: if `name` ends in `-pp-cli`, use as-is; otherwise try `<name>-pp-cli` first, fall back to bare `<name>`. This handles naming inconsistencies in the registry.
- **Go 1.23+ required** for all installations.
- **All install paths** are constructed from registry fields: `github.com/mvanhorn/printing-press-library/<path>/cmd/<binary>@latest`. Do not hardcode paths.
- **MCP binary names** are always explicit in `mcp.binary` — no derivation needed for MCP installs.
- **Exit codes** follow a shared convention across all Printing Press CLIs. See Mode 4 step 7 for the full list.
