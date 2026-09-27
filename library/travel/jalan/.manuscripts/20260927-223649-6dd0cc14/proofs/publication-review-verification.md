# Publication review corrections

Both P1 findings from PR #2059 were reproduced and fixed. Partial accommodation results now survive all supported output paths without becoming a success-only result.

- MCP: a trusted stay command's wrapped process exit 8 preserves bounded stdout as the first content block and the diagnostic separately, with IsError=true. Real compiled subprocess tests verify successful observations, meta and fetch_failures remain parseable; ordinary successes and other failures are unchanged; oversized data and diagnostics carry explicit truncation metadata.
- Delivery: a completed partial stay envelope reaches the explicit file/webhook sink before the partial diagnostic is emitted. Execute-level tests verify byte-identical data, exactly one webhook call, stdout plus exit 8, no sink writes for usage/access/parse/empty failures, and one actionable delivery_failure diagnostic if the selected sink fails.

These induced failure cases are deterministic. Jalan query construction and source retrieval are unchanged. The fresh machine-written phase5-acceptance.json and publication-live-summary.json record renewed live acceptance on the corrected tree; full module tests, vet and build pass, and govulncheck reports no reachable vulnerabilities. The refreshed full live matrix passes 101/101 executed checks with 78 documented framework/inapplicable skips.

The narrow emitted-framework repair is recorded in .printing-press-patches/jalan-preserve-partial-output.json. Shared generator code and shared configuration were not changed. Original generation integrity and performance artifacts describe the pre-review baseline; the renewed acceptance fingerprint identifies the corrected publication source.
