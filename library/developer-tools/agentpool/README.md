# AgentPool Printing Press Companion

`agentpool-pp-cli` is a small Printing Press companion for the official
AgentPool CLI. It exists so Printing Press users can discover and install
AgentPool from the catalog while the real `agentpool` binary remains the source
of truth.

Install the official AgentPool CLI:

```bash
uv tool install agentpool-cli
```

Then use the wrapper for catalog-native onboarding and delegation:

```bash
agentpool-pp-cli doctor
agentpool-pp-cli usage
agentpool-pp-cli skill
agentpool-pp-cli mcp-config
agentpool-pp-cli exec preferences
```

This wrapper does not implement AgentPool provider detection, usage parsing,
SQLite state, session lifecycle, MCP tools, model catalogs, or safety policy.
