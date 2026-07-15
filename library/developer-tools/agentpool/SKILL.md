---
name: pp-agentpool
description: Use the Printing Press companion for AgentPool to inspect official capacity signals, run privacy-safe diagnostics, and delegate to the official agentpool CLI.
version: 0.1.0
---

# AgentPool

## Prerequisites: Install the CLI

This skill drives the `agentpool-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install agentpool --cli-only
   ```
2. Verify: `agentpool-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/agentpool/cmd/agentpool-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this when you want a Printing Press catalog entry for AgentPool but still want the official AgentPool CLI to remain the source of truth. This wrapper checks installation, prints onboarding guidance, reads usage/capacity summaries through the official CLI, and delegates to `agentpool`.

Do not treat this as a separate AgentPool implementation. It does not choose providers, parse usage, inspect SQLite state, run MCP itself, rank models, merge branches, push code, store credentials, or add `provider=auto`.

## Command Reference

- `agentpool-pp-cli install` - Show the official install and upgrade flow for `agentpool-cli`.
- `agentpool-pp-cli doctor` - Delegate to `agentpool doctor --privacy`.
- `agentpool-pp-cli usage` - Delegate to `agentpool usage-summary --json`.
- `agentpool-pp-cli skill` - Print official AgentPool agent guidance.
- `agentpool-pp-cli mcp-config` - Delegate MCP host config generation to AgentPool.
- `agentpool-pp-cli exec` - Pass arguments through to the official `agentpool` binary.

## Recipes

Check the local setup:

```bash
agentpool-pp-cli doctor
```

Read cached usage through the official AgentPool CLI:

```bash
agentpool-pp-cli usage
```

Print the current AgentPool agent guidance:

```bash
agentpool-pp-cli skill
```

Generate MCP host configuration through the official AgentPool CLI:

```bash
agentpool-pp-cli mcp-config
```

Pass a command through unchanged:

```bash
agentpool-pp-cli exec preferences
```

## Safety

The wrapper should stay boring. If a future change adds provider adapters, usage parsers, session state, MCP server code, or policy logic here, reject it and move that behavior to the official AgentPool CLI instead.
