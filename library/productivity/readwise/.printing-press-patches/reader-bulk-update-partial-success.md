Reader bulk updates POST, preserve per-item results and fail scripts on HTTP 207.

Keep POST and the Reader-specific 207 count warning on command stderr; render item results through existing output/select pipeline before returning apiErr (exit 5), regardless of allow-partial-failure. Preserve success/error fields in compact output.

These are endpoint-specific API contracts from the 2026-11-18 drift report. Keep this intent when reprinting; do not replace the generic shared machinery.

Round 2 correction: bulk update uses PATCH in both CLI and typed MCP, matching the spec and pre-reprint CLI. No live verification is allowed in this slice; httptest asserts the wire method.
