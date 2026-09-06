# HomeClaw Printing Press research notes

## Source and runtime

- Source repository: `omarshahine/HomeClaw`
- Runtime app inspected: `/Applications/HomeClaw.app`
- CLI executable: `/Applications/HomeClaw.app/Contents/MacOS/homeclaw-cli`
- MCP server: `/Applications/HomeClaw.app/Contents/Resources/mcp-server.js`
- Platform requirement: macOS

## Observed CLI surface

The installed CLI exposes read-only discovery commands including `list`, `status`, `scenes`, `device-map`, and `events`, plus mutating HomeKit commands. The Printing Press wrapper delegates arguments without rewriting them and does not install, symlink, elevate, or authorize access.

## Observed MCP surface

The installed Node stdio server accepted JSON-RPC `initialize` with protocol version `2025-03-26` and returned server name `homeclaw`, version `1.0.1`, and the tools capability.

## Wrapper design decision

The package ships two binaries because Printing Press requires the MCP manifest entry point to be an MCP-named binary with no subcommand args. `homeclaw-pp-cli` remains the human/agent CLI wrapper; `homeclaw-pp-mcp` is the stdio MCP launcher.
