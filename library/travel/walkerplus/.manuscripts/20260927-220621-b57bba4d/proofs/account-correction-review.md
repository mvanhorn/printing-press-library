# Corrected-account review follow-up

PR2072 is submitted by zjsng from the zjsng fork. The earlier unmerged PR2060 is closed and links to its replacement; its original review history remains available.

The replacement review found one schedule defect and two generated MCP output defects. Explicit AND-connected weekday closures now stay together and are removed before positive recurrence parsing. The independently reproduced Monday/Thursday case excludes both closed days; remaining days stay possible unless daily activity is explicit. Non-exhaustive weekday lists remain unresolved.

The MCP mirror preserves compact small JSON byte-for-byte. Larger valid event-list JSON is compacted with complete event records, query, coverage and metadata retained. CLI JSON errors keep numeric codes and ordinary metadata, including when a long message needs an explicit limitation. Output capture is bounded to 1 MiB per stream; stdout overflow produces a structured error. Diagnostic truncation does not discard valid stdout. Aggregate returned text across content blocks is bounded to 60,000 bytes.

Focused MCP tests/vet/build pass, and the independent conjunction regression fails before the fix and passes after. The full Go suite passes 11 packages; six more have no tests. An initial sandboxed run could not bind local httptest servers; the authorized rerun passes. Both logs are retained. Vet and CLI/MCP/all-package builds pass. Source hashes before and after verification match.

Fresh uncached live E2E passes eight tests plus 16 subtests in 39.949 seconds. Full Printing Press live acceptance passes 50/50 executed checks with zero failures and 40 separately disclosed framework/inapplicable skips. Publication validation passes every check, including vulnerability and source-fingerprint checks. Fresh cold/warm measurements identify the exact private CLI binary used.

Workspace and canonical local source, CLI and stdio MCP bundles are updated. Original artifacts remain preserved; shared GitHub/Git configuration is unchanged. The local publication profile selects zjsng explicitly.
