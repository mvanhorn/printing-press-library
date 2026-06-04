# AgentPool CLI

## Intent

This Printing Press CLI is a catalog-native companion for the official AgentPool
Python CLI. It must remain a facade: check prerequisites, print onboarding
guidance, and delegate to the official `agentpool` binary. It must not
reimplement AgentPool provider detection, usage parsing, SQLite state, session
lifecycle, MCP tools, model catalogs, or safety policy.

## Commands

- `install` - Show or run the official `uv tool install agentpool-cli` install and upgrade flow.
- `doctor` - Check that the official `agentpool` binary is available, then delegate to `agentpool doctor --privacy`.
- `usage` - Delegate to `agentpool usage-summary --json`; pass any extra flags to the official CLI.
- `skill` - Print official AgentPool agent guidance via `agentpool skills get agentpool`.
- `mcp-config` - Delegate host configuration generation to `agentpool mcp-config`.
- `exec` - Pass arguments through to the official `agentpool` binary unchanged.
