# HomeClaw — Printing Press wrapper

This catalog entry provides `homeclaw-pp-cli`, a thin macOS-only wrapper around the HomeClaw app's existing CLI and Node stdio MCP server. It does not install HomeClaw, create symlinks, request HomeKit permission, or perform HomeKit operations by itself.

## Install

```bash
npx -y @mvanhorn/printing-press-library install homeclaw --cli-only
```

HomeClaw must be installed from the Mac App Store and running for commands that use its Unix socket. Use `HOMECLAW_CLI_PATH`, `HOMECLAW_MCP_SERVER_PATH`, and `HOMECLAW_NODE_PATH` for nonstandard locations.

## Usage

```bash
homeclaw-pp-cli --help
homeclaw-pp-cli mcp
```

For Hermes, prefer the HomeClaw repository's portable Agent Plugin package; this wrapper exists for Printing Press catalog discovery and CLI/MCP compatibility.
