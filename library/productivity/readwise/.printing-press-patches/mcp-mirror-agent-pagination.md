# MCP Mirror Agent Pagination

Local cobratree patch for the generated MCP command mirror:

- `internal/mcp/cobratree/shellout.go` forces `--agent` on mirrored CLI subprocesses.
- `internal/mcp/cobratree/shellout.go` byte-budgets mirrored output and consumes `offset` as the pagination cursor.
- `internal/mcp/cobratree/paginate.go` implements mirrored JSON array/object pagination.
- `internal/mcp/cobratree/walker.go` registers the `offset` parameter on each mirror tool.

These are LOCAL PATCHES. A bare reprint can drop them; re-apply after a reprint unless the cobratree generator has absorbed the same behavior upstream.
