# AgentPool wrapper manuscript

This manuscript records the custom wrapper print for AgentPool.

- API slug: `agentpool`
- CLI binary: `agentpool-pp-cli`
- Category: `developer-tools`
- Printing Press version: `4.20.1`
- Official AgentPool package: `agentpool-cli`
- Official install path: `uv tool install agentpool-cli`
- Official binary: `agentpool`

## Product Decision

The Printing Press catalog entry should be a facade and onboarding layer. It
should help users discover, install, and use AgentPool from Printing Press while
delegating behavior to the official AgentPool CLI.

This avoids a shadow AgentPool implementation. The wrapper must not implement
provider detection, usage parsing, SQLite state, session lifecycle, MCP server
behavior, model catalogs, browser scraping, credential storage, model ranking,
merge/push behavior, or `provider=auto`.

## Verified Commands

```bash
agentpool-pp-cli --help
agentpool-pp-cli install
agentpool-pp-cli exec preferences
agentpool-pp-cli doctor --json
agentpool-pp-cli usage --provider fake-question
agentpool-pp-cli skill
agentpool-pp-cli mcp-config --client codex --json
```

The wrapper was validated against official `agentpool 0.1.10`.
