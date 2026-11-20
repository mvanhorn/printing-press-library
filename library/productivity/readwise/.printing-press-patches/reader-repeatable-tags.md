Reader list sends up to five tag query parameters, including empty tags; local filters fail closed.

Use StringArray and url.Values.Add for repeated tags. Reject more than five as usage error. With tags use live strategy so local data cannot silently ignore AND semantics. Typed MCP schema and handler must accept and encode arrays; keep bulk_update MCP method POST aligned with the CLI.

These are endpoint-specific API contracts from the 2026-11-18 drift report. Keep this intent when reprinting; do not replace the generic shared machinery.
