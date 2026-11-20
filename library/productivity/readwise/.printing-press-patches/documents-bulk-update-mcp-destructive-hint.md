# Documents Bulk Update MCP Destructive Hint

## Why this local patch exists

Peer-security review found the typed MCP `documents_bulk-update` tool lacked a
destructive hint even though it can archive, move, or retag up to 50 Reader
documents in one request. CLI-side confirmation protects human CLI use, but MCP
hosts also need declarative annotations to surface confirmations on the agent
surface.

## Reprint guard

On reprint, preserve the typed MCP `documents_bulk-update` registration with:

- `readOnlyHint=false`
- `destructiveHint=true`
- a description that clearly says it is destructive and updates up to 50 Reader
  documents in one call

Keep the behavior declarative; do not change the handler logic or CLI gate for
this patch.
