# AgentPool Printed CLI Agent Guide

This directory contains the generated `agentpool-pp-cli` Printing Press companion
for AgentPool. It is intentionally a facade around the official Python
`agentpool` CLI, not a second AgentPool implementation.

## Local Operating Contract

Start by checking the wrapper and the official CLI:

```bash
agentpool-pp-cli --version
agentpool-pp-cli doctor
```

Use the wrapper only for Printing Press-native onboarding and delegation:

```bash
agentpool-pp-cli install
agentpool-pp-cli usage
agentpool-pp-cli skill
agentpool-pp-cli mcp-config
agentpool-pp-cli exec preferences
```

If the official `agentpool` binary is not installed, use:

```bash
uv tool install agentpool-cli
```

If it is installed in a custom location, set `AGENTPOOL_BIN=/path/to/agentpool`.

## Facade Boundary

Do not add provider detection, usage parsing, SQLite access, session lifecycle,
MCP server behavior, model catalogs, browser scraping, credential storage,
merge/push behavior, model ranking, or `provider=auto` logic here. Those
belong in the official AgentPool CLI. This wrapper should delegate to
`agentpool` so Printing Press users always experience the current official
AgentPool behavior.

## Local Customizations

If this CLI is edited beyond the printed facade, record code-level
customizations in `.printing-press-patches/` following the root repository
contract. README and SKILL copy updates do not need patch entries.
