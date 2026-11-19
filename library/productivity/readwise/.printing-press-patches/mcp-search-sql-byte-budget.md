# MCP Search and SQL Byte Budget

## Why this local patch exists

The generated MCP `search` and `sql` tools previously returned bare, unbounded
JSON arrays. Large local search hits or analytical SQL result sets could exceed
an MCP host context window, unlike typed endpoint list tools that already use a
bounded envelope.

This patch routes both tools through the shared list envelope, adds `offset` to
the MCP search tool and generic store search, and keeps SQL byte-bounded without
the typed-tool 50-item cap.

## Reprint guard

On reprint, preserve these behaviors:

- MCP `search` always returns a `{results, count, ...}` envelope, even for small
  result sets.
- MCP `search` is capped by both `mcpToolResultMaxBytes` and
  `mcpToolResultMaxItems`, and exposes `offset` for paging.
- Generic `Store.Search` accepts `offset` and applies `LIMIT ? OFFSET ?`.
- MCP `sql` always returns the same envelope and is byte-budgeted, but is not
  item-capped at 50 rows.
- A single oversized row returns a valid truncation envelope with zero items
  instead of panicking or returning unbounded JSON.

The regression tests in `internal/mcp/tools_test.go` should stay in-process,
using a temp SQLite store only. They must not require network, auth, a real MCP
host, or the user's local database.
