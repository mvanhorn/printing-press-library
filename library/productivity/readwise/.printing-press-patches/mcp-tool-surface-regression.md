# MCP Tool Surface Regression Coverage

## Why this local patch exists

The generated MCP server registers tools through both typed in-process
handlers and a runtime Cobra-tree shell-out mirror. `search` is intentionally
typed, while the Cobra walker skips hidden commands, `mcp:hidden` commands, and
top-level framework commands to avoid duplicate or private shell-out tools.

This patch adds regression tests for that contract because a future generated
edit could otherwise silently overwrite a typed tool or expose a hidden
command.

## Reprint guard

On reprint, preserve MCP surface coverage that proves:

- The registered MCP server surface contains unique final tool names.
- `search` is present and resolves to the typed in-process handler, not the
  shell-out mirror.
- The Cobra-tree walker skips commands with `Hidden: true`, commands annotated
  with `mcp:hidden`, and top-level framework commands such as `search`.

The tests must remain pure in-process and must not require auth, network, a
real local store, or spawning the companion CLI.
