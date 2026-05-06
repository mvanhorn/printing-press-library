---
name: {{.SkillName}}
description: "{{.EnrichedDesc}}"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
{{- if .OpenClawMeta}}
{{.OpenClawMeta -}}
{{- end}}
---

# {{.APIName}} — Printing Press CLI

{{.Description}}

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `{{.CLIBinary}} --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/{{.InstallPath}}/cmd/{{.CLIBinary}}@latest
   ```
3. Verify: `{{.CLIBinary}} --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
{{- if eq .AuthType "api_key"}}
5. Auth setup — set the API key and register it with the CLI:
   ```bash
{{- range .EnvVars}}
   export {{.}}="your-key-here"
{{- end}}
   {{.CLIBinary}} auth set-token
   ```
   Run `{{.CLIBinary}} doctor` to verify credentials.
{{- else if eq .AuthType "composed"}}
5. Auth setup — log in via browser:
   ```bash
   {{.CLIBinary}} auth login
   ```
   Run `{{.CLIBinary}} doctor` to verify credentials.
{{- end}}

{{- if .HasMCP}}

## MCP Server Installation

{{- if eq .MCPReady "partial"}}

> **Note:** Not all tools are available via MCP ({{.PublicToolCount}} of {{.ToolCount}} tools exposed).
{{- end}}

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/{{.InstallPath}}/cmd/{{.MCPBinary}}@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add{{range .EnvVars}} -e {{.}}=value{{end}} {{.MCPBinary}} -- {{.MCPBinary}}
   ```
{{- if .EnvVars}}
   Ask the user for actual values of required API keys before running.
{{- end}}
3. Verify: `claude mcp list`
{{- end}}

## Direct Use

1. Check if installed: `which {{.CLIBinary}}`
   If not found, offer to install (see CLI Installation above).
2. Discover commands: `{{.CLIBinary}} --help`
{{- if .DomainCommands}}
   Key commands:
{{- range .DomainCommands}}
   - `{{.Name}}` — {{.Description}}
{{- end}}
{{- end}}
3. Match the user query to the best command. Drill into subcommand help if needed: `{{.CLIBinary}} <command> --help`
4. Execute with the `--agent` flag:
   ```bash
   {{.CLIBinary}} <command> [subcommand] [args] --agent
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
