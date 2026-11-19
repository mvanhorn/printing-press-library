# MCP continuation is computed from the final page

This generated-tree patch belongs at the MCP adapter boundary: agent CLI wrappers, typed API cursors, and registration policy must agree with the Readwise runtime. It is an upstream Printing Press candidate; no CLI/store behavior changes here.

Preserve a dynamic canonical `PP_MCP_MAX_BYTES` budget (default 32000), including the exported `var MaxBytes = Budget()` compatibility line. Typed pages must return their fitted payload without a second envelope trim. Their opaque continuation follows the rows actually serialized; omit unknown counts and hide raw upstream cursors while an opaque cursor is present. Public `truncated` replaces `_pp_truncated`.

Mirror stdout has a separate 16 MiB hard ceiling, while stderr and failure text remain small and bounded. Page complete stdout before text bounding. Descend into `{meta,results}`, preserve outer and inner siblings, and include wrapper documentation in each registered mirror description. Envelope fitting sizes each item once and searches the page length rather than repeatedly encoding all rows.

Call `dropNonReadOnlyTools(s)` after every registration under the read-only environment switch. The call-line marker includes indentation to distinguish it from the function definition. An exact daily-review missing-detail 404 is a successful empty state; other 404s remain errors. Bulk-update 207 keeps per-item results and adds `partial_failure` and `failed_count`.

Behavioral and mutation evidence is in `SLICE-E1-REPORT.md`. This is a reprint guard, not a release ledger update.
