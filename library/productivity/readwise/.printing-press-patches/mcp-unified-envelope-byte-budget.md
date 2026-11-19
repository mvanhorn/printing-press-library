# MCP Unified Envelope and Byte Budget

## Why this local patch exists

The generated MCP surface had two pagination contracts: typed/search/sql tools
used one list envelope and byte budget, while the command mirror used a separate
`items`/`_pagination` shape and a different byte budget. Agents had to special
case otherwise equivalent list results, and mirrored over-budget objects without
a top-level array could still exceed the MCP host budget.

## Reprint guard

On reprint, preserve these behaviors unless the Printing Press generator has
absorbed the same contract upstream:

- `internal/mcp/envelope` remains a leaf package imported by both typed MCP
  tools and `internal/mcp/cobratree`.
- All list-shaped MCP results use the flat canonical envelope:
  `{results,count,returned_count,truncated,next_offset,max_bytes,note}`.
- `PP_MCP_MAX_BYTES` is the single override for every MCP surface. The default
  of 32000 is a host-safe floor, not a regression from the earlier 60000 budget.
- Search and command-mirror pagination emit `next_offset` when truncated because
  they accept an `offset` input; typed REST list tools and SQL do not emit
  `next_offset`. SQL keeps LIMIT/OFFSET note guidance.
- Object responses with a top-level array preserve sibling metadata, replace
  the largest array field in place with the budgeted page, and attach pagination
  metadata instead of flattening to `{results:[...]}`.
- Mirrored over-budget output that is not pageable list-shaped JSON is replaced
  with the shared bounded preview envelope.

Do not update `CHANGELOG.md`, `.printing-press-release.json`, or generated
release version fields for this guard; those are stamped by the library publish
workflow.
