# Publication review fixes

Verified on 2026-09-27 UTC after the first public review.

- Removed unsupported generated import, workflow, sync, search and export routes from the CLI and its MCP mirror. The calendar POST is a read-only query; the write registry is empty. MCP context retains the five typed source tools and eight planning mirrors with accurate bounds and time-window guidance.
- Reproduced CSV, TSV and quiet output successfully before the formatting refactor. Dispatch is now explicit; regression tests preserve exact output, price strings, nulls, freshness and writer errors.
- Companion stdout over 60,000 bytes returns an explicit MCP error without partial inventory or a preview. Complete JSON at 59,999 and 60,000 bytes remains exact; 60,001 bytes fails for ASCII and Japanese fixtures. Capture memory remains bounded and child streams are drained.
- Rejection tests confirm unsupported commands fail before HTTP, cache or local-store writes; all eight planning paths remain available.

Validation: focused CLI/planning tests pass; focused MCP registration/context and shellout tests pass; `go test -race -count=1 ./...`, `go vet ./...` and CLI/MCP builds pass. Fresh full live dogfood passes 87 mandatory checks with zero failures; 53 auxiliary rows remain skipped/unverified. All eight approved planning leaves pass live happy-path and JSON checks. Native publish validation passes all checks. The smaller matrix reflects removal of unsupported commands.

See `phase5-acceptance.json` for the source-bound proof and `.printing-press-patches/tablecheck-publish-readonly-surface-and-mcp-output.json` for the durable regeneration contract. Prior source comparisons and efficiency measurements remain historical observations; the review fixes do not change the TableCheck HTTP adapter.

## Follow-up review

A scan-shaped result reproduced an independent native-format issue: the competing venue/check/failure arrays became an outer-object CSV/plain row and empty quiet output. Native availability output now deliberately renders every check, including unknown and failed rows. Quiet mode emits one slug per check and rejects explicit selections that remove that identity. Full JSON stays unchanged. Regression fixtures cover two venues across two dates, partial failures, field selection, unchanged JSON and writer errors.

Raw-source local-data and network-fallback errors, staleness hints and max-age help now point to supported live reads or the separate freshness-aware planning cache. Tests retain original wrapped errors, status, local bytes and provenance, and preserve quiet fresh/disabled-age behavior.

The refreshed race/vet/build/publish checks pass, with 87 mandatory live checks passing and 53 auxiliary rows skipped. An additional two-day live scan matches default and selected CSV/plain rows and quiet identities against JSON. The initial JSON read used two requests; the final JSON read used zero and retained original fetch timestamps. See `native-format-live.json` for the bounded observation.
